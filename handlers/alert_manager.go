package handlers

import (
	"log"
	"sync"
)

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