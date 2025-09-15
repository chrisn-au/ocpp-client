# Task 2: Enhanced Local Transaction Handlers
## StartTransaction and StopTransaction Message Processing

### Overview
Enhance the existing StartTransaction and StopTransaction handlers to properly manage transaction state in Redis, track active transactions, and provide comprehensive transaction lifecycle management. This works with the OCPP messages that charge points send to start/stop transactions locally.

### Project Context
**Working Directory**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/`

**Existing Code Base**:
- `main.go` contains basic `handleStartTransaction` and `handleStopTransaction` functions
- Redis client already initialized and available as `s.redisClient`
- Transaction ID counter mechanism needs implementation
- Meter value processing integration already exists (`handleMeterValues`)
- Gorilla Mux router with existing API endpoints

**Current Transaction Handlers** (in main.go around line 200+):
- `handleStartTransaction` - Basic implementation that returns confirmation
- `handleStopTransaction` - Basic implementation that logs stop requests
- These need enhancement for Redis state management and proper lifecycle tracking

**Available Infrastructure**:
- Redis connection established (`s.redisClient *redis.Client`)
- JSON response helpers (`sendJSONResponse`, `sendErrorResponse`)
- Validator for request validation
- Router setup in `setupRoutes()` for adding new endpoints
- Meter value processing in `handlers/` directory

**Integration Points**:
- Connects with Task 1 remote transaction control endpoints
- Links with existing meter value processing system
- Uses same Redis ServerState that had correlation bug (now FIXED)
- Builds on existing connector status tracking patterns

### Files to Modify
- `main.go` - Enhance existing transaction handlers and add helper functions

---

## Implementation

### 1. Enhanced StartTransaction Handler

Replace the existing `handleStartTransaction` function in `main.go`:

```go
func (s *OCPPServer) handleStartTransaction(clientID string, request *core.StartTransactionRequest) (*core.StartTransactionConfirmation, error) {
    log.Printf("TRANSACTION_START: Request from %s - Connector: %d, IdTag: %s, Meter: %d, Reservation: %v",
        clientID, request.ConnectorId, request.IdTag, request.MeterStart, request.ReservationId)

    // Generate unique transaction ID
    transactionID := s.generateTransactionID()

    // Basic IdTag validation (accept all for testing - enhance later)
    idTagInfo := types.NewIdTagInfo(authorization.AuthorizationStatusAccepted)

    // Create transaction data
    transactionData := map[string]interface{}{
        "transactionId":   transactionID,
        "clientId":        clientID,
        "connectorId":     request.ConnectorId,
        "idTag":           request.IdTag,
        "startTime":       time.Now().UTC(),
        "startMeterValue": request.MeterStart,
        "status":          "active",
        "reservationId":   request.ReservationId,
    }

    // Store transaction in Redis
    err := s.storeTransaction(clientID, transactionID, transactionData)
    if err != nil {
        log.Printf("ERROR: Failed to store transaction for %s: %v", clientID, err)
        idTagInfo = types.NewIdTagInfo(authorization.AuthorizationStatusInvalid)
        return core.NewStartTransactionConfirmation(idTagInfo), nil
    }

    // Update connector status to "Charging"
    s.updateConnectorStatus(clientID, request.ConnectorId, "Charging", &transactionID)

    // Store active transaction mapping
    s.setActiveTransaction(clientID, request.ConnectorId, transactionID)

    log.Printf("TRANSACTION_START: Started transaction %d for %s on connector %d",
        transactionID, clientID, request.ConnectorId)

    // Create confirmation with transaction ID
    confirmation := core.NewStartTransactionConfirmation(idTagInfo)
    confirmation.TransactionId = transactionID

    return confirmation, nil
}
```

### 2. Enhanced StopTransaction Handler

Replace the existing `handleStopTransaction` function in `main.go`:

```go
func (s *OCPPServer) handleStopTransaction(clientID string, request *core.StopTransactionRequest) (*core.StopTransactionConfirmation, error) {
    log.Printf("TRANSACTION_STOP: Request from %s - Transaction: %d, Meter: %d, Reason: %s",
        clientID, request.TransactionId, request.MeterStop, request.Reason)

    // Get transaction data
    transactionData, err := s.getTransaction(clientID, request.TransactionId)
    if err != nil {
        log.Printf("WARNING: Transaction %d not found for %s: %v", request.TransactionId, clientID, err)
        // Still return success - charge point might have restarted
        return core.NewStopTransactionConfirmation(), nil
    }

    // Calculate transaction duration and energy
    startTime, _ := time.Parse(time.RFC3339, transactionData["startTime"].(string))
    duration := time.Since(startTime)
    startMeter := int(transactionData["startMeterValue"].(float64))
    energyConsumed := request.MeterStop - startMeter

    // Update transaction with stop data
    transactionData["endTime"] = time.Now().UTC()
    transactionData["endMeterValue"] = request.MeterStop
    transactionData["status"] = "completed"
    transactionData["stopReason"] = string(request.Reason)
    transactionData["duration"] = duration.Seconds()
    transactionData["energyConsumed"] = energyConsumed

    // Store updated transaction
    err = s.storeTransaction(clientID, request.TransactionId, transactionData)
    if err != nil {
        log.Printf("ERROR: Failed to update transaction %d for %s: %v", request.TransactionId, clientID, err)
    }

    // Get connector ID from transaction data
    connectorID := int(transactionData["connectorId"].(float64))

    // Clear active transaction mapping
    s.clearActiveTransaction(clientID, connectorID)

    // Update connector status to "Available"
    s.updateConnectorStatus(clientID, connectorID, "Available", nil)

    // Process meter values if provided
    if request.TransactionData != nil && len(request.TransactionData) > 0 {
        log.Printf("TRANSACTION_STOP: Processing %d meter values for transaction %d",
            len(request.TransactionData), request.TransactionId)

        for i, meterValue := range request.TransactionData {
            log.Printf("METER_VALUE_%d: Time: %s, Values: %d", i, meterValue.Timestamp, len(meterValue.SampledValue))
            // Process each meter value (implement s.processMeterValue if needed)
            s.processMeterValueForTransaction(clientID, request.TransactionId, meterValue)
        }
    }

    log.Printf("TRANSACTION_STOP: Completed transaction %d for %s - Energy: %d Wh, Duration: %.0f seconds",
        request.TransactionId, clientID, energyConsumed, duration.Seconds())

    return core.NewStopTransactionConfirmation(), nil
}
```

### 3. Transaction Management Helper Functions

Add these helper functions to `main.go`:

```go
// Generate unique transaction ID
func (s *OCPPServer) generateTransactionID() int {
    // Use Redis to generate unique IDs
    result, err := s.redisClient.Incr(context.Background(), "ocpp:transaction:counter").Result()
    if err != nil {
        // Fallback to timestamp-based ID
        return int(time.Now().UnixNano() / 1000000) // milliseconds
    }
    return int(result)
}

// Store transaction data in Redis
func (s *OCPPServer) storeTransaction(clientID string, transactionID int, data map[string]interface{}) error {
    // Store in multiple Redis structures for different access patterns

    // 1. Store complete transaction data
    transactionKey := fmt.Sprintf("ocpp:transaction:data:%d", transactionID)
    dataJSON, err := json.Marshal(data)
    if err != nil {
        return err
    }

    err = s.redisClient.Set(context.Background(), transactionKey, dataJSON, 24*time.Hour).Err()
    if err != nil {
        return err
    }

    // 2. Store transaction ID mapping for client
    clientTransactionsKey := fmt.Sprintf("ocpp:transactions:client:%s", clientID)
    score := float64(time.Now().Unix()) // Use timestamp as score for ordering
    err = s.redisClient.ZAdd(context.Background(), clientTransactionsKey, &redis.Z{
        Score:  score,
        Member: transactionID,
    }).Err()
    if err != nil {
        return err
    }

    // 3. Store global transaction mapping (for finding client by transaction ID)
    globalMappingKey := "ocpp:transaction:mapping"
    err = s.redisClient.HSet(context.Background(), globalMappingKey, transactionID, clientID).Err()
    if err != nil {
        return err
    }

    log.Printf("TRANSACTION_STORE: Stored transaction %d for %s", transactionID, clientID)
    return nil
}

// Get transaction data from Redis
func (s *OCPPServer) getTransaction(clientID string, transactionID int) (map[string]interface{}, error) {
    transactionKey := fmt.Sprintf("ocpp:transaction:data:%d", transactionID)
    dataJSON, err := s.redisClient.Get(context.Background(), transactionKey).Result()
    if err != nil {
        return nil, err
    }

    var data map[string]interface{}
    err = json.Unmarshal([]byte(dataJSON), &data)
    if err != nil {
        return nil, err
    }

    return data, nil
}

// Set active transaction for connector
func (s *OCPPServer) setActiveTransaction(clientID string, connectorID, transactionID int) {
    activeKey := fmt.Sprintf("ocpp:transaction:active:%s:%d", clientID, connectorID)
    err := s.redisClient.Set(context.Background(), activeKey, transactionID, 24*time.Hour).Err()
    if err != nil {
        log.Printf("ERROR: Failed to set active transaction for %s:%d: %v", clientID, connectorID, err)
    }
}

// Clear active transaction for connector
func (s *OCPPServer) clearActiveTransaction(clientID string, connectorID int) {
    activeKey := fmt.Sprintf("ocpp:transaction:active:%s:%d", clientID, connectorID)
    err := s.redisClient.Del(context.Background(), activeKey).Err()
    if err != nil {
        log.Printf("ERROR: Failed to clear active transaction for %s:%d: %v", clientID, connectorID, err)
    }
}

// Get active transaction for connector
func (s *OCPPServer) getActiveTransaction(clientID string, connectorID int) (int, error) {
    activeKey := fmt.Sprintf("ocpp:transaction:active:%s:%d", clientID, connectorID)
    result, err := s.redisClient.Get(context.Background(), activeKey).Result()
    if err != nil {
        return 0, err
    }

    transactionID, err := strconv.Atoi(result)
    if err != nil {
        return 0, err
    }

    return transactionID, nil
}

// Update connector status
func (s *OCPPServer) updateConnectorStatus(clientID string, connectorID int, status string, transactionID *int) {
    statusKey := fmt.Sprintf("ocpp:status:%s:%d", clientID, connectorID)

    statusData := map[string]interface{}{
        "status":    status,
        "timestamp": time.Now().UTC(),
    }

    if transactionID != nil {
        statusData["transactionId"] = *transactionID
    }

    statusJSON, _ := json.Marshal(statusData)
    err := s.redisClient.Set(context.Background(), statusKey, statusJSON, 24*time.Hour).Err()
    if err != nil {
        log.Printf("ERROR: Failed to update status for %s:%d: %v", clientID, connectorID, err)
    }

    log.Printf("CONNECTOR_STATUS: Updated %s:%d to %s", clientID, connectorID, status)
}

// Process meter value for transaction
func (s *OCPPServer) processMeterValueForTransaction(clientID string, transactionID int, meterValue *types.MeterValue) {
    // Store meter value data linked to transaction
    meterKey := fmt.Sprintf("ocpp:meter:transaction:%d", transactionID)

    meterData := map[string]interface{}{
        "timestamp":     meterValue.Timestamp,
        "transactionId": transactionID,
        "clientId":      clientID,
        "values":        make([]map[string]interface{}, 0),
    }

    for _, sample := range meterValue.SampledValue {
        valueData := map[string]interface{}{
            "value": sample.Value,
        }

        if sample.Measurand != nil {
            valueData["measurand"] = string(*sample.Measurand)
        }
        if sample.Unit != nil {
            valueData["unit"] = string(*sample.Unit)
        }
        if sample.Context != nil {
            valueData["context"] = string(*sample.Context)
        }
        if sample.Location != nil {
            valueData["location"] = string(*sample.Location)
        }

        meterData["values"] = append(meterData["values"].([]map[string]interface{}), valueData)
    }

    // Store as list item with score based on timestamp
    score := float64(meterValue.Timestamp.Unix())
    valueJSON, _ := json.Marshal(meterData)

    err := s.redisClient.ZAdd(context.Background(), meterKey, &redis.Z{
        Score:  score,
        Member: string(valueJSON),
    }).Err()

    if err != nil {
        log.Printf("ERROR: Failed to store meter value for transaction %d: %v", transactionID, err)
    } else {
        log.Printf("METER_VALUE: Stored for transaction %d at %s", transactionID, meterValue.Timestamp)
    }
}
```

### 4. Update findClientByTransactionID Function

Update the helper function from Task 1 to use the new Redis structure:

```go
// Find client by transaction ID (updated for new Redis structure)
func (s *OCPPServer) findClientByTransactionID(transactionID int) (string, error) {
    globalMappingKey := "ocpp:transaction:mapping"
    clientID, err := s.redisClient.HGet(context.Background(), globalMappingKey, fmt.Sprintf("%d", transactionID)).Result()
    if err != nil {
        return "", fmt.Errorf("transaction %d not found: %v", transactionID, err)
    }
    return clientID, nil
}
```

### 5. Add Transaction Query Endpoints

Add these simple query endpoints to `main.go`:

```go
// Get active transactions for a client
func (s *OCPPServer) handleGetActiveTransactions(w http.ResponseWriter, r *http.Request) {
    clientID := r.URL.Query().Get("clientId")
    if clientID == "" {
        s.sendErrorResponse(w, http.StatusBadRequest, "clientId parameter required", nil)
        return
    }

    activeTransactions := make([]map[string]interface{}, 0)

    // Check up to 10 connectors for active transactions
    for connectorID := 1; connectorID <= 10; connectorID++ {
        transactionID, err := s.getActiveTransaction(clientID, connectorID)
        if err != nil {
            continue // No active transaction on this connector
        }

        transactionData, err := s.getTransaction(clientID, transactionID)
        if err != nil {
            continue
        }

        activeTransactions = append(activeTransactions, transactionData)
    }

    s.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
        "success": true,
        "data": map[string]interface{}{
            "clientId":     clientID,
            "transactions": activeTransactions,
            "count":        len(activeTransactions),
        },
    })
}

// Get transaction by ID
func (s *OCPPServer) handleGetTransactionByID(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    transactionIDStr := vars["id"]

    transactionID, err := strconv.Atoi(transactionIDStr)
    if err != nil {
        s.sendErrorResponse(w, http.StatusBadRequest, "Invalid transaction ID", err)
        return
    }

    // Find client for this transaction
    clientID, err := s.findClientByTransactionID(transactionID)
    if err != nil {
        s.sendErrorResponse(w, http.StatusNotFound, "Transaction not found", err)
        return
    }

    transactionData, err := s.getTransaction(clientID, transactionID)
    if err != nil {
        s.sendErrorResponse(w, http.StatusNotFound, "Transaction data not found", err)
        return
    }

    s.sendJSONResponse(w, http.StatusOK, map[string]interface{}{
        "success": true,
        "data":    transactionData,
    })
}
```

### 6. Add Query Routes

Add these routes to `setupRoutes()` in `main.go`:

```go
func (s *OCPPServer) setupRoutes() {
    // ... existing routes ...

    // Transaction query endpoints
    s.router.HandleFunc("/api/v1/transactions/active", s.handleGetActiveTransactions).Methods("GET")
    s.router.HandleFunc("/api/v1/transactions/{id:[0-9]+}", s.handleGetTransactionByID).Methods("GET")

    // ... rest of existing routes ...
}
```

---

## Testing

### Test Complete Transaction Flow

1. **Start a transaction** (charge point sends StartTransaction):
```bash
# Monitor logs while charge point starts transaction
# Should see:
# TRANSACTION_START: Request from TEST-CP-001 - Connector: 1, IdTag: USER123, Meter: 0
# TRANSACTION_STORE: Stored transaction 1234 for TEST-CP-001
# CONNECTOR_STATUS: Updated TEST-CP-001:1 to Charging
```

2. **Check active transactions**:
```bash
curl "http://localhost:8083/api/v1/transactions/active?clientId=TEST-CP-001"

# Expected response:
{
  "success": true,
  "data": {
    "clientId": "TEST-CP-001",
    "transactions": [
      {
        "transactionId": 1234,
        "clientId": "TEST-CP-001",
        "connectorId": 1,
        "idTag": "USER123",
        "startTime": "2025-09-15T10:30:00Z",
        "startMeterValue": 0,
        "status": "active"
      }
    ],
    "count": 1
  }
}
```

3. **Get specific transaction**:
```bash
curl "http://localhost:8083/api/v1/transactions/1234"

# Expected response:
{
  "success": true,
  "data": {
    "transactionId": 1234,
    "clientId": "TEST-CP-001",
    "connectorId": 1,
    "idTag": "USER123",
    "startTime": "2025-09-15T10:30:00Z",
    "startMeterValue": 0,
    "status": "active"
  }
}
```

4. **Stop the transaction** (charge point sends StopTransaction):
```bash
# Monitor logs while charge point stops transaction
# Should see:
# TRANSACTION_STOP: Request from TEST-CP-001 - Transaction: 1234, Meter: 1500, Reason: Local
# TRANSACTION_STOP: Completed transaction 1234 for TEST-CP-001 - Energy: 1500 Wh, Duration: 300 seconds
# CONNECTOR_STATUS: Updated TEST-CP-001:1 to Available
```

### Test Remote Control Integration

Test that Task 1 (Remote Control) and Task 2 (Local Handlers) work together:

1. **Remote start** → **Local start confirmation**:
```bash
# Send remote start
curl -X POST http://localhost:8083/api/v1/transactions/remote-start \
  -H "Content-Type: application/json" \
  -d '{"clientId": "TEST-CP-001", "idTag": "USER123"}'

# Wait for charge point to respond with StartTransaction message
# Check active transactions to confirm it was created
curl "http://localhost:8083/api/v1/transactions/active?clientId=TEST-CP-001"
```

2. **Remote stop** → **Local stop confirmation**:
```bash
# Send remote stop
curl -X POST http://localhost:8083/api/v1/transactions/remote-stop \
  -H "Content-Type: application/json" \
  -d '{"transactionId": 1234}'

# Wait for charge point to respond with StopTransaction message
# Check that transaction status is "completed"
curl "http://localhost:8083/api/v1/transactions/1234"
```

---

## Redis Key Structure

After implementing both tasks, your Redis will have:

```
ocpp:transaction:counter              → Auto-incrementing transaction ID counter
ocpp:transaction:data:1234            → Complete transaction data (JSON)
ocpp:transaction:active:TEST-CP-001:1 → Active transaction ID for connector
ocpp:transactions:client:TEST-CP-001  → Sorted set of transaction IDs for client
ocpp:transaction:mapping              → Hash mapping transaction ID → client ID
ocpp:status:TEST-CP-001:1            → Connector status data (JSON)
ocpp:meter:transaction:1234          → Sorted set of meter values for transaction
ocpp:client:connected:TEST-CP-001    → Client connection status
```

---

## ServerState Testing Coverage

These two tasks together will thoroughly test the ServerState:

1. **Request-Response Correlation**: Remote commands test Redis-based correlation
2. **Transaction Lifecycle**: Start/stop handlers test state persistence
3. **Concurrent Operations**: Multiple transactions and connectors
4. **Data Consistency**: Transaction data stored and retrieved correctly
5. **Connection Tracking**: Client connection status management
6. **Timeout Handling**: Remote requests with unresponsive charge points
7. **State Recovery**: Transactions survive server restarts (persisted in Redis)

## Next Steps

After implementing both tasks:
1. Test with a charge point simulator for complete flows
2. Verify Redis keys are created/cleaned correctly
3. Test server restart scenarios (state persistence)
4. Test multiple concurrent transactions
5. Monitor for any ServerState correlation issues