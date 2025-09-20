package services

import (
	"fmt"
	"log"
	"time"

	"github.com/lorenzodonini/ocpp-go/ocpp1.6/remotetrigger"
	"github.com/lorenzodonini/ocpp-go/ocppj"

	"ocpp-server/internal/correlation"
	"ocpp-server/internal/helpers"
	"ocpp-server/internal/types"
)

const (
	triggerMessageTimeout = 10 * time.Second
)

// TriggerMessageService handles TriggerMessage business logic and OCPP communication.
//
// This service implements the OCPP 1.6 TriggerMessage feature, providing a clean interface
// for requesting specific messages from charge points on demand. It encapsulates the
// complexity of OCPP protocol handling, request correlation, and timeout management.
//
// The service manages the complete lifecycle of TriggerMessage requests:
//   1. Validates charge point connectivity and message type support
//   2. Creates properly formatted OCPP TriggerMessage requests
//   3. Manages request-response correlation using unique request IDs
//   4. Handles timeout scenarios for unresponsive charge points
//   5. Provides structured responses for API layer consumption
//
// Dependencies:
//   - ocppServer: OCPP server instance for sending requests to charge points
//   - chargePointService: Service for checking charge point connectivity status
//   - correlationManager: Manager for correlating requests with responses
//
// Thread Safety:
// This service is designed to be thread-safe for concurrent TriggerMessage requests
// to different charge points. The correlation manager handles concurrent request
// tracking internally.
//
// Usage Example:
//   service := NewTriggerMessageService(ocppServer, cpService, correlationMgr)
//   responseChan, result, err := service.SendTriggerMessage("CP001", "StatusNotification", &connectorID)
//   if err != nil {
//     // Handle error (charge point offline, invalid message type, etc.)
//   }
//   // Wait for response on responseChan with timeout
type TriggerMessageService struct {
	ocppServer         *ocppj.Server
	chargePointService *ChargePointService
	correlationManager *correlation.Manager
}

// NewTriggerMessageService creates a new TriggerMessageService instance.
//
// This constructor initializes a TriggerMessageService with the required dependencies
// for handling OCPP TriggerMessage requests. All parameters are required and must not be nil.
//
// Parameters:
//   - ocppServer: OCPP server instance for sending requests to charge points
//   - chargePointService: Service for checking charge point connectivity and status
//   - correlationManager: Manager for correlating requests with responses
//
// Returns:
//   A fully configured TriggerMessageService ready to handle trigger requests.
//
// The returned service provides methods for:
//   - Sending TriggerMessage requests with proper validation
//   - Managing request-response correlation
//   - Handling timeout scenarios
//   - Validating message types and connector IDs
func NewTriggerMessageService(
	ocppServer *ocppj.Server,
	chargePointService *ChargePointService,
	correlationManager *correlation.Manager,
) *TriggerMessageService {
	return &TriggerMessageService{
		ocppServer:         ocppServer,
		chargePointService: chargePointService,
		correlationManager: correlationManager,
	}
}

// TriggerMessageResult represents the immediate result of a TriggerMessage operation.
//
// This struct contains the essential information about a TriggerMessage request that was
// sent to a charge point. It's used internally by the service layer and converted to
// appropriate API response models before being returned to HTTP clients.
//
// Fields:
//   - RequestID: Unique identifier generated for request-response correlation
//   - ClientID: The charge point identifier that received the trigger request
//   - RequestedMessage: The type of message that was requested to be triggered
//   - ConnectorID: Optional connector identifier for connector-specific messages
//   - Status: Will be populated based on the charge point's response
//   - Message: Will contain a human-readable description of the result
//
// This result is returned immediately when the TriggerMessage request is sent,
// before waiting for the charge point's response. The actual response status
// (Accepted/Rejected/NotImplemented) is received asynchronously via the response channel.
type TriggerMessageResult struct {
	RequestID        string `json:"requestId"`
	ClientID         string `json:"clientId"`
	RequestedMessage string `json:"requestedMessage"`
	ConnectorID      *int   `json:"connectorId,omitempty"`
	Status           string `json:"status"`
	Message          string `json:"message"`
}

// SendTriggerMessage initiates a TriggerMessage request to a charge point.
//
// This method handles the complete process of sending a TriggerMessage OCPP request,
// including validation, request creation, correlation setup, and error handling.
//
// The method performs the following operations:
//  1. Validates that the target charge point is currently connected
//  2. Converts the string message type to the appropriate OCPP enum
//  3. Creates a properly formatted OCPP TriggerMessage request
//  4. Sets up request-response correlation with a unique request ID
//  5. Sends the request via the OCPP server transport
//  6. Returns a response channel for awaiting the charge point's response
//
// Parameters:
//   - clientID: Unique identifier of the target charge point (must be connected)
//   - requestedMessage: Type of message to trigger ("StatusNotification", "Heartbeat", etc.)
//   - connectorID: Optional connector ID for connector-specific messages (nil for all/none)
//
// Returns:
//   - chan types.LiveConfigResponse: Channel that will receive the charge point's response
//   - *TriggerMessageResult: Immediate result containing request metadata
//   - error: Non-nil if the request could not be sent (charge point offline, invalid message type, etc.)
//
// Error Conditions:
//   - "client not connected": Charge point is not currently connected to the server
//   - "unsupported message type": The requested message type is not supported
//   - "failed to send request": OCPP transport error occurred
//
// Usage Example:
//   responseChan, result, err := service.SendTriggerMessage("CP001", "StatusNotification", &connectorID)
//   if err != nil {
//     return err
//   }
//   // Wait for response with timeout
//   select {
//   case response := <-responseChan:
//     // Handle charge point response
//   case <-time.After(10 * time.Second):
//     // Handle timeout
//   }
//
// Thread Safety:
// This method is thread-safe and can be called concurrently for different charge points.
// Each request uses a unique correlation key to prevent interference between concurrent requests.
func (s *TriggerMessageService) SendTriggerMessage(clientID string, requestedMessage string, connectorID *int) (chan types.LiveConfigResponse, *TriggerMessageResult, error) {
	log.Printf("TRIGGER_MESSAGE_DEBUG: SendTriggerMessage called for client=%s, message=%s", clientID, requestedMessage)

	// Check if client is connected
	if !s.chargePointService.IsOnline(clientID) {
		return nil, nil, fmt.Errorf("client not connected")
	}

	// Convert string to MessageTrigger
	var messageTrigger remotetrigger.MessageTrigger
	switch requestedMessage {
	case "StatusNotification":
		messageTrigger = "StatusNotification"
	case "Heartbeat":
		messageTrigger = "Heartbeat"
	case "MeterValues":
		messageTrigger = "MeterValues"
	case "BootNotification":
		messageTrigger = "BootNotification"
	default:
		return nil, nil, fmt.Errorf("unsupported message type: %s", requestedMessage)
	}

	// Create OCPP TriggerMessage request
	request := remotetrigger.NewTriggerMessageRequest(messageTrigger)

	// Debug: Print request details
	log.Printf("TRIGGER_MESSAGE_DEBUG: Created request with FeatureName: %s", request.GetFeatureName())

	// Set connector ID if provided and message supports it
	if connectorID != nil && (requestedMessage == "StatusNotification" || requestedMessage == "MeterValues") {
		request.ConnectorId = connectorID
	}

	log.Printf("TRIGGER_MESSAGE: Sending TriggerMessage to %s - Message: %s, ConnectorID: %v",
		clientID, requestedMessage, connectorID)

	// Generate request ID for correlation
	requestID := helpers.GenerateRequestID()
	correlationKey := fmt.Sprintf("%s:TriggerMessage:%s", clientID, requestID)
	responseChan := s.correlationManager.AddPendingRequest(correlationKey, clientID, "TriggerMessage")

	// Send request
	log.Printf("TRIGGER_MESSAGE_DEBUG: About to call SendRequest for action: %s", request.GetFeatureName())
	log.Printf("TRIGGER_MESSAGE_DEBUG: Request type: %T", request)
	err := s.ocppServer.SendRequest(clientID, request)
	log.Printf("TRIGGER_MESSAGE_DEBUG: SendRequest returned, err: %v", err)
	if err != nil {
		log.Printf("TRIGGER_MESSAGE: Error sending to %s: %v", clientID, err)
		s.correlationManager.CleanupPendingRequest(correlationKey)
		return nil, nil, fmt.Errorf("failed to send request to charge point: %w", err)
	}

	result := &TriggerMessageResult{
		RequestID:        requestID,
		ClientID:         clientID,
		RequestedMessage: requestedMessage,
		ConnectorID:      connectorID,
	}

	return responseChan, result, nil
}

// ValidateRequestedMessage validates if the requested message type is supported.
//
// This method checks whether the specified message type is supported by the
// TriggerMessage implementation. It serves as a utility for validation in
// request handlers and other components that need to verify message types
// before attempting to send trigger requests.
//
// Parameters:
//   - messageType: The message type string to validate
//
// Returns:
//   - bool: true if the message type is supported, false otherwise
//
// Supported Message Types:
//   - "StatusNotification": Request current connector or charge point status
//   - "Heartbeat": Request immediate heartbeat for connectivity testing
//   - "MeterValues": Request current meter readings from connectors
//   - "BootNotification": Request charge point information and capabilities
//
// Usage Example:
//   if !service.ValidateRequestedMessage("StatusNotification") {
//     return fmt.Errorf("unsupported message type")
//   }
//
// Note: This validation is also performed internally by SendTriggerMessage,
// so external validation is optional but can be useful for early validation
// in request processing pipelines.
func (s *TriggerMessageService) ValidateRequestedMessage(messageType string) bool {
	validTypes := map[string]bool{
		"StatusNotification": true,
		"Heartbeat":          true,
		"MeterValues":        true,
		"BootNotification":   true,
	}
	return validTypes[messageType]
}

// GetTimeout returns the configured timeout duration for TriggerMessage operations.
//
// This method provides access to the timeout value used when waiting for
// charge point responses to TriggerMessage requests. The timeout is used
// by HTTP handlers to determine how long to wait for a response before
// returning a timeout error to the client.
//
// Returns:
//   - time.Duration: The timeout duration (currently 10 seconds)
//
// The timeout value is defined as a package constant and applies to all
// TriggerMessage operations. This ensures consistent timeout behavior
// across all trigger requests regardless of message type or charge point.
//
// Usage Example:
//   timeout := service.GetTimeout()
//   select {
//   case response := <-responseChan:
//     // Handle response
//   case <-time.After(timeout):
//     // Handle timeout
//   }
func (s *TriggerMessageService) GetTimeout() time.Duration {
	return triggerMessageTimeout
}