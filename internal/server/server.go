package server

import (
	"context"
	"log"
	"net/http"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/lorenzodonini/ocpp-go/transport"

	cfgmgr "ocpp-server/config"
	"ocpp-server/handlers"
	"ocpp-server/internal/correlation"
)

// Config holds the server configuration
type Config struct {
	RedisAddr     string
	RedisPassword string
	HTTPPort      string
}

// Server represents the OCPP server with all its components
type Server struct {
	ocppServer               *ocppj.Server
	redisTransport           transport.Transport
	businessState            *ocppj.RedisBusinessState
	httpServer               *http.Server
	transactionCounter       int
	configManager            *cfgmgr.ConfigurationManager
	meterValueProcessor      *handlers.MeterValueProcessor
	remoteTransactionHandler *handlers.RemoteTransactionHandler
	transactionHandler       *handlers.TransactionHandler
	correlationManager       *correlation.Manager
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
	server.ocppServer = ocppj.NewServerWithTransport(redisTransport, nil, serverState, core.Profile)

	// Create remote transaction handler
	server.remoteTransactionHandler = handlers.NewRemoteTransactionHandler(
		server.ocppServer,
		server.redisTransport,
		server.businessState,
		server, // Pass server as PendingRequestManager
	)

	// Create transaction handler
	server.transactionHandler = handlers.NewTransactionHandler(businessState, server.meterValueProcessor)

	return server, nil
}

// Start starts the OCPP and HTTP servers
func (s *Server) Start(ctx context.Context, redisConfig *transport.RedisConfig, httpPort string) error {
	// Setup handlers
	s.setupOCPPHandlers()
	s.setupHTTPAPI(httpPort)

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

// GetRemoteTransactionHandler returns the remote transaction handler
func (s *Server) GetRemoteTransactionHandler() *handlers.RemoteTransactionHandler {
	return s.remoteTransactionHandler
}

// GetCorrelationManager returns the correlation manager
func (s *Server) GetCorrelationManager() *correlation.Manager {
	return s.correlationManager
}

// PendingRequestManager interface implementation for handlers package
func (s *Server) AddPendingRequest(requestID, clientID, requestType string) chan handlers.LiveConfigResponse {
	return s.correlationManager.AddPendingRequestForHandlers(requestID, clientID, requestType)
}

func (s *Server) CleanupPendingRequest(requestID string) {
	s.correlationManager.CleanupPendingRequest(requestID)
}

func (s *Server) SendPendingResponse(clientID, requestType string, response handlers.LiveConfigResponse) {
	s.correlationManager.SendPendingResponseFromHandlers(clientID, requestType, response)
}