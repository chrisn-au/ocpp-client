package server

import (
	"context"
	"log"
	"net/http"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/remotetrigger"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/lorenzodonini/ocpp-go/transport"

	cfgmgr "ocpp-server/config"
	"ocpp-server/internal/handlers"
	"ocpp-server/internal/correlation"
	"ocpp-server/internal/mqtt"
	"ocpp-server/internal/types"
)

// Config holds the server configuration
type Config struct {
	RedisAddr                 string
	RedisPassword             string
	HTTPPort                  string
	MQTTEnabled               bool
	MQTTHost                  string
	MQTTPort                  int
	MQTTUsername              string
	MQTTPassword              string
	MQTTClientID              string
	MQTTBusinessEventsEnabled bool // Enable business-level MQTT events
}

// Server represents the OCPP server with all its components
type Server struct {
	ocppServer               *ocppj.Server
	redisTransport           transport.Transport
	businessState            *ocppj.RedisBusinessState
	httpServer               *http.Server
	transactionCounter       int
	configManager            *cfgmgr.ConfigurationManager
	meterValueProcessor *handlers.MeterValueProcessor
	transactionHandler  *handlers.TransactionHandler
	correlationManager       *correlation.Manager
	mqttPublisher            *mqtt.Publisher
}

// NewServer creates a new server instance
func NewServer(config Config, redisTransport transport.Transport, businessState *ocppj.RedisBusinessState, serverState ocppj.ServerState) (*Server, error) {
	server := &Server{
		redisTransport:     redisTransport,
		businessState:      businessState,
		transactionCounter: 1000, // Start transaction IDs from 1000
		correlationManager: correlation.NewManager(),
	}

	// Create configuration manager
	server.configManager = cfgmgr.NewConfigurationManager(businessState)

	// Create meter value processor
	server.meterValueProcessor = handlers.NewMeterValueProcessor(businessState, server.configManager)

	// Create OCPP server with distributed state
	server.ocppServer = ocppj.NewServerWithTransport(redisTransport, nil, serverState, core.Profile, remotetrigger.Profile)


	// Create MQTT publisher if enabled
	if config.MQTTEnabled {
		mqttConfig := mqtt.PublisherConfig{
			BrokerHost:            config.MQTTHost,
			BrokerPort:            config.MQTTPort,
			Username:              config.MQTTUsername,
			Password:              config.MQTTPassword,
			ClientID:              config.MQTTClientID,
			QoS:                   0, // At most once delivery
			Retained:              false,
			BusinessEventsEnabled: config.MQTTBusinessEventsEnabled,
		}

		var err error
		server.mqttPublisher, err = mqtt.NewPublisher(mqttConfig)
		if err != nil {
			log.Printf("Failed to create MQTT publisher: %v", err)
			return nil, err
		}
	}

	// Create transaction handler with MQTT publisher if available
	if server.mqttPublisher != nil {
		server.transactionHandler = handlers.NewTransactionHandlerWithMQTT(businessState, server.meterValueProcessor, server.mqttPublisher)
	} else {
		server.transactionHandler = handlers.NewTransactionHandler(businessState, server.meterValueProcessor)
	}

	return server, nil
}

// Start starts the OCPP and HTTP servers
func (s *Server) Start(ctx context.Context, redisConfig *transport.RedisConfig, httpPort string) error {
	// Setup handlers
	s.setupOCPPHandlers()
	s.setupHTTPAPI(httpPort)

	// Connect to MQTT broker if enabled
	if s.mqttPublisher != nil {
		if err := s.mqttPublisher.Connect(); err != nil {
			log.Printf("Failed to connect to MQTT broker: %v", err)
			// Don't fail the entire server startup if MQTT connection fails
		} else {
			log.Println("MQTT publisher connected successfully")
		}
	}

	// Start OCPP server
	go func() {
		if err := s.ocppServer.StartWithTransport(ctx, redisConfig); err != nil {
			log.Fatalf("OCPP server failed to start: %v", err)
		}
	}()

	// Start HTTP server
	go func() {
		log.Printf("HTTP API server listening on port %s", httpPort)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed to start: %v", err)
		}
	}()

	log.Println("Server started and listening for Redis messages and HTTP requests")
	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Printf("Error stopping HTTP server: %v", err)
	}

	if err := s.ocppServer.StopWithTransport(ctx); err != nil {
		log.Printf("Error stopping OCPP server: %v", err)
	}

	// Disconnect MQTT publisher
	if s.mqttPublisher != nil {
		s.mqttPublisher.Disconnect()
	}

	return nil
}

// IsChargerOnline checks if a charger is currently connected
func (s *Server) IsChargerOnline(clientID string) bool {
	connectedClients := s.redisTransport.GetConnectedClients()
	for _, client := range connectedClients {
		if client == clientID {
			return true
		}
	}
	return false
}

// GetTransactionCounter returns the current transaction counter value
func (s *Server) GetTransactionCounter() int {
	return s.transactionCounter
}

// IncrementTransactionCounter increments and returns the new transaction counter value
func (s *Server) IncrementTransactionCounter() int {
	s.transactionCounter++
	return s.transactionCounter
}

// GetOCPPServer returns the OCPP server instance
func (s *Server) GetOCPPServer() *ocppj.Server {
	return s.ocppServer
}

// GetBusinessState returns the business state
func (s *Server) GetBusinessState() *ocppj.RedisBusinessState {
	return s.businessState
}

// GetConfigManager returns the configuration manager
func (s *Server) GetConfigManager() *cfgmgr.ConfigurationManager {
	return s.configManager
}

// GetTransactionHandler returns the transaction handler
func (s *Server) GetTransactionHandler() *handlers.TransactionHandler {
	return s.transactionHandler
}


// GetCorrelationManager returns the correlation manager
func (s *Server) GetCorrelationManager() *correlation.Manager {
	return s.correlationManager
}

// GetMQTTPublisher returns the MQTT publisher
func (s *Server) GetMQTTPublisher() *mqtt.Publisher {
	return s.mqttPublisher
}

// PendingRequestManager interface implementation for handlers package
func (s *Server) AddPendingRequest(requestID, clientID, requestType string) chan types.LiveConfigResponse {
	return s.correlationManager.AddPendingRequestForHandlers(requestID, clientID, requestType)
}

func (s *Server) CleanupPendingRequest(requestID string) {
	s.correlationManager.CleanupPendingRequest(requestID)
}

func (s *Server) SendPendingResponse(clientID, requestType string, response types.LiveConfigResponse) {
	s.correlationManager.SendPendingResponseFromHandlers(clientID, requestType, response)
}