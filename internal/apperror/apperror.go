// Package apperror defines business errors and the error codes returned to API clients.
package apperror

import (
	"errors"
	"net/http"
)

// Code is a machine readable error code.
type Code string

const (
	CodeInvalidRequestBody Code = "INVALID_REQUEST_BODY"
	CodeInvalidProductID   Code = "INVALID_PRODUCT_ID"
	CodeInvalidName        Code = "INVALID_NAME"
	CodeInvalidPrice       Code = "INVALID_PRICE"
	CodeInvalidSalePrice   Code = "INVALID_SALE_PRICE"
	CodeProductNotFound    Code = "PRODUCT_NOT_FOUND"
	CodeInternal           Code = "INTERNAL_SERVER_ERROR"
)

// Error is a business error carrying a Code.
type Error struct {
	Code    Code
	Message string
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

// Is lets errors.Is match errors by code.
func (e *Error) Is(target error) bool {
	var t *Error
	if !errors.As(target, &t) {
		return false
	}
	return t.Code == e.Code
}

// New creates a new business error.
func New(code Code, msg string) *Error { return &Error{Code: code, Message: msg} }

// ErrProductNotFound is returned when a product does not exist.
var ErrProductNotFound = New(CodeProductNotFound, "product not found")

// CodeOf extracts the Code of err, falling back to INTERNAL_SERVER_ERROR.
func CodeOf(err error) Code {
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return CodeInternal
}

// HTTPStatus maps a Code to an HTTP status.
func HTTPStatus(code Code) int {
	switch code {
	case CodeProductNotFound:
		return http.StatusNotFound
	case CodeInternal:
		return http.StatusInternalServerError
	default:
		return http.StatusBadRequest
	}
}
