package config

type SuccessResponse struct {
	SessionID string `json:"session_id"`
	Count     *int64 `json:"count"`
	Data      any    `json:"data"`
	Metadata  any    `json:"metadata,omitempty"`
}
