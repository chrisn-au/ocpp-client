package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"ocpp-server/internal/api/v1/models"
	"ocpp-server/internal/helpers"
	"ocpp-server/internal/services"
)

const (
	liveConfigTimeout = 10 * time.Second
)

// ConfigurationHandler handles configuration related requests
type ConfigurationHandler struct {
	configService *services.ConfigurationService
}

// NewConfigurationHandler creates a new configuration handler
func NewConfigurationHandler(configService *services.ConfigurationService) *ConfigurationHandler {
	return &ConfigurationHandler{
		configService: configService,
	}
}

// GetStoredConfiguration handles requests to get stored configuration
func (h *ConfigurationHandler) GetStoredConfiguration(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientID"]

	// Parse query parameters for specific keys
	keysParam := r.URL.Query().Get("keys")

	configData, unknownKeys := h.configService.GetStoredConfiguration(clientID, nil)
	if keysParam != "" {
		// If specific keys requested, filter the response
		// This could be optimized by passing keys to the service
		configData, unknownKeys = h.configService.GetStoredConfiguration(clientID, []string{keysParam})
	}

	responseData := models.ConfigurationResponse{
		Configuration: configData,
	}

	if len(unknownKeys) > 0 {
		responseData.UnknownKeys = unknownKeys
	}

	response := models.APIResponse{
		Success: true,
		Message: "Configuration retrieved",
		Data:    responseData,
	}
	helpers.SendJSONResponse(w, http.StatusOK, response)
}

// ChangeStoredConfiguration handles requests to change stored configuration
func (h *ConfigurationHandler) ChangeStoredConfiguration(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientID"]

	var req models.ConfigurationChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := models.APIResponse{
			Success: false,
			Message: "Invalid request body",
		}
		helpers.SendJSONResponse(w, http.StatusBadRequest, response)
		return
	}

	if req.Key == "" || req.Value == "" {
		response := models.APIResponse{
			Success: false,
			Message: "Key and value are required",
		}
		helpers.SendJSONResponse(w, http.StatusBadRequest, response)
		return
	}

	status := h.configService.ChangeStoredConfiguration(clientID, req.Key, req.Value)

	responseData := models.ConfigurationChangeResponse{
		Status: status,
	}

	response := models.APIResponse{
		Success: true,
		Message: "Configuration change processed",
		Data:    responseData,
	}
	helpers.SendJSONResponse(w, http.StatusOK, response)
}

// ExportConfiguration handles requests to export configuration
func (h *ConfigurationHandler) ExportConfiguration(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientID"]

	config := h.configService.ExportConfiguration(clientID)

	response := models.APIResponse{
		Success: true,
		Message: "Configuration exported",
		Data:    config,
	}
	helpers.SendJSONResponse(w, http.StatusOK, response)
}

// GetLiveConfiguration handles requests to get live configuration from charge point
func (h *ConfigurationHandler) GetLiveConfiguration(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientID"]

	// Check if charger is online
	if !h.configService.IsChargerOnline(clientID) {
		errorData := models.ErrorData{
			Online: &[]bool{false}[0],
			Note:   "Falling back to stored configuration. Use /configuration endpoint for stored values.",
		}

		response := models.APIResponse{
			Success: false,
			Message: "Charger is offline - returning stored configuration",
			Data:    errorData,
		}
		helpers.SendJSONResponse(w, http.StatusServiceUnavailable, response)
		return
	}

	// Parse query parameters for specific keys
	keysParam := r.URL.Query().Get("keys")

	// Send GetConfiguration request to the live charger and wait for response
	responseChan, err := h.configService.GetLiveConfiguration(clientID, keysParam)
	if err != nil {
		log.Printf("Error sending GetConfiguration to charger %s: %v", clientID, err)

		errorData := models.ErrorData{
			Error:  err.Error(),
			Online: &[]bool{true}[0],
		}

		response := models.APIResponse{
			Success: false,
			Message: "Failed to send request to charger",
			Data:    errorData,
		}
		helpers.SendJSONResponse(w, http.StatusServiceUnavailable, response)
		return
	}

	// Wait for response with timeout
	select {
	case liveResponse := <-responseChan:
		if liveResponse.Success {
			response := models.APIResponse{
				Success: true,
				Message: "Live configuration retrieved from charger",
				Data:    liveResponse.Data,
			}
			helpers.SendJSONResponse(w, http.StatusOK, response)
		} else {
			errorData := models.ErrorData{
				Error:  liveResponse.Error,
				Online: &[]bool{true}[0],
			}

			response := models.APIResponse{
				Success: false,
				Message: "Charger rejected configuration request",
				Data:    errorData,
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
		}

	case <-time.After(liveConfigTimeout):
		log.Printf("Timeout waiting for GetConfiguration response from %s", clientID)

		errorData := models.ErrorData{
			Online:  &[]bool{true}[0],
			Timeout: fmt.Sprintf("%.0fs", liveConfigTimeout.Seconds()),
			Note:    "Charger did not respond within timeout. Use /configuration endpoint for stored values.",
		}

		response := models.APIResponse{
			Success: false,
			Message: "Timeout waiting for charger response",
			Data:    errorData,
		}
		helpers.SendJSONResponse(w, http.StatusRequestTimeout, response)
	}
}

// ChangeLiveConfiguration handles requests to change live configuration on charge point
func (h *ConfigurationHandler) ChangeLiveConfiguration(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientID"]

	// Check if charger is online
	if !h.configService.IsChargerOnline(clientID) {
		errorData := models.ErrorData{
			Online: &[]bool{false}[0],
			Note:   "Use /configuration endpoint to change stored configuration.",
		}

		response := models.APIResponse{
			Success: false,
			Message: "Charger is offline - cannot change live configuration",
			Data:    errorData,
		}
		helpers.SendJSONResponse(w, http.StatusServiceUnavailable, response)
		return
	}

	var req models.ConfigurationChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response := models.APIResponse{
			Success: false,
			Message: "Invalid request body",
		}
		helpers.SendJSONResponse(w, http.StatusBadRequest, response)
		return
	}

	if req.Key == "" || req.Value == "" {
		response := models.APIResponse{
			Success: false,
			Message: "Key and value are required",
		}
		helpers.SendJSONResponse(w, http.StatusBadRequest, response)
		return
	}

	// Send ChangeConfiguration request to the live charger
	err := h.configService.ChangeLiveConfiguration(clientID, req.Key, req.Value)
	if err != nil {
		log.Printf("Error sending ChangeConfiguration to charger %s: %v", clientID, err)

		errorData := models.ErrorData{
			Error:  err.Error(),
			Online: &[]bool{true}[0],
		}

		response := models.APIResponse{
			Success: false,
			Message: "Failed to send configuration change to charger",
			Data:    errorData,
		}
		helpers.SendJSONResponse(w, http.StatusServiceUnavailable, response)
		return
	}

	// Note: The actual response will be handled by the OCPP response handler
	responseData := models.LiveConfigurationChangeResponse{
		ClientID: clientID,
		Key:      req.Key,
		Value:    req.Value,
		Online:   true,
		Note:     "Request sent to charger. Response will be processed asynchronously. Check server logs for the charger's response.",
	}

	response := models.APIResponse{
		Success: true,
		Message: "ChangeConfiguration request sent to charger",
		Data:    responseData,
	}
	helpers.SendJSONResponse(w, http.StatusAccepted, response)
}