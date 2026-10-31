package config

type SuccessResponse struct {
	SessionID string `json:"session_id"`
	Count     *int64 `json:"count"`
	Data      any    `json:"data"`
	Metadata  any    `json:"metadata,omitempty"`
}

type ContextKey string

const (
	ContextKeyRequestID ContextKey = "request-id"
	ContextAppID        ContextKey = "app-id"
)

var AllowHeadersAPI = []string{"Accept", "Content-Type", "Origin", "X-CSRF-Token", "app-id"}
