package constants

import "fmt"

// These are not API errors
const (
	MsgFailedToLoadConfig      = "failed to load configuration"
	MsgFailedToCreateValidator = "failed to create url validator"
	MsgFailedToCreateAuthSvc   = "failed to create auth service"
	MsgFailedToHttpServer      = "HTTP server error"
	MsgFailedToStopHttpServer  = "HTTP server graceful shutdown failed"
)

// --- API Error Definitions ---
type ErrorDefinition struct {
	Code    string
	Message string
}

// e.g constants.ErrURLFetch.Errorf("Received 404")
func (e ErrorDefinition) Errorf(args ...interface{}) ErrorDefinition {
	if len(args) > 0 {
		return ErrorDefinition{
			Code:    e.Code,
			Message: fmt.Sprintf(e.Message+": %v", args...),
		}
	}
	return e
}

var (
	// --- General Errors (90000 range) ---
	ErrInternalServer    = ErrorDefinition{Code: "WPA99999", Message: "An internal server error occurred"}
	ErrRateLimit         = ErrorDefinition{Code: "WPA90001", Message: "Too Many Requests"}
	ErrInvalidRequest    = ErrorDefinition{Code: "WPA90002", Message: "Invalid request payload"}
	ErrJsonRequest       = ErrorDefinition{Code: "WPA90002", Message: "Invalid JSON request body"}
	ErrJsonWriteResponse = ErrorDefinition{Code: "WPA90002", Message: "Failed to write JSON response"}

	// --- Auth Service Errors (10000 range) ---
	ErrTokenSign         = ErrorDefinition{Code: "WPA10002", Message: "Failed to sign token"}
	ErrTokenExpired      = ErrorDefinition{Code: "WPA10003", Message: "Token is expired"}
	ErrTokenInvalid      = ErrorDefinition{Code: "WPA10004", Message: "Invalid token"}
	ErrTokenClaims       = ErrorDefinition{Code: "WPA10005", Message: "Invalid token claims"}
	ErrAuthHeaderMissing = ErrorDefinition{Code: "WPA10006", Message: "Authorization header is missing"}
	ErrAuthHeaderFormat  = ErrorDefinition{Code: "WPA10007", Message: "Authorization header must be in 'Bearer <token>' format"}

	// --- Analysis & Validation Errors (20000 range) ---
	ErrURLParse         = ErrorDefinition{Code: "WPA20001", Message: "Failed to parse page content"}
	ErrURLEmpty         = ErrorDefinition{Code: "WPA20002", Message: "URL cannot be empty"}
	ErrURLSchemeMissing = ErrorDefinition{Code: "WPA20003", Message: "URL must start with http:// or https://"}
	ErrURLInvalidFormat = ErrorDefinition{Code: "WPA20004", Message: "Invalid URL format"}
	ErrURLHostMissing   = ErrorDefinition{Code: "WPA20005", Message: "URL must include a valid host"}
	ErrRequestIDMissing = ErrorDefinition{Code: "WPA20006", Message: "requestId is missing"}
	ErrURLMissing       = ErrorDefinition{Code: "WPA20007", Message: "url is missing"}
	ErrURLInvalid       = ErrorDefinition{Code: "WPA20008", Message: "Invalid URL format. Must be a valid URL , e.g. https://example.com"}

	// --- Fetcher Errors (30000 range) ---
	ErrURLFetch           = ErrorDefinition{Code: "WPA30001", Message: "Failed to fetch page"}
	ErrCircuitBreakerOpen = ErrorDefinition{Code: "WPA30002", Message: "Service is temporarily unavailable"}

	// --- Async queue related Errors (40000 range) ---
	ErrFailedToSchedule = ErrorDefinition{Code: "WPA40001", Message: "Failed to schedule service"}
)
