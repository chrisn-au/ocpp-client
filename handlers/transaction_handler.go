package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/lorenzodonini/ocpp-go/ocppj"
)

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

// TransactionHandler handles OCPP transaction-related messages
type TransactionHandler struct {
	businessState       TransactionBusinessStateInterface
	meterValueProcessor *MeterValueProcessor
	mu                  sync.RWMutex
}

// NewTransactionHandler creates a new transaction handler
func NewTransactionHandler(businessState TransactionBusinessStateInterface, meterValueProcessor *MeterValueProcessor) *TransactionHandler {
	return &TransactionHandler{
		businessState:       businessState,
		meterValueProcessor: meterValueProcessor,
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
	if err := h.updateConnectorStatus(clientID, request.ConnectorId, statusStr, transactionID); err != nil {
		log.Printf("Failed to update connector status: %v", err)
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

func (h *TransactionHandler) parseMeterValue(value string) (int, error) {
	var meterValue int
	if _, err := fmt.Sscanf(value, "%d", &meterValue); err != nil {
		return 0, fmt.Errorf("failed to parse meter value %s: %w", value, err)
	}
	return meterValue, nil
}