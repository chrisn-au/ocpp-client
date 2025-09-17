package handlers

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"

	"ocpp-server/internal/api/v1/models"
	"ocpp-server/internal/helpers"
	"ocpp-server/internal/services"
)

// ChargePointsHandler handles charge point related requests
type ChargePointsHandler struct {
	chargePointService *services.ChargePointService
}

// NewChargePointsHandler creates a new charge points handler
func NewChargePointsHandler(chargePointService *services.ChargePointService) *ChargePointsHandler {
	return &ChargePointsHandler{
		chargePointService: chargePointService,
	}
}

// GetChargePoints handles requests to get all charge points
func (h *ChargePointsHandler) GetChargePoints(w http.ResponseWriter, r *http.Request) {
	chargePoints, err := h.chargePointService.GetAllChargePoints()
	if err != nil {
		response := models.APIResponse{
			Success: false,
			Message: "Failed to retrieve charge points",
		}
		helpers.SendJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	responseData := models.ChargePointsResponse{
		ChargePoints: chargePoints,
		Count:        len(chargePoints),
	}

	response := models.APIResponse{
		Success: true,
		Message: "Charge points retrieved",
		Data:    responseData,
	}
	helpers.SendJSONResponse(w, http.StatusOK, response)
}

// GetChargePoint handles requests to get a specific charge point
func (h *ChargePointsHandler) GetChargePoint(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientID"]

	chargePoint, err := h.chargePointService.GetChargePoint(clientID)
	if err != nil {
		response := models.APIResponse{
			Success: false,
			Message: "Failed to retrieve charge point",
		}
		helpers.SendJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	if chargePoint == nil {
		response := models.APIResponse{
			Success: false,
			Message: "Charge point not found",
		}
		helpers.SendJSONResponse(w, http.StatusNotFound, response)
		return
	}

	response := models.APIResponse{
		Success: true,
		Message: "Charge point retrieved",
		Data:    chargePoint,
	}
	helpers.SendJSONResponse(w, http.StatusOK, response)
}

// GetConnectors handles requests to get connectors for a charge point
func (h *ChargePointsHandler) GetConnectors(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientID"]

	connectors, err := h.chargePointService.GetAllConnectors(clientID)
	if err != nil {
		response := models.APIResponse{
			Success: false,
			Message: "Failed to retrieve connectors",
		}
		helpers.SendJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	responseData := models.ConnectorsResponse{
		Connectors: connectors,
		Count:      len(connectors),
	}

	response := models.APIResponse{
		Success: true,
		Message: "Connectors retrieved",
		Data:    responseData,
	}
	helpers.SendJSONResponse(w, http.StatusOK, response)
}

// GetConnector handles requests to get a specific connector
func (h *ChargePointsHandler) GetConnector(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientID"]
	connectorIDStr := vars["connectorID"]

	connectorID, err := strconv.Atoi(connectorIDStr)
	if err != nil {
		response := models.APIResponse{
			Success: false,
			Message: "Invalid connector ID",
		}
		helpers.SendJSONResponse(w, http.StatusBadRequest, response)
		return
	}

	connector, err := h.chargePointService.GetConnector(clientID, connectorID)
	if err != nil {
		response := models.APIResponse{
			Success: false,
			Message: "Failed to retrieve connector",
		}
		helpers.SendJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	if connector == nil {
		response := models.APIResponse{
			Success: false,
			Message: "Connector not found",
		}
		helpers.SendJSONResponse(w, http.StatusNotFound, response)
		return
	}

	response := models.APIResponse{
		Success: true,
		Message: "Connector retrieved",
		Data:    connector,
	}
	helpers.SendJSONResponse(w, http.StatusOK, response)
}

// GetChargePointStatus handles requests to get charge point online status
func (h *ChargePointsHandler) GetChargePointStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientID"]

	isOnline := h.chargePointService.IsOnline(clientID)

	responseData := models.ChargePointStatusResponse{
		ClientID: clientID,
		Online:   isOnline,
	}

	response := models.APIResponse{
		Success: true,
		Message: "Charger status retrieved",
		Data:    responseData,
	}
	helpers.SendJSONResponse(w, http.StatusOK, response)
}