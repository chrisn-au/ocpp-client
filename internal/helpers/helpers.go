package helpers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// GenerateRequestID generates a unique request ID based on current timestamp
func GenerateRequestID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// SendJSONResponse sends a JSON response with the given status code
func SendJSONResponse(w http.ResponseWriter, statusCode int, response interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}