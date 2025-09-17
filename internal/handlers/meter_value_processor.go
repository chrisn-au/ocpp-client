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
	"ocpp-server/models"
)

// BusinessStateInterface defines the Redis operations needed
type BusinessStateInterface interface {
	SetWithTTL(ctx context.Context, key, value string, ttl time.Duration) error
	Set(ctx context.Context, key, value string) error
	Get(ctx context.Context, key string) (string, error)
}

// MeterValueProcessor handles meter value collection and aggregation
type MeterValueProcessor struct {
	businessState   BusinessStateInterface
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

// NewMeterValueProcessor creates a new meter value processor
func NewMeterValueProcessor(businessState BusinessStateInterface, configManager ConfigManagerInterface) *MeterValueProcessor {
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
		var timestamp time.Time
		if ocppValue.Timestamp != nil {
			timestamp = ocppValue.Timestamp.Time
		} else {
			timestamp = time.Now()
		}

		mv := models.MeterValue{
			Timestamp:    timestamp,
			SampledValue: make([]models.SampledValue, 0, len(ocppValue.SampledValue)),
		}

		for _, sample := range ocppValue.SampledValue {
			sv := models.SampledValue{
				Value:     sample.Value,
				Context:   sample.Context,
				Format:    sample.Format,
				Measurand: sample.Measurand,
				Phase:     sample.Phase,
				Location:  sample.Location,
				Unit:      sample.Unit,
			}

			// Set defaults if empty
			if sv.Measurand == "" {
				sv.Measurand = types.MeasurandEnergyActiveImportRegister
			}
			if sv.Unit == "" {
				sv.Unit = types.UnitOfMeasureWh
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
		mvp.updateMeasurandStats(chargePointID, connectorID, measurand, stat)
	}
}

// updateMeasurandStats stores measurand statistics in Redis
func (mvp *MeterValueProcessor) updateMeasurandStats(chargePointID string, connectorID int, measurand string, stat *models.MeasurandStats) {
	key := fmt.Sprintf("stats:%s:%d:%s", chargePointID, connectorID, measurand)

	data, err := json.Marshal(stat)
	if err != nil {
		log.Printf("Error marshaling stats: %v", err)
		return
	}

	ctx := context.Background()
	ttl := 24 * time.Hour // Stats expire after 24 hours
	if err := mvp.businessState.SetWithTTL(ctx, key, string(data), ttl); err != nil {
		log.Printf("Error storing stats: %v", err)
	}
}

// GetMeterValues retrieves historical meter values
func (mvp *MeterValueProcessor) GetMeterValues(query *models.MeterValueQuery) ([]models.MeterValueCollection, error) {
	// Implementation would scan Redis keys matching the pattern and filter by query parameters
	// For now, return empty result with not implemented error
	return nil, fmt.Errorf("not implemented")
}

// GetAggregatedValues retrieves aggregated meter values
func (mvp *MeterValueProcessor) GetAggregatedValues(chargePointID string, connectorID int, period string, startTime, endTime time.Time) (*models.MeterValueAggregate, error) {
	return mvp.aggregator.GetAggregate(chargePointID, connectorID, period, startTime, endTime)
}

// ConvertMeterValues is exported for testing
func (mvp *MeterValueProcessor) ConvertMeterValues(ocppValues []types.MeterValue) []models.MeterValue {
	return mvp.convertMeterValues(ocppValues)
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

	// For now, just log the aggregation attempt
	// Full implementation would scan meter values and aggregate them
}

// performDailyAggregation aggregates meter values for the past day
func (mvp *MeterValueProcessor) performDailyAggregation() {
	endTime := time.Now().Truncate(24 * time.Hour)
	startTime := endTime.Add(-24 * time.Hour)

	log.Printf("Starting daily aggregation for period %s to %s",
		startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	// For now, just log the aggregation attempt
	// Full implementation would scan meter values and aggregate them
}