package ocpp

import (
	"fmt"
	"log"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"

	"ocpp-server/internal/correlation"
	"ocpp-server/internal/types"
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
		case foundRequest.Channel <- types.LiveConfigResponse{
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
	correlationManager.SendLiveResponse(correlationKey, types.LiveConfigResponse{
		Success: string(res.Status) == "Accepted",
		Data:    responseData,
	})
}