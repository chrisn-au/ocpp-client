package models

// APIResponse is the standard response format for all API endpoints
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorData provides additional context for error responses
type ErrorData struct {
	Error   string `json:"error,omitempty"`
	Online  *bool  `json:"online,omitempty"`
	Timeout string `json:"timeout,omitempty"`
	Note    string `json:"note,omitempty"`
}