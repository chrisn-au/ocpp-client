# OCPP Core + Smart Charging - Agent Execution Plan

## Overview
Detailed, step-by-step instructions for agents to implement OCPP 1.6 Core and Smart Charging profiles. Each chunk includes implementation, tests, documentation, and validation criteria.

## Project Structure
```
/Users/chrishome/development/home/mcp-access/csms/ocpp-server/
├── handlers/           # OCPP message handlers
├── config/            # Configuration management
├── commands/          # Server-initiated commands
├── models/            # Data structures
├── middleware/        # Auth middleware (placeholder)
├── tests/             # Unit and integration tests
├── docs/              # API documentation
└── scripts/           # Test scripts for validation
```

---

## CHUNK 1.1: Enhanced Authorization & Data Transfer

### Objective
Implement Authorize and DataTransfer OCPP handlers with ID tag validation and vendor-specific data routing.

### Prerequisites
- Current Redis business state working
- Basic OCPP handlers functional

### Implementation Tasks

#### Task 1.1.1: Create Authorization Manager
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/auth/id_tag_manager.go`

```go
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
)

// IDTagManager handles ID tag authorization and caching
type IDTagManager struct {
	businessState BusinessStateInterface
	config        *AuthConfig
}

type AuthConfig struct {
	DefaultAuthStatus types.AuthorizationStatus
	CacheExpiry      time.Duration
	AllowUnknownTags bool
}

type IDTagInfo struct {
	IDTag         string                    `json:"idTag"`
	Status        types.AuthorizationStatus `json:"status"`
	ExpiryDate    *time.Time               `json:"expiryDate,omitempty"`
	ParentIDTag   *string                  `json:"parentIdTag,omitempty"`
	CachedAt      time.Time                `json:"cachedAt"`
	Source        string                   `json:"source"` // "local", "external", "default"
}

type BusinessStateInterface interface {
	GetIDTagInfo(idTag string) (*IDTagInfo, error)
	SetIDTagInfo(info *IDTagInfo) error
	InvalidateIDTagCache(idTag string) error
}

// NewIDTagManager creates a new ID tag manager
func NewIDTagManager(businessState BusinessStateInterface, config *AuthConfig) *IDTagManager {
	if config == nil {
		config = &AuthConfig{
			DefaultAuthStatus: types.AuthorizationStatusAccepted, // Accept all for now
			CacheExpiry:      24 * time.Hour,
			AllowUnknownTags: true,
		}
	}

	return &IDTagManager{
		businessState: businessState,
		config:        config,
	}
}

// AuthorizeIDTag validates an ID tag and returns authorization info
func (itm *IDTagManager) AuthorizeIDTag(idTag string) (*types.IdTagInfo, error) {
	// Check cache first
	cachedInfo, err := itm.businessState.GetIDTagInfo(idTag)
	if err == nil && cachedInfo != nil {
		// Check if cache is still valid
		if time.Since(cachedInfo.CachedAt) < itm.config.CacheExpiry {
			return &types.IdTagInfo{
				Status:      cachedInfo.Status,
				ExpiryDate:  cachedInfo.ExpiryDate,
				ParentIdTag: cachedInfo.ParentIDTag,
			}, nil
		}
	}

	// For now, use default policy - later this will call external auth service
	authInfo := &IDTagInfo{
		IDTag:      idTag,
		Status:     itm.config.DefaultAuthStatus,
		CachedAt:   time.Now(),
		Source:     "default",
	}

	// Store in cache
	if err := itm.businessState.SetIDTagInfo(authInfo); err != nil {
		// Log error but don't fail authorization
	}

	return &types.IdTagInfo{
		Status:      authInfo.Status,
		ExpiryDate:  authInfo.ExpiryDate,
		ParentIdTag: authInfo.ParentIDTag,
	}, nil
}

// GenerateIDTag creates a new random ID tag for testing
func (itm *IDTagManager) GenerateIDTag() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return "TAG-" + hex.EncodeToString(bytes)[:12]
}
```

#### Task 1.1.2: Create Data Transfer Handler
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/handlers/data_transfer.go`

```go
package handlers

import (
	"encoding/json"
	"log"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
)

// DataTransferManager handles vendor-specific data transfer requests
type DataTransferManager struct {
	businessState BusinessStateInterface
	vendorHandlers map[string]VendorHandler
}

type VendorHandler interface {
	HandleData(chargePointID string, messageId *string, data *string) (*core.DataTransferConfirmation, error)
}

type DataTransferLog struct {
	ChargePointID string    `json:"chargePointId"`
	VendorID      string    `json:"vendorId"`
	MessageID     *string   `json:"messageId,omitempty"`
	Data          *string   `json:"data,omitempty"`
	Response      *string   `json:"response,omitempty"`
	Status        string    `json:"status"`
	Timestamp     time.Time `json:"timestamp"`
}

// NewDataTransferManager creates a new data transfer manager
func NewDataTransferManager(businessState BusinessStateInterface) *DataTransferManager {
	return &DataTransferManager{
		businessState: businessState,
		vendorHandlers: make(map[string]VendorHandler),
	}
}

// RegisterVendorHandler registers a handler for a specific vendor
func (dtm *DataTransferManager) RegisterVendorHandler(vendorID string, handler VendorHandler) {
	dtm.vendorHandlers[vendorID] = handler
}

// HandleDataTransfer processes a data transfer request
func (dtm *DataTransferManager) HandleDataTransfer(chargePointID string, req *core.DataTransferRequest) *core.DataTransferConfirmation {
	log.Printf("DataTransfer from %s: VendorId=%s, MessageId=%v",
		chargePointID, req.VendorId, req.MessageId)

	// Log the request
	transferLog := &DataTransferLog{
		ChargePointID: chargePointID,
		VendorID:      req.VendorId,
		MessageID:     req.MessageId,
		Data:          req.Data,
		Status:        "received",
		Timestamp:     time.Now(),
	}

	// Check if we have a handler for this vendor
	if handler, exists := dtm.vendorHandlers[req.VendorId]; exists {
		response, err := handler.HandleData(chargePointID, req.MessageId, req.Data)
		if err != nil {
			log.Printf("Vendor handler error for %s: %v", req.VendorId, err)
			transferLog.Status = "error"
			dtm.logDataTransfer(transferLog)
			return core.NewDataTransferConfirmation(core.DataTransferStatusRejected)
		}

		transferLog.Status = "handled"
		if response.Data != nil {
			transferLog.Response = response.Data
		}
		dtm.logDataTransfer(transferLog)
		return response
	}

	// Default handler - just accept and log
	transferLog.Status = "accepted"
	dtm.logDataTransfer(transferLog)

	return core.NewDataTransferConfirmation(core.DataTransferStatusAccepted)
}

func (dtm *DataTransferManager) logDataTransfer(log *DataTransferLog) {
	// Store in business state for audit trail
	// Implementation depends on business state interface
}

// DefaultVendorHandler provides a simple echo/logging handler
type DefaultVendorHandler struct{}

func (dvh *DefaultVendorHandler) HandleData(chargePointID string, messageId *string, data *string) (*core.DataTransferConfirmation, error) {
	response := "Echo: received data"
	if data != nil {
		response = "Echo: " + *data
	}

	return core.NewDataTransferConfirmation(core.DataTransferStatusAccepted, &response), nil
}
```

#### Task 1.1.3: Update Main OCPP Handlers
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/main.go`

**Add to Server struct:**
```go
type Server struct {
	// ... existing fields
	idTagManager      *auth.IDTagManager
	dataTransferMgr   *handlers.DataTransferManager
}
```

**Add initialization in main():**
```go
// Create auth manager
authConfig := &auth.AuthConfig{
	DefaultAuthStatus: types.AuthorizationStatusAccepted,
	CacheExpiry:      24 * time.Hour,
	AllowUnknownTags: true,
}
server.idTagManager = auth.NewIDTagManager(businessState, authConfig)

// Create data transfer manager
server.dataTransferMgr = handlers.NewDataTransferManager(businessState)
// Register default vendor handler
server.dataTransferMgr.RegisterVendorHandler("default", &handlers.DefaultVendorHandler{})
```

**Add to setupOCPPHandlers():**
```go
case *core.AuthorizeRequest:
	s.handleAuthorize(clientID, requestId, req)

case *core.DataTransferRequest:
	s.handleDataTransfer(clientID, requestId, req)
```

**Add handler methods:**
```go
func (s *Server) handleAuthorize(clientID, requestId string, req *core.AuthorizeRequest) {
	log.Printf("Authorize from %s: IdTag=%s", clientID, req.IdTag)

	idTagInfo, err := s.idTagManager.AuthorizeIDTag(req.IdTag)
	if err != nil {
		log.Printf("Authorization error for %s: %v", req.IdTag, err)
		idTagInfo = &types.IdTagInfo{Status: types.AuthorizationStatusInvalid}
	}

	response := core.NewAuthorizeConfirmation(*idTagInfo)
	if err := s.ocppServer.SendResponse(clientID, requestId, response); err != nil {
		log.Printf("Error sending Authorize response: %v", err)
	} else {
		log.Printf("Sent Authorize response to %s: Status=%s", clientID, idTagInfo.Status)
	}
}

func (s *Server) handleDataTransfer(clientID, requestId string, req *core.DataTransferRequest) {
	response := s.dataTransferMgr.HandleDataTransfer(clientID, req)

	if err := s.ocppServer.SendResponse(clientID, requestId, response); err != nil {
		log.Printf("Error sending DataTransfer response: %v", err)
	} else {
		log.Printf("Sent DataTransfer response to %s: Status=%s", clientID, response.Status)
	}
}
```

### Testing

#### Task 1.1.4: Unit Tests
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/tests/auth_test.go`

```go
package tests

import (
	"testing"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"ocpp-server/auth"
)

// MockBusinessState for testing
type MockBusinessState struct {
	mock.Mock
}

func (m *MockBusinessState) GetIDTagInfo(idTag string) (*auth.IDTagInfo, error) {
	args := m.Called(idTag)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.IDTagInfo), args.Error(1)
}

func (m *MockBusinessState) SetIDTagInfo(info *auth.IDTagInfo) error {
	args := m.Called(info)
	return args.Error(0)
}

func (m *MockBusinessState) InvalidateIDTagCache(idTag string) error {
	args := m.Called(idTag)
	return args.Error(0)
}

func TestIDTagManager_AuthorizeIDTag_Accepted(t *testing.T) {
	mockState := new(MockBusinessState)
	config := &auth.AuthConfig{
		DefaultAuthStatus: types.AuthorizationStatusAccepted,
		CacheExpiry:      time.Hour,
		AllowUnknownTags: true,
	}

	manager := auth.NewIDTagManager(mockState, config)

	// Mock no cached entry
	mockState.On("GetIDTagInfo", "test-tag").Return(nil, nil)
	mockState.On("SetIDTagInfo", mock.AnythingOfType("*auth.IDTagInfo")).Return(nil)

	result, err := manager.AuthorizeIDTag("test-tag")

	assert.NoError(t, err)
	assert.Equal(t, types.AuthorizationStatusAccepted, result.Status)
	mockState.AssertExpectations(t)
}

func TestIDTagManager_AuthorizeIDTag_CachedEntry(t *testing.T) {
	mockState := new(MockBusinessState)
	config := &auth.AuthConfig{
		DefaultAuthStatus: types.AuthorizationStatusAccepted,
		CacheExpiry:      time.Hour,
	}

	manager := auth.NewIDTagManager(mockState, config)

	// Mock cached entry
	cachedInfo := &auth.IDTagInfo{
		IDTag:    "test-tag",
		Status:   types.AuthorizationStatusAccepted,
		CachedAt: time.Now(),
		Source:   "cache",
	}
	mockState.On("GetIDTagInfo", "test-tag").Return(cachedInfo, nil)

	result, err := manager.AuthorizeIDTag("test-tag")

	assert.NoError(t, err)
	assert.Equal(t, types.AuthorizationStatusAccepted, result.Status)
	mockState.AssertExpectations(t)
}

func TestIDTagManager_GenerateIDTag(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := auth.NewIDTagManager(mockState, nil)

	tag1 := manager.GenerateIDTag()
	tag2 := manager.GenerateIDTag()

	assert.NotEmpty(t, tag1)
	assert.NotEmpty(t, tag2)
	assert.NotEqual(t, tag1, tag2)
	assert.Contains(t, tag1, "TAG-")
}
```

**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/tests/data_transfer_test.go`

```go
package tests

import (
	"testing"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"ocpp-server/handlers"
)

func TestDataTransferManager_HandleDataTransfer_DefaultHandler(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := handlers.NewDataTransferManager(mockState)

	vendorID := "test-vendor"
	messageID := "test-message"
	data := "test-data"

	req := &core.DataTransferRequest{
		VendorId:  vendorID,
		MessageId: &messageID,
		Data:      &data,
	}

	response := manager.HandleDataTransfer("test-cp", req)

	assert.Equal(t, core.DataTransferStatusAccepted, response.Status)
}

func TestDataTransferManager_RegisterVendorHandler(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := handlers.NewDataTransferManager(mockState)

	mockHandler := &MockVendorHandler{}
	vendorID := "custom-vendor"

	manager.RegisterVendorHandler(vendorID, mockHandler)

	messageID := "test-message"
	data := "test-data"
	req := &core.DataTransferRequest{
		VendorId:  vendorID,
		MessageId: &messageID,
		Data:      &data,
	}

	expectedResponse := core.NewDataTransferConfirmation(core.DataTransferStatusAccepted, &data)
	mockHandler.On("HandleData", "test-cp", &messageID, &data).Return(expectedResponse, nil)

	response := manager.HandleDataTransfer("test-cp", req)

	assert.Equal(t, core.DataTransferStatusAccepted, response.Status)
	mockHandler.AssertExpectations(t)
}

type MockVendorHandler struct {
	mock.Mock
}

func (m *MockVendorHandler) HandleData(chargePointID string, messageId *string, data *string) (*core.DataTransferConfirmation, error) {
	args := m.Called(chargePointID, messageId, data)
	return args.Get(0).(*core.DataTransferConfirmation), args.Error(1)
}
```

#### Task 1.1.5: Integration Tests
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/tests/integration/authorize_test.go`

```go
package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthorizeIntegration(t *testing.T) {
	// Setup test server and client
	server, client, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test data
	testIDTag := "TEST-AUTH-001"

	// Create authorize request
	request := core.NewAuthorizeRequest(testIDTag)

	// Send request through client
	response, err := client.SendRequest(request)
	require.NoError(t, err)

	// Verify response
	authResponse, ok := response.(*core.AuthorizeConfirmation)
	require.True(t, ok, "Expected AuthorizeConfirmation")

	assert.Equal(t, types.AuthorizationStatusAccepted, authResponse.IdTagInfo.Status)

	// Verify server state was updated
	time.Sleep(100 * time.Millisecond) // Allow for async processing

	// Check that ID tag was cached
	cachedInfo, err := server.businessState.GetIDTagInfo(testIDTag)
	require.NoError(t, err)
	assert.NotNil(t, cachedInfo)
	assert.Equal(t, testIDTag, cachedInfo.IDTag)
}
```

### External Testing Scripts

#### Task 1.1.6: Create Test Scripts
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/scripts/test_authorize.sh`

```bash
#!/bin/bash

# Test script for Authorize functionality
# Usage: ./test_authorize.sh [server_url]

SERVER_URL=${1:-"http://localhost:8083"}
CLIENT_ID="TEST-CP-AUTH"

echo "Testing Authorize functionality..."

# Test 1: Generate test ID tag
echo "1. Generating test ID tag..."
ID_TAG=$(curl -s "${SERVER_URL}/api/v1/test/generate-id-tag" | jq -r '.data.idTag')
echo "Generated ID tag: $ID_TAG"

# Test 2: Test authorization via REST API (if available)
echo "2. Testing ID tag authorization..."
AUTH_RESULT=$(curl -s -X POST "${SERVER_URL}/api/v1/test/authorize" \
  -H "Content-Type: application/json" \
  -d "{\"idTag\": \"$ID_TAG\"}")
echo "Authorization result: $AUTH_RESULT"

# Test 3: Check ID tag cache
echo "3. Checking ID tag cache..."
CACHE_RESULT=$(curl -s "${SERVER_URL}/api/v1/test/id-tags/${ID_TAG}")
echo "Cache result: $CACHE_RESULT"

echo "Authorize testing complete!"
```

**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/scripts/test_data_transfer.py`

```python
#!/usr/bin/env python3
"""
Test script for DataTransfer functionality
Simulates OCPP client sending DataTransfer requests
"""

import asyncio
import json
import websockets
import uuid
from datetime import datetime

class OCPPClient:
    def __init__(self, server_url, client_id):
        self.server_url = server_url
        self.client_id = client_id
        self.websocket = None

    async def connect(self):
        uri = f"{self.server_url}/{self.client_id}"
        self.websocket = await websockets.connect(uri, subprotocols=["ocpp1.6"])
        print(f"Connected to {uri}")

    async def send_data_transfer(self, vendor_id, message_id=None, data=None):
        request_id = str(uuid.uuid4())

        message = [
            2,  # Call message type
            request_id,
            "DataTransfer",
            {
                "vendorId": vendor_id,
                "messageId": message_id,
                "data": data
            }
        ]

        await self.websocket.send(json.dumps(message))
        print(f"Sent DataTransfer: {message}")

        # Wait for response
        response = await self.websocket.recv()
        response_data = json.loads(response)
        print(f"Received response: {response_data}")

        return response_data

    async def disconnect(self):
        if self.websocket:
            await self.websocket.close()

async def test_data_transfer():
    client = OCPPClient("ws://localhost:8080", "TEST-CP-DT")

    try:
        await client.connect()

        # Test 1: Simple data transfer
        print("\n=== Test 1: Simple DataTransfer ===")
        await client.send_data_transfer("test-vendor", "ping", "hello")

        # Test 2: Data transfer with JSON data
        print("\n=== Test 2: DataTransfer with JSON ===")
        json_data = json.dumps({"temperature": 25.5, "humidity": 60})
        await client.send_data_transfer("sensor-vendor", "status", json_data)

        # Test 3: Data transfer without optional fields
        print("\n=== Test 3: Minimal DataTransfer ===")
        await client.send_data_transfer("minimal-vendor")

    finally:
        await client.disconnect()

if __name__ == "__main__":
    asyncio.run(test_data_transfer())
```

### Documentation

#### Task 1.1.7: API Documentation
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/docs/authorization.md`

```markdown
# Authorization System

## Overview
The OCPP server implements a flexible authorization system for ID tag validation with caching and external service integration.

## Components

### IDTagManager
Manages ID tag authorization with the following features:
- Local caching for performance
- Configurable authorization policies
- External service integration (future)
- Audit trail logging

### Configuration
```go
type AuthConfig struct {
    DefaultAuthStatus types.AuthorizationStatus // Default auth status for unknown tags
    CacheExpiry      time.Duration             // How long to cache auth results
    AllowUnknownTags bool                      // Whether to accept unknown tags
}
```

## OCPP Messages Supported

### Authorize
**Request**: ID tag to validate
**Response**: Authorization status (Accepted/Invalid/Expired/Blocked/ConcurrentTx)

Example:
```json
{
  "messageType": 2,
  "messageId": "12345",
  "action": "Authorize",
  "payload": {
    "idTag": "TAG-ABC123"
  }
}
```

**Response**:
```json
{
  "messageType": 3,
  "messageId": "12345",
  "payload": {
    "idTagInfo": {
      "status": "Accepted"
    }
  }
}
```

## REST API Endpoints (Testing)

### Generate Test ID Tag
`GET /api/v1/test/generate-id-tag`

Returns a new random ID tag for testing purposes.

### Test Authorization
`POST /api/v1/test/authorize`
```json
{
  "idTag": "TAG-ABC123"
}
```

Returns authorization result without OCPP message overhead.

## Data Transfer System

### Overview
Handles vendor-specific data exchange between charge points and the server.

### Supported Vendors
- `default`: Echo handler for testing
- Custom vendors can be registered via `RegisterVendorHandler()`

### Message Format
```json
{
  "vendorId": "vendor-name",
  "messageId": "optional-message-type",
  "data": "vendor-specific-data"
}
```

### Audit Trail
All data transfer requests are logged with:
- Charge point ID
- Vendor ID
- Message ID and data
- Response data
- Processing status
- Timestamp
```

### Health Check Documentation
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/docs/testing.md`

```markdown
# Testing Guide - Chunk 1.1

## Unit Tests
Run unit tests for authorization and data transfer:
```bash
cd /Users/chrishome/development/home/mcp-access/csms/ocpp-server
go test ./tests/... -v
```

## Integration Tests
Run integration tests with real Redis:
```bash
# Start test environment
docker-compose up redis -d

# Run integration tests
go test ./tests/integration/... -v

# Cleanup
docker-compose down
```

## External Testing
Test the server with real OCPP clients:

### Prerequisites
1. Server running on port 8083 (HTTP API) and 8080 (WebSocket)
2. Redis running on port 6379
3. Python 3.7+ with websockets library for data transfer tests

### Test Scripts
```bash
# Test authorization
./scripts/test_authorize.sh

# Test data transfer (requires Python)
python3 scripts/test_data_transfer.py
```

## Validation Criteria

### ✅ Chunk 1.1 Complete When:
1. **Unit Tests Pass**: All auth and data transfer unit tests pass
2. **Integration Tests Pass**: Real OCPP message flow works
3. **External Scripts Pass**: External test scripts complete successfully
4. **Redis Storage**: ID tag cache entries visible in Redis DB
5. **Logs Show**: Proper request/response logging for both message types
6. **Error Handling**: Invalid requests return appropriate error responses

### Manual Verification
1. Check Redis for cached ID tags: `redis-cli HGETALL "ocpp:idtag:TAG-ABC123"`
2. Verify logs show authorization attempts
3. Confirm data transfer requests are logged and responded to
4. Test with unknown ID tags and verify default behavior

## Common Issues
- **Redis Connection**: Ensure Redis is running and accessible
- **WebSocket Connection**: Verify charge point can connect to port 8080
- **OCPP Compliance**: Messages must follow OCPP 1.6 specification exactly
```

### Approval Criteria

#### Task 1.1.8: Validation Checklist
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/CHUNK_1_1_APPROVAL.md`

```markdown
# Chunk 1.1 Approval Checklist

## Implementation Complete ✓
- [ ] IDTagManager implemented with caching
- [ ] DataTransferManager with vendor handler support
- [ ] OCPP handlers added to main server
- [ ] Redis business state integration
- [ ] Error handling and logging

## Tests Complete ✓
- [ ] Unit tests for IDTagManager (cache hit/miss, generation)
- [ ] Unit tests for DataTransferManager (default/custom handlers)
- [ ] Integration tests with real OCPP messages
- [ ] Mock business state for isolated testing

## External Validation ✓
- [ ] test_authorize.sh script runs successfully
- [ ] test_data_transfer.py connects and exchanges messages
- [ ] Redis shows cached ID tag entries
- [ ] Server logs show proper message processing

## Documentation ✓
- [ ] Authorization system documented
- [ ] Data transfer system documented
- [ ] API endpoints documented
- [ ] Testing procedures documented

## Performance ✓
- [ ] ID tag authorization < 100ms response time
- [ ] Data transfer handling < 50ms
- [ ] Redis cache hit ratio > 90% after warm-up
- [ ] No memory leaks during extended testing

## Error Handling ✓
- [ ] Invalid ID tags return appropriate status
- [ ] Unknown vendor data transfer accepted gracefully
- [ ] Redis connection failures don't crash server
- [ ] Malformed OCPP messages rejected properly

## Ready for Chunk 1.2 ✓
- [ ] All above criteria met
- [ ] No blocking issues found
- [ ] Performance acceptable
- [ ] Team approval obtained

**Approval Date**: _____________
**Approved By**: _____________
**Notes**: _____________
```

## Execution Instructions for Agent

### Prerequisites Check
1. Verify current directory: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server`
2. Ensure Redis is running and accessible
3. Confirm basic OCPP server is functional

### Implementation Order
1. **Create auth package** (Task 1.1.1) - 45 minutes
2. **Create data transfer handler** (Task 1.1.2) - 45 minutes
3. **Update main server** (Task 1.1.3) - 30 minutes
4. **Write unit tests** (Task 1.1.4) - 60 minutes
5. **Write integration tests** (Task 1.1.5) - 45 minutes
6. **Create test scripts** (Task 1.1.6) - 30 minutes
7. **Write documentation** (Task 1.1.7) - 30 minutes
8. **Validate and approve** (Task 1.1.8) - 30 minutes

**Total Time**: ~5 hours

### Success Criteria
- All tests pass
- External scripts run successfully
- Documentation is complete and accurate
- System performance meets requirements
- Ready to proceed to Chunk 1.2

### Failure Criteria
- Any test failures
- Performance issues
- Missing documentation
- External validation failures

**Do not proceed to next chunk until all approval criteria are met.**