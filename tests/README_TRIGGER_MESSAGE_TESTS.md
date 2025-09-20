# TriggerMessage Test Suite Documentation

This document provides comprehensive documentation for the TriggerMessage feature test suite, covering all aspects of testing from unit tests to performance testing.

## Overview

The TriggerMessage test suite validates the OCPP 1.6 TriggerMessage feature implementation, which allows the Central System to request specific messages from charge points on demand. The test suite ensures reliability, performance, and correct behavior under various scenarios.

## Test Structure

```
tests/
├── testutils/
│   └── trigger_message_utils.go         # Shared test utilities and helpers
├── integration/
│   └── trigger_message_test.go          # Integration tests with Redis/OCPP
├── performance/
│   └── trigger_message_performance_test.go  # Load and performance tests
└── README_TRIGGER_MESSAGE_TESTS.md     # This documentation

internal/
├── services/
│   └── trigger_message_test.go          # Service layer unit tests
├── api/v1/handlers/
│   └── trigger_test.go                  # HTTP handler tests
└── ocpp/
    └── response_handlers_test.go        # OCPP response handler tests
```

## Test Categories

### 1. Unit Tests

#### Service Layer Tests (`internal/services/trigger_message_test.go`)
Tests the core business logic of the TriggerMessage service:

- **Success scenarios**: Valid requests to online charge points
- **Validation**: Message type validation and connector ID validation
- **Error handling**: Offline charge points, invalid message types, network errors
- **Correlation**: Request ID generation and correlation key management
- **Concurrency**: Thread safety with concurrent requests

**Key Test Cases:**
- `TestTriggerMessageService_SendTriggerMessage_Success`
- `TestTriggerMessageService_SendTriggerMessage_OfflineChargePoint`
- `TestTriggerMessageService_SendTriggerMessage_InvalidMessageType`
- `TestTriggerMessageService_SendTriggerMessage_ConcurrentRequests`

#### HTTP Handler Tests (`internal/api/v1/handlers/trigger_test.go`)
Tests the HTTP API layer:

- **Request parsing**: JSON body parsing and URL parameter extraction
- **Validation**: Input validation and error responses
- **Response formatting**: Correct HTTP status codes and response structure
- **Timeout handling**: Request timeout scenarios
- **Error mapping**: Service errors to HTTP errors

**Key Test Cases:**
- `TestTriggerMessageHandler_Success`
- `TestTriggerMessageHandler_MissingClientID`
- `TestTriggerMessageHandler_InvalidRequestBody`
- `TestTriggerMessageHandler_Timeout`

#### Response Handler Tests (`internal/ocpp/response_handlers_test.go`)
Tests OCPP response handling:

- **Status handling**: Accepted, Rejected, NotSupported responses
- **Correlation cleanup**: Proper cleanup of pending requests
- **Channel management**: Response delivery and blocked channel handling
- **Data structure**: Response data format validation

**Key Test Cases:**
- `TestHandleTriggerMessageResponse_Accepted`
- `TestHandleTriggerMessageResponse_Rejected`
- `TestHandleTriggerMessageResponse_NoPendingRequest`

### 2. Integration Tests (`tests/integration/trigger_message_test.go`)

Tests the complete end-to-end flow with real Redis integration:

- **Full workflow**: HTTP request → Service → OCPP → Response correlation
- **Redis integration**: Correlation manager with Redis backend
- **Message flow**: Complete OCPP message creation and handling
- **Concurrent operations**: Multiple simultaneous trigger requests

**Key Test Cases:**
- `TestTriggerMessageIntegration_EndToEndFlow`
- `TestTriggerMessageIntegration_HTTPEndpoint`
- `TestTriggerMessageIntegration_ConcurrentRequests`
- `TestTriggerMessageIntegration_DifferentMessageTypes`

### 3. Performance Tests (`tests/performance/trigger_message_performance_test.go`)

Tests system performance under load:

- **Concurrent load**: 100+ simultaneous requests
- **Sustained throughput**: 50+ requests per second over time
- **Memory usage**: Memory efficiency under load
- **Network latency**: Performance with various network conditions

**Key Test Cases:**
- `TestTriggerMessagePerformance_ConcurrentRequests`
- `TestTriggerMessagePerformance_HighThroughput`
- `TestTriggerMessagePerformance_MemoryUsage`
- `BenchmarkTriggerMessageService_SendTriggerMessage`

## Test Utilities (`tests/testutils/trigger_message_utils.go`)

Shared utilities for consistent testing across all test files:

### Core Utilities

- **`TriggerMessageTestData`**: Test data builder with fluent API
- **`MockResponseChannel`**: Mock response channels with controlled responses
- **`TriggerMessageTestMatcher`**: Mock matchers for OCPP requests
- **`TriggerMessageResponseBuilder`**: Response builders for different scenarios
- **`TriggerMessageAssertions`**: Common assertions for test validation

### Usage Examples

```go
// Create test data with fluent API
testData := testutils.NewTriggerMessageTestData().
    WithClientID("cp-001").
    WithRequestedMessage("StatusNotification").
    WithConnectorID(&connectorID)

// Create mock response channel
mockResponse := testutils.NewMockResponseChannel(true,
    map[string]interface{}{"status": "Accepted"}, "")

// Use test matchers in mocks
mockOCPPServer.On("SendRequest", clientID,
    matcher.MatchOCPPRequest("StatusNotification", &connectorID)).Return(nil)
```

## Running Tests

### Prerequisites

1. **Redis Server**: Running on `localhost:6379` for integration tests
2. **Go Dependencies**: All required packages installed (`go mod download`)

### Running Specific Test Categories

```bash
# Run all TriggerMessage tests
go test ./... -v -run TriggerMessage

# Run only unit tests (fast)
go test ./internal/services -v -run TriggerMessage
go test ./internal/api/v1/handlers -v -run TriggerMessage
go test ./internal/ocpp -v -run TriggerMessage

# Run integration tests (requires Redis)
go test ./tests/integration -v -run TriggerMessage

# Run performance tests (slow)
go test ./tests/performance -v -run TriggerMessage
go test ./tests/performance -v -run TriggerMessage -timeout 30s

# Run benchmarks
go test ./tests/performance -bench=BenchmarkTriggerMessage -benchmem

# Skip performance tests for quick runs
go test ./... -short -v -run TriggerMessage
```

### Test Coverage

Generate test coverage report:

```bash
# Generate coverage for TriggerMessage components
go test -coverprofile=trigger_coverage.out \
    ./internal/services \
    ./internal/api/v1/handlers \
    ./internal/ocpp \
    -run TriggerMessage

# View coverage report
go tool cover -html=trigger_coverage.out

# Coverage requirements: Minimum 80% overall
```

## Test Scenarios

### Positive Test Scenarios

1. **Basic Trigger Request**
   - Send StatusNotification trigger to online charge point
   - Verify OCPP message creation and sending
   - Confirm successful response correlation

2. **Message Type Variations**
   - Test all supported message types: StatusNotification, Heartbeat, MeterValues, BootNotification
   - Verify correct OCPP enum conversion for each type

3. **Connector ID Handling**
   - Test with specific connector ID (e.g., connector 1)
   - Test without connector ID (trigger for all connectors)
   - Verify connector ID validation (>= 0)

4. **Concurrent Operations**
   - Multiple simultaneous triggers to different charge points
   - Multiple triggers to same charge point with different message types
   - Verify unique request ID generation and correlation

### Negative Test Scenarios

1. **Offline Charge Point**
   - Trigger request to disconnected charge point
   - Verify "client not connected" error response
   - Confirm no OCPP message is sent

2. **Invalid Message Types**
   - Unsupported message type (e.g., "InvalidMessage")
   - Empty string message type
   - Case-sensitive validation ("statusnotification" vs "StatusNotification")

3. **Invalid Input Validation**
   - Missing client ID in URL
   - Invalid JSON request body
   - Negative connector ID values
   - Missing required fields

4. **Network and Timeout Scenarios**
   - Charge point doesn't respond within timeout
   - Network errors during OCPP message sending
   - Correlation manager cleanup of expired requests

5. **Charge Point Rejection**
   - Charge point returns "Rejected" status
   - Charge point returns "NotImplemented" status
   - Response correlation with failure statuses

### Edge Cases

1. **Rapid Successive Requests**
   - Multiple triggers in quick succession to same charge point
   - Verify correlation manager handles overlapping requests

2. **Resource Cleanup**
   - Proper cleanup of correlation data after responses
   - Memory usage under sustained load
   - Handling of blocked response channels

3. **Boundary Conditions**
   - Maximum connector ID values
   - Very long client ID strings
   - Concurrent access to correlation manager

## Performance Requirements

### Throughput Requirements

- **Concurrent Requests**: Handle 100+ simultaneous trigger requests
- **Sustained Load**: Process 50+ requests per second continuously
- **Response Time**: 95% of requests complete within 200ms (excluding charge point response time)

### Reliability Requirements

- **Success Rate**: 95%+ success rate under normal load
- **Error Handling**: Graceful degradation under high load
- **Memory Usage**: Stable memory usage under sustained load

### Scalability Requirements

- **Concurrent Charge Points**: Support 1000+ connected charge points
- **Correlation Management**: Efficient correlation with 10,000+ pending requests
- **Resource Cleanup**: Automatic cleanup of expired/orphaned requests

## Mock Strategy

### Service Layer Mocks

- **MockOCPPServer**: Simulates OCPP server for message sending
- **MockChargePointService**: Controls charge point online/offline status
- **MockCorrelationManager**: Manages request-response correlation

### Integration Test Mocks

- **MockRedisTransport**: Simulates Redis transport with connected clients
- **IntegrationMockOCPPServer**: Full OCPP server simulation with response handlers
- **MockBusinessState**: Redis-backed business state for charge point data

### Performance Test Mocks

- **MockPerformanceOCPPServer**: High-performance mock with configurable latency and error rates
- **MockPerformanceChargePointService**: Thread-safe charge point service for load testing

## Debugging and Troubleshooting

### Common Test Failures

1. **Redis Connection Errors**
   ```
   Error: Redis must be running for integration tests
   ```
   - Solution: Start Redis server on localhost:6379
   - Alternative: Use Redis Docker container

2. **Timeout Failures in Performance Tests**
   ```
   Error: Timeout waiting for response
   ```
   - Cause: System under heavy load or slow Redis
   - Solution: Increase test timeouts or reduce concurrent load

3. **Mock Assertion Failures**
   ```
   Error: mock: Unexpected call to SendRequest
   ```
   - Cause: Mock expectations don't match actual calls
   - Solution: Verify mock setup matches test scenario

### Test Data Validation

Use built-in assertions for consistent validation:

```go
// Validate HTTP responses
assertions := testutils.NewTriggerMessageAssertions()
response := assertions.AssertHTTPResponse(t, recorder, http.StatusOK, true)

// Validate trigger message response structure
assertions.AssertTriggerMessageResponse(t, response.Data,
    "cp-001", "StatusNotification", "Accepted", &connectorID)
```

### Logging and Observability

- Enable verbose test output: `go test -v`
- Performance test metrics are logged automatically
- Integration tests include Redis operation logging
- Mock interactions can be traced with testify mock

## Maintenance and Updates

### Adding New Message Types

When adding support for new trigger message types:

1. Update `ValidMessageTypes()` in test utilities
2. Add test cases for the new message type in all test files
3. Update mock matchers to handle new OCPP enum values
4. Add integration tests for end-to-end flow
5. Update performance tests with new message type variations

### Updating Test Dependencies

- Keep testify/mock library updated for latest mock features
- Monitor Redis Go client for compatibility issues
- Update OCPP library when new versions are available

### Test Environment Management

- Use separate Redis databases for different test categories
- Clean up test data between test runs
- Monitor test execution time and optimize slow tests
- Maintain test isolation to prevent test interference

## Contributing

When adding new tests for TriggerMessage functionality:

1. **Follow Naming Conventions**: Use descriptive test names that explain the scenario
2. **Use Test Utilities**: Leverage existing utilities for consistency
3. **Add Performance Tests**: Include performance validation for new features
4. **Update Documentation**: Keep this documentation current with test changes
5. **Validate Coverage**: Ensure new code has adequate test coverage

### Test Review Checklist

- [ ] Tests cover both positive and negative scenarios
- [ ] Performance implications are considered
- [ ] Mock interactions are properly validated
- [ ] Test data is cleaned up properly
- [ ] Error conditions are tested thoroughly
- [ ] Concurrent access patterns are validated
- [ ] Integration tests include full end-to-end flow