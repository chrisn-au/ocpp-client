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