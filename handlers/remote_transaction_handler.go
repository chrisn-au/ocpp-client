package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/lorenzodonini/ocpp-go/ocpp"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/lorenzodonini/ocpp-go/ocppj"
)

// RemoteTransactionRequest structures
type RemoteStartRequest struct {
	ClientID    string `json:"clientId" validate:"required"`
	ConnectorID *int   `json:"connectorId,omitempty"`
	IdTag       string `json:"idTag" validate:"required,max=20"`
}

type RemoteStopRequest struct {
	ClientID      string `json:"clientId,omitempty"`
	TransactionID int    `json:"transactionId" validate:"required,min=1"`
}

type RemoteTransactionResult struct {
	RequestID   string `json:"requestId"`
	ClientID    string `json:"clientId"`
	ConnectorID int    `json:"connectorId"`
	Status      string `json:"status"`
	Message     string `json:"message"`
}



// Interfaces for dependency injection
type OCPPServerInterface interface {
	SendRequest(clientID string, request ocpp.Request) error
}

type TransportInterface interface {
	GetConnectedClients() []string
}

type OCPPBusinessStateInterface interface {
	GetActiveTransactions(clientID string) ([]*ocppj.TransactionInfo, error)
}

type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// LiveConfigResponse represents response data for pending requests
type LiveConfigResponse struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// PendingRequestManager interface for managing pending requests
type PendingRequestManager interface {
	AddPendingRequest(requestID, clientID, requestType string) chan LiveConfigResponse
	CleanupPendingRequest(requestID string)
	SendPendingResponse(clientID, requestType string, response LiveConfigResponse)
}

// RemoteTransactionHandler handles remote transaction control operations
type RemoteTransactionHandler struct {
	ocppServer         OCPPServerInterface
	transport          TransportInterface
	businessState      OCPPBusinessStateInterface
	requestManager     PendingRequestManager
	timeout            time.Duration
}

// NewRemoteTransactionHandler creates a new handler instance
func NewRemoteTransactionHandler(
	ocppServer OCPPServerInterface,
	transport TransportInterface,
	businessState OCPPBusinessStateInterface,
	requestManager PendingRequestManager,
) *RemoteTransactionHandler {
	return &RemoteTransactionHandler{
		ocppServer:     ocppServer,
		transport:      transport,
		businessState:  businessState,
		requestManager: requestManager,
		timeout:        10 * time.Second,
	}
}

// RegisterRoutes registers the remote transaction endpoints
func (h *RemoteTransactionHandler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/v1/transactions/remote-start", h.HandleRemoteStartTransaction).Methods("POST")
	router.HandleFunc("/api/v1/transactions/remote-stop", h.HandleRemoteStopTransaction).Methods("POST")
}

// HandleRemoteStartTransaction handles remote start transaction requests
func (h *RemoteTransactionHandler) HandleRemoteStartTransaction(w http.ResponseWriter, r *http.Request) {
	var req RemoteStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendJSONResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if req.ClientID == "" || req.IdTag == "" {
		h.sendJSONResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "clientId and idTag are required",
		})
		return
	}

	// Check if client is connected
	if !h.isClientConnected(req.ClientID) {
		h.sendJSONResponse(w, http.StatusNotFound, APIResponse{
			Success: false,
			Message: "Client not connected",
		})
		return
	}

	// Default connector ID to 1 if not specified
	connectorID := 1
	if req.ConnectorID != nil {
		connectorID = *req.ConnectorID
	}

	// Create OCPP RemoteStartTransaction request
	request := core.NewRemoteStartTransactionRequest(req.IdTag)
	request.ConnectorId = &connectorID

	log.Printf("REMOTE_START: Sending RemoteStartTransaction to %s - Connector: %d, IdTag: %s",
		req.ClientID, connectorID, req.IdTag)

	// Generate request ID for correlation
	requestID := h.generateRequestID()
	correlationKey := fmt.Sprintf("%s:RemoteStartTransaction:%s", req.ClientID, requestID)
	responseChan := h.requestManager.AddPendingRequest(correlationKey, req.ClientID, "RemoteStartTransaction")

	// Send request
	err := h.ocppServer.SendRequest(req.ClientID, request)
	if err != nil {
		log.Printf("REMOTE_START: Error sending to %s: %v", req.ClientID, err)
		h.requestManager.CleanupPendingRequest(correlationKey)

		h.sendJSONResponse(w, http.StatusServiceUnavailable, APIResponse{
			Success: false,
			Message: "Failed to send request to charge point",
		})
		return
	}

	// Wait for response with timeout
	h.waitForResponse(w, responseChan, RemoteTransactionResult{
		RequestID:   requestID,
		ClientID:    req.ClientID,
		ConnectorID: connectorID,
		Status:      "",
		Message:     "",
	}, "RemoteStartTransaction")
}

// HandleRemoteStopTransaction handles remote stop transaction requests
func (h *RemoteTransactionHandler) HandleRemoteStopTransaction(w http.ResponseWriter, r *http.Request) {
	var req RemoteStopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendJSONResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if req.TransactionID <= 0 {
		h.sendJSONResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Valid transactionId is required",
		})
		return
	}

	// Find which client has this transaction if not provided
	clientID := req.ClientID
	if clientID == "" {
		var err error
		clientID, err = h.findClientByTransactionID(req.TransactionID)
		if err != nil {
			h.sendJSONResponse(w, http.StatusNotFound, APIResponse{
				Success: false,
				Message: "Transaction not found",
			})
			return
		}
	}

	// Check if client is connected
	if !h.isClientConnected(clientID) {
		h.sendJSONResponse(w, http.StatusServiceUnavailable, APIResponse{
			Success: false,
			Message: "Client not connected",
		})
		return
	}

	// Create OCPP RemoteStopTransaction request
	request := core.NewRemoteStopTransactionRequest(req.TransactionID)

	log.Printf("REMOTE_STOP: Sending RemoteStopTransaction to %s - Transaction: %d",
		clientID, req.TransactionID)

	// Generate request ID for correlation
	requestID := h.generateRequestID()
	correlationKey := fmt.Sprintf("%s:RemoteStopTransaction:%s", clientID, requestID)
	responseChan := h.requestManager.AddPendingRequest(correlationKey, clientID, "RemoteStopTransaction")

	// Send request
	err := h.ocppServer.SendRequest(clientID, request)
	if err != nil {
		log.Printf("REMOTE_STOP: Error sending to %s: %v", clientID, err)
		h.requestManager.CleanupPendingRequest(correlationKey)

		h.sendJSONResponse(w, http.StatusServiceUnavailable, APIResponse{
			Success: false,
			Message: "Failed to send request to charge point",
		})
		return
	}

	// Wait for response with timeout
	h.waitForResponse(w, responseChan, RemoteTransactionResult{
		RequestID:   requestID,
		ClientID:    clientID,
		ConnectorID: 0,
		Status:      "",
		Message:     "",
	}, "RemoteStopTransaction")
}

// HandleRemoteStartTransactionResponse processes RemoteStartTransaction confirmations
func (h *RemoteTransactionHandler) HandleRemoteStartTransactionResponse(clientID, requestId string, res *core.RemoteStartTransactionConfirmation) {
	log.Printf("RemoteStartTransaction response from %s: Status=%s", clientID, res.Status)

	responseData := map[string]interface{}{
		"status":   string(res.Status),
		"clientID": clientID,
	}

	h.requestManager.SendPendingResponse(clientID, "RemoteStartTransaction", LiveConfigResponse{
		Success: res.Status == types.RemoteStartStopStatusAccepted,
		Data:    responseData,
	})
}

// HandleRemoteStopTransactionResponse processes RemoteStopTransaction confirmations
func (h *RemoteTransactionHandler) HandleRemoteStopTransactionResponse(clientID, requestId string, res *core.RemoteStopTransactionConfirmation) {
	log.Printf("RemoteStopTransaction response from %s: Status=%s", clientID, res.Status)

	responseData := map[string]interface{}{
		"status":   string(res.Status),
		"clientID": clientID,
	}

	h.requestManager.SendPendingResponse(clientID, "RemoteStopTransaction", LiveConfigResponse{
		Success: res.Status == types.RemoteStartStopStatusAccepted,
		Data:    responseData,
	})
}

// Helper methods

func (h *RemoteTransactionHandler) isClientConnected(clientID string) bool {
	connectedClients := h.transport.GetConnectedClients()
	for _, client := range connectedClients {
		if client == clientID {
			return true
		}
	}
	return false
}

func (h *RemoteTransactionHandler) findClientByTransactionID(transactionID int) (string, error) {
	transactions, err := h.businessState.GetActiveTransactions("")
	if err != nil {
		return "", err
	}

	for _, transaction := range transactions {
		if transaction.TransactionID == transactionID {
			return transaction.ClientID, nil
		}
	}

	return "", fmt.Errorf("transaction %d not found", transactionID)
}

func (h *RemoteTransactionHandler) generateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}



func (h *RemoteTransactionHandler) waitForResponse(w http.ResponseWriter, responseChan chan LiveConfigResponse, result RemoteTransactionResult, operation string) {
	select {
	case liveResponse := <-responseChan:
		if liveResponse.Success {
			result.Status = "accepted"
			result.Message = fmt.Sprintf("%s accepted by charge point", operation)
			log.Printf("%s: Accepted by %s", operation, result.ClientID)
			h.sendJSONResponse(w, http.StatusOK, APIResponse{
				Success: true,
				Message: fmt.Sprintf("Remote %s successful", operation),
				Data:    result,
			})
		} else {
			result.Status = "rejected"
			result.Message = fmt.Sprintf("%s rejected by charge point", operation)
			log.Printf("%s: Rejected by %s", operation, result.ClientID)
			h.sendJSONResponse(w, http.StatusOK, APIResponse{
				Success: false,
				Message: fmt.Sprintf("Remote %s rejected", operation),
				Data:    result,
			})
		}

	case <-time.After(h.timeout):
		log.Printf("%s: Timeout waiting for response from %s", operation, result.ClientID)
		result.Status = "timeout"
		result.Message = "Request timeout"
		h.sendJSONResponse(w, http.StatusRequestTimeout, APIResponse{
			Success: false,
			Message: "Timeout waiting for charge point response",
			Data:    result,
		})
	}
}

func (h *RemoteTransactionHandler) sendJSONResponse(w http.ResponseWriter, statusCode int, response APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}