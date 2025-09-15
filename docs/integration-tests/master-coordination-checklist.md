# Integration Test Suite - Master Coordination Checklist

## 🎯 Overview
This document coordinates the 5 agents building the comprehensive integration test suite for the OCPP server. All agents must follow this checklist to ensure successful integration and avoid conflicts.

## 📋 Agent Assignments & Dependencies

### **Agent 1: System Foundation Tests (Tests 1-3)**
- **Focus**: Docker health, dependencies, configuration
- **Dependencies**: None (runs first)
- **Deliverables**: Docker test utilities, Redis connectivity tools
- **Estimated Time**: 4-6 hours

### **Agent 2: Core OCPP Flows (Tests 4-6)**
- **Focus**: Registration, transactions, authorization
- **Dependencies**: Agent 1 infrastructure
- **Deliverables**: OCPP simulator, message validators
- **Estimated Time**: 6-8 hours

### **Agent 3: OCPP Advanced Features (Tests 7-8)**
- **Focus**: Status notifications, data transfer
- **Dependencies**: Agent 2 OCPP simulator
- **Deliverables**: Advanced OCPP testing tools
- **Estimated Time**: 4-5 hours

### **Agent 4: HTTP API & Correlation (Tests 9-12)**
- **Focus**: REST API, request/response correlation
- **Dependencies**: Agent 1 infrastructure, Agent 2 OCPP simulator
- **Deliverables**: HTTP test framework, correlation tests
- **Estimated Time**: 5-7 hours

### **Agent 5: Redis & Resilience (Tests 13-18)**
- **Focus**: Redis operations, error handling, load testing
- **Dependencies**: All other agents' components
- **Deliverables**: Resilience framework, load testing tools
- **Estimated Time**: 8-10 hours

## ✅ Pre-Execution Checklist

### **Project Setup (All Agents)**
- [ ] **Read Codebase**: Review internal/ packages and understand architecture
- [ ] **Study Specifications**: Read OCPP 1.6 Core Profile specification
- [ ] **Environment Setup**: Go 1.21+, Docker, Redis client tools
- [ ] **Test Structure**: Create tests/ directory with proper structure
- [ ] **Dependencies**: Install required Go testing packages

### **Coordination Requirements**
- [ ] **Agent 1 First**: Must complete infrastructure before others begin
- [ ] **Shared Components**: Agents 2-5 reuse Agent 1's Docker utilities
- [ ] **OCPP Simulator**: Agents 3-5 reuse Agent 2's OCPP simulator
- [ ] **Test Data**: Coordinate test data cleanup between agents
- [ ] **Naming Conventions**: Use consistent test and utility naming

## 🛠 Technical Standards (All Agents)

### **Required File Structure**
```
tests/
├── integration/
│   ├── 01_docker_health_test.go              # Agent 1
│   ├── 02_service_dependencies_test.go       # Agent 1
│   ├── 03_config_validation_test.go          # Agent 1
│   ├── 04_charge_point_registration_test.go  # Agent 2
│   ├── 05_transaction_lifecycle_test.go      # Agent 2
│   ├── 06_authorization_flow_test.go         # Agent 2
│   ├── 07_status_notification_test.go        # Agent 3
│   ├── 08_data_transfer_test.go              # Agent 3
│   ├── 09_charge_point_api_test.go           # Agent 4
│   ├── 10_transaction_api_test.go            # Agent 4
│   ├── 11_synchronous_ocpp_api_test.go       # Agent 4
│   ├── 12_asynchronous_ocpp_api_test.go      # Agent 4
│   ├── 13_redis_queue_processing_test.go     # Agent 5
│   ├── 14_correlation_manager_test.go        # Agent 5
│   ├── 15_redis_state_persistence_test.go    # Agent 5
│   ├── 16_redis_connection_failures_test.go  # Agent 5
│   ├── 17_invalid_message_handling_test.go   # Agent 5
│   ├── 18_system_load_test.go                # Agent 5
│   └── common/
│       ├── docker_utils.go                   # Agent 1 (shared)
│       ├── redis_utils.go                    # Agent 1 (shared)
│       ├── ocpp_simulator.go                 # Agent 2 (shared)
│       ├── http_test_client.go               # Agent 4 (shared)
│       └── test_helpers.go                   # Shared utilities
├── fixtures/
│   ├── docker-compose.test.yml               # Agent 1
│   ├── ocpp_messages.json                    # Agent 2
│   ├── api_test_data.json                    # Agent 4
│   └── load_test_data.json                   # Agent 5
└── Makefile                                   # Test runner
```

### **Coding Standards**
```go
// Required imports for all test files
import (
    "testing"
    "context"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// Test naming convention
func TestFeatureName_Scenario_ExpectedOutcome(t *testing.T) {
    // Setup
    setup := setupTestEnvironment(t)
    defer setup.Cleanup()

    // Execute
    result, err := performTestOperation()

    // Verify
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}

// Required test timeout
func TestLongRunningOperation(t *testing.T) {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Test implementation with context
}
```

### **Shared Resource Management**
```go
// Test data isolation
const testPrefix = "test:integration:"

func (utils *TestUtils) CleanupTestData(t *testing.T) {
    // Clean Redis test keys
    keys, err := utils.redisClient.Keys(context.Background(), testPrefix+"*").Result()
    require.NoError(t, err)

    if len(keys) > 0 {
        utils.redisClient.Del(context.Background(), keys...)
    }
}

// Unique test identifiers
func generateTestID() string {
    return fmt.Sprintf("test-%d-%d", time.Now().Unix(), rand.Intn(1000))
}
```

## 🔄 Execution Workflow

### **Phase 1: Foundation (Agent 1)**
1. **Docker Infrastructure**: Verify Docker Compose stack health
2. **Service Dependencies**: Test startup sequence and connectivity
3. **Configuration**: Validate environment variable handling
4. **Deliverable**: Working Docker test framework

### **Phase 2: Core OCPP (Agent 2)**
1. **OCPP Simulator**: Build reusable OCPP message simulator
2. **Registration Flow**: Test BootNotification and Heartbeat
3. **Transactions**: Test StartTransaction → StopTransaction lifecycle
4. **Authorization**: Test ID tag validation
5. **Deliverable**: OCPP testing framework

### **Phase 3: Advanced OCPP (Agent 3)**
1. **Status Management**: Test connector status notifications
2. **Data Transfer**: Test vendor-specific data exchange
3. **Multi-Connector**: Test multiple connector scenarios
4. **Deliverable**: Advanced OCPP feature tests

### **Phase 4: HTTP Integration (Agent 4)**
1. **REST APIs**: Test charge point and transaction endpoints
2. **Correlation System**: Test synchronous OCPP commands
3. **Async Commands**: Test fire-and-forget commands
4. **Deliverable**: Complete HTTP API test suite

### **Phase 5: Resilience Testing (Agent 5)**
1. **Redis Operations**: Test queue processing and state persistence
2. **Failure Scenarios**: Test Redis connection failures
3. **Load Testing**: Test system under concurrent load
4. **Deliverable**: Production readiness validation

## ⚠️ Critical Coordination Points

### **Test Data Management**
- **Isolation**: Each test must clean up its data
- **Unique IDs**: Use timestamps/random IDs to avoid conflicts
- **Redis Keys**: Use test prefix for all Redis keys
- **Charge Point IDs**: Use unique prefixes per agent (TEST-A1-, TEST-A2-, etc.)

### **Resource Sharing**
- **Docker Containers**: Share containers, isolate data
- **Redis Client**: Reuse connections, separate namespaces
- **HTTP Ports**: Use different ports for concurrent testing
- **Queue Names**: Use test-specific queue names

### **Error Handling**
- **Timeout Management**: All tests must have timeouts (< 30s each)
- **Cleanup on Failure**: Tests must clean up even if they fail
- **Resource Leaks**: Monitor and prevent container/connection leaks
- **Parallel Execution**: Tests must be safe to run in parallel

## 📊 Success Metrics

### **Individual Agent Success**
- [ ] All assigned tests pass consistently (5+ runs)
- [ ] No resource leaks (containers, connections, memory)
- [ ] Tests complete within time limits
- [ ] Proper error handling and cleanup
- [ ] Comprehensive test coverage of assigned features

### **Integration Success**
- [ ] All 18 tests pass when run together
- [ ] Total execution time < 15 minutes
- [ ] No test interference or data conflicts
- [ ] Shared utilities work across all agents
- [ ] System demonstrates production readiness

### **Quality Gates**
- [ ] **Code Quality**: All code follows Go conventions
- [ ] **Documentation**: Clear test descriptions and expected behaviors
- [ ] **Error Messages**: Helpful failure messages for debugging
- [ ] **Performance**: Tests meet response time requirements
- [ ] **Reliability**: Tests pass consistently across different environments

## 🚀 Final Integration Checklist

### **Pre-Integration (Each Agent)**
- [ ] All individual tests pass locally
- [ ] Shared utilities created and documented
- [ ] Test data properly isolated and cleaned up
- [ ] Resource usage monitored and optimized
- [ ] Error scenarios properly handled

### **Integration Testing (All Agents)**
- [ ] Run all tests together successfully
- [ ] Verify no test conflicts or interference
- [ ] Confirm shared utilities work correctly
- [ ] Validate total execution time acceptable
- [ ] Test suite ready for CI/CD integration

### **Production Readiness**
- [ ] Docker stack deploys reliably
- [ ] All OCPP flows work correctly
- [ ] HTTP APIs function properly
- [ ] System handles errors gracefully
- [ ] Performance meets requirements under load

## 📞 Support & Coordination

### **Issue Resolution Process**
1. **Individual Issues**: Resolve within agent scope first
2. **Shared Component Issues**: Coordinate with component owner
3. **Integration Conflicts**: Escalate to coordination review
4. **Blocking Issues**: May require architectural adjustments

### **Communication Requirements**
- **Progress Updates**: Each agent reports completion status
- **Blocking Issues**: Immediate escalation if blocking other agents
- **Shared Changes**: Coordinate any changes to shared utilities
- **Final Integration**: All agents participate in final validation

## 🎯 Success Deliverable

**Complete Integration Test Suite** ready for deployment validation:
- 18 comprehensive integration tests
- Full Docker stack validation
- OCPP protocol compliance verification
- HTTP API functionality confirmation
- System resilience under load and failure conditions
- Production deployment readiness certification

This test suite will ensure confidence in every Docker stack redeployment and system update.