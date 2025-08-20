# Contributing to ConureDB

Thank you for your interest in contributing to ConureDB! This document provides guidelines and information for contributors.

## Code of Conduct

By participating in this project, you are expected to uphold our Code of Conduct. Please be respectful, inclusive, and constructive in all interactions.

## How to Contribute

### Reporting Bugs

1. Check if the bug has already been reported in [Issues](https://github.com/conure-db/conure-db/issues)
2. If not, create a new issue using the bug report template
3. Include as much detail as possible: version, environment, steps to reproduce, logs

### Suggesting Features

1. Check existing [Issues](https://github.com/conure-db/conure-db/issues) and [Discussions](https://github.com/conure-db/conure-db/discussions)
2. Create a new issue using the feature request template
3. Clearly describe the use case and expected behavior

### Contributing Code

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/your-feature-name`
3. Make your changes following our coding standards
4. Add tests for new functionality
5. Ensure all tests pass: `go test ./...`
6. Commit with clear, descriptive messages
7. Push to your fork and create a Pull Request

## Development Setup

### Prerequisites

- Go 1.23.0 or later
- Git

### Local Development

```bash
# Clone your fork
git clone https://github.com/your-username/conure-db.git
cd conure-db

# Install dependencies
go mod download

# Build the project
go build ./...

# Run tests
go test ./...

# Run a local instance
go run ./cmd/conure-db --node-id=dev --data-dir=./dev-data --bootstrap
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests in verbose mode
go test -v ./...

# Run specific package tests
go test ./pkg/api
```

### Testing Kubernetes Deployment

```bash
# Install with Helm (requires Kubernetes cluster)
helm install conuredb ./charts/conuredb-ha

# Test scaling
helm upgrade conuredb ./charts/conuredb-ha --set voters.replicas=5
```

## Coding Standards

### Go Style

- Follow standard Go conventions and `gofmt` formatting
- Use meaningful variable and function names
- Add comments for exported functions and complex logic
- Keep functions focused and reasonably sized
- Handle errors appropriately

### Code Organization

- Place related functionality in appropriate packages
- Separate business logic from HTTP handlers
- Use interfaces to define contracts between components
- Write testable code with dependency injection where appropriate

### Documentation

- Update README.md for user-facing changes
- Add godoc comments for exported functions
- Update Kubernetes documentation for deployment changes
- Include examples in documentation

## Testing Guidelines

### Unit Tests

- Write tests for all new functionality
- Test both happy path and error conditions
- Use table-driven tests where appropriate
- Mock external dependencies

### Integration Tests

- Test complete workflows end-to-end
- Test cluster formation and scaling scenarios
- Test API endpoints with real HTTP requests
- Test Kubernetes deployment scenarios

### Performance Tests

- Include benchmarks for performance-critical code
- Test memory usage and allocations
- Test cluster performance under load

## Commit Message Guidelines

Use clear, descriptive commit messages:

```txt
type(scope): short description

Longer explanation if needed

- Additional bullet points if needed
- Reference issues: Fixes #123
```

Types:

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `chore`: Maintenance tasks

Examples:

```txt
feat(api): add follower read support with stale parameter
fix(raft): handle leadership transfer during scaling
docs(k8s): update Helm chart documentation
test(btree): add benchmarks for large datasets
```

## Review Process

### Pull Request Guidelines

- Keep PRs focused on a single change
- Include tests for new functionality
- Update documentation as needed
- Ensure CI passes before requesting review
- Respond to feedback promptly and constructively

### Review Criteria

Reviewers will check:

- Code quality and style
- Test coverage
- Documentation updates
- Backward compatibility
- Performance implications
- Security considerations

## Release Process

ConureDB follows semantic versioning (SemVer):

- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

## Getting Help

- Create an issue for bugs or feature requests
- Start a discussion for questions or ideas
- Check existing documentation and issues first
- Be patient and respectful when asking for help

## Recognition

Contributors will be recognized in:

- Release notes for significant contributions
- GitHub contributors list
- Special mentions for major features or fixes

Thank you for contributing to ConureDB!
