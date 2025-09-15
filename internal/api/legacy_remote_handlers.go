package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/lorenzodonini/ocpp-go/transport"

	"ocpp-server/internal/correlation"
	"ocpp-server/internal/helpers"
)

// RemoteStartHandler handles legacy remote start transaction requests
func RemoteStartHandler(
	redisTransport transport.Transport,
	ocppServer *ocppj.Server,
	correlationManager *correlation.Manager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		clientID := vars["clientID"]

		var req RemoteStartRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response := APIResponse{
				Success: false,
				Message: "Invalid request body",
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			return
		}

		if req.IdTag == "" {
			response := APIResponse{
				Success: false,
				Message: "idTag is required",
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			return
		}

		// Check if client is connected
		if !IsChargerOnline(redisTransport, clientID) {
			response := APIResponse{
				Success: false,
				Message: "Client not connected",
			}
			helpers.SendJSONResponse(w, http.StatusNotFound, response)
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
			clientID, connectorID, req.IdTag)

		// Generate request ID for correlation
		requestID := helpers.GenerateRequestID()
		correlationKey := fmt.Sprintf("%s:RemoteStartTransaction:%s", clientID, requestID)
		responseChan := correlationManager.AddPendingRequest(correlationKey, clientID, "RemoteStartTransaction")

		// Send request
		err := ocppServer.SendRequest(clientID, request)
		if err != nil {
			log.Printf("REMOTE_START: Error sending to %s: %v", clientID, err)
			// Clean up pending request on error
			correlationManager.CleanupPendingRequest(correlationKey)

			response := APIResponse{
				Success: false,
				Message: "Failed to send request to charge point",
			}
			helpers.SendJSONResponse(w, http.StatusServiceUnavailable, response)
			return
		}

		// Wait for response with timeout
		select {
		case liveResponse := <-responseChan:
			if liveResponse.Success {
				result := RemoteTransactionResult{
					RequestID:   requestID,
					ClientID:    clientID,
					ConnectorID: connectorID,
					Status:      "accepted",
					Message:     "RemoteStartTransaction accepted by charge point",
				}
				log.Printf("REMOTE_START: Accepted by %s", clientID)
				helpers.SendJSONResponse(w, http.StatusOK, APIResponse{
					Success: true,
					Message: "Remote start transaction successful",
					Data:    result,
				})
			} else {
				result := RemoteTransactionResult{
					RequestID:   requestID,
					ClientID:    clientID,
					ConnectorID: connectorID,
					Status:      "rejected",
					Message:     "RemoteStartTransaction rejected by charge point",
				}
				log.Printf("REMOTE_START: Rejected by %s", clientID)
				helpers.SendJSONResponse(w, http.StatusOK, APIResponse{
					Success: false,
					Message: "Remote start transaction rejected",
					Data:    result,
				})
			}

		case <-time.After(liveConfigTimeout):
			log.Printf("REMOTE_START: Timeout waiting for response from %s", clientID)
			result := RemoteTransactionResult{
				RequestID:   requestID,
				ClientID:    clientID,
				ConnectorID: connectorID,
				Status:      "timeout",
				Message:     "Request timeout",
			}
			helpers.SendJSONResponse(w, http.StatusRequestTimeout, APIResponse{
				Success: false,
				Message: "Timeout waiting for charge point response",
				Data:    result,
			})
		}
	}
}

// RemoteStopHandler handles legacy remote stop transaction requests
func RemoteStopHandler(
	redisTransport transport.Transport,
	ocppServer *ocppj.Server,
	correlationManager *correlation.Manager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		clientID := vars["clientID"]

		var req RemoteStopRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			response := APIResponse{
				Success: false,
				Message: "Invalid request body",
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			return
		}

		if req.TransactionID <= 0 {
			response := APIResponse{
				Success: false,
				Message: "Valid transactionId is required",
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			return
		}

		// Check if client is connected
		if !IsChargerOnline(redisTransport, clientID) {
			response := APIResponse{
				Success: false,
				Message: "Client not connected",
			}
			helpers.SendJSONResponse(w, http.StatusServiceUnavailable, response)
			return
		}

		// Create OCPP RemoteStopTransaction request
		request := core.NewRemoteStopTransactionRequest(req.TransactionID)

		log.Printf("REMOTE_STOP: Sending RemoteStopTransaction to %s - Transaction: %d",
			clientID, req.TransactionID)

		// Generate request ID for correlation
		requestID := helpers.GenerateRequestID()
		correlationKey := fmt.Sprintf("%s:RemoteStopTransaction:%s", clientID, requestID)
		responseChan := correlationManager.AddPendingRequest(correlationKey, clientID, "RemoteStopTransaction")

		// Send request
		err := ocppServer.SendRequest(clientID, request)
		if err != nil {
			log.Printf("REMOTE_STOP: Error sending to %s: %v", clientID, err)
			// Clean up pending request on error
			correlationManager.CleanupPendingRequest(correlationKey)

			response := APIResponse{
				Success: false,
				Message: "Failed to send request to charge point",
			}
			helpers.SendJSONResponse(w, http.StatusServiceUnavailable, response)
			return
		}

		// Wait for response with timeout
		select {
		case liveResponse := <-responseChan:
			if liveResponse.Success {
				result := RemoteTransactionResult{
					RequestID:   requestID,
					ClientID:    clientID,
					ConnectorID: 0,
					Status:      "accepted",
					Message:     "RemoteStopTransaction accepted by charge point",
				}
				log.Printf("REMOTE_STOP: Accepted by %s for transaction %d", clientID, req.TransactionID)
				helpers.SendJSONResponse(w, http.StatusOK, APIResponse{
					Success: true,
					Message: "Remote stop transaction successful",
					Data:    result,
				})
			} else {
				result := RemoteTransactionResult{
					RequestID:   requestID,
					ClientID:    clientID,
					ConnectorID: 0,
					Status:      "rejected",
					Message:     "RemoteStopTransaction rejected by charge point",
				}
				log.Printf("REMOTE_STOP: Rejected by %s for transaction %d", clientID, req.TransactionID)
				helpers.SendJSONResponse(w, http.StatusOK, APIResponse{
					Success: false,
					Message: "Remote stop transaction rejected",
					Data:    result,
				})
			}

		case <-time.After(liveConfigTimeout):
			log.Printf("REMOTE_STOP: Timeout waiting for response from %s", clientID)
			result := RemoteTransactionResult{
				RequestID:   requestID,
				ClientID:    clientID,
				ConnectorID: 0,
				Status:      "timeout",
				Message:     "Request timeout",
			}
			helpers.SendJSONResponse(w, http.StatusRequestTimeout, APIResponse{
				Success: false,
				Message: "Timeout waiting for charge point response",
				Data:    result,
			})
		}
	}
}