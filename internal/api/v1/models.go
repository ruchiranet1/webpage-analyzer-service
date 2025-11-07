package v1

type AnalyzeRequest struct {
	RequestID string `json:"requestId" binding:"required"`
	UserID    string `json:"userId" binding:"required"`
	Channel   string `json:"channel" binding:"required"`
	Email     string `json:"email" binding:"required,email"`
	URL       string `json:"url" binding:"required"`
}

type ErrorResponse struct {
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}
