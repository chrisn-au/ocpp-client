# Agent 4: HTTP API & Correlation - Briefing Document

## 🎯 Mission Overview
You are responsible for implementing **Tests 9-12** which validate the HTTP REST API endpoints and the critical request/response correlation system. These tests ensure the HTTP-to-OCPP bridge works correctly and that synchronous API calls properly correlate with asynchronous OCPP responses.

## 📋 Your Test Assignments

### **Test 9: Charge Point Information API**
**File**: `tests/integration/09_charge_point_api_test.go`

**Purpose**: Test GET `/chargepoints/{id}` endpoint with various scenarios

**API Scenarios**:
1. **Existing Charge Point**: Returns complete charge point information
2. **Non-existent Charge Point**: Returns 404 with appropriate error
3. **Malformed Request**: Invalid charge point ID format
4. **Data Completeness**: All fields properly serialized to JSON

**Test Steps**:
1. Pre-populate charge point data (via Agent 2's OCPP simulator)
2. Test successful retrieval with valid charge point ID
3. Test 404 response for non-existent charge point
4. Verify JSON structure matches specification
5. Test concurrent API requests
6. Validate response headers and status codes

**Expected API Response**:
```json
GET /chargepoints/TEST-CP-001 → 200 OK
{
  "id": "TEST-CP-001",
  "status": "Available",
  "last_heartbeat": "2024-01-15T10:30:00.000Z",
  "connector_info": [
    {
      "connector_id": 1,
      "status": "Available",
      "availability_type": "Operative"
    }
  ]
}
```

### **Test 10: Transaction Information API**
**File**: `tests/integration/10_transaction_api_test.go`

**Purpose**: Test GET `/chargepoints/{cpid}/transactions/{txnid}` nested resource endpoint

**Transaction API Scenarios**:
1. **Active Transaction**: Returns current transaction details
2. **Completed Transaction**: Returns historical transaction data
3. **Invalid Transaction ID**: Returns 404 error
4. **Invalid Charge Point**: Returns 404 error
5. **Data Integrity**: All transaction fields present and correct

**Test Steps**:
1. Create transaction via OCPP StartTransaction (Agent 2 simulator)
2. Test API retrieval of active transaction
3. Complete transaction via OCPP StopTransaction
4. Test API retrieval of completed transaction
5. Test error scenarios (invalid IDs)
6. Verify nested resource URL handling

**Expected API Response**:
```json
GET /chargepoints/TEST-CP-001/transactions/123 → 200 OK
{
  "id": 123,
  "charge_point_id": "TEST-CP-001",
  "connector_id": 1,
  "id_tag": "VALID001",
  "start_time": "2024-01-15T10:30:00.000Z",
  "meter_start": 1234,
  "status": "active"
}
```

### **Test 11: Synchronous OCPP Commands (GetConfiguration)**
**File**: `tests/integration/11_synchronous_ocpp_api_test.go`

**Purpose**: Test HTTP → OCPP → HTTP correlation flow with timeout handling

**Correlation Flow**:
1. **HTTP Request** → POST `/api/charge-points/{id}/get-configuration`
2. **OCPP Message** → Queued to `ocpp:responses:{id}` with correlation ID
3. **OCPP Response** → Simulated charge point response
4. **HTTP Response** → Correlated back to original HTTP request

**Test Steps**:
1. Set up charge point that can respond to GetConfiguration
2. Send HTTP POST request for configuration
3. Verify OCPP message queued with correlation ID
4. Simulate OCPP response from charge point
5. Verify HTTP response contains configuration data
6. Test timeout scenarios (30 second timeout)
7. Test error responses from charge point

**Expected API Flow**:
```bash
# HTTP Request
POST /api/charge-points/TEST-CP-001/get-configuration
{
  "keys": ["Heartbeat Interval", "MeterValuesSampledData"]
}

# Queued OCPP Message (Redis: ocpp:responses:TEST-CP-001)
{
  "charge_point_id": "TEST-CP-001",
  "message_id": "http-GetConfiguration-TEST-CP-001-1234567890",
  "message_type": 2,
  "payload": [2, "http-GetConfiguration-TEST-CP-001-1234567890", "GetConfiguration", {...}]
}

# HTTP Response (after correlation)
{
  "success": true,
  "data": {
    "configurationKey": [
      {"key": "HeartbeatInterval", "readonly": false, "value": "300"},
      {"key": "MeterValuesSampledData", "readonly": false, "value": "Energy.Active.Import.Register"}
    ]
  }
}
```

### **Test 12: Asynchronous OCPP Commands (ChangeAvailability)**
**File**: `tests/integration/12_asynchronous_ocpp_api_test.go`

**Purpose**: Test fire-and-forget OCPP commands with immediate HTTP response

**Asynchronous Command Scenarios**:
1. **Valid Request**: Command queued successfully
2. **Invalid Parameters**: Validation errors before queuing
3. **Missing Parameters**: Proper error responses
4. **Queue Verification**: Command actually queued to Redis

**Test Steps**:
1. Send ChangeAvailability command via HTTP API
2. Verify immediate HTTP response confirms queuing
3. Verify OCPP message queued to Redis
4. Test parameter validation (connectorId, type)
5. Test error scenarios (invalid parameters)
6. Verify no correlation ID needed for fire-and-forget

**Expected API Flow**:
```bash
# HTTP Request
POST /api/charge-points/TEST-CP-001/change-availability
{
  "connectorId": 1,
  "type": "Inoperative"
}

# Immediate HTTP Response
{
  "success": true,
  "message": "ChangeAvailability request sent successfully",
  "data": {
    "chargePointId": "TEST-CP-001",
    "connectorId": 1,
    "type": "Inoperative"
  }
}

# Queued OCPP Message (Redis: ocpp:requests)
{
  "action": "ChangeAvailability",
  "chargePointId": "TEST-CP-001",
  "connectorId": 1,
  "type": "Inoperative"
}
```

## 🛠 Technical Implementation Guide

### **Required Dependencies**
```go
import (
    "testing"
    "net/http"
    "net/http/httptest"
    "encoding/json"
    "bytes"
    "context"
    "time"
    "github.com/redis/go-redis/v9"
    "github.com/gorilla/mux"
    "enhanced-ocpp-server/internal/api"
    "enhanced-ocpp-server/internal/server"
    "enhanced-ocpp-server/internal/ocpp"
)
```

### **HTTP Test Client**
```go
type HTTPTestClient struct {
    server     *httptest.Server
    baseURL    string
    httpClient *http.Client
}

func NewHTTPTestClient(router *mux.Router) *HTTPTestClient {
    server := httptest.NewServer(router)
    return &HTTPTestClient{
        server:     server,
        baseURL:    server.URL,
        httpClient: &http.Client{Timeout: 30 * time.Second},
    }
}

func (client *HTTPTestClient) GetChargePoint(cpID string) (*http.Response, error) {
    url := fmt.Sprintf("%s/chargepoints/%s", client.baseURL, cpID)
    return client.httpClient.Get(url)
}
```

### **Correlation Testing Framework**
```go
type CorrelationTester struct {
    redisClient        *redis.Client
    correlationManager *ocpp.CorrelationManager
    httpClient         *HTTPTestClient
}

func (ct *CorrelationTester) TestSynchronousRequest(t *testing.T, cpID string, requestData map[string]interface{}) {
    // Send HTTP request
    response, err := ct.httpClient.PostJSON(
        fmt.Sprintf("/api/charge-points/%s/get-configuration", cpID),
        requestData,
    )
    require.NoError(t, err)

    // Verify response
    assert.Equal(t, http.StatusOK, response.StatusCode)

    // Verify OCPP message was queued
    ct.verifyOCPPMessageQueued(t, cpID)
}
```

### **Redis Queue Monitoring**
```go
func (ct *CorrelationTester) verifyOCPPMessageQueued(t *testing.T, cpID string) {
    responseQueue := fmt.Sprintf("ocpp:responses:%s", cpID)

    // Check message was queued
    length, err := ct.redisClient.LLen(context.Background(), responseQueue).Result()
    require.NoError(t, err)
    assert.Greater(t, length, int64(0))

    // Peek at message to verify format
    message, err := ct.redisClient.LIndex(context.Background(), responseQueue, 0).Result()
    require.NoError(t, err)

    var ocppMessage map[string]interface{}
    err = json.Unmarshal([]byte(message), &ocppMessage)
    require.NoError(t, err)

    // Verify OCPP-J format
    assert.Contains(t, ocppMessage, "message_id")
    assert.Contains(t, ocppMessage, "payload")
}
```

## 📁 File Structure to Create
```
tests/
├── integration/
│   ├── 09_charge_point_api_test.go
│   ├── 10_transaction_api_test.go
│   ├── 11_synchronous_ocpp_api_test.go
│   ├── 12_asynchronous_ocpp_api_test.go
│   └── http/
│       ├── test_client.go           # HTTP test utilities
│       ├── correlation_tester.go    # Correlation testing framework
│       └── api_validators.go        # API response validation
└── fixtures/
    ├── api_requests.json            # Sample API request bodies
    └── api_responses.json           # Expected API responses
```

## ✅ Implementation Checklist

### **Pre-Development**
- [ ] Study internal/api/handlers.go API implementations
- [ ] Review internal/server/http_setup.go route configuration
- [ ] Understand correlation manager in internal/ocpp/correlation.go
- [ ] Set up HTTP testing framework with httptest

### **Test 9: Charge Point API**
- [ ] Implement HTTP test client utilities
- [ ] Create charge point data setup (reuse Agent 2/3 simulators)
- [ ] Test successful charge point retrieval
- [ ] Test 404 scenarios for non-existent charge points
- [ ] Verify JSON response structure and completeness
- [ ] Test error handling and edge cases
- [ ] Add concurrent request testing

### **Test 10: Transaction API**
- [ ] Set up transaction test data via OCPP simulator
- [ ] Test nested resource URL handling
- [ ] Verify active transaction retrieval
- [ ] Test completed transaction retrieval
- [ ] Test invalid transaction/charge point ID scenarios
- [ ] Verify data integrity and completeness
- [ ] Add performance testing for large transaction datasets

### **Test 11: Synchronous OCPP API**
- [ ] Implement correlation testing framework
- [ ] Set up mock charge point responses
- [ ] Test GetConfiguration request/response flow
- [ ] Verify correlation ID generation and matching
- [ ] Test timeout scenarios (30 second limit)
- [ ] Test error responses from charge points
- [ ] Verify proper cleanup of correlation data

### **Test 12: Asynchronous OCPP API**
- [ ] Test ChangeAvailability command queuing
- [ ] Verify immediate HTTP response format
- [ ] Test parameter validation before queuing
- [ ] Verify OCPP message format in queue
- [ ] Test error scenarios with invalid parameters
- [ ] Add performance testing for high-frequency commands

### **HTTP Testing Infrastructure**
- [ ] Create reusable HTTP test client
- [ ] Implement JSON request/response utilities
- [ ] Add API response validation framework
- [ ] Create mock charge point response simulator
- [ ] Implement Redis queue monitoring tools
- [ ] Add comprehensive error scenario testing

### **Quality Assurance**
- [ ] All API responses follow REST conventions
- [ ] Correlation system handles timeouts correctly
- [ ] No memory leaks in correlation manager
- [ ] Proper HTTP status codes for all scenarios
- [ ] JSON responses properly formatted
- [ ] Performance meets requirements (< 2s for sync requests)

## 🔍 Validation Criteria

### **API Compliance**
- REST endpoint conventions followed
- Proper HTTP status codes (200, 404, 400, 500)
- JSON responses well-formed and complete
- Error messages clear and actionable

### **Correlation System**
- Correlation IDs unique and traceable
- Timeout handling works correctly (30s)
- Memory cleanup prevents leaks
- Concurrent correlations handled properly

### **Performance Requirements**
- GET APIs: < 500ms response time
- Synchronous OCPP APIs: < 30s (with 30s timeout)
- Asynchronous APIs: < 1s immediate response
- API handles 10+ concurrent requests

## 📊 Test Data Requirements

### **API Test Data**
```json
{
  "charge_points": [
    {
      "id": "TEST-CP-001",
      "status": "Available",
      "connectors": [{"id": 1, "status": "Available"}]
    }
  ],
  "transactions": [
    {
      "id": 123,
      "charge_point_id": "TEST-CP-001",
      "connector_id": 1,
      "id_tag": "VALID001",
      "status": "active"
    }
  ],
  "api_requests": {
    "get_configuration": {
      "keys": ["HeartbeatInterval", "MeterValuesSampledData"]
    },
    "change_availability": {
      "connectorId": 1,
      "type": "Inoperative"
    }
  }
}
```

## 🚀 Success Deliverables

1. **HTTP API Tests**: 4 comprehensive API endpoint tests
2. **Correlation Framework**: Request/response correlation testing tools
3. **API Validation Tools**: Response validation and error checking
4. **Performance Tests**: API load and timeout testing

## 🤝 Coordination Notes

- **Dependencies**: Requires Agent 1's infrastructure and Agent 2's OCPP simulator
- **Data Setup**: Coordinate with Agent 2/3 for charge point and transaction data
- **State Validation**: Work with Agent 5 on Redis state consistency
- **End-to-End Flow**: Your tests validate the complete HTTP→OCPP→HTTP pipeline

Your API tests are critical for validating the user-facing interface and ensuring the correlation system works reliably under all conditions.