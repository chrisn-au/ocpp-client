# Agent 5: Redis & Resilience - Briefing Document

## 🎯 Mission Overview
You are responsible for implementing **Tests 13-18** which validate the Redis transport system, state persistence, error handling, and system resilience under adverse conditions. These tests ensure the system remains stable and recovers gracefully from failures.

## 📋 Your Test Assignments

### **Test 13: Redis Queue Processing**
**File**: `tests/integration/13_redis_queue_processing_test.go`

**Purpose**: Test Redis message processor functionality and queue management

**Queue Processing Scenarios**:
1. **Normal Message Flow**: Valid OCPP messages processed correctly
2. **Invalid Message Handling**: Malformed JSON gracefully handled
3. **Queue Blocking**: Processor doesn't block on empty queues
4. **Message Ordering**: FIFO queue processing maintained
5. **High Volume**: Multiple messages processed efficiently

**Test Steps**:
1. Start Redis message processor (internal/ocpp/processor.go)
2. Queue multiple valid OCPP messages to `ocpp:requests`
3. Verify messages consumed and processed in order
4. Queue invalid/malformed messages
5. Verify invalid messages logged and discarded
6. Test empty queue behavior (no blocking)
7. Verify queue recovery after Redis reconnection

**Expected Behavior**:
```go
// Queue multiple messages
messages := []string{
    `{"action": "BootNotification", "chargePointId": "CP-001"}`,
    `{"action": "Heartbeat", "chargePointId": "CP-001"}`,
    `{"invalid": "malformed_json"`,  // Invalid JSON
    `{"action": "StatusNotification", "chargePointId": "CP-001"}`
}

// Verify: 3 valid messages processed, 1 invalid logged and discarded
```

### **Test 14: Request/Response Correlation**
**File**: `tests/integration/14_correlation_manager_test.go`

**Purpose**: Test correlation manager memory management and timeout handling

**Correlation Scenarios**:
1. **Normal Correlation**: Request/response properly matched
2. **Timeout Handling**: Expired correlations cleaned up
3. **Memory Management**: No memory leaks with many correlations
4. **Concurrent Correlations**: Multiple simultaneous correlations
5. **Cleanup Verification**: Proper cleanup on success/timeout

**Test Steps**:
1. Register multiple pending correlations
2. Verify correlations stored correctly
3. Simulate responses for some correlations
4. Let others timeout (30+ seconds)
5. Verify expired correlations cleaned up
6. Test concurrent correlation registration/cleanup
7. Monitor memory usage during high correlation volume

**Memory Safety Test**:
```go
// Register 1000 correlations
for i := 0; i < 1000; i++ {
    correlationID := fmt.Sprintf("test-%d", i)
    cm.RegisterPendingRequest(correlationID, 30*time.Second)
}

// Verify memory usage stable
// Verify cleanup goroutine removes expired entries
```

### **Test 15: Redis State Persistence**
**File**: `tests/integration/15_redis_state_persistence_test.go`

**Purpose**: Test all StateStore interface operations and data integrity

**State Store Operations**:
1. **Charge Point CRUD**: Create, Read, Update, Delete operations
2. **Transaction Management**: Save, retrieve, update transactions
3. **Configuration Storage**: Charge point configuration persistence
4. **Concurrent Access**: Multiple simultaneous state operations
5. **Data Integrity**: Verify data consistency after operations

**Test Steps**:
1. Test all StateStore interface methods
2. Verify data persisted correctly in Redis
3. Test concurrent access from multiple goroutines
4. Verify data integrity after Redis restart
5. Test large data payloads
6. Verify proper error handling for Redis failures

**StateStore Operations Test**:
```go
// Test all interface methods
func TestStateStoreOperations(t *testing.T, stateStore store.StateStore) {
    // Charge Point operations
    info := &store.ChargePointInfo{...}
    err := stateStore.SaveChargePointInfo(ctx, "CP-001", info)
    require.NoError(t, err)

    retrieved, err := stateStore.GetChargePointInfo(ctx, "CP-001")
    require.NoError(t, err)
    assert.Equal(t, info, retrieved)

    // Transaction operations
    txn := &store.Transaction{...}
    err = stateStore.SaveTransaction(ctx, "CP-001", txn)
    require.NoError(t, err)

    // Configuration operations
    config := &store.ChargePointConfiguration{...}
    err = stateStore.SaveChargePointConfiguration(ctx, "CP-001", config)
    require.NoError(t, err)
}
```

### **Test 16: Redis Connection Failures**
**File**: `tests/integration/16_redis_connection_failures_test.go`

**Purpose**: Test system behavior when Redis is unavailable or fails

**Failure Scenarios**:
1. **Redis Down at Startup**: Server starts gracefully
2. **Redis Connection Lost**: Reconnection attempts successful
3. **Intermittent Failures**: System handles temporary Redis issues
4. **Recovery Testing**: System recovers when Redis returns
5. **Graceful Degradation**: Non-critical operations continue

**Test Steps**:
1. Start system with Redis unavailable
2. Verify server starts (may log errors but doesn't crash)
3. Start Redis and verify reconnection
4. Stop Redis during operation
5. Verify system handles Redis unavailability gracefully
6. Restart Redis and verify full recovery
7. Test partial Redis failures (timeouts, etc.)

**Resilience Test Flow**:
```bash
# Test sequence
1. docker stop redis-container
2. Start OCPP server → Should start with Redis errors
3. docker start redis-container
4. Verify reconnection within 30 seconds
5. Test all Redis operations work normally
```

### **Test 17: Invalid OCPP Message Handling**
**File**: `tests/integration/17_invalid_message_handling_test.go`

**Purpose**: Test system stability under malformed/invalid OCPP messages

**Invalid Message Scenarios**:
1. **Malformed JSON**: Invalid JSON syntax
2. **Missing Fields**: Required OCPP fields missing
3. **Invalid Actions**: Unknown OCPP actions
4. **Type Mismatches**: Wrong data types for fields
5. **Oversized Messages**: Messages exceeding limits

**Test Steps**:
1. Queue various invalid messages to Redis
2. Verify system processes them without crashing
3. Check error logging for appropriate messages
4. Verify system continues processing valid messages
5. Test recovery after burst of invalid messages
6. Monitor system stability and memory usage

**Invalid Message Test Cases**:
```json
// Test cases to queue
[
  `{invalid json syntax`,
  `{"action": "UnknownAction", "chargePointId": "CP-001"}`,
  `{"action": "BootNotification"}`,  // Missing chargePointId
  `{"action": "BootNotification", "chargePointId": 123}`,  // Wrong type
  `{"action": "BootNotification", "chargePointId": "` + strings.Repeat("A", 10000) + `"}`  // Oversized
]
```

### **Test 18: System Load & Concurrent Operations**
**File**: `tests/integration/18_system_load_test.go`

**Purpose**: Test system under concurrent load and stress conditions

**Load Testing Scenarios**:
1. **Multiple Charge Points**: 50+ simultaneous charge points
2. **Concurrent Transactions**: Multiple active transactions
3. **High Message Volume**: 100+ messages per second
4. **Memory Stability**: Extended operation monitoring
5. **Response Time Degradation**: Performance under load

**Test Steps**:
1. Simulate 50 charge points with different IDs
2. Start multiple concurrent transactions
3. Send high volume of OCPP messages
4. Monitor memory usage over 10+ minutes
5. Verify response times stay within limits
6. Test system recovery after load removal
7. Verify no data corruption under load

**Load Test Implementation**:
```go
func TestHighConcurrentLoad(t *testing.T) {
    chargePoints := 50
    messagesPerCP := 100

    var wg sync.WaitGroup

    // Start multiple charge point simulators
    for i := 0; i < chargePoints; i++ {
        wg.Add(1)
        go func(cpID string) {
            defer wg.Done()
            simulator := NewOCPPSimulator(cpID)

            // Send burst of messages
            for j := 0; j < messagesPerCP; j++ {
                simulator.SendHeartbeat()
                time.Sleep(10 * time.Millisecond)
            }
        }(fmt.Sprintf("CP-%03d", i))
    }

    wg.Wait()

    // Verify system stability
    verifyMemoryUsage(t)
    verifyResponseTimes(t)
}
```

## 🛠 Technical Implementation Guide

### **Required Dependencies**
```go
import (
    "testing"
    "context"
    "sync"
    "time"
    "runtime"
    "encoding/json"
    "github.com/redis/go-redis/v9"
    "github.com/testcontainers/testcontainers-go"
    "enhanced-ocpp-server/internal/store"
    "enhanced-ocpp-server/internal/ocpp"
)
```

### **Redis Testing Utilities**
```go
type RedisTestUtils struct {
    client   *redis.Client
    testData map[string]interface{}
}

func (r *RedisTestUtils) FlushTestData(t *testing.T) {
    // Clean up test keys
    keys, err := r.client.Keys(context.Background(), "test:*").Result()
    require.NoError(t, err)

    if len(keys) > 0 {
        r.client.Del(context.Background(), keys...)
    }
}

func (r *RedisTestUtils) MonitorMemoryUsage() *MemoryStats {
    var m1, m2 runtime.MemStats
    runtime.GC()
    runtime.ReadMemStats(&m1)

    time.Sleep(1 * time.Second)

    runtime.ReadMemStats(&m2)
    return &MemoryStats{
        Before: m1.Alloc,
        After:  m2.Alloc,
        Delta:  int64(m2.Alloc) - int64(m1.Alloc),
    }
}
```

### **Fault Injection Framework**
```go
type FaultInjector struct {
    redisContainer testcontainers.Container
}

func (fi *FaultInjector) StopRedis() error {
    return fi.redisContainer.Stop(context.Background(), nil)
}

func (fi *FaultInjector) StartRedis() error {
    return fi.redisContainer.Start(context.Background())
}

func (fi *FaultInjector) InjectLatency(delay time.Duration) {
    // Simulate network latency to Redis
}
```

## 📁 File Structure to Create
```
tests/
├── integration/
│   ├── 13_redis_queue_processing_test.go
│   ├── 14_correlation_manager_test.go
│   ├── 15_redis_state_persistence_test.go
│   ├── 16_redis_connection_failures_test.go
│   ├── 17_invalid_message_handling_test.go
│   ├── 18_system_load_test.go
│   └── resilience/
│       ├── redis_utils.go           # Redis testing utilities
│       ├── fault_injector.go        # Fault injection framework
│       ├── load_simulator.go        # Load testing tools
│       └── memory_monitor.go        # Memory usage monitoring
└── fixtures/
    ├── invalid_messages.json        # Invalid OCPP message samples
    └── load_test_data.json          # Load testing datasets
```

## ✅ Implementation Checklist

### **Pre-Development**
- [ ] Study Redis integration in internal/ocpp/processor.go
- [ ] Review correlation manager implementation
- [ ] Understand StateStore Redis implementation
- [ ] Set up fault injection testing framework

### **Test 13: Redis Queue Processing**
- [ ] Implement queue monitoring utilities
- [ ] Test valid message processing flow
- [ ] Add invalid message handling tests
- [ ] Verify message ordering (FIFO)
- [ ] Test empty queue behavior
- [ ] Add high-volume message processing
- [ ] Verify queue recovery after Redis restart

### **Test 14: Correlation Manager**
- [ ] Test correlation registration and cleanup
- [ ] Implement timeout testing framework
- [ ] Add memory leak detection
- [ ] Test concurrent correlation handling
- [ ] Verify cleanup goroutine functionality
- [ ] Add stress testing for high correlation volume

### **Test 15: Redis State Persistence**
- [ ] Test all StateStore interface methods
- [ ] Verify data integrity after operations
- [ ] Add concurrent access testing
- [ ] Test large data payload handling
- [ ] Verify Redis key structure and cleanup
- [ ] Add data consistency validation

### **Test 16: Redis Connection Failures**
- [ ] Implement fault injection framework
- [ ] Test startup with Redis unavailable
- [ ] Add reconnection testing
- [ ] Test graceful degradation scenarios
- [ ] Verify recovery after Redis restart
- [ ] Add intermittent failure handling

### **Test 17: Invalid Message Handling**
- [ ] Create invalid message test suite
- [ ] Test malformed JSON handling
- [ ] Add unknown action processing
- [ ] Test oversized message handling
- [ ] Verify system stability under invalid input
- [ ] Add error logging validation

### **Test 18: System Load & Concurrent Operations**
- [ ] Implement multi-charge point simulation
- [ ] Add concurrent transaction testing
- [ ] Create high-volume message generator
- [ ] Implement memory usage monitoring
- [ ] Add response time measurement
- [ ] Test extended operation stability

### **Resilience Testing Infrastructure**
- [ ] Create Redis testing utilities
- [ ] Implement fault injection framework
- [ ] Add load simulation tools
- [ ] Create memory monitoring utilities
- [ ] Implement performance measurement tools
- [ ] Add comprehensive error scenario testing

### **Quality Assurance**
- [ ] All tests pass under various Redis states
- [ ] No memory leaks detected during extended testing
- [ ] System recovers gracefully from all failure scenarios
- [ ] Performance degrades gracefully under load
- [ ] Error handling comprehensive and appropriate
- [ ] Test suite completes within 15 minutes

## 🔍 Validation Criteria

### **Resilience Requirements**
- System starts even if Redis initially unavailable
- Automatic reconnection within 30 seconds
- No crashes due to invalid messages
- Graceful degradation under high load
- Memory usage stable during extended operation

### **Performance Under Load**
- Handles 50+ concurrent charge points
- Processes 100+ messages per second
- Memory growth < 10% during load tests
- Response times increase < 2x under load
- Recovery to normal performance within 60 seconds

### **Error Handling**
- All Redis connection failures handled gracefully
- Invalid messages logged but don't crash system
- Proper error responses for malformed requests
- System continues normal operation after errors

## 📊 Test Data Requirements

### **Load Testing Data**
```json
{
  "charge_points": {
    "count": 50,
    "id_pattern": "LOAD-CP-{:03d}",
    "messages_per_cp": 100
  },
  "invalid_messages": [
    "{invalid json",
    "{\"action\": \"Unknown\"}",
    "{\"action\": null}",
    "{\"action\": \"BootNotification\", \"chargePointId\": " + "\"A\".repeat(10000) + "}"
  ],
  "performance_targets": {
    "max_memory_growth_mb": 100,
    "max_response_time_ms": 5000,
    "max_recovery_time_s": 60
  }
}
```

## 🚀 Success Deliverables

1. **Resilience Tests**: 6 comprehensive resilience and load tests
2. **Fault Injection Framework**: Redis failure simulation tools
3. **Load Testing Tools**: High-volume and concurrent testing utilities
4. **Performance Monitoring**: Memory and response time monitoring

## 🤝 Coordination Notes

- **Dependencies**: Requires all other agents' components for integration testing
- **Final Validation**: Your tests validate the complete system under stress
- **Performance Baselines**: Establish performance benchmarks for the system
- **Production Readiness**: Your tests determine if system is ready for production load

Your resilience tests are the final validation that the system can handle real-world conditions and recover gracefully from failures.