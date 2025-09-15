# Agent 2: Core OCPP Flows - Briefing Document

## 🎯 Mission Overview
You are responsible for implementing **Tests 4-6** which validate the core OCPP 1.6 protocol flows. These tests ensure proper charge point registration, transaction lifecycle management, and authorization handling through the Redis transport system.

## 📋 Your Test Assignments

### **Test 4: Charge Point Registration Flow**
**File**: `tests/integration/04_charge_point_registration_test.go`

**Purpose**: Test complete BootNotification → Heartbeat sequence

**OCPP Message Flow**:
1. **BootNotification** → Server accepts and stores charge point info
2. **Heartbeat** → Server updates last heartbeat timestamp
3. **State Verification** → Charge point info persisted correctly

**Test Steps**:
1. Send BootNotification message via Redis queue `ocpp:requests`
2. Verify BootNotificationConfirmation response in `ocpp:responses:{cpId}`
3. Check charge point info saved in Redis state store
4. Send Heartbeat message
5. Verify Heartbeat response and timestamp update
6. Validate OCPP-J message format compliance

**Expected OCPP Messages**:
```json
// BootNotification Request
{
  "action": "BootNotification",
  "chargePointId": "TEST-CP-001",
  "chargePointModel": "Test Model",
  "chargePointVendor": "Test Vendor",
  "firmwareVersion": "1.0.0"
}

// Expected Response (OCPP-J format)
{
  "charge_point_id": "TEST-CP-001",
  "message_id": "boot-123",
  "message_type": 3,
  "payload": [3, "boot-123", {
    "currentTime": "2024-01-15T10:30:00.000Z",
    "interval": 300,
    "status": "Accepted"
  }]
}
```

### **Test 5: Transaction Lifecycle (Start/Stop)**
**File**: `tests/integration/05_transaction_lifecycle_test.go`

**Purpose**: Test complete transaction flow with meter values

**Transaction Flow**:
1. **StartTransaction** → Creates transaction record
2. **MeterValues** → Updates transaction with meter readings
3. **StopTransaction** → Completes and archives transaction

**Test Steps**:
1. Send StartTransaction with valid idTag
2. Verify transaction created with unique ID
3. Send multiple MeterValues messages
4. Verify meter data linked to transaction
5. Send StopTransaction
6. Verify transaction marked as completed
7. Test transaction persistence and retrieval

**Expected Results**:
- Transaction ID generated and returned
- Transaction state: "active" → "completed"
- Meter values properly stored and linked
- Transaction data retrievable via API

### **Test 6: Authorization Flow**
**File**: `tests/integration/06_authorization_flow_test.go`

**Purpose**: Test ID tag authorization system

**Authorization Scenarios**:
1. **Valid ID Tag** → Authorization accepted
2. **Invalid ID Tag** → Authorization rejected
3. **Blocked ID Tag** → Authorization blocked
4. **Unknown ID Tag** → Authorization with appropriate status

**Test Steps**:
1. Test valid idTag authorization
2. Test invalid/blocked idTag scenarios
3. Verify proper IdTagInfo responses
4. Test authorization caching behavior
5. Verify OCPP-J response format

**Expected Responses**:
```json
// Valid Tag Response
{
  "idTagInfo": {
    "status": "Accepted",
    "expiryDate": "2024-12-31T23:59:59.000Z"
  }
}

// Invalid Tag Response
{
  "idTagInfo": {
    "status": "Invalid"
  }
}
```

## 🛠 Technical Implementation Guide

### **Required Dependencies**
```go
import (
    "testing"
    "context"
    "encoding/json"
    "time"
    "github.com/redis/go-redis/v9"
    "github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
    "github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
    "enhanced-ocpp-server/internal/store"
)
```

### **OCPP Message Simulator**
```go
type OCPPSimulator struct {
    redisClient *redis.Client
    chargePointID string
}

func (sim *OCPPSimulator) SendBootNotification(model, vendor, firmware string) error {
    message := map[string]interface{}{
        "action": "BootNotification",
        "chargePointId": sim.chargePointID,
        "chargePointModel": model,
        "chargePointVendor": vendor,
        "firmwareVersion": firmware,
    }

    messageBytes, _ := json.Marshal(message)
    return sim.redisClient.LPush(context.Background(), "ocpp:requests", messageBytes).Err()
}
```

### **Response Verification**
```go
func verifyOCPPResponse(t *testing.T, redisClient *redis.Client, cpID string, expectedAction string) map[string]interface{} {
    responseQueue := fmt.Sprintf("ocpp:responses:%s", cpID)

    // Wait for response with timeout
    result, err := redisClient.BRPop(context.Background(), 5*time.Second, responseQueue).Result()
    require.NoError(t, err)

    var response map[string]interface{}
    err = json.Unmarshal([]byte(result[1]), &response)
    require.NoError(t, err)

    // Verify OCPP-J format
    payload, ok := response["payload"].([]interface{})
    require.True(t, ok)
    require.Len(t, payload, 3) // [messageType, messageId, payload]

    return response
}
```

### **State Store Validation**
```go
func validateChargePointState(t *testing.T, stateStore store.StateStore, cpID string) {
    info, err := stateStore.GetChargePointInfo(context.Background(), cpID)
    require.NoError(t, err)
    require.NotNil(t, info)

    assert.Equal(t, cpID, info.ID)
    assert.Equal(t, "Available", info.Status)
    assert.WithinDuration(t, time.Now(), info.LastHeartbeat, 10*time.Second)
}
```

## 📁 File Structure to Create
```
tests/
├── integration/
│   ├── 04_charge_point_registration_test.go
│   ├── 05_transaction_lifecycle_test.go
│   ├── 06_authorization_flow_test.go
│   └── ocpp/
│       ├── simulator.go          # OCPP message simulator
│       ├── validators.go         # Response validation helpers
│       └── test_data.go          # Test data and fixtures
└── fixtures/
    └── ocpp_messages.json        # Sample OCPP messages
```

## ✅ Implementation Checklist

### **Pre-Development**
- [ ] Study OCPP 1.6 Core Profile specification
- [ ] Review internal/ocpp/handler.go implementations
- [ ] Understand Redis queue message formats
- [ ] Examine internal/store interfaces for state validation

### **Test 4: Charge Point Registration**
- [ ] Implement OCPP message simulator
- [ ] Create BootNotification message builder
- [ ] Add response queue monitoring
- [ ] Verify OCPP-J format compliance
- [ ] Test state store persistence
- [ ] Add Heartbeat sequence validation
- [ ] Test multiple charge point registration

### **Test 5: Transaction Lifecycle**
- [ ] Implement StartTransaction message handling
- [ ] Create transaction ID validation
- [ ] Add MeterValues message processing
- [ ] Test meter value linking to transactions
- [ ] Implement StopTransaction validation
- [ ] Verify transaction state transitions
- [ ] Test concurrent transaction handling

### **Test 6: Authorization Flow**
- [ ] Create Authorize message builder
- [ ] Implement IdTagInfo response validation
- [ ] Test valid/invalid/blocked ID tag scenarios
- [ ] Add authorization caching tests
- [ ] Verify OCPP status codes
- [ ] Test edge cases (empty tags, special characters)

### **OCPP Testing Infrastructure**
- [ ] Create reusable OCPP simulator
- [ ] Implement message builders for all tested actions
- [ ] Add response validators with OCPP-J format checking
- [ ] Create test data fixtures
- [ ] Implement state store validation helpers
- [ ] Add Redis queue monitoring utilities

### **Quality Assurance**
- [ ] All tests pass with various charge point IDs
- [ ] Messages follow OCPP 1.6 specification exactly
- [ ] State persistence verified after each operation
- [ ] Redis queues properly cleaned between tests
- [ ] Error scenarios handled appropriately
- [ ] Performance within acceptable limits

## 🔍 Validation Criteria

### **OCPP Compliance**
- All messages follow OCPP 1.6 Core Profile specification
- Responses use correct OCPP-J format [messageType, messageId, payload]
- Status codes match OCPP specification
- DateTime formats comply with ISO 8601

### **State Management**
- Charge point info correctly persisted and retrievable
- Transaction lifecycle properly tracked
- Authorization results cached appropriately
- State updates reflected in Redis immediately

### **Performance Requirements**
- BootNotification response: < 2 seconds
- Transaction operations: < 1 second each
- Authorization response: < 500ms
- Total test suite: < 3 minutes

## 📊 Test Data Requirements

### **Charge Point Test Data**
```json
{
  "valid_charge_points": [
    {
      "id": "TEST-CP-001",
      "model": "TestCharger v1.0",
      "vendor": "TestVendor",
      "firmware": "1.2.3"
    }
  ],
  "valid_id_tags": ["VALID001", "VALID002"],
  "invalid_id_tags": ["INVALID", "BLOCKED"],
  "meter_values": [
    {"timestamp": "2024-01-15T10:30:00Z", "value": 1234.56},
    {"timestamp": "2024-01-15T10:31:00Z", "value": 1245.78}
  ]
}
```

## 🚀 Success Deliverables

1. **Working OCPP Tests**: 3 tests covering core protocol flows
2. **OCPP Simulator**: Reusable component for other agents
3. **Validation Framework**: OCPP message and state validators
4. **Test Documentation**: Clear OCPP flow documentation

## 🤝 Coordination Notes

- **Dependencies**: Requires Agent 1's infrastructure tests to pass first
- **Shared Components**: Your OCPP simulator will be used by Agent 3
- **State Management**: Coordinate with Agent 5 on Redis state testing
- **API Integration**: Your transaction tests will be validated by Agent 4's API tests

Your OCPP flow tests are the foundation for protocol compliance - ensure they strictly follow OCPP 1.6 specification.