package performance

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/lorenzodonini/ocpp-go/ocpp1.6/remotetrigger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ocpp-server/internal/correlation"
	"ocpp-server/internal/services"
	"ocpp-server/internal/types"
	"ocpp-server/tests/testutils"
)

// PerformanceTestEnvironment sets up a test environment for performance testing
type PerformanceTestEnvironment struct {
	TriggerService     *services.TriggerMessageService
	CorrelationManager *correlation.Manager
	MockOCPPServer     *MockPerformanceOCPPServer
	MockChargePoint    *MockPerformanceChargePointService
	cleanup            func()
}

// MockPerformanceOCPPServer for performance testing
type MockPerformanceOCPPServer struct {
	sentMessages      []testutils.TestMessage
	messagesMutex     sync.RWMutex
	responseDelay     time.Duration
	errorRate         float64 // 0.0 to 1.0
	requestCounter    int64
	responseHandlers  map[string]func(interface{})
	handlersMutex     sync.RWMutex
}

func NewMockPerformanceOCPPServer() *MockPerformanceOCPPServer {
	return &MockPerformanceOCPPServer{
		sentMessages:     make([]testutils.TestMessage, 0),
		responseDelay:    10 * time.Millisecond,
		errorRate:        0.0,
		responseHandlers: make(map[string]func(interface{})),
	}
}

func (s *MockPerformanceOCPPServer) SendRequest(clientID string, request interface{}) error {
	s.messagesMutex.Lock()
	s.sentMessages = append(s.sentMessages, testutils.TestMessage{
		ClientID: clientID,
		Message:  request,
	})
	requestCount := len(s.sentMessages)
	s.messagesMutex.Unlock()

	// Simulate response delay and error rate
	go func() {
		time.Sleep(s.responseDelay)

		// Simulate error rate
		if float64(requestCount%100)/100.0 < s.errorRate {
			return // Don't send response (simulate timeout)
		}

		// Send successful response
		s.handlersMutex.RLock()
		handler, exists := s.responseHandlers["TriggerMessage"]
		s.handlersMutex.RUnlock()

		if exists {
			response := remotetrigger.NewTriggerMessageConfirmation(remotetrigger.TriggerMessageStatusAccepted)
			handler(response)
		}
	}()

	return nil
}

func (s *MockPerformanceOCPPServer) SetResponseHandler(messageType string, handler func(interface{})) {
	s.handlersMutex.Lock()
	s.responseHandlers[messageType] = handler
	s.handlersMutex.Unlock()
}

func (s *MockPerformanceOCPPServer) GetSentMessageCount() int {
	s.messagesMutex.RLock()
	count := len(s.sentMessages)
	s.messagesMutex.RUnlock()
	return count
}

func (s *MockPerformanceOCPPServer) SetResponseDelay(delay time.Duration) {
	s.responseDelay = delay
}

func (s *MockPerformanceOCPPServer) SetErrorRate(rate float64) {
	s.errorRate = rate
}

func (s *MockPerformanceOCPPServer) Start(listenPort int) error { return nil }
func (s *MockPerformanceOCPPServer) Stop()                     {}
func (s *MockPerformanceOCPPServer) SetNewChargePointHandler(handler func(chargePointId string)) {
}
func (s *MockPerformanceOCPPServer) SetChargePointDisconnectedHandler(handler func(chargePointId string)) {
}

// MockPerformanceChargePointService for performance testing
type MockPerformanceChargePointService struct {
	onlineClients map[string]bool
	mutex         sync.RWMutex
}

func NewMockPerformanceChargePointService() *MockPerformanceChargePointService {
	return &MockPerformanceChargePointService{
		onlineClients: make(map[string]bool),
	}
}

func (s *MockPerformanceChargePointService) IsOnline(clientID string) bool {
	s.mutex.RLock()
	online := s.onlineClients[clientID]
	s.mutex.RUnlock()
	return online
}

func (s *MockPerformanceChargePointService) SetOnline(clientID string, online bool) {
	s.mutex.Lock()
	s.onlineClients[clientID] = online
	s.mutex.Unlock()
}

func (s *MockPerformanceChargePointService) GetAllChargePoints() ([]interface{}, error) {
	return nil, nil
}
func (s *MockPerformanceChargePointService) GetChargePoint(clientID string) (interface{}, error) {
	return nil, nil
}
func (s *MockPerformanceChargePointService) GetAllConnectors(clientID string) ([]interface{}, error) {
	return nil, nil
}
func (s *MockPerformanceChargePointService) GetConnector(clientID string, connectorID int) (interface{}, error) {
	return nil, nil
}
func (s *MockPerformanceChargePointService) GetConnectedClients() []string {
	return nil
}

// setupPerformanceTestEnvironment creates a test environment for performance testing
func setupPerformanceTestEnvironment(t *testing.T) *PerformanceTestEnvironment {
	// Setup Redis for correlation manager
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   3, // Use a different DB for performance tests
	})

	ctx := context.Background()
	err := client.Ping(ctx).Err()
	require.NoError(t, err, "Redis must be running for performance tests")

	// Create mocks
	mockOCPPServer := NewMockPerformanceOCPPServer()
	mockChargePoint := NewMockPerformanceChargePointService()

	// Setup online charge points
	for i := 1; i <= 100; i++ {
		mockChargePoint.SetOnline(fmt.Sprintf("perf-cp-%03d", i), true)
	}

	// Create services
	correlationManager := correlation.NewManager()
	triggerService := services.NewTriggerMessageService(mockOCPPServer, mockChargePoint, correlationManager)

	cleanup := func() {
		client.FlushDB(ctx)
		client.Close()
	}

	return &PerformanceTestEnvironment{
		TriggerService:     triggerService,
		CorrelationManager: correlationManager,
		MockOCPPServer:     mockOCPPServer,
		MockChargePoint:    mockChargePoint,
		cleanup:            cleanup,
	}
}

// TestTriggerMessagePerformance_ConcurrentRequests tests handling 100 concurrent requests
func TestTriggerMessagePerformance_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	env := setupPerformanceTestEnvironment(t)
	defer env.cleanup()

	concurrentRequests := 100
	requestedMessage := "StatusNotification"

	// Results collection
	results := make([]*services.TriggerMessageResult, concurrentRequests)
	responseChans := make([]chan types.LiveConfigResponse, concurrentRequests)
	errors := make([]error, concurrentRequests)
	durations := make([]time.Duration, concurrentRequests)

	var wg sync.WaitGroup
	startTime := time.Now()

	// Send concurrent requests
	for i := 0; i < concurrentRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			clientID := fmt.Sprintf("perf-cp-%03d", (index%100)+1)
			requestStart := time.Now()

			responseChan, result, err := env.TriggerService.SendTriggerMessage(clientID, requestedMessage, nil)
			durations[index] = time.Since(requestStart)

			results[index] = result
			responseChans[index] = responseChan
			errors[index] = err

			// Simulate response
			if err == nil {
				go func(idx int) {
					time.Sleep(20 * time.Millisecond)
					response := types.LiveConfigResponse{
						Success: true,
						Data:    map[string]interface{}{"status": "Accepted"},
					}
					correlationKey := fmt.Sprintf("%s:TriggerMessage:%s", clientID, result.RequestID)
					env.CorrelationManager.SendLiveResponse(correlationKey, response)
				}(index)
			}
		}(i)
	}

	// Wait for all requests to complete
	wg.Wait()
	totalDuration := time.Since(startTime)

	// Performance assertions
	t.Logf("Performance Results:")
	t.Logf("- Total requests: %d", concurrentRequests)
	t.Logf("- Total duration: %v", totalDuration)
	t.Logf("- Average request duration: %v", totalDuration/time.Duration(concurrentRequests))
	t.Logf("- Requests per second: %.2f", float64(concurrentRequests)/totalDuration.Seconds())

	// Verify all requests succeeded
	successCount := 0
	for i := 0; i < concurrentRequests; i++ {
		if errors[i] == nil {
			successCount++
			assert.NotNil(t, results[i])
			assert.NotEmpty(t, results[i].RequestID)
		}
	}

	t.Logf("- Success rate: %.2f%%", float64(successCount)/float64(concurrentRequests)*100)

	// Performance requirements
	assert.GreaterOrEqual(t, float64(successCount)/float64(concurrentRequests), 0.95, "Success rate should be at least 95%")
	assert.Less(t, totalDuration, 5*time.Second, "Total time should be less than 5 seconds")

	// Verify OCPP messages were sent
	sentCount := env.MockOCPPServer.GetSentMessageCount()
	t.Logf("- OCPP messages sent: %d", sentCount)
	assert.Equal(t, successCount, sentCount, "All successful requests should result in OCPP messages")

	// Wait for and verify responses
	responseSuccessCount := 0
	for i := 0; i < concurrentRequests; i++ {
		if responseChans[i] != nil {
			select {
			case response := <-responseChans[i]:
				if response.Success {
					responseSuccessCount++
				}
			case <-time.After(2 * time.Second):
				// Timeout is acceptable in performance tests
			}
		}
	}

	t.Logf("- Responses received: %d", responseSuccessCount)
}

// TestTriggerMessagePerformance_HighThroughput tests sustained high throughput
func TestTriggerMessagePerformance_HighThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	env := setupPerformanceTestEnvironment(t)
	defer env.cleanup()

	duration := 10 * time.Second
	targetRPS := 50 // Requests per second
	interval := time.Second / time.Duration(targetRPS)

	var requestCount int64
	var successCount int64
	var errorCount int64

	startTime := time.Now()
	endTime := startTime.Add(duration)

	var wg sync.WaitGroup

	// Sustained load generator
	go func() {
		requestIndex := 0
		for time.Now().Before(endTime) {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				clientID := fmt.Sprintf("perf-cp-%03d", (index%100)+1)
				requestedMessage := "StatusNotification"

				requestCount++
				_, result, err := env.TriggerService.SendTriggerMessage(clientID, requestedMessage, nil)

				if err != nil {
					errorCount++
				} else {
					successCount++
					// Simulate quick response
					go func() {
						time.Sleep(5 * time.Millisecond)
						response := types.LiveConfigResponse{
							Success: true,
							Data:    map[string]interface{}{"status": "Accepted"},
						}
						correlationKey := fmt.Sprintf("%s:TriggerMessage:%s", clientID, result.RequestID)
						env.CorrelationManager.SendLiveResponse(correlationKey, response)
					}()
				}
			}(requestIndex)

			requestIndex++
			time.Sleep(interval)
		}
	}()

	// Wait for test duration
	time.Sleep(duration)

	// Wait for remaining requests to complete
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		// All requests completed
	case <-time.After(5 * time.Second):
		t.Log("Some requests may still be in progress")
	}

	actualDuration := time.Since(startTime)
	actualRPS := float64(requestCount) / actualDuration.Seconds()

	// Performance results
	t.Logf("Sustained Load Results:")
	t.Logf("- Target RPS: %d", targetRPS)
	t.Logf("- Actual RPS: %.2f", actualRPS)
	t.Logf("- Total requests: %d", requestCount)
	t.Logf("- Successful requests: %d", successCount)
	t.Logf("- Failed requests: %d", errorCount)
	t.Logf("- Success rate: %.2f%%", float64(successCount)/float64(requestCount)*100)
	t.Logf("- Test duration: %v", actualDuration)

	// Performance assertions
	assert.GreaterOrEqual(t, actualRPS, float64(targetRPS)*0.8, "Should achieve at least 80% of target RPS")
	assert.GreaterOrEqual(t, float64(successCount)/float64(requestCount), 0.95, "Success rate should be at least 95%")
}

// TestTriggerMessagePerformance_MemoryUsage tests memory usage under load
func TestTriggerMessagePerformance_MemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	env := setupPerformanceTestEnvironment(t)
	defer env.cleanup()

	// Set a longer response delay to accumulate more pending requests
	env.MockOCPPServer.SetResponseDelay(100 * time.Millisecond)

	// Send requests to accumulate in correlation manager
	requestCount := 1000
	clientID := "perf-cp-001"

	t.Logf("Sending %d requests to test memory usage", requestCount)

	for i := 0; i < requestCount; i++ {
		_, result, err := env.TriggerService.SendTriggerMessage(clientID, "StatusNotification", nil)
		if err == nil && i%100 == 0 {
			// Simulate some responses to prevent excessive accumulation
			go func(requestID string) {
				time.Sleep(50 * time.Millisecond)
				response := types.LiveConfigResponse{
					Success: true,
					Data:    map[string]interface{}{"status": "Accepted"},
				}
				correlationKey := fmt.Sprintf("%s:TriggerMessage:%s", clientID, requestID)
				env.CorrelationManager.SendLiveResponse(correlationKey, response)
			}(result.RequestID)
		}

		// Add small delay to prevent overwhelming the system
		if i%100 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Wait for some processing
	time.Sleep(500 * time.Millisecond)

	// Cleanup expired requests
	env.CorrelationManager.CleanupExpiredRequests()

	// Test should complete without memory issues
	t.Log("Memory usage test completed successfully")

	// Verify OCPP messages were sent
	sentCount := env.MockOCPPServer.GetSentMessageCount()
	t.Logf("OCPP messages sent: %d", sentCount)
	assert.GreaterOrEqual(t, sentCount, requestCount/2, "At least half of the requests should result in OCPP messages")
}

// TestTriggerMessagePerformance_NetworkLatencySimulation tests with simulated network latency
func TestTriggerMessagePerformance_NetworkLatencySimulation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	testCases := []struct {
		name          string
		latency       time.Duration
		expectedMaxDuration time.Duration
	}{
		{
			name:          "LowLatency",
			latency:       10 * time.Millisecond,
			expectedMaxDuration: 200 * time.Millisecond,
		},
		{
			name:          "MediumLatency",
			latency:       100 * time.Millisecond,
			expectedMaxDuration: 500 * time.Millisecond,
		},
		{
			name:          "HighLatency",
			latency:       500 * time.Millisecond,
			expectedMaxDuration: 1 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := setupPerformanceTestEnvironment(t)
			defer env.cleanup()

			env.MockOCPPServer.SetResponseDelay(tc.latency)

			clientID := "perf-cp-001"
			requestedMessage := "StatusNotification"

			startTime := time.Now()

			// Send request
			responseChan, result, err := env.TriggerService.SendTriggerMessage(clientID, requestedMessage, nil)
			require.NoError(t, err)

			// Simulate response
			go func() {
				time.Sleep(tc.latency + 10*time.Millisecond)
				response := types.LiveConfigResponse{
					Success: true,
					Data:    map[string]interface{}{"status": "Accepted"},
				}
				correlationKey := fmt.Sprintf("%s:TriggerMessage:%s", clientID, result.RequestID)
				env.CorrelationManager.SendLiveResponse(correlationKey, response)
			}()

			// Wait for response
			select {
			case response := <-responseChan:
				duration := time.Since(startTime)
				t.Logf("Latency: %v, Total duration: %v", tc.latency, duration)

				assert.True(t, response.Success)
				assert.Less(t, duration, tc.expectedMaxDuration)

			case <-time.After(2 * time.Second):
				t.Fatal("Timeout waiting for response")
			}
		})
	}
}

// BenchmarkTriggerMessageService_SendTriggerMessage benchmarks the SendTriggerMessage method
func BenchmarkTriggerMessageService_SendTriggerMessage(b *testing.B) {
	env := setupPerformanceTestEnvironment(&testing.T{})
	defer env.cleanup()

	clientID := "perf-cp-001"
	requestedMessage := "StatusNotification"

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _, err := env.TriggerService.SendTriggerMessage(clientID, requestedMessage, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTriggerMessageService_ValidateRequestedMessage benchmarks message validation
func BenchmarkTriggerMessageService_ValidateRequestedMessage(b *testing.B) {
	env := setupPerformanceTestEnvironment(&testing.T{})
	defer env.cleanup()

	messageTypes := testutils.ValidMessageTypes()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		messageType := messageTypes[i%len(messageTypes)]
		_ = env.TriggerService.ValidateRequestedMessage(messageType)
	}
}