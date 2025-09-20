# OCPP-Server - OCPP Protocol Message Processor

**Type**: microservice
**Language**: Go
**Purpose**: Core OCPP protocol processor that handles messages from Redis queues and provides HTTP API
**Dependencies**: Redis, lorenzodonini/ocpp-go, gorilla/mux, go-redis/redis
**Status**: active

## Overview

OCPP-Server processes OCPP messages from Redis transport queues using a distributed architecture. The server handles incoming OCPP protocol messages (BootNotification, StatusNotification, StartTransaction, etc.) through Redis-backed transport, maintains charge point state in distributed Redis storage, and exposes a comprehensive REST API for external system integration. Built using the lorenzodonini/ocpp-go library with custom Redis transport layer.

The server operates as a stateless message processor with all state externalized to Redis, enabling horizontal scaling and resilience. Business logic separates concerns between protocol handling, state management, and API services. The architecture supports OCPP 1.6 Core Profile with configuration management, transaction processing, message triggering, and real-time status tracking.

Processing flow: Redis transport receives WebSocket messages → OCPP server processes protocol messages → Business state updated in Redis → HTTP API serves external requests. All charge point connections, transactions, and configuration managed through Redis for distributed consistency.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   WS-Server     │───►│   Redis Queues  │◄───│  OCPP-Server    │
│   (WebSocket)   │    │   (Transport)   │    │  (Processor)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                │                       │
                                │                       ▼
                                ▼              ┌─────────────────┐
                       ┌─────────────────┐     │   HTTP API      │
                       │  Redis State    │     │   (REST)        │
                       │  (Business)     │     └─────────────────┘
                       └─────────────────┘
```

### Internal Package Structure

```
internal/
├── server/                 # Core server implementation
│   ├── server.go          # Main server struct and lifecycle (lines 26-160)
│   └── setup.go           # OCPP handlers and HTTP API setup (lines 16-147)
├── api/v1/                # HTTP API layer
│   ├── handlers/          # Domain-specific request handlers
│   │   ├── chargepoints.go    # Charge point operations
│   │   ├── transactions.go    # Transaction management
│   │   ├── configuration.go   # Configuration API
│   │   └── health.go          # System health checks
│   ├── models/            # API request/response models
│   │   ├── requests.go        # HTTP request structures
│   │   ├── responses.go       # HTTP response structures
│   │   └── common.go          # Shared API models
│   └── routes.go          # Route definitions (lines 10-60)
├── services/              # Business logic layer
│   ├── chargepoint.go     # Charge point business operations
│   ├── transaction.go     # Transaction business logic
│   └── configuration.go   # Configuration management services
├── ocpp/                  # OCPP protocol handlers
│   ├── handlers.go        # Core OCPP message handlers (lines 14-173)
│   └── response_handlers.go  # Response correlation handlers
├── correlation/           # Request correlation management
│   └── manager.go         # Pending request correlation (lines 24-181)
├── handlers/              # Message processing handlers
│   └── meter_value_aggregator.go  # Meter value processing
└── types/                 # Internal data structures
    └── types.go           # Common type definitions (lines 3-8)
```

### External Dependencies

- **config/manager.go**: OCPP configuration management with Redis persistence (lines 19-481)
- **models/**: Business data models for charge points and transactions

## Message Flow

### Incoming OCPP Message Processing

1. **Message Reception**: Redis transport receives OCPP message from WebSocket proxy
2. **Protocol Dispatch**: `internal/server/setup.go:18` - Transport request handler routes by message type
3. **Message Handling**: Specific handlers in `internal/ocpp/handlers.go` process messages:
   - BootNotification → `HandleBootNotification:15`
   - StatusNotification → Transaction handler via callback
   - StartTransaction → Transaction handler with ID generation
   - Configuration requests → ConfigurationManager interaction

### Outgoing Request Correlation

1. **Request Initiation**: HTTP API generates request ID and registers with correlation manager
2. **Pending Registration**: `internal/correlation/manager.go:38` - AddPendingRequest creates response channel
3. **OCPP Transmission**: Request sent via OCPP server transport to charge point
4. **Response Correlation**: `internal/server/setup.go:71` - Transport response handler matches response to pending request
5. **Channel Delivery**: Response delivered through correlation manager channel to waiting HTTP handler

### State Management Flow

- **Server State**: Connection lifecycle, session management via ocppj.ServerState
- **Business State**: Charge point info, transaction data via ocppj.RedisBusinessState
- **Configuration State**: OCPP configuration keys via ConfigurationManager with Redis persistence
- **Correlation State**: Pending HTTP API requests via correlation.Manager in memory

## Configuration

### Environment Variables

| Variable | Default | Description | Usage |
|----------|---------|-------------|--------|
| `REDIS_ADDR` | `localhost:6379` | Redis server address | `main.go:24` |
| `REDIS_PASSWORD` | _(empty)_ | Redis authentication password | `main.go:25` |
| `HTTP_PORT` | `8081` | HTTP API server port | `main.go:26` |

### Redis Configuration Structure

```go
// main.go:29-37
&transport.RedisConfig{
    Addr:                redisAddr,
    Password:            redisPassword,
    DB:                  0,
    ChannelPrefix:       "ocpp",
    UseDistributedState: true,
    StateKeyPrefix:      "ocpp",
    StateTTL:            30 * time.Second,
}
```

### OCPP Configuration Keys

**Core Configuration** (managed by `config/manager.go:49-254`):
- `HeartbeatInterval`: 300s default, validator ensures non-negative integer
- `ConnectionTimeOut`: 60s default, reboot required for changes
- `MeterValueSampleInterval`: 60s default, range 0-3600s
- `MeterValuesSampledData`: "Energy.Active.Import.Register,Power.Active.Import"

**Smart Charging** (read-only keys):
- `ChargeProfileMaxStackLevel`: 10, range 1-100
- `ChargingScheduleAllowedChargingRateUnit`: "Current,Power"
- `MaxChargingProfilesInstalled`: 10, range 1-100

## API Reference

### OCPP Protocol APIs

### Core Endpoints

**Health Check**
```http
GET /health
```
Returns server status with timestamp

**Connected Clients**
```http
GET /clients
```
Lists active charge point connections from Redis transport

### Charge Point Management

**List Charge Points**
```http
GET /api/v1/chargepoints
```
Retrieves all charge points from business state

**Get Charge Point Details**
```http
GET /api/v1/chargepoints/{clientID}
```
Returns specific charge point information including connector status

**Connector Information**
```http
GET /api/v1/chargepoints/{clientID}/connectors
GET /api/v1/chargepoints/{clientID}/connectors/{connectorID}
```

### Transaction Control

**Remote Start Transaction**
```http
POST /api/v1/transactions/remote-start
Content-Type: application/json

{
  "clientId": "CP001",
  "connectorId": 1,
  "idTag": "USER123"
}
```

Generates correlation ID, sends RemoteStartTransaction OCPP request, waits for confirmation via correlation manager.

**Remote Stop Transaction**
```http
POST /api/v1/transactions/remote-stop
Content-Type: application/json

{
  "clientId": "CP001",
  "transactionId": 123
}
```

### Message Triggering

```http
POST /api/v1/chargepoints/{clientID}/trigger
```

Request specific messages from charge points on demand. Useful for immediate status updates, diagnostics, and testing connectivity.

**Request Body:**
```json
{
  "requestedMessage": "StatusNotification",
  "connectorId": 1
}
```

**Supported Message Types:**
- `StatusNotification`: Current connector or charge point status
- `Heartbeat`: Immediate connectivity test
- `MeterValues`: Current meter readings
- `BootNotification`: Charge point information
- `DiagnosticsStatusNotification`: Diagnostics status
- `FirmwareStatusNotification`: Firmware update status

**Examples:**
```bash
# Request status for all connectors
curl -X POST http://localhost:8081/api/v1/chargepoints/CP001/trigger \
  -H "Content-Type: application/json" \
  -d '{"requestedMessage": "StatusNotification"}'

# Request status for specific connector
curl -X POST http://localhost:8081/api/v1/chargepoints/CP001/trigger \
  -H "Content-Type: application/json" \
  -d '{"requestedMessage": "StatusNotification", "connectorId": 1}'

# Test connectivity
curl -X POST http://localhost:8081/api/v1/chargepoints/CP001/trigger \
  -H "Content-Type: application/json" \
  -d '{"requestedMessage": "Heartbeat"}'
```

### Configuration Management

**Live Configuration (Real-time OCPP)**
```http
GET /api/v1/chargepoints/{clientID}/configuration/live
PUT /api/v1/chargepoints/{clientID}/configuration/live
```

Uses correlation manager to send GetConfiguration/ChangeConfiguration OCPP requests, timeout: 10s

**Stored Configuration (Redis-backed)**
```http
GET /api/v1/chargepoints/{clientID}/configuration
PUT /api/v1/chargepoints/{clientID}/configuration
```

Directly manipulates configuration via ConfigurationManager without charge point communication

### Error Handling

Standard HTTP status codes with JSON response format:
```json
{
  "success": boolean,
  "message": string,
  "data": object
}
```

- 200: Success
- 400: Bad Request (validation errors)
- 404: Not Found (charge point/transaction)
- 408: Request Timeout (correlation timeout)
- 503: Service Unavailable (charge point offline)

## Redis Integration

### Transport Layer

**Queue Patterns**:
- Incoming messages: `ocpp:requests:{clientID}`
- Outgoing responses: `ocpp:responses:{clientID}`
- Client connections: `ocpp:clients`

**State Storage**:
- Business state prefix: `ocpp:business`
- Server state prefix: `ocpp:server`
- Configuration prefix: `ocpp:config:{clientID}`

### State Operations

**Charge Point State** (`internal/services/chargepoint.go`):
- `GetAllChargePoints()`: businessState.GetAllChargePoints()
- `GetChargePoint(clientID)`: businessState.GetChargePointInfo(clientID)
- Connection status via redisTransport.GetConnectedClients()

**Transaction State** (`internal/services/transaction.go`):
- StartTransaction: Atomic operation via businessState.StartTransaction()
- StopTransaction: businessState.StopTransaction() with meter reading
- Transaction lookup: businessState.GetTransaction(transactionID)

**Configuration Persistence** (`config/manager.go:383-404`):
- Load: businessState.GetChargePointConfiguration(clientID)
- Save: businessState.SetChargePointConfiguration(clientID, config)
- Validation: Built-in validators for each configuration key

### User Management & Authentication APIs

**For comprehensive user, contract, and token management, the CSMS includes a separate API service:**

- **Service**: CSMS API Service
- **Port**: 59800
- **Base URL**: `http://localhost:59800/api/v1`
- **Documentation**: [CSMS API Service README](../csms-api-service/README.md)
- **API Reference**: [API Reference Guide](../csms-api-service/API_REFERENCE.md)

**Key Features:**
- **User Management**: Complete CRUD operations for user accounts
- **Contract Management**: OCPI-compliant contract management with family plan support
- **Token Management**: Multi-modal authentication (RFID, App, Vehicle autocharge)
- **Family Plans**: Shared contracts with role-based access (Owner, Member, Viewer)
- **OCPI Compliance**: Full OCPI 2.2.1 token and contract standard support
- **Advanced Filtering**: Comprehensive search, filtering, and pagination
- **Token Grouping**: Support for linked authentication methods

**Quick Examples:**
```bash
# List all users
curl "http://localhost:59800/api/v1/users"

# Create a family contract
curl -X POST http://localhost:59800/api/v1/contracts \
  -H "Content-Type: application/json" \
  -d '{"contract_id": "FAMILY-001", "name": "Smith Family Plan", ...}'

# Add RFID token
curl -X POST http://localhost:59800/api/v1/tokens \
  -H "Content-Type: application/json" \
  -d '{"uid": "rfid001", "type": "RFID", "contract_id": "...", ...}'
```

## OCPP Protocol Support

### Supported Messages (OCPP 1.6 Core Profile)

**Incoming Messages** (handled in `internal/server/setup.go:18-68`):
- BootNotification: Charge point registration and info update
- Heartbeat: Keep-alive with last seen timestamp
- StatusNotification: Connector status updates via transaction handler
- StartTransaction: Transaction initiation with ID generation
- StopTransaction: Transaction completion with meter reading
- MeterValues: Meter data processing via transaction handler
- GetConfiguration: Configuration retrieval via ConfigurationManager
- ChangeConfiguration: Configuration modification with validation

**Outgoing Requests** (correlation-based):
- RemoteStartTransaction: Initiated via HTTP API with correlation
- RemoteStopTransaction: Transaction control via HTTP API
- TriggerMessage: Request specific messages from charge points on demand
- GetConfiguration: Live configuration queries
- ChangeConfiguration: Live configuration changes

### Message Processing Pipeline

1. **Transport Reception**: Redis transport delivers message to OCPP server
2. **Type Dispatch**: `server/setup.go:21` switches on request type
3. **Handler Execution**: Domain-specific handler processes message
4. **State Update**: Business state modified in Redis
5. **Response Generation**: OCPP response created and sent
6. **Error Handling**: Validation errors sent as OCPP error responses

### Transaction Management

**Transaction Lifecycle**:
1. StartTransaction request → Generate transaction ID (server.go:114-121)
2. Update connector status and transaction state atomically
3. MeterValues during transaction → Processed by MeterValueProcessor
4. StopTransaction → Update final meter reading and close transaction

**Transaction ID Generation**: Sequential counter starting at 1000, incremented per transaction

## Development

### Building

```bash
# Build binary
go build -o ocpp-server main.go

# Build with debugging
go build -race -o ocpp-server main.go

# Run tests
go test ./...
```

### Project Structure

```
ocpp-server/
├── main.go                    # Entry point with Redis setup
├── config/                    # OCPP configuration management
│   └── manager.go            # ConfigurationManager implementation
├── internal/                  # Internal packages
│   ├── server/               # Core server implementation
│   ├── api/v1/              # HTTP API layer
│   ├── services/            # Business logic services
│   ├── ocpp/                # OCPP protocol handlers
│   ├── correlation/         # Request correlation
│   ├── handlers/            # Message processors
│   └── types/               # Data structures
├── models/                  # Business data models
├── tests/                   # Test suite
├── docs/                    # Documentation
└── scripts/                 # Build/deployment scripts
```

### Key Files

- `main.go`: Application bootstrap with Redis factory setup
- `internal/server/server.go`: Server struct with component lifecycle
- `internal/server/setup.go`: OCPP and HTTP handler registration
- `internal/api/v1/routes.go`: HTTP route definitions
- `config/manager.go`: OCPP configuration with Redis persistence
- `internal/correlation/manager.go`: Request correlation for HTTP API

### Testing

```bash
# Unit tests
go test ./internal/...

# Integration tests
go test ./tests/integration/...

# With coverage
go test -cover ./...
```

## Troubleshooting

### Common Issues

**Redis Connection Failures**:
- Check REDIS_ADDR environment variable
- Verify Redis server accessibility
- Check Redis authentication if REDIS_PASSWORD set

**OCPP Message Processing Errors**:
- Review server logs for handler errors in `internal/ocpp/handlers.go`
- Check business state Redis connectivity
- Validate OCPP message format against protocol specification

**HTTP API Timeout Issues**:
- Live configuration endpoints timeout after 10s (correlation/manager.go:13)
- Check charge point connectivity via `/clients` endpoint
- Use stored configuration endpoints for offline charge points

**Configuration Validation Failures**:
- Check `config/manager.go` validators for specific configuration keys
- Review supported value ranges and types
- Verify readonly status of configuration keys

### Debugging

Enable debug logging:
```bash
export DEBUG=1
go run main.go
```

Monitor Redis state:
```bash
redis-cli monitor
redis-cli keys "ocpp:*"
```

Check component health:
```bash
curl http://localhost:8081/health
curl http://localhost:8081/clients
```