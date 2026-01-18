// Package gateway provides the core LLM gateway functionality.
package gateway

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"modelgate/internal/domain"
)

// Dispatcher errors
var (
	ErrQueueFull     = errors.New("request queue full - server overloaded")
	ErrQueueTimeout  = errors.New("request timed out waiting in queue")
	ErrShuttingDown  = errors.New("server is shutting down")
	ErrTenantLimited = errors.New("tenant concurrency limit reached")
)

// =============================================================================
// Request Types
// =============================================================================

// DispatchRequest wraps an incoming LLM request with response channel
type DispatchRequest struct {
	Ctx        context.Context
	ChatReq    *domain.ChatRequest
	TenantID   string
	TenantSlug string
	APIKeyID   string
	RoleID     string
	GroupID    string
	Priority   int // Higher = processed first (0-10)

	// Internal
	ResponseCh chan *DispatchResult
	EnqueuedAt time.Time
}

// DispatchResult contains the result of processing a request
type DispatchResult struct {
	Response *domain.ChatResponse      // For non-streaming
	EventsCh <-chan domain.StreamEvent // For streaming
	Error    error
}

// =============================================================================
// Dispatcher Configuration
// =============================================================================

// DispatcherConfig holds dispatcher configuration
type DispatcherConfig struct {
	// Adaptive worker pool settings
	MinWorkers    int           // Minimum workers (always running)
	MaxWorkers    int           // Maximum workers (scale up limit)
	IdleTimeout   time.Duration // How long idle workers wait before exiting
	ScaleUpStep   int           // Workers to add when scaling up
	ScaleDownStep int           // Workers to remove when scaling down

	// Queue settings
	MaxQueuedRequests int           // Total queue buffer size
	QueueTimeout      time.Duration // Max time to wait in queue

	// Scaling thresholds
	ScaleUpThreshold   float64       // Queue utilization % to trigger scale up
	ScaleDownThreshold float64       // Queue utilization % to trigger scale down
	ScaleInterval      time.Duration // How often to check for scaling

	// Queue distribution (percentages for priority queues)
	HighPriorityPercent   int // e.g., 30% of queue for high priority
	NormalPriorityPercent int // e.g., 50% of queue for normal priority
}

// DefaultDispatcherConfig returns sensible defaults for adaptive scaling
func DefaultDispatcherConfig() DispatcherConfig {
	return DispatcherConfig{
		MinWorkers:            5,   // Always have at least 5 workers
		MaxWorkers:            200, // Scale up to 200 under heavy load
		IdleTimeout:           30 * time.Second,
		ScaleUpStep:           10,
		ScaleDownStep:         5,
		MaxQueuedRequests:     1000,
		QueueTimeout:          60 * time.Second,
		ScaleUpThreshold:      0.7, // Scale up when queue > 70% full
		ScaleDownThreshold:    0.2, // Scale down when queue < 20% full
		ScaleInterval:         5 * time.Second,
		HighPriorityPercent:   30,
		NormalPriorityPercent: 50,
	}
}

// =============================================================================
// Dispatcher Metrics
// =============================================================================

// DispatcherMetrics tracks dispatcher performance
type DispatcherMetrics struct {
	// Request counts
	RequestsReceived  int64
	RequestsQueued    int64
	RequestsProcessed int64
	RequestsRejected  int64
	RequestsTimedOut  int64

	// Queue depths (current)
	HighPriorityQueueDepth   int32
	NormalPriorityQueueDepth int32
	LowPriorityQueueDepth    int32

	// Worker stats
	CurrentWorkers    int32
	WorkersScaledUp   int64
	WorkersScaledDown int64

	// Timing (in milliseconds)
	TotalQueueWaitMs  int64
	TotalProcessingMs int64
	RequestCount      int64 // For computing averages
	MaxQueueWaitMs    int64
	MaxProcessingMs   int64
	LastQueueWaitMs   int64
	LastProcessingMs  int64
}

// =============================================================================
// Per-Tenant Limiting
// =============================================================================

// TenantLimiter tracks per-tenant concurrency limits
type TenantLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*tenantSemaphore
}

type tenantSemaphore struct {
	current int32
	limit   int32
}

// NewTenantLimiter creates a new tenant limiter
func NewTenantLimiter() *TenantLimiter {
	return &TenantLimiter{
		limiters: make(map[string]*tenantSemaphore),
	}
}

// SetLimit sets or updates the limit for a tenant
func (tl *TenantLimiter) SetLimit(tenantID string, limit int32) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	if sem, exists := tl.limiters[tenantID]; exists {
		sem.limit = limit
	} else {
		tl.limiters[tenantID] = &tenantSemaphore{limit: limit}
	}
}

// Acquire tries to acquire a slot for a tenant
func (tl *TenantLimiter) Acquire(tenantID string, limit int32) bool {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	sem, exists := tl.limiters[tenantID]
	if !exists {
		sem = &tenantSemaphore{limit: limit}
		tl.limiters[tenantID] = sem
	} else if limit > 0 {
		sem.limit = limit // Update limit if provided
	}

	if sem.current >= sem.limit {
		return false
	}

	sem.current++
	return true
}

// Release releases a slot for a tenant
func (tl *TenantLimiter) Release(tenantID string) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	if sem, exists := tl.limiters[tenantID]; exists {
		if sem.current > 0 {
			sem.current--
		}
	}
}

// GetStats returns tenant concurrency stats
func (tl *TenantLimiter) GetStats(tenantID string) (current, limit int32) {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	if sem, exists := tl.limiters[tenantID]; exists {
		return sem.current, sem.limit
	}
	return 0, 0
}

// =============================================================================
// Dispatcher Implementation
// =============================================================================

// Dispatcher manages request queuing with adaptive worker pool
type Dispatcher struct {
	mu sync.RWMutex

	// Configuration
	config DispatcherConfig

	// Priority-based request queues
	highPriorityQueue   chan *DispatchRequest
	normalPriorityQueue chan *DispatchRequest
	lowPriorityQueue    chan *DispatchRequest

	// Adaptive worker pool
	activeWorkers atomic.Int32
	workerWg      sync.WaitGroup
	workAvailable chan struct{} // Signal that work is available
	shutdownCh    chan struct{}
	isRunning     bool

	// Gateway service for actual processing
	gateway *Service

	// Per-tenant limiting
	tenantLimiter *TenantLimiter

	// Scaling control
	scalerStop chan struct{}

	// Metrics
	metrics DispatcherMetrics
}

// NewDispatcher creates a new adaptive request dispatcher
func NewDispatcher(cfg DispatcherConfig, gateway *Service) *Dispatcher {
	// Calculate queue sizes based on percentages
	highQueueSize := (cfg.MaxQueuedRequests * cfg.HighPriorityPercent) / 100
	normalQueueSize := (cfg.MaxQueuedRequests * cfg.NormalPriorityPercent) / 100
	lowQueueSize := cfg.MaxQueuedRequests - highQueueSize - normalQueueSize

	if highQueueSize < 1 {
		highQueueSize = 1
	}
	if normalQueueSize < 1 {
		normalQueueSize = 1
	}
	if lowQueueSize < 1 {
		lowQueueSize = 1
	}

	d := &Dispatcher{
		config:              cfg,
		highPriorityQueue:   make(chan *DispatchRequest, highQueueSize),
		normalPriorityQueue: make(chan *DispatchRequest, normalQueueSize),
		lowPriorityQueue:    make(chan *DispatchRequest, lowQueueSize),
		workAvailable:       make(chan struct{}, 1),
		shutdownCh:          make(chan struct{}),
		scalerStop:          make(chan struct{}),
		gateway:             gateway,
		tenantLimiter:       NewTenantLimiter(),
		metrics:             DispatcherMetrics{},
	}

	slog.Info("Adaptive dispatcher created",
		"min_workers", cfg.MinWorkers,
		"max_workers", cfg.MaxWorkers,
		"max_queued", cfg.MaxQueuedRequests,
		"high_queue_size", highQueueSize,
		"normal_queue_size", normalQueueSize,
		"low_queue_size", lowQueueSize,
		"scale_up_threshold", cfg.ScaleUpThreshold,
		"scale_down_threshold", cfg.ScaleDownThreshold,
	)

	return d
}

// Start launches the adaptive worker pool
func (d *Dispatcher) Start() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.isRunning {
		return
	}

	d.isRunning = true

	// Start minimum workers
	for i := 0; i < d.config.MinWorkers; i++ {
		d.spawnWorker()
	}

	// Start auto-scaler
	go d.autoScaler()

	slog.Info("Adaptive dispatcher started", "initial_workers", d.config.MinWorkers)
}

// spawnWorker creates a new adaptive worker goroutine
func (d *Dispatcher) spawnWorker() {
	d.workerWg.Add(1)
	d.activeWorkers.Add(1)
	go d.worker()
}

// Stop gracefully shuts down the dispatcher
func (d *Dispatcher) Stop() {
	d.mu.Lock()
	if !d.isRunning {
		d.mu.Unlock()
		return
	}
	d.isRunning = false
	d.mu.Unlock()

	slog.Info("Dispatcher stopping...")
	close(d.scalerStop) // Stop scaler first
	close(d.shutdownCh)
	d.workerWg.Wait()
	slog.Info("Dispatcher stopped")
}

// Submit submits a request for processing with backpressure
func (d *Dispatcher) Submit(ctx context.Context, req *DispatchRequest) (*DispatchResult, error) {
	atomic.AddInt64(&d.metrics.RequestsReceived, 1)

	req.EnqueuedAt = time.Now()
	req.ResponseCh = make(chan *DispatchResult, 1)

	// Select appropriate queue based on priority
	queue := d.selectQueue(req.Priority)

	// Try to enqueue without blocking
	select {
	case queue <- req:
		// Successfully queued
		atomic.AddInt64(&d.metrics.RequestsQueued, 1)
		d.updateQueueDepth(req.Priority, 1)

		// Signal that work is available
		select {
		case d.workAvailable <- struct{}{}:
		default:
		}

		return d.waitForResult(ctx, req)

	case <-ctx.Done():
		atomic.AddInt64(&d.metrics.RequestsTimedOut, 1)
		return nil, ctx.Err()

	default:
		// Queue is full - apply backpressure
		atomic.AddInt64(&d.metrics.RequestsRejected, 1)

		slog.Warn("Request rejected - queue full",
			"priority", req.Priority,
			"tenant", req.TenantSlug,
			"current_workers", d.activeWorkers.Load(),
		)

		return nil, ErrQueueFull
	}
}

// selectQueue returns the appropriate queue based on priority (0-10)
func (d *Dispatcher) selectQueue(priority int) chan *DispatchRequest {
	switch {
	case priority >= 8:
		return d.highPriorityQueue
	case priority >= 4:
		return d.normalPriorityQueue
	default:
		return d.lowPriorityQueue
	}
}

// updateQueueDepth updates queue depth metrics
func (d *Dispatcher) updateQueueDepth(priority int, delta int32) {
	switch {
	case priority >= 8:
		atomic.AddInt32(&d.metrics.HighPriorityQueueDepth, delta)
	case priority >= 4:
		atomic.AddInt32(&d.metrics.NormalPriorityQueueDepth, delta)
	default:
		atomic.AddInt32(&d.metrics.LowPriorityQueueDepth, delta)
	}
}

// waitForResult waits for the request to be processed
func (d *Dispatcher) waitForResult(ctx context.Context, req *DispatchRequest) (*DispatchResult, error) {
	select {
	case result := <-req.ResponseCh:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-d.shutdownCh:
		return nil, ErrShuttingDown
	case <-time.After(d.config.QueueTimeout):
		atomic.AddInt64(&d.metrics.RequestsTimedOut, 1)
		return nil, ErrQueueTimeout
	}
}

// worker is an adaptive worker that exits when idle too long
func (d *Dispatcher) worker() {
	defer func() {
		d.activeWorkers.Add(-1)
		d.workerWg.Done()
	}()

	idleTimer := time.NewTimer(d.config.IdleTimeout)
	defer idleTimer.Stop()

	for {
		// Reset idle timer
		if !idleTimer.Stop() {
			select {
			case <-idleTimer.C:
			default:
			}
		}
		idleTimer.Reset(d.config.IdleTimeout)

		// Priority-based selection: high > normal > low
		select {
		case <-d.shutdownCh:
			return

		case req := <-d.highPriorityQueue:
			d.updateQueueDepth(10, -1)
			d.processRequest(req)

		default:
			select {
			case <-d.shutdownCh:
				return

			case req := <-d.highPriorityQueue:
				d.updateQueueDepth(10, -1)
				d.processRequest(req)

			case req := <-d.normalPriorityQueue:
				d.updateQueueDepth(5, -1)
				d.processRequest(req)

			default:
				select {
				case <-d.shutdownCh:
					return

				case req := <-d.highPriorityQueue:
					d.updateQueueDepth(10, -1)
					d.processRequest(req)

				case req := <-d.normalPriorityQueue:
					d.updateQueueDepth(5, -1)
					d.processRequest(req)

				case req := <-d.lowPriorityQueue:
					d.updateQueueDepth(0, -1)
					d.processRequest(req)

				case <-idleTimer.C:
					// Check if we should exit (above minimum workers)
					current := d.activeWorkers.Load()
					if int(current) > d.config.MinWorkers {
						slog.Debug("Worker exiting due to idle timeout",
							"current_workers", current,
							"min_workers", d.config.MinWorkers)
						atomic.AddInt64(&d.metrics.WorkersScaledDown, 1)
						return
					}
					// We're at minimum, continue waiting

				case <-d.workAvailable:
					// Work signal received, loop back to try getting work
				}
			}
		}
	}
}

// processRequest does the actual work with per-tenant limiting
func (d *Dispatcher) processRequest(req *DispatchRequest) {
	// Record queue wait time
	waitTime := time.Since(req.EnqueuedAt)
	waitMs := waitTime.Milliseconds()
	atomic.AddInt64(&d.metrics.TotalQueueWaitMs, waitMs)
	atomic.StoreInt64(&d.metrics.LastQueueWaitMs, waitMs)

	// Update max wait time
	for {
		current := atomic.LoadInt64(&d.metrics.MaxQueueWaitMs)
		if waitMs <= current || atomic.CompareAndSwapInt64(&d.metrics.MaxQueueWaitMs, current, waitMs) {
			break
		}
	}

	// Check if context already cancelled
	if req.Ctx.Err() != nil {
		req.ResponseCh <- &DispatchResult{Error: req.Ctx.Err()}
		return
	}

	// Get tenant limit based on plan
	tenantLimit := d.getTenantLimit(req.TenantSlug)

	// Try to acquire tenant slot
	if !d.tenantLimiter.Acquire(req.TenantID, tenantLimit) {
		slog.Warn("Tenant concurrency limit reached",
			"tenant", req.TenantSlug,
			"limit", tenantLimit)
		req.ResponseCh <- &DispatchResult{Error: ErrTenantLimited}
		return
	}
	defer d.tenantLimiter.Release(req.TenantID)

	processStart := time.Now()

	// Process via gateway
	var result DispatchResult
	if req.ChatReq.Streaming {
		events, err := d.gateway.ChatStream(req.Ctx, req.ChatReq)
		result = DispatchResult{EventsCh: events, Error: err}
	} else {
		resp, err := d.gateway.ChatComplete(req.Ctx, req.ChatReq)
		result = DispatchResult{Response: resp, Error: err}
	}

	// Record processing time
	processingMs := time.Since(processStart).Milliseconds()
	atomic.AddInt64(&d.metrics.TotalProcessingMs, processingMs)
	atomic.StoreInt64(&d.metrics.LastProcessingMs, processingMs)
	atomic.AddInt64(&d.metrics.RequestCount, 1)

	// Update max processing time
	for {
		current := atomic.LoadInt64(&d.metrics.MaxProcessingMs)
		if processingMs <= current || atomic.CompareAndSwapInt64(&d.metrics.MaxProcessingMs, current, processingMs) {
			break
		}
	}

	atomic.AddInt64(&d.metrics.RequestsProcessed, 1)

	// Send result
	select {
	case req.ResponseCh <- &result:
	default:
		// Response channel closed or full - request was cancelled
	}
}

// getTenantLimit returns the concurrent request limit for a tenant based on plan
func (d *Dispatcher) getTenantLimit(tenantSlug string) int32 {
	// TODO: Look up from database based on tenant plan
	// For now, use default limits based on tenant type:
	//   Free:         5 concurrent requests
	//   Starter:     20 concurrent requests
	//   Professional: 50 concurrent requests
	//   Enterprise: 100 concurrent requests

	// Default to starter tier limit
	return 20
}

// SetTenantLimit allows dynamically setting tenant limits
func (d *Dispatcher) SetTenantLimit(tenantID string, limit int32) {
	d.tenantLimiter.SetLimit(tenantID, limit)
}

// TenantStats returns per-tenant concurrency stats
func (d *Dispatcher) TenantStats(tenantID string) (current, limit int32) {
	return d.tenantLimiter.GetStats(tenantID)
}

// autoScaler monitors load and adjusts worker count
func (d *Dispatcher) autoScaler() {
	ticker := time.NewTicker(d.config.ScaleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.scalerStop:
			return
		case <-ticker.C:
			d.checkAndScale()
		}
	}
}

// checkAndScale adjusts worker count based on queue utilization
func (d *Dispatcher) checkAndScale() {
	// Calculate queue utilization
	queued := len(d.highPriorityQueue) + len(d.normalPriorityQueue) + len(d.lowPriorityQueue)
	maxQueued := d.config.MaxQueuedRequests
	utilization := float64(queued) / float64(maxQueued)

	currentWorkers := int(d.activeWorkers.Load())

	if utilization > d.config.ScaleUpThreshold && currentWorkers < d.config.MaxWorkers {
		// Scale up
		toAdd := d.config.ScaleUpStep
		if currentWorkers+toAdd > d.config.MaxWorkers {
			toAdd = d.config.MaxWorkers - currentWorkers
		}

		if toAdd > 0 {
			slog.Info("Scaling up workers",
				"current", currentWorkers,
				"adding", toAdd,
				"utilization", utilization,
			)

			for i := 0; i < toAdd; i++ {
				d.spawnWorker()
			}
			atomic.AddInt64(&d.metrics.WorkersScaledUp, int64(toAdd))
		}

	} else if utilization < d.config.ScaleDownThreshold && currentWorkers > d.config.MinWorkers {
		// Scale down is handled by idle timeout in workers
		// Just log for observability
		slog.Debug("Low utilization, workers will scale down via idle timeout",
			"current", currentWorkers,
			"min", d.config.MinWorkers,
			"utilization", utilization,
		)
	}
}

// =============================================================================
// Metrics & Health
// =============================================================================

// Stats returns a copy of current dispatcher metrics
func (d *Dispatcher) Stats() DispatcherMetrics {
	return DispatcherMetrics{
		RequestsReceived:         atomic.LoadInt64(&d.metrics.RequestsReceived),
		RequestsQueued:           atomic.LoadInt64(&d.metrics.RequestsQueued),
		RequestsProcessed:        atomic.LoadInt64(&d.metrics.RequestsProcessed),
		RequestsRejected:         atomic.LoadInt64(&d.metrics.RequestsRejected),
		RequestsTimedOut:         atomic.LoadInt64(&d.metrics.RequestsTimedOut),
		HighPriorityQueueDepth:   atomic.LoadInt32(&d.metrics.HighPriorityQueueDepth),
		NormalPriorityQueueDepth: atomic.LoadInt32(&d.metrics.NormalPriorityQueueDepth),
		LowPriorityQueueDepth:    atomic.LoadInt32(&d.metrics.LowPriorityQueueDepth),
		CurrentWorkers:           d.activeWorkers.Load(),
		WorkersScaledUp:          atomic.LoadInt64(&d.metrics.WorkersScaledUp),
		WorkersScaledDown:        atomic.LoadInt64(&d.metrics.WorkersScaledDown),
		TotalQueueWaitMs:         atomic.LoadInt64(&d.metrics.TotalQueueWaitMs),
		TotalProcessingMs:        atomic.LoadInt64(&d.metrics.TotalProcessingMs),
		RequestCount:             atomic.LoadInt64(&d.metrics.RequestCount),
		MaxQueueWaitMs:           atomic.LoadInt64(&d.metrics.MaxQueueWaitMs),
		MaxProcessingMs:          atomic.LoadInt64(&d.metrics.MaxProcessingMs),
		LastQueueWaitMs:          atomic.LoadInt64(&d.metrics.LastQueueWaitMs),
		LastProcessingMs:         atomic.LoadInt64(&d.metrics.LastProcessingMs),
	}
}

// IsHealthy returns true if dispatcher is operating normally
func (d *Dispatcher) IsHealthy() bool {
	d.mu.RLock()
	running := d.isRunning
	d.mu.RUnlock()

	if !running {
		return false
	}

	// Check if queues are not completely full
	highFull := len(d.highPriorityQueue) >= cap(d.highPriorityQueue)
	normalFull := len(d.normalPriorityQueue) >= cap(d.normalPriorityQueue)
	lowFull := len(d.lowPriorityQueue) >= cap(d.lowPriorityQueue)

	// Unhealthy if all queues are full
	if highFull && normalFull && lowFull {
		return false
	}

	return true
}

// Capacity returns current capacity information
func (d *Dispatcher) Capacity() (active, maxConcurrent, queued, maxQueued int) {
	active = int(d.activeWorkers.Load())
	maxConcurrent = d.config.MaxWorkers

	queued = len(d.highPriorityQueue) + len(d.normalPriorityQueue) + len(d.lowPriorityQueue)
	maxQueued = cap(d.highPriorityQueue) + cap(d.normalPriorityQueue) + cap(d.lowPriorityQueue)

	return
}

// AvgQueueWaitMs returns average queue wait time in milliseconds
func (d *Dispatcher) AvgQueueWaitMs() float64 {
	count := atomic.LoadInt64(&d.metrics.RequestCount)
	if count == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&d.metrics.TotalQueueWaitMs)) / float64(count)
}

// AvgProcessingMs returns average processing time in milliseconds
func (d *Dispatcher) AvgProcessingMs() float64 {
	count := atomic.LoadInt64(&d.metrics.RequestCount)
	if count == 0 {
		return 0
	}
	return float64(atomic.LoadInt64(&d.metrics.TotalProcessingMs)) / float64(count)
}
