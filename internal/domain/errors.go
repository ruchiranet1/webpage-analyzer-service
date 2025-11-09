package domain

// AppError provides a structured way to handle errors throughout the application.
type AppError struct {
	// pattern would be wpa12000
	Code int `json:"code"`

	// Message is to display to the user
	Message string `json:"message"`

	// message for the troubleshooting by the developers
	internalMessage error
}

// NewAppError creates a new application error.
func NewAppError(code int, message string, internalError error) *AppError {
	return &AppError{
		Code:            code,
		Message:         message,
		internalMessage: internalError,
	}
}

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
