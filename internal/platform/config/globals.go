package config

type SuccessResponse struct {
	SessionID string      `json:"session_id"`
	Count     *int64      `json:"count"`
	Data      interface{} `json:"data"`
	Metadata  interface{} `json:"metadata,omitempty"`
}

type ErrorReponse struct {
	SessionID string `json:"session_id"`
}

type ContextKey string

const (
	ContextKeyAppID  ContextKey = "app_id"
	ContextKeyUserID ContextKey = "user_id"
)
