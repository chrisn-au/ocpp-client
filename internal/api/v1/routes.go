package v1

import (
	"github.com/gorilla/mux"

	"ocpp-server/internal/api/v1/handlers"
	"ocpp-server/internal/services"
)

// RegisterRoutes registers all v1 API routes
func RegisterRoutes(
	router *mux.Router,
	chargePointService *services.ChargePointService,
	transactionService *services.TransactionService,
	configService *services.ConfigurationService,
	remoteTransactionService *services.RemoteTransactionService,
) {
	// Create handlers
	healthHandler := handlers.NewHealthHandler()
	connectedClientsHandler := handlers.NewConnectedClientsHandler(chargePointService)
	chargePointsHandler := handlers.NewChargePointsHandler(chargePointService)
	transactionsHandler := handlers.NewTransactionsHandler(
		transactionService,
		chargePointService,
		remoteTransactionService,
	)
	configurationHandler := handlers.NewConfigurationHandler(configService)

	// Health and system endpoints
	router.HandleFunc("/health", healthHandler.Health).Methods("GET")
	router.HandleFunc("/clients", connectedClientsHandler.GetConnectedClients).Methods("GET")


	// V1 API endpoints
	v1Router := router.PathPrefix("/api/v1").Subrouter()

	// Charge point management
	v1Router.HandleFunc("/chargepoints", chargePointsHandler.GetChargePoints).Methods("GET")
	v1Router.HandleFunc("/chargepoints/{clientID}", chargePointsHandler.GetChargePoint).Methods("GET")
	v1Router.HandleFunc("/chargepoints/{clientID}/connectors", chargePointsHandler.GetConnectors).Methods("GET")
	v1Router.HandleFunc("/chargepoints/{clientID}/connectors/{connectorID}", chargePointsHandler.GetConnector).Methods("GET")
	v1Router.HandleFunc("/chargepoints/{clientID}/status", chargePointsHandler.GetChargePointStatus).Methods("GET")

	// Transaction management
	v1Router.HandleFunc("/transactions", transactionsHandler.GetTransactions).Methods("GET")
	v1Router.HandleFunc("/transactions/{transactionID}", transactionsHandler.GetTransaction).Methods("GET")

	// Remote transaction control
	v1Router.HandleFunc("/transactions/remote-start", transactionsHandler.RemoteStartTransaction).Methods("POST")
	v1Router.HandleFunc("/transactions/remote-stop", transactionsHandler.RemoteStopTransaction).Methods("POST")

	// Configuration management
	v1Router.HandleFunc("/chargepoints/{clientID}/configuration", configurationHandler.GetStoredConfiguration).Methods("GET")
	v1Router.HandleFunc("/chargepoints/{clientID}/configuration", configurationHandler.ChangeStoredConfiguration).Methods("PUT")
	v1Router.HandleFunc("/chargepoints/{clientID}/configuration/export", configurationHandler.ExportConfiguration).Methods("GET")

	// Live configuration management
	v1Router.HandleFunc("/chargepoints/{clientID}/configuration/live", configurationHandler.GetLiveConfiguration).Methods("GET")
	v1Router.HandleFunc("/chargepoints/{clientID}/configuration/live", configurationHandler.ChangeLiveConfiguration).Methods("PUT")
}