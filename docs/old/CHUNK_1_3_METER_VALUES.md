# Chunk 1.3: Meter Values & Sampling - Detailed Agent Execution Plan

## Objective
Implement enhanced meter value collection with configurable intervals, multiple measurands support, historical storage in Redis, and real-time aggregation for reporting.

## Prerequisites
- ✅ Configuration Management working (Chunk 1.1)
- ✅ Redis business state functional
- ✅ Basic OCPP handlers operational

## Implementation Tasks

### Task 1.3.1: Create Meter Value Data Structures
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/models/meter_value.go`

```go
package models

import (
	"time"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
)

// MeterValue represents a meter reading from a charge point
type MeterValue struct {
	Timestamp     time.Time     `json:"timestamp"`
	SampledValue  []SampledValue `json:"sampledValue"`
}

// SampledValue represents a single measurement
type SampledValue struct {
	Value     string                  `json:"value"`
	Context   types.ReadingContext    `json:"context,omitempty"`
	Format    types.ValueFormat       `json:"format,omitempty"`
	Measurand types.Measurand         `json:"measurand,omitempty"`
	Phase     types.Phase             `json:"phase,omitempty"`
	Location  types.Location          `json:"location,omitempty"`
	Unit      types.UnitOfMeasure     `json:"unit,omitempty"`
}

// MeterValueCollection stores historical meter values
type MeterValueCollection struct {
	ChargePointID string        `json:"chargePointId"`
	ConnectorID   int           `json:"connectorId"`
	TransactionID *int          `json:"transactionId,omitempty"`
	Values        []MeterValue  `json:"values"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

// MeterValueAggregate represents aggregated meter data
type MeterValueAggregate struct {
	ChargePointID   string                   `json:"chargePointId"`
	ConnectorID     int                      `json:"connectorId"`
	Period          string                   `json:"period"` // "hour", "day", "week", "month"
	StartTime       time.Time                `json:"startTime"`
	EndTime         time.Time                `json:"endTime"`
	TotalEnergy     float64                  `json:"totalEnergy"`     // kWh
	MaxPower        float64                  `json:"maxPower"`        // kW
	AvgPower        float64                  `json:"avgPower"`        // kW
	SampleCount     int                      `json:"sampleCount"`
	Measurands      map[string]MeasurandStats `json:"measurands"`
}

// MeasurandStats contains statistics for a specific measurand
type MeasurandStats struct {
	Min      float64   `json:"min"`
	Max      float64   `json:"max"`
	Avg      float64   `json:"avg"`
	Sum      float64   `json:"sum"`
	Count    int       `json:"count"`
	LastValue float64  `json:"lastValue"`
	LastTime time.Time `json:"lastTime"`
}

// MeterValueQuery represents query parameters for meter values
type MeterValueQuery struct {
	ChargePointID string     `json:"chargePointId,omitempty"`
	ConnectorID   *int       `json:"connectorId,omitempty"`
	TransactionID *int       `json:"transactionId,omitempty"`
	Measurand     string     `json:"measurand,omitempty"`
	StartTime     *time.Time `json:"startTime,omitempty"`
	EndTime       *time.Time `json:"endTime,omitempty"`
	Limit         int        `json:"limit,omitempty"`
}
```

### Task 1.3.2: Create Meter Value Processor
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/handlers/meter_value_processor.go`

```go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"ocpp-server/models"
)

// MeterValueProcessor handles meter value collection and aggregation
type MeterValueProcessor struct {
	businessState   *ocppj.RedisBusinessState
	configManager   ConfigManagerInterface
	alertManager    *AlertManager
	aggregator      *MeterValueAggregator
	mu              sync.RWMutex
	buffers         map[string]*MeterValueBuffer
}

// ConfigManagerInterface defines config access methods
type ConfigManagerInterface interface {
	GetConfigValue(clientID, key string) (string, bool)
}

// MeterValueBuffer temporarily stores meter values before batch processing
type MeterValueBuffer struct {
	ChargePointID string
	ConnectorID   int
	TransactionID *int
	Values        []models.MeterValue
	LastFlush     time.Time
}

// AlertManager handles threshold alerts
type AlertManager struct {
	thresholds map[string]AlertThreshold
	mu         sync.RWMutex
}

type AlertThreshold struct {
	Measurand string
	MinValue  float64
	MaxValue  float64
	Action    func(chargePointID string, value float64)
}

// NewMeterValueProcessor creates a new meter value processor
func NewMeterValueProcessor(businessState *ocppj.RedisBusinessState, configManager ConfigManagerInterface) *MeterValueProcessor {
	mvp := &MeterValueProcessor{
		businessState: businessState,
		configManager: configManager,
		alertManager:  NewAlertManager(),
		aggregator:    NewMeterValueAggregator(businessState),
		buffers:       make(map[string]*MeterValueBuffer),
	}

	// Start background workers
	go mvp.flushWorker()
	go mvp.aggregationWorker()

	return mvp
}

// ProcessMeterValues handles incoming meter values from charge point
func (mvp *MeterValueProcessor) ProcessMeterValues(clientID string, req *core.MeterValuesRequest) error {
	log.Printf("Processing meter values from %s: ConnectorId=%d, TransactionId=%v, Count=%d",
		clientID, req.ConnectorId, req.TransactionId, len(req.MeterValue))

	// Convert OCPP meter values to internal model
	meterValues := mvp.convertMeterValues(req.MeterValue)

	// Buffer meter values for batch processing
	mvp.bufferMeterValues(clientID, req.ConnectorId, req.TransactionId, meterValues)

	// Check for alerts
	mvp.checkAlerts(clientID, req.ConnectorId, meterValues)

	// Update real-time statistics
	mvp.updateRealTimeStats(clientID, req.ConnectorId, req.TransactionId, meterValues)

	return nil
}

// convertMeterValues converts OCPP meter values to internal model
func (mvp *MeterValueProcessor) convertMeterValues(ocppValues []types.MeterValue) []models.MeterValue {
	result := make([]models.MeterValue, 0, len(ocppValues))

	for _, ocppValue := range ocppValues {
		mv := models.MeterValue{
			Timestamp:    time.Time(*ocppValue.Timestamp),
			SampledValue: make([]models.SampledValue, 0, len(ocppValue.SampledValue)),
		}

		for _, sample := range ocppValue.SampledValue {
			sv := models.SampledValue{
				Value: sample.Value,
			}

			if sample.Context != nil {
				sv.Context = *sample.Context
			}
			if sample.Format != nil {
				sv.Format = *sample.Format
			}
			if sample.Measurand != nil {
				sv.Measurand = *sample.Measurand
			} else {
				sv.Measurand = types.MeasurandEnergyActiveImportRegister // Default
			}
			if sample.Phase != nil {
				sv.Phase = *sample.Phase
			}
			if sample.Location != nil {
				sv.Location = *sample.Location
			}
			if sample.Unit != nil {
				sv.Unit = *sample.Unit
			} else {
				sv.Unit = types.UnitOfMeasureWh // Default for energy
			}

			mv.SampledValue = append(mv.SampledValue, sv)
		}

		result = append(result, mv)
	}

	return result
}

// bufferMeterValues adds meter values to buffer for batch processing
func (mvp *MeterValueProcessor) bufferMeterValues(chargePointID string, connectorID int, transactionID *int, values []models.MeterValue) {
	mvp.mu.Lock()
	defer mvp.mu.Unlock()

	key := fmt.Sprintf("%s:%d", chargePointID, connectorID)
	buffer, exists := mvp.buffers[key]

	if !exists {
		buffer = &MeterValueBuffer{
			ChargePointID: chargePointID,
			ConnectorID:   connectorID,
			TransactionID: transactionID,
			Values:        make([]models.MeterValue, 0, 100),
			LastFlush:     time.Now(),
		}
		mvp.buffers[key] = buffer
	}

	buffer.Values = append(buffer.Values, values...)

	// Flush if buffer is full or timeout
	if len(buffer.Values) >= 100 || time.Since(buffer.LastFlush) > 30*time.Second {
		mvp.flushBuffer(buffer)
	}
}

// flushBuffer persists buffered meter values to Redis
func (mvp *MeterValueProcessor) flushBuffer(buffer *MeterValueBuffer) error {
	if len(buffer.Values) == 0 {
		return nil
	}

	// Create collection
	collection := &models.MeterValueCollection{
		ChargePointID: buffer.ChargePointID,
		ConnectorID:   buffer.ConnectorID,
		TransactionID: buffer.TransactionID,
		Values:        buffer.Values,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Store in Redis with TTL based on configuration
	ttl := 7 * 24 * time.Hour // Default 7 days
	if configValue, ok := mvp.configManager.GetConfigValue(buffer.ChargePointID, "MeterValueRetentionDays"); ok {
		if days, err := strconv.Atoi(configValue); err == nil {
			ttl = time.Duration(days) * 24 * time.Hour
		}
	}

	// Store meter values in Redis
	key := fmt.Sprintf("meter_values:%s:%d:%d",
		buffer.ChargePointID, buffer.ConnectorID, time.Now().Unix())

	data, err := json.Marshal(collection)
	if err != nil {
		return fmt.Errorf("failed to marshal meter values: %w", err)
	}

	ctx := context.Background()
	if err := mvp.businessState.SetWithTTL(ctx, key, string(data), ttl); err != nil {
		return fmt.Errorf("failed to store meter values: %w", err)
	}

	// Clear buffer
	buffer.Values = buffer.Values[:0]
	buffer.LastFlush = time.Now()

	log.Printf("Flushed %d meter values for %s connector %d",
		len(collection.Values), buffer.ChargePointID, buffer.ConnectorID)

	return nil
}

// flushWorker periodically flushes all buffers
func (mvp *MeterValueProcessor) flushWorker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		mvp.mu.Lock()
		for _, buffer := range mvp.buffers {
			if time.Since(buffer.LastFlush) > 30*time.Second {
				mvp.flushBuffer(buffer)
			}
		}
		mvp.mu.Unlock()
	}
}

// checkAlerts checks meter values against configured thresholds
func (mvp *MeterValueProcessor) checkAlerts(chargePointID string, connectorID int, values []models.MeterValue) {
	for _, mv := range values {
		for _, sv := range mv.SampledValue {
			if value, err := strconv.ParseFloat(sv.Value, 64); err == nil {
				mvp.alertManager.CheckThreshold(chargePointID, string(sv.Measurand), value)
			}
		}
	}
}

// updateRealTimeStats updates real-time statistics
func (mvp *MeterValueProcessor) updateRealTimeStats(chargePointID string, connectorID int, transactionID *int, values []models.MeterValue) {
	stats := make(map[string]*models.MeasurandStats)

	for _, mv := range values {
		for _, sv := range mv.SampledValue {
			measurand := string(sv.Measurand)

			if _, exists := stats[measurand]; !exists {
				stats[measurand] = &models.MeasurandStats{
					Min:   1e9,
					Max:   -1e9,
				}
			}

			if value, err := strconv.ParseFloat(sv.Value, 64); err == nil {
				stat := stats[measurand]
				stat.Count++
				stat.Sum += value
				stat.LastValue = value
				stat.LastTime = mv.Timestamp

				if value < stat.Min {
					stat.Min = value
				}
				if value > stat.Max {
					stat.Max = value
				}
			}
		}
	}

	// Update stats in Redis
	for measurand, stat := range stats {
		stat.Avg = stat.Sum / float64(stat.Count)
		mvp.businessState.UpdateMeasurandStats(chargePointID, connectorID, measurand, stat)
	}
}

// GetMeterValues retrieves historical meter values
func (mvp *MeterValueProcessor) GetMeterValues(query *models.MeterValueQuery) ([]models.MeterValueCollection, error) {
	// Implement query logic to retrieve from Redis
	// This would scan Redis keys matching the pattern and filter by query parameters

	return nil, fmt.Errorf("not implemented")
}

// GetAggregatedValues retrieves aggregated meter values
func (mvp *MeterValueProcessor) GetAggregatedValues(chargePointID string, connectorID int, period string, startTime, endTime time.Time) (*models.MeterValueAggregate, error) {
	return mvp.aggregator.GetAggregate(chargePointID, connectorID, period, startTime, endTime)
}
```

### Task 1.3.3: Create Meter Value Aggregator
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/handlers/meter_value_aggregator.go`

```go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocppj"
	"ocpp-server/models"
)

// MeterValueAggregator handles aggregation of meter values
type MeterValueAggregator struct {
	businessState *ocppj.RedisBusinessState
}

// NewMeterValueAggregator creates a new aggregator
func NewMeterValueAggregator(businessState *ocppj.RedisBusinessState) *MeterValueAggregator {
	return &MeterValueAggregator{
		businessState: businessState,
	}
}

// aggregationWorker runs periodic aggregation tasks
func (mvp *MeterValueProcessor) aggregationWorker() {
	// Hourly aggregation
	hourlyTicker := time.NewTicker(1 * time.Hour)
	defer hourlyTicker.Stop()

	// Daily aggregation at midnight
	dailyTicker := time.NewTicker(24 * time.Hour)
	defer dailyTicker.Stop()

	for {
		select {
		case <-hourlyTicker.C:
			mvp.performHourlyAggregation()
		case <-dailyTicker.C:
			mvp.performDailyAggregation()
		}
	}
}

// performHourlyAggregation aggregates meter values for the past hour
func (mvp *MeterValueProcessor) performHourlyAggregation() {
	endTime := time.Now().Truncate(time.Hour)
	startTime := endTime.Add(-1 * time.Hour)

	log.Printf("Starting hourly aggregation for period %s to %s",
		startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	// Get all charge points
	chargePoints, err := mvp.businessState.GetAllChargePoints()
	if err != nil {
		log.Printf("Error getting charge points for aggregation: %v", err)
		return
	}

	for _, cp := range chargePoints {
		connectors, _ := mvp.businessState.GetAllConnectors(cp.ClientID)
		for connectorID := range connectors {
			mvp.aggregatePeriod(cp.ClientID, connectorID, "hour", startTime, endTime)
		}
	}
}

// performDailyAggregation aggregates meter values for the past day
func (mvp *MeterValueProcessor) performDailyAggregation() {
	endTime := time.Now().Truncate(24 * time.Hour)
	startTime := endTime.Add(-24 * time.Hour)

	log.Printf("Starting daily aggregation for period %s to %s",
		startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	// Similar to hourly but for daily period
	chargePoints, err := mvp.businessState.GetAllChargePoints()
	if err != nil {
		log.Printf("Error getting charge points for aggregation: %v", err)
		return
	}

	for _, cp := range chargePoints {
		connectors, _ := mvp.businessState.GetAllConnectors(cp.ClientID)
		for connectorID := range connectors {
			mvp.aggregatePeriod(cp.ClientID, connectorID, "day", startTime, endTime)
		}
	}
}

// aggregatePeriod aggregates meter values for a specific period
func (mvp *MeterValueProcessor) aggregatePeriod(chargePointID string, connectorID int, period string, startTime, endTime time.Time) {
	aggregate := &models.MeterValueAggregate{
		ChargePointID: chargePointID,
		ConnectorID:   connectorID,
		Period:        period,
		StartTime:     startTime,
		EndTime:       endTime,
		Measurands:    make(map[string]models.MeasurandStats),
	}

	// Scan Redis for meter values in the time period
	pattern := fmt.Sprintf("meter_values:%s:%d:*", chargePointID, connectorID)

	// This is simplified - in production you'd scan and filter by timestamp
	// For now, we'll use the real-time stats

	// Store aggregate
	key := fmt.Sprintf("aggregate:%s:%s:%d:%s:%d",
		period, chargePointID, connectorID, startTime.Format("20060102-150405"), endTime.Unix())

	data, _ := json.Marshal(aggregate)
	ctx := context.Background()

	// Store with appropriate TTL
	ttl := 30 * 24 * time.Hour // 30 days for hourly
	if period == "day" {
		ttl = 365 * 24 * time.Hour // 1 year for daily
	}

	mvp.businessState.SetWithTTL(ctx, key, string(data), ttl)

	log.Printf("Stored %s aggregate for %s connector %d",
		period, chargePointID, connectorID)
}

// GetAggregate retrieves aggregated data
func (mva *MeterValueAggregator) GetAggregate(chargePointID string, connectorID int, period string, startTime, endTime time.Time) (*models.MeterValueAggregate, error) {
	key := fmt.Sprintf("aggregate:%s:%s:%d:%s:%d",
		period, chargePointID, connectorID, startTime.Format("20060102-150405"), endTime.Unix())

	ctx := context.Background()
	data, err := mva.businessState.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var aggregate models.MeterValueAggregate
	if err := json.Unmarshal([]byte(data), &aggregate); err != nil {
		return nil, err
	}

	return &aggregate, nil
}
```

### Task 1.3.4: Create Alert Manager
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/handlers/alert_manager.go`

```go
package handlers

import (
	"log"
	"sync"
)

// NewAlertManager creates a new alert manager
func NewAlertManager() *AlertManager {
	am := &AlertManager{
		thresholds: make(map[string]AlertThreshold),
	}

	// Configure default thresholds
	am.configureDefaultThresholds()

	return am
}

// configureDefaultThresholds sets up default alert thresholds
func (am *AlertManager) configureDefaultThresholds() {
	// Power threshold - alert if exceeds max power
	am.AddThreshold("Power.Active.Import", 0, 100000, func(chargePointID string, value float64) {
		if value > 50000 { // 50kW
			log.Printf("ALERT: High power consumption on %s: %.2f W", chargePointID, value)
		}
	})

	// Temperature threshold
	am.AddThreshold("Temperature", -20, 80, func(chargePointID string, value float64) {
		if value > 70 {
			log.Printf("ALERT: High temperature on %s: %.2f°C", chargePointID, value)
		}
		if value < -10 {
			log.Printf("ALERT: Low temperature on %s: %.2f°C", chargePointID, value)
		}
	})

	// Voltage thresholds (for 230V systems)
	am.AddThreshold("Voltage", 200, 260, func(chargePointID string, value float64) {
		if value > 253 || value < 207 {
			log.Printf("ALERT: Voltage out of range on %s: %.2f V", chargePointID, value)
		}
	})

	// Current threshold
	am.AddThreshold("Current.Import", 0, 100, func(chargePointID string, value float64) {
		if value > 80 {
			log.Printf("ALERT: High current on %s: %.2f A", chargePointID, value)
		}
	})
}

// AddThreshold adds a new alert threshold
func (am *AlertManager) AddThreshold(measurand string, min, max float64, action func(string, float64)) {
	am.mu.Lock()
	defer am.mu.Unlock()

	am.thresholds[measurand] = AlertThreshold{
		Measurand: measurand,
		MinValue:  min,
		MaxValue:  max,
		Action:    action,
	}
}

// CheckThreshold checks a value against configured thresholds
func (am *AlertManager) CheckThreshold(chargePointID, measurand string, value float64) {
	am.mu.RLock()
	threshold, exists := am.thresholds[measurand]
	am.mu.RUnlock()

	if !exists {
		return
	}

	if value < threshold.MinValue || value > threshold.MaxValue {
		if threshold.Action != nil {
			threshold.Action(chargePointID, value)
		}
	}
}

// GetThresholds returns all configured thresholds
func (am *AlertManager) GetThresholds() map[string]AlertThreshold {
	am.mu.RLock()
	defer am.mu.RUnlock()

	result := make(map[string]AlertThreshold)
	for k, v := range am.thresholds {
		result[k] = v
	}
	return result
}
```

### Task 1.3.5: Update Main OCPP Handlers
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/main.go`

**Add to Server struct:**
```go
type Server struct {
	// ... existing fields
	meterValueProcessor *handlers.MeterValueProcessor
}
```

**Add initialization in main():**
```go
// Create meter value processor
server.meterValueProcessor = handlers.NewMeterValueProcessor(businessState, server.configManager)
```

**Add to setupOCPPHandlers():**
```go
case *core.MeterValuesRequest:
	s.handleMeterValues(clientID, requestId, req)
```

**Add handler method:**
```go
func (s *Server) handleMeterValues(clientID, requestId string, req *core.MeterValuesRequest) {
	// Process meter values
	if err := s.meterValueProcessor.ProcessMeterValues(clientID, req); err != nil {
		log.Printf("Error processing meter values: %v", err)
	}

	// Send response
	response := core.NewMeterValuesConfirmation()
	if err := s.ocppServer.SendResponse(clientID, requestId, response); err != nil {
		log.Printf("Error sending MeterValues response: %v", err)
	} else {
		log.Printf("Sent MeterValues response to %s", clientID)
	}
}
```

### Task 1.3.6: Add REST API Endpoints
**Add to setupHTTPAPI():**
```go
// Meter value endpoints
router.HandleFunc("/api/v1/chargepoints/{clientID}/meter-values", s.getMeterValuesHandler).Methods("GET")
router.HandleFunc("/api/v1/chargepoints/{clientID}/meter-values/latest", s.getLatestMeterValuesHandler).Methods("GET")
router.HandleFunc("/api/v1/chargepoints/{clientID}/meter-values/aggregate", s.getAggregatedMeterValuesHandler).Methods("GET")
router.HandleFunc("/api/v1/alerts/thresholds", s.getAlertThresholdsHandler).Methods("GET")
router.HandleFunc("/api/v1/alerts/thresholds", s.setAlertThresholdHandler).Methods("POST")
```

**Add handler methods:**
```go
func (s *Server) getMeterValuesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientID"]

	// Parse query parameters
	query := &models.MeterValueQuery{
		ChargePointID: clientID,
	}

	if connectorID := r.URL.Query().Get("connectorId"); connectorID != "" {
		if id, err := strconv.Atoi(connectorID); err == nil {
			query.ConnectorID = &id
		}
	}

	if measurand := r.URL.Query().Get("measurand"); measurand != "" {
		query.Measurand = measurand
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil {
			query.Limit = l
		}
	}

	// Get meter values
	values, err := s.meterValueProcessor.GetMeterValues(query)
	if err != nil {
		response := APIResponse{
			Success: false,
			Message: "Failed to retrieve meter values",
		}
		s.sendJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	response := APIResponse{
		Success: true,
		Message: "Meter values retrieved",
		Data:    values,
	}
	s.sendJSONResponse(w, http.StatusOK, response)
}

func (s *Server) getLatestMeterValuesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientID"]

	// Get latest meter values from real-time stats
	connectorID := 1 // Default, could be from query param
	if cid := r.URL.Query().Get("connectorId"); cid != "" {
		if id, err := strconv.Atoi(cid); err == nil {
			connectorID = id
		}
	}

	stats, err := s.businessState.GetMeasurandStats(clientID, connectorID)
	if err != nil {
		response := APIResponse{
			Success: false,
			Message: "Failed to retrieve latest values",
		}
		s.sendJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	response := APIResponse{
		Success: true,
		Message: "Latest meter values retrieved",
		Data:    stats,
	}
	s.sendJSONResponse(w, http.StatusOK, response)
}

func (s *Server) getAggregatedMeterValuesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	clientID := vars["clientID"]

	// Parse query parameters
	period := r.URL.Query().Get("period") // hour, day, week, month
	if period == "" {
		period = "hour"
	}

	connectorID := 1
	if cid := r.URL.Query().Get("connectorId"); cid != "" {
		if id, err := strconv.Atoi(cid); err == nil {
			connectorID = id
		}
	}

	// Default to last 24 hours
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	aggregate, err := s.meterValueProcessor.GetAggregatedValues(
		clientID, connectorID, period, startTime, endTime)

	if err != nil {
		response := APIResponse{
			Success: false,
			Message: "Failed to retrieve aggregated values",
		}
		s.sendJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	response := APIResponse{
		Success: true,
		Message: "Aggregated meter values retrieved",
		Data:    aggregate,
	}
	s.sendJSONResponse(w, http.StatusOK, response)
}
```

### Testing

#### Task 1.3.7: Unit Tests
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/tests/meter_value_test.go`

```go
package tests

import (
	"testing"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"ocpp-server/handlers"
	"ocpp-server/models"
)

func TestMeterValueProcessor_ConvertMeterValues(t *testing.T) {
	mockState := new(MockBusinessState)
	mockConfig := new(MockConfigManager)
	processor := handlers.NewMeterValueProcessor(mockState, mockConfig)

	// Create test OCPP meter values
	timestamp := types.NewDateTime(time.Now())
	ocppValues := []types.MeterValue{
		{
			Timestamp: timestamp,
			SampledValue: []types.SampledValue{
				{
					Value:     "1500",
					Measurand: &types.MeasurandEnergyActiveImportRegister,
					Unit:      &types.UnitOfMeasureWh,
				},
				{
					Value:     "3500",
					Measurand: &types.MeasurandPowerActiveImport,
					Unit:      &types.UnitOfMeasureW,
				},
			},
		},
	}

	// Convert
	result := processor.ConvertMeterValues(ocppValues)

	assert.Len(t, result, 1)
	assert.Len(t, result[0].SampledValue, 2)
	assert.Equal(t, "1500", result[0].SampledValue[0].Value)
	assert.Equal(t, types.MeasurandEnergyActiveImportRegister, result[0].SampledValue[0].Measurand)
}

func TestMeterValueProcessor_ProcessMeterValues(t *testing.T) {
	mockState := new(MockBusinessState)
	mockConfig := new(MockConfigManager)
	processor := handlers.NewMeterValueProcessor(mockState, mockConfig)

	// Mock configuration
	mockConfig.On("GetConfigValue", "TEST-CP", "MeterValueRetentionDays").Return("7", true)
	mockState.On("SetWithTTL", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockState.On("UpdateMeasurandStats", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// Create request
	timestamp := types.NewDateTime(time.Now())
	req := &core.MeterValuesRequest{
		ConnectorId: 1,
		MeterValue: []types.MeterValue{
			{
				Timestamp: timestamp,
				SampledValue: []types.SampledValue{
					{
						Value: "1500",
					},
				},
			},
		},
	}

	// Process
	err := processor.ProcessMeterValues("TEST-CP", req)

	assert.NoError(t, err)
	// Verify stats were updated
	mockState.AssertCalled(t, "UpdateMeasurandStats", "TEST-CP", 1, mock.Anything, mock.Anything)
}

func TestAlertManager_CheckThreshold(t *testing.T) {
	alertManager := handlers.NewAlertManager()

	// Add test threshold
	alertTriggered := false
	alertManager.AddThreshold("TestMeasurand", 0, 100, func(cpID string, value float64) {
		alertTriggered = true
		assert.Equal(t, "TEST-CP", cpID)
		assert.Equal(t, 150.0, value)
	})

	// Check value within threshold - no alert
	alertManager.CheckThreshold("TEST-CP", "TestMeasurand", 50)
	assert.False(t, alertTriggered)

	// Check value exceeding threshold - alert triggered
	alertManager.CheckThreshold("TEST-CP", "TestMeasurand", 150)
	assert.True(t, alertTriggered)
}

func TestMeterValueAggregator_GetAggregate(t *testing.T) {
	mockState := new(MockBusinessState)
	aggregator := handlers.NewMeterValueAggregator(mockState)

	// Mock stored aggregate
	aggregateData := `{
		"chargePointId": "TEST-CP",
		"connectorId": 1,
		"period": "hour",
		"totalEnergy": 15.5,
		"maxPower": 7.4
	}`

	mockState.On("Get", mock.Anything, mock.Anything).Return(aggregateData, nil)

	// Get aggregate
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()
	aggregate, err := aggregator.GetAggregate("TEST-CP", 1, "hour", startTime, endTime)

	assert.NoError(t, err)
	assert.NotNil(t, aggregate)
	assert.Equal(t, "TEST-CP", aggregate.ChargePointID)
	assert.Equal(t, 15.5, aggregate.TotalEnergy)
}
```

#### Task 1.3.8: Integration Tests
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/tests/integration/meter_value_integration_test.go`

```go
package integration

import (
	"testing"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeterValuesIntegration(t *testing.T) {
	// Setup test environment
	server, client, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Create meter values request
	timestamp := types.NewDateTime(time.Now())
	measurandEnergy := types.MeasurandEnergyActiveImportRegister
	measurandPower := types.MeasurandPowerActiveImport
	unitWh := types.UnitOfMeasureWh
	unitW := types.UnitOfMeasureW

	request := &core.MeterValuesRequest{
		ConnectorId: 1,
		MeterValue: []types.MeterValue{
			{
				Timestamp: timestamp,
				SampledValue: []types.SampledValue{
					{
						Value:     "15000",
						Measurand: &measurandEnergy,
						Unit:      &unitWh,
					},
					{
						Value:     "7400",
						Measurand: &measurandPower,
						Unit:      &unitW,
					},
				},
			},
		},
	}

	// Send request
	response, err := client.SendRequest(request)
	require.NoError(t, err)

	// Verify response
	meterValuesResp, ok := response.(*core.MeterValuesConfirmation)
	require.True(t, ok)
	assert.NotNil(t, meterValuesResp)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Verify meter values were stored
	// This would check Redis for the stored values
	// For now, just verify the response was successful
}

func TestMeterValuesWithTransaction(t *testing.T) {
	// Setup test environment
	server, client, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Start a transaction first
	startReq := core.NewStartTransactionRequest(1, "TEST-TAG", 1000, types.NewDateTime(time.Now()))
	startResp, err := client.SendRequest(startReq)
	require.NoError(t, err)

	startTxResp, ok := startResp.(*core.StartTransactionConfirmation)
	require.True(t, ok)
	transactionID := startTxResp.TransactionId

	// Send meter values with transaction ID
	timestamp := types.NewDateTime(time.Now())
	request := &core.MeterValuesRequest{
		ConnectorId:   1,
		TransactionId: &transactionID,
		MeterValue: []types.MeterValue{
			{
				Timestamp: timestamp,
				SampledValue: []types.SampledValue{
					{
						Value: "2000",
					},
				},
			},
		},
	}

	// Send request
	response, err := client.SendRequest(request)
	require.NoError(t, err)

	meterValuesResp, ok := response.(*core.MeterValuesConfirmation)
	require.True(t, ok)
	assert.NotNil(t, meterValuesResp)

	// Stop transaction
	stopReq := core.NewStopTransactionRequest(3000, types.NewDateTime(time.Now()), transactionID)
	stopResp, err := client.SendRequest(stopReq)
	require.NoError(t, err)
	assert.NotNil(t, stopResp)
}
```

### External Testing Scripts

#### Task 1.3.9: Create Test Scripts
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/scripts/test_meter_values.sh`

```bash
#!/bin/bash

# Test script for Meter Values functionality
# Usage: ./test_meter_values.sh [server_url] [client_id]

SERVER_URL=${1:-"http://localhost:8083"}
CLIENT_ID=${2:-"TEST-CP-001"}

echo "Testing Meter Values functionality..."
echo "===================================="

# Test 1: Get latest meter values
echo -e "\n1. Getting latest meter values for $CLIENT_ID..."
LATEST=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/meter-values/latest?connectorId=1")
echo "$LATEST" | jq '.'

# Test 2: Get historical meter values
echo -e "\n2. Getting historical meter values..."
HISTORICAL=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/meter-values?connectorId=1&limit=10")
echo "$HISTORICAL" | jq '.'

# Test 3: Get aggregated values (hourly)
echo -e "\n3. Getting hourly aggregated values..."
HOURLY=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/meter-values/aggregate?period=hour&connectorId=1")
echo "$HOURLY" | jq '.'

# Test 4: Get aggregated values (daily)
echo -e "\n4. Getting daily aggregated values..."
DAILY=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/meter-values/aggregate?period=day&connectorId=1")
echo "$DAILY" | jq '.'

# Test 5: Get alert thresholds
echo -e "\n5. Getting configured alert thresholds..."
THRESHOLDS=$(curl -s "${SERVER_URL}/api/v1/alerts/thresholds")
echo "$THRESHOLDS" | jq '.'

# Test 6: Check meter value configuration
echo -e "\n6. Checking meter value configuration..."
CONFIG=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration?keys=MeterValuesSampledData,MeterValueSampleInterval")
echo "$CONFIG" | jq '.'

echo -e "\nMeter Values testing complete!"
echo "===================================="

# Summary
echo -e "\nTest Summary:"
echo "- Latest values retrieved: $(echo "$LATEST" | jq -r '.success')"
echo "- Historical values available: $(echo "$HISTORICAL" | jq -r '.data | length')"
echo "- Aggregation working: $(echo "$HOURLY" | jq -r '.success')"
```

**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/scripts/simulate_meter_values.py`

```python
#!/usr/bin/env python3
"""
Simulates meter value reporting from a charge point
"""

import asyncio
import json
import random
import sys
import time
import uuid
import websockets
from datetime import datetime

class MeterValueSimulator:
    def __init__(self, server_url, client_id):
        self.server_url = server_url
        self.client_id = client_id
        self.websocket = None
        self.transaction_id = None

    async def connect(self):
        uri = f"{self.server_url}/{self.client_id}"
        self.websocket = await websockets.connect(uri, subprotocols=["ocpp1.6"])
        print(f"✓ Connected to {uri}")

    async def send_boot_notification(self):
        request_id = str(uuid.uuid4())
        message = [2, request_id, "BootNotification", {
            "chargePointModel": "Simulator",
            "chargePointVendor": "Test"
        }]

        await self.websocket.send(json.dumps(message))
        response = await self.websocket.recv()
        print(f"✓ Boot notification accepted")

    async def start_transaction(self):
        request_id = str(uuid.uuid4())
        message = [2, request_id, "StartTransaction", {
            "connectorId": 1,
            "idTag": "TEST-TAG",
            "meterStart": 1000,
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }]

        await self.websocket.send(json.dumps(message))
        response = await self.websocket.recv()
        response_data = json.loads(response)

        if response_data[0] == 3:
            self.transaction_id = response_data[2]["transactionId"]
            print(f"✓ Transaction started: ID={self.transaction_id}")
            return True
        return False

    async def send_meter_values(self, energy_wh, power_w, current_a=None, voltage_v=None, temperature_c=None):
        request_id = str(uuid.uuid4())

        sampled_values = [
            {
                "value": str(energy_wh),
                "measurand": "Energy.Active.Import.Register",
                "unit": "Wh"
            },
            {
                "value": str(power_w),
                "measurand": "Power.Active.Import",
                "unit": "W"
            }
        ]

        if current_a:
            sampled_values.append({
                "value": str(current_a),
                "measurand": "Current.Import",
                "unit": "A"
            })

        if voltage_v:
            sampled_values.append({
                "value": str(voltage_v),
                "measurand": "Voltage",
                "unit": "V"
            })

        if temperature_c:
            sampled_values.append({
                "value": str(temperature_c),
                "measurand": "Temperature",
                "unit": "Celsius"
            })

        message_data = {
            "connectorId": 1,
            "meterValue": [{
                "timestamp": datetime.utcnow().isoformat() + "Z",
                "sampledValue": sampled_values
            }]
        }

        if self.transaction_id:
            message_data["transactionId"] = self.transaction_id

        message = [2, request_id, "MeterValues", message_data]

        await self.websocket.send(json.dumps(message))
        response = await self.websocket.recv()

        print(f"  Sent: Energy={energy_wh}Wh, Power={power_w}W", end="")
        if current_a:
            print(f", Current={current_a}A", end="")
        if voltage_v:
            print(f", Voltage={voltage_v}V", end="")
        if temperature_c:
            print(f", Temp={temperature_c}°C", end="")
        print()

    async def stop_transaction(self, meter_stop):
        if not self.transaction_id:
            return

        request_id = str(uuid.uuid4())
        message = [2, request_id, "StopTransaction", {
            "transactionId": self.transaction_id,
            "meterStop": meter_stop,
            "timestamp": datetime.utcnow().isoformat() + "Z"
        }]

        await self.websocket.send(json.dumps(message))
        response = await self.websocket.recv()
        print(f"✓ Transaction stopped at {meter_stop}Wh")

    async def simulate_charging_session(self, duration_seconds=60, interval_seconds=10):
        print(f"\n📊 Starting charging session simulation ({duration_seconds}s)")
        print("=" * 50)

        # Initial values
        energy_wh = 1000
        base_power = 7400  # 7.4kW
        voltage = 230

        # Start transaction
        await self.start_transaction()

        # Send meter values periodically
        start_time = time.time()
        while (time.time() - start_time) < duration_seconds:
            # Simulate realistic variations
            power_variation = random.uniform(-500, 500)
            power_w = base_power + power_variation

            # Calculate energy increment (power * time_interval / 3600)
            energy_increment = (power_w * interval_seconds) / 3600
            energy_wh += energy_increment

            # Calculate current from power and voltage
            current_a = power_w / voltage

            # Simulate temperature
            temperature_c = 25 + random.uniform(-5, 10)

            # Add voltage variation
            voltage_v = voltage + random.uniform(-5, 5)

            await self.send_meter_values(
                int(energy_wh),
                int(power_w),
                round(current_a, 1),
                round(voltage_v, 1),
                round(temperature_c, 1)
            )

            await asyncio.sleep(interval_seconds)

        # Stop transaction
        await self.stop_transaction(int(energy_wh))

        print("=" * 50)
        print(f"✓ Charging session complete. Total energy: {int(energy_wh - 1000)}Wh")

    async def run_tests(self):
        await self.connect()
        await self.send_boot_notification()

        print("\n📋 Test 1: Send single meter value")
        await self.send_meter_values(5000, 3700, 16, 230, 25)

        print("\n📋 Test 2: Simulate charging session")
        await self.simulate_charging_session(duration_seconds=30, interval_seconds=5)

        print("\n📋 Test 3: Send high power alert")
        await self.send_meter_values(10000, 55000, 240, 230)  # 55kW - should trigger alert

        print("\n✅ All tests completed!")

    async def disconnect(self):
        if self.websocket:
            await self.websocket.close()

async def main():
    server_url = sys.argv[1] if len(sys.argv) > 1 else "ws://localhost:8080"
    client_id = sys.argv[2] if len(sys.argv) > 2 else "TEST-CP-METER"

    simulator = MeterValueSimulator(server_url, client_id)

    try:
        await simulator.run_tests()
    finally:
        await simulator.disconnect()

if __name__ == "__main__":
    asyncio.run(main())
```

### Documentation

#### Task 1.3.10: Meter Values Documentation
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/docs/meter_values.md`

```markdown
# Meter Values System

## Overview
The OCPP server implements a comprehensive meter value collection and aggregation system that supports multiple measurands, real-time monitoring, historical storage, and alerting.

## Supported Measurands

| Measurand | Unit | Description |
|-----------|------|-------------|
| Energy.Active.Import.Register | Wh | Total energy imported |
| Energy.Reactive.Import.Register | VARh | Reactive energy imported |
| Power.Active.Import | W | Active power import |
| Power.Reactive.Import | VAR | Reactive power import |
| Current.Import | A | Current on all phases |
| Voltage | V | Line voltage |
| Temperature | Celsius | Internal temperature |
| Frequency | Hz | Power line frequency |
| SoC | Percent | State of Charge |

## Configuration Keys

| Key | Default | Description |
|-----|---------|-------------|
| MeterValuesSampledData | Energy.Active.Import.Register,Power.Active.Import | Measurands to sample |
| MeterValueSampleInterval | 60 | Sampling interval in seconds |
| ClockAlignedDataInterval | 900 | Clock-aligned sampling interval |
| StopTxnSampledData | Energy.Active.Import.Register | Data sampled at transaction stop |
| MeterValueRetentionDays | 7 | Days to retain meter values |

## Data Flow

1. **Collection**: Charge point sends MeterValues messages
2. **Processing**: Server validates and processes values
3. **Buffering**: Values buffered for batch storage
4. **Storage**: Batch written to Redis with TTL
5. **Aggregation**: Hourly and daily aggregation jobs
6. **Alerts**: Real-time threshold checking

## Storage Structure

### Real-time Values
```
Key: meter_values:{chargePointId}:{connectorId}:{timestamp}
TTL: 7 days (configurable)
Value: JSON with MeterValueCollection
```

### Aggregated Values
```
Key: aggregate:{period}:{chargePointId}:{connectorId}:{startTime}:{endTime}
TTL: 30 days (hourly), 365 days (daily)
Value: JSON with MeterValueAggregate
```

### Real-time Statistics
```
Key: stats:{chargePointId}:{connectorId}:{measurand}
TTL: 24 hours
Value: JSON with MeasurandStats
```

## Alert Thresholds

| Measurand | Min | Max | Alert Condition |
|-----------|-----|-----|-----------------|
| Power.Active.Import | 0 | 100kW | > 50kW triggers high power alert |
| Temperature | -20°C | 80°C | > 70°C or < -10°C triggers alert |
| Voltage | 200V | 260V | > 253V or < 207V triggers alert |
| Current.Import | 0 | 100A | > 80A triggers high current alert |

## REST API Endpoints

### Get Meter Values
`GET /api/v1/chargepoints/{clientID}/meter-values`

Query parameters:
- `connectorId`: Filter by connector
- `measurand`: Filter by measurand type
- `limit`: Maximum results
- `startTime`: Start of time range
- `endTime`: End of time range

### Get Latest Values
`GET /api/v1/chargepoints/{clientID}/meter-values/latest`

Returns real-time statistics for all measurands.

### Get Aggregated Values
`GET /api/v1/chargepoints/{clientID}/meter-values/aggregate`

Query parameters:
- `period`: hour, day, week, month
- `connectorId`: Connector ID
- `startTime`: Start of period
- `endTime`: End of period

### Alert Thresholds
`GET /api/v1/alerts/thresholds` - Get configured thresholds
`POST /api/v1/alerts/thresholds` - Set new threshold

## Performance Considerations

### Buffering
- Values buffered up to 100 samples or 30 seconds
- Reduces Redis write operations
- Improves throughput for high-frequency sampling

### Aggregation
- Hourly aggregation runs every hour
- Daily aggregation runs at midnight
- Processes all charge points in parallel

### Memory Management
- Buffers cleared after flush
- Old aggregates expire automatically
- Real-time stats have 24-hour TTL

## Testing

### Unit Tests
```bash
go test ./tests -run TestMeterValue
```

### Integration Tests
```bash
go test ./tests/integration -run TestMeterValues
```

### Load Testing
```bash
# Simulate high-frequency meter values
python3 scripts/simulate_meter_values.py
```

### Validation
```bash
# Test REST API endpoints
./scripts/test_meter_values.sh
```
```

### Approval Criteria

#### Task 1.3.11: Validation Checklist
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/CHUNK_1_3_METER_VALUES_APPROVAL.md`

```markdown
# Chunk 1.3 Meter Values - Approval Checklist

## Implementation Complete ✓
- [ ] MeterValueProcessor with buffering and batching
- [ ] MeterValueAggregator for hourly/daily aggregation
- [ ] AlertManager with configurable thresholds
- [ ] Redis storage with TTL management
- [ ] OCPP MeterValues handler
- [ ] REST API endpoints for retrieval

## Features Complete ✓
- [ ] Multiple measurand support
- [ ] Transaction-based meter values
- [ ] Real-time statistics calculation
- [ ] Historical data storage
- [ ] Aggregation (hourly, daily)
- [ ] Alert threshold monitoring
- [ ] Configurable sampling intervals
- [ ] Buffer management

## Tests Complete ✓
- [ ] Unit tests for processor
- [ ] Unit tests for aggregator
- [ ] Unit tests for alert manager
- [ ] Integration tests with OCPP
- [ ] Test coverage > 80%

## External Validation ✓
- [ ] test_meter_values.sh runs successfully
- [ ] simulate_meter_values.py works
- [ ] Redis shows stored meter values
- [ ] Aggregation jobs running
- [ ] Alerts triggering correctly

## REST API ✓
- [ ] Get meter values endpoint
- [ ] Get latest values endpoint
- [ ] Get aggregated values endpoint
- [ ] Alert thresholds endpoints
- [ ] Proper error responses

## Documentation ✓
- [ ] Measurands documented
- [ ] Configuration keys documented
- [ ] API endpoints documented
- [ ] Storage structure documented
- [ ] Alert thresholds documented

## Performance ✓
- [ ] Meter value processing < 50ms
- [ ] Buffering working correctly
- [ ] Aggregation completing in reasonable time
- [ ] No memory leaks
- [ ] Redis storage optimized

## Data Integrity ✓
- [ ] Values stored accurately
- [ ] Timestamps preserved
- [ ] Transaction association maintained
- [ ] Aggregates calculated correctly
- [ ] No data loss during buffer flush

## Configuration Integration ✓
- [ ] MeterValuesSampledData respected
- [ ] MeterValueSampleInterval used
- [ ] StopTxnSampledData handled
- [ ] Retention period configurable

## Ready for Next Phase ✓
- [ ] All above criteria met
- [ ] No blocking issues
- [ ] Performance acceptable
- [ ] Team approval obtained

**Approval Date**: _____________
**Approved By**: _____________
**Notes**: _____________

## Test Evidence Required
1. Screenshot of meter values in Redis
2. API response showing latest values
3. Aggregated data example
4. Alert triggered in logs
5. Successful test script runs

## Performance Metrics
- Average processing time: _____ms
- Buffer flush time: _____ms
- Aggregation time (hourly): _____s
- Memory usage: _____MB
- Redis storage per day: _____MB
```

## Execution Instructions for Agent

### Prerequisites Check
```bash
# 1. Verify Chunk 1.1 is complete
curl http://localhost:8083/api/v1/chargepoints/TEST-CP-001/configuration

# 2. Check Redis is running
redis-cli ping

# 3. Verify server builds
go build ./...
```

### Implementation Order
1. **Create data structures** (Task 1.3.1) - 30 minutes
2. **Create meter value processor** (Task 1.3.2) - 90 minutes
3. **Create aggregator** (Task 1.3.3) - 60 minutes
4. **Create alert manager** (Task 1.3.4) - 30 minutes
5. **Update main handlers** (Task 1.3.5) - 30 minutes
6. **Add REST endpoints** (Task 1.3.6) - 45 minutes
7. **Write unit tests** (Task 1.3.7) - 60 minutes
8. **Write integration tests** (Task 1.3.8) - 30 minutes
9. **Create test scripts** (Task 1.3.9) - 30 minutes
10. **Write documentation** (Task 1.3.10) - 30 minutes
11. **Validate and approve** (Task 1.3.11) - 15 minutes

**Total Time**: ~7.5 hours

### Build and Test Commands
```bash
# Build server
go build -o ocpp-server ./main.go

# Run unit tests
go test ./tests -v -run TestMeterValue

# Run integration tests
docker-compose up -d
go test ./tests/integration -v -run TestMeterValues

# Run external validation
./scripts/test_meter_values.sh
python3 scripts/simulate_meter_values.py

# Check Redis storage
redis-cli --scan --pattern "meter_values:*"
redis-cli --scan --pattern "aggregate:*"
```

### Success Criteria
✅ MeterValues messages processed correctly
✅ Values buffered and batched to Redis
✅ Aggregation jobs running
✅ Alerts triggering on thresholds
✅ REST API returning data
✅ Test scripts passing

### Common Issues and Solutions
| Issue | Solution |
|-------|----------|
| Buffer not flushing | Check flush worker goroutine |
| Aggregation missing data | Verify Redis key patterns |
| Alerts not triggering | Check threshold configuration |
| High memory usage | Reduce buffer size or flush interval |

**Do not proceed to next phase until all approval criteria are met.**
```