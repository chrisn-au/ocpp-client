# Task 1: Remote Transaction Control
## REST API Endpoints for RemoteStart/RemoteStop

### Overview
Implement REST API endpoints that trigger OCPP RemoteStartTransaction and RemoteStopTransaction commands. This task focuses on the external API layer that allows CSMS systems to initiate transactions remotely.

### Project Context
**Working Directory**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/`

**Existing Code Structure**:
- `main.go` - Contains complete OCPP server with Redis ServerState, HTTP API, and handlers
- `CRITICAL_ISSUES.md` - Documents Redis ServerState correlation bug (FIXED)
- Local `ocpp-go` library at `../ocpp-go/` with Redis transport implementation
- `config/` directory with configuration management
- `handlers/` directory with meter value processing

**Current Server Capabilities**:
- Redis-backed distributed ServerState (correlation bug FIXED)
- HTTP REST API with Gorilla Mux router on port 8083
- Boot notification and heartbeat handlers
- Live configuration endpoints (`/api/v1/chargepoints/{clientId}/configuration/live`)
- Meter value processing integration
- Validator for request validation (already imported)

**Key Components in main.go**:
- `OCPPServer` struct with `centralSystem *ocppj.Server`, `redisClient *redis.Client`, `router *mux.Router`
- Existing handlers: `handleBootNotification`, `handleHeartbeat`, `handleStatusNotification`
- Configuration handlers: `handleGetLiveConfiguration`, `handleMeterValues`
- Helper functions: `sendJSONResponse`, `sendErrorResponse`
- Router setup in `setupRoutes()` function

**Critical Context**:
This task specifically tests the Redis ServerState correlation mechanism that was recently fixed. The bug caused request-response correlation failures where valid CALL_RESULT responses were being discarded. This implementation will verify the fix works correctly for server-initiated commands.

### Files to Modify/Create
- `main.go` - Add route handlers and helper functions
- Test with existing ServerState implementation

---

## Implementation

### 1. Add Remote Start Transaction Endpoint

Add this handler to `main.go`:

```go
type RemoteStartRequest struct {
    ClientID     string  `json:"clientId" validate:"required"`
    ConnectorID  *int    `json:"connectorId,omitempty"`
    IdTag        string  `json:"idTag" validate:"required"`
}

type RemoteTransactionResult struct {
    RequestID   string `json:"requestId"`
    ClientID    string `json:"clientId"`
    ConnectorID int    `json:"connectorId"`
    Status      string `json:"status"` // "accepted", "rejected", "timeout", "error"
    Message     string `json:"message"`
}

func (s *OCPPServer) handleRemoteStartTransaction(w http.ResponseWriter, r *http.Request) {
    var req RemoteStartRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
        return
    }

    if err := s.validator.Struct(req); err != nil {
        s.sendErrorResponse(w, http.StatusBadRequest, "Validation failed", err)
        return
    }

    // Check if client is connected
    if !s.isClientConnected(req.ClientID) {
        s.sendErrorResponse(w, http.StatusNotFound, "Client not connected", nil)
        return
    }

    // Default connector ID to 1 if not specified
    connectorID := 1
    if req.ConnectorID != nil {
        connectorID = *req.ConnectorID
    }

    // Create OCPP RemoteStartTransaction request
    request := remotetrigger.NewRemoteStartTransactionRequest(req.IdTag)
    request.ConnectorId = &connectorID

    log.Printf("REMOTE_START: Sending RemoteStartTransaction to %s - Connector: %d, IdTag: %s",
        req.ClientID, connectorID, req.IdTag)

    // Send request with timeout
    requestID := generateRequestID()
    result, err := s.sendRequestWithTimeout(req.ClientID, request, 30*time.Second)

    response := RemoteTransactionResult{
        RequestID:   requestID,
        ClientID:    req.ClientID,
        ConnectorID: connectorID,
    }

    if err != nil {
        log.Printf("REMOTE_START: Request failed for %s: %v", req.ClientID, err)
        response.Status = "timeout"
        response.Message = fmt.Sprintf("Request timeout: %v", err)
        s.sendJSONResponse(w, http.StatusRequestTimeout, response)
        return
    }

    confirmation := result.(*remotetrigger.RemoteStartTransactionConfirmation)

    if confirmation.Status == remotetrigger.RemoteStartStopStatusAccepted {
        response.Status = "accepted"
        response.Message = "RemoteStartTransaction accepted by charge point"
        log.Printf("REMOTE_START: Accepted by %s", req.ClientID)
    } else {
        response.Status = "rejected"
        response.Message = "RemoteStartTransaction rejected by charge point"
        log.Printf("REMOTE_START: Rejected by %s", req.ClientID)
    }

    s.sendJSONResponse(w, http.StatusOK, response)
}
```

### 2. Add Remote Stop Transaction Endpoint

Add this handler to `main.go`:

```go
type RemoteStopRequest struct {
    TransactionID int `json:"transactionId" validate:"required,min=1"`
}

func (s *OCPPServer) handleRemoteStopTransaction(w http.ResponseWriter, r *http.Request) {
    var req RemoteStopRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
        return
    }

    if err := s.validator.Struct(req); err != nil {
        s.sendErrorResponse(w, http.StatusBadRequest, "Validation failed", err)
        return
    }

    // Find which client has this transaction
    clientID, err := s.findClientByTransactionID(req.TransactionID)
    if err != nil {
        s.sendErrorResponse(w, http.StatusNotFound, "Transaction not found", err)
        return
    }

    // Check if client is connected
    if !s.isClientConnected(clientID) {
        s.sendErrorResponse(w, http.StatusServiceUnavailable, "Client not connected", nil)
        return
    }

    // Create OCPP RemoteStopTransaction request
    request := remotetrigger.NewRemoteStopTransactionRequest(req.TransactionID)

    log.Printf("REMOTE_STOP: Sending RemoteStopTransaction to %s - Transaction: %d",
        clientID, req.TransactionID)

    // Send request with timeout
    requestID := generateRequestID()
    result, err := s.sendRequestWithTimeout(clientID, request, 30*time.Second)

    response := RemoteTransactionResult{
        RequestID:   requestID,
        ClientID:    clientID,
        ConnectorID: 0, // Will be filled from transaction data if needed
    }

    if err != nil {
        log.Printf("REMOTE_STOP: Request failed for %s: %v", clientID, err)
        response.Status = "timeout"
        response.Message = fmt.Sprintf("Request timeout: %v", err)
        s.sendJSONResponse(w, http.StatusRequestTimeout, response)
        return
    }

    confirmation := result.(*remotetrigger.RemoteStopTransactionConfirmation)

    if confirmation.Status == remotetrigger.RemoteStartStopStatusAccepted {
        response.Status = "accepted"
        response.Message = "RemoteStopTransaction accepted by charge point"
        log.Printf("REMOTE_STOP: Accepted by %s for transaction %d", clientID, req.TransactionID)
    } else {
        response.Status = "rejected"
        response.Message = "RemoteStopTransaction rejected by charge point"
        log.Printf("REMOTE_STOP: Rejected by %s for transaction %d", clientID, req.TransactionID)
    }

    s.sendJSONResponse(w, http.StatusOK, response)
}
```

### 3. Add Helper Functions

Add these helper functions to `main.go`:

```go
// Check if client is connected
func (s *OCPPServer) isClientConnected(clientID string) bool {
    // Check Redis for client connection status
    key := fmt.Sprintf("ocpp:client:connected:%s", clientID)
    result, err := s.redisClient.Get(context.Background(), key).Result()
    if err != nil {
        return false
    }
    return result == "true"
}

// Find client by transaction ID
func (s *OCPPServer) findClientByTransactionID(transactionID int) (string, error) {
    // Search Redis for transaction mapping
    pattern := "ocpp:transaction:*"
    keys, err := s.redisClient.Keys(context.Background(), pattern).Result()
    if err != nil {
        return "", err
    }

    for _, key := range keys {
        data, err := s.redisClient.HGetAll(context.Background(), key).Result()
        if err != nil {
            continue
        }

        for txnID, clientData := range data {
            if txnID == fmt.Sprintf("%d", transactionID) {
                // Extract client ID from key or data
                parts := strings.Split(key, ":")
                if len(parts) >= 3 {
                    return parts[2], nil
                }
            }
        }
    }

    return "", fmt.Errorf("transaction %d not found", transactionID)
}

// Send request with timeout using existing ServerState
func (s *OCPPServer) sendRequestWithTimeout(clientID string, request ocpp.Request, timeout time.Duration) (ocpp.Response, error) {
    // Use the existing SendRequestAsync method with timeout
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()

    responseChan := make(chan ocpp.Response, 1)
    errorChan := make(chan error, 1)

    go func() {
        response, err := s.centralSystem.SendRequestAsync(clientID, request, func(response ocpp.Response, err error) {
            if err != nil {
                errorChan <- err
            } else {
                responseChan <- response
            }
        })
        if response != nil {
            responseChan <- response
        }
    }()

    select {
    case response := <-responseChan:
        return response, nil
    case err := <-errorChan:
        return nil, err
    case <-ctx.Done():
        return nil, fmt.Errorf("request timeout after %v", timeout)
    }
}

// Generate unique request ID
func generateRequestID() string {
    return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
```

### 4. Add Routes to Router

Add these routes in the `setupRoutes()` function in `main.go`:

```go
func (s *OCPPServer) setupRoutes() {
    // ... existing routes ...

    // Remote transaction control endpoints
    s.router.HandleFunc("/api/v1/transactions/remote-start", s.handleRemoteStartTransaction).Methods("POST")
    s.router.HandleFunc("/api/v1/transactions/remote-stop", s.handleRemoteStopTransaction).Methods("POST")

    // ... rest of existing routes ...
}
```

### 5. Update Client Connection Tracking

Modify the `handleBootNotification` to track connection status:

```go
func (s *OCPPServer) handleBootNotification(clientID string, request *core.BootNotificationRequest) (*core.BootNotificationConfirmation, error) {
    log.Printf("BOOT: Boot notification from %s - Model: %s, Vendor: %s, Serial: %s",
        clientID, request.ChargePointModel, request.ChargePointVendor, request.ChargePointSerialNumber)

    // Store client connection status in Redis
    connectionKey := fmt.Sprintf("ocpp:client:connected:%s", clientID)
    err := s.redisClient.Set(context.Background(), connectionKey, "true", 24*time.Hour).Err()
    if err != nil {
        log.Printf("ERROR: Failed to store connection status for %s: %v", clientID, err)
    }

    // Store boot notification data
    bootDataKey := fmt.Sprintf("ocpp:boot:%s", clientID)
    bootData := map[string]interface{}{
        "chargePointModel":        request.ChargePointModel,
        "chargePointVendor":       request.ChargePointVendor,
        "chargePointSerialNumber": request.ChargePointSerialNumber,
        "firmwareVersion":         request.FirmwareVersion,
        "lastBootTime":            time.Now().UTC(),
    }

    bootJSON, _ := json.Marshal(bootData)
    s.redisClient.Set(context.Background(), bootDataKey, bootJSON, 24*time.Hour)

    // Return accepted status with current time
    return core.NewBootNotificationConfirmation(core.RegistrationStatusAccepted, time.Now().UTC(), 300), nil
}
```

---

## Testing

### Test Remote Start Transaction
```bash
# Test remote start transaction
curl -X POST http://localhost:8083/api/v1/transactions/remote-start \
  -H "Content-Type: application/json" \
  -d '{
    "clientId": "TEST-CP-001",
    "connectorId": 1,
    "idTag": "USER123"
  }'

# Expected successful response:
{
  "requestId": "req_1663234567890",
  "clientId": "TEST-CP-001",
  "connectorId": 1,
  "status": "accepted",
  "message": "RemoteStartTransaction accepted by charge point"
}
```

### Test Remote Stop Transaction
```bash
# Test remote stop transaction (use transaction ID from StartTransaction response)
curl -X POST http://localhost:8083/api/v1/transactions/remote-stop \
  -H "Content-Type: application/json" \
  -d '{
    "transactionId": 12345
  }'

# Expected successful response:
{
  "requestId": "req_1663234567891",
  "clientId": "TEST-CP-001",
  "connectorId": 0,
  "status": "accepted",
  "message": "RemoteStopTransaction accepted by charge point"
}
```

### Test Error Cases
```bash
# Test with disconnected client
curl -X POST http://localhost:8083/api/v1/transactions/remote-start \
  -H "Content-Type: application/json" \
  -d '{
    "clientId": "OFFLINE-CP",
    "idTag": "USER123"
  }'

# Expected error response:
{
  "success": false,
  "error": "Client not connected",
  "message": "Client not connected"
}
```

---

## ServerState Testing Points

This implementation will test the ServerState thoroughly by:

1. **Request Correlation**: RemoteStart/RemoteStop requests test the Redis correlation mechanism
2. **Distributed State**: Multiple server instances can handle remote requests
3. **Message Parsing**: Tests that responses are properly correlated with requests
4. **Timeout Handling**: Tests timeout behavior when charge points don't respond
5. **Connection Tracking**: Tests Redis-based client connection management

## Next Steps

After implementing this task:
1. Test with a connected charge point simulator
2. Verify Redis keys are created/cleaned up properly
3. Test timeout scenarios by disconnecting charge point
4. Verify multiple concurrent requests work correctly
5. Ready for Task 2: Enhanced Local Transaction Handlers