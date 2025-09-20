package testutils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/gorilla/mux"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/remotetrigger"
	"github.com/stretchr/testify/mock"

	"ocpp-server/internal/api/v1/models"
	"ocpp-server/internal/correlation"
	"ocpp-server/internal/services"
	"ocpp-server/internal/types"
)

// TriggerMessageTestData contains common test data for TriggerMessage tests
type TriggerMessageTestData struct {
	ClientID         string
	RequestedMessage string
	ConnectorID      *int
	RequestID        string
	CorrelationKey   string
}

// NewTriggerMessageTestData creates test data with default values
func NewTriggerMessageTestData() *TriggerMessageTestData {
	connectorID := 1
	return &TriggerMessageTestData{
		ClientID:         "test-cp-001",
		RequestedMessage: "StatusNotification",
		ConnectorID:      &connectorID,
		RequestID:        "req-12345",
		CorrelationKey:   "test-cp-001:TriggerMessage:req-12345",
	}
}

// WithClientID sets a custom client ID
func (td *TriggerMessageTestData) WithClientID(clientID string) *TriggerMessageTestData {
	td.ClientID = clientID
	td.CorrelationKey = fmt.Sprintf("%s:TriggerMessage:%s", clientID, td.RequestID)
	return td
}

// WithRequestedMessage sets a custom requested message
func (td *TriggerMessageTestData) WithRequestedMessage(message string) *TriggerMessageTestData {
	td.RequestedMessage = message
	return td
}

// WithConnectorID sets a custom connector ID
func (td *TriggerMessageTestData) WithConnectorID(connectorID *int) *TriggerMessageTestData {
	td.ConnectorID = connectorID
	return td
}

// WithoutConnectorID removes the connector ID (sets to nil)
func (td *TriggerMessageTestData) WithoutConnectorID() *TriggerMessageTestData {
	td.ConnectorID = nil
	return td
}

// WithRequestID sets a custom request ID
func (td *TriggerMessageTestData) WithRequestID(requestID string) *TriggerMessageTestData {
	td.RequestID = requestID
	td.CorrelationKey = fmt.Sprintf("%s:TriggerMessage:%s", td.ClientID, requestID)
	return td
}

// ToHTTPRequest creates an HTTP request from the test data
func (td *TriggerMessageTestData) ToHTTPRequest() *http.Request {
	requestBody := models.TriggerMessageRequest{
		RequestedMessage: td.RequestedMessage,
		ConnectorID:      td.ConnectorID,
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(requestBody)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/chargepoints/%s/trigger", td.ClientID), &buf)
	req = mux.SetURLVars(req, map[string]string{"clientID": td.ClientID})
	req.Header.Set("Content-Type", "application/json")

	return req
}

// ToServiceResult creates a service result from the test data
func (td *TriggerMessageTestData) ToServiceResult() *services.TriggerMessageResult {
	return &services.TriggerMessageResult{
		RequestID:        td.RequestID,
		ClientID:         td.ClientID,
		RequestedMessage: td.RequestedMessage,
		ConnectorID:      td.ConnectorID,
	}
}

// ToOCPPRequest creates an OCPP TriggerMessage request from the test data
func (td *TriggerMessageTestData) ToOCPPRequest() *remotetrigger.TriggerMessageRequest {
	var messageTrigger remotetrigger.MessageTrigger
	switch td.RequestedMessage {
	case "StatusNotification":
		messageTrigger = remotetrigger.MessageTriggerStatusNotification
	case "Heartbeat":
		messageTrigger = remotetrigger.MessageTriggerHeartbeat
	case "MeterValues":
		messageTrigger = remotetrigger.MessageTriggerMeterValues
	case "BootNotification":
		messageTrigger = remotetrigger.MessageTriggerBootNotification
	default:
		messageTrigger = remotetrigger.MessageTriggerStatusNotification
	}

	request := remotetrigger.NewTriggerMessageRequest(messageTrigger)
	if td.ConnectorID != nil {
		request.ConnectorId = td.ConnectorID
	}

	return request
}

// MockResponseChannel creates a mock response channel with a predefined response
type MockResponseChannel struct {
	Channel  chan types.LiveConfigResponse
	Response types.LiveConfigResponse
}

// NewMockResponseChannel creates a new mock response channel
func NewMockResponseChannel(success bool, data map[string]interface{}, errorMsg string) *MockResponseChannel {
	channel := make(chan types.LiveConfigResponse, 1)
	response := types.LiveConfigResponse{
		Success: success,
		Data:    data,
		Error:   errorMsg,
	}

	return &MockResponseChannel{
		Channel:  channel,
		Response: response,
	}
}

// SendResponse sends the predefined response to the channel
func (mrc *MockResponseChannel) SendResponse() {
	mrc.Channel <- mrc.Response
}

// SendResponseAfterDelay sends the response after a specified delay
func (mrc *MockResponseChannel) SendResponseAfterDelay(delay time.Duration) {
	go func() {
		time.Sleep(delay)
		mrc.SendResponse()
	}()
}

// GetChannel returns the response channel
func (mrc *MockResponseChannel) GetChannel() chan types.LiveConfigResponse {
	return mrc.Channel
}

// TriggerMessageTestMatcher provides matchers for testing TriggerMessage requests
type TriggerMessageTestMatcher struct{}

// NewTriggerMessageTestMatcher creates a new test matcher
func NewTriggerMessageTestMatcher() *TriggerMessageTestMatcher {
	return &TriggerMessageTestMatcher{}
}

// MatchOCPPRequest creates a mock matcher for OCPP TriggerMessage requests
func (m *TriggerMessageTestMatcher) MatchOCPPRequest(expectedMessage string, expectedConnectorID *int) interface{} {
	return mock.MatchedBy(func(req *remotetrigger.TriggerMessageRequest) bool {
		// Check message type
		var expectedTrigger remotetrigger.MessageTrigger
		switch expectedMessage {
		case "StatusNotification":
			expectedTrigger = remotetrigger.MessageTriggerStatusNotification
		case "Heartbeat":
			expectedTrigger = remotetrigger.MessageTriggerHeartbeat
		case "MeterValues":
			expectedTrigger = remotetrigger.MessageTriggerMeterValues
		case "BootNotification":
			expectedTrigger = remotetrigger.MessageTriggerBootNotification
		default:
			return false
		}

		if req.RequestedMessage != expectedTrigger {
			return false
		}

		// Check connector ID
		if expectedConnectorID == nil {
			return req.ConnectorId == nil
		}

		return req.ConnectorId != nil && *req.ConnectorId == *expectedConnectorID
	})
}

// MatchCorrelationKey creates a mock matcher for correlation keys
func (m *TriggerMessageTestMatcher) MatchCorrelationKey(clientID, requestID string) interface{} {
	expectedKey := fmt.Sprintf("%s:TriggerMessage:%s", clientID, requestID)
	return mock.MatchedBy(func(key string) bool {
		return key == expectedKey
	})
}

// MatchCorrelationKeyPattern creates a mock matcher for correlation key patterns
func (m *TriggerMessageTestMatcher) MatchCorrelationKeyPattern(clientID string) interface{} {
	return mock.MatchedBy(func(key string) bool {
		expectedPrefix := fmt.Sprintf("%s:TriggerMessage:", clientID)
		return len(key) > len(expectedPrefix) && key[:len(expectedPrefix)] == expectedPrefix
	})
}

// TriggerMessageResponseBuilder helps build different types of responses
type TriggerMessageResponseBuilder struct {
	clientID string
}

// NewTriggerMessageResponseBuilder creates a new response builder
func NewTriggerMessageResponseBuilder(clientID string) *TriggerMessageResponseBuilder {
	return &TriggerMessageResponseBuilder{clientID: clientID}
}

// BuildAcceptedResponse builds an accepted response
func (b *TriggerMessageResponseBuilder) BuildAcceptedResponse() types.LiveConfigResponse {
	return types.LiveConfigResponse{
		Success: true,
		Data: map[string]interface{}{
			"status":   "Accepted",
			"clientID": b.clientID,
		},
	}
}

// BuildRejectedResponse builds a rejected response
func (b *TriggerMessageResponseBuilder) BuildRejectedResponse() types.LiveConfigResponse {
	return types.LiveConfigResponse{
		Success: false,
		Data: map[string]interface{}{
			"status":   "Rejected",
			"clientID": b.clientID,
		},
		Error: "TriggerMessage rejected by charge point",
	}
}

// BuildNotSupportedResponse builds a not supported response
func (b *TriggerMessageResponseBuilder) BuildNotSupportedResponse() types.LiveConfigResponse {
	return types.LiveConfigResponse{
		Success: false,
		Data: map[string]interface{}{
			"status":   "NotSupported",
			"clientID": b.clientID,
		},
		Error: "TriggerMessage not supported by charge point",
	}
}

// BuildTimeoutResponse builds a timeout response
func (b *TriggerMessageResponseBuilder) BuildTimeoutResponse() types.LiveConfigResponse {
	return types.LiveConfigResponse{
		Success: false,
		Data: map[string]interface{}{
			"status":   "Timeout",
			"clientID": b.clientID,
		},
		Error: "Request timeout",
	}
}

// TriggerMessageAssertions provides common assertions for TriggerMessage tests
type TriggerMessageAssertions struct{}

// NewTriggerMessageAssertions creates a new assertions helper
func NewTriggerMessageAssertions() *TriggerMessageAssertions {
	return &TriggerMessageAssertions{}
}

// AssertHTTPResponse validates an HTTP response for TriggerMessage endpoints
func (a *TriggerMessageAssertions) AssertHTTPResponse(t interface {
	Errorf(format string, args ...interface{})
	FailNow()
}, response *httptest.ResponseRecorder, expectedStatus int, expectedSuccess bool) *models.APIResponse {
	if response.Code != expectedStatus {
		t.Errorf("Expected status %d, got %d", expectedStatus, response.Code)
		t.FailNow()
	}

	var apiResponse models.APIResponse
	if err := json.Unmarshal(response.Body.Bytes(), &apiResponse); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
		t.FailNow()
	}

	if apiResponse.Success != expectedSuccess {
		t.Errorf("Expected success %v, got %v", expectedSuccess, apiResponse.Success)
	}

	return &apiResponse
}

// AssertTriggerMessageResponse validates a TriggerMessage response structure
func (a *TriggerMessageAssertions) AssertTriggerMessageResponse(t interface {
	Errorf(format string, args ...interface{})
	FailNow()
}, data interface{}, expectedClientID, expectedRequestedMessage, expectedStatus string, expectedConnectorID *int) {
	responseData, ok := data.(map[string]interface{})
	if !ok {
		t.Errorf("Response data should be a map")
		t.FailNow()
	}

	// Check required fields
	if clientID, exists := responseData["clientId"]; !exists || clientID != expectedClientID {
		t.Errorf("Expected clientId %s, got %v", expectedClientID, clientID)
	}

	if requestedMessage, exists := responseData["requestedMessage"]; !exists || requestedMessage != expectedRequestedMessage {
		t.Errorf("Expected requestedMessage %s, got %v", expectedRequestedMessage, requestedMessage)
	}

	if status, exists := responseData["status"]; !exists || status != expectedStatus {
		t.Errorf("Expected status %s, got %v", expectedStatus, status)
	}

	// Check connector ID
	if expectedConnectorID == nil {
		if connectorID, exists := responseData["connectorId"]; exists && connectorID != nil {
			t.Errorf("Expected no connectorId, got %v", connectorID)
		}
	} else {
		if connectorID, exists := responseData["connectorId"]; !exists {
			t.Errorf("Expected connectorId %d, but field is missing", *expectedConnectorID)
		} else if connectorID != float64(*expectedConnectorID) { // JSON numbers are float64
			t.Errorf("Expected connectorId %d, got %v", *expectedConnectorID, connectorID)
		}
	}

	// Check that requestId exists
	if requestID, exists := responseData["requestId"]; !exists || requestID == "" {
		t.Errorf("Expected requestId to be present and non-empty, got %v", requestID)
	}
}

// ValidMessageTypes returns a list of all valid trigger message types for testing
func ValidMessageTypes() []string {
	return []string{
		"StatusNotification",
		"Heartbeat",
		"MeterValues",
		"BootNotification",
	}
}

// InvalidMessageTypes returns a list of invalid trigger message types for testing
func InvalidMessageTypes() []string {
	return []string{
		"InvalidMessage",
		"",
		"statusnotification", // case sensitive
		"STATUSNOTIFICATION", // case sensitive
		"RemoteStartTransaction", // valid OCPP message but not supported for trigger
		"Authorization", // valid OCPP message but not supported for trigger
	}
}

// GenerateUniqueRequestID generates a unique request ID for testing
func GenerateUniqueRequestID() string {
	return fmt.Sprintf("test-req-%d", time.Now().UnixNano())
}

// GenerateTestClientID generates a test client ID
func GenerateTestClientID(suffix string) string {
	return fmt.Sprintf("test-cp-%s", suffix)
}

// CreatePendingRequest creates a mock pending request for testing
func CreatePendingRequest(clientID, requestType string) *correlation.PendingRequest {
	return &correlation.PendingRequest{
		Channel:   make(chan types.LiveConfigResponse, 1),
		Timestamp: time.Now(),
		ClientID:  clientID,
		Type:      requestType,
	}
}

// SimulateChargePointResponse simulates a charge point response with a delay
func SimulateChargePointResponse(responseChan chan types.LiveConfigResponse, response types.LiveConfigResponse, delay time.Duration) {
	go func() {
		time.Sleep(delay)
		select {
		case responseChan <- response:
			// Response sent
		default:
			// Channel was closed or blocked
		}
	}()
}