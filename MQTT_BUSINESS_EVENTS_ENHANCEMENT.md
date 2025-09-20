# MQTT Business Events Enhancement Summary

## Overview

The OCPP server has been enhanced to publish business-level transaction events alongside the existing OCPP protocol events. This provides two complementary event streams for different use cases.

## Changes Made

### 1. Enhanced MQTT Publisher (`internal/mqtt/publisher.go`)

#### New Configuration
- Added `BusinessEventsEnabled bool` field to `PublisherConfig`
- Allows independent control of business events vs protocol events

#### New Event Types
```go
// Business event wrapper
type BusinessEvent struct {
    Timestamp time.Time   `json:"timestamp"`
    ClientID  string      `json:"clientId"`
    EventType string      `json:"eventType"`
    EventID   string      `json:"eventId"`
    Payload   interface{} `json:"payload"`
}

// Transaction lifecycle events
type TransactionEvent struct {
    TransactionID int       `json:"transactionId"`
    ConnectorID   int       `json:"connectorId"`
    IdTag         string    `json:"idTag"`
    MeterStart    int       `json:"meterStart,omitempty"`
    MeterStop     int       `json:"meterStop,omitempty"`
    CurrentMeter  int       `json:"currentMeter,omitempty"`
    StartTime     time.Time `json:"startTime,omitempty"`
    StopTime      time.Time `json:"stopTime,omitempty"`
    EnergyUsed    float64   `json:"energyUsed,omitempty"` // kWh
    Duration      float64   `json:"duration,omitempty"`   // minutes
    Reason        string    `json:"reason,omitempty"`
    Status        string    `json:"status"`
}

// Connector status events
type ConnectorEvent struct {
    ConnectorID     int    `json:"connectorId"`
    Status          string `json:"status"`
    PreviousStatus  string `json:"previousStatus,omitempty"`
    TransactionID   *int   `json:"transactionId,omitempty"`
    ErrorCode       string `json:"errorCode,omitempty"`
    Info            string `json:"info,omitempty"`
    VendorID        string `json:"vendorId,omitempty"`
    VendorErrorCode string `json:"vendorErrorCode,omitempty"`
}

// Meter reading events with business intelligence
type MeterReadingEvent struct {
    TransactionID *int                       `json:"transactionId,omitempty"`
    ConnectorID   int                        `json:"connectorId"`
    Timestamp     time.Time                  `json:"timestamp"`
    Measurands    map[string]MeterMeasurand  `json:"measurands"`
    CurrentPower  float64                    `json:"currentPower,omitempty"` // kW
    TotalEnergy   float64                    `json:"totalEnergy,omitempty"`  // kWh
}

// Billing events
type BillingEvent struct {
    TransactionID    int       `json:"transactionId"`
    ConnectorID      int       `json:"connectorId"`
    IdTag            string    `json:"idTag"`
    StartTime        time.Time `json:"startTime"`
    EndTime          time.Time `json:"endTime,omitempty"`
    EnergyConsumed   float64   `json:"energyConsumed"`   // kWh
    Duration         float64   `json:"duration"`         // minutes
    EstimatedCost    float64   `json:"estimatedCost,omitempty"`
    Currency         string    `json:"currency,omitempty"`
    PricingModel     string    `json:"pricingModel,omitempty"`
    EnergyRate       float64   `json:"energyRate,omitempty"`    // per kWh
    TimeRate         float64   `json:"timeRate,omitempty"`      // per minute
}
```

#### New Publishing Methods
- `PublishTransactionEvent(clientID, eventType string, event interface{})`
- `PublishConnectorEvent(clientID string, event interface{})`
- `PublishMeterReadingEvent(clientID string, event interface{})`
- `PublishBillingEvent(clientID string, event interface{})`

#### Helper Methods for Event Creation
- `CreateTransactionStartedEvent()` - Creates events for transaction start
- `CreateTransactionCompletedEvent()` - Creates events for transaction completion
- `CreateMeterReadingEvent()` - Converts OCPP meter values to business events
- `CreateConnectorEvent()` - Creates connector status change events
- `CreateBillingEvent()` - Creates billing/cost estimation events

### 2. Enhanced Transaction Handler (`internal/handlers/transaction_handler.go`)

#### New Interface
```go
type MQTTPublisherInterface interface {
    PublishTransactionEvent(clientID, eventType string, event interface{})
    PublishMeterReadingEvent(clientID string, event interface{})
    PublishConnectorEvent(clientID string, event interface{})
    PublishBillingEvent(clientID string, event interface{})
}
```

#### Enhanced Constructor
- `NewTransactionHandlerWithMQTT()` - Creates handler with MQTT publisher for business events

#### Business Event Types
- `TransactionStartedEvent` - Published when transactions start
- `TransactionCompletedEvent` - Published when transactions complete
- `ConnectorStatusEvent` - Published when connector status changes
- `MeterReadingBusinessEvent` - Published with meter value updates
- `BillingSessionEvent` - Published with cost/billing information

#### Event Integration Points
1. **StartTransaction Handler**: Publishes transaction started events
2. **StopTransaction Handler**: Publishes transaction completed and billing events
3. **StatusNotification Handler**: Publishes connector status change events
4. **MeterValues Handler**: Publishes meter reading business events

#### State Tracking
- Added `connectorStates` map to track previous connector states
- Added `getPreviousConnectorStatus()` method for change detection

### 3. Server Configuration (`internal/server/server.go`)

#### New Config Field
```go
type Config struct {
    // ... existing fields
    MQTTBusinessEventsEnabled bool // Enable business-level MQTT events
}
```

#### Enhanced Initialization
- Updated MQTT publisher configuration to include business events setting
- Updated transaction handler creation to pass MQTT publisher when available

### 4. Main Configuration (`main.go`)

#### New Environment Variable
- `MQTT_BUSINESS_EVENTS_ENABLED` - Controls business events (defaults to `true`)

## Topic Structure

### Protocol Events (Existing)
- `ocpp/messages/{clientID}/{messageType}` - OCPP requests
- `ocpp/responses/{clientID}/{messageType}` - OCPP responses

### Business Events (New)
- `csms/transactions/{clientID}/started` - Transaction started
- `csms/transactions/{clientID}/completed` - Transaction completed
- `csms/transactions/{clientID}/meter_reading` - Meter value updates
- `csms/connectors/{clientID}/status_changed` - Connector status changes
- `csms/billing/{clientID}/session_cost` - Billing/cost information

## Key Features

### 1. Dual Event Streams
- **Protocol Events**: Raw OCPP messages for technical integration
- **Business Events**: Processed data for business applications

### 2. Business Intelligence
- Automatic unit conversion (Wh → kWh, W → kW)
- Calculated fields (duration, energy consumed, estimated cost)
- Aggregated measurements from multiple meter values
- Previous state tracking for change detection

### 3. Configuration Flexibility
- Business events can be enabled/disabled independently
- Protocol events continue to work as before
- Non-breaking changes to existing functionality

### 4. Event Processing
- Asynchronous publishing to avoid blocking OCPP handlers
- Error handling and logging for failed publishes
- Proper event ID generation for traceability

## Benefits

### For System Integrators
- **Protocol Events**: Perfect for debugging, compliance, and technical monitoring
- Raw OCPP data maintains full protocol fidelity

### For Business Applications
- **Business Events**: Ideal for dashboards, analytics, and billing systems
- Pre-processed data with business context
- Consistent units and calculated fields

### For Operations
- **Complementary Streams**: Different views of the same data
- **Configurable**: Enable only what you need
- **Non-Breaking**: Existing integrations continue working

## Example Use Cases

### Business Events
1. **Billing Systems**: Use session cost events for automatic billing
2. **Dashboards**: Display real-time energy consumption and costs
3. **Analytics**: Track usage patterns and efficiency metrics
4. **Notifications**: Alert on connector status changes or transaction events

### Protocol Events
1. **Debugging**: Analyze raw OCPP message flow
2. **Compliance**: Audit OCPP protocol adherence
3. **Integration**: Build low-level OCPP client tools
4. **Monitoring**: Track protocol-level errors and performance

## Configuration Example

```bash
# Enable MQTT with both protocol and business events
export MQTT_ENABLED=true
export MQTT_HOST=localhost
export MQTT_PORT=1883
export MQTT_BUSINESS_EVENTS_ENABLED=true

# Or disable business events (protocol events still work)
export MQTT_BUSINESS_EVENTS_ENABLED=false
```

## Files Modified

1. `/internal/mqtt/publisher.go` - Enhanced MQTT publisher with business events
2. `/internal/handlers/transaction_handler.go` - Added business event publishing
3. `/internal/server/server.go` - Updated configuration and initialization
4. `/main.go` - Added business events configuration parameter

## Testing

The implementation has been successfully compiled and tested. The binary builds without errors and includes all the enhanced functionality for dual event streams.

## Next Steps

1. **Deploy**: Use the enhanced server in your environment
2. **Subscribe**: Set up MQTT subscribers for the new business event topics
3. **Integrate**: Connect business applications to the new event streams
4. **Monitor**: Use both event streams for comprehensive system monitoring