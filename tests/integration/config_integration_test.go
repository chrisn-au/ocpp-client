package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cfgmgr "ocpp-server/config"
)

// Test setup for integration tests
func setupTestEnvironment(t *testing.T) (*cfgmgr.ConfigurationManager, *redis.Client, func()) {
	// Connect to Redis (assuming it's running on localhost:6379)
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use a different DB for testing
	})

	// Test Redis connection
	ctx := context.Background()
	err := client.Ping(ctx).Err()
	require.NoError(t, err, "Redis must be running for integration tests")

	// Create a real business state implementation for testing
	businessState := &testBusinessState{client: client}

	// Create configuration manager
	configManager := cfgmgr.NewConfigurationManager(businessState)

	// Cleanup function
	cleanup := func() {
		// Clean up test data
		client.FlushDB(ctx)
		client.Close()
	}

	return configManager, client, cleanup
}

// testBusinessState implements the BusinessStateInterface for testing
type testBusinessState struct {
	client *redis.Client
}

func (t *testBusinessState) GetChargePointConfiguration(clientID string) (map[string]string, error) {
	key := "test:config:" + clientID
	ctx := context.Background()

	result, err := t.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (t *testBusinessState) SetChargePointConfiguration(clientID string, config map[string]string) error {
	key := "test:config:" + clientID
	ctx := context.Background()

	// Delete existing configuration
	if err := t.client.Del(ctx, key).Err(); err != nil {
		return err
	}

	// Set new configuration
	if len(config) > 0 {
		if err := t.client.HMSet(ctx, key, config).Err(); err != nil {
			return err
		}

		// Set expiry
		t.client.Expire(ctx, key, time.Hour)
	}

	return nil
}

func TestGetConfigurationIntegration(t *testing.T) {
	// Setup test environment
	configManager, client, cleanup := setupTestEnvironment(t)
	defer cleanup()

	clientID := "TEST-CP-001"

	// Test 1: Get all configuration (default values)
	keys, unknown := configManager.GetConfiguration(clientID, []string{})

	assert.True(t, len(keys) > 10, "Should have multiple configuration keys")
	assert.Empty(t, unknown, "Should have no unknown keys when getting all")

	// Verify some essential keys exist
	keyMap := make(map[string]core.ConfigurationKey)
	for _, key := range keys {
		keyMap[key.Key] = key
	}

	// Check HeartbeatInterval
	heartbeat, exists := keyMap["HeartbeatInterval"]
	assert.True(t, exists, "HeartbeatInterval should exist")
	assert.Equal(t, "300", *heartbeat.Value, "Default HeartbeatInterval should be 300")
	assert.False(t, heartbeat.Readonly, "HeartbeatInterval should not be readonly")

	// Check a readonly key
	maxStack, exists := keyMap["ChargeProfileMaxStackLevel"]
	assert.True(t, exists, "ChargeProfileMaxStackLevel should exist")
	assert.True(t, maxStack.Readonly, "ChargeProfileMaxStackLevel should be readonly")

	// Test 2: Get specific keys
	specificKeys, unknownKeys := configManager.GetConfiguration(clientID,
		[]string{"HeartbeatInterval", "UnknownKey", "MeterValueSampleInterval"})

	assert.Len(t, specificKeys, 2, "Should return 2 known keys")
	assert.Len(t, unknownKeys, 1, "Should return 1 unknown key")
	assert.Equal(t, "UnknownKey", unknownKeys[0], "Unknown key should be returned")

	// Test 3: Get configuration after setting custom values
	// First set a custom value
	status := configManager.ChangeConfiguration(clientID, "HeartbeatInterval", "600")
	assert.Equal(t, core.ConfigurationStatusAccepted, status)

	// Now get the configuration again
	customKeys, _ := configManager.GetConfiguration(clientID, []string{"HeartbeatInterval"})
	assert.Len(t, customKeys, 1)
	assert.Equal(t, "600", *customKeys[0].Value, "Should return custom value")

	// Verify it's persisted in Redis
	ctx := context.Background()
	value, err := client.HGet(ctx, "test:config:"+clientID, "HeartbeatInterval").Result()
	assert.NoError(t, err)
	assert.Equal(t, "600", value, "Value should be persisted in Redis")
}

func TestChangeConfigurationIntegration(t *testing.T) {
	// Setup test environment
	configManager, client, cleanup := setupTestEnvironment(t)
	defer cleanup()

	clientID := "TEST-CP-002"

	// Test 1: Change valid configuration
	status := configManager.ChangeConfiguration(clientID, "HeartbeatInterval", "600")
	assert.Equal(t, core.ConfigurationStatusAccepted, status)

	// Verify the change persisted
	keys, _ := configManager.GetConfiguration(clientID, []string{"HeartbeatInterval"})
	assert.Len(t, keys, 1)
	assert.Equal(t, "600", *keys[0].Value)

	// Test 2: Try to change read-only configuration
	status = configManager.ChangeConfiguration(clientID, "ChargeProfileMaxStackLevel", "20")
	assert.Equal(t, core.ConfigurationStatusRejected, status)

	// Test 3: Change unknown key
	status = configManager.ChangeConfiguration(clientID, "UnknownKey", "value")
	assert.Equal(t, core.ConfigurationStatusNotSupported, status)

	// Test 4: Invalid value validation
	status = configManager.ChangeConfiguration(clientID, "HeartbeatInterval", "not-a-number")
	assert.Equal(t, core.ConfigurationStatusRejected, status)

	// Test 5: Value out of range
	status = configManager.ChangeConfiguration(clientID, "LightIntensity", "150")
	assert.Equal(t, core.ConfigurationStatusRejected, status)

	// Test 6: Boolean validation
	status = configManager.ChangeConfiguration(clientID, "LocalAuthorizeOffline", "yes")
	assert.Equal(t, core.ConfigurationStatusRejected, status)

	// Valid boolean change
	status = configManager.ChangeConfiguration(clientID, "LocalAuthorizeOffline", "false")
	assert.Equal(t, core.ConfigurationStatusAccepted, status)

	// Test 7: Reboot required keys
	status = configManager.ChangeConfiguration(clientID, "WebSocketPingInterval", "120")
	assert.Equal(t, core.ConfigurationStatusRebootRequired, status)

	// Test 8: CSV validation
	status = configManager.ChangeConfiguration(clientID, "MeterValuesSampledData",
		"Energy.Active.Import.Register,Power.Active.Import")
	assert.Equal(t, core.ConfigurationStatusAccepted, status)

	// Invalid CSV value
	status = configManager.ChangeConfiguration(clientID, "MeterValuesSampledData",
		"Energy.Active.Import.Register,InvalidMeasurand")
	assert.Equal(t, core.ConfigurationStatusRejected, status)
}

func TestConfigurationPersistenceIntegration(t *testing.T) {
	// Setup test environment
	configManager, client, cleanup := setupTestEnvironment(t)
	defer cleanup()

	clientID := "TEST-CP-003"

	// Set multiple configuration values
	values := map[string]string{
		"HeartbeatInterval":      "600",
		"MeterValueSampleInterval": "30",
		"LocalAuthorizeOffline":  "false",
	}

	for key, value := range values {
		status := configManager.ChangeConfiguration(clientID, key, value)
		assert.Equal(t, core.ConfigurationStatusAccepted, status, "Failed to set %s", key)
	}

	// Create a new configuration manager instance to test persistence
	businessState := &testBusinessState{client: client}
	newConfigManager := cfgmgr.NewConfigurationManager(businessState)

	// Retrieve all configurations
	keys, _ := newConfigManager.GetConfiguration(clientID, []string{})

	// Verify values are persisted
	keyMap := make(map[string]string)
	for _, key := range keys {
		keyMap[key.Key] = *key.Value
	}

	for key, expectedValue := range values {
		actualValue, exists := keyMap[key]
		assert.True(t, exists, "Key %s should exist", key)
		assert.Equal(t, expectedValue, actualValue, "Value for %s should be persisted", key)
	}

	// Verify Redis storage directly
	ctx := context.Background()
	redisKey := "test:config:" + clientID
	for key, expectedValue := range values {
		value, err := client.HGet(ctx, redisKey, key).Result()
		assert.NoError(t, err, "Should retrieve %s from Redis", key)
		assert.Equal(t, expectedValue, value, "Redis value for %s should match", key)
	}
}

func TestExportConfigurationIntegration(t *testing.T) {
	// Setup test environment
	configManager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	clientID := "TEST-CP-004"

	// Set some custom values
	configManager.ChangeConfiguration(clientID, "HeartbeatInterval", "900")
	configManager.ChangeConfiguration(clientID, "LocalAuthorizeOffline", "false")

	// Export configuration
	exported := configManager.ExportConfiguration(clientID)

	assert.NotEmpty(t, exported, "Exported configuration should not be empty")

	// Check custom values
	heartbeat, exists := exported["HeartbeatInterval"]
	assert.True(t, exists, "HeartbeatInterval should be in export")
	heartbeatMap := heartbeat.(map[string]interface{})
	assert.Equal(t, "900", heartbeatMap["value"])
	assert.Equal(t, false, heartbeatMap["readonly"])

	// Check readonly value
	maxStack, exists := exported["ChargeProfileMaxStackLevel"]
	assert.True(t, exists, "ChargeProfileMaxStackLevel should be in export")
	maxStackMap := maxStack.(map[string]interface{})
	assert.Equal(t, true, maxStackMap["readonly"])

	// Should have all standard keys
	assert.GreaterOrEqual(t, len(exported), 20, "Should export at least 20 keys")
}

func TestGetConfigValueIntegration(t *testing.T) {
	// Setup test environment
	configManager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	clientID := "TEST-CP-005"

	// Test getting default value
	value, exists := configManager.GetConfigValue(clientID, "HeartbeatInterval")
	assert.True(t, exists)
	assert.Equal(t, "300", value)

	// Set custom value
	configManager.ChangeConfiguration(clientID, "HeartbeatInterval", "600")

	// Test getting custom value
	value, exists = configManager.GetConfigValue(clientID, "HeartbeatInterval")
	assert.True(t, exists)
	assert.Equal(t, "600", value)

	// Test getting unknown key
	value, exists = configManager.GetConfigValue(clientID, "UnknownKey")
	assert.False(t, exists)
	assert.Equal(t, "", value)
}

func TestConcurrentAccessIntegration(t *testing.T) {
	// Setup test environment
	configManager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	clientID := "TEST-CP-006"

	// Test concurrent read/write operations
	done := make(chan bool, 10)

	// Start multiple goroutines doing configuration operations
	for i := 0; i < 5; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Change configuration
			status := configManager.ChangeConfiguration(clientID, "HeartbeatInterval", "600")
			assert.Equal(t, core.ConfigurationStatusAccepted, status)

			// Read configuration
			keys, _ := configManager.GetConfiguration(clientID, []string{"HeartbeatInterval"})
			assert.Len(t, keys, 1)

			// Export configuration
			exported := configManager.ExportConfiguration(clientID)
			assert.NotEmpty(t, exported)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 5; i++ {
		select {
		case <-done:
			// OK
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent operations")
		}
	}
}

func TestConfigurationValidationRulesIntegration(t *testing.T) {
	// Setup test environment
	configManager, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	clientID := "TEST-CP-007"

	// Test all validator types
	testCases := []struct {
		key           string
		validValue    string
		invalidValue  string
		expectedValid core.ConfigurationStatus
	}{
		{
			key:           "HeartbeatInterval",
			validValue:    "600",
			invalidValue:  "not-a-number",
			expectedValid: core.ConfigurationStatusAccepted,
		},
		{
			key:           "LightIntensity",
			validValue:    "75",
			invalidValue:  "150", // Out of range
			expectedValid: core.ConfigurationStatusAccepted,
		},
		{
			key:           "LocalAuthorizeOffline",
			validValue:    "true",
			invalidValue:  "yes",
			expectedValid: core.ConfigurationStatusAccepted,
		},
		{
			key:           "MeterValuesSampledData",
			validValue:    "Energy.Active.Import.Register,Power.Active.Import",
			invalidValue:  "InvalidMeasurand",
			expectedValid: core.ConfigurationStatusAccepted,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.key, func(t *testing.T) {
			// Test valid value
			status := configManager.ChangeConfiguration(clientID, tc.key, tc.validValue)
			assert.Equal(t, tc.expectedValid, status, "Valid value should be accepted for %s", tc.key)

			// Test invalid value
			status = configManager.ChangeConfiguration(clientID, tc.key, tc.invalidValue)
			assert.Equal(t, core.ConfigurationStatusRejected, status, "Invalid value should be rejected for %s", tc.key)
		})
	}
}