package ocpp

import (
	"fmt"
	"log"

	"github.com/lorenzodonini/ocpp-go/ocpp"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/remotetrigger"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"

	"ocpp-server/internal/correlation"
	internaltypes "ocpp-server/internal/types"
)

// HandleGetConfigurationResponse processes GetConfiguration confirmation from charge points
func HandleGetConfigurationResponse(correlationManager *correlation.Manager, clientID, requestId string, res *core.GetConfigurationConfirmation) {
	log.Printf("GetConfiguration response from %s with request ID %s: %d configuration keys", clientID, requestId, len(res.ConfigurationKey))

	// Prepare response data
	configData := make(map[string]interface{})
	for _, kv := range res.ConfigurationKey {
		configData[kv.Key] = map[string]interface{}{
			"value":    *kv.Value,
			"readonly": kv.Readonly,
		}
		log.Printf("  %s: %s (readonly: %t)", kv.Key, *kv.Value, kv.Readonly)
	}

	responseData := map[string]interface{}{
		"configuration": configData,
		"clientID":      clientID,
	}

	if len(res.UnknownKey) > 0 {
		log.Printf("Unknown keys from %s: %v", clientID, res.UnknownKey)
		responseData["unknownKeys"] = res.UnknownKey
	}

	// Find pending request by client and type (since we can't reliably match request IDs)
	foundKey, foundRequest := correlationManager.FindPendingRequest(clientID, "GetConfiguration")

	if foundRequest != nil {
		log.Printf("RESPONSE_HANDLER: Found pending request %s for client %s", foundKey, clientID)

		// Send response to waiting HTTP handler
		select {
		case foundRequest.Channel <- internaltypes.LiveConfigResponse{
			Success: true,
			Data:    responseData,
		}:
			log.Printf("RESPONSE_HANDLER: Response sent for %s", foundKey)
		default:
			log.Printf("RESPONSE_HANDLER: Channel blocked for %s", foundKey)
		}

		// Clean up the pending request
		correlationManager.DeletePendingRequest(foundKey)
	} else {
		log.Printf("RESPONSE_HANDLER: No pending GetConfiguration request found for client %s", clientID)
	}
}

// HandleChangeConfigurationResponse processes ChangeConfiguration confirmation from charge points
func HandleChangeConfigurationResponse(correlationManager *correlation.Manager, clientID, requestId string, res *core.ChangeConfigurationConfirmation) {
	log.Printf("ChangeConfiguration response from %s: Status=%s", clientID, res.Status)

	responseData := map[string]interface{}{
		"status":   string(res.Status),
		"clientID": clientID,
	}

	// Use correlation key instead of OCPP request ID
	correlationKey := fmt.Sprintf("%s:ChangeConfiguration", clientID)
	log.Printf("RESPONSE_HANDLER: Using correlation key %s for ChangeConfiguration response", correlationKey)

	// Send response to waiting HTTP handler
	correlationManager.SendLiveResponse(correlationKey, internaltypes.LiveConfigResponse{
		Success: string(res.Status) == "Accepted",
		Data:    responseData,
	})
}

// HandleRemoteStartTransactionResponse processes RemoteStartTransaction confirmation from charge points
func HandleRemoteStartTransactionResponse(correlationManager *correlation.Manager, clientID, requestId string, res *core.RemoteStartTransactionConfirmation) {
	log.Printf("RemoteStartTransaction response from %s: Status=%s", clientID, res.Status)

	responseData := map[string]interface{}{
		"status":   string(res.Status),
		"clientID": clientID,
	}

	// Find pending request by client and type
	foundKey, foundRequest := correlationManager.FindPendingRequest(clientID, "RemoteStartTransaction")

	if foundRequest != nil {
		log.Printf("RESPONSE_HANDLER: Found pending RemoteStartTransaction request %s for client %s", foundKey, clientID)

		// Send response to waiting HTTP handler
		select {
		case foundRequest.Channel <- internaltypes.LiveConfigResponse{
			Success: res.Status == types.RemoteStartStopStatusAccepted,
			Data:    responseData,
		}:
			log.Printf("RESPONSE_HANDLER: RemoteStartTransaction response sent for %s", foundKey)
		default:
			log.Printf("RESPONSE_HANDLER: Channel blocked for RemoteStartTransaction %s", foundKey)
		}

		// Clean up the pending request
		correlationManager.DeletePendingRequest(foundKey)
	} else {
		log.Printf("RESPONSE_HANDLER: No pending RemoteStartTransaction request found for client %s", clientID)
	}
}

// HandleRemoteStopTransactionResponse processes RemoteStopTransaction confirmation from charge points
func HandleRemoteStopTransactionResponse(correlationManager *correlation.Manager, clientID, requestId string, res *core.RemoteStopTransactionConfirmation) {
	log.Printf("RemoteStopTransaction response from %s: Status=%s", clientID, res.Status)

	responseData := map[string]interface{}{
		"status":   string(res.Status),
		"clientID": clientID,
	}

	// Find pending request by client and type
	foundKey, foundRequest := correlationManager.FindPendingRequest(clientID, "RemoteStopTransaction")

	if foundRequest != nil {
		log.Printf("RESPONSE_HANDLER: Found pending RemoteStopTransaction request %s for client %s", foundKey, clientID)

		// Send response to waiting HTTP handler
		select {
		case foundRequest.Channel <- internaltypes.LiveConfigResponse{
			Success: res.Status == types.RemoteStartStopStatusAccepted,
			Data:    responseData,
		}:
			log.Printf("RESPONSE_HANDLER: RemoteStopTransaction response sent for %s", foundKey)
		default:
			log.Printf("RESPONSE_HANDLER: Channel blocked for RemoteStopTransaction %s", foundKey)
		}

		// Clean up the pending request
		correlationManager.DeletePendingRequest(foundKey)
	} else {
		log.Printf("RESPONSE_HANDLER: No pending RemoteStopTransaction request found for client %s", clientID)
	}
}

// HandleTriggerMessageResponse processes TriggerMessage confirmation from charge points
func HandleTriggerMessageResponse(correlationManager *correlation.Manager, clientID, requestId string, res *remotetrigger.TriggerMessageConfirmation) {
	log.Printf("TriggerMessage response from %s: Status=%s", clientID, res.Status)

	responseData := map[string]interface{}{
		"status":   string(res.Status),
		"clientID": clientID,
	}

	// Find pending request by client and type
	foundKey, foundRequest := correlationManager.FindPendingRequest(clientID, "TriggerMessage")

	if foundRequest != nil {
		log.Printf("RESPONSE_HANDLER: Found pending TriggerMessage request %s for client %s", foundKey, clientID)

		// Send response to waiting HTTP handler
		select {
		case foundRequest.Channel <- internaltypes.LiveConfigResponse{
			Success: res.Status == remotetrigger.TriggerMessageStatusAccepted,
			Data:    responseData,
		}:
			log.Printf("RESPONSE_HANDLER: TriggerMessage response sent for %s", foundKey)
		default:
			log.Printf("RESPONSE_HANDLER: Channel blocked for TriggerMessage %s", foundKey)
		}

		// Clean up the pending request
		correlationManager.DeletePendingRequest(foundKey)
	} else {
		log.Printf("RESPONSE_HANDLER: No pending TriggerMessage request found for client %s", clientID)
	}
}

// HandleGetConfigurationError processes GetConfiguration error responses from charge points
func HandleGetConfigurationError(correlationManager *correlation.Manager, clientID string, err *ocpp.Error) {
	log.Printf("ERROR_HANDLER: GetConfiguration error from %s: %s", clientID, err.Error())

	// Find pending request by client and type - same pattern as success case
	foundKey, foundRequest := correlationManager.FindPendingRequest(clientID, "GetConfiguration")

	if foundRequest != nil {
		log.Printf("ERROR_HANDLER: Found pending GetConfiguration request %s for client %s", foundKey, clientID)

		// Send error response to waiting HTTP handler
		select {
		case foundRequest.Channel <- internaltypes.LiveConfigResponse{
			Success: false,
			Error:   err.Error(),
		}:
			log.Printf("ERROR_HANDLER: GetConfiguration error response sent for %s", foundKey)
		default:
			log.Printf("ERROR_HANDLER: Channel blocked for GetConfiguration %s", foundKey)
		}

		// Clean up the pending request
		correlationManager.DeletePendingRequest(foundKey)
	} else {
		log.Printf("ERROR_HANDLER: No pending GetConfiguration request found for client %s", clientID)
	}
}

// HandleChangeConfigurationError processes ChangeConfiguration error responses from charge points
func HandleChangeConfigurationError(correlationManager *correlation.Manager, clientID string, err *ocpp.Error) {
	log.Printf("ERROR_HANDLER: ChangeConfiguration error from %s: %s", clientID, err.Error())

	// Find pending request by client and type - same pattern as success case
	foundKey, foundRequest := correlationManager.FindPendingRequest(clientID, "ChangeConfiguration")

	if foundRequest != nil {
		log.Printf("ERROR_HANDLER: Found pending ChangeConfiguration request %s for client %s", foundKey, clientID)

		// Send error response to waiting HTTP handler
		select {
		case foundRequest.Channel <- internaltypes.LiveConfigResponse{
			Success: false,
			Error:   err.Error(),
		}:
			log.Printf("ERROR_HANDLER: ChangeConfiguration error response sent for %s", foundKey)
		default:
			log.Printf("ERROR_HANDLER: Channel blocked for ChangeConfiguration %s", foundKey)
		}

		// Clean up the pending request
		correlationManager.DeletePendingRequest(foundKey)
	} else {
		log.Printf("ERROR_HANDLER: No pending ChangeConfiguration request found for client %s", clientID)
	}
}

// HandleRemoteStartTransactionError processes RemoteStartTransaction error responses from charge points
func HandleRemoteStartTransactionError(correlationManager *correlation.Manager, clientID string, err *ocpp.Error) {
	log.Printf("ERROR_HANDLER: RemoteStartTransaction error from %s: %s", clientID, err.Error())

	// Find pending request by client and type - same pattern as success case
	foundKey, foundRequest := correlationManager.FindPendingRequest(clientID, "RemoteStartTransaction")

	if foundRequest != nil {
		log.Printf("ERROR_HANDLER: Found pending RemoteStartTransaction request %s for client %s", foundKey, clientID)

		// Send error response to waiting HTTP handler
		select {
		case foundRequest.Channel <- internaltypes.LiveConfigResponse{
			Success: false,
			Error:   err.Error(),
		}:
			log.Printf("ERROR_HANDLER: RemoteStartTransaction error response sent for %s", foundKey)
		default:
			log.Printf("ERROR_HANDLER: Channel blocked for RemoteStartTransaction %s", foundKey)
		}

		// Clean up the pending request
		correlationManager.DeletePendingRequest(foundKey)
	} else {
		log.Printf("ERROR_HANDLER: No pending RemoteStartTransaction request found for client %s", clientID)
	}
}

// HandleRemoteStopTransactionError processes RemoteStopTransaction error responses from charge points
func HandleRemoteStopTransactionError(correlationManager *correlation.Manager, clientID string, err *ocpp.Error) {
	log.Printf("ERROR_HANDLER: RemoteStopTransaction error from %s: %s", clientID, err.Error())

	// Find pending request by client and type - same pattern as success case
	foundKey, foundRequest := correlationManager.FindPendingRequest(clientID, "RemoteStopTransaction")

	if foundRequest != nil {
		log.Printf("ERROR_HANDLER: Found pending RemoteStopTransaction request %s for client %s", foundKey, clientID)

		// Send error response to waiting HTTP handler
		select {
		case foundRequest.Channel <- internaltypes.LiveConfigResponse{
			Success: false,
			Error:   err.Error(),
		}:
			log.Printf("ERROR_HANDLER: RemoteStopTransaction error response sent for %s", foundKey)
		default:
			log.Printf("ERROR_HANDLER: Channel blocked for RemoteStopTransaction %s", foundKey)
		}

		// Clean up the pending request
		correlationManager.DeletePendingRequest(foundKey)
	} else {
		log.Printf("ERROR_HANDLER: No pending RemoteStopTransaction request found for client %s", clientID)
	}
}

// HandleTriggerMessageError processes TriggerMessage error responses from charge points
func HandleTriggerMessageError(correlationManager *correlation.Manager, clientID string, err *ocpp.Error) {
	log.Printf("ERROR_HANDLER: TriggerMessage error from %s: %s", clientID, err.Error())

	// Find pending request by client and type - same pattern as success case
	foundKey, foundRequest := correlationManager.FindPendingRequest(clientID, "TriggerMessage")

	if foundRequest != nil {
		log.Printf("ERROR_HANDLER: Found pending TriggerMessage request %s for client %s", foundKey, clientID)

		// Send error response to waiting HTTP handler
		select {
		case foundRequest.Channel <- internaltypes.LiveConfigResponse{
			Success: false,
			Error:   err.Error(),
		}:
			log.Printf("ERROR_HANDLER: TriggerMessage error response sent for %s", foundKey)
		default:
			log.Printf("ERROR_HANDLER: Channel blocked for TriggerMessage %s", foundKey)
		}

		// Clean up the pending request
		correlationManager.DeletePendingRequest(foundKey)
	} else {
		log.Printf("ERROR_HANDLER: No pending TriggerMessage request found for client %s", clientID)
	}
}