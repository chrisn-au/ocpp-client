# OCPP-Go Enhancement: Docker-First Implementation Guide

## Overview

**Goal**: Build and test entirely in Docker with non-breaking phases that show end-to-end flow as early as Phase 2.

**Strategy**:
- Phase 1: Docker setup + basic Redis transport (minimal viable flow)
- Phase 2: End-to-end message flow working (even without business logic)
- Phase 3: Add business logic and state management
- Phase 4: Production deployment

## Prerequisites

- Docker and Docker Compose installed
- Your existing `ws-server` (already complete)
- Redis running (via Docker)

## Phase 1: Docker Setup + Minimal Transport (2 hours)

### Step 1.1: Create Docker Workspace
```bash
# Create workspace
mkdir -p ~/workspace/ocpp-enhanced
cd ~/workspace/ocpp-enhanced

# Clone OCPP-Go
git clone https://github.com/lorenzodonini/ocpp-go.git
cd ocpp-go

# Create feature branch
git checkout -b feature/redis-enhancement
```

### Step 1.2: Create Minimal Redis Transport
Create `cmd/simple-processor/main.go`:
```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
)

type MessageEnvelope struct {
	ChargePointID string      `json:"charge_point_id"`
	MessageType   int         `json:"message_type"`
	MessageID     string      `json:"message_id"`
	Action        string      `json:"action,omitempty"`
	Payload       interface{} `json:"payload"`
	Timestamp     string      `json:"timestamp"`
}

func main() {
	log.Println("🚀 Starting Simple OCPP Processor (Phase 1)")

	// Redis connection
	redisURL := getEnv("REDIS_URL", "redis://redis:6379")
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("❌ Failed to parse Redis URL: %v", err)
	}

	client := redis.NewClient(opt)
	ctx := context.Background()

	// Test Redis connection
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("❌ Failed to connect to Redis: %v", err)
	}
	log.Println("✅ Connected to Redis")

	// Start message consumer
	go consumeMessages(ctx, client)

	log.Println("🔄 Simple processor started - listening for messages")
	log.Println("📝 This will show all message flow from your ws-server")

	// Wait for shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("🛑 Shutting down...")
	client.Close()
}

func consumeMessages(ctx context.Context, client *redis.Client) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Listen for messages from your ws-server
			result, err := client.BRPop(ctx, 1*time.Second, "ocpp:requests").Result()
			if err != nil {
				if err == redis.Nil {
					continue // Timeout, keep trying
				}
				log.Printf("❌ Redis error: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}

			if len(result) != 2 {
				log.Printf("⚠️  Unexpected Redis result: %v", result)
				continue
			}

			// Parse message from your ws-server
			var envelope MessageEnvelope
			if err := json.Unmarshal([]byte(result[1]), &envelope); err != nil {
				log.Printf("❌ Failed to parse message: %v", err)
				continue
			}

			log.Printf("📨 Received: %s from %s (Type: %d, Action: %s)",
				envelope.MessageID, envelope.ChargePointID, envelope.MessageType, envelope.Action)

			// Handle the message
			handleMessage(ctx, client, &envelope)
		}
	}
}

func handleMessage(ctx context.Context, client *redis.Client, envelope *MessageEnvelope) {
	switch envelope.MessageType {
	case 2: // CALL - request from charge point
		log.Printf("🔵 Processing CALL: %s", envelope.Action)
		handleIncomingRequest(ctx, client, envelope)
	case 3: // CALL_RESULT - response to our request
		log.Printf("🟢 Processing CALL_RESULT for: %s", envelope.MessageID)
		handleIncomingResponse(ctx, client, envelope)
	case 4: // CALL_ERROR
		log.Printf("🔴 Processing CALL_ERROR for: %s", envelope.MessageID)
		handleIncomingError(ctx, client, envelope)
	default:
		log.Printf("⚠️  Unknown message type: %d", envelope.MessageType)
	}
}

func handleIncomingRequest(ctx context.Context, client *redis.Client, envelope *MessageEnvelope) {
	// For Phase 1, just echo back a simple response for any request
	var response []interface{}

	switch envelope.Action {
	case "BootNotification":
		response = []interface{}{
			3, // CALL_RESULT
			envelope.MessageID,
			map[string]interface{}{
				"status":   "Accepted",
				"currentTime": time.Now().Format(time.RFC3339),
				"interval": 300,
			},
		}
	case "Heartbeat":
		response = []interface{}{
			3, // CALL_RESULT
			envelope.MessageID,
			map[string]interface{}{
				"currentTime": time.Now().Format(time.RFC3339),
			},
		}
	default:
		// Generic response for unknown actions
		response = []interface{}{
			4, // CALL_ERROR
			envelope.MessageID,
			"NotImplemented",
			"Action not implemented in Phase 1",
			map[string]interface{}{},
		}
	}

	// Send response back through your ws-server
	sendResponse(ctx, client, envelope.ChargePointID, response)
}

func handleIncomingResponse(ctx context.Context, client *redis.Client, envelope *MessageEnvelope) {
	// For Phase 1, just log that we received a response
	log.Printf("✅ Response received for message %s - no processing yet", envelope.MessageID)
}

func handleIncomingError(ctx context.Context, client *redis.Client, envelope *MessageEnvelope) {
	log.Printf("❌ Error received for message %s", envelope.MessageID)
}

func sendResponse(ctx context.Context, client *redis.Client, chargePointId string, response []interface{}) {
	responseEnvelope := MessageEnvelope{
		ChargePointID: chargePointId,
		MessageType:   response[0].(int),
		MessageID:     response[1].(string),
		Payload:       response,
		Timestamp:     time.Now().Format(time.RFC3339),
	}

	data, err := json.Marshal(responseEnvelope)
	if err != nil {
		log.Printf("❌ Failed to marshal response: %v", err)
		return
	}

	// Send to your ws-server response queue
	responseQueue := fmt.Sprintf("ocpp:responses:%s", chargePointId)
	err = client.LPush(ctx, responseQueue, data).Err()
	if err != nil {
		log.Printf("❌ Failed to send response: %v", err)
		return
	}

	log.Printf("📤 Sent response to %s: %s", chargePointId, response[1])
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
```

### Step 1.3: Create Docker Setup
Create `Dockerfile.simple`:
```dockerfile
FROM golang:1.19-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build simple processor
RUN CGO_ENABLED=0 GOOS=linux go build -o simple-processor ./cmd/simple-processor

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/simple-processor .

CMD ["./simple-processor"]
```

Create `docker-compose.phase1.yml`:
```yaml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes

  # Your existing ws-server (unchanged)
  ws-server:
    build: ../ws-server  # Path to your existing ws-server
    ports:
      - "8080:8080"
    environment:
      - REDIS_URL=redis://redis:6379
      - LOG_LEVEL=debug
    depends_on:
      - redis

  # Simple OCPP processor (Phase 1)
  simple-processor:
    build:
      context: .
      dockerfile: Dockerfile.simple
    environment:
      - REDIS_URL=redis://redis:6379
    depends_on:
      - redis
      - ws-server
    restart: unless-stopped

  # Redis CLI for monitoring
  redis-monitor:
    image: redis:7-alpine
    depends_on:
      - redis
    command: redis-cli -h redis MONITOR
    profiles: ["monitor"]  # Optional service
```

### Step 1.4: Test Phase 1 End-to-End
```bash
# Add Go module dependencies
echo 'module github.com/lorenzodonini/ocpp-go

go 1.19

require github.com/go-redis/redis/v8 v8.11.5' > go.mod
go mod tidy

# Start Phase 1 system
docker-compose -f docker-compose.phase1.yml up --build

# In another terminal, test with a simple WebSocket client
# We'll create a test script for this
```

Create `test-phase1.sh`:
```bash
#!/bin/bash
echo "🧪 Testing Phase 1 End-to-End Flow"

# Test WebSocket connection and message flow
docker run --rm --network ocpp-go_default -it alpine/curl sh -c '
  apk add --no-cache nodejs npm
  npm install -g wscat

  echo "📡 Connecting to ws-server..."
  echo "[2, \"boot-123\", \"BootNotification\", {\"chargePointVendor\": \"TestVendor\", \"chargePointModel\": \"TestModel\"}]" | wscat -c ws://ws-server:8080/TEST-CP-PHASE1 --wait 5
'
```

**Expected Phase 1 Output:**
```
simple-processor | 🚀 Starting Simple OCPP Processor (Phase 1)
simple-processor | ✅ Connected to Redis
simple-processor | 🔄 Simple processor started - listening for messages
simple-processor | 📨 Received: boot-123 from TEST-CP-PHASE1 (Type: 2, Action: BootNotification)
simple-processor | 🔵 Processing CALL: BootNotification
simple-processor | 📤 Sent response to TEST-CP-PHASE1: boot-123
ws-server        | Sent response to TEST-CP-PHASE1
```

## Phase 2: Enhanced Message Flow + Early State (4 hours)

### Step 2.1: Add StateStore Interface
Create `internal/statestore/interface.go`:
```go
package statestore

import (
    "context"
    "time"
)

// Minimal state structures for Phase 2
type RequestContext struct {
    MessageID     string                 `json:"message_id"`
    Action        string                 `json:"action"`
    ChargePointID string                 `json:"charge_point_id"`
    Timestamp     time.Time              `json:"timestamp"`
    ExpiresAt     time.Time              `json:"expires_at"`
    Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type SimpleStateStore interface {
    SavePendingRequest(ctx context.Context, reqCtx *RequestContext) error
    GetPendingRequest(ctx context.Context, msgID string) (*RequestContext, error)
    DeletePendingRequest(ctx context.Context, msgID string) error
    HealthCheck(ctx context.Context) error
}
```

Create `internal/statestore/redis.go`:
```go
package statestore

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/go-redis/redis/v8"
)

type RedisStateStore struct {
    client *redis.Client
}

func NewRedisStateStore(client *redis.Client) *RedisStateStore {
    return &RedisStateStore{client: client}
}

func (r *RedisStateStore) SavePendingRequest(ctx context.Context, reqCtx *RequestContext) error {
    key := fmt.Sprintf("pending:%s", reqCtx.MessageID)
    data, err := json.Marshal(reqCtx)
    if err != nil {
        return fmt.Errorf("failed to marshal request context: %w", err)
    }

    ttl := time.Until(reqCtx.ExpiresAt)
    if ttl <= 0 {
        ttl = 30 * time.Second
    }

    return r.client.Set(ctx, key, data, ttl).Err()
}

func (r *RedisStateStore) GetPendingRequest(ctx context.Context, msgID string) (*RequestContext, error) {
    key := fmt.Sprintf("pending:%s", msgID)
    data, err := r.client.Get(ctx, key).Result()
    if err != nil {
        if err == redis.Nil {
            return nil, fmt.Errorf("request context %s not found", msgID)
        }
        return nil, err
    }

    var reqCtx RequestContext
    err = json.Unmarshal([]byte(data), &reqCtx)
    return &reqCtx, err
}

func (r *RedisStateStore) DeletePendingRequest(ctx context.Context, msgID string) error {
    key := fmt.Sprintf("pending:%s", msgID)
    return r.client.Del(ctx, key).Err()
}

func (r *RedisStateStore) HealthCheck(ctx context.Context) error {
    return r.client.Ping(ctx).Err()
}
```

### Step 2.2: Enhanced Processor with State
Create `cmd/enhanced-processor/main.go`:
```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "time"

    "github.com/go-redis/redis/v8"
    "github.com/lorenzodonini/ocpp-go/internal/statestore"
)

type MessageEnvelope struct {
    ChargePointID string      `json:"charge_point_id"`
    MessageType   int         `json:"message_type"`
    MessageID     string      `json:"message_id"`
    Action        string      `json:"action,omitempty"`
    Payload       interface{} `json:"payload"`
    Timestamp     string      `json:"timestamp"`
}

type EnhancedProcessor struct {
    client     *redis.Client
    stateStore statestore.SimpleStateStore
    ctx        context.Context
}

func main() {
    log.Println("🚀 Starting Enhanced OCPP Processor (Phase 2)")

    ctx := context.Background()

    // Redis connection
    redisURL := getEnv("REDIS_URL", "redis://redis:6379")
    opt, err := redis.ParseURL(redisURL)
    if err != nil {
        log.Fatalf("❌ Failed to parse Redis URL: %v", err)
    }

    client := redis.NewClient(opt)

    // Test Redis connection
    if err := client.Ping(ctx).Err(); err != nil {
        log.Fatalf("❌ Failed to connect to Redis: %v", err)
    }
    log.Println("✅ Connected to Redis")

    // Create state store
    stateStore := statestore.NewRedisStateStore(client)
    if err := stateStore.HealthCheck(ctx); err != nil {
        log.Fatalf("❌ StateStore health check failed: %v", err)
    }
    log.Println("✅ StateStore ready")

    // Create processor
    processor := &EnhancedProcessor{
        client:     client,
        stateStore: stateStore,
        ctx:        ctx,
    }

    // Start message consumer
    go processor.consumeMessages()

    log.Println("🔄 Enhanced processor started")
    log.Println("📊 Now tracking request/response correlation")
    log.Println("🎯 Ready for RemoteStartTransaction testing")

    // Wait for shutdown
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    <-sigChan

    log.Println("🛑 Shutting down...")
    client.Close()
}

func (ep *EnhancedProcessor) consumeMessages() {
    for {
        select {
        case <-ep.ctx.Done():
            return
        default:
            result, err := ep.client.BRPop(ep.ctx, 1*time.Second, "ocpp:requests").Result()
            if err != nil {
                if err == redis.Nil {
                    continue
                }
                log.Printf("❌ Redis error: %v", err)
                time.Sleep(5 * time.Second)
                continue
            }

            if len(result) != 2 {
                continue
            }

            var envelope MessageEnvelope
            if err := json.Unmarshal([]byte(result[1]), &envelope); err != nil {
                log.Printf("❌ Failed to parse message: %v", err)
                continue
            }

            ep.handleMessage(&envelope)
        }
    }
}

func (ep *EnhancedProcessor) handleMessage(envelope *MessageEnvelope) {
    switch envelope.MessageType {
    case 2: // CALL - request from charge point
        log.Printf("🔵 CALL: %s from %s", envelope.Action, envelope.ChargePointID)
        ep.handleIncomingRequest(envelope)
    case 3: // CALL_RESULT - response to our request
        log.Printf("🟢 CALL_RESULT: %s", envelope.MessageID)
        ep.handleIncomingResponse(envelope)
    case 4: // CALL_ERROR
        log.Printf("🔴 CALL_ERROR: %s", envelope.MessageID)
        ep.handleIncomingError(envelope)
    }
}

func (ep *EnhancedProcessor) handleIncomingRequest(envelope *MessageEnvelope) {
    var response []interface{}

    switch envelope.Action {
    case "BootNotification":
        response = []interface{}{
            3,
            envelope.MessageID,
            map[string]interface{}{
                "status":      "Accepted",
                "currentTime": time.Now().Format(time.RFC3339),
                "interval":    300,
            },
        }
        log.Printf("✅ BootNotification accepted for %s", envelope.ChargePointID)

    case "Heartbeat":
        response = []interface{}{
            3,
            envelope.MessageID,
            map[string]interface{}{
                "currentTime": time.Now().Format(time.RFC3339),
            },
        }

    case "StartTransaction":
        // Extract transaction details
        payload := envelope.Payload.([]interface{})
        if len(payload) >= 4 {
            if txnData, ok := payload[3].(map[string]interface{}); ok {
                log.Printf("🔄 StartTransaction: Connector %v, IdTag: %v",
                    txnData["connectorId"], txnData["idTag"])
            }
        }

        // Generate transaction ID
        txnId := int(time.Now().UnixNano() / 1000000) % 1000000

        response = []interface{}{
            3,
            envelope.MessageID,
            map[string]interface{}{
                "transactionId": txnId,
                "idTagInfo": map[string]interface{}{
                    "status": "Accepted",
                },
            },
        }
        log.Printf("✅ Transaction %d started for %s", txnId, envelope.ChargePointID)

    default:
        response = []interface{}{
            4, // CALL_ERROR
            envelope.MessageID,
            "NotImplemented",
            fmt.Sprintf("Action %s not implemented yet", envelope.Action),
            map[string]interface{}{},
        }
        log.Printf("⚠️  Unhandled action: %s", envelope.Action)
    }

    ep.sendResponse(envelope.ChargePointID, response)
}

func (ep *EnhancedProcessor) handleIncomingResponse(envelope *MessageEnvelope) {
    // Get original request context
    reqCtx, err := ep.stateStore.GetPendingRequest(ep.ctx, envelope.MessageID)
    if err != nil {
        log.Printf("⚠️  No pending request for %s: %v", envelope.MessageID, err)
        return
    }

    log.Printf("🔗 Correlated response: %s -> %s (Action: %s)",
        envelope.MessageID, reqCtx.ChargePointID, reqCtx.Action)

    // Handle based on original action
    switch reqCtx.Action {
    case "RemoteStartTransaction":
        ep.handleRemoteStartResponse(reqCtx, envelope)
    case "RemoteStopTransaction":
        ep.handleRemoteStopResponse(reqCtx, envelope)
    default:
        log.Printf("✅ Response handled for %s", reqCtx.Action)
    }

    // Cleanup
    ep.stateStore.DeletePendingRequest(ep.ctx, envelope.MessageID)
}

func (ep *EnhancedProcessor) handleRemoteStartResponse(reqCtx *statestore.RequestContext, envelope *MessageEnvelope) {
    payload := envelope.Payload.([]interface{})
    if len(payload) >= 3 {
        if responseData, ok := payload[2].(map[string]interface{}); ok {
            status := responseData["status"]
            log.Printf("🎯 RemoteStartTransaction Response: %s -> %v", reqCtx.ChargePointID, status)

            // This is where business logic would go
            if status == "Accepted" {
                log.Printf("🟢 Remote start ACCEPTED for charge point %s", reqCtx.ChargePointID)
                // TODO: Update user session, send notifications, etc.
            } else {
                log.Printf("🔴 Remote start REJECTED for charge point %s", reqCtx.ChargePointID)
                // TODO: Handle rejection, notify user, etc.
            }
        }
    }
}

func (ep *EnhancedProcessor) handleRemoteStopResponse(reqCtx *statestore.RequestContext, envelope *MessageEnvelope) {
    log.Printf("🛑 RemoteStopTransaction response for %s", reqCtx.ChargePointID)
    // Similar handling to RemoteStart
}

func (ep *EnhancedProcessor) handleIncomingError(envelope *MessageEnvelope) {
    if reqCtx, err := ep.stateStore.GetPendingRequest(ep.ctx, envelope.MessageID); err == nil {
        log.Printf("❌ Error for %s action: %s", reqCtx.Action, reqCtx.ChargePointID)
        ep.stateStore.DeletePendingRequest(ep.ctx, envelope.MessageID)
    }
}

// RemoteStartTransaction - simulate sending outbound request
func (ep *EnhancedProcessor) SendRemoteStartTransaction(chargePointId, idTag string) error {
    messageId := fmt.Sprintf("remote-start-%d", time.Now().UnixNano())

    // Store request context
    reqCtx := &statestore.RequestContext{
        MessageID:     messageId,
        Action:        "RemoteStartTransaction",
        ChargePointID: chargePointId,
        Timestamp:     time.Now(),
        ExpiresAt:     time.Now().Add(30 * time.Second),
        Metadata: map[string]interface{}{
            "idTag": idTag,
        },
    }

    if err := ep.stateStore.SavePendingRequest(ep.ctx, reqCtx); err != nil {
        return err
    }

    // Create RemoteStartTransaction request
    request := []interface{}{
        2, // CALL
        messageId,
        "RemoteStartTransaction",
        map[string]interface{}{
            "idTag": idTag,
        },
    }

    log.Printf("📤 Sending RemoteStartTransaction to %s", chargePointId)
    return ep.sendResponse(chargePointId, request)
}

func (ep *EnhancedProcessor) sendResponse(chargePointId string, response []interface{}) error {
    responseEnvelope := MessageEnvelope{
        ChargePointID: chargePointId,
        MessageType:   response[0].(int),
        MessageID:     response[1].(string),
        Payload:       response,
        Timestamp:     time.Now().Format(time.RFC3339),
    }

    data, err := json.Marshal(responseEnvelope)
    if err != nil {
        return err
    }

    responseQueue := fmt.Sprintf("ocpp:responses:%s", chargePointId)
    return ep.client.LPush(ep.ctx, responseQueue, data).Err()
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}
```

### Step 2.3: Docker Setup for Phase 2
Create `Dockerfile.enhanced`:
```dockerfile
FROM golang:1.19-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build enhanced processor
RUN CGO_ENABLED=0 GOOS=linux go build -o enhanced-processor ./cmd/enhanced-processor

FROM alpine:latest
RUN apk --no-cache add ca-certificates curl
WORKDIR /root/

COPY --from=builder /app/enhanced-processor .

# Health check endpoint (simple HTTP server)
EXPOSE 8081

CMD ["./enhanced-processor"]
```

Create `docker-compose.phase2.yml`:
```yaml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    command: redis-server --appendonly yes

  ws-server:
    build: ../ws-server
    ports:
      - "8080:8080"
    environment:
      - REDIS_URL=redis://redis:6379
      - LOG_LEVEL=debug
    depends_on:
      - redis

  enhanced-processor:
    build:
      context: .
      dockerfile: Dockerfile.enhanced
    environment:
      - REDIS_URL=redis://redis:6379
    depends_on:
      - redis
      - ws-server
    restart: unless-stopped

  # Test runner for automated testing
  test-runner:
    build:
      context: .
      dockerfile: Dockerfile.test
    environment:
      - WS_SERVER_URL=ws://ws-server:8080
      - REDIS_URL=redis://redis:6379
    depends_on:
      - ws-server
      - enhanced-processor
    profiles: ["test"]
```

### Step 2.4: Create Test Container
Create `Dockerfile.test`:
```dockerfile
FROM node:18-alpine

# Install wscat and testing tools
RUN npm install -g wscat

# Copy test scripts
COPY test/ /tests/
WORKDIR /tests

CMD ["./run-phase2-tests.sh"]
```

Create `test/run-phase2-tests.sh`:
```bash
#!/bin/sh
echo "🧪 Running Phase 2 End-to-End Tests"

# Test 1: BootNotification
echo "Test 1: BootNotification Flow"
echo '[2, "boot-test", "BootNotification", {"chargePointVendor": "TestVendor", "chargePointModel": "TestModel"}]' | \
wscat -c ws://ws-server:8080/TEST-CP-PHASE2 --wait 2

sleep 2

# Test 2: StartTransaction
echo "Test 2: StartTransaction Flow"
echo '[2, "start-test", "StartTransaction", {"connectorId": 1, "idTag": "RFID123", "meterStart": 0, "timestamp": "2024-01-01T12:00:00Z"}]' | \
wscat -c ws://ws-server:8080/TEST-CP-PHASE2 --wait 2

sleep 2

# Test 3: Heartbeat
echo "Test 3: Heartbeat Flow"
echo '[2, "heartbeat-test", "Heartbeat", {}]' | \
wscat -c ws://ws-server:8080/TEST-CP-PHASE2 --wait 2

echo "✅ Phase 2 tests completed"
```

### Step 2.5: Test Phase 2
```bash
# Update go.mod for new dependencies
go mod tidy

# Start Phase 2 system
docker-compose -f docker-compose.phase2.yml up --build

# In another terminal, run tests
docker-compose -f docker-compose.phase2.yml run --rm test-runner

# Monitor Redis to see request correlation
docker-compose -f docker-compose.phase2.yml --profile monitor up redis-monitor
```

**Expected Phase 2 Output:**
```
enhanced-processor | 🚀 Starting Enhanced OCPP Processor (Phase 2)
enhanced-processor | ✅ Connected to Redis
enhanced-processor | ✅ StateStore ready
enhanced-processor | 🔄 Enhanced processor started
enhanced-processor | 📊 Now tracking request/response correlation

# When test runs:
enhanced-processor | 🔵 CALL: BootNotification from TEST-CP-PHASE2
enhanced-processor | ✅ BootNotification accepted for TEST-CP-PHASE2
enhanced-processor | 🔵 CALL: StartTransaction from TEST-CP-PHASE2
enhanced-processor | ✅ Transaction 123456 started for TEST-CP-PHASE2
```

## Phase 3: Full Business Logic Integration (6 hours)

This phase will add:
- Complete StateStore with transactions, charge point state
- Full OCPP-Go integration
- Business logic handlers
- Response handler system

## Phase 4: Production Deployment (2 hours)

This phase will add:
- Health endpoints
- Monitoring
- Scaling configuration
- Production Docker setup

## Benefits of This Approach

✅ **Non-Breaking**: Each phase builds on the previous without breaking existing functionality
✅ **Early Visibility**: Phase 2 shows complete end-to-end flow with request correlation
✅ **Docker-First**: No local Go development needed, everything in containers
✅ **Testable**: Each phase has automated tests
✅ **Your WS-Server Unchanged**: Works with existing ws-server from day one

**Ready to start Phase 1?** This approach gives you working message flow in 2 hours, and request/response correlation in 6 hours total.