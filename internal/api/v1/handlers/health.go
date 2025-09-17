package handlers

import (
	"net/http"
	"time"

	"ocpp-server/internal/api/v1/models"
	"ocpp-server/internal/helpers"
	"ocpp-server/internal/services"
)

// HealthHandler handles health check requests
type HealthHandler struct{}

// NewHealthHandler creates a new health handler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health handles health check requests
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	response := models.APIResponse{
		Success: true,
		Message: "OCPP Server is running",
		Data: map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	}
	helpers.SendJSONResponse(w, http.StatusOK, response)
}

// ConnectedClientsHandler handles connected clients requests
type ConnectedClientsHandler struct {
	chargePointService *services.ChargePointService
}

// NewConnectedClientsHandler creates a new connected clients handler
func NewConnectedClientsHandler(chargePointService *services.ChargePointService) *ConnectedClientsHandler {
	return &ConnectedClientsHandler{
		chargePointService: chargePointService,
	}
}

// GetConnectedClients handles requests to get connected clients
func (h *ConnectedClientsHandler) GetConnectedClients(w http.ResponseWriter, r *http.Request) {
	clients := h.chargePointService.GetConnectedClients()

	responseData := models.ConnectedClientsResponse{
		Clients: clients,
		Count:   len(clients),
	}

	response := models.APIResponse{
		Success: true,
		Message: "Connected clients retrieved",
		Data:    responseData,
	}
	helpers.SendJSONResponse(w, http.StatusOK, response)
}