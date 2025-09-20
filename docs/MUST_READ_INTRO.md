# OCPP Server - MUST READ INTRO

## What is OCPP Server?

The OCPP Server is a distributed, scalable implementation of the Open Charge Point Protocol (OCPP) built in Go. It serves as the central system (CSMS) that manages electric vehicle charging stations, handling real-time communication, transaction processing, and charge point configuration.

## Core Architecture

```
Redis Queue System
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Charge Point  │◄──►│   WS-Server     │◄──►│   OCPP Server   │
│   (WebSocket)   │    │   (Proxy)       │    │   (Processor)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                       │
                                ▼                       ▼
                       ┌─────────────────────────────────────────┐
                       │              Redis                      │
                       │  ocpp:requests    ocpp:responses:CP-ID  │
                       └─────────────────────────────────────────┘
```

**Key Design Principles:**
- **Stateless Processing**: Any OCPP Server instance can handle any message
- **Redis-based Queuing**: Decouples WebSocket connections from OCPP processing
- **Charge Point ID Routing**: Natural OCPP identifier for message routing
- **Horizontal Scalability**: Add more server instances without coordination

## Project Structure

```
ocpp-server/
├── main.go                     # Application entry point
├── internal/
│   ├── server/                 # Core server implementation
│   │   ├── server.go          # Main server struct and lifecycle
│   │   └── setup.go           # OCPP handlers registration
│   ├── api/v1/                # HTTP REST API
│   │   ├── handlers/          # HTTP request handlers
│   │   ├── models/            # Request/response models
│   │   └── routes.go          # Route definitions
│   ├── ocpp/                  # OCPP protocol handlers
│   ├── correlation/           # Message correlation management
│   ├── types/                 # Shared type definitions
│   └── services/              # Business logic services
├── config/                     # Configuration management
├── tests/                      # Test suite
└── docs/                       # Documentation
```

## Key Components

### 1. OCPP Protocol Handler (`internal/ocpp/`)
- Processes OCPP 1.6 and 2.0.1 messages
- Handles charge point lifecycle (Boot, Heartbeat, StatusNotification)
- Manages transactions (StartTransaction, StopTransaction, MeterValues)
- Configuration management (GetConfiguration, ChangeConfiguration)

### 2. HTTP API (`internal/api/v1/`)
- RESTful endpoints for external systems
- Charge point management and status queries
- Command sending to charge points
- Health monitoring and diagnostics

### 3. State Management
- **Server State**: WebSocket connections, session management
- **Business State**: Charge point status, transactions, configurations
- **Redis Backend**: Distributed state with TTL support
- **Correlation Manager**: Tracks request/response pairs

### 4. Configuration System (`config/`)
- Dynamic configuration management
- Per-charge-point settings
- Standard OCPP configuration keys
- Runtime configuration updates

## Message Flow

### Incoming OCPP Message (Charge Point → CSMS)
1. Charge Point sends WebSocket message to WS-Server
2. WS-Server publishes to `ocpp:requests` Redis queue
3. OCPP Server consumes message from queue
4. Extracts Charge Point ID from message
5. Processes message using ocpp-go library
6. Publishes response to `ocpp:responses:{charge_point_id}`
7. WS-Server receives response and sends to Charge Point

### Outgoing OCPP Command (CSMS → Charge Point)
1. HTTP API receives command request
2. Creates OCPP message with correlation ID
3. Publishes to `ocpp:responses:{charge_point_id}` queue
4. WS-Server forwards to appropriate WebSocket connection
5. Response flows back through normal message flow

## Key Technologies

- **Go 1.16+**: Main programming language
- **ocpp-go**: Core OCPP protocol library
- **Redis**: Message queuing and state storage
- **Gorilla Mux**: HTTP routing
- **WebSocket**: Real-time communication
- **Docker**: Containerization

## Environment Configuration

```bash
# Redis Configuration
REDIS_ADDR=localhost:6379          # Redis server address
REDIS_PASSWORD=                    # Redis password (optional)
REDIS_DISTRIBUTED_STATE=true      # Enable distributed state
REDIS_STATE_PREFIX=ocpp           # Redis key prefix
REDIS_STATE_TTL=30s               # State expiration time

# Server Configuration
HTTP_PORT=8081                    # HTTP API port
```

## Development Workflow

### Local Development
```bash
# Start Redis
redis-server

# Run server
go run main.go

# In another terminal, test
curl http://localhost:8081/health
```

### Docker Development
```bash
# Full stack with Redis, WS-Server, OCPP Server, and test clients
docker-compose up -d

# Check logs
docker-compose logs ocpp-server

# Health check
curl http://localhost:8083/health
```

### Testing
```bash
# Unit tests
go test ./...

# Integration tests
go test ./tests/integration/...

# With coverage
go test -cover ./...
```

## API Endpoints

### Core Endpoints
- `GET /health` - Server health status
- `GET /api/v1/chargepoints` - List all charge points
- `GET /api/v1/chargepoints/{id}/status` - Charge point status
- `POST /api/v1/chargepoints/{id}/commands` - Send commands

### Management Endpoints
- `GET /api/v1/chargepoints/{id}/configuration` - Get configuration
- `POST /api/v1/chargepoints/{id}/configuration` - Update configuration
- `GET /api/v1/chargepoints/{id}/transactions` - Transaction history

## Scalability Features

### Horizontal Scaling
- Deploy multiple OCPP Server instances
- Shared Redis state eliminates coordination overhead
- Load balancer can distribute HTTP API requests
- Each instance processes messages from shared queue

### State Management
- Redis clustering for high availability
- State TTL prevents memory leaks
- Separate business and server state domains
- Backup and restore capabilities

### Performance Optimizations
- Connection pooling to Redis
- Goroutine-based concurrent processing
- Efficient JSON marshaling/unmarshaling
- WebSocket connection reuse

## OCPP Protocol Support

### OCPP 1.6 (JSON)
- Core Profile: ✅ Complete
- Firmware Management: ✅ Complete
- Local Auth List: ✅ Complete
- Reservation: ✅ Complete
- Smart Charging: ✅ Complete
- Remote Trigger: ✅ Complete

### OCPP 2.0.1
- Core functionality: ✅ Complete
- Advanced features: 🚧 In development

## Monitoring & Observability

### Health Checks
- HTTP health endpoint
- Redis connectivity check
- Message queue depth monitoring
- Processing latency metrics

### Logging
- Structured logging with levels
- Request/response correlation IDs
- Charge point activity tracking
- Error and performance logging

### Metrics (Planned)
- Prometheus metrics endpoint
- Message throughput counters
- Response time histograms
- Error rate tracking

## Common Use Cases

### 1. Charge Point Management
```bash
# Get charge point status
curl http://localhost:8081/api/v1/chargepoints/CP-001/status

# Send remote start transaction
curl -X POST http://localhost:8081/api/v1/chargepoints/CP-001/commands \
  -d '{"command": "RemoteStartTransaction", "connectorId": 1, "idTag": "USER123"}'
```

### 2. Configuration Management
```bash
# Get configuration
curl http://localhost:8081/api/v1/chargepoints/CP-001/configuration

# Change configuration
curl -X POST http://localhost:8081/api/v1/chargepoints/CP-001/configuration \
  -d '{"key": "HeartbeatInterval", "value": "300"}'
```

### 3. Transaction Monitoring
```bash
# Get active transactions
curl http://localhost:8081/api/v1/chargepoints/CP-001/transactions?status=active

# Transaction history
curl http://localhost:8081/api/v1/chargepoints/CP-001/transactions?limit=10
```

## Troubleshooting

### Common Issues

1. **Connection Failed**
   - Check Redis connectivity
   - Verify network configuration
   - Check Docker network settings

2. **Messages Not Processing**
   - Monitor Redis queue depth: `redis-cli llen ocpp:requests`
   - Check server logs for processing errors
   - Verify charge point ID format

3. **State Not Persisting**
   - Check Redis TTL settings
   - Verify REDIS_DISTRIBUTED_STATE=true
   - Monitor Redis memory usage

### Debug Tools
```bash
# Monitor Redis queues
redis-cli monitor

# Check queue depths
redis-cli llen ocpp:requests
redis-cli llen ocpp:responses:CP-001

# View server logs
docker-compose logs -f ocpp-server
```

## Next Steps

1. **Read the Architecture**: `docs/ARCHITECTURE.md`
2. **API Documentation**: `docs/api/` directory
3. **Development Guide**: `ocpp-server/README.md`
4. **Integration Tests**: `tests/integration/`
5. **Docker Compose Setup**: `docker-compose.yml`

## Key Files to Understand

- `main.go` - Application bootstrap and configuration
- `internal/server/server.go` - Core server implementation
- `internal/server/setup.go` - OCPP handler registration
- `internal/api/v1/routes.go` - HTTP API routing
- `config/manager.go` - Configuration management
- `docker-compose.yml` - Full system deployment

---

**Quick Start**: Run `docker-compose up -d` and visit `http://localhost:8083/health`

**Architecture**: Stateless, horizontally scalable OCPP processing with Redis queuing

**Purpose**: Bridge between charge points (WebSocket/OCPP-J) and backend systems (HTTP/REST)