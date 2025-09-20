package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"ocpp-server/internal/api/v1/models"
	"ocpp-server/internal/services"
	"ocpp-server/internal/types"
)

// MockTriggerMessageService mocks the trigger message service for testing
type MockTriggerMessageService struct {
	mock.Mock
}

func (m *MockTriggerMessageService) SendTriggerMessage(clientID string, requestedMessage string, connectorID *int) (chan types.LiveConfigResponse, *services.TriggerMessageResult, error) {
	args := m.Called(clientID, requestedMessage, connectorID)
	if args.Get(0) == nil {
		return nil, nil, args.Error(2)
	}
	return args.Get(0).(chan types.LiveConfigResponse), args.Get(1).(*services.TriggerMessageResult), args.Error(2)
}

func (m *MockTriggerMessageService) GetTimeout() time.Duration {
	args := m.Called()
	return args.Get(0).(time.Duration)
}

func (m *MockTriggerMessageService) ValidateRequestedMessage(messageType string) bool {
	args := m.Called(messageType)
	return args.Bool(0)
}

// setupTestRequest creates an HTTP request for testing
func setupTestRequest(method, url string, body interface{}) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, url, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// setupMuxRequest sets up a request with mux variables
func setupMuxRequest(method, url string, body interface{}, clientID string) *http.Request {
	req := setupTestRequest(method, url, body)
	// Set mux vars
	req = mux.SetURLVars(req, map[string]string{"clientID": clientID})
	return req
}

// TestTriggerMessageHandler_Success tests successful trigger message request
func TestTriggerMessageHandler_Success(t *testing.T) {
	// Setup mock service
	mockService := new(MockTriggerMessageService)

	// Test data
	clientID := "test-cp-001"
	requestBody := models.TriggerMessageRequest{
		RequestedMessage: "StatusNotification",
		ConnectorID:      nil,
	}

	// Create response channel with successful response
	responseChan := make(chan types.LiveConfigResponse, 1)
	responseChan <- types.LiveConfigResponse{
		Success: true,
		Data:    map[string]interface{}{"status": "Accepted"},
	}

	result := &services.TriggerMessageResult{
		RequestID:        "req-12345",
		ClientID:         clientID,
		RequestedMessage: "StatusNotification",
		ConnectorID:      nil,
	}

	// Setup expectations
	mockService.On("SendTriggerMessage", clientID, "StatusNotification", (*int)(nil)).Return(responseChan, result, nil)
	mockService.On("GetTimeout").Return(10 * time.Second)

	// Create handler
	handler := TriggerMessageHandler(mockService)

	// Create request
	req := setupMuxRequest("POST", "/api/v1/chargepoints/test-cp-001/trigger", requestBody, clientID)
	rr := httptest.NewRecorder()

	// Execute
	handler(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var response models.APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
	assert.Contains(t, response.Message, "successfully")

	// Check response data
	data, ok := response.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "req-12345", data["requestId"])
	assert.Equal(t, clientID, data["clientId"])
	assert.Equal(t, "StatusNotification", data["requestedMessage"])
	assert.Equal(t, "Accepted", data["status"])

	mockService.AssertExpectations(t)
}

// TestTriggerMessageHandler_MissingClientID tests missing client ID in URL
func TestTriggerMessageHandler_MissingClientID(t *testing.T) {
	mockService := new(MockTriggerMessageService)
	handler := TriggerMessageHandler(mockService)

	requestBody := models.TriggerMessageRequest{
		RequestedMessage: "StatusNotification",
	}

	// Create request without client ID in URL vars
	req := setupTestRequest("POST", "/api/v1/chargepoints//trigger", requestBody)
	rr := httptest.NewRecorder()

	// Execute
	handler(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response models.APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "Client ID is required")

	mockService.AssertNotCalled(t, "SendTriggerMessage")
}

// TestTriggerMessageHandler_InvalidRequestBody tests invalid JSON body
func TestTriggerMessageHandler_InvalidRequestBody(t *testing.T) {
	mockService := new(MockTriggerMessageService)
	handler := TriggerMessageHandler(mockService)

	clientID := "test-cp-001"

	// Create request with invalid JSON
	req := httptest.NewRequest("POST", "/api/v1/chargepoints/test-cp-001/trigger", strings.NewReader("invalid json"))
	req = mux.SetURLVars(req, map[string]string{"clientID": clientID})
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	// Execute
	handler(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response models.APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "Invalid request body")

	mockService.AssertNotCalled(t, "SendTriggerMessage")
}

// TestTriggerMessageHandler_MissingRequestedMessage tests missing requestedMessage field
func TestTriggerMessageHandler_MissingRequestedMessage(t *testing.T) {
	mockService := new(MockTriggerMessageService)
	handler := TriggerMessageHandler(mockService)

	clientID := "test-cp-001"
	requestBody := models.TriggerMessageRequest{
		// Missing RequestedMessage
		ConnectorID: nil,
	}

	req := setupMuxRequest("POST", "/api/v1/chargepoints/test-cp-001/trigger", requestBody, clientID)
	rr := httptest.NewRecorder()

	// Execute
	handler(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response models.APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "requestedMessage is required")

	mockService.AssertNotCalled(t, "SendTriggerMessage")
}

// TestTriggerMessageHandler_UnsupportedMessageType tests unsupported message type
func TestTriggerMessageHandler_UnsupportedMessageType(t *testing.T) {
	mockService := new(MockTriggerMessageService)
	handler := TriggerMessageHandler(mockService)

	clientID := "test-cp-001"
	requestBody := models.TriggerMessageRequest{
		RequestedMessage: "UnsupportedMessage",
		ConnectorID:      nil,
	}

	req := setupMuxRequest("POST", "/api/v1/chargepoints/test-cp-001/trigger", requestBody, clientID)
	rr := httptest.NewRecorder()

	// Execute
	handler(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response models.APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "Unsupported message type")

	mockService.AssertNotCalled(t, "SendTriggerMessage")
}

// TestTriggerMessageHandler_InvalidConnectorID tests invalid connector ID
func TestTriggerMessageHandler_InvalidConnectorID(t *testing.T) {
	mockService := new(MockTriggerMessageService)
	handler := TriggerMessageHandler(mockService)

	clientID := "test-cp-001"
	invalidConnectorID := -1
	requestBody := models.TriggerMessageRequest{
		RequestedMessage: "StatusNotification",
		ConnectorID:      &invalidConnectorID,
	}

	req := setupMuxRequest("POST", "/api/v1/chargepoints/test-cp-001/trigger", requestBody, clientID)
	rr := httptest.NewRecorder()

	// Execute
	handler(rr, req)

	// Assert
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var response models.APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "connectorId must be >= 0")

	mockService.AssertNotCalled(t, "SendTriggerMessage")
}

// TestTriggerMessageHandler_OfflineChargePoint tests offline charge point scenario
func TestTriggerMessageHandler_OfflineChargePoint(t *testing.T) {
	mockService := new(MockTriggerMessageService)
	handler := TriggerMessageHandler(mockService)

	clientID := "offline-cp-001"
	requestBody := models.TriggerMessageRequest{
		RequestedMessage: "StatusNotification",
		ConnectorID:      nil,
	}

	// Setup expectations - service returns "client not connected" error
	mockService.On("SendTriggerMessage", clientID, "StatusNotification", (*int)(nil)).Return(nil, nil, fmt.Errorf("client not connected"))

	req := setupMuxRequest("POST", "/api/v1/chargepoints/offline-cp-001/trigger", requestBody, clientID)
	rr := httptest.NewRecorder()

	// Execute
	handler(rr, req)

	// Assert
	assert.Equal(t, http.StatusNotFound, rr.Code)

	var response models.APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "client not connected")

	mockService.AssertExpectations(t)
}

// TestTriggerMessageHandler_ServiceError tests general service error
func TestTriggerMessageHandler_ServiceError(t *testing.T) {
	mockService := new(MockTriggerMessageService)
	handler := TriggerMessageHandler(mockService)

	clientID := "test-cp-001"
	requestBody := models.TriggerMessageRequest{
		RequestedMessage: "StatusNotification",
		ConnectorID:      nil,
	}

	// Setup expectations - service returns general error
	mockService.On("SendTriggerMessage", clientID, "StatusNotification", (*int)(nil)).Return(nil, nil, fmt.Errorf("network error"))

	req := setupMuxRequest("POST", "/api/v1/chargepoints/test-cp-001/trigger", requestBody, clientID)
	rr := httptest.NewRecorder()

	// Execute
	handler(rr, req)

	// Assert
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)

	var response models.APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "network error")

	mockService.AssertExpectations(t)
}

// TestTriggerMessageHandler_RejectedByChargePoint tests charge point rejection
func TestTriggerMessageHandler_RejectedByChargePoint(t *testing.T) {
	mockService := new(MockTriggerMessageService)
	handler := TriggerMessageHandler(mockService)

	clientID := "test-cp-001"
	requestBody := models.TriggerMessageRequest{
		RequestedMessage: "StatusNotification",
		ConnectorID:      nil,
	}

	// Create response channel with rejection response
	responseChan := make(chan types.LiveConfigResponse, 1)
	responseChan <- types.LiveConfigResponse{
		Success: false,
		Error:   "TriggerMessage rejected",
	}

	result := &services.TriggerMessageResult{
		RequestID:        "req-12345",
		ClientID:         clientID,
		RequestedMessage: "StatusNotification",
		ConnectorID:      nil,
	}

	// Setup expectations
	mockService.On("SendTriggerMessage", clientID, "StatusNotification", (*int)(nil)).Return(responseChan, result, nil)
	mockService.On("GetTimeout").Return(10 * time.Second)

	req := setupMuxRequest("POST", "/api/v1/chargepoints/test-cp-001/trigger", requestBody, clientID)
	rr := httptest.NewRecorder()

	// Execute
	handler(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var response models.APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "rejected")

	// Check response data
	data, ok := response.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Rejected", data["status"])

	mockService.AssertExpectations(t)
}

// TestTriggerMessageHandler_Timeout tests timeout scenario
func TestTriggerMessageHandler_Timeout(t *testing.T) {
	mockService := new(MockTriggerMessageService)
	handler := TriggerMessageHandler(mockService)

	clientID := "test-cp-001"
	requestBody := models.TriggerMessageRequest{
		RequestedMessage: "StatusNotification",
		ConnectorID:      nil,
	}

	// Create response channel but don't send response (simulates timeout)
	responseChan := make(chan types.LiveConfigResponse, 1)

	result := &services.TriggerMessageResult{
		RequestID:        "req-12345",
		ClientID:         clientID,
		RequestedMessage: "StatusNotification",
		ConnectorID:      nil,
	}

	// Setup expectations with very short timeout
	mockService.On("SendTriggerMessage", clientID, "StatusNotification", (*int)(nil)).Return(responseChan, result, nil)
	mockService.On("GetTimeout").Return(1 * time.Millisecond) // Very short timeout for testing

	req := setupMuxRequest("POST", "/api/v1/chargepoints/test-cp-001/trigger", requestBody, clientID)
	rr := httptest.NewRecorder()

	// Execute
	handler(rr, req)

	// Assert
	assert.Equal(t, http.StatusRequestTimeout, rr.Code)

	var response models.APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "Timeout")

	// Check response data
	data, ok := response.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "Timeout", data["status"])

	mockService.AssertExpectations(t)
}

// TestTriggerMessageHandler_WithConnectorID tests trigger with specific connector ID
func TestTriggerMessageHandler_WithConnectorID(t *testing.T) {
	mockService := new(MockTriggerMessageService)
	handler := TriggerMessageHandler(mockService)

	clientID := "test-cp-001"
	connectorID := 2
	requestBody := models.TriggerMessageRequest{
		RequestedMessage: "StatusNotification",
		ConnectorID:      &connectorID,
	}

	// Create response channel with successful response
	responseChan := make(chan types.LiveConfigResponse, 1)
	responseChan <- types.LiveConfigResponse{
		Success: true,
		Data:    map[string]interface{}{"status": "Accepted"},
	}

	result := &services.TriggerMessageResult{
		RequestID:        "req-12345",
		ClientID:         clientID,
		RequestedMessage: "StatusNotification",
		ConnectorID:      &connectorID,
	}

	// Setup expectations
	mockService.On("SendTriggerMessage", clientID, "StatusNotification", &connectorID).Return(responseChan, result, nil)
	mockService.On("GetTimeout").Return(10 * time.Second)

	req := setupMuxRequest("POST", "/api/v1/chargepoints/test-cp-001/trigger", requestBody, clientID)
	rr := httptest.NewRecorder()

	// Execute
	handler(rr, req)

	// Assert
	assert.Equal(t, http.StatusOK, rr.Code)

	var response models.APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)

	// Check response data includes connector ID
	data, ok := response.Data.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, float64(connectorID), data["connectorId"]) // JSON numbers are float64

	mockService.AssertExpectations(t)
}

// TestTriggerMessageHandler_ValidMessageTypes tests all valid message types
func TestTriggerMessageHandler_ValidMessageTypes(t *testing.T) {
	validMessageTypes := []string{
		"StatusNotification",
		"Heartbeat",
		"MeterValues",
		"BootNotification",
	}

	for _, messageType := range validMessageTypes {
		t.Run(messageType, func(t *testing.T) {
			mockService := new(MockTriggerMessageService)
			handler := TriggerMessageHandler(mockService)

			clientID := "test-cp-001"
			requestBody := models.TriggerMessageRequest{
				RequestedMessage: messageType,
				ConnectorID:      nil,
			}

			// Create response channel with successful response
			responseChan := make(chan types.LiveConfigResponse, 1)
			responseChan <- types.LiveConfigResponse{
				Success: true,
				Data:    map[string]interface{}{"status": "Accepted"},
			}

			result := &services.TriggerMessageResult{
				RequestID:        "req-12345",
				ClientID:         clientID,
				RequestedMessage: messageType,
				ConnectorID:      nil,
			}

			// Setup expectations
			mockService.On("SendTriggerMessage", clientID, messageType, (*int)(nil)).Return(responseChan, result, nil)
			mockService.On("GetTimeout").Return(10 * time.Second)

			req := setupMuxRequest("POST", "/api/v1/chargepoints/test-cp-001/trigger", requestBody, clientID)
			rr := httptest.NewRecorder()

			// Execute
			handler(rr, req)

			// Assert
			assert.Equal(t, http.StatusOK, rr.Code)

			var response models.APIResponse
			err := json.Unmarshal(rr.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.True(t, response.Success)

			// Check response data
			data, ok := response.Data.(map[string]interface{})
			assert.True(t, ok)
			assert.Equal(t, messageType, data["requestedMessage"])

			mockService.AssertExpectations(t)
		})
	}
}

// TestTriggerMessageHandler_ConcurrentRequests tests handling concurrent requests
func TestTriggerMessageHandler_ConcurrentRequests(t *testing.T) {
	mockService := new(MockTriggerMessageService)
	handler := TriggerMessageHandler(mockService)

	clientID := "test-cp-001"
	concurrentRequests := 5

	// Setup expectations for concurrent requests
	for i := 0; i < concurrentRequests; i++ {
		responseChan := make(chan types.LiveConfigResponse, 1)
		responseChan <- types.LiveConfigResponse{
			Success: true,
			Data:    map[string]interface{}{"status": "Accepted"},
		}

		result := &services.TriggerMessageResult{
			RequestID:        fmt.Sprintf("req-%d", i),
			ClientID:         clientID,
			RequestedMessage: "StatusNotification",
			ConnectorID:      nil,
		}

		mockService.On("SendTriggerMessage", clientID, "StatusNotification", (*int)(nil)).Return(responseChan, result, nil).Once()
		mockService.On("GetTimeout").Return(10 * time.Second).Once()
	}

	// Execute concurrent requests
	results := make([]int, concurrentRequests)
	done := make(chan bool, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func(index int) {
			requestBody := models.TriggerMessageRequest{
				RequestedMessage: "StatusNotification",
				ConnectorID:      nil,
			}

			req := setupMuxRequest("POST", "/api/v1/chargepoints/test-cp-001/trigger", requestBody, clientID)
			rr := httptest.NewRecorder()

			handler(rr, req)
			results[index] = rr.Code
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < concurrentRequests; i++ {
		<-done
	}

	// Assert all requests succeeded
	for i := 0; i < concurrentRequests; i++ {
		assert.Equal(t, http.StatusOK, results[i])
	}

	mockService.AssertExpectations(t)
}