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

// Legacy request types for backward compatibility
type LegacyRemoteStartRequest struct {
	ConnectorID *int   `json:"connectorId,omitempty"`
	IdTag       string `json:"idTag" validate:"required,max=20"`
}

type LegacyRemoteStopRequest struct {
	TransactionID int `json:"transactionId" validate:"required,min=1"`
}