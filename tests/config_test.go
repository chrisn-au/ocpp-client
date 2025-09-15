package tests

import (
	"testing"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	cfgmgr "ocpp-server/config"
)

type MockBusinessState struct {
	mock.Mock
}

func (m *MockBusinessState) GetChargePointConfiguration(clientID string) (map[string]string, error) {
	args := m.Called(clientID)
	if args.Get(0) == nil {
		return make(map[string]string), args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockBusinessState) SetChargePointConfiguration(clientID string, config map[string]string) error {
	args := m.Called(clientID, config)
	return args.Error(0)
}

func TestConfigurationManager_GetConfiguration_AllKeys(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	mockState.On("GetChargePointConfiguration", "test-cp").Return(map[string]string{}, nil)

	// Get all configuration (empty key list)
	keys, unknown := manager.GetConfiguration("test-cp", []string{})

	assert.True(t, len(keys) > 10) // Should have many standard keys
	assert.Empty(t, unknown)

	// Check some standard keys exist
	hasHeartbeat := false
	for _, kv := range keys {
		if kv.Key == "HeartbeatInterval" {
			hasHeartbeat = true
			assert.NotNil(t, kv.Value)
			assert.Equal(t, "300", *kv.Value)
			assert.False(t, kv.Readonly)
		}
	}
	assert.True(t, hasHeartbeat)
}

func TestConfigurationManager_GetConfiguration_SpecificKeys(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	mockState.On("GetChargePointConfiguration", "test-cp").Return(
		map[string]string{"HeartbeatInterval": "600"}, nil)

	// Request specific keys
	keys, unknown := manager.GetConfiguration("test-cp",
		[]string{"HeartbeatInterval", "UnknownKey"})

	assert.Len(t, keys, 1)
	assert.Len(t, unknown, 1)
	assert.Equal(t, "UnknownKey", unknown[0])

	// Check HeartbeatInterval has custom value
	assert.Equal(t, "HeartbeatInterval", keys[0].Key)
	assert.Equal(t, "600", *keys[0].Value)
}

func TestConfigurationManager_ChangeConfiguration_Valid(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	mockState.On("GetChargePointConfiguration", "test-cp").Return(map[string]string{}, nil)
	mockState.On("SetChargePointConfiguration", "test-cp", mock.Anything).Return(nil)

	// Change valid configuration
	status := manager.ChangeConfiguration("test-cp", "HeartbeatInterval", "600")

	assert.Equal(t, core.ConfigurationStatusAccepted, status)
	mockState.AssertExpectations(t)
}

func TestConfigurationManager_ChangeConfiguration_ReadOnly(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	// Try to change read-only key
	status := manager.ChangeConfiguration("test-cp", "ChargeProfileMaxStackLevel", "20")

	assert.Equal(t, core.ConfigurationStatusRejected, status)
}

func TestConfigurationManager_ChangeConfiguration_InvalidValue(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	// Try to set invalid integer
	status := manager.ChangeConfiguration("test-cp", "HeartbeatInterval", "not-a-number")

	assert.Equal(t, core.ConfigurationStatusRejected, status)
}

func TestConfigurationManager_ChangeConfiguration_UnknownKey(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	// Try to change unknown key
	status := manager.ChangeConfiguration("test-cp", "UnknownKey", "value")

	assert.Equal(t, core.ConfigurationStatusNotSupported, status)
}

func TestConfigurationManager_ChangeConfiguration_RebootRequired(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	mockState.On("GetChargePointConfiguration", "test-cp").Return(map[string]string{}, nil)
	mockState.On("SetChargePointConfiguration", "test-cp", mock.Anything).Return(nil)

	// Change a key that requires reboot
	status := manager.ChangeConfiguration("test-cp", "WebSocketPingInterval", "120")

	assert.Equal(t, core.ConfigurationStatusRebootRequired, status)
	mockState.AssertExpectations(t)
}

func TestConfigurationManager_Validators_Integer(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	mockState.On("GetChargePointConfiguration", "test-cp").Return(map[string]string{}, nil)
	mockState.On("SetChargePointConfiguration", "test-cp", mock.Anything).Return(nil)

	// Test valid integer values
	status := manager.ChangeConfiguration("test-cp", "LightIntensity", "50")
	assert.Equal(t, core.ConfigurationStatusAccepted, status)

	// Test invalid integer values
	status = manager.ChangeConfiguration("test-cp", "LightIntensity", "150") // Out of range
	assert.Equal(t, core.ConfigurationStatusRejected, status)

	status = manager.ChangeConfiguration("test-cp", "LightIntensity", "not-a-number")
	assert.Equal(t, core.ConfigurationStatusRejected, status)
}

func TestConfigurationManager_Validators_Boolean(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	// Test invalid boolean values - these should be rejected without calling Redis
	status := manager.ChangeConfiguration("test-cp", "LocalAuthorizeOffline", "yes")
	assert.Equal(t, core.ConfigurationStatusRejected, status)

	status = manager.ChangeConfiguration("test-cp", "LocalAuthorizeOffline", "1")
	assert.Equal(t, core.ConfigurationStatusRejected, status)
}

func TestConfigurationManager_Validators_CSV(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	// Test invalid CSV values for restricted fields - should be rejected without calling Redis
	status := manager.ChangeConfiguration("test-cp", "ChargingScheduleAllowedChargingRateUnit", "Current,InvalidValue")
	assert.Equal(t, core.ConfigurationStatusRejected, status)
}

func TestConfigurationManager_GetConfigValue(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	mockState.On("GetChargePointConfiguration", "test-cp").Return(
		map[string]string{"HeartbeatInterval": "600"}, nil)

	// Test getting existing key with custom value
	value, exists := manager.GetConfigValue("test-cp", "HeartbeatInterval")
	assert.True(t, exists)
	assert.Equal(t, "600", value)

	// Test getting existing key with default value
	value, exists = manager.GetConfigValue("test-cp", "ConnectionTimeOut")
	assert.True(t, exists)
	assert.Equal(t, "60", value)

	// Test getting non-existing key
	value, exists = manager.GetConfigValue("test-cp", "UnknownKey")
	assert.False(t, exists)
	assert.Equal(t, "", value)
}

func TestConfigurationManager_ExportConfiguration(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	mockState.On("GetChargePointConfiguration", "test-cp").Return(
		map[string]string{"HeartbeatInterval": "600"}, nil)

	// Export all configuration
	config := manager.ExportConfiguration("test-cp")

	assert.NotEmpty(t, config)

	// Check that HeartbeatInterval has custom value
	heartbeatConfig, exists := config["HeartbeatInterval"]
	assert.True(t, exists)
	heartbeatMap := heartbeatConfig.(map[string]interface{})
	assert.Equal(t, "600", heartbeatMap["value"])
	assert.Equal(t, false, heartbeatMap["readonly"])

	// Check that read-only key is properly marked
	chargeProfileConfig, exists := config["ChargeProfileMaxStackLevel"]
	assert.True(t, exists)
	chargeProfileMap := chargeProfileConfig.(map[string]interface{})
	assert.Equal(t, true, chargeProfileMap["readonly"])
}

func TestConfigurationManager_ChangeConfiguration_NoChange(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	mockState.On("GetChargePointConfiguration", "test-cp").Return(
		map[string]string{"HeartbeatInterval": "300"}, nil)

	// Try to set the same value (no change)
	status := manager.ChangeConfiguration("test-cp", "HeartbeatInterval", "300")

	assert.Equal(t, core.ConfigurationStatusAccepted, status)
	// SetChargePointConfiguration should not be called
	mockState.AssertNotCalled(t, "SetChargePointConfiguration")
}

func TestConfigurationManager_StandardKeys_Coverage(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := cfgmgr.NewConfigurationManager(mockState)

	mockState.On("GetChargePointConfiguration", "test-cp").Return(map[string]string{}, nil)

	// Get all keys to check coverage
	keys, _ := manager.GetConfiguration("test-cp", []string{})

	// Check for essential OCPP 1.6 keys
	expectedKeys := []string{
		"HeartbeatInterval",
		"ConnectionTimeOut",
		"MeterValuesSampledData",
		"MeterValueSampleInterval",
		"LocalAuthorizeOffline",
		"ChargeProfileMaxStackLevel",
		"SupportedFeatureProfiles",
	}

	keyMap := make(map[string]bool)
	for _, key := range keys {
		keyMap[key.Key] = true
	}

	for _, expectedKey := range expectedKeys {
		assert.True(t, keyMap[expectedKey], "Expected key %s not found", expectedKey)
	}

	// Should have at least 20 standard keys
	assert.GreaterOrEqual(t, len(keys), 20, "Should have at least 20 standard configuration keys")
}