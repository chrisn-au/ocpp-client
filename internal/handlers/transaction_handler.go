package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/lorenzodonini/ocpp-go/ocppj"
)

// Business event types for MQTT publishing
type TransactionStartedEvent struct {
	TransactionID int       `json:"transactionId"`
	ConnectorID   int       `json:"connectorId"`
	IdTag         string    `json:"idTag"`
	MeterStart    int       `json:"meterStart"`
	StartTime     time.Time `json:"startTime"`
	Status        string    `json:"status"`
}

type TransactionCompletedEvent struct {
	TransactionID int       `json:"transactionId"`
	ConnectorID   int       `json:"connectorId"`
	IdTag         string    `json:"idTag"`
	MeterStart    int       `json:"meterStart"`
	MeterStop     int       `json:"meterStop"`
	StartTime     time.Time `json:"startTime"`
	StopTime      time.Time `json:"stopTime"`
	EnergyUsed    float64   `json:"energyUsed"` // kWh
	Duration      float64   `json:"duration"`   // minutes
	Reason        string    `json:"reason"`
	Status        string    `json:"status"`
}

type ConnectorStatusEvent struct {
	ConnectorID     int    `json:"connectorId"`
	Status          string `json:"status"`
	PreviousStatus  string `json:"previousStatus,omitempty"`
	TransactionID   *int   `json:"transactionId,omitempty"`
	ErrorCode       string `json:"errorCode,omitempty"`
	Info            string `json:"info,omitempty"`
	VendorID        string `json:"vendorId,omitempty"`
	VendorErrorCode string `json:"vendorErrorCode,omitempty"`
}

type MeterReadingBusinessEvent struct {
	TransactionID *int                       `json:"transactionId,omitempty"`
	ConnectorID   int                        `json:"connectorId"`
	Timestamp     time.Time                  `json:"timestamp"`
	Measurands    map[string]MeterMeasurand  `json:"measurands"`
	CurrentPower  float64                    `json:"currentPower,omitempty"` // kW
	TotalEnergy   float64                    `json:"totalEnergy,omitempty"`  // kWh
}

type MeterMeasurand struct {
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
	Context  string  `json:"context,omitempty"`
	Location string  `json:"location,omitempty"`
	Phase    string  `json:"phase,omitempty"`
}

type BillingSessionEvent struct {
	TransactionID    int       `json:"transactionId"`
	ConnectorID      int       `json:"connectorId"`
	IdTag            string    `json:"idTag"`
	StartTime        time.Time `json:"startTime"`
	EndTime          time.Time `json:"endTime"`
	EnergyConsumed   float64   `json:"energyConsumed"`   // kWh
	Duration         float64   `json:"duration"`         // minutes
	EstimatedCost    float64   `json:"estimatedCost"`
	Currency         string    `json:"currency"`
	PricingModel     string    `json:"pricingModel"`
	EnergyRate       float64   `json:"energyRate"`    // per kWh
	TimeRate         float64   `json:"timeRate"`      // per minute
}

// TransactionBusinessStateInterface defines the Redis operations needed for transactions
type TransactionBusinessStateInterface interface {
	SetWithTTL(ctx context.Context, key, value string, ttl time.Duration) error
	Set(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, error)
	CreateTransaction(info *ocppj.TransactionInfo) error
	GetTransaction(transactionID int) (*ocppj.TransactionInfo, error)
	UpdateTransaction(info *ocppj.TransactionInfo) error
	GetActiveTransactions(clientID string) ([]*ocppj.TransactionInfo, error)
}

// MQTTPublisherInterface defines the MQTT publishing operations needed for business events
type MQTTPublisherInterface interface {
	PublishTransactionEvent(clientID, eventType string, event interface{})
	PublishMeterReadingEvent(clientID string, event interface{})
	PublishConnectorEvent(clientID string, event interface{})
	PublishBillingEvent(clientID string, event interface{})
}

// TransactionHandler handles OCPP transaction-related messages
type TransactionHandler struct {
	businessState       TransactionBusinessStateInterface
	meterValueProcessor *MeterValueProcessor
	mqttPublisher       MQTTPublisherInterface
	mu                  sync.RWMutex
	connectorStates     map[string]string // tracks previous connector states for business events
}

// NewTransactionHandler creates a new transaction handler
func NewTransactionHandler(businessState TransactionBusinessStateInterface, meterValueProcessor *MeterValueProcessor) *TransactionHandler {
	return &TransactionHandler{
		businessState:       businessState,
		meterValueProcessor: meterValueProcessor,
		connectorStates:     make(map[string]string),
	}
}

// NewTransactionHandlerWithMQTT creates a new transaction handler with MQTT publisher
func NewTransactionHandlerWithMQTT(businessState TransactionBusinessStateInterface, meterValueProcessor *MeterValueProcessor, mqttPublisher MQTTPublisherInterface) *TransactionHandler {
	return &TransactionHandler{
		businessState:       businessState,
		meterValueProcessor: meterValueProcessor,
		mqttPublisher:       mqttPublisher,
		connectorStates:     make(map[string]string),
	}
}

// HandleStartTransaction processes StartTransaction requests from charge points
func (h *TransactionHandler) HandleStartTransaction(clientID, requestID string, request *core.StartTransactionRequest, sendResponse func(response *core.StartTransactionConfirmation)) {
	log.Printf("StartTransaction from %s: ConnectorID=%d, IdTag=%s, MeterStart=%d",
		clientID, request.ConnectorId, request.IdTag, request.MeterStart)

	// Generate a transaction ID
	transactionID := h.generateTransactionID()

	// Create transaction record
	transaction := &ocppj.TransactionInfo{
		TransactionID: transactionID,
		ClientID:      clientID,
		ConnectorID:   request.ConnectorId,
		IdTag:         request.IdTag,
		StartTime:     request.Timestamp.Time,
		MeterStart:    request.MeterStart,
		CurrentMeter:  request.MeterStart,
		Status:        "Active",
	}

	// Store transaction in business state
	if err := h.businessState.CreateTransaction(transaction); err != nil {
		log.Printf("Failed to store transaction: %v", err)
		// Still allow transaction to proceed - the ID is the important part
	}

	// Update connector status to charging
	if err := h.updateConnectorStatus(clientID, request.ConnectorId, "Charging", &transactionID); err != nil {
		log.Printf("Failed to update connector status: %v", err)
	}

	// Publish business event for transaction started
	if h.mqttPublisher != nil {
		event := &TransactionStartedEvent{
			TransactionID: transactionID,
			ConnectorID:   request.ConnectorId,
			IdTag:         request.IdTag,
			MeterStart:    request.MeterStart,
			StartTime:     request.Timestamp.Time,
			Status:        "started",
		}
		h.mqttPublisher.PublishTransactionEvent(clientID, "started", event)
	}

	log.Printf("StartTransaction successful - assigned transactionID: %d", transactionID)

	// Create IdTagInfo with accepted status
	idTagInfo := &types.IdTagInfo{
		Status: types.AuthorizationStatusAccepted,
	}

	// Send successful response
	response := core.NewStartTransactionConfirmation(idTagInfo, transactionID)
	sendResponse(response)
}

// HandleStopTransaction processes StopTransaction requests from charge points
func (h *TransactionHandler) HandleStopTransaction(clientID, requestID string, request *core.StopTransactionRequest, sendResponse func(response *core.StopTransactionConfirmation)) {
	log.Printf("StopTransaction from %s: TransactionID=%d, MeterStop=%d, Reason=%s",
		clientID, request.TransactionId, request.MeterStop, request.Reason)

	// Get existing transaction
	transaction, err := h.businessState.GetTransaction(request.TransactionId)
	if err != nil {
		log.Printf("Transaction %d not found: %v", request.TransactionId, err)
		// Still send successful response - transaction might have been cleaned up
	} else if transaction != nil {
		// Update transaction with final meter reading
		transaction.CurrentMeter = request.MeterStop
		transaction.Status = "Stopped"

		// Store updated transaction
		if err := h.businessState.UpdateTransaction(transaction); err != nil {
			log.Printf("Failed to update transaction: %v", err)
		}

		// Update connector status to available
		if err := h.updateConnectorStatus(clientID, transaction.ConnectorID, "Available", nil); err != nil {
			log.Printf("Failed to update connector status: %v", err)
		}

		// Publish business events for transaction completion
		if h.mqttPublisher != nil {
			stopTime := time.Now()
			energyUsed := float64(request.MeterStop-transaction.MeterStart) / 1000.0 // Convert Wh to kWh
			duration := stopTime.Sub(transaction.StartTime).Minutes()

			// Create transaction completed event
			event := &TransactionCompletedEvent{
				TransactionID: transaction.TransactionID,
				ConnectorID:   transaction.ConnectorID,
				IdTag:         transaction.IdTag,
				MeterStart:    transaction.MeterStart,
				MeterStop:     request.MeterStop,
				StartTime:     transaction.StartTime,
				StopTime:      stopTime,
				EnergyUsed:    energyUsed,
				Duration:      duration,
				Reason:        string(request.Reason),
				Status:        "completed",
			}
			h.mqttPublisher.PublishTransactionEvent(clientID, "completed", event)

			// Create billing event with estimated cost calculation
			estimatedCost := energyUsed * 0.12 // Example rate: $0.12 per kWh
			billingEvent := &BillingSessionEvent{
				TransactionID:  transaction.TransactionID,
				ConnectorID:    transaction.ConnectorID,
				IdTag:          transaction.IdTag,
				StartTime:      transaction.StartTime,
				EndTime:        stopTime,
				EnergyConsumed: energyUsed,
				Duration:       duration,
				EstimatedCost:  estimatedCost,
				Currency:       "USD",
				PricingModel:   "energy_based",
				EnergyRate:     0.12, // energy rate per kWh
				TimeRate:       0.0,  // time rate per minute
			}
			h.mqttPublisher.PublishBillingEvent(clientID, billingEvent)
		}

		log.Printf("StopTransaction successful - transaction %d stopped with %d Wh",
			request.TransactionId, request.MeterStop)
	}

	// Send successful response
	response := core.NewStopTransactionConfirmation()
	sendResponse(response)
}

// HandleStatusNotification processes StatusNotification requests from charge points
func (h *TransactionHandler) HandleStatusNotification(clientID, requestID string, request *core.StatusNotificationRequest, sendResponse func(response *core.StatusNotificationConfirmation)) {
	log.Printf("StatusNotification from %s: ConnectorID=%d, Status=%s, ErrorCode=%s",
		clientID, request.ConnectorId, request.Status, request.ErrorCode)

	// Update connector status in business state
	var transactionID *int
	if request.Status == core.ChargePointStatusCharging {
		// If charging, there should be an active transaction
		if tx, err := h.getActiveTransactionForConnector(clientID, request.ConnectorId); err == nil {
			transactionID = &tx.TransactionID
		}
	}

	statusStr := string(request.Status)

	// Get previous status for business event
	previousStatus := h.getPreviousConnectorStatus(clientID, request.ConnectorId)

	if err := h.updateConnectorStatus(clientID, request.ConnectorId, statusStr, transactionID); err != nil {
		log.Printf("Failed to update connector status: %v", err)
	}

	// Publish business event for connector status change
	if h.mqttPublisher != nil && previousStatus != statusStr {
		event := &ConnectorStatusEvent{
			ConnectorID:     request.ConnectorId,
			Status:          statusStr,
			PreviousStatus:  previousStatus,
			TransactionID:   transactionID,
			ErrorCode:       string(request.ErrorCode),
			Info:            request.Info,
			VendorID:        request.VendorId,
			VendorErrorCode: request.VendorErrorCode,
		}
		h.mqttPublisher.PublishConnectorEvent(clientID, event)
	}

	log.Printf("StatusNotification processed - Connector %d of %s is now %s",
		request.ConnectorId, clientID, request.Status)

	// Send successful response
	response := core.NewStatusNotificationConfirmation()
	sendResponse(response)
}

// HandleMeterValues processes MeterValues requests from charge points
func (h *TransactionHandler) HandleMeterValues(clientID, requestID string, request *core.MeterValuesRequest, sendResponse func(response *core.MeterValuesConfirmation)) {
	log.Printf("MeterValues from %s: ConnectorID=%d, Values=%d",
		clientID, request.ConnectorId, len(request.MeterValue))

	// Process meter values using the existing processor
	if h.meterValueProcessor != nil {
		if err := h.meterValueProcessor.ProcessMeterValues(clientID, request); err != nil {
			log.Printf("Error processing meter values: %v", err)
		}
	}

	// If there's a transaction ID, update the transaction record
	if request.TransactionId != nil {
		transaction, err := h.businessState.GetTransaction(*request.TransactionId)
		if err != nil {
			log.Printf("Transaction %d not found for meter values: %v", *request.TransactionId, err)
		} else if transaction != nil {
			// Update current meter reading from the latest meter value
			if len(request.MeterValue) > 0 {
				latestValue := request.MeterValue[len(request.MeterValue)-1]
				if len(latestValue.SampledValue) > 0 {
					// Look for energy register reading
					for _, sample := range latestValue.SampledValue {
						if sample.Measurand == types.MeasurandEnergyActiveImportRegister {
							if meterValue, err := h.parseMeterValue(sample.Value); err == nil {
								transaction.CurrentMeter = meterValue
								if err := h.businessState.UpdateTransaction(transaction); err != nil {
									log.Printf("Failed to update transaction meter: %v", err)
								} else {
									log.Printf("Updated transaction %d meter to %d",
										transaction.TransactionID, meterValue)
								}
							}
							break
						}
					}
				}
			}
		}
	}

	// Publish business event for meter reading
	if h.mqttPublisher != nil && len(request.MeterValue) > 0 {
		event := h.createMeterReadingEvent(request.ConnectorId, request.TransactionId, request.MeterValue)
		if event != nil {
			h.mqttPublisher.PublishMeterReadingEvent(clientID, event)
		}
	}

	log.Printf("MeterValues processed successfully")

	// Send successful response
	response := core.NewMeterValuesConfirmation()
	sendResponse(response)
}

// Helper methods

func (h *TransactionHandler) generateTransactionID() int {
	// Simple transaction ID generation - in production use a more robust method
	return int(time.Now().UnixNano() % 1000000)
}


func (h *TransactionHandler) getActiveTransactionForConnector(clientID string, connectorID int) (*ocppj.TransactionInfo, error) {
	// This is a simplified implementation - in production you'd have an index
	// For now, we'll just return nil since we don't have a connector->transaction mapping
	return nil, fmt.Errorf("no active transaction found for connector %d", connectorID)
}

func (h *TransactionHandler) updateConnectorStatus(clientID string, connectorID int, status string, transactionID *int) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Store the new status in local state for tracking changes
	connectorKey := fmt.Sprintf("%s:%d", clientID, connectorID)
	h.connectorStates[connectorKey] = status

	key := fmt.Sprintf("connector:%s:%d", clientID, connectorID)

	connectorStatus := map[string]interface{}{
		"status":      status,
		"lastUpdate": time.Now().Format(time.RFC3339),
	}

	if transactionID != nil {
		connectorStatus["transactionId"] = *transactionID
	}

	data, err := json.Marshal(connectorStatus)
	if err != nil {
		return fmt.Errorf("failed to marshal connector status: %w", err)
	}

	ctx := context.Background()
	return h.businessState.SetWithTTL(ctx, key, string(data), 24*time.Hour)
}

// getPreviousConnectorStatus gets the previous status for connector state tracking
func (h *TransactionHandler) getPreviousConnectorStatus(clientID string, connectorID int) string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	connectorKey := fmt.Sprintf("%s:%d", clientID, connectorID)
	if previousStatus, exists := h.connectorStates[connectorKey]; exists {
		return previousStatus
	}

	// Try to get from Redis if not in local state
	key := fmt.Sprintf("connector:%s:%d", clientID, connectorID)
	ctx := context.Background()
	if data, err := h.businessState.Get(ctx, key); err == nil {
		var connectorStatus map[string]interface{}
		if err := json.Unmarshal([]byte(data), &connectorStatus); err == nil {
			if status, ok := connectorStatus["status"].(string); ok {
				return status
			}
		}
	}

	return "Unknown"
}

// createMeterReadingEvent creates a business event from OCPP meter values
func (h *TransactionHandler) createMeterReadingEvent(connectorID int, transactionID *int, ocppMeterValues []types.MeterValue) *MeterReadingBusinessEvent {
	if len(ocppMeterValues) == 0 {
		return nil
	}

	// Use the timestamp from the first meter value, or current time if not available
	timestamp := time.Now()
	if ocppMeterValues[0].Timestamp != nil {
		timestamp = ocppMeterValues[0].Timestamp.Time
	}

	event := &MeterReadingBusinessEvent{
		TransactionID: transactionID,
		ConnectorID:   connectorID,
		Timestamp:     timestamp,
		Measurands:    make(map[string]MeterMeasurand),
	}

	// Process all sampled values from all meter values
	for _, meterValue := range ocppMeterValues {
		for _, sample := range meterValue.SampledValue {
			measurand := string(sample.Measurand)
			if measurand == "" {
				measurand = string(types.MeasurandEnergyActiveImportRegister)
			}

			// Parse the value
			value, err := strconv.ParseFloat(sample.Value, 64)
			if err != nil {
				log.Printf("Failed to parse meter value %s: %v", sample.Value, err)
				continue
			}

			// Convert units to business-friendly format
			unit := string(sample.Unit)
			if unit == "" {
				unit = string(types.UnitOfMeasureWh)
			}

			// Convert Wh to kWh for energy readings
			if unit == string(types.UnitOfMeasureWh) && (measurand == string(types.MeasurandEnergyActiveImportRegister) || measurand == string(types.MeasurandEnergyReactiveImportRegister)) {
				value = value / 1000.0
				unit = "kWh"
			}

			// Convert W to kW for power readings
			if unit == string(types.UnitOfMeasureW) && (measurand == string(types.MeasurandPowerActiveImport) || measurand == string(types.MeasurandPowerReactiveImport)) {
				value = value / 1000.0
				unit = "kW"
			}

			event.Measurands[measurand] = MeterMeasurand{
				Value:    value,
				Unit:     unit,
				Context:  string(sample.Context),
				Location: string(sample.Location),
				Phase:    string(sample.Phase),
			}

			// Set convenience fields for common measurements
			if measurand == string(types.MeasurandPowerActiveImport) {
				event.CurrentPower = value
			}
			if measurand == string(types.MeasurandEnergyActiveImportRegister) {
				event.TotalEnergy = value
			}
		}
	}

	return event
}

func (h *TransactionHandler) parseMeterValue(value string) (int, error) {
	var meterValue int
	if _, err := fmt.Sscanf(value, "%d", &meterValue); err != nil {
		return 0, fmt.Errorf("failed to parse meter value %s: %w", value, err)
	}
	return meterValue, nil
}