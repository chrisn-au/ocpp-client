package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"ocpp-server/internal/api/v1/models"
	"ocpp-server/internal/helpers"
	"ocpp-server/internal/services"
)

const (
	triggerMessageTimeout = 10 * time.Second
)

// TriggerMessageHandler creates an HTTP handler for processing TriggerMessage requests.
//
// This handler implements the OCPP 1.6 TriggerMessage feature, allowing the Central System
// to request specific messages from charge points on demand. This is useful for:
//
//   - Immediate status updates without waiting for scheduled notifications
//   - Diagnostic information gathering during troubleshooting
//   - Testing charge point connectivity and responsiveness
//   - Forcing meter value updates for real-time monitoring
//
// The handler supports the following message types:
//   - StatusNotification: Request current connector or charge point status
//   - Heartbeat: Test basic connectivity and responsiveness
//   - MeterValues: Request current meter readings from specific connectors
//   - BootNotification: Request charge point information and capabilities
//
// Request Flow:
//  1. Validates client ID from URL path
//  2. Parses and validates JSON request body
//  3. Checks message type support and connector ID validity
//  4. Sends TriggerMessage OCPP request via correlation manager
//  5. Waits for charge point response with configurable timeout
//  6. Returns HTTP response indicating acceptance, rejection, or timeout
//
// HTTP Status Codes:
//   - 200 OK: Request successfully processed (accepted or rejected by charge point)
//   - 400 Bad Request: Invalid request parameters or unsupported message type
//   - 404 Not Found: Charge point not connected
//   - 408 Request Timeout: Charge point did not respond within timeout
//   - 503 Service Unavailable: Server error sending request
//
// Usage Example:
//   POST /api/v1/chargepoints/CP001/trigger
//   {
//     "requestedMessage": "StatusNotification",
//     "connectorId": 1
//   }
//
// The triggerMessageService parameter provides the business logic for sending
// TriggerMessage requests and managing correlation between requests and responses.
func TriggerMessageHandler(
	triggerMessageService *services.TriggerMessageService,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Parse client ID from URL path
		vars := mux.Vars(r)
		clientID := vars["clientID"]
		if clientID == "" {
			log.Printf("TRIGGER_MESSAGE: Client ID is required in URL path")
			response := models.APIResponse{
				Success: false,
				Message: "Client ID is required in URL path",
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			return
		}

		// Parse request body
		var req models.TriggerMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Printf("TRIGGER_MESSAGE: Invalid request body for client %s: %v", clientID, err)
			response := models.APIResponse{
				Success: false,
				Message: "Invalid request body",
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			return
		}

		// Validate required fields
		if req.RequestedMessage == "" {
			log.Printf("TRIGGER_MESSAGE: RequestedMessage is required for client %s", clientID)
			response := models.APIResponse{
				Success: false,
				Message: "requestedMessage is required",
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			return
		}

		// Validate supported message types
		supportedMessages := map[string]bool{
			"StatusNotification": true,
			"Heartbeat":          true,
			"MeterValues":        true,
			"BootNotification":   true,
		}
		if !supportedMessages[req.RequestedMessage] {
			log.Printf("TRIGGER_MESSAGE: Unsupported message type %s for client %s", req.RequestedMessage, clientID)
			response := models.APIResponse{
				Success: false,
				Message: "Unsupported message type. Supported types: StatusNotification, Heartbeat, MeterValues, BootNotification",
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			return
		}

		// Validate connector ID for messages that support it
		if req.ConnectorID != nil && *req.ConnectorID < 0 {
			log.Printf("TRIGGER_MESSAGE: Invalid connector ID %d for client %s", *req.ConnectorID, clientID)
			response := models.APIResponse{
				Success: false,
				Message: "connectorId must be >= 0",
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			return
		}

		log.Printf("TRIGGER_MESSAGE: Processing trigger message request for client %s - Message: %s, ConnectorID: %v",
			clientID, req.RequestedMessage, req.ConnectorID)

		// Use the trigger message service
		responseChan, result, err := triggerMessageService.SendTriggerMessage(clientID, req.RequestedMessage, req.ConnectorID)
		if err != nil {
			statusCode := http.StatusServiceUnavailable
			if err.Error() == "client not connected" {
				statusCode = http.StatusNotFound
				log.Printf("TRIGGER_MESSAGE: Client %s not connected", clientID)
			} else {
				log.Printf("TRIGGER_MESSAGE: Failed to send trigger message to client %s: %v", clientID, err)
			}

			response := models.APIResponse{
				Success: false,
				Message: err.Error(),
			}
			helpers.SendJSONResponse(w, statusCode, response)
			return
		}

		// Wait for response with timeout
		timeout := triggerMessageService.GetTimeout()
		select {
		case liveResponse := <-responseChan:
			apiResult := models.TriggerMessageResponse{
				RequestID:        result.RequestID,
				ClientID:         result.ClientID,
				RequestedMessage: result.RequestedMessage,
				ConnectorID:      result.ConnectorID,
			}

			if liveResponse.Success {
				apiResult.Status = "Accepted"
				apiResult.Message = "TriggerMessage accepted by charge point"

				log.Printf("TRIGGER_MESSAGE: Successful for client %s - Message: %s, RequestID: %s",
					clientID, req.RequestedMessage, result.RequestID)

				helpers.SendJSONResponse(w, http.StatusOK, models.APIResponse{
					Success: true,
					Message: "Trigger message sent successfully",
					Data:    apiResult,
				})
			} else {
				apiResult.Status = "Rejected"
				apiResult.Message = "TriggerMessage rejected by charge point"

				log.Printf("TRIGGER_MESSAGE: Rejected for client %s - Message: %s, RequestID: %s",
					clientID, req.RequestedMessage, result.RequestID)

				helpers.SendJSONResponse(w, http.StatusOK, models.APIResponse{
					Success: false,
					Message: "Trigger message rejected by charge point",
					Data:    apiResult,
				})
			}

		case <-time.After(timeout):
			apiResult := models.TriggerMessageResponse{
				RequestID:        result.RequestID,
				ClientID:         result.ClientID,
				RequestedMessage: result.RequestedMessage,
				ConnectorID:      result.ConnectorID,
				Status:           "Timeout",
				Message:          "Request timeout",
			}

			log.Printf("TRIGGER_MESSAGE: Timeout for client %s - Message: %s, RequestID: %s, Timeout: %v",
				clientID, req.RequestedMessage, result.RequestID, timeout)

			helpers.SendJSONResponse(w, http.StatusRequestTimeout, models.APIResponse{
				Success: false,
				Message: "Timeout waiting for charge point response",
				Data:    apiResult,
			})
		}
	}
}