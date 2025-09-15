# OCPP Server

A high-performance OCPP (Open Charge Point Protocol) server implementation built in Go, featuring Redis-based distributed state management and comprehensive HTTP API support.

## 🚀 Features

- **OCPP 1.6 & 2.0.1 Support**: Complete implementation of OCPP protocols
- **Distributed State Management**: Redis-backed state storage for scalability
- **WebSocket Communication**: Real-time bidirectional communication with charge points
- **HTTP API**: RESTful API for external system integration
- **Docker Support**: Containerized deployment with Docker Compose
- **Health Monitoring**: Built-in health checks and monitoring endpoints
- **Business Logic State**: Separate business and server state management
- **Load Balancing Ready**: Stateless design with external state storage

## 📋 Table of Contents

- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [API Documentation](#api-documentation)
- [Development](#development)
- [Testing](#testing)
- [Deployment](#deployment)
- [Architecture](#architecture)

## 🚀 Quick Start

### Prerequisites

- Go 1.16 or higher
- Redis server
- Docker & Docker Compose (optional)

### Local Development

```bash
# Clone the repository
git clone <repository-url>
cd ocpp-server

# Install dependencies
go mod tidy

# Start Redis (if not running)
redis-server

# Run the server
go run main.go
```

### Docker Deployment

```bash
# Start the entire stack
docker-compose up -d

# Check health
curl http://localhost:8083/health
```

## ⚙️ Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_ADDR` | `localhost:6379` | Redis server address |
| `REDIS_PASSWORD` | _(empty)_ | Redis password |
| `REDIS_DISTRIBUTED_STATE` | `true` | Enable distributed state |
| `REDIS_STATE_PREFIX` | `ocpp` | Redis key prefix |
| `REDIS_STATE_TTL` | `30s` | State TTL duration |
| `HTTP_PORT` | `8081` | HTTP API port |

### Example Configuration

```bash
export REDIS_ADDR="redis:6379"
export REDIS_DISTRIBUTED_STATE="true"
export REDIS_STATE_PREFIX="ocpp"
export HTTP_PORT="8081"
```

## 📖 API Documentation

### Health Check

```http
GET /health
```

**Response:**
```json
{
  "success": true,
  "message": "OCPP Server is running",
  "data": {
    "timestamp": "2024-01-15T10:30:00Z"
  }
}
```

### Charge Point Management

```http
# Get charge point status
GET /api/v1/chargepoints/{id}/status

# Send command to charge point
POST /api/v1/chargepoints/{id}/commands
```

See [API Documentation](docs/api.md) for complete endpoint reference.

## 🛠 Development

### Project Structure

```
ocpp-server/
├── cmd/
│   └── main.go              # Application entry point
├── internal/
│   ├── server/              # Server implementation
│   ├── api/                 # HTTP API handlers
│   ├── ocpp/                # OCPP protocol handlers
│   └── state/               # State management
├── handlers/                # HTTP request handlers
├── models/                  # Data models
├── config/                  # Configuration management
├── tests/                   # Test suite
├── scripts/                 # Build and deployment scripts
├── docs/                    # Documentation
├── Dockerfile              # Container definition
└── docker-compose.yml      # Multi-service deployment
```

### Building

```bash
# Build binary
go build -o bin/ocpp-server main.go

# Build Docker image
docker build -t ocpp-server .

# Run tests
go test ./...

# Run with race detection
go run -race main.go
```

### Code Style

This project follows standard Go conventions:

- `gofmt` for formatting
- `go vet` for static analysis
- `golint` for style checks

```bash
# Format code
go fmt ./...

# Run linters
go vet ./...
golint ./...
```

## 🧪 Testing

### Unit Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test ./internal/server -v
```

### Integration Tests

```bash
# Run integration tests
go test ./tests/integration/... -v

# Test with Docker
docker-compose -f docker-compose.test.yml up --abort-on-container-exit
```

### Load Testing

```bash
# Run load tests
go test ./tests/load/... -v

# Custom load test
./scripts/load-test.sh --connections=100 --duration=60s
```

## 🚢 Deployment

### Docker Compose

```yaml
version: '3.8'
services:
  ocpp-server:
    build: .
    ports:
      - "8081:8081"
    environment:
      - REDIS_ADDR=redis:6379
    depends_on:
      - redis

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
```

### Kubernetes

```bash
# Apply Kubernetes manifests
kubectl apply -f k8s/

# Check deployment
kubectl get pods -l app=ocpp-server
```

### Production Considerations

- **Scaling**: Multiple server instances with shared Redis state
- **Security**: TLS/SSL termination at load balancer
- **Monitoring**: Prometheus metrics and health checks
- **Logging**: Structured logging with log aggregation
- **Backup**: Redis persistence and backup strategies

## 🏗 Architecture

### System Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Charge Point  │◄──►│   OCPP Server   │◄──►│   CSMS/Backend  │
│                 │    │                 │    │                 │
│  (WebSocket)    │    │  (HTTP API)     │    │   (REST API)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │
                                ▼
                       ┌─────────────────┐
                       │     Redis       │
                       │  (State Store)  │
                       └─────────────────┘
```

### State Management

- **Server State**: Connection management, session data
- **Business State**: Charge point status, transaction data
- **Distributed**: Redis-backed for horizontal scaling
- **TTL Support**: Automatic cleanup of expired data

### Protocol Support

- **OCPP 1.6**: Full JSON support
- **OCPP 2.0.1**: Complete implementation
- **WebSocket**: Bidirectional communication
- **HTTP**: RESTful API for external integration

## 📚 Documentation

- [API Reference](docs/api.md)
- [Configuration Guide](docs/configuration.md)
- [Deployment Guide](docs/deployment.md)
- [Developer Guide](docs/development.md)
- [Protocol Reference](docs/ocpp-protocol.md)

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Write tests for new features
- Follow Go best practices
- Update documentation
- Ensure CI/CD passes

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🐛 Issues & Support

- **Bug Reports**: [GitHub Issues](../../issues)
- **Feature Requests**: [GitHub Discussions](../../discussions)
- **Documentation**: [Wiki](../../wiki)

## 🏆 Acknowledgments

- [OCPP Protocol](https://www.openchargealliance.org/) - Open Charge Alliance
- [lorenzodonini/ocpp-go](https://github.com/lorenzodonini/ocpp-go) - Core OCPP library
- [Redis](https://redis.io/) - State management backend

---

**Version**: 1.0.0
**Status**: Production Ready ✅
**Maintainer**: CSMS Development Team