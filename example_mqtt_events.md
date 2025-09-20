# MQTT Business Events Example

This document demonstrates the enhanced MQTT publisher with business-level transaction events.

## Configuration

To enable business events, set the following environment variables:

```bash
export MQTT_ENABLED=true
export MQTT_HOST=localhost
export MQTT_PORT=1883
export MQTT_BUSINESS_EVENTS_ENABLED=true
```

## Event Topics and Payloads

### 1. Transaction Started Event
**Topic:** `csms/transactions/{clientID}/started`
**Payload Example:**
```json
{
  "timestamp": "2023-12-07T10:15:30Z",
  "clientId": "charge-point-001",
  "eventType": "started",
  "eventId": "started_1701940530123456789",
  "payload": {
    "transactionId": 123456,
    "connectorId": 1,
    "idTag": "user123",
    "meterStart": 1000,
    "startTime": "2023-12-07T10:15:30Z",
    "status": "started"
  }
}
```

### 2. Transaction Completed Event
**Topic:** `csms/transactions/{clientID}/completed`
**Payload Example:**
```json
{
  "timestamp": "2023-12-07T11:45:30Z",
  "clientId": "charge-point-001",
  "eventType": "completed",
  "eventId": "completed_1701946530123456789",
  "payload": {
    "transactionId": 123456,
    "connectorId": 1,
    "idTag": "user123",
    "meterStart": 1000,
    "meterStop": 25000,
    "startTime": "2023-12-07T10:15:30Z",
    "stopTime": "2023-12-07T11:45:30Z",
    "energyUsed": 24.0,
    "duration": 90.0,
    "reason": "Local",
    "status": "completed"
  }
}
```

### 3. Meter Reading Event
**Topic:** `csms/transactions/{clientID}/meter_reading`
**Payload Example:**
```json
{
  "timestamp": "2023-12-07T10:30:30Z",
  "clientId": "charge-point-001",
  "eventType": "meter_reading",
  "eventId": "meter_reading_1701941430123456789",
  "payload": {
    "transactionId": 123456,
    "connectorId": 1,
    "timestamp": "2023-12-07T10:30:30Z",
    "measurands": {
      "Energy.Active.Import.Register": {
        "value": 12.5,
        "unit": "kWh",
        "context": "Sample.Periodic"
      },
      "Power.Active.Import": {
        "value": 7.2,
        "unit": "kW",
        "context": "Sample.Periodic"
      }
    },
    "currentPower": 7.2,
    "totalEnergy": 12.5
  }
}
```

### 4. Connector Status Changed Event
**Topic:** `csms/connectors/{clientID}/status_changed`
**Payload Example:**
```json
{
  "timestamp": "2023-12-07T10:15:25Z",
  "clientId": "charge-point-001",
  "eventType": "status_changed",
  "eventId": "status_changed_1701940525123456789",
  "payload": {
    "connectorId": 1,
    "status": "Charging",
    "previousStatus": "Preparing",
    "transactionId": 123456,
    "errorCode": "NoError"
  }
}
```

### 5. Billing Session Cost Event
**Topic:** `csms/billing/{clientID}/session_cost`
**Payload Example:**
```json
{
  "timestamp": "2023-12-07T11:45:30Z",
  "clientId": "charge-point-001",
  "eventType": "session_cost",
  "eventId": "session_cost_1701946530123456789",
  "payload": {
    "transactionId": 123456,
    "connectorId": 1,
    "idTag": "user123",
    "startTime": "2023-12-07T10:15:30Z",
    "endTime": "2023-12-07T11:45:30Z",
    "energyConsumed": 24.0,
    "duration": 90.0,
    "estimatedCost": 2.88,
    "currency": "USD",
    "pricingModel": "energy_based",
    "energyRate": 0.12,
    "timeRate": 0.0
  }
}
```

## Event Streams Comparison

### Protocol Events (existing)
These are raw OCPP messages published to:
- `ocpp/messages/{clientID}/{messageType}` (requests)
- `ocpp/responses/{clientID}/{messageType}` (responses)

Contains the actual OCPP protocol data as defined by the OCPP specification.

### Business Events (new)
These are business-friendly events published to:
- `csms/transactions/{clientID}/*` (transaction lifecycle)
- `csms/connectors/{clientID}/*` (connector status)
- `csms/billing/{clientID}/*` (billing information)

Contains processed, business-relevant data with:
- Converted units (Wh to kWh, W to kW)
- Calculated fields (duration, energy used, estimated cost)
- Business context (previous states, aggregated measurements)

## Benefits

1. **Protocol Events**: Perfect for debugging, compliance, and technical integration
2. **Business Events**: Ideal for dashboards, analytics, billing systems, and business intelligence
3. **Complementary**: Both streams provide different views of the same underlying data
4. **Configurable**: Can be enabled/disabled independently via configuration