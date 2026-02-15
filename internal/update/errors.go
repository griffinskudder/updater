package update

import (
	"fmt"
	"net/http"
	"updater/internal/models"
)

// ServiceError represents errors from the update service with HTTP context
type ServiceError struct {
	Code       string
	Message    string
	StatusCode int
	Err        error
}

func (e *ServiceError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *ServiceError) Unwrap() error {
	return e.Err
}

// Error constructors for common service errors

func NewApplicationNotFoundError(appID string) *ServiceError {
	return &ServiceError{
		Code:       models.ErrorCodeApplicationNotFound,
		Message:    fmt.Sprintf("application '%s' not found", appID),
		StatusCode: http.StatusNotFound,
	}
}

func NewInvalidRequestError(message string, err error) *ServiceError {
	return &ServiceError{
		Code:       models.ErrorCodeInvalidRequest,
		Message:    message,
		StatusCode: http.StatusBadRequest,
		Err:        err,
	}
}

func NewValidationError(message string, err error) *ServiceError {
	return &ServiceError{
		Code:       models.ErrorCodeValidation,
		Message:    message,
		StatusCode: http.StatusUnprocessableEntity,
		Err:        err,
	}
}

func NewInternalError(message string, err error) *ServiceError {
	return &ServiceError{
		Code:       models.ErrorCodeInternalError,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
		Err:        err,
	}
}

func NewConflictError(message string) *ServiceError {
	return &ServiceError{
		Code:       models.ErrorCodeConflict,
		Message:    message,
		StatusCode: http.StatusConflict,
	}
}

func NewNotFoundError(message string) *ServiceError {
	return &ServiceError{
		Code:       models.ErrorCodeNotFound,
		Message:    message,
		StatusCode: http.StatusNotFound,
	}
}
