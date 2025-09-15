package config

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
)

// BusinessStateInterface defines the interface for configuration persistence
type BusinessStateInterface interface {
	GetChargePointConfiguration(clientID string) (map[string]string, error)
	SetChargePointConfiguration(clientID string, config map[string]string) error
}

// ConfigurationManager manages charge point configurations
type ConfigurationManager struct {
	businessState BusinessStateInterface
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
func NewConfigurationManager(businessState BusinessStateInterface) *ConfigurationManager {
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
func (cm *ConfigurationManager) GetConfiguration(clientID string, keys []string) ([]core.ConfigurationKey, []string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var configurationKeys []core.ConfigurationKey
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

			configurationKeys = append(configurationKeys, core.ConfigurationKey{
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

				configurationKeys = append(configurationKeys, core.ConfigurationKey{
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