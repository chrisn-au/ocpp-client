package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/lorenzodonini/ocpp-go/transport"

	cfgmgr "ocpp-server/config"
	"ocpp-server/internal/correlation"
	"ocpp-server/internal/helpers"
	"ocpp-server/internal/types"
)

const (
	liveConfigTimeout = 10 * time.Second
)

// GetConfigurationHandler handles requests to get stored configuration
func GetConfigurationHandler(configManager *cfgmgr.ConfigurationManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		clientID := vars["clientID"]

		// Parse query parameters for specific keys
		keysParam := r.URL.Query().Get("keys")
		var keys []string
		if keysParam != "" {
			keys = strings.Split(keysParam, ",")
			for i, key := range keys {
				keys[i] = strings.TrimSpace(key)
			}
		}

		configurationKeys, unknownKeys := configManager.GetConfiguration(clientID, keys)

		configData := make(map[string]interface{})
		for _, kv := range configurationKeys {
			configData[kv.Key] = map[string]interface{}{
				"value":    *kv.Value,
				"readonly": kv.Readonly,
			}
		}

		result := map[string]interface{}{
			"configuration": configData,
		}

		if len(unknownKeys) > 0 {
			result["unknownKeys"] = unknownKeys
		}

		response := APIResponse{
			Success: true,
			Message: "Configuration retrieved",
			Data:    result,
		}
		helpers.SendJSONResponse(w, http.StatusOK, response)
	}
}

// ChangeConfigurationHandler handles requests to change stored configuration
func ChangeConfigurationHandler(configManager *cfgmgr.ConfigurationManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		clientID := vars["clientID"]

		var requestBody struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}

		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			response := APIResponse{
				Success: false,
				Message: "Invalid request body",
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			return
		}

		status := configManager.ChangeConfiguration(clientID, requestBody.Key, requestBody.Value)

		response := APIResponse{
			Success: true,
			Message: "Configuration change processed",
			Data: map[string]interface{}{
				"status": string(status),
			},
		}
		helpers.SendJSONResponse(w, http.StatusOK, response)
	}
}

// ExportConfigurationHandler handles requests to export configuration
func ExportConfigurationHandler(configManager *cfgmgr.ConfigurationManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		clientID := vars["clientID"]

		config := configManager.ExportConfiguration(clientID)

		response := APIResponse{
			Success: true,
			Message: "Configuration exported",
			Data:    config,
		}
		helpers.SendJSONResponse(w, http.StatusOK, response)
	}
}

// GetLiveConfigurationHandler handles requests to get live configuration from charge point
func GetLiveConfigurationHandler(
	redisTransport transport.Transport,
	ocppServer *ocppj.Server,
	correlationManager *correlation.Manager,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		clientID := vars["clientID"]

		// Check if charger is online
		if !IsChargerOnline(redisTransport, clientID) {
			response := APIResponse{
				Success: false,
				Message: "Charger is offline - returning stored configuration",
				Data: map[string]interface{}{
					"online": false,
					"note":   "Falling back to stored configuration. Use /configuration endpoint for stored values.",
				},
			}
			helpers.SendJSONResponse(w, http.StatusServiceUnavailable, response)
			return
		}

		// Parse query parameters for specific keys
		keysParam := r.URL.Query().Get("keys")
		var keys []string
		if keysParam != "" {
			keys = strings.Split(keysParam, ",")
			for i, key := range keys {
				keys[i] = strings.TrimSpace(key)
			}
		}

		// Send GetConfiguration request to the live charger and wait for response
		responseChan, err := SendGetConfigurationToCharger(ocppServer, correlationManager, clientID, keys)
		if err != nil {
			log.Printf("Error sending GetConfiguration to charger %s: %v", clientID, err)
			response := APIResponse{
				Success: false,
				Message: "Failed to send request to charger",
				Data: map[string]interface{}{
					"error":  err.Error(),
					"online": true,
				},
			}
			helpers.SendJSONResponse(w, http.StatusServiceUnavailable, response)
			return
		}

		// Wait for response with timeout
		select {
		case liveResponse := <-responseChan:
			if liveResponse.Success {
				response := APIResponse{
					Success: true,
					Message: "Live configuration retrieved from charger",
					Data:    liveResponse.Data,
				}
				helpers.SendJSONResponse(w, http.StatusOK, response)
			} else {
				response := APIResponse{
					Success: false,
					Message: "Charger rejected configuration request",
					Data: map[string]interface{}{
						"error":  liveResponse.Error,
						"online": true,
					},
				}
				helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			}

		case <-time.After(liveConfigTimeout):
			log.Printf("Timeout waiting for GetConfiguration response from %s", clientID)
			response := APIResponse{
				Success: false,
				Message: "Timeout waiting for charger response",
				Data: map[string]interface{}{
					"online":  true,
					"timeout": fmt.Sprintf("%.0fs", liveConfigTimeout.Seconds()),
					"note":    "Charger did not respond within timeout. Use /configuration endpoint for stored values.",
				},
			}
			helpers.SendJSONResponse(w, http.StatusRequestTimeout, response)
		}
	}
}

// ChangeLiveConfigurationHandler handles requests to change live configuration on charge point
func ChangeLiveConfigurationHandler(
	redisTransport transport.Transport,
	ocppServer *ocppj.Server,
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		clientID := vars["clientID"]

		// Check if charger is online
		if !IsChargerOnline(redisTransport, clientID) {
			response := APIResponse{
				Success: false,
				Message: "Charger is offline - cannot change live configuration",
				Data: map[string]interface{}{
					"online": false,
					"note":   "Use /configuration endpoint to change stored configuration.",
				},
			}
			helpers.SendJSONResponse(w, http.StatusServiceUnavailable, response)
			return
		}

		var requestBody struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}

		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			response := APIResponse{
				Success: false,
				Message: "Invalid request body",
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			return
		}

		// Send ChangeConfiguration request to the live charger
		request := core.NewChangeConfigurationRequest(requestBody.Key, requestBody.Value)
		err := ocppServer.SendRequest(clientID, request)
		if err != nil {
			log.Printf("Error sending ChangeConfiguration to charger %s: %v", clientID, err)
			response := APIResponse{
				Success: false,
				Message: "Failed to send configuration change to charger",
				Data: map[string]interface{}{
					"error":  err.Error(),
					"online": true,
				},
			}
			helpers.SendJSONResponse(w, http.StatusServiceUnavailable, response)
			return
		}

		// Note: The actual response will be handled by the OCPP response handler
		response := APIResponse{
			Success: true,
			Message: "ChangeConfiguration request sent to charger",
			Data: map[string]interface{}{
				"clientID": clientID,
				"key":      requestBody.Key,
				"value":    requestBody.Value,
				"online":   true,
				"note":     "Request sent to charger. Response will be processed asynchronously. Check server logs for the charger's response.",
			},
		}
		helpers.SendJSONResponse(w, http.StatusAccepted, response)
	}
}

// SendGetConfigurationToCharger sends a GetConfiguration request to a live charger
func SendGetConfigurationToCharger(
	ocppServer *ocppj.Server,
	correlationManager *correlation.Manager,
	clientID string,
	keys []string,
) (chan types.LiveConfigResponse, error) {
	request := core.NewGetConfigurationRequest(keys)
	log.Printf("SEND_REQUEST: Sending GetConfiguration to %s with keys: %v", clientID, keys)

	// Use a temporary correlation key for now - we'll update it after sending
	tempKey := fmt.Sprintf("%s:GetConfiguration:temp", clientID)
	responseChan := correlationManager.AddPendingRequest(tempKey, clientID, "GetConfiguration")

	err := ocppServer.SendRequest(clientID, request)
	if err != nil {
		log.Printf("SEND_REQUEST: Error sending to %s: %v", clientID, err)
		// Clean up pending request on error
		correlationManager.CleanupPendingRequest(tempKey)
		return nil, err
	}

	log.Printf("SEND_REQUEST: Successfully sent GetConfiguration to %s", clientID)
	return responseChan, nil
}