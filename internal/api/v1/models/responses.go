package models

// RemoteTransactionResult represents the result of a remote transaction operation
type RemoteTransactionResult struct {
	RequestID   string `json:"requestId"`
	ClientID    string `json:"clientId"`
	ConnectorID int    `json:"connectorId"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

// ChargePointStatusResponse represents the online status of a charge point
type ChargePointStatusResponse struct {
	ClientID string `json:"clientId"`
	Online   bool   `json:"online"`
}

// ConnectedClientsResponse represents connected clients information
type ConnectedClientsResponse struct {
	Clients []string `json:"clients"`
	Count   int      `json:"count"`
}

// ChargePointsResponse represents charge points collection
type ChargePointsResponse struct {
	ChargePoints []interface{} `json:"chargePoints"`
	Count        int           `json:"count"`
}

// ConnectorsResponse represents connectors collection
type ConnectorsResponse struct {
	Connectors []interface{} `json:"connectors"`
	Count      int           `json:"count"`
}

// TransactionsResponse represents transactions collection
type TransactionsResponse struct {
	Transactions []interface{} `json:"transactions"`
	Count        int           `json:"count"`
}

// ConfigurationResponse represents configuration data
type ConfigurationResponse struct {
	Configuration map[string]interface{} `json:"configuration"`
	UnknownKeys   []string               `json:"unknownKeys,omitempty"`
}

// ConfigurationChangeResponse represents configuration change result
type ConfigurationChangeResponse struct {
	Status string `json:"status"`
}

// LiveConfigurationChangeResponse represents live configuration change result
type LiveConfigurationChangeResponse struct {
	ClientID string `json:"clientId"`
	Key      string `json:"key"`
	Value    string `json:"value"`
	Online   bool   `json:"online"`
	Note     string `json:"note"`
}

// TriggerMessageResponse represents the result of a TriggerMessage operation.
//
// This struct contains the response data for TriggerMessage requests sent to charge points.
// It provides information about whether the charge point accepted, rejected, or failed to
// respond to the trigger request within the timeout period.
//
// Fields:
//   - RequestID: Unique identifier for correlating the request with its response
//   - ClientID: The charge point identifier that received the trigger request
//   - RequestedMessage: The type of message that was requested to be triggered
//   - ConnectorID: The connector ID for connector-specific messages (optional)
//   - Status: The result status from the charge point or server
//   - Message: Human-readable description of the result
//
// Possible Status Values:
//   - "Accepted": Charge point accepted the trigger request and will send the message
//   - "Rejected": Charge point rejected the trigger request (e.g., not in appropriate state)
//   - "NotImplemented": Charge point doesn't support the requested message type
//   - "Timeout": Charge point didn't respond within the configured timeout period
//
// Important Notes:
//   - A status of "Accepted" means the charge point will send the requested message separately
//   - The actual triggered message (e.g., StatusNotification) is sent as a different OCPP message
//   - This response only indicates whether the trigger request was accepted, not the content
//     of the triggered message
//   - Timeout responses indicate network or charge point communication issues
//
// Usage in API Response:
//   {
//     "success": true,
//     "message": "Trigger message sent successfully",
//     "data": {
//       "requestId": "1697360400123456789",
//       "clientId": "CP001",
//       "requestedMessage": "StatusNotification",
//       "connectorId": 1,
//       "status": "Accepted",
//       "message": "TriggerMessage accepted by charge point"
//     }
//   }
type TriggerMessageResponse struct {
	RequestID        string `json:"requestId"`
	ClientID         string `json:"clientId"`
	RequestedMessage string `json:"requestedMessage"`
	ConnectorID      *int   `json:"connectorId,omitempty"`
	Status           string `json:"status"`
	Message          string `json:"message"`
}