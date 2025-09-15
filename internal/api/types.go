package api

// APIResponse is the standard response format for all API endpoints
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}


// RemoteStartRequest represents a request to start a remote transaction
type RemoteStartRequest struct {
	ConnectorID *int   `json:"connectorId,omitempty"`
	IdTag       string `json:"idTag" validate:"required,max=20"`
}

// RemoteStopRequest represents a request to stop a remote transaction
type RemoteStopRequest struct {
	TransactionID int `json:"transactionId" validate:"required,min=1"`
}

// RemoteTransactionResult represents the result of a remote transaction operation
type RemoteTransactionResult struct {
	RequestID   string `json:"requestId"`
	ClientID    string `json:"clientId"`
	ConnectorID int    `json:"connectorId"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}