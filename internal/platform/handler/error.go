package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"
)

// ErrorCode is the closed set from ../CLAUDE.md §6. The frontend switches on
// these strings, so a code may be added but never renamed.
type ErrorCode string

const (
	// 4xx
	ECValidationError ErrorCode = "VALIDATION_ERROR"
	ECUnauthenticated ErrorCode = "UNAUTHENTICATED"
	ECForbidden       ErrorCode = "FORBIDDEN"
	ECRateLimited     ErrorCode = "RATE_LIMITED"

	// 5xx
	ECUpstreamError ErrorCode = "UPSTREAM_ERROR"
	ECInternal      ErrorCode = "INTERNAL"
)

// errorStatus maps code to HTTP status; an unmapped code is served as 500.
var errorStatus = map[ErrorCode]int{
	ECValidationError: http.StatusBadRequest,
	ECUnauthenticated: http.StatusUnauthorized,
	ECForbidden:       http.StatusForbidden,
	ECRateLimited:     http.StatusTooManyRequests,
	ECUpstreamError:   http.StatusBadGateway,
	ECInternal:        http.StatusInternalServerError,
}

// errorMessages is the default text per code. These reach the browser, so they
// must never name an internal cause — that belongs in the log.
var errorMessages = map[ErrorCode]string{
	ECValidationError: "Some of the submitted data is invalid.",
	ECUnauthenticated: "Your session has expired. Please sign in again.",
	ECForbidden:       "That request could not be completed. Reload the page and try again.",
	ECRateLimited:     "Too many requests. Wait a minute and try again.",
	ECUpstreamError:   "Could not reach Anthropic. Please try again.",
	ECInternal:        "Something went wrong. Please try again.",
}

// messageFor never returns "": an unmapped code would otherwise serve an empty
// message to the client.
func messageFor(code ErrorCode) string {
	if msg, ok := errorMessages[code]; ok {
		return msg
	}
	return errorMessages[ECInternal]
}

// APIError is an error that knows how it should be served. Err is logged, never
// serialized, hence no json tags: the wire shape is ErrorResponse's job.
type APIError struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *APIError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

func (e *APIError) Status() int {
	if status, ok := errorStatus[e.Code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// ErrorResponse is the wire format for every failed request: {"error":{...}}.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// Error builds an APIError with the code's default message, keeping cause for
// the log.
func Error(code ErrorCode, cause error) *APIError {
	return &APIError{
		Code:    code,
		Message: messageFor(code),
		Err:     cause,
	}
}

// ErrorWithMessage overrides the default text; never to surface an internal detail.
func ErrorWithMessage(code ErrorCode, message string, cause error) *APIError {
	return &APIError{Code: code, Message: message, Err: cause}
}

// ReturnWithError serves err as the ErrorResponse envelope. Anything that is not
// an *APIError becomes INTERNAL, so an unclassified failure cannot leak its text.
func (h *Handler) ReturnWithError(w http.ResponseWriter, r *http.Request, err error) {
	apiErr := AsAPIError(err)

	// Code and cause only — never the user message or anything from the body.
	log.Printf("%s %s -> %d %s: %v", r.Method, r.URL.Path, apiErr.Status(), apiErr.Code, apiErr.Err)

	h.ReturnWithJSON(w, apiErr.Status(), ErrorResponse{
		Error: ErrorBody{Code: apiErr.Code, Message: apiErr.Message},
	})
}

// AsAPIError resolves any error to the APIError it should be served as. A
// missing message is filled on a copy: the matched error may be a shared
// sentinel that must not be rewritten.
func AsAPIError(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.Message == "" {
			return &APIError{Code: apiErr.Code, Message: messageFor(apiErr.Code), Err: apiErr.Err}
		}
		return apiErr
	}
	return Error(ECInternal, err)
}
