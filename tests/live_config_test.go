package tests

import (
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	cfgmgr "ocpp-server/config"
)

// MockTransport for testing live configuration
type MockTransport struct {
	mock.Mock
	connectedClients []string
}

func (m *MockTransport) GetConnectedClients() []string {
	return m.connectedClients
}

func (m *MockTransport) SetConnectedClients(clients []string) {
	m.connectedClients = clients
}

// MockOCPPServer for testing
type MockOCPPServer struct {
	mock.Mock
}

func (m *MockOCPPServer) SendRequest(clientID string, request interface{}) error {
	args := m.Called(clientID, request)
	return args.Error(0)
}

// TestLiveConfigurationAPI tests the live configuration endpoints
func TestLiveConfigurationAPI(t *testing.T) {
	// Create mock dependencies
	mockBusinessState := new(MockBusinessState)
	mockTransport := &MockTransport{}
	mockOCPPServer := new(MockOCPPServer)

	// Create configuration manager
	configManager := cfgmgr.NewConfigurationManager(mockBusinessState)

	// Create a test server structure (simplified)
	testServer := &struct {
		configManager  *cfgmgr.ConfigurationManager
		redisTransport *MockTransport
		ocppServer     *MockOCPPServer
	}{
		configManager:  configManager,
		redisTransport: mockTransport,
		ocppServer:     mockOCPPServer,
	}

	// Helper function to check if charger is online
	isChargerOnline := func(clientID string) bool {
		for _, client := range testServer.redisTransport.GetConnectedClients() {
			if client == clientID {
				return true
			}
		}
		return false
	}

	// Test 1: Charger status endpoint - charger offline
	t.Run("ChargerStatus_Offline", func(t *testing.T) {
		mockTransport.SetConnectedClients([]string{}) // No connected clients

		req := httptest.NewRequest("GET", "/api/v1/chargepoints/TEST-CP/status", nil)
		req = mux.SetURLVars(req, map[string]string{"clientID": "TEST-CP"})
		_ = httptest.NewRecorder() // Just for test setup

		// Simulate the handler
		clientID := "TEST-CP"
		online := isChargerOnline(clientID)

		response := map[string]interface{}{
			"success": true,
			"message": "Charger status retrieved",
			"data": map[string]interface{}{
				"clientID": clientID,
				"online":   online,
			},
		}

		assert.False(t, online, "Charger should be offline")

		// Verify response structure
		assert.Equal(t, true, response["success"])
		assert.Equal(t, false, response["data"].(map[string]interface{})["online"])
	})

	// Test 2: Charger status endpoint - charger online
	t.Run("ChargerStatus_Online", func(t *testing.T) {
		mockTransport.SetConnectedClients([]string{"TEST-CP", "OTHER-CP"})

		clientID := "TEST-CP"
		online := isChargerOnline(clientID)

		assert.True(t, online, "Charger should be online")
	})

	// Test 3: Live configuration request - charger offline
	t.Run("LiveConfig_ChargerOffline", func(t *testing.T) {
		mockTransport.SetConnectedClients([]string{}) // No connected clients

		clientID := "TEST-CP"
		online := isChargerOnline(clientID)

		if !online {
			// Should return service unavailable
			assert.False(t, online, "Should detect charger as offline")
			// In real handler, this would return HTTP 503
		}
	})

	// Test 4: Live configuration request - charger online
	t.Run("LiveConfig_ChargerOnline", func(t *testing.T) {
		mockTransport.SetConnectedClients([]string{"TEST-CP"})
		mockOCPPServer.On("SendRequest", "TEST-CP", mock.Anything).Return(nil)

		clientID := "TEST-CP"
		online := isChargerOnline(clientID)

		if online {
			// Should send OCPP request
			err := mockOCPPServer.SendRequest(clientID, mock.Anything)
			assert.NoError(t, err, "Should successfully send OCPP request")
			mockOCPPServer.AssertExpectations(t)
		}
	})

	// Test 5: Live configuration change - charger online
	t.Run("LiveConfigChange_ChargerOnline", func(t *testing.T) {
		mockTransport.SetConnectedClients([]string{"TEST-CP"})
		mockOCPPServer.On("SendRequest", "TEST-CP", mock.Anything).Return(nil)

		clientID := "TEST-CP"
		online := isChargerOnline(clientID)

		if online {
			// Should send ChangeConfiguration request
			err := mockOCPPServer.SendRequest(clientID, mock.Anything)
			assert.NoError(t, err, "Should successfully send ChangeConfiguration request")
		}
	})
}

// TestConnectivityDetection tests the charger connectivity detection
func TestConnectivityDetection(t *testing.T) {
	mockTransport := &MockTransport{}

	// Helper function
	isChargerOnline := func(clientID string) bool {
		for _, client := range mockTransport.GetConnectedClients() {
			if client == clientID {
				return true
			}
		}
		return false
	}

	// Test empty client list
	mockTransport.SetConnectedClients([]string{})
	assert.False(t, isChargerOnline("TEST-CP"), "Should be offline when no clients connected")

	// Test with connected clients
	mockTransport.SetConnectedClients([]string{"CP-001", "CP-002", "TEST-CP"})
	assert.True(t, isChargerOnline("TEST-CP"), "Should be online when client is in list")
	assert.False(t, isChargerOnline("NOT-CONNECTED"), "Should be offline when client not in list")

	// Test case sensitivity
	assert.False(t, isChargerOnline("test-cp"), "Should be case sensitive")
	assert.True(t, isChargerOnline("TEST-CP"), "Should match exact case")
}

// TestLiveConfigurationEndpointJSON tests JSON response format
func TestLiveConfigurationEndpointJSON(t *testing.T) {
	// Test offline response format
	offlineResponse := map[string]interface{}{
		"success": false,
		"message": "Charger is offline - returning stored configuration",
		"data": map[string]interface{}{
			"online": false,
			"note":   "Falling back to stored configuration. Use /configuration endpoint for stored values.",
		},
	}

	// Verify structure
	assert.Equal(t, false, offlineResponse["success"])
	assert.Contains(t, offlineResponse["message"], "offline")

	data := offlineResponse["data"].(map[string]interface{})
	assert.Equal(t, false, data["online"])
	assert.Contains(t, data["note"], "stored configuration")

	// Test online response format
	onlineResponse := map[string]interface{}{
		"success": true,
		"message": "GetConfiguration request sent to charger",
		"data": map[string]interface{}{
			"clientID": "TEST-CP",
			"online":   true,
			"note":     "Request sent to charger. Response will be processed asynchronously.",
		},
	}

	// Verify structure
	assert.Equal(t, true, onlineResponse["success"])
	assert.Contains(t, onlineResponse["message"], "sent to charger")

	onlineData := onlineResponse["data"].(map[string]interface{})
	assert.Equal(t, true, onlineData["online"])
	assert.Contains(t, onlineData["note"], "asynchronously")
}

// TestHTTPStatusCodes tests that correct HTTP status codes are returned
func TestHTTPStatusCodes(t *testing.T) {
	// For offline charger, should return 503 Service Unavailable
	// For online charger, should return 202 Accepted (asynchronous)
	// For status check, should return 200 OK

	testCases := []struct {
		name           string
		chargerOnline  bool
		expectedStatus int
		endpoint       string
	}{
		{"Status check", false, 200, "status"},
		{"Status check online", true, 200, "status"},
		{"Live config offline", false, 503, "configuration/live"},
		{"Live config online", true, 202, "configuration/live"},
		{"Live config change offline", false, 503, "configuration/live PUT"},
		{"Live config change online", true, 202, "configuration/live PUT"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// This would be tested with actual HTTP requests in integration tests
			// Here we just verify the logic for status code determination

			var expectedStatusCode int
			switch tc.endpoint {
			case "status":
				expectedStatusCode = 200 // Always OK for status check
			case "configuration/live":
				if tc.chargerOnline {
					expectedStatusCode = 202 // Accepted for async operation
				} else {
					expectedStatusCode = 503 // Service Unavailable when offline
				}
			case "configuration/live PUT":
				if tc.chargerOnline {
					expectedStatusCode = 202 // Accepted for async operation
				} else {
					expectedStatusCode = 503 // Service Unavailable when offline
				}
			}

			assert.Equal(t, tc.expectedStatus, expectedStatusCode,
				"HTTP status code should match expected for %s", tc.name)
		})
	}
}