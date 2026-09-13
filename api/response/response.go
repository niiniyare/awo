// Package response defines the canonical API response envelope.
//
// Success: {"data": {...}, "meta": {...}}
// Error:   {"error": {"code": "...", "message": "...", "fields": {...}}}
//
// All framework handlers use these types. Module authors MUST use these types
// to maintain consistent API contracts across the platform.
package response

import (
	"errors"

	"awo.so/awo/runtime"
)

// Success is the envelope for successful responses.
type Success struct {
	Data any   `json:"data"`
	Meta *Meta `json:"meta,omitempty"`
}

// Meta carries pagination and additional context for list responses.
type Meta struct {
	Total    int64  `json:"total,omitempty"`
	Page     int    `json:"page,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
	HasMore  bool   `json:"has_more,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
}

// Envelope wraps the error in the canonical error envelope.
type Envelope struct {
	Error Error `json:"error"`
}

// Error is the machine-readable error body.
type Error struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// HTTPStatus returns the appropriate HTTP status code for err.
// Uses errors.As chains — never type-switches directly.
func HTTPStatus(err error) int {
	return runtime.HTTPStatus(err)
}

// ErrorBody converts any framework error to an Error envelope body.
func ErrorBody(err error) Error {
	var be *runtime.BusinessError
	if errors.As(err, &be) {
		return Error{Code: be.Code, Message: be.Message}
	}

	var ve *runtime.ValidationError
	if errors.As(err, &ve) {
		return Error{Code: "validation_error", Message: "One or more fields are invalid", Fields: ve.Fields}
	}

	var nfe *runtime.NotFoundError
	if errors.As(err, &nfe) {
		return Error{Code: "not_found", Message: nfe.Error()}
	}

	var pe *runtime.PermissionError
	if errors.As(err, &pe) {
		return Error{Code: "forbidden", Message: pe.Error()}
	}

	return Error{Code: "internal_error", Message: "An unexpected error occurred"}
}

// Wrap returns the canonical Envelope for an error. Used in handler returns:
//
//	return c.Status(response.HTTPStatus(err)).JSON(response.Wrap(err))
func Wrap(err error) Envelope {
	return Envelope{Error: ErrorBody(err)}
}
