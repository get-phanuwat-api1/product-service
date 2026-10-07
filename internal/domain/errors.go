package domain

import "errors"

// ErrorCode is a machine readable error code returned to API clients.
type ErrorCode string

const (
	ErrCodeInvalidRequestBody ErrorCode = "INVALID_REQUEST_BODY"
	ErrCodeInvalidProductID   ErrorCode = "INVALID_PRODUCT_ID"
	ErrCodeInvalidName        ErrorCode = "INVALID_NAME"
	ErrCodeInvalidPrice       ErrorCode = "INVALID_PRICE"
	ErrCodeInvalidSalePrice   ErrorCode = "INVALID_SALE_PRICE"
	ErrCodeProductNotFound    ErrorCode = "PRODUCT_NOT_FOUND"
	ErrCodeInternal           ErrorCode = "INTERNAL_SERVER_ERROR"
)

// Error is a business error carrying an ErrorCode.
type Error struct {
	Code    ErrorCode
	Message string
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

// Is lets errors.Is match domain errors by code.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return t.Code == e.Code
}

// NewError creates a new domain error.
func NewError(code ErrorCode, msg string) *Error { return &Error{Code: code, Message: msg} }

// ErrProductNotFound is returned when a product does not exist.
var ErrProductNotFound = NewError(ErrCodeProductNotFound, "product not found")

// CodeOf extracts the ErrorCode of err, falling back to INTERNAL_SERVER_ERROR.
func CodeOf(err error) ErrorCode {
	var de *Error
	if errors.As(err, &de) {
		return de.Code
	}
	return ErrCodeInternal
}
