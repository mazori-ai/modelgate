# Contributing to ModelGate

First off, thank you for considering contributing to ModelGate! It's people like you that make ModelGate such a great tool for the AI community.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [How to Contribute](#how-to-contribute)
- [Pull Request Process](#pull-request-process)
- [Style Guidelines](#style-guidelines)
- [Community](#community)

## Code of Conduct

This project and everyone participating in it is governed by our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report unacceptable behavior to [conduct@modelgate.dev](mailto:conduct@modelgate.dev).

## Getting Started

### Prerequisites

- **Go 1.22+** - Backend development
- **Node.js 20+** - Frontend development
- **PostgreSQL 15+** with pgvector extension
- **Docker** (optional, for containerized development)
- **Make** - Build automation

### Development Setup

1. **Fork and clone the repository**

   ```bash
   git clone https://github.com/YOUR_USERNAME/modelgate.git
   cd modelgate
   ```

2. **Set up the database**

   ```bash
   # Using Docker (recommended)
   docker run -d \
     --name modelgate-postgres \
     -e POSTGRES_USER=postgres \
     -e POSTGRES_PASSWORD=postgres \
     -e POSTGRES_DB=modelgate \
     -p 5432:5432 \
     pgvector/pgvector:pg16

   # Or install PostgreSQL locally with pgvector extension
   ```

3. **Install Go dependencies**

   ```bash
   go mod download
   ```

4. **Build the backend**

   ```bash
   make build
   # or
   go build -o bin/modelgate ./cmd/modelgate
   ```

5. **Set up the frontend**

   ```bash
   cd web
   npm install
   npm run build
   cd ..
   ```

6. **Configure environment**

   ```bash
   cp .env.example .env
   # Edit .env with your settings
   ```

7. **Run the server**

   ```bash
   make run
   # or
   ./bin/modelgate
   ```

8. **Verify it's working**

   ```bash
   curl http://localhost:8080/health
   ```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific package tests
go test ./internal/policy/...

# Run frontend tests
cd web && npm test
```

## How to Contribute

### Reporting Bugs

Before creating bug reports, please check the existing issues to avoid duplicates. When you create a bug report, include as many details as possible:

- **Use a clear and descriptive title**
- **Describe the exact steps to reproduce the problem**
- **Provide specific examples** (config files, API requests, etc.)
- **Describe the behavior you observed and expected**
- **Include logs** if applicable
- **Environment details** (OS, Go version, Docker version, etc.)

### Suggesting Features

Feature requests are welcome! Please:

- **Check if it already exists** in issues or discussions
- **Provide a clear use case** - why is this feature needed?
- **Describe the solution** you'd like
- **Consider alternatives** you've thought about

### Your First Code Contribution

Unsure where to begin? Look for issues labeled:

- `good first issue` - Simple issues for newcomers
- `help wanted` - Issues we'd love help with
- `documentation` - Docs improvements

### Pull Requests

1. **Create a branch** from `main`:
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-fix
   ```

2. **Make your changes** following our style guidelines

3. **Write or update tests** as needed

4. **Run the test suite**:
   ```bash
   make test
   make lint
   ```

5. **Commit with clear messages**:
   ```bash
   git commit -m "feat: add support for XYZ provider"
   # or
   git commit -m "fix: resolve rate limiting edge case"
   ```

6. **Push and create a Pull Request**

## Pull Request Process

1. **Fill out the PR template** completely
2. **Link related issues** using `Fixes #123` or `Closes #123`
3. **Ensure CI passes** - all tests and lints must pass
4. **Request review** from maintainers
5. **Address feedback** promptly
6. **Squash commits** if requested before merge

### PR Title Convention

We use [Conventional Commits](https://www.conventionalcommits.org/):

| Prefix | Description |
|--------|-------------|
| `feat:` | New feature |
| `fix:` | Bug fix |
| `docs:` | Documentation only |
| `style:` | Formatting, no code change |
| `refactor:` | Code change that neither fixes nor adds |
| `perf:` | Performance improvement |
| `test:` | Adding or updating tests |
| `chore:` | Maintenance tasks |

Examples:
- `feat: add Azure OpenAI provider support`
- `fix: resolve memory leak in connection pool`
- `docs: update API reference for chat completions`

## Style Guidelines

### Go Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` and `goimports`
- Run `golangci-lint` before committing
- Write descriptive function comments
- Keep functions focused and small
- Handle errors explicitly

```go
// Good
func (s *Server) handleRequest(ctx context.Context, req *Request) (*Response, error) {
    if err := s.validate(req); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }
    // ...
}

// Avoid
func (s *Server) handleRequest(ctx context.Context, req *Request) (*Response, error) {
    s.validate(req) // ignoring error
    // ...
}
```

### TypeScript/React Code Style

- Use TypeScript strict mode
- Follow React hooks best practices
- Use functional components
- Run `npm run lint` before committing

### Commit Messages

- Use present tense ("add feature" not "added feature")
- Use imperative mood ("move cursor to..." not "moves cursor to...")
- First line max 72 characters
- Reference issues in the body

### Documentation

- Update README if adding new features
- Add JSDoc/GoDoc comments for public APIs
- Include examples for complex functionality
- Keep docs in sync with code

## Project Structure

```
modelgate/
├── cmd/modelgate/        # Main application entry point
├── internal/
│   ├── domain/           # Core domain types
│   ├── gateway/          # LLM gateway service
│   ├── graphql/          # GraphQL API
│   ├── http/             # HTTP server and handlers
│   ├── mcp/              # MCP protocol implementation
│   ├── policy/           # Policy enforcement
│   └── storage/          # Database and storage
├── web/                  # React frontend
├── migrations/           # Database migrations
├── examples/             # Example integrations
└── docs/                 # Documentation
```

## Community

- **GitHub Discussions** - Questions and ideas
- **Discord** - Real-time chat (link in README)
- **Twitter** - Updates and announcements

## Recognition

Contributors are recognized in:
- README.md Contributors section
- Release notes
- Annual contributor appreciation

## Questions?

Feel free to:
- Open a Discussion for questions
- Join our Discord for real-time help
- Email [maintainers@modelgate.dev](mailto:maintainers@modelgate.dev)

Thank you for contributing! 🎉

