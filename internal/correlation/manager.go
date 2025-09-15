package correlation

import (
	"fmt"
	"log"
	"sync"
	"time"

	"ocpp-server/handlers"
	"ocpp-server/internal/types"
)

const (
	liveConfigTimeout = 10 * time.Second
)

// PendingRequest represents a pending request awaiting a response
type PendingRequest struct {
	Channel   chan types.LiveConfigResponse
	Timestamp time.Time
	ClientID  string
	Type      string // "GetConfiguration", "ChangeConfiguration", etc.
}

// Manager manages pending requests and their correlation
type Manager struct {
	pendingRequests map[string]*PendingRequest
	requestsMutex   sync.RWMutex
}

// NewManager creates a new correlation manager
func NewManager() *Manager {
	return &Manager{
		pendingRequests: make(map[string]*PendingRequest),
	}
}

// AddPendingRequest adds a new pending request and returns a channel for the response
func (m *Manager) AddPendingRequest(requestID, clientID, requestType string) chan types.LiveConfigResponse {
	m.requestsMutex.Lock()
	defer m.requestsMutex.Unlock()

	responseChan := make(chan types.LiveConfigResponse, 1)
	m.pendingRequests[requestID] = &PendingRequest{
		Channel:   responseChan,
		Timestamp: time.Now(),
		ClientID:  clientID,
		Type:      requestType,
	}

	log.Printf("PENDING_REQUEST: Added %s request %s for client %s", requestType, requestID, clientID)
	return responseChan
}

// SendLiveResponse sends a response to a waiting pending request
func (m *Manager) SendLiveResponse(correlationKey string, response types.LiveConfigResponse) {
	m.requestsMutex.Lock()
	defer m.requestsMutex.Unlock()

	if pending, exists := m.pendingRequests[correlationKey]; exists {
		log.Printf("PENDING_REQUEST: Sending response for correlation key %s", correlationKey)
		select {
		case pending.Channel <- response:
			log.Printf("PENDING_REQUEST: Response sent for correlation key %s", correlationKey)
		default:
			log.Printf("PENDING_REQUEST: Channel blocked for correlation key %s", correlationKey)
		}
		delete(m.pendingRequests, correlationKey)
	} else {
		log.Printf("PENDING_REQUEST: No pending request found for correlation key %s", correlationKey)
	}
}

// CleanupExpiredRequests removes expired pending requests
func (m *Manager) CleanupExpiredRequests() {
	m.requestsMutex.Lock()
	defer m.requestsMutex.Unlock()

	now := time.Now()
	for requestID, pending := range m.pendingRequests {
		if now.Sub(pending.Timestamp) > liveConfigTimeout+time.Second {
			log.Printf("PENDING_REQUEST: Cleaning up expired request %s", requestID)
			close(pending.Channel)
			delete(m.pendingRequests, requestID)
		}
	}
}

// FindPendingRequest finds a pending request by client ID and type
func (m *Manager) FindPendingRequest(clientID, requestType string) (string, *PendingRequest) {
	m.requestsMutex.RLock()
	defer m.requestsMutex.RUnlock()

	for key, pending := range m.pendingRequests {
		if pending.ClientID == clientID && pending.Type == requestType {
			return key, pending
		}
	}
	return "", nil
}

// DeletePendingRequest removes a pending request
func (m *Manager) DeletePendingRequest(requestID string) {
	m.requestsMutex.Lock()
	defer m.requestsMutex.Unlock()
	delete(m.pendingRequests, requestID)
}

// CleanupPendingRequest closes and removes a pending request
func (m *Manager) CleanupPendingRequest(requestID string) {
	m.requestsMutex.Lock()
	defer m.requestsMutex.Unlock()

	if pending, exists := m.pendingRequests[requestID]; exists {
		close(pending.Channel)
		delete(m.pendingRequests, requestID)
	}
}

// SendPendingResponse sends a response to a pending request identified by client ID and type
func (m *Manager) SendPendingResponse(clientID, requestType string, response types.LiveConfigResponse) {
	m.requestsMutex.Lock()
	defer m.requestsMutex.Unlock()

	var foundKey string
	var foundRequest *PendingRequest
	for key, pending := range m.pendingRequests {
		if pending.ClientID == clientID && pending.Type == requestType {
			foundKey = key
			foundRequest = pending
			break
		}
	}

	if foundRequest != nil {
		log.Printf("RESPONSE_HANDLER: Found pending %s request %s for client %s", requestType, foundKey, clientID)

		select {
		case foundRequest.Channel <- response:
			log.Printf("RESPONSE_HANDLER: %s response sent for %s", requestType, foundKey)
		default:
			log.Printf("RESPONSE_HANDLER: Channel blocked for %s", foundKey)
		}

		delete(m.pendingRequests, foundKey)
	} else {
		log.Printf("RESPONSE_HANDLER: No pending %s request found for client %s", requestType, clientID)
	}
}

// AddPendingRequestForHandlers adds a pending request and returns a channel compatible with handlers package
func (m *Manager) AddPendingRequestForHandlers(requestID, clientID, requestType string) chan handlers.LiveConfigResponse {
	// Convert internal type to handlers type
	internalChan := m.AddPendingRequest(requestID, clientID, requestType)
	handlersChan := make(chan handlers.LiveConfigResponse, 1)

	go func() {
		resp := <-internalChan
		handlersChan <- handlers.LiveConfigResponse{
			Success: resp.Success,
			Data:    resp.Data,
			Error:   resp.Error,
		}
		close(handlersChan)
	}()

	return handlersChan
}

// SendPendingResponseFromHandlers sends a response from handlers package format
func (m *Manager) SendPendingResponseFromHandlers(clientID, requestType string, response handlers.LiveConfigResponse) {
	m.SendPendingResponse(clientID, requestType, types.LiveConfigResponse{
		Success: response.Success,
		Data:    response.Data,
		Error:   response.Error,
	})
}

// GenerateCorrelationKey generates a correlation key for a request
func GenerateCorrelationKey(clientID, requestType, requestID string) string {
	return fmt.Sprintf("%s:%s:%s", clientID, requestType, requestID)
}