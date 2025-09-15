# OCPP Server Implementation Plan - Agent Execution

## Overview
This plan implements a horizontally scalable, multi-tenant OCPP server with Redis-based distributed state and MongoDB persistence. Execute tasks in sequence as they build upon each other.

## Current State
- ✅ Basic OCPP server with Redis transport working
- ✅ Docker setup with Redis, WebSocket server, client
- ✅ PRD approved with architecture decisions
- 🎯 Next: Redis-based distributed state for horizontal scaling

---

## Phase 1: Redis Distributed State (Priority: CRITICAL)

### Task 1.1: Create RedisServerState Implementation
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-go/ocppj/redis_state.go`

**Implementation Requirements:**
```go
package ocppj

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/lorenzodonini/ocpp-go/ocpp"
)

type RedisServerState struct {
	client     *redis.Client
	keyPrefix  string
	defaultTTL time.Duration
	mu         sync.RWMutex
}

// Implement all ServerState interface methods:
// - AddPendingRequest(clientID, requestID string, req ocpp.Request)
// - GetClientState(clientID string) ClientState
// - HasPendingRequest(clientID, requestID string) bool
// - HasClientPendingRequest(clientID string) bool
// - DeletePendingRequest(clientID, requestID string) bool
// - ClearClientPendingRequest(clientID string)
// - GetPendingRequest(clientID, requestID string) (ocpp.Request, bool)
```

**Redis Key Patterns:**
- `{keyPrefix}:pending:{clientID}` - Hash storing pending requests per client
- Key: `requestID`, Value: `JSON serialized request`
- TTL: `defaultTTL` (default 30 seconds)

**Constructor:**
```go
func NewRedisServerState(client *redis.Client, keyPrefix string, ttl time.Duration) ServerState {
	if ttl == 0 {
		ttl = 30 * time.Second
	}
	return &RedisServerState{
		client:     client,
		keyPrefix:  keyPrefix,
		defaultTTL: ttl,
	}
}
```

### Task 1.2: Create RedisClientState Implementation
**File**: Same as above

**Requirements:**
```go
type RedisClientState struct {
	clientID   string
	redis      *redis.Client
	keyPrefix  string
	defaultTTL time.Duration
}

// Implement ClientState interface:
// - HasPendingRequest(requestID string) bool
// - GetPendingRequest(requestID string) (ocpp.Request, bool)
// - AddPendingRequest(requestID string, req ocpp.Request)
// - DeletePendingRequest(requestID string) bool
// - ClearPendingRequests()
```

### Task 1.3: Update Redis Transport Factory
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-go/transport/redis/factory.go`

**Add to RedisConfig:**
```go
type RedisConfig struct {
	// ... existing fields
	UseDistributedState bool          `env:"REDIS_DISTRIBUTED_STATE" envDefault:"false"`
	StateKeyPrefix     string        `env:"REDIS_STATE_PREFIX" envDefault:"ocpp"`
	StateTTL           time.Duration `env:"REDIS_STATE_TTL" envDefault:"30s"`
}
```

**Update factory method to optionally create RedisServerState and pass to server creation.**

### Task 1.4: Update Server Creation
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/main.go`

**Add distributed state option:**
```go
func createOCPPServer(redisTransport transport.Transport, config *transport.RedisConfig) *ocppj.Server {
	var serverState ocppj.ServerState

	if config.UseDistributedState {
		// Create Redis client for state management
		redisClient := redis.NewClient(&redis.Options{
			Addr:     config.Addr,
			Password: config.Password,
			DB:       config.DB + 1, // Use different DB for state
		})
		serverState = ocppj.NewRedisServerState(redisClient, config.StateKeyPrefix, config.StateTTL)
	} else {
		serverState = ocppj.NewMemoryServerState()
	}

	return ocppj.NewServerWithTransport(redisTransport, nil, serverState, core.Profile)
}
```

### Task 1.5: Environment Configuration
**File**: `/Users/chrishome/development/home/mcp-access/csms/docker-compose.yml`

**Add environment variables:**
```yaml
ocpp-server:
  environment:
    - REDIS_DISTRIBUTED_STATE=true
    - REDIS_STATE_PREFIX=ocpp
    - REDIS_STATE_TTL=30s
```

### Task 1.6: Test Multi-Instance Scaling
**Add to docker-compose.yml:**
```yaml
ocpp-server-2:
  build:
    context: .
    dockerfile: ocpp-server/Dockerfile
  container_name: ocpp-server-2
  networks:
    - ocpp-network
  environment:
    - REDIS_ADDR=redis:6379
    - REDIS_DISTRIBUTED_STATE=true
    - HTTP_PORT=8082
  depends_on:
    redis:
      condition: service_healthy
```

**Test Commands:**
```bash
# Start both servers
docker-compose up ocpp-server ocpp-server-2

# Test that both can handle requests from same client
curl http://localhost:8083/clients
curl http://localhost:8084/clients
```

---

## Phase 2: Multi-Tenant Foundation

### Task 2.1: Add Dependencies
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/go.mod`

**Add:**
```go
require (
    github.com/golang-jwt/jwt/v4 v4.5.0
    go.mongodb.org/mongo-driver v1.12.1
    github.com/eclipse/paho.mqtt.golang v1.4.3
)
```

### Task 2.2: Create Tenant Model
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/models/tenant.go`

```go
package models

import (
    "time"
    "go.mongodb.org/mongo-driver/bson/primitive"
)

type Tenant struct {
    ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    Name         string            `bson:"name" json:"name"`
    DatabaseName string            `bson:"database_name" json:"database_name"`
    Status       string            `bson:"status" json:"status"` // active, suspended
    Configuration TenantConfig    `bson:"configuration" json:"configuration"`
    CreatedAt    time.Time        `bson:"created_at" json:"created_at"`
    UpdatedAt    time.Time        `bson:"updated_at" json:"updated_at"`
}

type TenantConfig struct {
    MaxChargePoints   int           `bson:"max_charge_points" json:"max_charge_points"`
    APIRateLimit     int           `bson:"api_rate_limit" json:"api_rate_limit"`
    QueueEnabled     bool          `bson:"queue_enabled" json:"queue_enabled"`
    MQTTTopicPrefix  string        `bson:"mqtt_topic_prefix" json:"mqtt_topic_prefix"`
}
```

### Task 2.3: Create Session Models
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/models/session.go`

```go
package models

import (
    "time"
    "go.mongodb.org/mongo-driver/bson/primitive"
)

type ChargePoint struct {
    ID            string                 `bson:"_id" json:"id"`
    TenantID      string                `bson:"tenant_id" json:"tenant_id"`
    Model         string                `bson:"model" json:"model"`
    Vendor        string                `bson:"vendor" json:"vendor"`
    SerialNumber  string                `bson:"serial_number" json:"serial_number"`
    Connectors    []Connector           `bson:"connectors" json:"connectors"`
    Configuration map[string]interface{} `bson:"configuration" json:"configuration"`
    LastSeen      time.Time             `bson:"last_seen" json:"last_seen"`
    Status        string                `bson:"status" json:"status"`
    CreatedAt     time.Time             `bson:"created_at" json:"created_at"`
}

type Connector struct {
    ID        int    `bson:"id" json:"id"`
    Type      string `bson:"type" json:"type"`
    MaxPower  int    `bson:"max_power" json:"max_power"`
    Status    string `bson:"status" json:"status"`
}

type Session struct {
    ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    TenantID         string            `bson:"tenant_id" json:"tenant_id"`
    ChargePointID    string            `bson:"charge_point_id" json:"charge_point_id"`
    ConnectorID      int               `bson:"connector_id" json:"connector_id"`
    TransactionID    int               `bson:"transaction_id" json:"transaction_id"`
    IDTag            string            `bson:"id_tag" json:"id_tag"`
    StartTime        time.Time         `bson:"start_time" json:"start_time"`
    StopTime         *time.Time        `bson:"stop_time,omitempty" json:"stop_time,omitempty"`
    StartMeterValue  int               `bson:"start_meter_value" json:"start_meter_value"`
    StopMeterValue   *int              `bson:"stop_meter_value,omitempty" json:"stop_meter_value,omitempty"`
    MeterValues      []MeterReading    `bson:"meter_values" json:"meter_values"`
    Status           string            `bson:"status" json:"status"` // active, completed, faulted
    StopReason       string            `bson:"stop_reason,omitempty" json:"stop_reason,omitempty"`
}

type MeterReading struct {
    Timestamp time.Time `bson:"timestamp" json:"timestamp"`
    Value     int       `bson:"value" json:"value"`
    Unit      string    `bson:"unit" json:"unit"`
    Context   string    `bson:"context" json:"context"`
}
```

### Task 2.4: Create Database Manager
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/database/manager.go`

```go
package database

import (
    "context"
    "fmt"
    "sync"
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

type Manager struct {
    client      *mongo.Client
    databases   map[string]*mongo.Database
    mu          sync.RWMutex
    globalDB    *mongo.Database
}

func NewManager(uri string) (*Manager, error) {
    client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
    if err != nil {
        return nil, err
    }

    return &Manager{
        client:    client,
        databases: make(map[string]*mongo.Database),
        globalDB:  client.Database("ocpp_global"),
    }, nil
}

func (m *Manager) GetTenantDB(tenantID string) *mongo.Database {
    m.mu.RLock()
    db, exists := m.databases[tenantID]
    m.mu.RUnlock()

    if exists {
        return db
    }

    m.mu.Lock()
    defer m.mu.Unlock()

    // Double-check after acquiring write lock
    if db, exists := m.databases[tenantID]; exists {
        return db
    }

    dbName := fmt.Sprintf("ocpp_tenant_%s", tenantID)
    db = m.client.Database(dbName)
    m.databases[tenantID] = db

    return db
}

func (m *Manager) GetGlobalDB() *mongo.Database {
    return m.globalDB
}
```

### Task 2.5: Create JWT Middleware
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/middleware/auth.go`

```go
package middleware

import (
    "context"
    "net/http"
    "strings"
    "github.com/golang-jwt/jwt/v4"
)

type TenantClaims struct {
    TenantID string `json:"tenant_id"`
    Role     string `json:"role"`
    jwt.RegisteredClaims
}

type contextKey string

const TenantContextKey contextKey = "tenant"

func JWTMiddleware(secretKey string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authHeader := r.Header.Get("Authorization")
            if authHeader == "" {
                http.Error(w, "Authorization header required", http.StatusUnauthorized)
                return
            }

            tokenString := strings.TrimPrefix(authHeader, "Bearer ")
            token, err := jwt.ParseWithClaims(tokenString, &TenantClaims{}, func(token *jwt.Token) (interface{}, error) {
                return []byte(secretKey), nil
            })

            if err != nil || !token.Valid {
                http.Error(w, "Invalid token", http.StatusUnauthorized)
                return
            }

            claims := token.Claims.(*TenantClaims)
            ctx := context.WithValue(r.Context(), TenantContextKey, claims)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}

func GetTenantFromContext(ctx context.Context) (*TenantClaims, bool) {
    claims, ok := ctx.Value(TenantContextKey).(*TenantClaims)
    return claims, ok
}
```

### Task 2.6: Create MQTT Publisher
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/events/publisher.go`

```go
package events

import (
    "encoding/json"
    "fmt"
    "time"
    mqtt "github.com/eclipse/paho.mqtt.golang"
)

type Publisher struct {
    client mqtt.Client
}

type SessionEvent struct {
    EventType     string    `json:"event_type"`
    Timestamp     time.Time `json:"timestamp"`
    SessionID     string    `json:"session_id"`
    ChargePointID string    `json:"charge_point_id"`
    TenantID      string    `json:"tenant_id"`
    Data          interface{} `json:"data"`
}

func NewPublisher(brokerURL string) (*Publisher, error) {
    opts := mqtt.NewClientOptions()
    opts.AddBroker(brokerURL)
    opts.SetClientID("ocpp-server")

    client := mqtt.NewClient(opts)
    if token := client.Connect(); token.Wait() && token.Error() != nil {
        return nil, token.Error()
    }

    return &Publisher{client: client}, nil
}

func (p *Publisher) PublishSessionEvent(event SessionEvent) error {
    topic := fmt.Sprintf("ocpp/events/%s/%s/session", event.TenantID, event.ChargePointID)

    data, err := json.Marshal(event)
    if err != nil {
        return err
    }

    token := p.client.Publish(topic, 1, false, data)
    token.Wait()
    return token.Error()
}

func (p *Publisher) PublishStatusUpdate(tenantID, chargePointID string, status interface{}) error {
    topic := fmt.Sprintf("ocpp/status/%s/%s", tenantID, chargePointID)

    data, err := json.Marshal(status)
    if err != nil {
        return err
    }

    token := p.client.Publish(topic, 1, false, data)
    token.Wait()
    return token.Error()
}
```

### Task 2.7: Update OCPP Handlers with Multi-tenancy
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/handlers/ocpp.go`

```go
package handlers

import (
    "context"
    "log"
    "strings"
    "time"

    "github.com/lorenzodonini/ocpp-go/ocpp"
    "github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
    "github.com/lorenzodonini/ocpp-go/ocpp1.6/types"

    "ocpp-server/database"
    "ocpp-server/events"
    "ocpp-server/models"
)

type OCPPHandler struct {
    dbManager *database.Manager
    publisher *events.Publisher
}

func NewOCPPHandler(dbManager *database.Manager, publisher *events.Publisher) *OCPPHandler {
    return &OCPPHandler{
        dbManager: dbManager,
        publisher: publisher,
    }
}

func (h *OCPPHandler) extractTenantID(clientID string) string {
    // Extract tenant from client ID (e.g., "tenant1_CP001" -> "tenant1")
    parts := strings.Split(clientID, "_")
    if len(parts) >= 2 {
        return parts[0]
    }
    return "default"
}

func (h *OCPPHandler) HandleBootNotification(clientID string, request *core.BootNotificationRequest, requestID string) *core.BootNotificationConfirmation {
    tenantID := h.extractTenantID(clientID)

    // Create/update charge point in tenant database
    chargePoint := models.ChargePoint{
        ID:           clientID,
        TenantID:     tenantID,
        Model:        request.ChargePointModel,
        Vendor:       request.ChargePointVendor,
        SerialNumber: request.ChargePointSerialNumber,
        Status:       "Available",
        LastSeen:     time.Now(),
        CreatedAt:    time.Now(),
    }

    // Save to tenant-specific database
    db := h.dbManager.GetTenantDB(tenantID)
    collection := db.Collection("charge_points")

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    // Upsert charge point
    // Implementation here...

    // Publish status update
    h.publisher.PublishStatusUpdate(tenantID, clientID, map[string]interface{}{
        "charge_point_id": clientID,
        "status":          "Available",
        "boot_time":       time.Now(),
    })

    log.Printf("BootNotification from %s (tenant: %s): Model=%s, Vendor=%s",
        clientID, tenantID, request.ChargePointModel, request.ChargePointVendor)

    currentTime := types.NewDateTime(time.Now())
    return core.NewBootNotificationConfirmation(currentTime, 300, core.RegistrationStatusAccepted)
}

// Similar implementations for other OCPP message handlers...
```

### Task 2.8: Update Main Server
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/main.go`

**Major restructure to include:**
- Database manager initialization
- MQTT publisher setup
- JWT middleware
- Tenant-aware OCPP handlers
- Updated REST API with tenant scoping

### Task 2.9: Docker Compose Updates
**File**: `/Users/chrishome/development/home/mcp-access/csms/docker-compose.yml`

**Add services:**
```yaml
mongodb:
  image: mongo:6
  container_name: ocpp-mongodb
  networks:
    - ocpp-network
  ports:
    - "27017:27017"
  volumes:
    - mongodb_data:/data/db

mqtt-broker:
  image: eclipse-mosquitto:2
  container_name: ocpp-mqtt
  networks:
    - ocpp-network
  ports:
    - "1883:1883"
    - "9001:9001"

volumes:
  mongodb_data:
```

### Task 2.10: Environment Variables
**Update ocpp-server environment:**
```yaml
environment:
  - REDIS_ADDR=redis:6379
  - REDIS_DISTRIBUTED_STATE=true
  - MONGODB_URI=mongodb://mongodb:27017
  - MQTT_BROKER_URL=tcp://mqtt-broker:1883
  - JWT_SECRET=your-secret-key
  - HTTP_PORT=8081
```

---

## Phase 3: Testing & Validation

### Task 3.1: Create Test Tenants
**Script**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/scripts/create-test-data.go`

Create test tenants and JWT tokens for validation.

### Task 3.2: Multi-Instance Testing
**Commands:**
```bash
# Start full stack
docker-compose up --build

# Test horizontal scaling
docker-compose up --scale ocpp-server=3

# Test tenant isolation
curl -H "Authorization: Bearer <tenant1-token>" http://localhost:8083/api/v1/chargepoints
curl -H "Authorization: Bearer <tenant2-token>" http://localhost:8083/api/v1/chargepoints
```

### Task 3.3: MQTT Validation
Test event publishing and ensure external services can subscribe to tenant-specific topics.

---

## Execution Notes

### Prerequisites
- Current working directory: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server`
- Go 1.16+ installed
- Docker and docker-compose available
- Existing Redis and ocpp-go setup working

### Execution Order
Execute tasks in numerical order. Each task builds on the previous ones.

### Testing Strategy
After each major task (1.x, 2.x), validate functionality before proceeding.

### Error Handling
If any task fails, address issues before proceeding to dependent tasks.

### Success Criteria
- Multiple OCPP server instances can run simultaneously
- Tenants are properly isolated in separate databases
- MQTT events are published for all session activities
- REST API enforces tenant boundaries
- Horizontal scaling works without session affinity

---

## Expected Timeline
- **Phase 1** (Redis Distributed State): 4-6 hours
- **Phase 2** (Multi-tenant Foundation): 8-12 hours
- **Phase 3** (Testing & Validation): 2-4 hours

**Total**: 1-2 full development days for a production-ready, horizontally scalable, multi-tenant OCPP server.