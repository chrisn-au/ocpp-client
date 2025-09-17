package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"ocpp-server/internal/api/v1/models"
	"ocpp-server/internal/helpers"
	"ocpp-server/internal/services"
)

const (
	remoteTransactionTimeout = 10 * time.Second
)

// TransactionsHandler handles transaction related requests
type TransactionsHandler struct {
	transactionService       *services.TransactionService
	chargePointService       *services.ChargePointService
	remoteTransactionService *services.RemoteTransactionService
}

// NewTransactionsHandler creates a new transactions handler
func NewTransactionsHandler(
	transactionService *services.TransactionService,
	chargePointService *services.ChargePointService,
	remoteTransactionService *services.RemoteTransactionService,
) *TransactionsHandler {
	return &TransactionsHandler{
		transactionService:       transactionService,
		chargePointService:       chargePointService,
		remoteTransactionService: remoteTransactionService,
	}
}

// GetTransactions handles requests to get transactions
func (h *TransactionsHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("clientId") // Optional filter
	status := r.URL.Query().Get("status")     // Optional status filter: "active", "all"

	var transactions []interface{}
	var err error
	var message string

	if status == "all" {
		transactions, err = h.transactionService.GetAllTransactions(clientID)
		message = "All transactions retrieved"
	} else {
		// Default to active transactions only
		transactions, err = h.transactionService.GetActiveTransactions(clientID)
		message = "Transactions retrieved"
	}

	if err != nil {
		response := models.APIResponse{
			Success: false,
			Message: "Failed to retrieve transactions",
		}
		helpers.SendJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	responseData := models.TransactionsResponse{
		Transactions: transactions,
		Count:        len(transactions),
	}

	response := models.APIResponse{
		Success: true,
		Message: message,
		Data:    responseData,
	}
	helpers.SendJSONResponse(w, http.StatusOK, response)
}

// GetTransaction handles requests to get a specific transaction
func (h *TransactionsHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	transactionIDStr := vars["transactionID"]

	transactionID, err := strconv.Atoi(transactionIDStr)
	if err != nil {
		response := models.APIResponse{
			Success: false,
			Message: "Invalid transaction ID",
		}
		helpers.SendJSONResponse(w, http.StatusBadRequest, response)
		return
	}

	transaction, err := h.transactionService.GetTransaction(transactionID)
	if err != nil {
		response := models.APIResponse{
			Success: false,
			Message: "Failed to retrieve transaction",
		}
		helpers.SendJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	if transaction == nil {
		response := models.APIResponse{
			Success: false,
			Message: "Transaction not found",
		}
		helpers.SendJSONResponse(w, http.StatusNotFound, response)
		return
	}

	response := models.APIResponse{
		Success: true,
		Message: "Transaction retrieved",
		Data:    transaction,
	}
	helpers.SendJSONResponse(w, http.StatusOK, response)
}

// RemoteStartTransaction handles remote start transaction requests
func (h *TransactionsHandler) RemoteStartTransaction(w http.ResponseWriter, r *http.Request) {
	var req models.RemoteStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := models.APIResponse{
			Success: false,
			Message: "Invalid request body",
		}
		helpers.SendJSONResponse(w, http.StatusBadRequest, response)
		return
	}

	// Validate required fields
	if req.ClientID == "" || req.IdTag == "" {
		response := models.APIResponse{
			Success: false,
			Message: "clientId and idTag are required",
		}
		helpers.SendJSONResponse(w, http.StatusBadRequest, response)
		return
	}

	// Use the remote transaction service
	responseChan, result, err := h.remoteTransactionService.StartRemoteTransaction(req.ClientID, req.ConnectorID, req.IdTag)
	if err != nil {
		statusCode := http.StatusServiceUnavailable
		if err.Error() == "client not connected" {
			statusCode = http.StatusNotFound
		}

		response := models.APIResponse{
			Success: false,
			Message: err.Error(),
		}
		helpers.SendJSONResponse(w, statusCode, response)
		return
	}

	// Wait for response with timeout
	timeout := h.remoteTransactionService.GetTimeout()
	select {
	case liveResponse := <-responseChan:
		apiResult := models.RemoteTransactionResult{
			RequestID:   result.RequestID,
			ClientID:    result.ClientID,
			ConnectorID: result.ConnectorID,
		}

		if liveResponse.Success {
			apiResult.Status = "accepted"
			apiResult.Message = "RemoteStartTransaction accepted by charge point"

			helpers.SendJSONResponse(w, http.StatusOK, models.APIResponse{
				Success: true,
				Message: "Remote start transaction successful",
				Data:    apiResult,
			})
		} else {
			apiResult.Status = "rejected"
			apiResult.Message = "RemoteStartTransaction rejected by charge point"

			helpers.SendJSONResponse(w, http.StatusOK, models.APIResponse{
				Success: false,
				Message: "Remote start transaction rejected",
				Data:    apiResult,
			})
		}

	case <-time.After(timeout):
		apiResult := models.RemoteTransactionResult{
			RequestID:   result.RequestID,
			ClientID:    result.ClientID,
			ConnectorID: result.ConnectorID,
			Status:      "timeout",
			Message:     "Request timeout",
		}
		helpers.SendJSONResponse(w, http.StatusRequestTimeout, models.APIResponse{
			Success: false,
			Message: "Timeout waiting for charge point response",
			Data:    apiResult,
		})
	}
}

// RemoteStopTransaction handles remote stop transaction requests
func (h *TransactionsHandler) RemoteStopTransaction(w http.ResponseWriter, r *http.Request) {
	var req models.RemoteStopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := models.APIResponse{
			Success: false,
			Message: "Invalid request body",
		}
		helpers.SendJSONResponse(w, http.StatusBadRequest, response)
		return
	}

	if req.TransactionID <= 0 {
		response := models.APIResponse{
			Success: false,
			Message: "Valid transactionId is required",
		}
		helpers.SendJSONResponse(w, http.StatusBadRequest, response)
		return
	}

	// If clientId not provided, try to find it from transaction
	clientID := req.ClientID
	if clientID == "" {
		// TODO: Implement transaction lookup to find clientID
		// For now, return an error
		response := models.APIResponse{
			Success: false,
			Message: "clientId is required",
		}
		helpers.SendJSONResponse(w, http.StatusBadRequest, response)
		return
	}

	// Use the remote transaction service
	responseChan, result, err := h.remoteTransactionService.StopRemoteTransaction(clientID, req.TransactionID)
	if err != nil {
		statusCode := http.StatusServiceUnavailable
		if err.Error() == "client not connected" {
			statusCode = http.StatusServiceUnavailable
		}

		response := models.APIResponse{
			Success: false,
			Message: err.Error(),
		}
		helpers.SendJSONResponse(w, statusCode, response)
		return
	}

	// Wait for response with timeout
	timeout := h.remoteTransactionService.GetTimeout()
	select {
	case liveResponse := <-responseChan:
		apiResult := models.RemoteTransactionResult{
			RequestID:   result.RequestID,
			ClientID:    result.ClientID,
			ConnectorID: result.ConnectorID,
		}

		if liveResponse.Success {
			apiResult.Status = "accepted"
			apiResult.Message = "RemoteStopTransaction accepted by charge point"

			helpers.SendJSONResponse(w, http.StatusOK, models.APIResponse{
				Success: true,
				Message: "Remote stop transaction successful",
				Data:    apiResult,
			})
		} else {
			apiResult.Status = "rejected"
			apiResult.Message = "RemoteStopTransaction rejected by charge point"

			helpers.SendJSONResponse(w, http.StatusOK, models.APIResponse{
				Success: false,
				Message: "Remote stop transaction rejected",
				Data:    apiResult,
			})
		}

	case <-time.After(timeout):
		apiResult := models.RemoteTransactionResult{
			RequestID:   result.RequestID,
			ClientID:    result.ClientID,
			ConnectorID: result.ConnectorID,
			Status:      "timeout",
			Message:     "Request timeout",
		}
		helpers.SendJSONResponse(w, http.StatusRequestTimeout, models.APIResponse{
			Success: false,
			Message: "Timeout waiting for charge point response",
			Data:    apiResult,
		})
	}
}