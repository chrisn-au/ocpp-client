# Phase 2: Remote Transaction Control - Chunk 2.1
## Agent Execution Plan for Transaction Management Implementation

### Overview
Implement comprehensive remote transaction control capabilities including server-initiated commands, transaction state management, and REST API endpoints for external system integration. This builds on the existing configuration and meter value infrastructure.

### Prerequisites
- Phase 1 configuration management completed
- Meter value processing infrastructure in place
- Redis distributed state operational
- MongoDB connection established

---

## Implementation Tasks

### Task 1: Transaction State Management
**File**: `transaction_state.go`
**Estimated Time**: 45 minutes

#### Create Transaction State Structures
```go
type TransactionState struct {
    TransactionID    int       `json:"transactionId" bson:"transactionId"`
    ClientID         string    `json:"clientId" bson:"clientId"`
    ConnectorID      int       `json:"connectorId" bson:"connectorId"`
    IdTag            string    `json:"idTag" bson:"idTag"`
    StartTime        time.Time `json:"startTime" bson:"startTime"`
    EndTime          *time.Time `json:"endTime,omitempty" bson:"endTime,omitempty"`
    StartMeterValue  int       `json:"startMeterValue" bson:"startMeterValue"`
    EndMeterValue    *int      `json:"endMeterValue,omitempty" bson:"endMeterValue,omitempty"`
    Status           string    `json:"status" bson:"status"` // "active", "stopped", "completed"
    ReservationID    *int      `json:"reservationId,omitempty" bson:"reservationId,omitempty"`
    CreatedAt        time.Time `json:"createdAt" bson:"createdAt"`
    UpdatedAt        time.Time `json:"updatedAt" bson:"updatedAt"`
}

type TransactionManager struct {
    redisClient   *redis.Client
    mongoClient   *mongo.Client
    database      string
    collection    string
    mutex         sync.RWMutex
}

type RemoteStartResult struct {
    RequestID    string `json:"requestId"`
    ClientID     string `json:"clientId"`
    ConnectorID  int    `json:"connectorId"`
    Status       string `json:"status"` // "accepted", "rejected", "timeout"
    TransactionID *int  `json:"transactionId,omitempty"`
    Message      string `json:"message"`
}
```

#### Implement Core Transaction Methods
```go
func NewTransactionManager(redis *redis.Client, mongo *mongo.Client, database string) *TransactionManager
func (tm *TransactionManager) StartTransaction(clientID string, connectorID int, idTag string, meterValue int, reservationID *int) (*TransactionState, error)
func (tm *TransactionManager) StopTransaction(transactionID int, meterValue int, reason string) error
func (tm *TransactionManager) GetActiveTransaction(clientID string, connectorID int) (*TransactionState, error)
func (tm *TransactionManager) GetTransactionByID(transactionID int) (*TransactionState, error)
func (tm *TransactionManager) UpdateTransactionMeterValue(transactionID int, meterValue int) error
func (tm *TransactionManager) GetTransactionHistory(clientID string, limit int, offset int) ([]*TransactionState, error)
```

#### Redis Keys for Transaction State
```go
const (
    RedisActiveTransactionKey = "ocpp:transaction:active:%s:%d"     // clientID:connectorID
    RedisTransactionDataKey   = "ocpp:transaction:data:%d"          // transactionID
    RedisClientTransactionsKey = "ocpp:transactions:client:%s"     // clientID (sorted set)
)
```

---

### Task 2: Remote Start Transaction Implementation
**File**: `remote_start.go`
**Estimated Time**: 60 minutes

#### Remote Start Request Handler
```go
type RemoteStartTransactionRequest struct {
    ClientID     string  `json:"clientId" validate:"required"`
    ConnectorID  *int    `json:"connectorId,omitempty"`
    IdTag        string  `json:"idTag" validate:"required"`
    ChargingProfile *types.ChargingProfile `json:"chargingProfile,omitempty"`
}

func (s *OCPPServer) handleRemoteStartTransaction(w http.ResponseWriter, r *http.Request) {
    var req RemoteStartTransactionRequest
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

    // Determine connector ID if not specified
    connectorID := 1
    if req.ConnectorID != nil {
        connectorID = *req.ConnectorID
    }

    // Check if connector is available
    if !s.isConnectorAvailable(req.ClientID, connectorID) {
        s.sendErrorResponse(w, http.StatusConflict, "Connector not available", nil)
        return
    }

    // Create OCPP RemoteStartTransaction request
    request := remotetrigger.NewRemoteStartTransactionRequest(req.IdTag)
    request.ConnectorId = &connectorID
    if req.ChargingProfile != nil {
        request.ChargingProfile = req.ChargingProfile
    }

    // Send request and wait for response
    result, err := s.sendRequestWithTimeout(req.ClientID, request, 30*time.Second)
    if err != nil {
        s.sendErrorResponse(w, http.StatusRequestTimeout, "Request timeout", err)
        return
    }

    response := result.(*remotetrigger.RemoteStartTransactionConfirmation)

    // Prepare response
    remoteStartResult := RemoteStartResult{
        RequestID:   generateRequestID(),
        ClientID:    req.ClientID,
        ConnectorID: connectorID,
        Status:      string(response.Status),
        Message:     "Remote start transaction request processed",
    }

    if response.Status == remotetrigger.RemoteStartStopStatusAccepted {
        // Transaction will be created when StartTransaction message arrives
        remoteStartResult.Status = "accepted"
        remoteStartResult.Message = "Transaction start accepted, waiting for confirmation"
    } else {
        remoteStartResult.Status = "rejected"
        remoteStartResult.Message = "Transaction start rejected by charge point"
    }

    s.sendJSONResponse(w, http.StatusOK, remoteStartResult)
}
```

#### Connector Availability Check
```go
func (s *OCPPServer) isConnectorAvailable(clientID string, connectorID int) bool {
    // Check current connector status
    statusKey := fmt.Sprintf("ocpp:status:%s:%d", clientID, connectorID)
    status, err := s.redisClient.Get(context.Background(), statusKey).Result()
    if err != nil {
        return false // Assume unavailable if status unknown
    }

    // Check if there's an active transaction
    activeTransaction, err := s.transactionManager.GetActiveTransaction(clientID, connectorID)
    if err == nil && activeTransaction != nil {
        return false // Connector has active transaction
    }

    // Available statuses for starting transaction
    availableStatuses := []string{"Available", "Preparing"}
    for _, availableStatus := range availableStatuses {
        if status == availableStatus {
            return true
        }
    }

    return false
}
```

---

### Task 3: Remote Stop Transaction Implementation
**File**: `remote_stop.go`
**Estimated Time**: 45 minutes

#### Remote Stop Request Handler
```go
type RemoteStopTransactionRequest struct {
    TransactionID int `json:"transactionId" validate:"required,min=1"`
}

func (s *OCPPServer) handleRemoteStopTransaction(w http.ResponseWriter, r *http.Request) {
    var req RemoteStopTransactionRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        s.sendErrorResponse(w, http.StatusBadRequest, "Invalid request body", err)
        return
    }

    if err := s.validator.Struct(req); err != nil {
        s.sendErrorResponse(w, http.StatusBadRequest, "Validation failed", err)
        return
    }

    // Get transaction details
    transaction, err := s.transactionManager.GetTransactionByID(req.TransactionID)
    if err != nil {
        s.sendErrorResponse(w, http.StatusNotFound, "Transaction not found", err)
        return
    }

    if transaction.Status != "active" {
        s.sendErrorResponse(w, http.StatusConflict, "Transaction not active", nil)
        return
    }

    // Check if client is connected
    if !s.isClientConnected(transaction.ClientID) {
        s.sendErrorResponse(w, http.StatusServiceUnavailable, "Client not connected", nil)
        return
    }

    // Create OCPP RemoteStopTransaction request
    request := remotetrigger.NewRemoteStopTransactionRequest(req.TransactionID)

    // Send request and wait for response
    result, err := s.sendRequestWithTimeout(transaction.ClientID, request, 30*time.Second)
    if err != nil {
        s.sendErrorResponse(w, http.StatusRequestTimeout, "Request timeout", err)
        return
    }

    response := result.(*remotetrigger.RemoteStopTransactionConfirmation)

    // Prepare response
    remoteStopResult := RemoteStartResult{
        RequestID:     generateRequestID(),
        ClientID:      transaction.ClientID,
        ConnectorID:   transaction.ConnectorID,
        TransactionID: &req.TransactionID,
        Status:        string(response.Status),
    }

    if response.Status == remotetrigger.RemoteStartStopStatusAccepted {
        remoteStopResult.Status = "accepted"
        remoteStopResult.Message = "Transaction stop accepted, waiting for confirmation"
    } else {
        remoteStopResult.Status = "rejected"
        remoteStopResult.Message = "Transaction stop rejected by charge point"
    }

    s.sendJSONResponse(w, http.StatusOK, remoteStopResult)
}
```

---

### Task 4: Enhanced Transaction Event Handlers
**File**: Update `main.go` transaction handlers
**Estimated Time**: 45 minutes

#### Enhanced StartTransaction Handler
```go
func (s *OCPPServer) handleStartTransaction(clientID string, request *core.StartTransactionRequest) (*core.StartTransactionConfirmation, error) {
    log.Printf("TRANSACTION: Start request from %s - Connector: %d, IdTag: %s, Meter: %d",
        clientID, request.ConnectorId, request.IdTag, request.MeterStart)

    // Validate IdTag authorization
    authorized, err := s.authorizeIdTag(request.IdTag)
    if err != nil {
        log.Printf("ERROR: Authorization check failed for %s: %v", request.IdTag, err)
        return core.NewStartTransactionConfirmation(types.NewIdTagInfo(authorization.AuthorizationStatusInvalid)), nil
    }

    if !authorized {
        log.Printf("TRANSACTION: IdTag %s not authorized", request.IdTag)
        return core.NewStartTransactionConfirmation(types.NewIdTagInfo(authorization.AuthorizationStatusInvalid)), nil
    }

    // Check connector availability
    if !s.isConnectorAvailable(clientID, request.ConnectorId) {
        log.Printf("TRANSACTION: Connector %d not available for %s", request.ConnectorId, clientID)
        return core.NewStartTransactionConfirmation(types.NewIdTagInfo(authorization.AuthorizationStatusBlocked)), nil
    }

    // Create transaction record
    transaction, err := s.transactionManager.StartTransaction(
        clientID,
        request.ConnectorId,
        request.IdTag,
        request.MeterStart,
        request.ReservationId,
    )
    if err != nil {
        log.Printf("ERROR: Failed to create transaction for %s: %v", clientID, err)
        return core.NewStartTransactionConfirmation(types.NewIdTagInfo(authorization.AuthorizationStatusInvalid)), nil
    }

    // Update connector status to "Charging"
    s.updateConnectorStatus(clientID, request.ConnectorId, "Charging", nil)

    // Publish transaction started event
    s.publishTransactionEvent("transaction.started", transaction)

    log.Printf("TRANSACTION: Started transaction %d for %s on connector %d",
        transaction.TransactionID, clientID, request.ConnectorId)

    return core.NewStartTransactionConfirmation(types.NewIdTagInfo(authorization.AuthorizationStatusAccepted)), nil
}
```

#### Enhanced StopTransaction Handler
```go
func (s *OCPPServer) handleStopTransaction(clientID string, request *core.StopTransactionRequest) (*core.StopTransactionConfirmation, error) {
    log.Printf("TRANSACTION: Stop request from %s - Transaction: %d, Meter: %d, Reason: %s",
        clientID, request.TransactionId, request.MeterStop, request.Reason)

    // Get transaction details
    transaction, err := s.transactionManager.GetTransactionByID(request.TransactionId)
    if err != nil {
        log.Printf("ERROR: Transaction %d not found: %v", request.TransactionId, err)
        return core.NewStopTransactionConfirmation(), nil
    }

    // Validate transaction belongs to this client
    if transaction.ClientID != clientID {
        log.Printf("ERROR: Transaction %d does not belong to client %s", request.TransactionId, clientID)
        return core.NewStopTransactionConfirmation(), nil
    }

    // Stop transaction
    err = s.transactionManager.StopTransaction(request.TransactionId, request.MeterStop, string(request.Reason))
    if err != nil {
        log.Printf("ERROR: Failed to stop transaction %d: %v", request.TransactionId, err)
        return core.NewStopTransactionConfirmation(), nil
    }

    // Process meter values if provided
    if request.TransactionData != nil {
        for _, meterValue := range request.TransactionData {
            s.processMeterValue(clientID, transaction.ConnectorID, meterValue, &request.TransactionId)
        }
    }

    // Update connector status to "Available"
    s.updateConnectorStatus(clientID, transaction.ConnectorID, "Available", nil)

    // Calculate session statistics
    energyConsumed := request.MeterStop - transaction.StartMeterValue
    duration := time.Since(transaction.StartTime)

    // Publish transaction stopped event
    s.publishTransactionEvent("transaction.stopped", map[string]interface{}{
        "transaction":     transaction,
        "endMeterValue":   request.MeterStop,
        "energyConsumed":  energyConsumed,
        "duration":        duration.Seconds(),
        "stopReason":      string(request.Reason),
    })

    log.Printf("TRANSACTION: Stopped transaction %d for %s - Energy: %d Wh, Duration: %.0f seconds",
        request.TransactionId, clientID, energyConsumed, duration.Seconds())

    return core.NewStopTransactionConfirmation(), nil
}
```

---

### Task 5: Transaction Query API Endpoints
**File**: `transaction_api.go`
**Estimated Time**: 30 minutes

#### Transaction Query Endpoints
```go
// GET /api/v1/transactions/active
func (s *OCPPServer) handleGetActiveTransactions(w http.ResponseWriter, r *http.Request) {
    clientID := r.URL.Query().Get("clientId")

    var transactions []*TransactionState
    var err error

    if clientID != "" {
        // Get active transactions for specific client
        for connectorID := 1; connectorID <= 10; connectorID++ {
            transaction, err := s.transactionManager.GetActiveTransaction(clientID, connectorID)
            if err == nil && transaction != nil {
                transactions = append(transactions, transaction)
            }
        }
    } else {
        // Get all active transactions (implement in TransactionManager)
        transactions, err = s.transactionManager.GetAllActiveTransactions()
        if err != nil {
            s.sendErrorResponse(w, http.StatusInternalServerError, "Failed to get active transactions", err)
            return
        }
    }

    s.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
        "success": true,
        "data": map[string]interface{}{
            "transactions": transactions,
            "count":        len(transactions),
        },
    })
}

// GET /api/v1/transactions/{id}
func (s *OCPPServer) handleGetTransaction(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    transactionID, err := strconv.Atoi(vars["id"])
    if err != nil {
        s.sendErrorResponse(w, http.StatusBadRequest, "Invalid transaction ID", err)
        return
    }

    transaction, err := s.transactionManager.GetTransactionByID(transactionID)
    if err != nil {
        s.sendErrorResponse(w, http.StatusNotFound, "Transaction not found", err)
        return
    }

    s.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
        "success": true,
        "data":    transaction,
    })
}

// GET /api/v1/chargepoints/{clientId}/transactions
func (s *OCPPServer) handleGetClientTransactions(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    clientID := vars["clientId"]

    limit := 50
    offset := 0

    if l := r.URL.Query().Get("limit"); l != "" {
        if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
            limit = parsed
        }
    }

    if o := r.URL.Query().Get("offset"); o != "" {
        if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
            offset = parsed
        }
    }

    transactions, err := s.transactionManager.GetTransactionHistory(clientID, limit, offset)
    if err != nil {
        s.sendErrorResponse(w, http.StatusInternalServerError, "Failed to get transaction history", err)
        return
    }

    s.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
        "success": true,
        "data": map[string]interface{}{
            "transactions": transactions,
            "limit":        limit,
            "offset":       offset,
            "count":        len(transactions),
        },
    })
}
```

---

### Task 6: Router Integration and Testing
**File**: Update `main.go` router setup
**Estimated Time**: 20 minutes

#### Add Transaction Routes
```go
func (s *OCPPServer) setupRoutes() {
    // ... existing routes ...

    // Transaction control endpoints
    s.router.HandleFunc("/api/v1/transactions/remote-start", s.handleRemoteStartTransaction).Methods("POST")
    s.router.HandleFunc("/api/v1/transactions/remote-stop", s.handleRemoteStopTransaction).Methods("POST")

    // Transaction query endpoints
    s.router.HandleFunc("/api/v1/transactions/active", s.handleGetActiveTransactions).Methods("GET")
    s.router.HandleFunc("/api/v1/transactions/{id:[0-9]+}", s.handleGetTransaction).Methods("GET")
    s.router.HandleFunc("/api/v1/chargepoints/{clientId}/transactions", s.handleGetClientTransactions).Methods("GET")
}
```

#### Initialize Transaction Manager
```go
func main() {
    // ... existing initialization ...

    // Initialize transaction manager
    transactionManager := NewTransactionManager(redisClient, mongoClient, "ocpp_server")
    server.transactionManager = transactionManager

    // ... rest of main ...
}
```

---

## Testing Scenarios

### Test 1: Remote Start Transaction
```bash
# Test remote start
curl -X POST http://localhost:8083/api/v1/transactions/remote-start \
  -H "Content-Type: application/json" \
  -d '{
    "clientId": "TEST-CP-001",
    "connectorId": 1,
    "idTag": "USER123"
  }'

# Expected response
{
  "requestId": "req_abc123",
  "clientId": "TEST-CP-001",
  "connectorId": 1,
  "status": "accepted",
  "message": "Transaction start accepted, waiting for confirmation"
}
```

### Test 2: Remote Stop Transaction
```bash
# Test remote stop
curl -X POST http://localhost:8083/api/v1/transactions/remote-stop \
  -H "Content-Type: application/json" \
  -d '{
    "transactionId": 12345
  }'
```

### Test 3: Query Active Transactions
```bash
# Get all active transactions
curl http://localhost:8083/api/v1/transactions/active

# Get active transactions for specific client
curl http://localhost:8083/api/v1/transactions/active?clientId=TEST-CP-001
```

---

## MongoDB Schema

### Transactions Collection
```javascript
// transactions collection indexes
db.transactions.createIndex({ "clientId": 1, "createdAt": -1 })
db.transactions.createIndex({ "transactionId": 1 }, { unique: true })
db.transactions.createIndex({ "status": 1 })
db.transactions.createIndex({ "startTime": 1 })
db.transactions.createIndex({ "endTime": 1 })
```

---

## MQTT Event Integration

### Transaction Events
```go
// Publish transaction events for microservices integration
func (s *OCPPServer) publishTransactionEvent(eventType string, data interface{}) {
    event := map[string]interface{}{
        "timestamp": time.Now().UTC(),
        "eventType": eventType,
        "source":    "ocpp-server",
        "data":      data,
    }

    eventJSON, _ := json.Marshal(event)
    topic := fmt.Sprintf("ocpp/transactions/%s", eventType)

    s.mqttClient.Publish(topic, 1, false, eventJSON)
}
```

---

## Error Handling

### Common Error Scenarios
1. **Client Not Connected**: Return 404 with clear message
2. **Connector Occupied**: Return 409 conflict status
3. **Invalid Transaction**: Return 404 for unknown transaction IDs
4. **Timeout**: Return 408 for unresponsive charge points
5. **Authorization Failed**: Return 403 for unauthorized requests

### Retry Logic
- Implement exponential backoff for failed requests
- Store failed requests for later retry when client reconnects
- Provide webhook notifications for persistent failures

---

## Performance Considerations

1. **Redis Optimization**: Use pipelining for bulk transaction queries
2. **MongoDB Indexing**: Ensure proper indexes for transaction queries
3. **Concurrent Requests**: Handle multiple simultaneous remote start requests per client
4. **Memory Management**: Clean up completed transaction state periodically
5. **Request Timeouts**: Configurable timeouts for different transaction operations

---

## Integration Points

### With Configuration Management
- Use configuration values for transaction timeouts
- Respect authorization settings and connector limits
- Apply power management profiles during transactions

### With Meter Values
- Link meter value processing to active transactions
- Calculate real-time energy consumption during transactions
- Generate alerts for unusual transaction patterns

### With External Systems
- MQTT events for billing system integration
- Webhook notifications for transaction state changes
- REST API endpoints for CSMS integration

---

**Estimated Total Implementation Time**: 3.5 hours
**Dependencies**: Configuration management, meter value processing
**Testing Requirements**: Unit tests, integration tests with Redis/MongoDB
**Documentation**: API endpoint documentation, transaction flow diagrams