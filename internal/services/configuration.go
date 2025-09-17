package services

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/core"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/lorenzodonini/ocpp-go/transport"

	cfgmgr "ocpp-server/config"
	"ocpp-server/internal/correlation"
	"ocpp-server/internal/types"
)

const (
	liveConfigTimeout = 10 * time.Second
)

// ConfigurationService handles configuration business logic
type ConfigurationService struct {
	configManager      *cfgmgr.ConfigurationManager
	redisTransport     transport.Transport
	ocppServer         *ocppj.Server
	correlationManager *correlation.Manager
}

// NewConfigurationService creates a new configuration service
func NewConfigurationService(
	configManager *cfgmgr.ConfigurationManager,
	redisTransport transport.Transport,
	ocppServer *ocppj.Server,
	correlationManager *correlation.Manager,
) *ConfigurationService {
	return &ConfigurationService{
		configManager:      configManager,
		redisTransport:     redisTransport,
		ocppServer:         ocppServer,
		correlationManager: correlationManager,
	}
}

// GetStoredConfiguration retrieves stored configuration for a charge point
func (s *ConfigurationService) GetStoredConfiguration(clientID string, keys []string) (map[string]interface{}, []string) {
	configurationKeys, unknownKeys := s.configManager.GetConfiguration(clientID, keys)

	configData := make(map[string]interface{})
	for _, kv := range configurationKeys {
		configData[kv.Key] = map[string]interface{}{
			"value":    *kv.Value,
			"readonly": kv.Readonly,
		}
	}

	return configData, unknownKeys
}

// ChangeStoredConfiguration changes stored configuration
func (s *ConfigurationService) ChangeStoredConfiguration(clientID, key, value string) string {
	status := s.configManager.ChangeConfiguration(clientID, key, value)
	return string(status)
}

// ExportConfiguration exports all configuration for a charge point
func (s *ConfigurationService) ExportConfiguration(clientID string) interface{} {
	return s.configManager.ExportConfiguration(clientID)
}

// IsChargerOnline checks if a charger is online
func (s *ConfigurationService) IsChargerOnline(clientID string) bool {
	connectedClients := s.redisTransport.GetConnectedClients()
	for _, client := range connectedClients {
		if client == clientID {
			return true
		}
	}
	return false
}

// GetLiveConfiguration retrieves live configuration from charge point
func (s *ConfigurationService) GetLiveConfiguration(clientID string, keysParam string) (chan types.LiveConfigResponse, error) {
	var keys []string
	if keysParam != "" {
		keys = strings.Split(keysParam, ",")
		for i, key := range keys {
			keys[i] = strings.TrimSpace(key)
		}
	}

	return s.sendGetConfigurationToCharger(clientID, keys)
}

// ChangeLiveConfiguration changes live configuration on charge point
func (s *ConfigurationService) ChangeLiveConfiguration(clientID, key, value string) error {
	request := core.NewChangeConfigurationRequest(key, value)
	err := s.ocppServer.SendRequest(clientID, request)
	if err != nil {
		log.Printf("Error sending ChangeConfiguration to charger %s: %v", clientID, err)
		return err
	}
	return nil
}

// sendGetConfigurationToCharger sends a GetConfiguration request to a live charger
func (s *ConfigurationService) sendGetConfigurationToCharger(clientID string, keys []string) (chan types.LiveConfigResponse, error) {
	request := core.NewGetConfigurationRequest(keys)
	log.Printf("SEND_REQUEST: Sending GetConfiguration to %s with keys: %v", clientID, keys)

	// Use a temporary correlation key for now - we'll update it after sending
	tempKey := fmt.Sprintf("%s:GetConfiguration:temp", clientID)
	responseChan := s.correlationManager.AddPendingRequest(tempKey, clientID, "GetConfiguration")

	err := s.ocppServer.SendRequest(clientID, request)
	if err != nil {
		log.Printf("SEND_REQUEST: Error sending to %s: %v", clientID, err)
		// Clean up pending request on error
		s.correlationManager.CleanupPendingRequest(tempKey)
		return nil, err
	}

	log.Printf("SEND_REQUEST: Successfully sent GetConfiguration to %s", clientID)
	return responseChan, nil
}