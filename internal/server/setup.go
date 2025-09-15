package server

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/lorenzodonini/ocpp-go/ocpp"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"

	"ocpp-server/internal/api"
	ocpphandlers "ocpp-server/internal/ocpp"
)

// setupOCPPHandlers configures all OCPP message handlers
func (s *Server) setupOCPPHandlers() {
	s.ocppServer.SetTransportRequestHandler(func(clientID string, request ocpp.Request, requestId string, action string) {
		log.Printf("REQUEST_HANDLER: Received request [%s] from client %s: %s, type: %T", requestId, clientID, action, request)

		switch req := request.(type) {
		case *core.BootNotificationRequest:
			ocpphandlers.HandleBootNotification(s.ocppServer, s.businessState, clientID, requestId, req)

		case *core.HeartbeatRequest:
			ocpphandlers.HandleHeartbeat(s.ocppServer, s.businessState, clientID, requestId, req)

		case *core.StatusNotificationRequest:
			s.transactionHandler.HandleStatusNotification(clientID, requestId, req, func(response *core.StatusNotificationConfirmation) {
				if err := s.ocppServer.SendResponse(clientID, requestId, response); err != nil {
					log.Printf("Error sending StatusNotification response: %v", err)
				}
			})

		case *core.StartTransactionRequest:
			s.transactionHandler.HandleStartTransaction(clientID, requestId, req, func(response *core.StartTransactionConfirmation) {
				if err := s.ocppServer.SendResponse(clientID, requestId, response); err != nil {
					log.Printf("Error sending StartTransaction response: %v", err)
				}
			})

		case *core.StopTransactionRequest:
			s.transactionHandler.HandleStopTransaction(clientID, requestId, req, func(response *core.StopTransactionConfirmation) {
				if err := s.ocppServer.SendResponse(clientID, requestId, response); err != nil {
					log.Printf("Error sending StopTransaction response: %v", err)
				}
			})

		case *core.GetConfigurationRequest:
			ocpphandlers.HandleGetConfiguration(s.ocppServer, s.configManager, clientID, requestId, req)

		case *core.ChangeConfigurationRequest:
			ocpphandlers.HandleChangeConfiguration(s.ocppServer, s.configManager, clientID, requestId, req)

		case *core.MeterValuesRequest:
			s.transactionHandler.HandleMeterValues(clientID, requestId, req, func(response *core.MeterValuesConfirmation) {
				if err := s.ocppServer.SendResponse(clientID, requestId, response); err != nil {
					log.Printf("Error sending MeterValues response: %v", err)
				}
			})

		default:
			log.Printf("Unsupported request type: %T from client %s", req, clientID)
			if err := s.ocppServer.SendError(clientID, requestId, "NotSupported", "Request not supported", nil); err != nil {
				log.Printf("Error sending error response: %v", err)
			}
		}
	})

	// Add response handler for outgoing requests
	s.ocppServer.SetTransportResponseHandler(func(clientID string, response ocpp.Response, requestId string) {
		log.Printf("RESPONSE_HANDLER: Received response [%s] from client %s, type: %T", requestId, clientID, response)

		switch res := response.(type) {
		case *core.GetConfigurationConfirmation:
			log.Printf("RESPONSE_HANDLER: Processing GetConfigurationConfirmation")
			ocpphandlers.HandleGetConfigurationResponse(s.correlationManager, clientID, requestId, res)

		case *core.ChangeConfigurationConfirmation:
			log.Printf("RESPONSE_HANDLER: Processing ChangeConfigurationConfirmation")
			ocpphandlers.HandleChangeConfigurationResponse(s.correlationManager, clientID, requestId, res)

		case *core.RemoteStartTransactionConfirmation:
			log.Printf("RESPONSE_HANDLER: Processing RemoteStartTransactionConfirmation")
			s.remoteTransactionHandler.HandleRemoteStartTransactionResponse(clientID, requestId, res)

		case *core.RemoteStopTransactionConfirmation:
			log.Printf("RESPONSE_HANDLER: Processing RemoteStopTransactionConfirmation")
			s.remoteTransactionHandler.HandleRemoteStopTransactionResponse(clientID, requestId, res)

		default:
			log.Printf("RESPONSE_HANDLER: Unknown response type: %T from client %s", res, clientID)
		}
	})

	s.ocppServer.SetTransportNewClientHandler(func(clientID string) {
		log.Printf("New client connected: %s", clientID)

		// Update business state - client is online
		if err := s.businessState.UpdateChargePointLastSeen(clientID); err != nil {
			log.Printf("Error updating charge point state for %s: %v", clientID, err)
		}
	})

	s.ocppServer.SetTransportDisconnectedClientHandler(func(clientID string) {
		log.Printf("Client disconnected: %s", clientID)

		// Update business state - client is offline
		if err := s.businessState.SetChargePointOffline(clientID); err != nil {
			log.Printf("Error setting charge point offline for %s: %v", clientID, err)
		}
	})
}

// setupHTTPAPI configures all HTTP API endpoints
func (s *Server) setupHTTPAPI(port string) {
	router := mux.NewRouter()

	// Health and legacy endpoints
	router.HandleFunc("/health", api.HealthHandler).Methods("GET")
	router.HandleFunc("/clients", api.GetClientsHandler(s.redisTransport)).Methods("GET")

	// Remote transaction control endpoints (new API)
	s.remoteTransactionHandler.RegisterRoutes(router)

	// Legacy remote transaction endpoints
	router.HandleFunc("/clients/{clientID}/remote-start",
		api.RemoteStartHandler(s.redisTransport, s.ocppServer, s.correlationManager)).Methods("POST")
	router.HandleFunc("/clients/{clientID}/remote-stop",
		api.RemoteStopHandler(s.redisTransport, s.ocppServer, s.correlationManager)).Methods("POST")

	// New business state endpoints
	router.HandleFunc("/chargepoints", api.GetChargePointsHandler(s.businessState)).Methods("GET")
	router.HandleFunc("/chargepoints/{clientID}", api.GetChargePointHandler(s.businessState)).Methods("GET")
	router.HandleFunc("/chargepoints/{clientID}/connectors", api.GetConnectorsHandler(s.businessState)).Methods("GET")
	router.HandleFunc("/chargepoints/{clientID}/connectors/{connectorID}", api.GetConnectorHandler(s.businessState)).Methods("GET")
	router.HandleFunc("/transactions", api.GetTransactionsHandler(s.businessState)).Methods("GET")
	router.HandleFunc("/transactions/{transactionID}", api.GetTransactionHandler(s.businessState)).Methods("GET")

	// Configuration endpoints
	router.HandleFunc("/api/v1/chargepoints/{clientID}/configuration",
		api.GetConfigurationHandler(s.configManager)).Methods("GET")
	router.HandleFunc("/api/v1/chargepoints/{clientID}/configuration",
		api.ChangeConfigurationHandler(s.configManager)).Methods("PUT")
	router.HandleFunc("/api/v1/chargepoints/{clientID}/configuration/export",
		api.ExportConfigurationHandler(s.configManager)).Methods("GET")

	// Live configuration endpoints
	router.HandleFunc("/api/v1/chargepoints/{clientID}/configuration/live",
		api.GetLiveConfigurationHandler(s.redisTransport, s.ocppServer, s.correlationManager)).Methods("GET")
	router.HandleFunc("/api/v1/chargepoints/{clientID}/configuration/live",
		api.ChangeLiveConfigurationHandler(s.redisTransport, s.ocppServer)).Methods("PUT")
	router.HandleFunc("/api/v1/chargepoints/{clientID}/status",
		api.GetChargerStatusHandler(s.redisTransport)).Methods("GET")

	s.httpServer = &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}
}