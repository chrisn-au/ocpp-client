# Redis Distributed State Implementation - Agent Task

## Objective
Replace in-memory ServerState with Redis-backed implementation to enable horizontal scaling of OCPP servers.

## Current State
- ✅ Basic OCPP server working with Redis transport
- ✅ Located at: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/`
- ✅ Uses local ocpp-go library at: `/Users/chrishome/development/home/mcp-access/csms/ocpp-go/`

## Scope: ONLY Redis State Implementation
**What to implement**: Redis-backed ServerState and ClientState
**What NOT to implement**: Multi-tenancy, MongoDB, MQTT, JWT - these are separate tasks

---

## Task 1: Create RedisServerState Implementation

### File: `/Users/chrishome/development/home/mcp-access/csms/ocpp-go/ocppj/redis_state.go`

**Create new file with:**

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

// RedisServerState implements ServerState interface using Redis for distributed state
type RedisServerState struct {
	client     *redis.Client
	keyPrefix  string
	defaultTTL time.Duration
	mu         sync.RWMutex
}

// NewRedisServerState creates a new Redis-backed ServerState
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

// AddPendingRequest stores a pending request in Redis
func (r *RedisServerState) AddPendingRequest(clientID, requestID string, req ocpp.Request) {
	key := fmt.Sprintf("%s:pending:%s", r.keyPrefix, clientID)

	data, err := json.Marshal(req)
	if err != nil {
		return // Log error in production
	}

	ctx := context.Background()
	r.client.HSet(ctx, key, requestID, data)
	r.client.Expire(ctx, key, r.defaultTTL)
}

// GetClientState returns a Redis-backed ClientState for the given client
func (r *RedisServerState) GetClientState(clientID string) ClientState {
	return &RedisClientState{
		clientID:   clientID,
		redis:      r.client,
		keyPrefix:  r.keyPrefix,
		defaultTTL: r.defaultTTL,
	}
}

// HasPendingRequest checks if a specific request is pending for a client
func (r *RedisServerState) HasPendingRequest(clientID, requestID string) bool {
	key := fmt.Sprintf("%s:pending:%s", r.keyPrefix, clientID)
	ctx := context.Background()

	exists, err := r.client.HExists(ctx, key, requestID).Result()
	if err != nil {
		return false
	}
	return exists
}

// HasClientPendingRequest checks if a client has any pending requests
func (r *RedisServerState) HasClientPendingRequest(clientID string) bool {
	key := fmt.Sprintf("%s:pending:%s", r.keyPrefix, clientID)
	ctx := context.Background()

	count, err := r.client.HLen(ctx, key).Result()
	if err != nil {
		return false
	}
	return count > 0
}

// DeletePendingRequest removes a specific pending request
func (r *RedisServerState) DeletePendingRequest(clientID, requestID string) bool {
	key := fmt.Sprintf("%s:pending:%s", r.keyPrefix, clientID)
	ctx := context.Background()

	deleted, err := r.client.HDel(ctx, key, requestID).Result()
	if err != nil {
		return false
	}
	return deleted > 0
}

// ClearClientPendingRequest removes all pending requests for a client
func (r *RedisServerState) ClearClientPendingRequest(clientID string) {
	key := fmt.Sprintf("%s:pending:%s", r.keyPrefix, clientID)
	ctx := context.Background()

	r.client.Del(ctx, key)
}

// GetPendingRequest retrieves a specific pending request
func (r *RedisServerState) GetPendingRequest(clientID, requestID string) (ocpp.Request, bool) {
	key := fmt.Sprintf("%s:pending:%s", r.keyPrefix, clientID)
	ctx := context.Background()

	data, err := r.client.HGet(ctx, key, requestID).Result()
	if err != nil {
		return nil, false
	}

	// Note: This requires knowing the request type for proper unmarshaling
	// For now, return raw data - the library will handle type conversion
	var req ocpp.Request
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		return nil, false
	}

	return req, true
}

// RedisClientState implements ClientState interface using Redis
type RedisClientState struct {
	clientID   string
	redis      *redis.Client
	keyPrefix  string
	defaultTTL time.Duration
}

// HasPendingRequest checks if a specific request is pending
func (r *RedisClientState) HasPendingRequest(requestID string) bool {
	key := fmt.Sprintf("%s:pending:%s", r.keyPrefix, r.clientID)
	ctx := context.Background()

	exists, err := r.redis.HExists(ctx, key, requestID).Result()
	if err != nil {
		return false
	}
	return exists
}

// GetPendingRequest retrieves a specific pending request
func (r *RedisClientState) GetPendingRequest(requestID string) (ocpp.Request, bool) {
	key := fmt.Sprintf("%s:pending:%s", r.keyPrefix, r.clientID)
	ctx := context.Background()

	data, err := r.redis.HGet(ctx, key, requestID).Result()
	if err != nil {
		return nil, false
	}

	var req ocpp.Request
	if err := json.Unmarshal([]byte(data), &req); err != nil {
		return nil, false
	}

	return req, true
}

// AddPendingRequest stores a pending request
func (r *RedisClientState) AddPendingRequest(requestID string, req ocpp.Request) {
	key := fmt.Sprintf("%s:pending:%s", r.keyPrefix, r.clientID)

	data, err := json.Marshal(req)
	if err != nil {
		return
	}

	ctx := context.Background()
	r.redis.HSet(ctx, key, requestID, data)
	r.redis.Expire(ctx, key, r.defaultTTL)
}

// DeletePendingRequest removes a specific pending request
func (r *RedisClientState) DeletePendingRequest(requestID string) bool {
	key := fmt.Sprintf("%s:pending:%s", r.keyPrefix, r.clientID)
	ctx := context.Background()

	deleted, err := r.redis.HDel(ctx, key, requestID).Result()
	if err != nil {
		return false
	}
	return deleted > 0
}

// ClearPendingRequests removes all pending requests for this client
func (r *RedisClientState) ClearPendingRequests() {
	key := fmt.Sprintf("%s:pending:%s", r.keyPrefix, r.clientID)
	ctx := context.Background()

	r.redis.Del(ctx, key)
}
```

---

## Task 2: Update Redis Transport Factory

### File: `/Users/chrishome/development/home/mcp-access/csms/ocpp-go/transport/redis/factory.go`

**Find the existing RedisConfig struct and add:**

```go
// Add these fields to existing RedisConfig
UseDistributedState bool          `env:"REDIS_DISTRIBUTED_STATE" envDefault:"false"`
StateKeyPrefix     string        `env:"REDIS_STATE_PREFIX" envDefault:"ocpp"`
StateTTL           time.Duration `env:"REDIS_STATE_TTL" envDefault:"30s"`
```

**Update the factory method to support creating servers with Redis state:**

Add method to create Redis-backed server state:
```go
func (f *RedisFactory) CreateServerState(config *RedisConfig) (ocppj.ServerState, error) {
	if !config.UseDistributedState {
		return ocppj.NewMemoryServerState(), nil
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.Addr,
		Password: config.Password,
		DB:       config.DB + 1, // Use different DB for state
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis for state: %w", err)
	}

	return ocppj.NewRedisServerState(redisClient, config.StateKeyPrefix, config.StateTTL), nil
}
```

---

## Task 3: Update Main Server

### File: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/main.go`

**Add distributed state configuration:**

1. **Update config loading** to include new Redis state options
2. **Create server with Redis state** instead of default memory state

**Replace the server creation section with:**

```go
// Create Redis transport
factory := redis.NewRedisFactory()
config := &transport.RedisConfig{
	Addr:                redisAddr,
	Password:            redisPassword,
	DB:                  0,
	ChannelPrefix:       "ocpp",
	UseDistributedState: true,  // Enable distributed state
	StateKeyPrefix:      "ocpp",
	StateTTL:            30 * time.Second,
}

redisTransport, err := factory.CreateTransport(config)
if err != nil {
	log.Fatalf("Failed to create Redis transport: %v", err)
}

// Create server state (Redis-backed if configured)
serverState, err := factory.CreateServerState(config)
if err != nil {
	log.Fatalf("Failed to create server state: %v", err)
}

// Create OCPP server with distributed state
server := ocppj.NewServerWithTransport(redisTransport, nil, serverState, core.Profile)
```

---

## Task 4: Update Docker Configuration

### File: `/Users/chrishome/development/home/mcp-access/csms/docker-compose.yml`

**Add environment variables to ocpp-server service:**

```yaml
ocpp-server:
  build:
    context: .
    dockerfile: ocpp-server/Dockerfile
  container_name: ocpp-server
  networks:
    - ocpp-network
  ports:
    - "8083:8081"
  environment:
    - REDIS_ADDR=redis:6379
    - REDIS_PASSWORD=
    - REDIS_DISTRIBUTED_STATE=true
    - REDIS_STATE_PREFIX=ocpp
    - REDIS_STATE_TTL=30s
    - HTTP_PORT=8081
  depends_on:
    redis:
      condition: service_healthy
  restart: unless-stopped
```

---

## Task 5: Test Multi-Instance Scaling

### Add second server instance to docker-compose.yml:

```yaml
ocpp-server-2:
  build:
    context: .
    dockerfile: ocpp-server/Dockerfile
  container_name: ocpp-server-2
  networks:
    - ocpp-network
  ports:
    - "8084:8081"
  environment:
    - REDIS_ADDR=redis:6379
    - REDIS_PASSWORD=
    - REDIS_DISTRIBUTED_STATE=true
    - REDIS_STATE_PREFIX=ocpp
    - REDIS_STATE_TTL=30s
    - HTTP_PORT=8081
  depends_on:
    redis:
      condition: service_healthy
  restart: unless-stopped
```

---

## Testing Commands

```bash
# Build and start both servers
docker-compose up --build ocpp-server ocpp-server-2

# Test that both servers share state
curl http://localhost:8083/clients
curl http://localhost:8084/clients

# Check Redis for distributed state
docker exec -it ocpp-redis redis-cli
> KEYS ocpp:pending:*
> HGETALL ocpp:pending:TEST-CP-001
```

## Success Criteria

✅ Both server instances can handle requests from the same client
✅ Pending requests are visible in Redis
✅ State persists across server restarts
✅ No session affinity required for load balancing

## Notes

- **Dependencies**: Ensure go-redis/redis/v8 is available in go.mod
- **Error Handling**: Basic error handling included, enhance for production
- **Testing**: Use existing client to verify functionality
- **Redis Keys**: Uses pattern `ocpp:pending:{clientID}` for state storage

**Total Estimated Time: 3-4 hours**