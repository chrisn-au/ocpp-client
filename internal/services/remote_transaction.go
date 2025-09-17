package services

import (
	"fmt"
	"log"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocppj"

	"ocpp-server/internal/correlation"
	"ocpp-server/internal/helpers"
	"ocpp-server/internal/types"
)

const (
	remoteTransactionTimeout = 10 * time.Second
)

// RemoteTransactionService handles remote transaction business logic
type RemoteTransactionService struct {
	ocppServer         *ocppj.Server
	chargePointService *ChargePointService
	correlationManager *correlation.Manager
}

// NewRemoteTransactionService creates a new remote transaction service
func NewRemoteTransactionService(
	ocppServer *ocppj.Server,
	chargePointService *ChargePointService,
	correlationManager *correlation.Manager,
) *RemoteTransactionService {
	return &RemoteTransactionService{
		ocppServer:         ocppServer,
		chargePointService: chargePointService,
		correlationManager: correlationManager,
	}
}

// RemoteStartResult represents the result of a remote start operation
type RemoteStartResult struct {
	RequestID   string `json:"requestId"`
	ClientID    string `json:"clientId"`
	ConnectorID int    `json:"connectorId"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

// RemoteStopResult represents the result of a remote stop operation
type RemoteStopResult struct {
	RequestID   string `json:"requestId"`
	ClientID    string `json:"clientId"`
	ConnectorID int    `json:"connectorId"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}

// StartRemoteTransaction initiates a remote start transaction
func (s *RemoteTransactionService) StartRemoteTransaction(clientID string, connectorID *int, idTag string) (chan types.LiveConfigResponse, *RemoteStartResult, error) {
	// Check if client is connected
	if !s.chargePointService.IsOnline(clientID) {
		return nil, nil, fmt.Errorf("client not connected")
	}

	// Default connector ID to 1 if not specified
	connID := 1
	if connectorID != nil {
		connID = *connectorID
	}

	// Create OCPP RemoteStartTransaction request
	request := core.NewRemoteStartTransactionRequest(idTag)
	request.ConnectorId = &connID

	log.Printf("REMOTE_START: Sending RemoteStartTransaction to %s - Connector: %d, IdTag: %s",
		clientID, connID, idTag)

	// Generate request ID for correlation
	requestID := helpers.GenerateRequestID()
	correlationKey := fmt.Sprintf("%s:RemoteStartTransaction:%s", clientID, requestID)
	responseChan := s.correlationManager.AddPendingRequest(correlationKey, clientID, "RemoteStartTransaction")

	// Send request
	err := s.ocppServer.SendRequest(clientID, request)
	if err != nil {
		log.Printf("REMOTE_START: Error sending to %s: %v", clientID, err)
		s.correlationManager.CleanupPendingRequest(correlationKey)
		return nil, nil, fmt.Errorf("failed to send request to charge point: %w", err)
	}

	result := &RemoteStartResult{
		RequestID:   requestID,
		ClientID:    clientID,
		ConnectorID: connID,
	}

	return responseChan, result, nil
}

// StopRemoteTransaction initiates a remote stop transaction
func (s *RemoteTransactionService) StopRemoteTransaction(clientID string, transactionID int) (chan types.LiveConfigResponse, *RemoteStopResult, error) {
	// Check if client is connected
	if !s.chargePointService.IsOnline(clientID) {
		return nil, nil, fmt.Errorf("client not connected")
	}

	// Create OCPP RemoteStopTransaction request
	request := core.NewRemoteStopTransactionRequest(transactionID)

	log.Printf("REMOTE_STOP: Sending RemoteStopTransaction to %s - Transaction: %d",
		clientID, transactionID)

	// Generate request ID for correlation
	requestID := helpers.GenerateRequestID()
	correlationKey := fmt.Sprintf("%s:RemoteStopTransaction:%s", clientID, requestID)
	responseChan := s.correlationManager.AddPendingRequest(correlationKey, clientID, "RemoteStopTransaction")

	// Send request
	err := s.ocppServer.SendRequest(clientID, request)
	if err != nil {
		log.Printf("REMOTE_STOP: Error sending to %s: %v", clientID, err)
		s.correlationManager.CleanupPendingRequest(correlationKey)
		return nil, nil, fmt.Errorf("failed to send request to charge point: %w", err)
	}

	result := &RemoteStopResult{
		RequestID:   requestID,
		ClientID:    clientID,
		ConnectorID: 0, // Not applicable for stop requests
	}

	return responseChan, result, nil
}

// GetTimeout returns the timeout for remote transaction operations
func (s *RemoteTransactionService) GetTimeout() time.Duration {
	return remoteTransactionTimeout
}