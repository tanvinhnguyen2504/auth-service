package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

// ErrorCode is a 5-digit string: the first three digits are the HTTP status the
// error is served with, the last two are the case within that status. So 40302
// is "the second distinct 403". The frontend switches on these strings, so a
// code may be added but never reassigned to a different meaning.
type ErrorCode string

const (
	// 400 — the request itself is malformed.
	CodeValidationFailed ErrorCode = "40001"
	CodeMissingAppID     ErrorCode = "40002"

	// 401 — the caller is not identified.
	CodeNoSession       ErrorCode = "40101"
	CodeSessionInvalid  ErrorCode = "40102"
	CodeNoUserInContext ErrorCode = "40103"

	// 403 — identified, but the request is refused.
	CodeCSRFCookieMissing ErrorCode = "40301"
	CodeCSRFTokenMismatch ErrorCode = "40302"
	CodeAppNotAllowed     ErrorCode = "40303"

	// 429
	CodeRateLimited ErrorCode = "42901"

	// 5xx
	CodeInternal ErrorCode = "50001"
	CodeUpstream ErrorCode = "50201"
)

// errorDetail is the two texts every code carries. reason is for us: a stable
// slug that goes to the log and to the API docs, naming the exact failure.
// message is for the browser, so it must never name an internal cause.
type errorDetail struct {
	reason  string
	message string
}

var errorDetails = map[ErrorCode]errorDetail{
	CodeValidationFailed: {"VALIDATION_FAILED", "Some of the submitted data is invalid."},
	CodeMissingAppID:     {"APP_ID_ABSENT", "Missing app-id."},

	CodeNoSession:       {"SESSION_COOKIE_ABSENT", "Your session has expired. Please sign in again."},
	CodeSessionInvalid:  {"SESSION_INVALID", "Your session has expired. Please sign in again."},
	CodeNoUserInContext: {"USER_NOT_IN_CONTEXT", "Your session has expired. Please sign in again."},

	CodeCSRFCookieMissing: {"CSRF_COOKIE_ABSENT", "That request could not be completed. Reload the page and try again."},
	CodeCSRFTokenMismatch: {"CSRF_TOKEN_MISMATCH", "That request could not be completed. Reload the page and try again."},
	CodeAppNotAllowed:     {"APP_ID_NOT_ALLOWED", "That request could not be completed. Reload the page and try again."},

	CodeRateLimited: {"RATE_LIMITED", "Too many requests. Wait a minute and try again."},

	CodeInternal: {"INTERNAL", "Something went wrong. Please try again."},
	CodeUpstream: {"UPSTREAM_ERROR", "Could not reach the upstream service. Please try again."},
}

// detailFor never returns a zero value: an unregistered code would otherwise
// serve an empty message to the client.
func detailFor(code ErrorCode) errorDetail {
	if detail, ok := errorDetails[code]; ok {
		return detail
	}
	return errorDetails[CodeInternal]
}

func messageFor(code ErrorCode) string { return detailFor(code).message }

// ReasonFor is the developer-facing slug for a code — for logs and docs, never
// for a response body.
func ReasonFor(code ErrorCode) string { return detailFor(code).reason }

// Status reads the HTTP status off the code's first three digits. A code that
// is not 5 digits, or whose prefix is not a real status, is served as 500 — the
// safe direction for a code nobody registered.
func (c ErrorCode) Status() int {
	if len(c) != 5 {
		return http.StatusInternalServerError
	}
	status, err := strconv.Atoi(string(c[:3]))
	if err != nil || status < 400 || status > 599 {
		return http.StatusInternalServerError
	}
	return status
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
		return fmt.Sprintf("%s %s: %s: %v", e.Code, ReasonFor(e.Code), e.Message, e.Err)
	}
	return fmt.Sprintf("%s %s: %s", e.Code, ReasonFor(e.Code), e.Message)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

func (e *APIError) Status() int {
	return e.Code.Status()
}

// ErrorResponse is the wire format for every failed request: {"error":{...}}.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}

type ErrorBody struct {
	Code    ErrorCode `json:"code" example:"40302"`
	Message string    `json:"message" example:"That request could not be completed. Reload the page and try again."`
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
// an *APIError becomes 50001, so an unclassified failure cannot leak its text.
func (h *Handler) ReturnWithError(w http.ResponseWriter, r *http.Request, err error) {
	apiErr := AsAPIError(err)

	// Code, reason and cause only — never the user message or anything from the body.
	log.Printf("%s %s -> %d %s %s: %v",
		r.Method, r.URL.Path, apiErr.Status(), apiErr.Code, ReasonFor(apiErr.Code), apiErr.Err)

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
	return Error(CodeInternal, err)
}
