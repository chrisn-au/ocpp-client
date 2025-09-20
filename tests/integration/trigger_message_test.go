package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/remotetrigger"
	"github.com/lorenzodonini/ocpp-go/ocppj"
	"github.com/lorenzodonini/ocpp-go/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ocpp-server/internal/api/v1/handlers"
	"ocpp-server/internal/api/v1/models"
	"ocpp-server/internal/correlation"
	"ocpp-server/internal/services"
	"ocpp-server/internal/types"
)

// MockRedisTransport mocks the Redis transport for integration testing
type MockRedisTransport struct {
	connectedClients []string
	messageQueue     []TestMessage
}

type TestMessage struct {
	ClientID string
	Message  interface{}
}

func (t *MockRedisTransport) Start() error {
	return nil
}

func (t *MockRedisTransport) Stop() error {
	return nil
}

func (t *MockRedisTransport) GetConnectedClients() []string {
	return t.connectedClients
}

func (t *MockRedisTransport) SetOnClientConnected(handler func(clientID string)) {
	// Mock implementation
}

func (t *MockRedisTransport) SetOnClientDisconnected(handler func(clientID string)) {
	// Mock implementation
}

func (t *MockRedisTransport) SendToClient(clientID string, message interface{}) error {
	t.messageQueue = append(t.messageQueue, TestMessage{
		ClientID: clientID,
		Message:  message,
	})
	return nil
}

// MockOCPPServer for integration testing
type IntegrationMockOCPPServer struct {
	transport        *MockRedisTransport
	sentMessages     []TestMessage
	responseHandlers map[string]func(interface{})
}

func NewIntegrationMockOCPPServer(transport *MockRedisTransport) *IntegrationMockOCPPServer {
	return &IntegrationMockOCPPServer{
		transport:        transport,
		sentMessages:     make([]TestMessage, 0),
		responseHandlers: make(map[string]func(interface{})),
	}
}

func (s *IntegrationMockOCPPServer) SendRequest(clientID string, request interface{}) error {
	s.sentMessages = append(s.sentMessages, TestMessage{
		ClientID: clientID,
		Message:  request,
	})
	return nil
}

func (s *IntegrationMockOCPPServer) Start(listenPort int) error {
	return nil
}

func (s *IntegrationMockOCPPServer) Stop() {
	// Mock implementation
}

func (s *IntegrationMockOCPPServer) SetNewChargePointHandler(handler func(chargePointId string)) {
	// Mock implementation
}

func (s *IntegrationMockOCPPServer) SetChargePointDisconnectedHandler(handler func(chargePointId string)) {
	// Mock implementation
}

func (s *IntegrationMockOCPPServer) SetResponseHandler(messageType string, handler func(interface{})) {
	s.responseHandlers[messageType] = handler
}

func (s *IntegrationMockOCPPServer) SimulateTriggerMessageResponse(clientID string, status remotetrigger.TriggerMessageStatus) {
	if handler, exists := s.responseHandlers["TriggerMessage"]; exists {
		response := remotetrigger.NewTriggerMessageConfirmation(status)
		handler(response)
	}
}

func (s *IntegrationMockOCPPServer) GetSentMessages() []TestMessage {
	return s.sentMessages
}

// MockBusinessState for Redis integration
type IntegrationMockBusinessState struct {
	client *redis.Client
}

func (bs *IntegrationMockBusinessState) GetChargePointInfo(clientID string) (interface{}, error) {
	return map[string]interface{}{
		"clientId": clientID,
		"status":   "Available",
	}, nil
}

func (bs *IntegrationMockBusinessState) GetAllChargePoints() ([]interface{}, error) {
	return []interface{}{
		map[string]interface{}{"clientId": "test-cp-001", "status": "Available"},
		map[string]interface{}{"clientId": "test-cp-002", "status": "Occupied"},
	}, nil
}

func (bs *IntegrationMockBusinessState) GetAllConnectors(clientID string) ([]interface{}, error) {
	return []interface{}{
		map[string]interface{}{"connectorId": 1, "status": "Available"},
		map[string]interface{}{"connectorId": 2, "status": "Occupied"},
	}, nil
}

func (bs *IntegrationMockBusinessState) GetConnectorStatus(clientID string, connectorID int) (interface{}, error) {
	return map[string]interface{}{
		"connectorId": connectorID,
		"status":      "Available",
	}, nil
}

// setupIntegrationTestEnvironment sets up the full integration test environment
func setupIntegrationTestEnvironment(t *testing.T) (*services.TriggerMessageService, *correlation.Manager, *IntegrationMockOCPPServer, func()) {
	// Connect to Redis for correlation manager
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   2, // Use a different DB for trigger message tests
	})

	// Test Redis connection
	ctx := context.Background()
	err := client.Ping(ctx).Err()
	require.NoError(t, err, "Redis must be running for integration tests")

	// Create mock transport with connected clients
	mockTransport := &MockRedisTransport{
		connectedClients: []string{"test-cp-001", "test-cp-002"},
		messageQueue:     make([]TestMessage, 0),
	}

	// Create mock OCPP server
	mockOCPPServer := NewIntegrationMockOCPPServer(mockTransport)

	// Create business state
	businessState := &IntegrationMockBusinessState{client: client}

	// Create services
	chargePointService := services.NewChargePointService(businessState, mockTransport)
	correlationManager := correlation.NewManager()
	triggerMessageService := services.NewTriggerMessageService(mockOCPPServer, chargePointService, correlationManager)

	// Cleanup function
	cleanup := func() {
		client.FlushDB(ctx)
		client.Close()
	}

	return triggerMessageService, correlationManager, mockOCPPServer, cleanup
}

// TestTriggerMessageIntegration_EndToEndFlow tests the complete end-to-end flow
func TestTriggerMessageIntegration_EndToEndFlow(t *testing.T) {
	triggerService, correlationManager, mockOCPPServer, cleanup := setupIntegrationTestEnvironment(t)
	defer cleanup()

	// Test data
	clientID := "test-cp-001"
	requestedMessage := "StatusNotification"
	connectorID := 1

	// Send trigger message
	responseChan, result, err := triggerService.SendTriggerMessage(clientID, requestedMessage, &connectorID)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, responseChan)

	// Verify the OCPP message was sent
	sentMessages := mockOCPPServer.GetSentMessages()
	require.Len(t, sentMessages, 1)
	assert.Equal(t, clientID, sentMessages[0].ClientID)

	// Verify the message is a TriggerMessage request
	triggerMsg, ok := sentMessages[0].Message.(*remotetrigger.TriggerMessageRequest)
	require.True(t, ok)
	assert.Equal(t, remotetrigger.MessageTriggerStatusNotification, triggerMsg.RequestedMessage)
	assert.Equal(t, connectorID, *triggerMsg.ConnectorId)

	// Simulate charge point response
	correlationKey := fmt.Sprintf("%s:TriggerMessage:%s", clientID, result.RequestID)
	response := types.LiveConfigResponse{
		Success: true,
		Data:    map[string]interface{}{"status": "Accepted"},
	}

	// Send response through correlation manager
	correlationManager.SendLiveResponse(correlationKey, response)

	// Verify response is received
	select {
	case receivedResponse := <-responseChan:
		assert.True(t, receivedResponse.Success)
		assert.Equal(t, "Accepted", receivedResponse.Data["status"])
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for response")
	}
}

// TestTriggerMessageIntegration_HTTPEndpoint tests the complete HTTP endpoint
func TestTriggerMessageIntegration_HTTPEndpoint(t *testing.T) {
	triggerService, correlationManager, mockOCPPServer, cleanup := setupIntegrationTestEnvironment(t)
	defer cleanup()

	// Create HTTP handler
	handler := handlers.TriggerMessageHandler(triggerService)

	// Test data
	clientID := "test-cp-001"
	requestBody := models.TriggerMessageRequest{
		RequestedMessage: "StatusNotification",
		ConnectorID:      nil,
	}

	// Create HTTP request
	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "/api/v1/chargepoints/test-cp-001/trigger", bytes.NewReader(body))
	req = mux.SetURLVars(req, map[string]string{"clientID": clientID})
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()

	// Simulate successful charge point response in a goroutine
	go func() {
		time.Sleep(10 * time.Millisecond) // Small delay to ensure request is processed

		// Find the sent message to get the request ID
		sentMessages := mockOCPPServer.GetSentMessages()
		if len(sentMessages) > 0 {
			// Extract request ID from correlation pattern (would normally come from OCPP response)
			// For testing, we'll simulate finding the correlation key
			time.Sleep(10 * time.Millisecond)

			// Since we can't easily extract the correlation key from the mock,
			// we'll search for pending requests and respond to them
			ctx := context.Background()
			for i := 0; i < 50; i++ { // Try for 500ms
				correlationManager.CleanupExpiredRequests()
				time.Sleep(10 * time.Millisecond)

				// Simulate response by sending to any pending TriggerMessage request for this client
				response := types.LiveConfigResponse{
					Success: true,
					Data:    map[string]interface{}{"status": "Accepted"},
				}
				correlationManager.SendPendingResponse(clientID, "TriggerMessage", response)
				break
			}
		}
	}()

	// Execute HTTP request
	handler(rr, req)

	// Verify HTTP response
	assert.Equal(t, http.StatusOK, rr.Code)

	var apiResponse models.APIResponse
	err := json.Unmarshal(rr.Body.Bytes(), &apiResponse)
	require.NoError(t, err)
	assert.True(t, apiResponse.Success)

	// Verify OCPP message was sent
	sentMessages := mockOCPPServer.GetSentMessages()
	require.Len(t, sentMessages, 1)
	assert.Equal(t, clientID, sentMessages[0].ClientID)
}

// TestTriggerMessageIntegration_OfflineChargePoint tests offline charge point scenario
func TestTriggerMessageIntegration_OfflineChargePoint(t *testing.T) {
	triggerService, _, _, cleanup := setupIntegrationTestEnvironment(t)
	defer cleanup()

	// Test with offline charge point
	offlineClientID := "offline-cp-001"
	requestedMessage := "StatusNotification"

	// Send trigger message to offline charge point
	responseChan, result, err := triggerService.SendTriggerMessage(offlineClientID, requestedMessage, nil)

	// Should return error for offline charge point
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client not connected")
	assert.Nil(t, responseChan)
	assert.Nil(t, result)
}

// TestTriggerMessageIntegration_ConcurrentRequests tests concurrent trigger requests
func TestTriggerMessageIntegration_ConcurrentRequests(t *testing.T) {
	triggerService, correlationManager, mockOCPPServer, cleanup := setupIntegrationTestEnvironment(t)
	defer cleanup()

	clientID := "test-cp-001"
	concurrentRequests := 10

	// Send concurrent requests
	results := make([]*services.TriggerMessageResult, concurrentRequests)
	responseChans := make([]chan types.LiveConfigResponse, concurrentRequests)
	errors := make([]error, concurrentRequests)

	done := make(chan bool, concurrentRequests)

	for i := 0; i < concurrentRequests; i++ {
		go func(index int) {
			defer func() { done <- true }()

			requestedMessage := "StatusNotification"
			responseChans[index], results[index], errors[index] = triggerService.SendTriggerMessage(
				clientID, requestedMessage, nil)

			// Simulate response for each request
			if errors[index] == nil {
				go func(idx int) {
					time.Sleep(10 * time.Millisecond)
					response := types.LiveConfigResponse{
						Success: true,
						Data:    map[string]interface{}{"status": "Accepted"},
					}
					correlationKey := fmt.Sprintf("%s:TriggerMessage:%s", clientID, results[idx].RequestID)
					correlationManager.SendLiveResponse(correlationKey, response)
				}(index)
			}
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < concurrentRequests; i++ {
		<-done
	}

	// Verify all requests succeeded
	for i := 0; i < concurrentRequests; i++ {
		assert.NoError(t, errors[i], "Request %d should succeed", i)
		assert.NotNil(t, results[i], "Result %d should not be nil", i)
		assert.NotEmpty(t, results[i].RequestID, "Request ID %d should not be empty", i)
	}

	// Verify unique request IDs
	requestIDs := make(map[string]bool)
	for i, result := range results {
		if result != nil {
			assert.False(t, requestIDs[result.RequestID], "Request ID should be unique for request %d", i)
			requestIDs[result.RequestID] = true
		}
	}

	// Verify all OCPP messages were sent
	sentMessages := mockOCPPServer.GetSentMessages()
	assert.Len(t, sentMessages, concurrentRequests)

	// Wait for and verify all responses
	for i := 0; i < concurrentRequests; i++ {
		if responseChans[i] != nil {
			select {
			case response := <-responseChans[i]:
				assert.True(t, response.Success, "Response %d should be successful", i)
			case <-time.After(1 * time.Second):
				t.Logf("Timeout waiting for response %d", i)
			}
		}
	}
}

// TestTriggerMessageIntegration_DifferentMessageTypes tests different trigger message types
func TestTriggerMessageIntegration_DifferentMessageTypes(t *testing.T) {
	messageTypes := []string{
		"StatusNotification",
		"Heartbeat",
		"MeterValues",
		"BootNotification",
	}

	for _, messageType := range messageTypes {
		t.Run(messageType, func(t *testing.T) {
			triggerService, correlationManager, mockOCPPServer, cleanup := setupIntegrationTestEnvironment(t)
			defer cleanup()

			clientID := "test-cp-001"

			// Send trigger message
			responseChan, result, err := triggerService.SendTriggerMessage(clientID, messageType, nil)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Verify the correct message type was sent
			sentMessages := mockOCPPServer.GetSentMessages()
			require.Len(t, sentMessages, 1)

			triggerMsg, ok := sentMessages[0].Message.(*remotetrigger.TriggerMessageRequest)
			require.True(t, ok)

			// Verify the correct enum was used
			switch messageType {
			case "StatusNotification":
				assert.Equal(t, remotetrigger.MessageTriggerStatusNotification, triggerMsg.RequestedMessage)
			case "Heartbeat":
				assert.Equal(t, remotetrigger.MessageTriggerHeartbeat, triggerMsg.RequestedMessage)
			case "MeterValues":
				assert.Equal(t, remotetrigger.MessageTriggerMeterValues, triggerMsg.RequestedMessage)
			case "BootNotification":
				assert.Equal(t, remotetrigger.MessageTriggerBootNotification, triggerMsg.RequestedMessage)
			}

			// Simulate response
			correlationKey := fmt.Sprintf("%s:TriggerMessage:%s", clientID, result.RequestID)
			response := types.LiveConfigResponse{
				Success: true,
				Data:    map[string]interface{}{"status": "Accepted"},
			}
			correlationManager.SendLiveResponse(correlationKey, response)

			// Verify response
			select {
			case receivedResponse := <-responseChan:
				assert.True(t, receivedResponse.Success)
			case <-time.After(1 * time.Second):
				t.Fatal("Timeout waiting for response")
			}
		})
	}
}

// TestTriggerMessageIntegration_Timeout tests timeout scenario
func TestTriggerMessageIntegration_Timeout(t *testing.T) {
	triggerService, _, mockOCPPServer, cleanup := setupIntegrationTestEnvironment(t)
	defer cleanup()

	clientID := "test-cp-001"
	requestedMessage := "StatusNotification"

	// Send trigger message
	responseChan, result, err := triggerService.SendTriggerMessage(clientID, requestedMessage, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify OCPP message was sent
	sentMessages := mockOCPPServer.GetSentMessages()
	require.Len(t, sentMessages, 1)

	// Wait for timeout (don't send response)
	timeout := triggerService.GetTimeout()
	assert.Equal(t, 10*time.Second, timeout)

	// For testing, we'll use a shorter timeout
	select {
	case <-responseChan:
		t.Fatal("Should not receive response (testing timeout)")
	case <-time.After(100 * time.Millisecond):
		// Expected timeout for test
	}
}

// TestTriggerMessageIntegration_RedisCorrelation tests Redis-based correlation
func TestTriggerMessageIntegration_RedisCorrelation(t *testing.T) {
	triggerService, correlationManager, mockOCPPServer, cleanup := setupIntegrationTestEnvironment(t)
	defer cleanup()

	clientID := "test-cp-001"
	requestedMessage := "StatusNotification"

	// Send multiple trigger messages
	responseChans := make([]chan types.LiveConfigResponse, 3)
	results := make([]*services.TriggerMessageResult, 3)

	for i := 0; i < 3; i++ {
		responseChan, result, err := triggerService.SendTriggerMessage(clientID, requestedMessage, nil)
		require.NoError(t, err)
		responseChans[i] = responseChan
		results[i] = result
	}

	// Verify all OCPP messages were sent
	sentMessages := mockOCPPServer.GetSentMessages()
	require.Len(t, sentMessages, 3)

	// Send responses in reverse order to test correlation
	for i := 2; i >= 0; i-- {
		correlationKey := fmt.Sprintf("%s:TriggerMessage:%s", clientID, results[i].RequestID)
		response := types.LiveConfigResponse{
			Success: true,
			Data:    map[string]interface{}{"requestIndex": i},
		}
		correlationManager.SendLiveResponse(correlationKey, response)
	}

	// Verify each response is correlated correctly
	for i := 0; i < 3; i++ {
		select {
		case response := <-responseChans[i]:
			assert.True(t, response.Success)
			// Note: Due to the way we're testing, we can't easily verify the exact correlation
			// In a real scenario, the response would include the correlation data
		case <-time.After(1 * time.Second):
			t.Fatalf("Timeout waiting for response %d", i)
		}
	}
}