# StatusNotification Implementation Guide

## Overview

StatusNotification is an OCPP message sent by charge points to inform the CSMS about connector status changes, error conditions, and operational state updates. This document covers the complete implementation in the OCPP server.

## Implementation Status

✅ **Fully Implemented** - StatusNotification is already working in the OCPP server with both OCPP 1.6 and 2.0.1 support.

## Message Flow

```
Charge Point → CSMS: StatusNotificationRequest
CSMS → Charge Point: StatusNotificationConfirmation
```

## OCPP Protocol Support

### OCPP 1.6 Implementation
- **File**: `ocpp-go/ocpp1.6/core/status_notification.go`
- **Handler**: `HandleStatusNotification` in `ocpp-server/internal/ocpp/handlers.go:62`

**Request Structure:**
```json
{
  "connectorId": 1,
  "status": "Available",
  "errorCode": "NoError",
  "info": "Optional additional info",
  "timestamp": "2023-12-07T10:30:00Z",
  "vendorId": "vendor123",
  "vendorErrorCode": "V001"
}
```

**Available Statuses:**
- `Available` - Ready for new user
- `Preparing` - Preparing for transaction
- `Charging` - Active charging session
- `SuspendedEVSE` - Suspended by charge point
- `SuspendedEV` - Suspended by EV
- `Finishing` - Transaction ending
- `Reserved` - Reserved for user
- `Unavailable` - Not available
- `Faulted` - Error condition

### OCPP 2.0.1 Implementation
- **File**: `ocpp-go/ocpp2.0.1/availability/status_notification.go`

**Request Structure:**
```json
{
  "timestamp": "2023-12-07T10:30:00Z",
  "connectorStatus": "Available",
  "evseId": 1,
  "connectorId": 1
}
```

**Available Statuses:**
- `Available` - Ready for new user
- `Occupied` - In use by EV
- `Reserved` - Reserved
- `Unavailable` - Not available
- `Faulted` - Error condition

## MQTT Publishing

StatusNotification messages are automatically published to MQTT when enabled.

### OCPP Protocol Topics

**Raw OCPP Messages:**
- `ocpp/messages/{clientID}/StatusNotification` - Incoming request
- `ocpp/responses/{clientID}/StatusNotification` - Outgoing response

### Business Event Topics

**Connector Status Changes:**
- `csms/connectors/{clientID}/status_changed`

**Example Business Event:**
```json
{
  "timestamp": "2023-12-07T10:30:00Z",
  "clientId": "charger123",
  "eventType": "status_changed",
  "eventId": "status_changed_1701940200123456789",
  "payload": {
    "connectorId": 1,
    "status": "Available",
    "previousStatus": "Charging",
    "transactionId": null,
    "errorCode": "NoError",
    "info": "",
    "vendorId": "",
    "vendorErrorCode": ""
  }
}
```

### MQTT Configuration

Enable business events in your server configuration:
```go
publisherConfig := mqtt.PublisherConfig{
    BrokerHost:            "localhost",
    BrokerPort:            1883,
    BusinessEventsEnabled: true, // Enable business events
}
```

## REST API Endpoints

### Get Connector Status

**All Connectors:**
```http
GET /api/v1/chargepoints/{clientID}/connectors
```

**Response:**
```json
{
  "success": true,
  "message": "Connectors retrieved",
  "data": {
    "connectors": [
      {
        "connectorId": 1,
        "status": "Available",
        "transaction": null
      }
    ],
    "count": 1
  }
}
```

**Specific Connector:**
```http
GET /api/v1/chargepoints/{clientID}/connectors/{connectorID}
```

**Response:**
```json
{
  "success": true,
  "message": "Connector retrieved",
  "data": {
    "connectorId": 1,
    "status": "Available",
    "transaction": null
  }
}
```

**Charge Point Status:**
```http
GET /api/v1/chargepoints/{clientID}/status
```

**Response:**
```json
{
  "success": true,
  "message": "Charger status retrieved",
  "data": {
    "isOnline": true,
    "lastSeen": "2023-12-07T10:30:00Z"
  }
}
```

## Business State Management

StatusNotification updates are automatically stored in Redis:

- **Connector Status**: Tracked per connector per charge point
- **Last Seen**: Charge point activity tracking
- **Transaction Association**: Links status to active transactions

## Handler Implementation

The StatusNotification handler (`ocpp-server/internal/ocpp/handlers.go:62`) performs:

1. **Logging**: Records incoming status changes
2. **State Update**: Updates connector status in Redis
3. **MQTT Publishing**: Publishes business events if enabled
4. **Response**: Sends confirmation back to charge point

```go
func HandleStatusNotification(server *ocppj.Server, businessState *ocppj.RedisBusinessState, clientID, requestId string, req *core.StatusNotificationRequest) {
    log.Printf("StatusNotification from %s: ConnectorId=%d, Status=%s, ErrorCode=%s",
        clientID, req.ConnectorId, req.Status, req.ErrorCode)

    // Update connector status in business state
    connectorStatus := &ocppj.ConnectorStatus{
        Status:      string(req.Status),
        Transaction: nil,
    }

    if err := businessState.SetConnectorStatus(clientID, req.ConnectorId, connectorStatus); err != nil {
        log.Printf("Error updating connector status: %v", err)
    }

    response := core.NewStatusNotificationConfirmation()
    if err := server.SendResponse(clientID, requestId, response); err != nil {
        log.Printf("Error sending StatusNotification response: %v", err)
    }
}
```

## Common Use Cases

### Monitoring Connector Availability
Subscribe to MQTT topic: `csms/connectors/+/status_changed`

### Real-time Dashboard Updates
Use REST API with polling or WebSocket integration with MQTT

### Billing Integration
Monitor status changes for transaction lifecycle management

### Maintenance Alerts
Watch for `Faulted` status to trigger maintenance workflows

## Error Handling

The system handles various error conditions:

- **Invalid Connector ID**: Validates connector exists
- **Redis Connection Issues**: Graceful degradation
- **MQTT Publishing Failures**: Logged but doesn't block OCPP response

## Performance Considerations

- **Asynchronous MQTT Publishing**: Non-blocking business event publication
- **Redis Optimization**: Efficient connector status storage
- **Rate Limiting**: Built-in OCPP message throttling

## Testing

Use the test charge point client to send StatusNotification:
```bash
CSMS_URL=http://localhost:3000 go run test-charge-point-client/main.go
```

Monitor MQTT messages:
```bash
mosquitto_sub -h localhost -t "csms/connectors/+/status_changed"
```

## Related Documentation

- [OCPP 1.6 Specification](https://www.openchargealliance.org/)
- [MQTT Business Events](./MQTT_BUSINESS_EVENTS_ENHANCEMENT.md)
- [REST API Documentation](./API.md)