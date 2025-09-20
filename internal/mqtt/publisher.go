package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/lorenzodonini/ocpp-go/ocpp"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
)

// PublisherConfig holds the MQTT publisher configuration
type PublisherConfig struct {
	BrokerHost           string
	BrokerPort           int
	Username             string
	Password             string
	ClientID             string
	QoS                  byte
	Retained             bool
	BusinessEventsEnabled bool // Enable publishing of business-level events
}

// Publisher handles MQTT message publishing
type Publisher struct {
	client mqtt.Client
	config PublisherConfig
}

// OcppMessage represents the MQTT payload structure for OCPP protocol events
type OcppMessage struct {
	Timestamp   time.Time   `json:"timestamp"`
	ClientID    string      `json:"clientId"`
	MessageType string      `json:"messageType"`
	RequestID   string      `json:"requestId"`
	Payload     interface{} `json:"payload"`
}

// BusinessEvent represents the MQTT payload structure for business-level events
type BusinessEvent struct {
	Timestamp time.Time   `json:"timestamp"`
	ClientID  string      `json:"clientId"`
	EventType string      `json:"eventType"`
	EventID   string      `json:"eventId"`
	Payload   interface{} `json:"payload"`
}

// TransactionEvent represents transaction lifecycle events
type TransactionEvent struct {
	TransactionID int       `json:"transactionId"`
	ConnectorID   int       `json:"connectorId"`
	IdTag         string    `json:"idTag"`
	MeterStart    int       `json:"meterStart,omitempty"`
	MeterStop     int       `json:"meterStop,omitempty"`
	CurrentMeter  int       `json:"currentMeter,omitempty"`
	StartTime     time.Time `json:"startTime,omitempty"`
	StopTime      time.Time `json:"stopTime,omitempty"`
	EnergyUsed    float64   `json:"energyUsed,omitempty"` // kWh
	Duration      float64   `json:"duration,omitempty"`   // minutes
	Reason        string    `json:"reason,omitempty"`
	Status        string    `json:"status"`
}

// ConnectorEvent represents connector status change events
type ConnectorEvent struct {
	ConnectorID    int    `json:"connectorId"`
	Status         string `json:"status"`
	PreviousStatus string `json:"previousStatus,omitempty"`
	TransactionID  *int   `json:"transactionId,omitempty"`
	ErrorCode      string `json:"errorCode,omitempty"`
	Info           string `json:"info,omitempty"`
	VendorID       string `json:"vendorId,omitempty"`
	VendorErrorCode string `json:"vendorErrorCode,omitempty"`
}

// MeterReadingEvent represents meter value updates for business intelligence
type MeterReadingEvent struct {
	TransactionID *int                       `json:"transactionId,omitempty"`
	ConnectorID   int                        `json:"connectorId"`
	Timestamp     time.Time                  `json:"timestamp"`
	Measurands    map[string]MeterMeasurand  `json:"measurands"`
	CurrentPower  float64                    `json:"currentPower,omitempty"` // kW
	TotalEnergy   float64                    `json:"totalEnergy,omitempty"`  // kWh
}

// MeterMeasurand represents a business-friendly meter measurement
type MeterMeasurand struct {
	Value        float64 `json:"value"`
	Unit         string  `json:"unit"`
	Context      string  `json:"context,omitempty"`
	Location     string  `json:"location,omitempty"`
	Phase        string  `json:"phase,omitempty"`
}

// BillingEvent represents billing-related events
type BillingEvent struct {
	TransactionID    int       `json:"transactionId"`
	ConnectorID      int       `json:"connectorId"`
	IdTag            string    `json:"idTag"`
	StartTime        time.Time `json:"startTime"`
	EndTime          time.Time `json:"endTime,omitempty"`
	EnergyConsumed   float64   `json:"energyConsumed"`   // kWh
	Duration         float64   `json:"duration"`         // minutes
	EstimatedCost    float64   `json:"estimatedCost,omitempty"`
	Currency         string    `json:"currency,omitempty"`
	PricingModel     string    `json:"pricingModel,omitempty"`
	EnergyRate       float64   `json:"energyRate,omitempty"`    // per kWh
	TimeRate         float64   `json:"timeRate,omitempty"`      // per minute
}

// NewPublisher creates a new MQTT publisher instance
func NewPublisher(config PublisherConfig) (*Publisher, error) {
	// Create MQTT client options
	opts := mqtt.NewClientOptions()
	brokerURL := fmt.Sprintf("tcp://%s:%d", config.BrokerHost, config.BrokerPort)
	opts.AddBroker(brokerURL)
	opts.SetClientID(config.ClientID)

	if config.Username != "" {
		opts.SetUsername(config.Username)
	}
	if config.Password != "" {
		opts.SetPassword(config.Password)
	}

	// Configure connection options
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(30 * time.Second)
	opts.SetMaxReconnectInterval(5 * time.Minute)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(10 * time.Second)
	opts.SetConnectTimeout(10 * time.Second)

	// Set connection lost handler
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Printf("MQTT connection lost: %v", err)
	})

	// Set on connect handler
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Printf("MQTT client connected to broker at %s", brokerURL)
	})

	// Create client
	client := mqtt.NewClient(opts)

	publisher := &Publisher{
		client: client,
		config: config,
	}

	return publisher, nil
}

// Connect establishes connection to the MQTT broker
func (p *Publisher) Connect() error {
	if token := p.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}
	return nil
}

// Disconnect closes the connection to the MQTT broker
func (p *Publisher) Disconnect() {
	if p.client.IsConnected() {
		p.client.Disconnect(250)
		log.Println("MQTT client disconnected")
	}
}

// IsConnected checks if the MQTT client is connected
func (p *Publisher) IsConnected() bool {
	return p.client.IsConnected()
}

// PublishOCPPMessage publishes an OCPP message to MQTT asynchronously
func (p *Publisher) PublishOCPPMessage(clientID, requestID, messageType string, payload ocpp.Request) {
	go func() {
		if err := p.publishOCPPMessageSync(clientID, requestID, messageType, payload); err != nil {
			log.Printf("Failed to publish MQTT message: %v", err)
		}
	}()
}

// publishOCPPMessageSync publishes an OCPP message to MQTT synchronously
func (p *Publisher) publishOCPPMessageSync(clientID, requestID, messageType string, payload ocpp.Request) error {
	if !p.client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}

	// Create the MQTT message payload
	message := OcppMessage{
		Timestamp:   time.Now(),
		ClientID:    clientID,
		MessageType: messageType,
		RequestID:   requestID,
		Payload:     payload,
	}

	// Marshal to JSON
	jsonPayload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal MQTT message: %w", err)
	}

	// Create topic: ocpp/messages/{clientID}/{messageType}
	topic := fmt.Sprintf("ocpp/messages/%s/%s", clientID, messageType)

	// Publish the message
	token := p.client.Publish(topic, p.config.QoS, p.config.Retained, jsonPayload)

	// Wait for publication to complete (with timeout)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("timeout waiting for MQTT publish to complete")
	}

	if token.Error() != nil {
		return fmt.Errorf("failed to publish MQTT message: %w", token.Error())
	}

	log.Printf("Published MQTT message to topic '%s' for client %s", topic, clientID)
	return nil
}

// PublishOCPPResponse publishes an OCPP response to MQTT asynchronously
func (p *Publisher) PublishOCPPResponse(clientID, requestID, messageType string, payload ocpp.Response) {
	go func() {
		if err := p.publishOCPPResponseSync(clientID, requestID, messageType, payload); err != nil {
			log.Printf("Failed to publish MQTT response: %v", err)
		}
	}()
}

// publishOCPPResponseSync publishes an OCPP response to MQTT synchronously
func (p *Publisher) publishOCPPResponseSync(clientID, requestID, messageType string, payload ocpp.Response) error {
	if !p.client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}

	// Create the MQTT message payload
	message := OcppMessage{
		Timestamp:   time.Now(),
		ClientID:    clientID,
		MessageType: messageType + "Response",
		RequestID:   requestID,
		Payload:     payload,
	}

	// Marshal to JSON
	jsonPayload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal MQTT response: %w", err)
	}

	// Create topic: ocpp/responses/{clientID}/{messageType}
	topic := fmt.Sprintf("ocpp/responses/%s/%s", clientID, messageType)

	// Publish the message
	token := p.client.Publish(topic, p.config.QoS, p.config.Retained, jsonPayload)

	// Wait for publication to complete (with timeout)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("timeout waiting for MQTT response publish to complete")
	}

	if token.Error() != nil {
		return fmt.Errorf("failed to publish MQTT response: %w", token.Error())
	}

	log.Printf("Published MQTT response to topic '%s' for client %s", topic, clientID)
	return nil
}

// Business Event Publishing Methods

// PublishTransactionEvent publishes transaction lifecycle events
func (p *Publisher) PublishTransactionEvent(clientID, eventType string, event interface{}) {
	if !p.config.BusinessEventsEnabled {
		return
	}
	go func() {
		if err := p.publishBusinessEventSync(clientID, eventType, "transaction", event); err != nil {
			log.Printf("Failed to publish transaction event: %v", err)
		}
	}()
}

// PublishConnectorEvent publishes connector status change events
func (p *Publisher) PublishConnectorEvent(clientID string, event interface{}) {
	if !p.config.BusinessEventsEnabled {
		return
	}
	go func() {
		if err := p.publishBusinessEventSync(clientID, "status_changed", "connector", event); err != nil {
			log.Printf("Failed to publish connector event: %v", err)
		}
	}()
}

// PublishMeterReadingEvent publishes meter reading events for business intelligence
func (p *Publisher) PublishMeterReadingEvent(clientID string, event interface{}) {
	if !p.config.BusinessEventsEnabled {
		return
	}
	go func() {
		if err := p.publishBusinessEventSync(clientID, "meter_reading", "transaction", event); err != nil {
			log.Printf("Failed to publish meter reading event: %v", err)
		}
	}()
}

// PublishBillingEvent publishes billing-related events
func (p *Publisher) PublishBillingEvent(clientID string, event interface{}) {
	if !p.config.BusinessEventsEnabled {
		return
	}
	go func() {
		if err := p.publishBusinessEventSync(clientID, "session_cost", "billing", event); err != nil {
			log.Printf("Failed to publish billing event: %v", err)
		}
	}()
}

// publishBusinessEventSync publishes a business event synchronously
func (p *Publisher) publishBusinessEventSync(clientID, eventType, category string, payload interface{}) error {
	if !p.client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}

	// Generate event ID
	eventID := fmt.Sprintf("%s_%d", eventType, time.Now().UnixNano())

	// Create the business event message
	message := BusinessEvent{
		Timestamp: time.Now(),
		ClientID:  clientID,
		EventType: eventType,
		EventID:   eventID,
		Payload:   payload,
	}

	// Marshal to JSON
	jsonPayload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal business event: %w", err)
	}

	// Create topic based on category and event type
	// Examples:
	// - csms/transactions/{clientID}/started
	// - csms/transactions/{clientID}/completed
	// - csms/transactions/{clientID}/meter_reading
	// - csms/connectors/{clientID}/status_changed
	// - csms/billing/{clientID}/session_cost
	topic := fmt.Sprintf("csms/%ss/%s/%s", category, clientID, eventType)

	// Publish the message
	token := p.client.Publish(topic, p.config.QoS, p.config.Retained, jsonPayload)

	// Wait for publication to complete (with timeout)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("timeout waiting for business event publish to complete")
	}

	if token.Error() != nil {
		return fmt.Errorf("failed to publish business event: %w", token.Error())
	}

	log.Printf("Published business event to topic '%s' for client %s (eventType: %s)", topic, clientID, eventType)
	return nil
}

// Helper methods for creating business events from OCPP data

// CreateTransactionStartedEvent creates a business event for transaction start
func (p *Publisher) CreateTransactionStartedEvent(transactionID int, connectorID int, idTag string, meterStart int, startTime time.Time) *TransactionEvent {
	return &TransactionEvent{
		TransactionID: transactionID,
		ConnectorID:   connectorID,
		IdTag:         idTag,
		MeterStart:    meterStart,
		StartTime:     startTime,
		Status:        "started",
	}
}

// CreateTransactionCompletedEvent creates a business event for transaction completion
func (p *Publisher) CreateTransactionCompletedEvent(transactionID int, connectorID int, idTag string, meterStart, meterStop int, startTime, stopTime time.Time, reason string) *TransactionEvent {
	energyUsed := float64(meterStop-meterStart) / 1000.0 // Convert Wh to kWh
	duration := stopTime.Sub(startTime).Minutes()

	return &TransactionEvent{
		TransactionID: transactionID,
		ConnectorID:   connectorID,
		IdTag:         idTag,
		MeterStart:    meterStart,
		MeterStop:     meterStop,
		StartTime:     startTime,
		StopTime:      stopTime,
		EnergyUsed:    energyUsed,
		Duration:      duration,
		Reason:        reason,
		Status:        "completed",
	}
}

// CreateMeterReadingEvent creates a business event from OCPP meter values
func (p *Publisher) CreateMeterReadingEvent(connectorID int, transactionID *int, ocppMeterValues []types.MeterValue) *MeterReadingEvent {
	if len(ocppMeterValues) == 0 {
		return nil
	}

	// Use the timestamp from the first meter value, or current time if not available
	timestamp := time.Now()
	if ocppMeterValues[0].Timestamp != nil {
		timestamp = ocppMeterValues[0].Timestamp.Time
	}

	event := &MeterReadingEvent{
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

// CreateConnectorEvent creates a business event for connector status changes
func (p *Publisher) CreateConnectorEvent(connectorID int, status, previousStatus string, transactionID *int, errorCode, info, vendorID, vendorErrorCode string) *ConnectorEvent {
	return &ConnectorEvent{
		ConnectorID:     connectorID,
		Status:          status,
		PreviousStatus:  previousStatus,
		TransactionID:   transactionID,
		ErrorCode:       errorCode,
		Info:            info,
		VendorID:        vendorID,
		VendorErrorCode: vendorErrorCode,
	}
}

// CreateBillingEvent creates a billing event from transaction data
func (p *Publisher) CreateBillingEvent(transactionID int, connectorID int, idTag string, startTime, endTime time.Time, energyConsumed float64, estimatedCost float64, currency, pricingModel string, energyRate, timeRate float64) *BillingEvent {
	duration := endTime.Sub(startTime).Minutes()

	return &BillingEvent{
		TransactionID:  transactionID,
		ConnectorID:    connectorID,
		IdTag:          idTag,
		StartTime:      startTime,
		EndTime:        endTime,
		EnergyConsumed: energyConsumed,
		Duration:       duration,
		EstimatedCost:  estimatedCost,
		Currency:       currency,
		PricingModel:   pricingModel,
		EnergyRate:     energyRate,
		TimeRate:       timeRate,
	}
}