package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"ocpp-server/models"
)

// MeterValueAggregator handles aggregation of meter values
type MeterValueAggregator struct {
	businessState BusinessStateInterface
}

// NewMeterValueAggregator creates a new aggregator
func NewMeterValueAggregator(businessState BusinessStateInterface) *MeterValueAggregator {
	return &MeterValueAggregator{
		businessState: businessState,
	}
}

// aggregatePeriod aggregates meter values for a specific period
func (mva *MeterValueAggregator) aggregatePeriod(chargePointID string, connectorID int, period string, startTime, endTime time.Time) {
	aggregate := &models.MeterValueAggregate{
		ChargePointID: chargePointID,
		ConnectorID:   connectorID,
		Period:        period,
		StartTime:     startTime,
		EndTime:       endTime,
		Measurands:    make(map[string]models.MeasurandStats),
	}

	// Scan Redis for meter values in the time period
	// pattern := fmt.Sprintf("meter_values:%s:%d:*", chargePointID, connectorID)

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

	mva.businessState.SetWithTTL(ctx, key, string(data), ttl)

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

// ProcessHourlyAggregation performs hourly aggregation for all charge points
func (mva *MeterValueAggregator) ProcessHourlyAggregation() {
	endTime := time.Now().Truncate(time.Hour)
	startTime := endTime.Add(-1 * time.Hour)

	log.Printf("Starting hourly aggregation for period %s to %s",
		startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	// Get all charge points from business state
	// This would need to be implemented in the business state
	// For now, we'll aggregate for a test charge point
	mva.aggregatePeriod("TEST-CP-001", 1, "hour", startTime, endTime)
}

// ProcessDailyAggregation performs daily aggregation for all charge points
func (mva *MeterValueAggregator) ProcessDailyAggregation() {
	endTime := time.Now().Truncate(24 * time.Hour)
	startTime := endTime.Add(-24 * time.Hour)

	log.Printf("Starting daily aggregation for period %s to %s",
		startTime.Format(time.RFC3339), endTime.Format(time.RFC3339))

	// Get all charge points from business state
	// This would need to be implemented in the business state
	// For now, we'll aggregate for a test charge point
	mva.aggregatePeriod("TEST-CP-001", 1, "day", startTime, endTime)
}