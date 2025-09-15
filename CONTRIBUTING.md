# Contributing to OCPP Server

Thank you for your interest in contributing to the OCPP Server project! This document provides guidelines and information for contributors.

## 📋 Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Contribution Workflow](#contribution-workflow)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Documentation](#documentation)
- [Submitting Changes](#submitting-changes)

## 🤝 Code of Conduct

This project adheres to a code of conduct that promotes a welcoming and inclusive environment:

- **Be respectful**: Treat all contributors with respect and professionalism
- **Be inclusive**: Welcome contributors of all backgrounds and experience levels
- **Be collaborative**: Work together to solve problems and improve the project
- **Be constructive**: Provide helpful feedback and suggestions

## 🚀 Getting Started

### Prerequisites

Before contributing, ensure you have:

- **Go 1.16+** installed
- **Git** for version control
- **Docker & Docker Compose** for testing
- **Redis** for local development
- Basic understanding of OCPP protocols (helpful but not required)

### Development Environment

1. **Fork the repository**
   ```bash
   # Fork on GitHub, then clone your fork
   git clone https://github.com/YOUR_USERNAME/ocpp-server.git
   cd ocpp-server
   ```

2. **Set up upstream remote**
   ```bash
   git remote add upstream https://github.com/ORIGINAL_OWNER/ocpp-server.git
   ```

3. **Install dependencies**
   ```bash
   go mod tidy
   ```

4. **Start Redis** (for local testing)
   ```bash
   # Using Docker
   docker run -d -p 6379:6379 redis:7-alpine

   # Or use Docker Compose
   docker-compose up redis -d
   ```

5. **Run the server**
   ```bash
   go run main.go
   ```

6. **Verify setup**
   ```bash
   curl http://localhost:8081/health
   ```

## 🛠 Development Setup

### Project Structure

```
ocpp-server/
├── internal/           # Core server implementation
│   ├── server/        # Server setup and management
│   ├── api/           # HTTP API handlers
│   ├── ocpp/          # OCPP protocol handlers
│   └── state/         # State management
├── handlers/          # HTTP request handlers
├── models/            # Data models and structures
├── config/            # Configuration management
├── tests/             # Test suites
├── scripts/           # Build and utility scripts
└── docs/              # Documentation
```

### Build and Run

```bash
# Development build
go build -o bin/ocpp-server main.go

# Run with hot reload (install air first: go install github.com/cosmtrek/air@latest)
air

# Build Docker image
docker build -t ocpp-server:dev .

# Run full stack
docker-compose up -d
```

## 🔄 Contribution Workflow

### 1. Create Feature Branch

```bash
# Ensure main branch is up-to-date
git checkout main
git pull upstream main

# Create feature branch
git checkout -b feature/your-feature-name
```

### 2. Make Changes

- Write code following our [coding standards](#coding-standards)
- Add tests for new functionality
- Update documentation as needed
- Ensure all tests pass

### 3. Commit Changes

```bash
# Stage your changes
git add .

# Commit with descriptive message
git commit -m "feat: add new charge point status endpoint

- Add GET /api/v1/chargepoints/{id}/status endpoint
- Include connection status and last heartbeat
- Add comprehensive error handling
- Update API documentation"
```

### 4. Push and Create PR

```bash
# Push to your fork
git push origin feature/your-feature-name

# Create Pull Request on GitHub
```

## 📝 Coding Standards

### Go Style Guide

Follow standard Go conventions:

- **Formatting**: Use `gofmt` for consistent formatting
- **Naming**: Use descriptive names following Go conventions
- **Documentation**: Add godoc comments for public functions
- **Error Handling**: Always handle errors appropriately
- **Testing**: Write unit tests for new functionality

### Code Quality Tools

```bash
# Format code
go fmt ./...

# Run static analysis
go vet ./...

# Run linters (install golangci-lint first)
golangci-lint run

# Check for security issues (install gosec first)
gosec ./...
```

### Commit Message Format

Follow conventional commit format:

```
<type>(<scope>): <description>

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**
```
feat(api): add charge point status endpoint
fix(redis): handle connection timeout gracefully
docs(readme): update installation instructions
test(server): add integration tests for WebSocket
```

## 🧪 Testing Guidelines

### Test Categories

1. **Unit Tests**: Test individual functions and methods
2. **Integration Tests**: Test component interactions
3. **End-to-End Tests**: Test complete workflows
4. **Load Tests**: Performance and scalability testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test package
go test ./internal/server -v

# Run integration tests
go test ./tests/integration/... -v

# Run with race detection
go test -race ./...
```

### Writing Tests

```go
func TestChargePointStatus(t *testing.T) {
    // Arrange
    server := setupTestServer(t)
    defer server.Close()

    // Act
    resp, err := http.Get(server.URL + "/api/v1/chargepoints/cp001/status")

    // Assert
    require.NoError(t, err)
    assert.Equal(t, http.StatusOK, resp.StatusCode)

    var status ChargePointStatus
    err = json.NewDecoder(resp.Body).Decode(&status)
    require.NoError(t, err)
    assert.Equal(t, "Available", status.Status)
}
```

### Test Data

- Use test fixtures for consistent test data
- Clean up test data after tests complete
- Use Docker containers for integration testing
- Mock external dependencies appropriately

## 📚 Documentation

### Documentation Requirements

- **Code Comments**: Godoc comments for public APIs
- **README Updates**: Update README for new features
- **API Documentation**: Document new endpoints
- **Change Documentation**: Update CHANGELOG.md

### Documentation Style

- **Clear and Concise**: Write for developers unfamiliar with the codebase
- **Examples**: Include code examples and usage patterns
- **API Documentation**: Follow OpenAPI/Swagger standards
- **Architecture**: Document design decisions and trade-offs

## 📤 Submitting Changes

### Pull Request Guidelines

1. **Title**: Clear, descriptive title following conventional commits
2. **Description**: Explain what changes were made and why
3. **Testing**: Describe how changes were tested
4. **Documentation**: Note any documentation updates
5. **Breaking Changes**: Clearly mark any breaking changes

### PR Template

```markdown
## Description
Brief description of changes made.

## Type of Change
- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that causes existing functionality to change)
- [ ] Documentation update

## Testing
- [ ] Unit tests pass
- [ ] Integration tests pass
- [ ] Load tests pass (if applicable)
- [ ] Manual testing completed

## Documentation
- [ ] Code comments updated
- [ ] README updated (if needed)
- [ ] CHANGELOG updated
- [ ] API documentation updated (if needed)

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] Tests added for new functionality
- [ ] All tests pass
- [ ] Documentation updated
```

### Review Process

1. **Automated Checks**: CI/CD pipeline runs automatically
2. **Code Review**: Maintainers review code and provide feedback
3. **Testing**: Comprehensive testing in CI environment
4. **Approval**: Requires approval from maintainers
5. **Merge**: Squash merge to main branch

### After Merge

- **Clean Up**: Delete feature branch
- **Update Local**: Pull latest changes to local main
- **Release**: Changes included in next release

## 🐛 Reporting Issues

### Bug Reports

Include the following information:

- **Environment**: Go version, OS, Redis version
- **Steps to Reproduce**: Detailed steps to reproduce the issue
- **Expected Behavior**: What should happen
- **Actual Behavior**: What actually happens
- **Logs**: Relevant log output or error messages
- **Configuration**: Relevant configuration settings

### Feature Requests

Include the following information:

- **Use Case**: Describe the problem this feature would solve
- **Proposed Solution**: Your ideas for implementation
- **Alternatives**: Other solutions you've considered
- **Additional Context**: Any other relevant information

## 💬 Getting Help

- **GitHub Discussions**: Ask questions and discuss ideas
- **GitHub Issues**: Report bugs and request features
- **Documentation**: Check existing documentation first
- **Code Examples**: Look at existing code for patterns

## 🏆 Recognition

Contributors are recognized in:

- **CHANGELOG.md**: Major contributions listed in releases
- **README.md**: Contributors section
- **GitHub**: Contribution graphs and statistics

Thank you for contributing to OCPP Server! 🚀