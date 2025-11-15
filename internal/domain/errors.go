package domain

import (
	"net/http"
	"webpage-analyzer-service/internal/constants"
)

// central error handling of the application.
type AppError struct {
	// StatusCode is the HTTP status code (e.g., 400, 401, 500).
	StatusCode int `json:"-"`

	// ErrorCode is the application-specific code (e.g "WPA20004").
	ErrorCode string `json:"error_code"`

	// public-facing error message.
	Message string `json:"message"`

	// private, internal error for logging.
	internalMessage error `json:"-"`
}

// This section provides "overloaded" constructors by calling the main NewAppError.
func NewAppError(statusCode int, errDef constants.ErrorDefinition, internalError error) *AppError {
	return &AppError{
		StatusCode:      statusCode,
		ErrorCode:       errDef.Code,
		Message:         errDef.Message,
		internalMessage: internalError,
	}
}

// NewAppErrorUser creates a new application error with a specific status code
func NewAppErrorUser(statusCode int, errDef constants.ErrorDefinition) *AppError {
	// Calls the main constructor with no internal error
	return NewAppError(statusCode, errDef, nil)
}

// NewInternalError creates a new 500-level error with an internal error.
func NewInternalError(errDef constants.ErrorDefinition, internalError error) *AppError {
	// Calls the main constructor, defaulting the status code to 500
	return NewAppError(http.StatusInternalServerError, errDef, internalError)
}

// NewInternalErrorUser creates a new 500-level user-facing error.
func NewInternalErrorUser(errDef constants.ErrorDefinition) *AppError {
	return NewAppError(http.StatusInternalServerError, errDef, nil)
}

// Error implements the standard Go `error` interface.
func (e *AppError) Error() string {
	if e.internalMessage != nil {
		return e.internalMessage.Error()
	}
	return e.Message
}

// InternalError returns the internal, private error.
func (e *AppError) InternalError() error {
	return e.internalMessage
}
