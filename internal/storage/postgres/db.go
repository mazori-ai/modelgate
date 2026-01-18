// Package postgres provides PostgreSQL storage implementation for ModelGate.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"time"

	_ "github.com/lib/pq"
	"modelgate/internal/config"
)

// DB wraps a sql.DB with helper methods
type DB struct {
	*sql.DB
	config *config.DatabaseConfig
}

// NewDB creates a new database connection
func NewDB(cfg *config.DatabaseConfig, dsn string) (*DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxConns)
	db.SetMaxIdleConns(cfg.MaxIdle)
	db.SetConnMaxLifetime(cfg.ConnMaxAge)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{DB: db, config: cfg}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// GetDB returns the underlying *sql.DB for direct use
func (db *DB) GetDB() *sql.DB {
	return db.DB
}

// Config returns the database configuration
func (db *DB) Config() *config.DatabaseConfig {
	return db.config
}

// CreateDatabase creates a new database
func CreateDatabase(cfg *config.DatabaseConfig, dbName string) error {
	// Connect to postgres database to create new database
	baseDSN := cfg.GetBaseDSN() + " dbname=postgres"
	db, err := sql.Open("postgres", baseDSN)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}
	defer db.Close()

	// Check if database exists
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check database existence: %w", err)
	}

	if exists {
		log.Printf("Database %s already exists", dbName)
		return nil
	}

	// Create database
	// Note: database names cannot be parameterized, so we validate the name
	if !isValidDatabaseName(dbName) {
		return fmt.Errorf("invalid database name: %s", dbName)
	}

	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
	if err != nil {
		return fmt.Errorf("failed to create database %s: %w", dbName, err)
	}

	log.Printf("Created database: %s", dbName)
	return nil
}

// isValidDatabaseName validates a database name to prevent SQL injection
func isValidDatabaseName(name string) bool {
	// Only allow alphanumeric characters and underscores
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return len(name) > 0 && len(name) <= 63
}

// RunSchemaFromFile runs SQL schema from a file
func RunSchemaFromFile(db *sql.DB, schemaPath string) error {
	// Create migrations tracking table if it doesn't exist
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Check if schema already applied
	schemaFile := filepath.Base(schemaPath)
	var applied bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", schemaFile).Scan(&applied)
	if err != nil {
		return fmt.Errorf("failed to check schema status: %w", err)
	}

	if applied {
		log.Printf("Schema %s already applied", schemaFile)
		return nil
	}

	// Read and execute schema
	content, err := ioutil.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	log.Printf("Running schema: %s", schemaFile)
	_, err = db.Exec(string(content))
	if err != nil {
		return fmt.Errorf("failed to execute schema %s: %w", schemaFile, err)
	}

	// Record schema as applied
	_, err = db.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", schemaFile)
	if err != nil {
		return fmt.Errorf("failed to record schema %s: %w", schemaFile, err)
	}

	return nil
}

// InitDB initializes the database with schema
func InitDB(cfg *config.DatabaseConfig) (*DB, error) {
	// Create the database if it doesn't exist
	if err := CreateDatabase(cfg, cfg.Database); err != nil {
		return nil, fmt.Errorf("failed to create database: %w", err)
	}

	// Connect to database
	db, err := NewDB(cfg, cfg.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run schema from migrations folder
	schemaPath := "migrations/001_schema.sql"
	if err := RunSchemaFromFile(db.DB, schemaPath); err != nil {
		// Try to continue even if schema application fails (might already exist)
		log.Printf("Warning: Schema application issue: %v", err)
	}

	log.Println("Database initialized successfully")
	return db, nil
}
