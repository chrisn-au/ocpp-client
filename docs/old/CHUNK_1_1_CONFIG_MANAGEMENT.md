# Chunk 1.1: Configuration Management - Detailed Agent Execution Plan

## Objective
Implement complete OCPP 1.6 GetConfiguration and ChangeConfiguration message handlers with proper validation, storage, and standard configuration keys.

## Prerequisites
- ✅ Redis business state working
- ✅ Basic OCPP handlers functional
- ✅ Docker environment running

## Implementation Tasks

### Task 1.1.1: Create Configuration Manager
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/config/manager.go`

```go
package config

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocppj"
)

// ConfigurationManager manages charge point configurations
type ConfigurationManager struct {
	businessState *ocppj.RedisBusinessState
	defaults      map[string]*ConfigValue
	mu            sync.RWMutex
}

// ConfigValue represents a configuration key-value pair
type ConfigValue struct {
	Key        string                   `json:"key"`
	Value      string                   `json:"value"`
	ReadOnly   bool                     `json:"readonly"`
	Validator  func(string) error       `json:"-"`
	OnChange   func(string, string) error `json:"-"` // Called when value changes
}

// NewConfigurationManager creates a new configuration manager
func NewConfigurationManager(businessState *ocppj.RedisBusinessState) *ConfigurationManager {
	cm := &ConfigurationManager{
		businessState: businessState,
		defaults:      make(map[string]*ConfigValue),
	}

	// Initialize with OCPP 1.6 standard configuration keys
	cm.initializeStandardKeys()

	return cm
}

// initializeStandardKeys sets up OCPP 1.6 Core configuration keys
func (cm *ConfigurationManager) initializeStandardKeys() {
	// Core Profile keys
	cm.defaults["HeartbeatInterval"] = &ConfigValue{
		Key:      "HeartbeatInterval",
		Value:    "300", // 5 minutes default
		ReadOnly: false,
		Validator: func(v string) error {
			val, err := strconv.Atoi(v)
			if err != nil || val < 0 {
				return fmt.Errorf("HeartbeatInterval must be non-negative integer")
			}
			return nil
		},
	}

	cm.defaults["ConnectionTimeOut"] = &ConfigValue{
		Key:      "ConnectionTimeOut",
		Value:    "60",
		ReadOnly: false,
		Validator: func(v string) error {
			val, err := strconv.Atoi(v)
			if err != nil || val < 0 {
				return fmt.Errorf("ConnectionTimeOut must be non-negative integer")
			}
			return nil
		},
	}

	cm.defaults["ResetRetries"] = &ConfigValue{
		Key:      "ResetRetries",
		Value:    "3",
		ReadOnly: false,
		Validator: func(v string) error {
			val, err := strconv.Atoi(v)
			if err != nil || val < 0 {
				return fmt.Errorf("ResetRetries must be non-negative integer")
			}
			return nil
		},
	}

	cm.defaults["BlinkRepeat"] = &ConfigValue{
		Key:      "BlinkRepeat",
		Value:    "3",
		ReadOnly: false,
		Validator: cm.integerValidator(0, 10),
	}

	cm.defaults["LightIntensity"] = &ConfigValue{
		Key:      "LightIntensity",
		Value:    "50",
		ReadOnly: false,
		Validator: cm.integerValidator(0, 100),
	}

	// Meter Values Configuration
	cm.defaults["MeterValuesSampledData"] = &ConfigValue{
		Key:      "MeterValuesSampledData",
		Value:    "Energy.Active.Import.Register,Power.Active.Import",
		ReadOnly: false,
		Validator: cm.csvValidator([]string{
			"Energy.Active.Import.Register",
			"Energy.Reactive.Import.Register",
			"Energy.Active.Export.Register",
			"Energy.Reactive.Export.Register",
			"Power.Active.Import",
			"Power.Reactive.Import",
			"Power.Active.Export",
			"Power.Reactive.Export",
			"Current.Import",
			"Current.Export",
			"Voltage",
			"Temperature",
		}),
	}

	cm.defaults["MeterValuesAlignedData"] = &ConfigValue{
		Key:      "MeterValuesAlignedData",
		Value:    "Energy.Active.Import.Register",
		ReadOnly: false,
		Validator: cm.csvValidator(nil), // Same as MeterValuesSampledData
	}

	cm.defaults["MeterValueSampleInterval"] = &ConfigValue{
		Key:      "MeterValueSampleInterval",
		Value:    "60", // 1 minute default
		ReadOnly: false,
		Validator: cm.integerValidator(0, 3600),
	}

	cm.defaults["ClockAlignedDataInterval"] = &ConfigValue{
		Key:      "ClockAlignedDataInterval",
		Value:    "900", // 15 minutes default
		ReadOnly: false,
		Validator: cm.integerValidator(0, 86400),
	}

	cm.defaults["StopTxnSampledData"] = &ConfigValue{
		Key:      "StopTxnSampledData",
		Value:    "Energy.Active.Import.Register",
		ReadOnly: false,
		Validator: cm.csvValidator(nil),
	}

	cm.defaults["StopTxnAlignedData"] = &ConfigValue{
		Key:      "StopTxnAlignedData",
		Value:    "",
		ReadOnly: false,
		Validator: cm.csvValidator(nil),
	}

	// Authorization Configuration
	cm.defaults["LocalAuthorizeOffline"] = &ConfigValue{
		Key:      "LocalAuthorizeOffline",
		Value:    "true",
		ReadOnly: false,
		Validator: cm.booleanValidator(),
	}

	cm.defaults["LocalPreAuthorize"] = &ConfigValue{
		Key:      "LocalPreAuthorize",
		Value:    "false",
		ReadOnly: false,
		Validator: cm.booleanValidator(),
	}

	cm.defaults["AuthorizeRemoteTxRequests"] = &ConfigValue{
		Key:      "AuthorizeRemoteTxRequests",
		Value:    "false",
		ReadOnly: false,
		Validator: cm.booleanValidator(),
	}

	// Smart Charging Configuration
	cm.defaults["ChargeProfileMaxStackLevel"] = &ConfigValue{
		Key:      "ChargeProfileMaxStackLevel",
		Value:    "10",
		ReadOnly: true,
		Validator: cm.integerValidator(1, 100),
	}

	cm.defaults["ChargingScheduleAllowedChargingRateUnit"] = &ConfigValue{
		Key:      "ChargingScheduleAllowedChargingRateUnit",
		Value:    "Current,Power",
		ReadOnly: true,
		Validator: cm.csvValidator([]string{"Current", "Power"}),
	}

	cm.defaults["ChargingScheduleMaxPeriods"] = &ConfigValue{
		Key:      "ChargingScheduleMaxPeriods",
		Value:    "24",
		ReadOnly: true,
		Validator: cm.integerValidator(1, 1000),
	}

	cm.defaults["MaxChargingProfilesInstalled"] = &ConfigValue{
		Key:      "MaxChargingProfilesInstalled",
		Value:    "10",
		ReadOnly: true,
		Validator: cm.integerValidator(1, 100),
	}

	// Connector Configuration
	cm.defaults["ConnectorSwitch3to1PhaseSupported"] = &ConfigValue{
		Key:      "ConnectorSwitch3to1PhaseSupported",
		Value:    "false",
		ReadOnly: true,
		Validator: cm.booleanValidator(),
	}

	// WebSocket Configuration
	cm.defaults["WebSocketPingInterval"] = &ConfigValue{
		Key:      "WebSocketPingInterval",
		Value:    "60",
		ReadOnly: false,
		Validator: cm.integerValidator(0, 3600),
	}

	// Firmware/Diagnostics
	cm.defaults["GetConfigurationMaxKeys"] = &ConfigValue{
		Key:      "GetConfigurationMaxKeys",
		Value:    "100",
		ReadOnly: true,
		Validator: cm.integerValidator(1, 1000),
	}

	cm.defaults["SupportedFeatureProfiles"] = &ConfigValue{
		Key:      "SupportedFeatureProfiles",
		Value:    "Core,SmartCharging,RemoteTrigger",
		ReadOnly: true,
		Validator: cm.csvValidator([]string{"Core", "SmartCharging", "RemoteTrigger", "LocalAuthListManagement", "Reservation", "FirmwareManagement"}),
	}

	// Custom vendor keys
	cm.defaults["VendorName"] = &ConfigValue{
		Key:      "VendorName",
		Value:    "OCPP-Server",
		ReadOnly: true,
	}

	cm.defaults["Model"] = &ConfigValue{
		Key:      "Model",
		Value:    "v1.0",
		ReadOnly: true,
	}
}

// Validator functions
func (cm *ConfigurationManager) integerValidator(min, max int) func(string) error {
	return func(v string) error {
		val, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("must be an integer")
		}
		if val < min || val > max {
			return fmt.Errorf("must be between %d and %d", min, max)
		}
		return nil
	}
}

func (cm *ConfigurationManager) booleanValidator() func(string) error {
	return func(v string) error {
		v = strings.ToLower(v)
		if v != "true" && v != "false" {
			return fmt.Errorf("must be true or false")
		}
		return nil
	}
}

func (cm *ConfigurationManager) csvValidator(allowedValues []string) func(string) error {
	return func(v string) error {
		if v == "" {
			return nil // Empty is allowed for some CSV fields
		}

		parts := strings.Split(v, ",")
		if allowedValues != nil && len(allowedValues) > 0 {
			for _, part := range parts {
				part = strings.TrimSpace(part)
				found := false
				for _, allowed := range allowedValues {
					if part == allowed {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("invalid value: %s", part)
				}
			}
		}
		return nil
	}
}

// GetConfiguration retrieves configuration values for a charge point
func (cm *ConfigurationManager) GetConfiguration(clientID string, keys []string) ([]*core.KeyValue, []string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var configurationKeys []*core.KeyValue
	var unknownKeys []string

	// Get charge point specific configuration from Redis
	cpConfig, err := cm.businessState.GetChargePointConfiguration(clientID)
	if err != nil {
		log.Printf("Error getting configuration for %s: %v", clientID, err)
		cpConfig = make(map[string]string)
	}

	// If no keys specified, return all known keys
	if len(keys) == 0 {
		for key, defaultVal := range cm.defaults {
			value := defaultVal.Value
			if cpValue, exists := cpConfig[key]; exists {
				value = cpValue
			}

			configurationKeys = append(configurationKeys, &core.KeyValue{
				Key:      key,
				Readonly: defaultVal.ReadOnly,
				Value:    &value,
			})
		}
	} else {
		// Return only requested keys
		for _, key := range keys {
			if defaultVal, exists := cm.defaults[key]; exists {
				value := defaultVal.Value
				if cpValue, exists := cpConfig[key]; exists {
					value = cpValue
				}

				configurationKeys = append(configurationKeys, &core.KeyValue{
					Key:      key,
					Readonly: defaultVal.ReadOnly,
					Value:    &value,
				})
			} else {
				unknownKeys = append(unknownKeys, key)
			}
		}
	}

	return configurationKeys, unknownKeys
}

// ChangeConfiguration changes a configuration value for a charge point
func (cm *ConfigurationManager) ChangeConfiguration(clientID, key, value string) core.ConfigurationStatus {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if key exists
	defaultVal, exists := cm.defaults[key]
	if !exists {
		return core.ConfigurationStatusNotSupported
	}

	// Check if key is read-only
	if defaultVal.ReadOnly {
		return core.ConfigurationStatusRejected
	}

	// Validate new value
	if defaultVal.Validator != nil {
		if err := defaultVal.Validator(value); err != nil {
			log.Printf("Validation failed for %s=%s: %v", key, value, err)
			return core.ConfigurationStatusRejected
		}
	}

	// Get current configuration
	cpConfig, err := cm.businessState.GetChargePointConfiguration(clientID)
	if err != nil {
		log.Printf("Error getting configuration for %s: %v", clientID, err)
		cpConfig = make(map[string]string)
	}

	// Check if value actually changed
	oldValue := defaultVal.Value
	if existingValue, exists := cpConfig[key]; exists {
		oldValue = existingValue
	}

	if oldValue == value {
		return core.ConfigurationStatusAccepted // No change needed
	}

	// Update configuration in Redis
	cpConfig[key] = value
	if err := cm.businessState.SetChargePointConfiguration(clientID, cpConfig); err != nil {
		log.Printf("Error saving configuration for %s: %v", clientID, err)
		return core.ConfigurationStatusRejected
	}

	// Call OnChange handler if defined
	if defaultVal.OnChange != nil {
		if err := defaultVal.OnChange(oldValue, value); err != nil {
			log.Printf("OnChange handler failed for %s: %v", key, err)
			// Still accept the change but log the error
		}
	}

	// Check if reboot is required for this key
	if cm.requiresReboot(key) {
		return core.ConfigurationStatusRebootRequired
	}

	return core.ConfigurationStatusAccepted
}

// requiresReboot checks if changing a configuration key requires reboot
func (cm *ConfigurationManager) requiresReboot(key string) bool {
	rebootKeys := []string{
		"WebSocketPingInterval",
		"ConnectionTimeOut",
		"SupportedFeatureProfiles",
	}

	for _, rebootKey := range rebootKeys {
		if key == rebootKey {
			return true
		}
	}
	return false
}

// GetConfigValue gets a single configuration value
func (cm *ConfigurationManager) GetConfigValue(clientID, key string) (string, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	defaultVal, exists := cm.defaults[key]
	if !exists {
		return "", false
	}

	// Check charge point specific configuration
	cpConfig, err := cm.businessState.GetChargePointConfiguration(clientID)
	if err == nil {
		if value, exists := cpConfig[key]; exists {
			return value, true
		}
	}

	return defaultVal.Value, true
}

// ExportConfiguration exports all configuration for a charge point
func (cm *ConfigurationManager) ExportConfiguration(clientID string) map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make(map[string]interface{})

	cpConfig, _ := cm.businessState.GetChargePointConfiguration(clientID)

	for key, defaultVal := range cm.defaults {
		value := defaultVal.Value
		if cpValue, exists := cpConfig[key]; exists {
			value = cpValue
		}

		result[key] = map[string]interface{}{
			"value":    value,
			"readonly": defaultVal.ReadOnly,
		}
	}

	return result
}
```

### Task 1.1.2: Add Business State Methods
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-go/ocppj/redis_business_state.go`

**Add these methods to RedisBusinessState:**

```go
// GetChargePointConfiguration retrieves configuration for a charge point
func (rbs *RedisBusinessState) GetChargePointConfiguration(clientID string) (map[string]string, error) {
	key := fmt.Sprintf("%s:config:%s", rbs.keyPrefix, clientID)
	ctx := context.Background()

	result, err := rbs.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	return result, nil
}

// SetChargePointConfiguration stores configuration for a charge point
func (rbs *RedisBusinessState) SetChargePointConfiguration(clientID string, config map[string]string) error {
	key := fmt.Sprintf("%s:config:%s", rbs.keyPrefix, clientID)
	ctx := context.Background()

	// Delete existing configuration
	if err := rbs.client.Del(ctx, key).Err(); err != nil {
		return err
	}

	// Set new configuration
	if len(config) > 0 {
		if err := rbs.client.HMSet(ctx, key, config).Err(); err != nil {
			return err
		}

		// Set expiry to keep data fresh
		rbs.client.Expire(ctx, key, 7*24*time.Hour)
	}

	return nil
}
```

### Task 1.1.3: Update Main OCPP Handlers
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/main.go`

**Add to Server struct:**
```go
type Server struct {
	// ... existing fields
	configManager *config.ConfigurationManager
}
```

**Add initialization in main():**
```go
// Create configuration manager
server.configManager = config.NewConfigurationManager(businessState)
```

**Add to setupOCPPHandlers():**
```go
case *core.GetConfigurationRequest:
	s.handleGetConfiguration(clientID, requestId, req)

case *core.ChangeConfigurationRequest:
	s.handleChangeConfiguration(clientID, requestId, req)
```

**Add handler methods:**
```go
func (s *Server) handleGetConfiguration(clientID, requestId string, req *core.GetConfigurationRequest) {
	log.Printf("GetConfiguration from %s: Keys=%v", clientID, req.Key)

	configurationKeys, unknownKeys := s.configManager.GetConfiguration(clientID, req.Key)

	response := core.NewGetConfigurationConfirmation(configurationKeys)
	if len(unknownKeys) > 0 {
		response.UnknownKey = unknownKeys
	}

	if err := s.ocppServer.SendResponse(clientID, requestId, response); err != nil {
		log.Printf("Error sending GetConfiguration response: %v", err)
	} else {
		log.Printf("Sent GetConfiguration response to %s: %d keys, %d unknown",
			clientID, len(configurationKeys), len(unknownKeys))
	}
}

func (s *Server) handleChangeConfiguration(clientID, requestId string, req *core.ChangeConfigurationRequest) {
	log.Printf("ChangeConfiguration from %s: Key=%s, Value=%s",
		clientID, req.Key, req.Value)

	status := s.configManager.ChangeConfiguration(clientID, req.Key, req.Value)

	response := core.NewChangeConfigurationConfirmation(status)

	if err := s.ocppServer.SendResponse(clientID, requestId, response); err != nil {
		log.Printf("Error sending ChangeConfiguration response: %v", err)
	} else {
		log.Printf("Sent ChangeConfiguration response to %s: Status=%s",
			clientID, status)
	}
}
```

### Testing

#### Task 1.1.4: Unit Tests
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/tests/config_test.go`

```go
package tests

import (
	"testing"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"ocpp-server/config"
)

type MockBusinessState struct {
	mock.Mock
}

func (m *MockBusinessState) GetChargePointConfiguration(clientID string) (map[string]string, error) {
	args := m.Called(clientID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockBusinessState) SetChargePointConfiguration(clientID string, config map[string]string) error {
	args := m.Called(clientID, config)
	return args.Error(0)
}

func TestConfigurationManager_GetConfiguration_AllKeys(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := config.NewConfigurationManager(mockState)

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
	manager := config.NewConfigurationManager(mockState)

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
	manager := config.NewConfigurationManager(mockState)

	mockState.On("GetChargePointConfiguration", "test-cp").Return(map[string]string{}, nil)
	mockState.On("SetChargePointConfiguration", "test-cp", mock.Anything).Return(nil)

	// Change valid configuration
	status := manager.ChangeConfiguration("test-cp", "HeartbeatInterval", "600")

	assert.Equal(t, core.ConfigurationStatusAccepted, status)
	mockState.AssertExpectations(t)
}

func TestConfigurationManager_ChangeConfiguration_ReadOnly(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := config.NewConfigurationManager(mockState)

	// Try to change read-only key
	status := manager.ChangeConfiguration("test-cp", "ChargeProfileMaxStackLevel", "20")

	assert.Equal(t, core.ConfigurationStatusRejected, status)
}

func TestConfigurationManager_ChangeConfiguration_InvalidValue(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := config.NewConfigurationManager(mockState)

	// Try to set invalid integer
	status := manager.ChangeConfiguration("test-cp", "HeartbeatInterval", "not-a-number")

	assert.Equal(t, core.ConfigurationStatusRejected, status)
}

func TestConfigurationManager_ChangeConfiguration_UnknownKey(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := config.NewConfigurationManager(mockState)

	// Try to change unknown key
	status := manager.ChangeConfiguration("test-cp", "UnknownKey", "value")

	assert.Equal(t, core.ConfigurationStatusNotSupported, status)
}

func TestConfigurationManager_Validators(t *testing.T) {
	mockState := new(MockBusinessState)
	manager := config.NewConfigurationManager(mockState)

	// Test integer validator
	intValidator := manager.IntegerValidator(0, 100)
	assert.NoError(t, intValidator("50"))
	assert.Error(t, intValidator("150"))
	assert.Error(t, intValidator("not-a-number"))

	// Test boolean validator
	boolValidator := manager.BooleanValidator()
	assert.NoError(t, boolValidator("true"))
	assert.NoError(t, boolValidator("false"))
	assert.Error(t, boolValidator("yes"))

	// Test CSV validator
	csvValidator := manager.CSVValidator([]string{"Current", "Power"})
	assert.NoError(t, csvValidator("Current,Power"))
	assert.Error(t, csvValidator("Current,InvalidValue"))
}
```

#### Task 1.1.5: Integration Tests
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/tests/integration/config_integration_test.go`

```go
package integration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetConfigurationIntegration(t *testing.T) {
	// Setup test environment
	server, client, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test 1: Get all configuration
	request := core.NewGetConfigurationRequest([]string{}...)
	response, err := client.SendRequest(request)
	require.NoError(t, err)

	getConfigResp, ok := response.(*core.GetConfigurationConfirmation)
	require.True(t, ok)

	// Should have multiple configuration keys
	assert.True(t, len(getConfigResp.ConfigurationKey) > 10)

	// Test 2: Get specific keys
	request = core.NewGetConfigurationRequest("HeartbeatInterval", "UnknownKey")
	response, err = client.SendRequest(request)
	require.NoError(t, err)

	getConfigResp, ok = response.(*core.GetConfigurationConfirmation)
	require.True(t, ok)

	assert.Len(t, getConfigResp.ConfigurationKey, 1)
	assert.Len(t, getConfigResp.UnknownKey, 1)
	assert.Equal(t, "UnknownKey", getConfigResp.UnknownKey[0])
}

func TestChangeConfigurationIntegration(t *testing.T) {
	// Setup test environment
	server, client, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Test 1: Change valid configuration
	request := core.NewChangeConfigurationRequest("HeartbeatInterval", "600")
	response, err := client.SendRequest(request)
	require.NoError(t, err)

	changeConfigResp, ok := response.(*core.ChangeConfigurationConfirmation)
	require.True(t, ok)
	assert.Equal(t, core.ConfigurationStatusAccepted, changeConfigResp.Status)

	// Verify the change persisted
	getRequest := core.NewGetConfigurationRequest("HeartbeatInterval")
	getResponse, err := client.SendRequest(getRequest)
	require.NoError(t, err)

	getConfigResp, ok := getResponse.(*core.GetConfigurationConfirmation)
	require.True(t, ok)
	assert.Len(t, getConfigResp.ConfigurationKey, 1)
	assert.Equal(t, "600", *getConfigResp.ConfigurationKey[0].Value)

	// Test 2: Try to change read-only configuration
	request = core.NewChangeConfigurationRequest("ChargeProfileMaxStackLevel", "20")
	response, err = client.SendRequest(request)
	require.NoError(t, err)

	changeConfigResp, ok = response.(*core.ChangeConfigurationConfirmation)
	require.True(t, ok)
	assert.Equal(t, core.ConfigurationStatusRejected, changeConfigResp.Status)

	// Test 3: Change unknown key
	request = core.NewChangeConfigurationRequest("UnknownKey", "value")
	response, err = client.SendRequest(request)
	require.NoError(t, err)

	changeConfigResp, ok = response.(*core.ChangeConfigurationConfirmation)
	require.True(t, ok)
	assert.Equal(t, core.ConfigurationStatusNotSupported, changeConfigResp.Status)
}
```

### External Testing Scripts

#### Task 1.1.6: Create Test Scripts
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/scripts/test_configuration.sh`

```bash
#!/bin/bash

# Test script for Configuration Management
# Usage: ./test_configuration.sh [server_url] [client_id]

SERVER_URL=${1:-"http://localhost:8083"}
CLIENT_ID=${2:-"TEST-CP-001"}

echo "Testing Configuration Management..."
echo "================================="

# Test 1: Get all configuration via REST API
echo -e "\n1. Getting all configuration for $CLIENT_ID..."
ALL_CONFIG=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration")
echo "$ALL_CONFIG" | jq '.'

# Test 2: Get specific configuration keys
echo -e "\n2. Getting specific configuration keys..."
SPECIFIC_CONFIG=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration?keys=HeartbeatInterval,MeterValueSampleInterval")
echo "$SPECIFIC_CONFIG" | jq '.'

# Test 3: Change configuration value
echo -e "\n3. Changing HeartbeatInterval to 600..."
CHANGE_RESULT=$(curl -s -X PUT "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "HeartbeatInterval",
    "value": "600"
  }')
echo "$CHANGE_RESULT" | jq '.'

# Test 4: Verify the change
echo -e "\n4. Verifying configuration change..."
VERIFY_CONFIG=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration?keys=HeartbeatInterval")
echo "$VERIFY_CONFIG" | jq '.'

# Test 5: Try to change read-only configuration
echo -e "\n5. Attempting to change read-only key (should fail)..."
READONLY_RESULT=$(curl -s -X PUT "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "ChargeProfileMaxStackLevel",
    "value": "20"
  }')
echo "$READONLY_RESULT" | jq '.'

# Test 6: Export all configuration
echo -e "\n6. Exporting complete configuration..."
EXPORT_CONFIG=$(curl -s "${SERVER_URL}/api/v1/chargepoints/${CLIENT_ID}/configuration/export")
echo "$EXPORT_CONFIG" | jq '.'

echo -e "\nConfiguration testing complete!"
echo "================================="

# Summary
echo -e "\nTest Summary:"
echo "- All configuration retrieved: $(echo "$ALL_CONFIG" | jq -r '.success')"
echo "- Configuration changed: $(echo "$CHANGE_RESULT" | jq -r '.success')"
echo "- Read-only rejection: $(echo "$READONLY_RESULT" | jq -r '.success')"
```

**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/scripts/validate_config.py`

```python
#!/usr/bin/env python3
"""
Validation script for Configuration Management
Tests OCPP message flow and configuration persistence
"""

import asyncio
import json
import sys
import websockets
import uuid

class ConfigurationValidator:
    def __init__(self, server_url, client_id):
        self.server_url = server_url
        self.client_id = client_id
        self.websocket = None
        self.test_results = []

    async def connect(self):
        uri = f"{self.server_url}/{self.client_id}"
        self.websocket = await websockets.connect(uri, subprotocols=["ocpp1.6"])
        print(f"✓ Connected to {uri}")

    async def send_get_configuration(self, keys=None):
        request_id = str(uuid.uuid4())

        payload = {}
        if keys:
            payload["key"] = keys

        message = [2, request_id, "GetConfiguration", payload]

        await self.websocket.send(json.dumps(message))
        response = await self.websocket.recv()
        return json.loads(response)

    async def send_change_configuration(self, key, value):
        request_id = str(uuid.uuid4())

        message = [2, request_id, "ChangeConfiguration", {
            "key": key,
            "value": value
        }]

        await self.websocket.send(json.dumps(message))
        response = await self.websocket.recv()
        return json.loads(response)

    async def test_get_all_configuration(self):
        print("\n📋 Test 1: Get all configuration keys")
        response = await self.send_get_configuration()

        if response[0] == 3:  # CallResult
            config_keys = response[2].get("configurationKey", [])
            print(f"  ✓ Received {len(config_keys)} configuration keys")

            # Check for essential keys
            key_names = [k["key"] for k in config_keys]
            essential_keys = ["HeartbeatInterval", "MeterValueSampleInterval"]

            for key in essential_keys:
                if key in key_names:
                    print(f"  ✓ Found essential key: {key}")
                else:
                    print(f"  ✗ Missing essential key: {key}")
                    return False

            self.test_results.append(("Get all configuration", True))
            return True
        else:
            print(f"  ✗ Unexpected response: {response}")
            self.test_results.append(("Get all configuration", False))
            return False

    async def test_get_specific_keys(self):
        print("\n📋 Test 2: Get specific configuration keys")
        response = await self.send_get_configuration(["HeartbeatInterval", "UnknownKey"])

        if response[0] == 3:
            config_keys = response[2].get("configurationKey", [])
            unknown_keys = response[2].get("unknownKey", [])

            print(f"  ✓ Found {len(config_keys)} known keys")
            print(f"  ✓ Found {len(unknown_keys)} unknown keys")

            if "UnknownKey" in unknown_keys:
                print("  ✓ Unknown key correctly identified")
                self.test_results.append(("Get specific keys", True))
                return True
            else:
                print("  ✗ Unknown key not identified")
                self.test_results.append(("Get specific keys", False))
                return False
        else:
            print(f"  ✗ Unexpected response: {response}")
            self.test_results.append(("Get specific keys", False))
            return False

    async def test_change_configuration(self):
        print("\n📋 Test 3: Change configuration value")

        # Get original value
        response = await self.send_get_configuration(["HeartbeatInterval"])
        original_value = None
        if response[0] == 3:
            config_keys = response[2].get("configurationKey", [])
            if config_keys:
                original_value = config_keys[0].get("value")
                print(f"  Original HeartbeatInterval: {original_value}")

        # Change value
        new_value = "900"
        response = await self.send_change_configuration("HeartbeatInterval", new_value)

        if response[0] == 3:
            status = response[2].get("status")
            print(f"  Change status: {status}")

            if status in ["Accepted", "RebootRequired"]:
                # Verify change
                response = await self.send_get_configuration(["HeartbeatInterval"])
                if response[0] == 3:
                    config_keys = response[2].get("configurationKey", [])
                    if config_keys and config_keys[0].get("value") == new_value:
                        print(f"  ✓ Value successfully changed to {new_value}")

                        # Restore original value
                        if original_value:
                            await self.send_change_configuration("HeartbeatInterval", original_value)

                        self.test_results.append(("Change configuration", True))
                        return True

        print("  ✗ Configuration change failed")
        self.test_results.append(("Change configuration", False))
        return False

    async def test_readonly_rejection(self):
        print("\n📋 Test 4: Reject read-only configuration change")
        response = await self.send_change_configuration("ChargeProfileMaxStackLevel", "20")

        if response[0] == 3:
            status = response[2].get("status")
            print(f"  Status: {status}")

            if status == "Rejected":
                print("  ✓ Read-only key correctly rejected")
                self.test_results.append(("Read-only rejection", True))
                return True
            else:
                print(f"  ✗ Expected 'Rejected', got '{status}'")
                self.test_results.append(("Read-only rejection", False))
                return False
        else:
            print(f"  ✗ Unexpected response: {response}")
            self.test_results.append(("Read-only rejection", False))
            return False

    async def run_all_tests(self):
        await self.connect()

        print("\n" + "="*50)
        print("Configuration Management Validation")
        print("="*50)

        await self.test_get_all_configuration()
        await self.test_get_specific_keys()
        await self.test_change_configuration()
        await self.test_readonly_rejection()

        print("\n" + "="*50)
        print("Test Results Summary")
        print("="*50)

        for test_name, result in self.test_results:
            status = "✓ PASS" if result else "✗ FAIL"
            print(f"{status}: {test_name}")

        all_passed = all(result for _, result in self.test_results)

        if all_passed:
            print("\n✅ All tests passed!")
            return 0
        else:
            print("\n❌ Some tests failed!")
            return 1

    async def disconnect(self):
        if self.websocket:
            await self.websocket.close()

async def main():
    server_url = sys.argv[1] if len(sys.argv) > 1 else "ws://localhost:8080"
    client_id = sys.argv[2] if len(sys.argv) > 2 else "TEST-CP-CONFIG"

    validator = ConfigurationValidator(server_url, client_id)

    try:
        result = await validator.run_all_tests()
        return result
    finally:
        await validator.disconnect()

if __name__ == "__main__":
    exit_code = asyncio.run(main())
    sys.exit(exit_code)
```

### Documentation

#### Task 1.1.7: Configuration API Documentation
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/docs/configuration_management.md`

```markdown
# Configuration Management System

## Overview
The OCPP server implements a comprehensive configuration management system compliant with OCPP 1.6 specification. It supports all standard configuration keys plus custom vendor-specific keys.

## Standard Configuration Keys

### Core Configuration
| Key | Type | Read-Only | Default | Description |
|-----|------|-----------|---------|-------------|
| HeartbeatInterval | Integer | No | 300 | Interval in seconds between heartbeat messages |
| ConnectionTimeOut | Integer | No | 60 | WebSocket connection timeout in seconds |
| ResetRetries | Integer | No | 3 | Number of retries for reset commands |
| BlinkRepeat | Integer | No | 3 | Number of times to blink indicator |
| LightIntensity | Integer | No | 50 | LED intensity percentage (0-100) |

### Meter Values Configuration
| Key | Type | Read-Only | Default | Description |
|-----|------|-----------|---------|-------------|
| MeterValuesSampledData | CSV | No | Energy.Active.Import.Register,Power.Active.Import | Measurands to be sampled |
| MeterValueSampleInterval | Integer | No | 60 | Interval for meter value sampling |
| ClockAlignedDataInterval | Integer | No | 900 | Interval for clock-aligned data |
| StopTxnSampledData | CSV | No | Energy.Active.Import.Register | Data to sample at transaction stop |

### Authorization Configuration
| Key | Type | Read-Only | Default | Description |
|-----|------|-----------|---------|-------------|
| LocalAuthorizeOffline | Boolean | No | true | Allow local authorization when offline |
| LocalPreAuthorize | Boolean | No | false | Use local authorization cache |
| AuthorizeRemoteTxRequests | Boolean | No | false | Require authorization for remote start |

### Smart Charging Configuration
| Key | Type | Read-Only | Default | Description |
|-----|------|-----------|---------|-------------|
| ChargeProfileMaxStackLevel | Integer | Yes | 10 | Maximum charging profile stack level |
| ChargingScheduleAllowedChargingRateUnit | CSV | Yes | Current,Power | Allowed charging rate units |
| ChargingScheduleMaxPeriods | Integer | Yes | 24 | Maximum periods in charging schedule |
| MaxChargingProfilesInstalled | Integer | Yes | 10 | Maximum number of installed profiles |

## OCPP Messages

### GetConfiguration
Retrieves configuration values from the charge point.

**Request:**
```json
{
  "key": ["HeartbeatInterval", "MeterValueSampleInterval"]  // Optional
}
```

**Response:**
```json
{
  "configurationKey": [
    {
      "key": "HeartbeatInterval",
      "readonly": false,
      "value": "300"
    }
  ],
  "unknownKey": ["UnknownKey"]  // Optional
}
```

### ChangeConfiguration
Changes a configuration value on the charge point.

**Request:**
```json
{
  "key": "HeartbeatInterval",
  "value": "600"
}
```

**Response:**
```json
{
  "status": "Accepted"  // Accepted, Rejected, RebootRequired, NotSupported
}
```

## REST API Endpoints

### Get Configuration
`GET /api/v1/chargepoints/{clientID}/configuration`

Query parameters:
- `keys`: Comma-separated list of configuration keys (optional)

**Response:**
```json
{
  "success": true,
  "data": {
    "configuration": {
      "HeartbeatInterval": {
        "value": "300",
        "readonly": false
      }
    }
  }
}
```

### Change Configuration
`PUT /api/v1/chargepoints/{clientID}/configuration`

**Request Body:**
```json
{
  "key": "HeartbeatInterval",
  "value": "600"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "status": "Accepted"
  }
}
```

### Export Configuration
`GET /api/v1/chargepoints/{clientID}/configuration/export`

Returns complete configuration as JSON file download.

## Validation Rules

### Integer Validators
- Must be valid integer
- Must be within min/max range
- Negative values allowed where appropriate

### Boolean Validators
- Must be "true" or "false" (case-insensitive)

### CSV Validators
- Comma-separated values
- Each value must be from allowed list
- Empty values allowed for some fields

## Configuration Persistence

Configuration is stored in Redis with the following structure:
- Key: `ocpp:config:{clientID}`
- Type: Hash
- TTL: 7 days (refreshed on access)
- Fields: Configuration key-value pairs

## Error Handling

| Error | Status | Description |
|-------|--------|-------------|
| Unknown Key | NotSupported | Configuration key not recognized |
| Read-Only | Rejected | Attempt to change read-only key |
| Invalid Value | Rejected | Value fails validation |
| Reboot Required | RebootRequired | Change requires charge point reboot |

## Testing

### Unit Tests
```bash
go test ./tests -run TestConfiguration
```

### Integration Tests
```bash
go test ./tests/integration -run TestConfiguration
```

### External Validation
```bash
# Bash script
./scripts/test_configuration.sh

# Python validator
python3 scripts/validate_config.py
```
```

### Approval Criteria

#### Task 1.1.8: Validation Checklist
**File**: `/Users/chrishome/development/home/mcp-access/csms/ocpp-server/CHUNK_1_1_CONFIG_APPROVAL.md`

```markdown
# Chunk 1.1 Configuration Management - Approval Checklist

## Implementation Complete ✓
- [ ] ConfigurationManager with all standard OCPP 1.6 keys
- [ ] Validation functions for all data types
- [ ] Redis persistence for charge point configurations
- [ ] OCPP message handlers (GetConfiguration, ChangeConfiguration)
- [ ] Error handling and logging

## Standard Keys Implemented ✓
- [ ] Core configuration keys (HeartbeatInterval, etc.)
- [ ] Meter values configuration
- [ ] Authorization configuration
- [ ] Smart charging configuration
- [ ] WebSocket configuration
- [ ] Vendor-specific keys

## Tests Complete ✓
- [ ] Unit tests for ConfigurationManager
- [ ] Unit tests for validators
- [ ] Integration tests with OCPP messages
- [ ] Test coverage > 80%

## External Validation ✓
- [ ] test_configuration.sh runs successfully
- [ ] validate_config.py passes all tests
- [ ] Redis shows persisted configurations
- [ ] Server logs show proper message handling

## REST API ✓
- [ ] GET configuration endpoint working
- [ ] PUT configuration endpoint working
- [ ] Export configuration endpoint working
- [ ] Proper error responses

## Documentation ✓
- [ ] Configuration keys documented
- [ ] API endpoints documented
- [ ] Validation rules documented
- [ ] Testing procedures documented

## Performance ✓
- [ ] Configuration retrieval < 50ms
- [ ] Configuration change < 100ms
- [ ] Redis operations optimized
- [ ] No memory leaks

## Edge Cases ✓
- [ ] Empty configuration handling
- [ ] Invalid value rejection
- [ ] Read-only key protection
- [ ] Unknown key handling
- [ ] Reboot-required keys identified

## OCPP Compliance ✓
- [ ] GetConfiguration compliant with OCPP 1.6
- [ ] ChangeConfiguration compliant with OCPP 1.6
- [ ] All standard keys present
- [ ] Proper status codes returned

## Ready for Chunk 1.2 ✓
- [ ] All above criteria met
- [ ] No blocking issues
- [ ] Performance acceptable
- [ ] Team approval obtained

**Approval Date**: _____________
**Approved By**: _____________
**Notes**: _____________

## Test Evidence Required
1. Screenshot of successful test_configuration.sh run
2. Screenshot of validate_config.py passing all tests
3. Redis keys showing configuration storage
4. Server logs showing message processing
5. REST API responses for all endpoints

## Sign-off Required From
- [ ] Development Team
- [ ] QA Team
- [ ] Technical Lead
```

## Execution Instructions for Agent

### Prerequisites Check
```bash
# 1. Verify current directory
pwd  # Should be /Users/chrishome/development/home/mcp-access/csms/ocpp-server

# 2. Check Redis is running
redis-cli ping  # Should return PONG

# 3. Verify server can build
go build ./...
```

### Implementation Order
1. **Create ConfigurationManager** (Task 1.1.1) - 60 minutes
2. **Add Redis methods** (Task 1.1.2) - 30 minutes
3. **Update main handlers** (Task 1.1.3) - 30 minutes
4. **Write unit tests** (Task 1.1.4) - 45 minutes
5. **Write integration tests** (Task 1.1.5) - 30 minutes
6. **Create test scripts** (Task 1.1.6) - 30 minutes
7. **Write documentation** (Task 1.1.7) - 30 minutes
8. **Validate and approve** (Task 1.1.8) - 15 minutes

**Total Time**: ~4.5 hours

### Build and Test Commands
```bash
# Build server
go build -o ocpp-server ./main.go

# Run unit tests
go test ./tests -v -run TestConfiguration

# Run integration tests
docker-compose up -d redis
go test ./tests/integration -v -run TestConfiguration

# Run external validation
./scripts/test_configuration.sh
python3 scripts/validate_config.py

# Check Redis
redis-cli HGETALL "ocpp:config:TEST-CP-001"
```

### Success Criteria
✅ All unit tests pass
✅ Integration tests pass
✅ External validation scripts succeed
✅ Configuration persisted in Redis
✅ OCPP messages handled correctly
✅ REST API endpoints functional

### Common Issues and Solutions
| Issue | Solution |
|-------|----------|
| Redis connection failed | Ensure Redis is running on port 6379 |
| Import errors | Run `go mod tidy` to fetch dependencies |
| Test timeout | Increase timeout in test files |
| Configuration not persisting | Check Redis DB number and key prefix |

**Do not proceed to Chunk 1.2 until all approval criteria are met.**
```