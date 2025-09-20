package services

import (
	"errors"
	"testing"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/remotetrigger"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"ocpp-server/internal/correlation"
	"ocpp-server/internal/types"
)

// MockOCPPServer mocks the OCPP server for testing
type MockOCPPServer struct {
	mock.Mock
}

func (m *MockOCPPServer) SendRequest(clientID string, request interface{}) error {
	args := m.Called(clientID, request)
	return args.Error(0)
}

func (m *MockOCPPServer) Start(listenPort int) error {
	args := m.Called(listenPort)
	return args.Error(0)
}

func (m *MockOCPPServer) Stop() {
	m.Called()
}

func (m *MockOCPPServer) SetNewChargePointHandler(handler func(chargePointId string)) {
	m.Called(handler)
}

func (m *MockOCPPServer) SetChargePointDisconnectedHandler(handler func(chargePointId string)) {
	m.Called(handler)
}

// MockChargePointService mocks the charge point service for testing
type MockChargePointService struct {
	mock.Mock
}

func (m *MockChargePointService) IsOnline(clientID string) bool {
	args := m.Called(clientID)
	return args.Bool(0)
}

func (m *MockChargePointService) GetAllChargePoints() ([]interface{}, error) {
	args := m.Called()
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockChargePointService) GetChargePoint(clientID string) (interface{}, error) {
	args := m.Called(clientID)
	return args.Get(0), args.Error(1)
}

func (m *MockChargePointService) GetAllConnectors(clientID string) ([]interface{}, error) {
	args := m.Called(clientID)
	return args.Get(0).([]interface{}), args.Error(1)
}

func (m *MockChargePointService) GetConnector(clientID string, connectorID int) (interface{}, error) {
	args := m.Called(clientID, connectorID)
	return args.Get(0), args.Error(1)
}

func (m *MockChargePointService) GetConnectedClients() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// MockCorrelationManager mocks the correlation manager for testing
type MockCorrelationManager struct {
	mock.Mock
}

func (m *MockCorrelationManager) AddPendingRequest(requestID, clientID, requestType string) chan types.LiveConfigResponse {
	args := m.Called(requestID, clientID, requestType)
	return args.Get(0).(chan types.LiveConfigResponse)
}

func (m *MockCorrelationManager) SendLiveResponse(correlationKey string, response types.LiveConfigResponse) {
	m.Called(correlationKey, response)
}

func (m *MockCorrelationManager) CleanupPendingRequest(requestID string) {
	m.Called(requestID)
}

func (m *MockCorrelationManager) DeletePendingRequest(requestID string) {
	m.Called(requestID)
}

func (m *MockCorrelationManager) CleanupExpiredRequests() {
	m.Called()
}

func (m *MockCorrelationManager) FindPendingRequest(clientID, requestType string) (string, *correlation.PendingRequest) {
	args := m.Called(clientID, requestType)
	if args.Get(1) == nil {
		return args.String(0), nil
	}
	return args.String(0), args.Get(1).(*correlation.PendingRequest)
}

func (m *MockCorrelationManager) SendPendingResponse(clientID, requestType string, response types.LiveConfigResponse) {
	m.Called(clientID, requestType, response)
}

func (m *MockCorrelationManager) AddPendingRequestForHandlers(requestID, clientID, requestType string) chan types.LiveConfigResponse {
	args := m.Called(requestID, clientID, requestType)
	return args.Get(0).(chan types.LiveConfigResponse)
}

func (m *MockCorrelationManager) SendPendingResponseFromHandlers(clientID, requestType string, response types.LiveConfigResponse) {
	m.Called(clientID, requestType, response)
}

// TestTriggerMessageService_SendTriggerMessage_Success tests successful trigger message sending
func TestTriggerMessageService_SendTriggerMessage_Success(t *testing.T) {
	// Setup mocks
	mockOCPPServer := new(MockOCPPServer)
	mockChargePointService := new(MockChargePointService)
	mockCorrelationManager := new(MockCorrelationManager)

	// Create service
	service := NewTriggerMessageService(mockOCPPServer, mockChargePointService, mockCorrelationManager)

	// Test data
	clientID := "test-cp-001"
	requestedMessage := "StatusNotification"
	connectorID := 1

	// Setup expectations
	mockChargePointService.On("IsOnline", clientID).Return(true)
	responseChan := make(chan types.LiveConfigResponse, 1)
	mockCorrelationManager.On("AddPendingRequest", mock.AnythingOfType("string"), clientID, "TriggerMessage").Return(responseChan)
	mockOCPPServer.On("SendRequest", clientID, mock.AnythingOfType("*remotetrigger.TriggerMessageRequest")).Return(nil)

	// Execute
	resultChan, result, err := service.SendTriggerMessage(clientID, requestedMessage, &connectorID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resultChan)
	assert.NotNil(t, result)
	assert.Equal(t, clientID, result.ClientID)
	assert.Equal(t, requestedMessage, result.RequestedMessage)
	assert.NotEmpty(t, result.RequestID)

	// Verify mocks
	mockChargePointService.AssertExpectations(t)
	mockCorrelationManager.AssertExpectations(t)
	mockOCPPServer.AssertExpectations(t)
}

// TestTriggerMessageService_SendTriggerMessage_OfflineChargePoint tests offline charge point handling
func TestTriggerMessageService_SendTriggerMessage_OfflineChargePoint(t *testing.T) {
	// Setup mocks
	mockOCPPServer := new(MockOCPPServer)
	mockChargePointService := new(MockChargePointService)
	mockCorrelationManager := new(MockCorrelationManager)

	// Create service
	service := NewTriggerMessageService(mockOCPPServer, mockChargePointService, mockCorrelationManager)

	// Test data
	clientID := "offline-cp-001"
	requestedMessage := "StatusNotification"

	// Setup expectations
	mockChargePointService.On("IsOnline", clientID).Return(false)

	// Execute
	resultChan, result, err := service.SendTriggerMessage(clientID, requestedMessage, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resultChan)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "client not connected")

	// Verify mocks
	mockChargePointService.AssertExpectations(t)
	mockCorrelationManager.AssertNotCalled(t, "AddPendingRequest")
	mockOCPPServer.AssertNotCalled(t, "SendRequest")
}

// TestTriggerMessageService_SendTriggerMessage_InvalidMessageType tests invalid message type validation
func TestTriggerMessageService_SendTriggerMessage_InvalidMessageType(t *testing.T) {
	// Setup mocks
	mockOCPPServer := new(MockOCPPServer)
	mockChargePointService := new(MockChargePointService)
	mockCorrelationManager := new(MockCorrelationManager)

	// Create service
	service := NewTriggerMessageService(mockOCPPServer, mockChargePointService, mockCorrelationManager)

	// Test data
	clientID := "test-cp-001"
	invalidMessageType := "InvalidMessageType"

	// Setup expectations
	mockChargePointService.On("IsOnline", clientID).Return(true)

	// Execute
	resultChan, result, err := service.SendTriggerMessage(clientID, invalidMessageType, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resultChan)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "unsupported message type")

	// Verify mocks
	mockChargePointService.AssertExpectations(t)
	mockCorrelationManager.AssertNotCalled(t, "AddPendingRequest")
	mockOCPPServer.AssertNotCalled(t, "SendRequest")
}

// TestTriggerMessageService_SendTriggerMessage_SendRequestError tests OCPP send request error
func TestTriggerMessageService_SendTriggerMessage_SendRequestError(t *testing.T) {
	// Setup mocks
	mockOCPPServer := new(MockOCPPServer)
	mockChargePointService := new(MockChargePointService)
	mockCorrelationManager := new(MockCorrelationManager)

	// Create service
	service := NewTriggerMessageService(mockOCPPServer, mockChargePointService, mockCorrelationManager)

	// Test data
	clientID := "test-cp-001"
	requestedMessage := "StatusNotification"

	// Setup expectations
	mockChargePointService.On("IsOnline", clientID).Return(true)
	responseChan := make(chan types.LiveConfigResponse, 1)
	mockCorrelationManager.On("AddPendingRequest", mock.AnythingOfType("string"), clientID, "TriggerMessage").Return(responseChan)
	mockOCPPServer.On("SendRequest", clientID, mock.AnythingOfType("*remotetrigger.TriggerMessageRequest")).Return(errors.New("network error"))
	mockCorrelationManager.On("CleanupPendingRequest", mock.AnythingOfType("string")).Return()

	// Execute
	resultChan, result, err := service.SendTriggerMessage(clientID, requestedMessage, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, resultChan)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to send request to charge point")

	// Verify mocks
	mockChargePointService.AssertExpectations(t)
	mockCorrelationManager.AssertExpectations(t)
	mockOCPPServer.AssertExpectations(t)
}

// TestTriggerMessageService_SendTriggerMessage_ValidMessageTypes tests all valid message types
func TestTriggerMessageService_SendTriggerMessage_ValidMessageTypes(t *testing.T) {
	validMessageTypes := []string{
		"StatusNotification",
		"Heartbeat",
		"MeterValues",
		"BootNotification",
	}

	for _, messageType := range validMessageTypes {
		t.Run(messageType, func(t *testing.T) {
			// Setup mocks
			mockOCPPServer := new(MockOCPPServer)
			mockChargePointService := new(MockChargePointService)
			mockCorrelationManager := new(MockCorrelationManager)

			// Create service
			service := NewTriggerMessageService(mockOCPPServer, mockChargePointService, mockCorrelationManager)

			// Test data
			clientID := "test-cp-001"

			// Setup expectations
			mockChargePointService.On("IsOnline", clientID).Return(true)
			responseChan := make(chan types.LiveConfigResponse, 1)
			mockCorrelationManager.On("AddPendingRequest", mock.AnythingOfType("string"), clientID, "TriggerMessage").Return(responseChan)
			mockOCPPServer.On("SendRequest", clientID, mock.AnythingOfType("*remotetrigger.TriggerMessageRequest")).Return(nil)

			// Execute
			resultChan, result, err := service.SendTriggerMessage(clientID, messageType, nil)

			// Assert
			assert.NoError(t, err)
			assert.NotNil(t, resultChan)
			assert.NotNil(t, result)
			assert.Equal(t, messageType, result.RequestedMessage)

			// Verify mocks
			mockChargePointService.AssertExpectations(t)
			mockCorrelationManager.AssertExpectations(t)
			mockOCPPServer.AssertExpectations(t)
		})
	}
}

// TestTriggerMessageService_SendTriggerMessage_WithConnectorID tests trigger with specific connector ID
func TestTriggerMessageService_SendTriggerMessage_WithConnectorID(t *testing.T) {
	// Setup mocks
	mockOCPPServer := new(MockOCPPServer)
	mockChargePointService := new(MockChargePointService)
	mockCorrelationManager := new(MockCorrelationManager)

	// Create service
	service := NewTriggerMessageService(mockOCPPServer, mockChargePointService, mockCorrelationManager)

	// Test data
	clientID := "test-cp-001"
	requestedMessage := "StatusNotification"
	connectorID := 2

	// Setup expectations
	mockChargePointService.On("IsOnline", clientID).Return(true)
	responseChan := make(chan types.LiveConfigResponse, 1)
	mockCorrelationManager.On("AddPendingRequest", mock.AnythingOfType("string"), clientID, "TriggerMessage").Return(responseChan)

	// Verify that the correct connector ID is passed to the OCPP request
	mockOCPPServer.On("SendRequest", clientID, mock.MatchedBy(func(req *remotetrigger.TriggerMessageRequest) bool {
		return req.ConnectorId != nil && *req.ConnectorId == connectorID
	})).Return(nil)

	// Execute
	resultChan, result, err := service.SendTriggerMessage(clientID, requestedMessage, &connectorID)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resultChan)
	assert.NotNil(t, result)
	assert.Equal(t, &connectorID, result.ConnectorID)

	// Verify mocks
	mockChargePointService.AssertExpectations(t)
	mockCorrelationManager.AssertExpectations(t)
	mockOCPPServer.AssertExpectations(t)
}

// TestTriggerMessageService_SendTriggerMessage_CorrelationKeyGeneration tests correlation key generation
func TestTriggerMessageService_SendTriggerMessage_CorrelationKeyGeneration(t *testing.T) {
	// Setup mocks
	mockOCPPServer := new(MockOCPPServer)
	mockChargePointService := new(MockChargePointService)
	mockCorrelationManager := new(MockCorrelationManager)

	// Create service
	service := NewTriggerMessageService(mockOCPPServer, mockChargePointService, mockCorrelationManager)

	// Test data
	clientID := "test-cp-001"
	requestedMessage := "StatusNotification"

	// Setup expectations
	mockChargePointService.On("IsOnline", clientID).Return(true)
	responseChan := make(chan types.LiveConfigResponse, 1)

	// Capture the correlation key format
	var capturedCorrelationKey string
	mockCorrelationManager.On("AddPendingRequest", mock.AnythingOfType("string"), clientID, "TriggerMessage").Run(func(args mock.Arguments) {
		capturedCorrelationKey = args.String(0)
	}).Return(responseChan)

	mockOCPPServer.On("SendRequest", clientID, mock.AnythingOfType("*remotetrigger.TriggerMessageRequest")).Return(nil)

	// Execute
	resultChan, result, err := service.SendTriggerMessage(clientID, requestedMessage, nil)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, resultChan)
	assert.NotNil(t, result)

	// Verify correlation key format: clientID:TriggerMessage:requestID
	assert.Contains(t, capturedCorrelationKey, clientID)
	assert.Contains(t, capturedCorrelationKey, "TriggerMessage")
	assert.Contains(t, capturedCorrelationKey, result.RequestID)

	// Verify mocks
	mockChargePointService.AssertExpectations(t)
	mockCorrelationManager.AssertExpectations(t)
	mockOCPPServer.AssertExpectations(t)
}

// TestTriggerMessageService_GetTimeout tests timeout configuration
func TestTriggerMessageService_GetTimeout(t *testing.T) {
	// Setup mocks
	mockOCPPServer := new(MockOCPPServer)
	mockChargePointService := new(MockChargePointService)
	mockCorrelationManager := new(MockCorrelationManager)

	// Create service
	service := NewTriggerMessageService(mockOCPPServer, mockChargePointService, mockCorrelationManager)

	// Execute
	timeout := service.GetTimeout()

	// Assert
	assert.Equal(t, 10*time.Second, timeout)
}

// TestTriggerMessageService_SendTriggerMessage_ConcurrentRequests tests concurrent trigger requests
func TestTriggerMessageService_SendTriggerMessage_ConcurrentRequests(t *testing.T) {
	// Setup mocks
	mockOCPPServer := new(MockOCPPServer)
	mockChargePointService := new(MockChargePointService)
	mockCorrelationManager := new(MockCorrelationManager)

	// Create service
	service := NewTriggerMessageService(mockOCPPServer, mockChargePointService, mockCorrelationManager)

	// Test data
	clientID := "test-cp-001"
	requestedMessage := "StatusNotification"
	concurrentRequests := 5

	// Setup expectations for concurrent requests
	mockChargePointService.On("IsOnline", clientID).Return(true).Times(concurrentRequests)
	for i := 0; i < concurrentRequests; i++ {
		responseChan := make(chan types.LiveConfigResponse, 1)
		mockCorrelationManager.On("AddPendingRequest", mock.AnythingOfType("string"), clientID, "TriggerMessage").Return(responseChan).Once()
		mockOCPPServer.On("SendRequest", clientID, mock.AnythingOfType("*remotetrigger.TriggerMessageRequest")).Return(nil).Once()
	}

	// Execute concurrent requests
	results := make([]*TriggerMessageResult, concurrentRequests)
	errors := make([]error, concurrentRequests)

	done := make(chan bool, concurrentRequests)
	for i := 0; i < concurrentRequests; i++ {
		go func(index int) {
			_, results[index], errors[index] = service.SendTriggerMessage(clientID, requestedMessage, nil)
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < concurrentRequests; i++ {
		<-done
	}

	// Assert all requests succeeded
	for i := 0; i < concurrentRequests; i++ {
		assert.NoError(t, errors[i])
		assert.NotNil(t, results[i])
		assert.NotEmpty(t, results[i].RequestID)
	}

	// Verify unique request IDs
	requestIDs := make(map[string]bool)
	for _, result := range results {
		assert.False(t, requestIDs[result.RequestID], "Request ID should be unique")
		requestIDs[result.RequestID] = true
	}

	// Verify mocks
	mockChargePointService.AssertExpectations(t)
	mockCorrelationManager.AssertExpectations(t)
	mockOCPPServer.AssertExpectations(t)
}

// TestTriggerMessageService_ValidateRequestedMessage tests message type validation
func TestTriggerMessageService_ValidateRequestedMessage(t *testing.T) {
	tests := []struct {
		name          string
		messageType   string
		expectedValid bool
	}{
		{"StatusNotification", "StatusNotification", true},
		{"Heartbeat", "Heartbeat", true},
		{"MeterValues", "MeterValues", true},
		{"BootNotification", "BootNotification", true},
		{"Invalid", "InvalidMessage", false},
		{"EmptyString", "", false},
		{"CaseSensitive", "statusnotification", false},
	}

	// Setup mocks
	mockOCPPServer := new(MockOCPPServer)
	mockChargePointService := new(MockChargePointService)
	mockCorrelationManager := new(MockCorrelationManager)

	// Create service
	service := NewTriggerMessageService(mockOCPPServer, mockChargePointService, mockCorrelationManager)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := service.ValidateRequestedMessage(tt.messageType)
			assert.Equal(t, tt.expectedValid, isValid)
		})
	}
}