package server

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/lorenzodonini/ocpp-go/ocpp"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/remotetrigger"

	v1api "ocpp-server/internal/api/v1"
	ocpphandlers "ocpp-server/internal/ocpp"
	"ocpp-server/internal/services"
)

// setupOCPPHandlers configures all OCPP message handlers
func (s *Server) setupOCPPHandlers() {
	s.ocppServer.SetTransportRequestHandler(func(clientID string, request ocpp.Request, requestId string, action string) {
		log.Printf("REQUEST_HANDLER: Received request [%s] from client %s: %s, type: %T", requestId, clientID, action, request)

		// Publish OCPP message to MQTT if publisher is available and connected
		if s.mqttPublisher != nil && s.mqttPublisher.IsConnected() {
			s.mqttPublisher.PublishOCPPMessage(clientID, requestId, action, request)
		}

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

		// Publish OCPP response to MQTT if publisher is available and connected
		if s.mqttPublisher != nil && s.mqttPublisher.IsConnected() {
			// Extract message type from response type
			messageType := ""
			switch response.(type) {
			case *core.GetConfigurationConfirmation:
				messageType = "GetConfiguration"
			case *core.ChangeConfigurationConfirmation:
				messageType = "ChangeConfiguration"
			case *core.RemoteStartTransactionConfirmation:
				messageType = "RemoteStartTransaction"
			case *core.RemoteStopTransactionConfirmation:
				messageType = "RemoteStopTransaction"
			case *remotetrigger.TriggerMessageConfirmation:
				messageType = "TriggerMessage"
			default:
				messageType = "Unknown"
			}
			s.mqttPublisher.PublishOCPPResponse(clientID, requestId, messageType, response)
		}

		switch res := response.(type) {
		case *core.GetConfigurationConfirmation:
			log.Printf("RESPONSE_HANDLER: Processing GetConfigurationConfirmation")
			ocpphandlers.HandleGetConfigurationResponse(s.correlationManager, clientID, requestId, res)

		case *core.ChangeConfigurationConfirmation:
			log.Printf("RESPONSE_HANDLER: Processing ChangeConfigurationConfirmation")
			ocpphandlers.HandleChangeConfigurationResponse(s.correlationManager, clientID, requestId, res)

		case *core.RemoteStartTransactionConfirmation:
			log.Printf("RESPONSE_HANDLER: Processing RemoteStartTransactionConfirmation")
			ocpphandlers.HandleRemoteStartTransactionResponse(s.correlationManager, clientID, requestId, res)

		case *core.RemoteStopTransactionConfirmation:
			log.Printf("RESPONSE_HANDLER: Processing RemoteStopTransactionConfirmation")
			ocpphandlers.HandleRemoteStopTransactionResponse(s.correlationManager, clientID, requestId, res)

		case *remotetrigger.TriggerMessageConfirmation:
			log.Printf("RESPONSE_HANDLER: Processing TriggerMessageConfirmation")
			ocpphandlers.HandleTriggerMessageResponse(s.correlationManager, clientID, requestId, res)

		default:
			log.Printf("RESPONSE_HANDLER: Unknown response type: %T from client %s", res, clientID)
		}
	})

	// Add error handler for CALLERROR messages
	s.ocppServer.SetTransportErrorHandler(func(clientID string, err *ocpp.Error, details interface{}) {
		log.Printf("ERROR_HANDLER: Received error from client %s: %s", clientID, err.Error())

		// Handle different request types using existing correlation logic
		// We need to determine which type of request this error is responding to
		// Check for pending requests of each type and handle the first match found

		// Try TriggerMessage first (most common)
		if foundKey, _ := s.correlationManager.FindPendingRequest(clientID, "TriggerMessage"); foundKey != "" {
			ocpphandlers.HandleTriggerMessageError(s.correlationManager, clientID, err)
			return
		}

		// Try GetConfiguration
		if foundKey, _ := s.correlationManager.FindPendingRequest(clientID, "GetConfiguration"); foundKey != "" {
			ocpphandlers.HandleGetConfigurationError(s.correlationManager, clientID, err)
			return
		}

		// Try ChangeConfiguration
		if foundKey, _ := s.correlationManager.FindPendingRequest(clientID, "ChangeConfiguration"); foundKey != "" {
			ocpphandlers.HandleChangeConfigurationError(s.correlationManager, clientID, err)
			return
		}

		// Try RemoteStartTransaction
		if foundKey, _ := s.correlationManager.FindPendingRequest(clientID, "RemoteStartTransaction"); foundKey != "" {
			ocpphandlers.HandleRemoteStartTransactionError(s.correlationManager, clientID, err)
			return
		}

		// Try RemoteStopTransaction
		if foundKey, _ := s.correlationManager.FindPendingRequest(clientID, "RemoteStopTransaction"); foundKey != "" {
			ocpphandlers.HandleRemoteStopTransactionError(s.correlationManager, clientID, err)
			return
		}

		log.Printf("ERROR_HANDLER: No pending request found for client %s error: %s", clientID, err.Error())
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

	// Create services
	chargePointService := services.NewChargePointService(s.businessState, s.redisTransport)
	transactionService := services.NewTransactionService(s.businessState)
	configService := services.NewConfigurationService(
		s.configManager,
		s.redisTransport,
		s.ocppServer,
		s.correlationManager,
	)
	remoteTransactionService := services.NewRemoteTransactionService(
		s.ocppServer,
		chargePointService,
		s.correlationManager,
	)
	triggerMessageService := services.NewTriggerMessageService(
		s.ocppServer,
		chargePointService,
		s.correlationManager,
	)

	// Register V1 API routes
	v1api.RegisterRoutes(
		router,
		chargePointService,
		transactionService,
		configService,
		remoteTransactionService,
		triggerMessageService,
	)

	s.httpServer = &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}
}