package tests

import (
	"context"
	"testing"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/types"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"ocpp-server/internal/handlers"
	"ocpp-server/models"
)

// Mock for meter value tests - different name to avoid conflicts
type MockMeterBusinessState struct {
	mock.Mock
}

func (m *MockMeterBusinessState) SetWithTTL(ctx context.Context, key, value string, ttl time.Duration) error {
	args := m.Called(ctx, key, value, ttl)
	return args.Error(0)
}

func (m *MockMeterBusinessState) Set(ctx context.Context, key, value string) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockMeterBusinessState) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

// Implement all required methods from RedisBusinessState interface
func (m *MockMeterBusinessState) SetChargePointInfo(info *ocppj.ChargePointInfo) error {
	args := m.Called(info)
	return args.Error(0)
}

func (m *MockMeterBusinessState) GetChargePointInfo(clientID string) (*ocppj.ChargePointInfo, error) {
	args := m.Called(clientID)
	return args.Get(0).(*ocppj.ChargePointInfo), args.Error(1)
}

func (m *MockMeterBusinessState) UpdateChargePointLastSeen(clientID string) error {
	args := m.Called(clientID)
	return args.Error(0)
}

func (m *MockMeterBusinessState) SetChargePointOffline(clientID string) error {
	args := m.Called(clientID)
	return args.Error(0)
}

func (m *MockMeterBusinessState) GetAllChargePoints() ([]*ocppj.ChargePointInfo, error) {
	args := m.Called()
	return args.Get(0).([]*ocppj.ChargePointInfo), args.Error(1)
}

func (m *MockMeterBusinessState) StoreTransaction(info *ocppj.TransactionInfo) error {
	args := m.Called(info)
	return args.Error(0)
}

func (m *MockMeterBusinessState) GetTransaction(transactionID int) (*ocppj.TransactionInfo, error) {
	args := m.Called(transactionID)
	return args.Get(0).(*ocppj.TransactionInfo), args.Error(1)
}

func (m *MockMeterBusinessState) UpdateTransaction(transactionID int, currentMeter int) error {
	args := m.Called(transactionID, currentMeter)
	return args.Error(0)
}

func (m *MockMeterBusinessState) GetActiveTransactions(clientID string) ([]*ocppj.TransactionInfo, error) {
	args := m.Called(clientID)
	return args.Get(0).([]*ocppj.TransactionInfo), args.Error(1)
}

func (m *MockMeterBusinessState) StopTransaction(transactionID int, meterStop int, reason string) error {
	args := m.Called(transactionID, meterStop, reason)
	return args.Error(0)
}

func (m *MockMeterBusinessState) SetConnectorStatus(clientID string, connectorID int, status *ocppj.ConnectorStatus) error {
	args := m.Called(clientID, connectorID, status)
	return args.Error(0)
}

func (m *MockMeterBusinessState) GetConnectorStatus(clientID string, connectorID int) (*ocppj.ConnectorStatus, error) {
	args := m.Called(clientID, connectorID)
	return args.Get(0).(*ocppj.ConnectorStatus), args.Error(1)
}

func (m *MockMeterBusinessState) GetAllConnectors(clientID string) (map[int]*ocppj.ConnectorStatus, error) {
	args := m.Called(clientID)
	return args.Get(0).(map[int]*ocppj.ConnectorStatus), args.Error(1)
}

func (m *MockMeterBusinessState) GetChargePointConfiguration(clientID string) (map[string]string, error) {
	args := m.Called(clientID)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockMeterBusinessState) SetChargePointConfiguration(clientID string, config map[string]string) error {
	args := m.Called(clientID, config)
	return args.Error(0)
}

type MockMeterConfigManager struct {
	mock.Mock
}

func (m *MockMeterConfigManager) GetConfigValue(clientID, key string) (string, bool) {
	args := m.Called(clientID, key)
	return args.String(0), args.Bool(1)
}

func TestMeterValue_ConvertMeterValues(t *testing.T) {
	mockState := new(MockMeterBusinessState)
	mockConfig := new(MockMeterConfigManager)
	processor := handlers.NewMeterValueProcessor(mockState, mockConfig)

	// Create test OCPP meter values
	timestamp := types.NewDateTime(time.Now())
	ocppValues := []types.MeterValue{
		{
			Timestamp: timestamp,
			SampledValue: []types.SampledValue{
				{
					Value:     "1500",
					Measurand: types.MeasurandEnergyActiveImportRegister,
					Unit:      types.UnitOfMeasureWh,
				},
				{
					Value:     "3500",
					Measurand: types.MeasurandPowerActiveImport,
					Unit:      types.UnitOfMeasureW,
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

func TestMeterValue_ProcessMeterValues(t *testing.T) {
	mockState := new(MockMeterBusinessState)
	mockConfig := new(MockMeterConfigManager)
	processor := handlers.NewMeterValueProcessor(mockState, mockConfig)

	// Mock configuration
	mockConfig.On("GetConfigValue", "TEST-CP", "MeterValueRetentionDays").Return("7", true)
	mockState.On("SetWithTTL", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

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
}

func TestMeterValue_AlertManager_CheckThreshold(t *testing.T) {
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

func TestMeterValue_DefaultMeasurands(t *testing.T) {
	mockState := new(MockMeterBusinessState)
	mockConfig := new(MockMeterConfigManager)
	processor := handlers.NewMeterValueProcessor(mockState, mockConfig)

	// Test conversion with empty measurand
	timestamp := types.NewDateTime(time.Now())
	ocppValues := []types.MeterValue{
		{
			Timestamp: timestamp,
			SampledValue: []types.SampledValue{
				{
					Value: "1500",
					// No measurand - should default to Energy.Active.Import.Register
				},
			},
		},
	}

	result := processor.ConvertMeterValues(ocppValues)

	assert.Len(t, result, 1)
	assert.Len(t, result[0].SampledValue, 1)
	assert.Equal(t, types.MeasurandEnergyActiveImportRegister, result[0].SampledValue[0].Measurand)
	assert.Equal(t, types.UnitOfMeasureWh, result[0].SampledValue[0].Unit)
}

func TestMeterValue_Collection(t *testing.T) {
	// Test MeterValueCollection structure
	collection := &models.MeterValueCollection{
		ChargePointID: "TEST-CP",
		ConnectorID:   1,
		TransactionID: nil,
		Values: []models.MeterValue{
			{
				Timestamp: time.Now(),
				SampledValue: []models.SampledValue{
					{
						Value:     "1500",
						Measurand: types.MeasurandEnergyActiveImportRegister,
						Unit:      types.UnitOfMeasureWh,
					},
				},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	assert.Equal(t, "TEST-CP", collection.ChargePointID)
	assert.Equal(t, 1, collection.ConnectorID)
	assert.Len(t, collection.Values, 1)
	assert.Equal(t, "1500", collection.Values[0].SampledValue[0].Value)
}

func TestMeterValue_MeasurandStats(t *testing.T) {
	// Test MeasurandStats calculations
	stats := &models.MeasurandStats{
		Min:       10.0,
		Max:       100.0,
		Sum:       550.0,
		Count:     10,
		LastValue: 55.0,
		LastTime:  time.Now(),
	}

	stats.Avg = stats.Sum / float64(stats.Count)

	assert.Equal(t, 55.0, stats.Avg)
	assert.Equal(t, 10.0, stats.Min)
	assert.Equal(t, 100.0, stats.Max)
	assert.Equal(t, 10, stats.Count)
}