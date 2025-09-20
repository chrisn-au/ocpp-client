package ocpp

import (
	"fmt"
	"testing"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/remotetrigger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"ocpp-server/internal/correlation"
	"ocpp-server/internal/types"
)

// MockCorrelationManager for testing response handlers
type MockCorrelationManagerForResponseHandler struct {
	mock.Mock
}

func (m *MockCorrelationManagerForResponseHandler) AddPendingRequest(requestID, clientID, requestType string) chan types.LiveConfigResponse {
	args := m.Called(requestID, clientID, requestType)
	return args.Get(0).(chan types.LiveConfigResponse)
}

func (m *MockCorrelationManagerForResponseHandler) SendLiveResponse(correlationKey string, response types.LiveConfigResponse) {
	m.Called(correlationKey, response)
}

func (m *MockCorrelationManagerForResponseHandler) CleanupPendingRequest(requestID string) {
	m.Called(requestID)
}

func (m *MockCorrelationManagerForResponseHandler) DeletePendingRequest(requestID string) {
	m.Called(requestID)
}

func (m *MockCorrelationManagerForResponseHandler) CleanupExpiredRequests() {
	m.Called()
}

func (m *MockCorrelationManagerForResponseHandler) FindPendingRequest(clientID, requestType string) (string, *correlation.PendingRequest) {
	args := m.Called(clientID, requestType)
	if args.Get(1) == nil {
		return args.String(0), nil
	}
	return args.String(0), args.Get(1).(*correlation.PendingRequest)
}

func (m *MockCorrelationManagerForResponseHandler) SendPendingResponse(clientID, requestType string, response types.LiveConfigResponse) {
	m.Called(clientID, requestType, response)
}

func (m *MockCorrelationManagerForResponseHandler) AddPendingRequestForHandlers(requestID, clientID, requestType string) chan types.LiveConfigResponse {
	args := m.Called(requestID, clientID, requestType)
	return args.Get(0).(chan types.LiveConfigResponse)
}

func (m *MockCorrelationManagerForResponseHandler) SendPendingResponseFromHandlers(clientID, requestType string, response types.LiveConfigResponse) {
	m.Called(clientID, requestType, response)
}

// TestHandleTriggerMessageResponse_Accepted tests handling of accepted trigger message response
func TestHandleTriggerMessageResponse_Accepted(t *testing.T) {
	// Setup
	mockCorrelationManager := new(MockCorrelationManagerForResponseHandler)
	clientID := "test-cp-001"
	requestID := "req-12345"

	// Create a response channel
	responseChan := make(chan types.LiveConfigResponse, 1)
	pendingRequest := &correlation.PendingRequest{
		Channel:   responseChan,
		Timestamp: time.Now(),
		ClientID:  clientID,
		Type:      "TriggerMessage",
	}

	// Create TriggerMessage confirmation with Accepted status
	confirmation := remotetrigger.NewTriggerMessageConfirmation(remotetrigger.TriggerMessageStatusAccepted)

	// Setup expectations
	mockCorrelationManager.On("FindPendingRequest", clientID, "TriggerMessage").Return("correlation-key", pendingRequest)
	mockCorrelationManager.On("DeletePendingRequest", "correlation-key").Return()

	// Execute
	HandleTriggerMessageResponse(mockCorrelationManager, clientID, requestID, confirmation)

	// Verify response was sent to channel
	select {
	case response := <-responseChan:
		assert.True(t, response.Success)
		assert.Equal(t, "Accepted", response.Data["status"])
		assert.Equal(t, clientID, response.Data["clientID"])
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected response to be sent to channel")
	}

	// Verify mocks
	mockCorrelationManager.AssertExpectations(t)
}

// TestHandleTriggerMessageResponse_Rejected tests handling of rejected trigger message response
func TestHandleTriggerMessageResponse_Rejected(t *testing.T) {
	// Setup
	mockCorrelationManager := new(MockCorrelationManagerForResponseHandler)
	clientID := "test-cp-001"
	requestID := "req-12345"

	// Create a response channel
	responseChan := make(chan types.LiveConfigResponse, 1)
	pendingRequest := &correlation.PendingRequest{
		Channel:   responseChan,
		Timestamp: time.Now(),
		ClientID:  clientID,
		Type:      "TriggerMessage",
	}

	// Create TriggerMessage confirmation with Rejected status
	confirmation := remotetrigger.NewTriggerMessageConfirmation(remotetrigger.TriggerMessageStatusRejected)

	// Setup expectations
	mockCorrelationManager.On("FindPendingRequest", clientID, "TriggerMessage").Return("correlation-key", pendingRequest)
	mockCorrelationManager.On("DeletePendingRequest", "correlation-key").Return()

	// Execute
	HandleTriggerMessageResponse(mockCorrelationManager, clientID, requestID, confirmation)

	// Verify response was sent to channel
	select {
	case response := <-responseChan:
		assert.False(t, response.Success)
		assert.Equal(t, "Rejected", response.Data["status"])
		assert.Equal(t, clientID, response.Data["clientID"])
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected response to be sent to channel")
	}

	// Verify mocks
	mockCorrelationManager.AssertExpectations(t)
}

// TestHandleTriggerMessageResponse_NotSupported tests handling of not supported trigger message response
func TestHandleTriggerMessageResponse_NotSupported(t *testing.T) {
	// Setup
	mockCorrelationManager := new(MockCorrelationManagerForResponseHandler)
	clientID := "test-cp-001"
	requestID := "req-12345"

	// Create a response channel
	responseChan := make(chan types.LiveConfigResponse, 1)
	pendingRequest := &correlation.PendingRequest{
		Channel:   responseChan,
		Timestamp: time.Now(),
		ClientID:  clientID,
		Type:      "TriggerMessage",
	}

	// Create TriggerMessage confirmation with NotSupported status
	confirmation := remotetrigger.NewTriggerMessageConfirmation(remotetrigger.TriggerMessageStatusNotSupported)

	// Setup expectations
	mockCorrelationManager.On("FindPendingRequest", clientID, "TriggerMessage").Return("correlation-key", pendingRequest)
	mockCorrelationManager.On("DeletePendingRequest", "correlation-key").Return()

	// Execute
	HandleTriggerMessageResponse(mockCorrelationManager, clientID, requestID, confirmation)

	// Verify response was sent to channel
	select {
	case response := <-responseChan:
		assert.False(t, response.Success)
		assert.Equal(t, "NotSupported", response.Data["status"])
		assert.Equal(t, clientID, response.Data["clientID"])
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected response to be sent to channel")
	}

	// Verify mocks
	mockCorrelationManager.AssertExpectations(t)
}

// TestHandleTriggerMessageResponse_NoPendingRequest tests handling when no pending request exists
func TestHandleTriggerMessageResponse_NoPendingRequest(t *testing.T) {
	// Setup
	mockCorrelationManager := new(MockCorrelationManagerForResponseHandler)
	clientID := "test-cp-001"
	requestID := "req-12345"

	// Create TriggerMessage confirmation
	confirmation := remotetrigger.NewTriggerMessageConfirmation(remotetrigger.TriggerMessageStatusAccepted)

	// Setup expectations - no pending request found
	mockCorrelationManager.On("FindPendingRequest", clientID, "TriggerMessage").Return("", (*correlation.PendingRequest)(nil))

	// Execute - should not panic or error
	HandleTriggerMessageResponse(mockCorrelationManager, clientID, requestID, confirmation)

	// Verify mocks
	mockCorrelationManager.AssertExpectations(t)

	// Verify DeletePendingRequest was not called since no request was found
	mockCorrelationManager.AssertNotCalled(t, "DeletePendingRequest")
}

// TestHandleTriggerMessageResponse_BlockedChannel tests handling when response channel is blocked
func TestHandleTriggerMessageResponse_BlockedChannel(t *testing.T) {
	// Setup
	mockCorrelationManager := new(MockCorrelationManagerForResponseHandler)
	clientID := "test-cp-001"
	requestID := "req-12345"

	// Create a response channel with no buffer (will block)
	responseChan := make(chan types.LiveConfigResponse)
	pendingRequest := &correlation.PendingRequest{
		Channel:   responseChan,
		Timestamp: time.Now(),
		ClientID:  clientID,
		Type:      "TriggerMessage",
	}

	// Create TriggerMessage confirmation
	confirmation := remotetrigger.NewTriggerMessageConfirmation(remotetrigger.TriggerMessageStatusAccepted)

	// Setup expectations
	mockCorrelationManager.On("FindPendingRequest", clientID, "TriggerMessage").Return("correlation-key", pendingRequest)
	mockCorrelationManager.On("DeletePendingRequest", "correlation-key").Return()

	// Execute - should not block indefinitely
	done := make(chan bool)
	go func() {
		HandleTriggerMessageResponse(mockCorrelationManager, clientID, requestID, confirmation)
		done <- true
	}()

	// Should complete quickly even with blocked channel
	select {
	case <-done:
		// Expected - function should return without blocking
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Function should not block when channel is full")
	}

	// Verify mocks
	mockCorrelationManager.AssertExpectations(t)
}

// TestHandleTriggerMessageResponse_AllStatuses tests all possible trigger message statuses
func TestHandleTriggerMessageResponse_AllStatuses(t *testing.T) {
	testCases := []struct {
		name           string
		status         remotetrigger.TriggerMessageStatus
		expectedSuccess bool
	}{
		{
			name:           "Accepted",
			status:         remotetrigger.TriggerMessageStatusAccepted,
			expectedSuccess: true,
		},
		{
			name:           "Rejected",
			status:         remotetrigger.TriggerMessageStatusRejected,
			expectedSuccess: false,
		},
		{
			name:           "NotSupported",
			status:         remotetrigger.TriggerMessageStatusNotSupported,
			expectedSuccess: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup
			mockCorrelationManager := new(MockCorrelationManagerForResponseHandler)
			clientID := "test-cp-001"
			requestID := "req-12345"

			// Create a response channel
			responseChan := make(chan types.LiveConfigResponse, 1)
			pendingRequest := &correlation.PendingRequest{
				Channel:   responseChan,
				Timestamp: time.Now(),
				ClientID:  clientID,
				Type:      "TriggerMessage",
			}

			// Create TriggerMessage confirmation with test status
			confirmation := remotetrigger.NewTriggerMessageConfirmation(tc.status)

			// Setup expectations
			mockCorrelationManager.On("FindPendingRequest", clientID, "TriggerMessage").Return("correlation-key", pendingRequest)
			mockCorrelationManager.On("DeletePendingRequest", "correlation-key").Return()

			// Execute
			HandleTriggerMessageResponse(mockCorrelationManager, clientID, requestID, confirmation)

			// Verify response
			select {
			case response := <-responseChan:
				assert.Equal(t, tc.expectedSuccess, response.Success)
				assert.Equal(t, string(tc.status), response.Data["status"])
				assert.Equal(t, clientID, response.Data["clientID"])
			case <-time.After(100 * time.Millisecond):
				t.Fatal("Expected response to be sent to channel")
			}

			// Verify mocks
			mockCorrelationManager.AssertExpectations(t)
		})
	}
}

// TestHandleTriggerMessageResponse_CorrelationCleanup tests proper cleanup of correlation
func TestHandleTriggerMessageResponse_CorrelationCleanup(t *testing.T) {
	// Setup
	mockCorrelationManager := new(MockCorrelationManagerForResponseHandler)
	clientID := "test-cp-001"
	requestID := "req-12345"
	correlationKey := "test-correlation-key"

	// Create a response channel
	responseChan := make(chan types.LiveConfigResponse, 1)
	pendingRequest := &correlation.PendingRequest{
		Channel:   responseChan,
		Timestamp: time.Now(),
		ClientID:  clientID,
		Type:      "TriggerMessage",
	}

	// Create TriggerMessage confirmation
	confirmation := remotetrigger.NewTriggerMessageConfirmation(remotetrigger.TriggerMessageStatusAccepted)

	// Setup expectations - verify specific correlation key is used for deletion
	mockCorrelationManager.On("FindPendingRequest", clientID, "TriggerMessage").Return(correlationKey, pendingRequest)
	mockCorrelationManager.On("DeletePendingRequest", correlationKey).Return()

	// Execute
	HandleTriggerMessageResponse(mockCorrelationManager, clientID, requestID, confirmation)

	// Verify mocks - specifically that the correct correlation key was used for deletion
	mockCorrelationManager.AssertExpectations(t)
}

// TestHandleTriggerMessageResponse_ResponseDataStructure tests the structure of response data
func TestHandleTriggerMessageResponse_ResponseDataStructure(t *testing.T) {
	// Setup
	mockCorrelationManager := new(MockCorrelationManagerForResponseHandler)
	clientID := "test-cp-001"
	requestID := "req-12345"

	// Create a response channel
	responseChan := make(chan types.LiveConfigResponse, 1)
	pendingRequest := &correlation.PendingRequest{
		Channel:   responseChan,
		Timestamp: time.Now(),
		ClientID:  clientID,
		Type:      "TriggerMessage",
	}

	// Create TriggerMessage confirmation
	confirmation := remotetrigger.NewTriggerMessageConfirmation(remotetrigger.TriggerMessageStatusAccepted)

	// Setup expectations
	mockCorrelationManager.On("FindPendingRequest", clientID, "TriggerMessage").Return("correlation-key", pendingRequest)
	mockCorrelationManager.On("DeletePendingRequest", "correlation-key").Return()

	// Execute
	HandleTriggerMessageResponse(mockCorrelationManager, clientID, requestID, confirmation)

	// Verify response data structure
	select {
	case response := <-responseChan:
		assert.True(t, response.Success)

		// Verify data structure
		data, ok := response.Data.(map[string]interface{})
		assert.True(t, ok, "Response data should be a map")

		// Verify required fields
		status, exists := data["status"]
		assert.True(t, exists, "Response should contain status field")
		assert.Equal(t, "Accepted", status)

		clientIDFromData, exists := data["clientID"]
		assert.True(t, exists, "Response should contain clientID field")
		assert.Equal(t, clientID, clientIDFromData)

		// Verify status is a string
		_, ok = status.(string)
		assert.True(t, ok, "Status should be a string")

	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected response to be sent to channel")
	}

	// Verify mocks
	mockCorrelationManager.AssertExpectations(t)
}

// TestHandleTriggerMessageResponse_ConcurrentCalls tests concurrent response handler calls
func TestHandleTriggerMessageResponse_ConcurrentCalls(t *testing.T) {
	concurrentCalls := 10

	for i := 0; i < concurrentCalls; i++ {
		t.Run(fmt.Sprintf("ConcurrentCall_%d", i), func(t *testing.T) {
			t.Parallel()

			// Setup
			mockCorrelationManager := new(MockCorrelationManagerForResponseHandler)
			clientID := fmt.Sprintf("test-cp-%03d", i)
			requestID := fmt.Sprintf("req-%d", i)

			// Create a response channel
			responseChan := make(chan types.LiveConfigResponse, 1)
			pendingRequest := &correlation.PendingRequest{
				Channel:   responseChan,
				Timestamp: time.Now(),
				ClientID:  clientID,
				Type:      "TriggerMessage",
			}

			// Create TriggerMessage confirmation
			confirmation := remotetrigger.NewTriggerMessageConfirmation(remotetrigger.TriggerMessageStatusAccepted)

			// Setup expectations
			mockCorrelationManager.On("FindPendingRequest", clientID, "TriggerMessage").Return("correlation-key", pendingRequest)
			mockCorrelationManager.On("DeletePendingRequest", "correlation-key").Return()

			// Execute
			HandleTriggerMessageResponse(mockCorrelationManager, clientID, requestID, confirmation)

			// Verify response
			select {
			case response := <-responseChan:
				assert.True(t, response.Success)
				assert.Equal(t, clientID, response.Data["clientID"])
			case <-time.After(500 * time.Millisecond):
				t.Fatal("Expected response to be sent to channel")
			}

			// Verify mocks
			mockCorrelationManager.AssertExpectations(t)
		})
	}
}