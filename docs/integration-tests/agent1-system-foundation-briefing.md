# Agent 1: System Foundation Tests - Briefing Document

## 🎯 Mission Overview
You are responsible for implementing **Tests 1-3** which validate the fundamental system infrastructure, Docker deployment health, and configuration management. These tests ensure the foundation is solid before testing OCPP functionality.

## 📋 Your Test Assignments

### **Test 1: Docker Stack Health Check**
**File**: `tests/integration/01_docker_health_test.go`

**Purpose**: Verify all containers start and are accessible after deployment

**Test Steps**:
1. Start Docker Compose stack (`../docker-compose up -d`)
2. Wait for all containers to be in "running" state
3. Verify HTTP health endpoint returns 200 OK
4. Verify Redis connectivity with PING/PONG
5. Check all expected containers are running
6. Validate no error logs in any container

**Expected Results**:
```json
GET /health → 200 OK
{
  "status": "healthy",
  "service": "enhanced-ocpp-server",
  "timestamp": "2024-01-15T10:30:00Z",
  "ocpp_lib": "github.com/lorenzodonini/ocpp-go"
}
```

### **Test 2: Service Dependencies & Initialization**
**File**: `tests/integration/02_service_dependencies_test.go`

**Purpose**: Verify proper startup sequence and dependency management

**Test Steps**:
1. Start Redis container first
2. Start OCPP server container
3. Verify Redis connection established within 5 seconds
4. Test StateStore operations (save/retrieve simple data)
5. Verify HTTP server binds to correct port
6. Test graceful shutdown sequence

**Validation Points**:
- Redis connection successful
- StateStore CRUD operations work
- HTTP server accessible on configured port
- Clean shutdown without hanging processes

### **Test 3: Environment Configuration Validation**
**File**: `tests/integration/03_config_validation_test.go`

**Purpose**: Verify environment variable handling and configuration

**Test Scenarios**:
1. **Default Configuration**: Start with no ENV vars set
2. **Custom Configuration**: Override all ENV vars
3. **Partial Configuration**: Mix default and custom values
4. **Invalid Configuration**: Test with invalid values

**Environment Variables to Test**:
- `REDIS_URL` (default: "redis://localhost:6379")
- `QUEUE_NAME` (default: "ocpp:requests")
- `CSMS_URL` (default: "http://localhost:3000")
- `HTTP_PORT` (default: "8081")

## 🛠 Technical Implementation Guide

### **Required Dependencies**
```go
import (
    "testing"
    "time"
    "context"
    "net/http"
    "encoding/json"
    "github.com/redis/go-redis/v9"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/compose"
)
```

### **Test Infrastructure Setup**
```go
// Use testcontainers for Docker Compose management
func setupDockerEnvironment(t *testing.T) *compose.ComposeStack {
    compose := testcontainers.NewLocalDockerCompose(
        []string{"docker-compose.yml"},
        "test-stack",
    )

    // Start stack and wait for readiness
    err := compose.WithCommand([]string{"up", "-d"}).Invoke()
    require.NoError(t, err)

    return compose
}
```

### **Health Check Implementation**
```go
func testHealthEndpoint(t *testing.T, baseURL string) {
    client := &http.Client{Timeout: 10 * time.Second}

    resp, err := client.Get(baseURL + "/health")
    require.NoError(t, err)
    defer resp.Body.Close()

    assert.Equal(t, http.StatusOK, resp.StatusCode)

    var health map[string]string
    err = json.NewDecoder(resp.Body).Decode(&health)
    require.NoError(t, err)

    assert.Equal(t, "healthy", health["status"])
    assert.Equal(t, "enhanced-ocpp-server", health["service"])
}
```

### **Redis Connectivity Test**
```go
func testRedisConnectivity(t *testing.T, redisURL string) {
    opt, err := redis.ParseURL(redisURL)
    require.NoError(t, err)

    client := redis.NewClient(opt)
    defer client.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    pong, err := client.Ping(ctx).Result()
    require.NoError(t, err)
    assert.Equal(t, "PONG", pong)
}
```

## 📁 File Structure to Create
```
tests/
├── integration/
│   ├── 01_docker_health_test.go
│   ├── 02_service_dependencies_test.go
│   ├── 03_config_validation_test.go
│   └── common/
│       ├── docker_utils.go
│       ├── http_utils.go
│       └── redis_utils.go
├── fixtures/
│   └── docker-compose.test.yml
└── Makefile (test runner)
```

## ✅ Implementation Checklist

### **Pre-Development**
- [ ] Review current Docker setup and docker-compose.yml
- [ ] Understand environment variable usage in cmd/main.go
- [ ] Set up Go testing dependencies (testcontainers, etc.)
- [ ] Create test directory structure

### **Test 1: Docker Health Check**
- [ ] Implement Docker Compose startup/teardown
- [ ] Create health endpoint test
- [ ] Add Redis connectivity validation
- [ ] Test container status verification
- [ ] Add comprehensive error logging check
- [ ] Test with both fresh start and restart scenarios

### **Test 2: Service Dependencies**
- [ ] Implement startup sequence testing
- [ ] Create Redis dependency validation
- [ ] Test StateStore operations
- [ ] Verify HTTP server binding
- [ ] Test graceful shutdown
- [ ] Add timeout handling for all operations

### **Test 3: Configuration Validation**
- [ ] Test default configuration scenario
- [ ] Test custom configuration override
- [ ] Test partial configuration mix
- [ ] Test invalid configuration handling
- [ ] Verify configuration actually applied (not just set)
- [ ] Test configuration reload scenarios

### **Test Infrastructure**
- [ ] Create reusable Docker utilities
- [ ] Implement common HTTP testing functions
- [ ] Create Redis testing helpers
- [ ] Add test fixtures and mock data
- [ ] Implement proper test cleanup
- [ ] Add test timeout and retry logic

### **Quality Assurance**
- [ ] All tests pass consistently (5+ runs)
- [ ] No resource leaks (containers, connections)
- [ ] Tests complete within 2 minutes total
- [ ] Proper error messages for failures
- [ ] Tests can run in parallel safely
- [ ] Add comprehensive test documentation

## 🔍 Validation Criteria

### **Success Metrics**
- All 3 tests pass consistently
- Docker stack starts reliably every time
- No hanging containers after test completion
- Redis connectivity verified under all conditions
- Configuration changes properly applied

### **Performance Requirements**
- Docker stack startup: < 30 seconds
- Health check response: < 1 second
- Redis connection: < 5 seconds
- Total test suite: < 2 minutes

### **Error Handling**
- Clear error messages for failed dependencies
- Proper cleanup on test failures
- Retry logic for transient failures
- Comprehensive logging of failure causes

## 🚀 Success Deliverables

1. **Working Test Suite**: 3 integration tests that pass consistently
2. **Test Infrastructure**: Reusable utilities for other agents
3. **Documentation**: Clear test descriptions and expected behaviors
4. **CI/CD Ready**: Tests suitable for automated deployment validation

## 🤝 Coordination Notes

- Your tests run **first** - other agents depend on your infrastructure validation
- Share your Docker and Redis utilities with other agents
- Coordinate on test data cleanup between test suites
- Ensure your tests don't interfere with tests from other agents

Your foundation tests are critical - they ensure the system is ready for OCPP protocol testing by other agents.