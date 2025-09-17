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