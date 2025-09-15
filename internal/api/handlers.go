package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/lorenzodonini/ocpp-go/transport"

	"ocpp-server/internal/helpers"
)

// HealthHandler handles health check requests
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	response := APIResponse{
		Success: true,
		Message: "OCPP Server is running",
		Data: map[string]interface{}{
			"timestamp": time.Now().Format(time.RFC3339),
		},
	}
	helpers.SendJSONResponse(w, http.StatusOK, response)
}

// GetClientsHandler handles requests to get connected clients
func GetClientsHandler(redisTransport transport.Transport) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clients := redisTransport.GetConnectedClients()
		response := APIResponse{
			Success: true,
			Message: "Connected clients retrieved",
			Data: map[string]interface{}{
				"clients": clients,
				"count":   len(clients),
			},
		}
		helpers.SendJSONResponse(w, http.StatusOK, response)
	}
}

// GetChargePointsHandler handles requests to get all charge points
func GetChargePointsHandler(businessState *ocppj.RedisBusinessState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chargePoints, err := businessState.GetAllChargePoints()
		if err != nil {
			response := APIResponse{
				Success: false,
				Message: "Failed to retrieve charge points",
			}
			helpers.SendJSONResponse(w, http.StatusInternalServerError, response)
			return
		}

		response := APIResponse{
			Success: true,
			Message: "Charge points retrieved",
			Data: map[string]interface{}{
				"chargePoints": chargePoints,
				"count":        len(chargePoints),
			},
		}
		helpers.SendJSONResponse(w, http.StatusOK, response)
	}
}

// GetChargePointHandler handles requests to get a specific charge point
func GetChargePointHandler(businessState *ocppj.RedisBusinessState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		clientID := vars["clientID"]

		chargePoint, err := businessState.GetChargePointInfo(clientID)
		if err != nil {
			response := APIResponse{
				Success: false,
				Message: "Failed to retrieve charge point",
			}
			helpers.SendJSONResponse(w, http.StatusInternalServerError, response)
			return
		}

		if chargePoint == nil {
			response := APIResponse{
				Success: false,
				Message: "Charge point not found",
			}
			helpers.SendJSONResponse(w, http.StatusNotFound, response)
			return
		}

		response := APIResponse{
			Success: true,
			Message: "Charge point retrieved",
			Data:    chargePoint,
		}
		helpers.SendJSONResponse(w, http.StatusOK, response)
	}
}

// GetConnectorsHandler handles requests to get connectors for a charge point
func GetConnectorsHandler(businessState *ocppj.RedisBusinessState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		clientID := vars["clientID"]

		connectors, err := businessState.GetAllConnectors(clientID)
		if err != nil {
			response := APIResponse{
				Success: false,
				Message: "Failed to retrieve connectors",
			}
			helpers.SendJSONResponse(w, http.StatusInternalServerError, response)
			return
		}

		response := APIResponse{
			Success: true,
			Message: "Connectors retrieved",
			Data: map[string]interface{}{
				"connectors": connectors,
				"count":      len(connectors),
			},
		}
		helpers.SendJSONResponse(w, http.StatusOK, response)
	}
}

// GetConnectorHandler handles requests to get a specific connector
func GetConnectorHandler(businessState *ocppj.RedisBusinessState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		clientID := vars["clientID"]
		connectorIDStr := vars["connectorID"]

		connectorID, err := strconv.Atoi(connectorIDStr)
		if err != nil {
			response := APIResponse{
				Success: false,
				Message: "Invalid connector ID",
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			return
		}

		connector, err := businessState.GetConnectorStatus(clientID, connectorID)
		if err != nil {
			response := APIResponse{
				Success: false,
				Message: "Failed to retrieve connector",
			}
			helpers.SendJSONResponse(w, http.StatusInternalServerError, response)
			return
		}

		if connector == nil {
			response := APIResponse{
				Success: false,
				Message: "Connector not found",
			}
			helpers.SendJSONResponse(w, http.StatusNotFound, response)
			return
		}

		response := APIResponse{
			Success: true,
			Message: "Connector retrieved",
			Data:    connector,
		}
		helpers.SendJSONResponse(w, http.StatusOK, response)
	}
}

// GetTransactionsHandler handles requests to get transactions
func GetTransactionsHandler(businessState *ocppj.RedisBusinessState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID := r.URL.Query().Get("clientId") // Optional filter

		transactions, err := businessState.GetActiveTransactions(clientID)
		if err != nil {
			response := APIResponse{
				Success: false,
				Message: "Failed to retrieve transactions",
			}
			helpers.SendJSONResponse(w, http.StatusInternalServerError, response)
			return
		}

		response := APIResponse{
			Success: true,
			Message: "Transactions retrieved",
			Data: map[string]interface{}{
				"transactions": transactions,
				"count":        len(transactions),
			},
		}
		helpers.SendJSONResponse(w, http.StatusOK, response)
	}
}

// GetTransactionHandler handles requests to get a specific transaction
func GetTransactionHandler(businessState *ocppj.RedisBusinessState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		transactionIDStr := vars["transactionID"]

		transactionID, err := strconv.Atoi(transactionIDStr)
		if err != nil {
			response := APIResponse{
				Success: false,
				Message: "Invalid transaction ID",
			}
			helpers.SendJSONResponse(w, http.StatusBadRequest, response)
			return
		}

		transaction, err := businessState.GetTransaction(transactionID)
		if err != nil {
			response := APIResponse{
				Success: false,
				Message: "Failed to retrieve transaction",
			}
			helpers.SendJSONResponse(w, http.StatusInternalServerError, response)
			return
		}

		if transaction == nil {
			response := APIResponse{
				Success: false,
				Message: "Transaction not found",
			}
			helpers.SendJSONResponse(w, http.StatusNotFound, response)
			return
		}

		response := APIResponse{
			Success: true,
			Message: "Transaction retrieved",
			Data:    transaction,
		}
		helpers.SendJSONResponse(w, http.StatusOK, response)
	}
}

// GetChargerStatusHandler handles requests to get charger online status
func GetChargerStatusHandler(redisTransport transport.Transport) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		clientID := vars["clientID"]

		isOnline := IsChargerOnline(redisTransport, clientID)

		response := APIResponse{
			Success: true,
			Message: "Charger status retrieved",
			Data: map[string]interface{}{
				"clientID": clientID,
				"online":   isOnline,
			},
		}
		helpers.SendJSONResponse(w, http.StatusOK, response)
	}
}

// IsChargerOnline checks if a charger is currently connected
func IsChargerOnline(redisTransport transport.Transport, clientID string) bool {
	connectedClients := redisTransport.GetConnectedClients()
	for _, client := range connectedClients {
		if client == clientID {
			return true
		}
	}
	return false
}