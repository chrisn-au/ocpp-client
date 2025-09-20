# OCPP Server REST API Endpoints

This document describes all REST API endpoints available in the OCPP Server.

## Table of Contents

### 🚀 Quick Access (Most Frequently Used)
- [Remote Start Transaction](#post-apiv1transactionsremote-start) - Start a charging session remotely
- [Remote Stop Transaction](#post-apiv1transactionsremote-stop) - Stop a charging session remotely
- [Get Connected Clients](#get-clients) - See which charge points are online

### 📋 OCPP Server API Reference
1. [Health & System](#health-and-system-information)
   - [Health Check](#get-health)
   - [Connected Clients](#get-clients)

2. [Charge Point Management](#charge-point-information)
   - [List Charge Points](#get-apiv1chargepoints)
   - [Get Charge Point Details](#get-apiv1chargepointsclientid)
   - [Connector Information](#get-apiv1chargepointsclientidconnectors)
   - [Status Check](#get-apiv1chargepointsclientidstatus)

3. [📱 Remote Transaction Control](#remote-transaction-control) ⭐
   - [Start Transaction](#post-apiv1transactionsremote-start)
   - [Stop Transaction](#post-apiv1transactionsremote-stop)

4. [Transaction Information](#transaction-information)
   - [List Transactions](#get-apiv1transactions)
   - [Get Transaction Details](#get-apiv1transactionstransactionid)

5. [Configuration Management](#configuration-management)
   - [Get Stored Configuration](#get-apiv1chargepointsclientidconfiguration)
   - [Change Stored Configuration](#put-apiv1chargepointsclientidconfiguration)
   - [Export Configuration](#get-apiv1chargepointsclientidconfigurationexport)
   - [Live Configuration](#live-configuration-management)

6. [API Reference](#api-reference-information)
   - [Error Handling](#error-handling)
   - [Architecture](#api-architecture)

### 👥 User Management & Authentication APIs
**For user, contract, and token management, see the separate CSMS API Service:**
- **Service URL**: `http://localhost:59800/api/v1`
- **Documentation**: [CSMS API Service README](../../csms-api-service/README.md)
- **API Reference**: [CSMS API Reference](../../csms-api-service/API_REFERENCE.md)
- **Features**:
  - User account management
  - OCPI-compliant contract management
  - Multi-modal authentication tokens (RFID, App, Vehicle)
  - Family plan support
  - Complete CRUD operations with filtering and pagination

---

## Base URL
All endpoints are served on the configured HTTP port (default: varies by deployment)

## API Versioning
The OCPP Server uses a clean, versioned API structure:

- **V1 API**: `/api/v1/*` - Production API with clean architecture and separation of concerns

## Architecture Overview
The API is organized into clean layers:
- **Handlers**: HTTP request/response handling separated by domain
- **Services**: Business logic layer with proper encapsulation
- **Models**: Clean API request/response models
- **Routing**: Organized route definitions with clear structure

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

### GET /api/v1/chargepoints
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

### GET /api/v1/chargepoints/{clientID}
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

### GET /api/v1/chargepoints/{clientID}/connectors
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

### GET /api/v1/chargepoints/{clientID}/connectors/{connectorID}
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

### GET /api/v1/transactions
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

### GET /api/v1/transactions/{transactionID}
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

## Remote Transaction Control ⭐

*These are the most frequently used endpoints for testing transaction operations.*

### POST /api/v1/transactions/remote-start
**🚀 START A CHARGING SESSION**
Initiates a remote start transaction on a charge point.

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
**🛑 STOP A CHARGING SESSION**
Stops a remote transaction on a charge point.

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

## API Reference Information

### Error Handling

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

### Authentication

Currently, this API does not implement authentication. All endpoints are publicly accessible.

### Rate Limiting

No rate limiting is currently implemented.

### Content Type

All endpoints that accept request bodies expect `application/json` content type.
All endpoints return `application/json` responses using the standard APIResponse format.

### API Architecture

The OCPP Server API is built with a clean, layered architecture for maintainability and separation of concerns:

### Directory Structure
```
internal/
├── api/
│   └── v1/
│       ├── handlers/      # Domain-specific API handlers
│       │   ├── chargepoints.go
│       │   ├── transactions.go
│       │   ├── configuration.go
│       │   └── health.go
│       ├── models/        # Clean API request/response models
│       │   ├── requests.go
│       │   ├── responses.go
│       │   └── common.go
│       └── routes.go      # Route definitions
├── services/              # Business logic layer
│   ├── chargepoint.go     # Charge point operations
│   ├── transaction.go     # Transaction operations
│   └── configuration.go   # Configuration operations
└── ocpp/                  # OCPP protocol message handlers
```

### Key Architecture Benefits

1. **Separation of Concerns**: API handlers, business logic, and data access are clearly separated
2. **Service Layer**: Business logic is encapsulated in dedicated services
3. **Clean Models**: API models are distinct from internal business models
4. **Domain Organization**: Handlers are organized by business domain
5. **Maintainability**: Modular structure enables easier testing and development
6. **Scalability**: Architecture supports future feature additions

### Development Patterns

- **Handlers**: Focus purely on HTTP request/response handling
- **Services**: Contain all business logic and validation
- **Models**: Define clean contracts for API communication
- **Routes**: Centralized route definitions with clear organization