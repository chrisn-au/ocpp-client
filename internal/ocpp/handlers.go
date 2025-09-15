package ocpp

import (
	"log"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/lorenzodonini/ocpp-go/ocppj"

	cfgmgr "ocpp-server/config"
)

// HandleBootNotification handles BootNotification requests from charge points
func HandleBootNotification(server *ocppj.Server, businessState *ocppj.RedisBusinessState, clientID, requestId string, req *core.BootNotificationRequest) {
	log.Printf("BootNotification from %s: ChargePointModel=%s, ChargePointVendor=%s",
		clientID, req.ChargePointModel, req.ChargePointVendor)

	// Update charge point info in business state
	chargePointInfo := &ocppj.ChargePointInfo{
		ClientID:      clientID,
		LastSeen:      time.Now(),
		IsOnline:      true,
		Configuration: map[string]string{
			"ChargePointModel":  req.ChargePointModel,
			"ChargePointVendor": req.ChargePointVendor,
		},
	}

	if err := businessState.SetChargePointInfo(chargePointInfo); err != nil {
		log.Printf("Error storing charge point info: %v", err)
	}

	currentTime := types.NewDateTime(time.Now())
	response := core.NewBootNotificationConfirmation(currentTime, 300, core.RegistrationStatusAccepted)
	if err := server.SendResponse(clientID, requestId, response); err != nil {
		log.Printf("Error sending BootNotification response: %v", err)
	} else {
		log.Printf("Sent BootNotification response to %s", clientID)
	}
}

// HandleHeartbeat handles Heartbeat requests from charge points
func HandleHeartbeat(server *ocppj.Server, businessState *ocppj.RedisBusinessState, clientID, requestId string, req *core.HeartbeatRequest) {
	log.Printf("Heartbeat from %s", clientID)

	// Update last seen time
	if err := businessState.UpdateChargePointLastSeen(clientID); err != nil {
		log.Printf("Error updating last seen for %s: %v", clientID, err)
	}

	currentTime := types.NewDateTime(time.Now())
	response := core.NewHeartbeatConfirmation(currentTime)
	if err := server.SendResponse(clientID, requestId, response); err != nil {
		log.Printf("Error sending Heartbeat response: %v", err)
	} else {
		log.Printf("Sent Heartbeat response to %s", clientID)
	}
}

// HandleStatusNotification handles StatusNotification requests from charge points
func HandleStatusNotification(server *ocppj.Server, businessState *ocppj.RedisBusinessState, clientID, requestId string, req *core.StatusNotificationRequest) {
	log.Printf("StatusNotification from %s: ConnectorId=%d, Status=%s, ErrorCode=%s",
		clientID, req.ConnectorId, req.Status, req.ErrorCode)

	// Update connector status in business state
	connectorStatus := &ocppj.ConnectorStatus{
		Status:      string(req.Status),
		Transaction: nil, // Will be set during start/stop transaction
	}

	if err := businessState.SetConnectorStatus(clientID, req.ConnectorId, connectorStatus); err != nil {
		log.Printf("Error updating connector status: %v", err)
	}

	response := core.NewStatusNotificationConfirmation()
	if err := server.SendResponse(clientID, requestId, response); err != nil {
		log.Printf("Error sending StatusNotification response: %v", err)
	} else {
		log.Printf("Sent StatusNotification response to %s", clientID)
	}
}

// HandleStartTransaction handles StartTransaction requests from charge points
func HandleStartTransaction(server *ocppj.Server, businessState *ocppj.RedisBusinessState, transactionCounter *int, clientID, requestId string, req *core.StartTransactionRequest) {
	log.Printf("StartTransaction from %s: ConnectorId=%d, IdTag=%s, MeterStart=%d",
		clientID, req.ConnectorId, req.IdTag, req.MeterStart)

	// Generate unique transaction ID
	*transactionCounter++
	transactionID := *transactionCounter

	// Start transaction in business state (atomic operation)
	if err := businessState.StartTransaction(clientID, req.ConnectorId, transactionID, req.IdTag, req.MeterStart); err != nil {
		log.Printf("Error starting transaction: %v", err)
		// Send error response
		if err := server.SendError(clientID, requestId, "InternalError", "Failed to start transaction", nil); err != nil {
			log.Printf("Error sending error response: %v", err)
		}
		return
	}

	// Create success response
	idTagInfo := &types.IdTagInfo{
		Status: types.AuthorizationStatusAccepted,
	}
	response := core.NewStartTransactionConfirmation(idTagInfo, transactionID)

	if err := server.SendResponse(clientID, requestId, response); err != nil {
		log.Printf("Error sending StartTransaction response: %v", err)
	} else {
		log.Printf("Sent StartTransaction response to %s with transactionId %d", clientID, transactionID)
	}
}

// HandleStopTransaction handles StopTransaction requests from charge points
func HandleStopTransaction(server *ocppj.Server, businessState *ocppj.RedisBusinessState, clientID, requestId string, req *core.StopTransactionRequest) {
	log.Printf("StopTransaction from %s: TransactionId=%d, MeterStop=%d",
		clientID, req.TransactionId, req.MeterStop)

	// Stop transaction in business state (atomic operation)
	if err := businessState.StopTransaction(req.TransactionId, req.MeterStop); err != nil {
		log.Printf("Error stopping transaction: %v", err)
		// Send error response
		if err := server.SendError(clientID, requestId, "InternalError", "Failed to stop transaction", nil); err != nil {
			log.Printf("Error sending error response: %v", err)
		}
		return
	}

	response := core.NewStopTransactionConfirmation()
	if err := server.SendResponse(clientID, requestId, response); err != nil {
		log.Printf("Error sending StopTransaction response: %v", err)
	} else {
		log.Printf("Sent StopTransaction response to %s", clientID)
	}
}

// HandleGetConfiguration handles GetConfiguration requests from charge points
func HandleGetConfiguration(server *ocppj.Server, configManager *cfgmgr.ConfigurationManager, clientID, requestId string, req *core.GetConfigurationRequest) {
	log.Printf("GetConfiguration from %s: Keys=%v", clientID, req.Key)

	configurationKeys, unknownKeys := configManager.GetConfiguration(clientID, req.Key)

	response := core.NewGetConfigurationConfirmation(configurationKeys)
	if len(unknownKeys) > 0 {
		response.UnknownKey = unknownKeys
	}

	if err := server.SendResponse(clientID, requestId, response); err != nil {
		log.Printf("Error sending GetConfiguration response: %v", err)
	} else {
		log.Printf("Sent GetConfiguration response to %s: %d keys, %d unknown",
			clientID, len(configurationKeys), len(unknownKeys))
	}
}

// HandleChangeConfiguration handles ChangeConfiguration requests from charge points
func HandleChangeConfiguration(server *ocppj.Server, configManager *cfgmgr.ConfigurationManager, clientID, requestId string, req *core.ChangeConfigurationRequest) {
	log.Printf("ChangeConfiguration from %s: Key=%s, Value=%s",
		clientID, req.Key, req.Value)

	status := configManager.ChangeConfiguration(clientID, req.Key, req.Value)

	response := core.NewChangeConfigurationConfirmation(status)

	if err := server.SendResponse(clientID, requestId, response); err != nil {
		log.Printf("Error sending ChangeConfiguration response: %v", err)
	} else {
		log.Printf("Sent ChangeConfiguration response to %s: Status=%s",
			clientID, status)
	}
}