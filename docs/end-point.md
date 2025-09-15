# OCPP Server REST API Endpoints

This document describes all REST API endpoints available in the OCPP Server.

## Base URL
All endpoints are served on the configured HTTP port (default: varies by deployment)

## Health and System Information

### GET /health
Returns the health status of the OCPP server.

**Response (200 OK):**
```json
{
  "success": true,
  "message": "OCPP Server is running",
  "data": {
    "timestamp": "2023-10-15T10:30:00Z"
  }
}
```

### GET /clients
Retrieves all currently connected OCPP clients.

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Connected clients retrieved",
  "data": {
    "clients": ["CP001", "CP002", "CP003"],
    "count": 3
  }
}
```

## Charge Point Information

### GET /chargepoints
Retrieves information about all charge points.

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Charge points retrieved",
  "data": {
    "chargePoints": [
      {
        "id": "CP001",
        "status": "Available",
        "lastSeen": "2023-10-15T10:30:00Z"
      }
    ],
    "count": 1
  }
}
```

### GET /chargepoints/{clientID}
Retrieves information about a specific charge point.

**Parameters:**
- `clientID` (path): Charge point identifier

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Charge point retrieved",
  "data": {
    "id": "CP001",
    "status": "Available",
    "connectors": [...],
    "lastSeen": "2023-10-15T10:30:00Z"
  }
}
```

**Response (404 Not Found):**
```json
{
  "success": false,
  "message": "Charge point not found"
}
```

### GET /chargepoints/{clientID}/connectors
Retrieves all connectors for a specific charge point.

**Parameters:**
- `clientID` (path): Charge point identifier

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Connectors retrieved",
  "data": {
    "connectors": [
      {
        "id": 1,
        "status": "Available",
        "currentTransaction": null
      }
    ],
    "count": 1
  }
}
```

### GET /chargepoints/{clientID}/connectors/{connectorID}
Retrieves information about a specific connector.

**Parameters:**
- `clientID` (path): Charge point identifier
- `connectorID` (path): Connector identifier (integer)

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Connector retrieved",
  "data": {
    "id": 1,
    "status": "Available",
    "currentTransaction": null
  }
}
```

**Response (400 Bad Request):**
```json
{
  "success": false,
  "message": "Invalid connector ID"
}
```

### GET /api/v1/chargepoints/{clientID}/status
Retrieves the online status of a specific charge point.

**Parameters:**
- `clientID` (path): Charge point identifier

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Charger status retrieved",
  "data": {
    "clientID": "CP001",
    "online": true
  }
}
```

## Transaction Information

### GET /transactions
Retrieves active transactions, optionally filtered by charge point.

**Query Parameters:**
- `clientId` (optional): Filter transactions by charge point ID

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Transactions retrieved",
  "data": {
    "transactions": [
      {
        "id": 123,
        "clientID": "CP001",
        "connectorId": 1,
        "idTag": "USER123",
        "startTime": "2023-10-15T10:00:00Z",
        "meterStart": 1000,
        "status": "Charging"
      }
    ],
    "count": 1
  }
}
```

### GET /transactions/{transactionID}
Retrieves information about a specific transaction.

**Parameters:**
- `transactionID` (path): Transaction identifier (integer)

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Transaction retrieved",
  "data": {
    "id": 123,
    "clientID": "CP001",
    "connectorId": 1,
    "idTag": "USER123",
    "startTime": "2023-10-15T10:00:00Z",
    "meterStart": 1000,
    "status": "Charging"
  }
}
```

**Response (400 Bad Request):**
```json
{
  "success": false,
  "message": "Invalid transaction ID"
}
```

**Response (404 Not Found):**
```json
{
  "success": false,
  "message": "Transaction not found"
}
```

## Remote Transaction Control

### POST /api/v1/transactions/remote-start
Initiates a remote start transaction on a charge point (new API).

**Request Body:**
```json
{
  "clientId": "CP001",
  "connectorId": 1,
  "idTag": "USER123"
}
```

**Request Body Parameters:**
- `clientId` (required): Target charge point identifier
- `connectorId` (optional): Connector identifier (defaults to 1)
- `idTag` (required): RFID tag or user identifier (max 20 characters)

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Remote RemoteStartTransaction successful",
  "data": {
    "requestId": "1697360400123456789",
    "clientId": "CP001",
    "connectorId": 1,
    "status": "accepted",
    "message": "RemoteStartTransaction accepted by charge point"
  }
}
```

**Response (400 Bad Request):**
```json
{
  "success": false,
  "message": "clientId and idTag are required"
}
```

**Response (404 Not Found):**
```json
{
  "success": false,
  "message": "Client not connected"
}
```

**Response (408 Request Timeout):**
```json
{
  "success": false,
  "message": "Timeout waiting for charge point response",
  "data": {
    "status": "timeout",
    "message": "Request timeout"
  }
}
```

### POST /api/v1/transactions/remote-stop
Stops a remote transaction on a charge point (new API).

**Request Body:**
```json
{
  "clientId": "CP001",
  "transactionId": 123
}
```

**Request Body Parameters:**
- `clientId` (optional): Target charge point identifier (will be auto-detected if not provided)
- `transactionId` (required): Transaction identifier (minimum 1)

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Remote RemoteStopTransaction successful",
  "data": {
    "requestId": "1697360400123456789",
    "clientId": "CP001",
    "connectorId": 0,
    "status": "accepted",
    "message": "RemoteStopTransaction accepted by charge point"
  }
}
```

**Response (400 Bad Request):**
```json
{
  "success": false,
  "message": "Valid transactionId is required"
}
```

**Response (404 Not Found):**
```json
{
  "success": false,
  "message": "Transaction not found"
}
```

### POST /clients/{clientID}/remote-start
Legacy remote start transaction endpoint.

**Parameters:**
- `clientID` (path): Target charge point identifier

**Request Body:**
```json
{
  "idTag": "USER123",
  "connectorId": 1
}
```

**Request Body Parameters:**
- `idTag` (required): RFID tag or user identifier
- `connectorId` (optional): Connector identifier (defaults to 1)

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Remote start transaction successful",
  "data": {
    "requestId": "1697360400123456789",
    "clientId": "CP001",
    "connectorId": 1,
    "status": "accepted",
    "message": "RemoteStartTransaction accepted by charge point"
  }
}
```

### POST /clients/{clientID}/remote-stop
Legacy remote stop transaction endpoint.

**Parameters:**
- `clientID` (path): Target charge point identifier

**Request Body:**
```json
{
  "transactionId": 123
}
```

**Request Body Parameters:**
- `transactionId` (required): Transaction identifier (minimum 1)

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Remote stop transaction successful",
  "data": {
    "requestId": "1697360400123456789",
    "clientId": "CP001",
    "connectorId": 0,
    "status": "accepted",
    "message": "RemoteStopTransaction accepted by charge point"
  }
}
```

## Configuration Management

### GET /api/v1/chargepoints/{clientID}/configuration
Retrieves stored configuration for a charge point.

**Parameters:**
- `clientID` (path): Charge point identifier

**Query Parameters:**
- `keys` (optional): Comma-separated list of configuration keys to retrieve

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Configuration retrieved",
  "data": {
    "configuration": {
      "MeterValueSampleInterval": {
        "value": "60",
        "readonly": false
      },
      "HeartbeatInterval": {
        "value": "86400",
        "readonly": false
      }
    },
    "unknownKeys": []
  }
}
```

### PUT /api/v1/chargepoints/{clientID}/configuration
Changes stored configuration for a charge point.

**Parameters:**
- `clientID` (path): Charge point identifier

**Request Body:**
```json
{
  "key": "MeterValueSampleInterval",
  "value": "30"
}
```

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Configuration change processed",
  "data": {
    "status": "Accepted"
  }
}
```

### GET /api/v1/chargepoints/{clientID}/configuration/export
Exports all configuration for a charge point.

**Parameters:**
- `clientID` (path): Charge point identifier

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Configuration exported",
  "data": {
    "clientID": "CP001",
    "configurations": {
      "MeterValueSampleInterval": "60",
      "HeartbeatInterval": "86400"
    }
  }
}
```

## Live Configuration Management

### GET /api/v1/chargepoints/{clientID}/configuration/live
Retrieves live configuration directly from the charge point.

**Parameters:**
- `clientID` (path): Charge point identifier

**Query Parameters:**
- `keys` (optional): Comma-separated list of configuration keys to retrieve

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Live configuration retrieved from charger",
  "data": {
    "configurationKey": [
      {
        "key": "MeterValueSampleInterval",
        "readonly": false,
        "value": "60"
      }
    ],
    "unknownKey": []
  }
}
```

**Response (503 Service Unavailable):**
```json
{
  "success": false,
  "message": "Charger is offline - returning stored configuration",
  "data": {
    "online": false,
    "note": "Falling back to stored configuration. Use /configuration endpoint for stored values."
  }
}
```

**Response (408 Request Timeout):**
```json
{
  "success": false,
  "message": "Timeout waiting for charger response",
  "data": {
    "online": true,
    "timeout": "10s",
    "note": "Charger did not respond within timeout. Use /configuration endpoint for stored values."
  }
}
```

### PUT /api/v1/chargepoints/{clientID}/configuration/live
Changes live configuration directly on the charge point.

**Parameters:**
- `clientID` (path): Charge point identifier

**Request Body:**
```json
{
  "key": "MeterValueSampleInterval",
  "value": "30"
}
```

**Response (202 Accepted):**
```json
{
  "success": true,
  "message": "ChangeConfiguration request sent to charger",
  "data": {
    "clientID": "CP001",
    "key": "MeterValueSampleInterval",
    "value": "30",
    "online": true,
    "note": "Request sent to charger. Response will be processed asynchronously. Check server logs for the charger's response."
  }
}
```

**Response (503 Service Unavailable):**
```json
{
  "success": false,
  "message": "Charger is offline - cannot change live configuration",
  "data": {
    "online": false,
    "note": "Use /configuration endpoint to change stored configuration."
  }
}
```

## Error Handling

All endpoints use standard HTTP status codes:

- `200 OK`: Request successful
- `202 Accepted`: Request accepted for asynchronous processing
- `400 Bad Request`: Invalid request parameters or body
- `404 Not Found`: Resource not found
- `408 Request Timeout`: Request timed out (synchronous endpoints only)
- `500 Internal Server Error`: Server-side error
- `503 Service Unavailable`: Service temporarily unavailable (e.g., charge point offline)

Error responses follow the APIResponse format:
```json
{
  "success": false,
  "message": "Human readable error message",
  "data": {
    "additional": "context information"
  }
}
```

## Authentication

Currently, this API does not implement authentication. All endpoints are publicly accessible.

## Rate Limiting

No rate limiting is currently implemented.

## Content Type

All endpoints that accept request bodies expect `application/json` content type.
All endpoints return `application/json` responses using the standard APIResponse format.