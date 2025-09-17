package services

import (
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/lorenzodonini/ocpp-go/transport"
)

// ChargePointService handles charge point business logic
type ChargePointService struct {
	businessState  *ocppj.RedisBusinessState
	redisTransport transport.Transport
}

// NewChargePointService creates a new charge point service
func NewChargePointService(businessState *ocppj.RedisBusinessState, redisTransport transport.Transport) *ChargePointService {
	return &ChargePointService{
		businessState:  businessState,
		redisTransport: redisTransport,
	}
}

// GetAllChargePoints retrieves all charge points
func (s *ChargePointService) GetAllChargePoints() ([]interface{}, error) {
	chargePoints, err := s.businessState.GetAllChargePoints()
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, len(chargePoints))
	for i, cp := range chargePoints {
		result[i] = cp
	}
	return result, nil
}

// GetChargePoint retrieves a specific charge point
func (s *ChargePointService) GetChargePoint(clientID string) (interface{}, error) {
	return s.businessState.GetChargePointInfo(clientID)
}

// GetAllConnectors retrieves all connectors for a charge point
func (s *ChargePointService) GetAllConnectors(clientID string) ([]interface{}, error) {
	connectors, err := s.businessState.GetAllConnectors(clientID)
	if err != nil {
		return nil, err
	}

	result := make([]interface{}, 0, len(connectors))
	for _, connector := range connectors {
		result = append(result, connector)
	}
	return result, nil
}

// GetConnector retrieves a specific connector
func (s *ChargePointService) GetConnector(clientID string, connectorID int) (interface{}, error) {
	return s.businessState.GetConnectorStatus(clientID, connectorID)
}

// IsOnline checks if a charge point is currently connected
func (s *ChargePointService) IsOnline(clientID string) bool {
	connectedClients := s.redisTransport.GetConnectedClients()
	for _, client := range connectedClients {
		if client == clientID {
			return true
		}
	}
	return false
}

// GetConnectedClients returns all connected clients
func (s *ChargePointService) GetConnectedClients() []string {
	return s.redisTransport.GetConnectedClients()
}