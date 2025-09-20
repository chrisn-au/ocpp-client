package models

// RemoteStartRequest represents a request to start a remote transaction
type RemoteStartRequest struct {
	ClientID    string `json:"clientId" validate:"required"`
	ConnectorID *int   `json:"connectorId,omitempty"`
	IdTag       string `json:"idTag" validate:"required,max=20"`
}

// RemoteStopRequest represents a request to stop a remote transaction
type RemoteStopRequest struct {
	ClientID      string `json:"clientId,omitempty"`
	TransactionID int    `json:"transactionId" validate:"required,min=1"`
}

// ConfigurationChangeRequest represents a request to change configuration
type ConfigurationChangeRequest struct {
	Key   string `json:"key" validate:"required"`
	Value string `json:"value" validate:"required"`
}

// TriggerMessageRequest represents a request to trigger a specific message from a charge point.
//
// This struct is used for the OCPP 1.6 TriggerMessage feature, which allows the Central System
// to request charge points to send specific messages on demand. This is particularly useful
// for obtaining immediate status updates, diagnostic information, or testing connectivity.
//
// Fields:
//   - RequestedMessage: The type of message to trigger (required)
//     Supported values: "StatusNotification", "Heartbeat", "MeterValues", "BootNotification"
//   - ConnectorID: Optional connector identifier for connector-specific messages (>= 0)
//     Used with StatusNotification and MeterValues. If omitted for StatusNotification,
//     the charge point will send status for all connectors.
//
// Usage Examples:
//   // Request status for all connectors
//   {
//     "requestedMessage": "StatusNotification"
//   }
//
//   // Request status for specific connector
//   {
//     "requestedMessage": "StatusNotification",
//     "connectorId": 1
//   }
//
//   // Test connectivity
//   {
//     "requestedMessage": "Heartbeat"
//   }
//
// The request is validated using struct tags to ensure:
//   - RequestedMessage is one of the supported OCPP message types
//   - ConnectorID, if provided, is non-negative (0 refers to the charge point itself)
type TriggerMessageRequest struct {
	RequestedMessage string `json:"requestedMessage" validate:"required,oneof=StatusNotification Heartbeat MeterValues BootNotification"`
	ConnectorID      *int   `json:"connectorId,omitempty" validate:"omitempty,min=0"`
}

// Legacy request types for backward compatibility
type LegacyRemoteStartRequest struct {
	ConnectorID *int   `json:"connectorId,omitempty"`
	IdTag       string `json:"idTag" validate:"required,max=20"`
}

type LegacyRemoteStopRequest struct {
	TransactionID int `json:"transactionId" validate:"required,min=1"`
}