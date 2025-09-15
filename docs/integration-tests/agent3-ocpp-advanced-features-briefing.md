# Agent 3: OCPP Advanced Features - Briefing Document

## 🎯 Mission Overview
You are responsible for implementing **Tests 7-8** which validate advanced OCPP 1.6 features beyond basic transactions. These tests cover status notifications for connector management and data transfer capabilities for vendor-specific extensions.

## 📋 Your Test Assignments

### **Test 7: Status Notification Handling**
**File**: `tests/integration/07_status_notification_test.go`

**Purpose**: Test connector status updates and multi-connector management

**Status Scenarios to Test**:
1. **Single Connector States**: Available → Preparing → Charging → Finishing → Available
2. **Multi-Connector Tracking**: Independent status for connectors 1, 2, 3
3. **Error States**: Faulted, Unavailable, Reserved
4. **State Persistence**: Status retained across server restarts

**Test Steps**:
1. Send StatusNotification for connector state changes
2. Verify connector info updated in state store
3. Test multiple connectors simultaneously
4. Verify status history tracking
5. Test error/fault status handling
6. Validate status retrievable via API

**Expected OCPP Messages**:
```json
// StatusNotification Request
{
  "action": "StatusNotification",
  "chargePointId": "TEST-CP-001",
  "connectorId": 1,
  "status": "Charging",
  "errorCode": "NoError",
  "timestamp": "2024-01-15T10:30:00.000Z"
}

// Expected State Update
{
  "connector_info": [
    {
      "connector_id": 1,
      "status": "Charging",
      "availability_type": "Operative",
      "last_updated": "2024-01-15T10:30:00.000Z"
    }
  ]
}
```

**Connector Status Flow Testing**:
```
Available → Preparing → Charging → SuspendedEV → Charging → Finishing → Available
     ↓
  Faulted (error scenario)
     ↓
  Unavailable (maintenance)
     ↓
  Available (recovery)
```

### **Test 8: Data Transfer Capability**
**File**: `tests/integration/08_data_transfer_test.go`

**Purpose**: Test custom data exchange and vendor-specific functionality

**Data Transfer Scenarios**:
1. **Vendor Data**: Custom vendor-specific messages
2. **Large Payloads**: Test size limits and handling
3. **Binary Data**: Base64 encoded data transfer
4. **Bidirectional**: Both CS→CP and CP→CS transfers
5. **Error Handling**: Invalid/malformed data

**Test Steps**:
1. Send DataTransfer with vendor-specific data
2. Verify data properly routed and stored
3. Test large payload handling (up to OCPP limits)
4. Test binary data encoding/decoding
5. Verify response status and data echo
6. Test error scenarios with invalid data

**Expected OCPP Messages**:
```json
// DataTransfer Request (Vendor Specific)
{
  "action": "DataTransfer",
  "chargePointId": "TEST-CP-001",
  "vendorId": "TestVendor",
  "messageId": "CustomCommand",
  "data": {
    "customField1": "value1",
    "customField2": 12345,
    "configuration": {
      "setting1": true,
      "setting2": "custom_value"
    }
  }
}

// DataTransfer Response
{
  "status": "Accepted",
  "data": {
    "response": "Data received successfully",
    "processedAt": "2024-01-15T10:30:00.000Z"
  }
}
```

**Data Transfer Test Cases**:
1. **Diagnostic Data**: Simulate diagnostic upload request
2. **Configuration Sync**: Custom configuration data exchange
3. **Firmware Metadata**: Firmware version and capabilities
4. **Usage Statistics**: Energy consumption and session data
5. **Error Logs**: Custom error reporting

## 🛠 Technical Implementation Guide

### **Required Dependencies**
```go
import (
    "testing"
    "context"
    "encoding/json"
    "encoding/base64"
    "time"
    "github.com/redis/go-redis/v9"
    "github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
    "github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
    "enhanced-ocpp-server/internal/store"
)
```

### **Status Notification Simulator**
```go
type StatusNotificationSimulator struct {
    ocppSim *OCPPSimulator // Reuse from Agent 2
}

func (sim *StatusNotificationSimulator) SendStatusNotification(connectorID int, status core.ChargePointStatus, errorCode core.ChargePointErrorCode) error {
    message := map[string]interface{}{
        "action": "StatusNotification",
        "chargePointId": sim.ocppSim.chargePointID,
        "connectorId": connectorID,
        "status": string(status),
        "errorCode": string(errorCode),
        "timestamp": time.Now().Format(time.RFC3339),
    }

    messageBytes, _ := json.Marshal(message)
    return sim.ocppSim.redisClient.LPush(context.Background(), "ocpp:requests", messageBytes).Err()
}
```

### **Data Transfer Testing**
```go
func (sim *StatusNotificationSimulator) SendDataTransfer(vendorID, messageID string, data interface{}) error {
    message := map[string]interface{}{
        "action": "DataTransfer",
        "chargePointId": sim.ocppSim.chargePointID,
        "vendorId": vendorID,
        "messageId": messageID,
        "data": data,
    }

    messageBytes, _ := json.Marshal(message)
    return sim.ocppSim.redisClient.LPush(context.Background(), "ocpp:requests", messageBytes).Err()
}
```

### **Multi-Connector State Validation**
```go
func validateConnectorStates(t *testing.T, stateStore store.StateStore, cpID string, expectedStates map[int]string) {
    info, err := stateStore.GetChargePointInfo(context.Background(), cpID)
    require.NoError(t, err)

    for _, connector := range info.ConnectorInfo {
        expectedStatus, exists := expectedStates[connector.ConnectorID]
        require.True(t, exists, "Unexpected connector ID: %d", connector.ConnectorID)
        assert.Equal(t, expectedStatus, connector.Status)
    }
}
```

## 📁 File Structure to Create
```
tests/
├── integration/
│   ├── 07_status_notification_test.go
│   ├── 08_data_transfer_test.go
│   └── ocpp/
│       ├── status_simulator.go      # Status notification utilities
│       ├── data_transfer_utils.go   # Data transfer helpers
│       └── connector_validators.go  # Connector state validation
└── fixtures/
    ├── connector_flows.json         # Status transition test data
    └── data_transfer_samples.json   # Sample vendor data
```

## ✅ Implementation Checklist

### **Pre-Development**
- [ ] Review OCPP 1.6 StatusNotification and DataTransfer specifications
- [ ] Study internal/ocpp/handler.go status and data transfer implementations
- [ ] Understand connector state management in store interfaces
- [ ] Coordinate with Agent 2 to reuse OCPP simulator

### **Test 7: Status Notification**
- [ ] Implement status notification simulator
- [ ] Create connector state transition tests
- [ ] Add multi-connector simultaneous testing
- [ ] Test all OCPP status values (Available, Preparing, Charging, etc.)
- [ ] Verify error code handling (NoError, ConnectorLockFailure, etc.)
- [ ] Test status persistence across operations
- [ ] Add status history tracking validation

### **Test 8: Data Transfer**
- [ ] Implement data transfer message builder
- [ ] Create vendor-specific data test cases
- [ ] Test large payload handling (within OCPP limits)
- [ ] Add binary data encoding/decoding tests
- [ ] Test bidirectional data transfer flows
- [ ] Verify response status codes (Accepted, Rejected, UnknownVendorId)
- [ ] Add data persistence and retrieval validation

### **Advanced Testing Scenarios**
- [ ] Test rapid status changes (stress testing)
- [ ] Verify concurrent status updates from multiple connectors
- [ ] Test data transfer with malformed/invalid data
- [ ] Add timeout handling for data transfer operations
- [ ] Test maximum payload size limits
- [ ] Verify proper error responses for edge cases

### **State Management Integration**
- [ ] Verify status changes reflected in Redis state store
- [ ] Test connector availability type tracking
- [ ] Validate status timestamps and ordering
- [ ] Test data transfer logging and persistence
- [ ] Verify state cleanup and management

### **Quality Assurance**
- [ ] All status transitions follow OCPP specification
- [ ] Data transfer supports all required data types
- [ ] Error handling covers edge cases
- [ ] Performance testing for high-frequency status updates
- [ ] Memory usage stable during extended testing
- [ ] Proper cleanup of test data

## 🔍 Validation Criteria

### **Status Notification Compliance**
- All OCPP 1.6 status values supported correctly
- Error codes properly mapped and handled
- Timestamps in correct ISO 8601 format
- Multi-connector independence verified

### **Data Transfer Compliance**
- Vendor ID and Message ID properly processed
- Data payloads correctly encoded/decoded
- Response status matches OCPP specification
- Large payloads handled within limits

### **Performance Requirements**
- Status notification processing: < 500ms
- Data transfer processing: < 2 seconds
- Multi-connector updates: < 1 second total
- Memory usage stable under load

## 📊 Test Data Requirements

### **Connector Status Test Data**
```json
{
  "status_transitions": [
    {
      "sequence": ["Available", "Preparing", "Charging", "Finishing", "Available"],
      "connector_id": 1,
      "error_codes": ["NoError", "NoError", "NoError", "NoError", "NoError"]
    },
    {
      "sequence": ["Available", "Faulted", "Unavailable", "Available"],
      "connector_id": 2,
      "error_codes": ["NoError", "ConnectorLockFailure", "NoError", "NoError"]
    }
  ]
}
```

### **Data Transfer Test Data**
```json
{
  "vendor_data_samples": [
    {
      "vendor_id": "TestVendor",
      "message_id": "DiagnosticUpload",
      "data": {
        "log_level": "INFO",
        "timestamp": "2024-01-15T10:30:00Z",
        "entries": ["System started", "Connection established"]
      }
    },
    {
      "vendor_id": "CustomProvider",
      "message_id": "ConfigSync",
      "data": {
        "settings": {"max_current": 32, "phases": 3},
        "version": "1.2.3"
      }
    }
  ]
}
```

## 🚀 Success Deliverables

1. **Advanced OCPP Tests**: 2 tests covering status and data transfer
2. **Status Management Tools**: Connector state testing utilities
3. **Data Transfer Framework**: Vendor data testing capabilities
4. **Validation Tools**: State and data verification helpers

## 🤝 Coordination Notes

- **Dependencies**: Requires Agent 2's OCPP simulator and basic message flow
- **Shared Components**: Extend Agent 2's OCPP testing framework
- **State Validation**: Coordinate with Agent 5 on Redis state testing
- **API Integration**: Your connector status will be validated by Agent 4's API tests

Your advanced OCPP tests ensure the system handles complex real-world scenarios beyond basic transactions.