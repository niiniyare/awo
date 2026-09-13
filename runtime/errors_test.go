package runtime_test

import (
	"errors"
	"fmt"
	"testing"

	"awo.so/awo/runtime"
)

// --- ValidationError ---

func TestValidationError_Error(t *testing.T) {
	ve := runtime.NewValidationError("email", "invalid format")
	msg := ve.Error()
	if msg == "" {
		t.Error("Error() should not be empty")
	}
}

func TestValidationError_AddField(t *testing.T) {
	ve := &runtime.ValidationError{}
	ve.AddField("name", "required")
	ve.AddField("email", "invalid")
	if len(ve.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(ve.Fields))
	}
	if ve.Fields["name"] != "required" {
		t.Errorf("name field message mismatch")
	}
}

func TestValidationError_IsEmpty(t *testing.T) {
	ve := &runtime.ValidationError{}
	if !ve.IsEmpty() {
		t.Error("new ValidationError should be empty")
	}
	ve.AddField("x", "err")
	if ve.IsEmpty() {
		t.Error("should not be empty after AddField")
	}
}

func TestIsValidation_True(t *testing.T) {
	ve := runtime.NewValidationError("field", "msg")
	if !runtime.IsValidation(ve) {
		t.Error("IsValidation should return true for *ValidationError")
	}
}

func TestIsValidation_WrappedError(t *testing.T) {
	ve := runtime.NewValidationError("field", "msg")
	wrapped := fmt.Errorf("hook.BeforeCreate: %w", ve)
	if !runtime.IsValidation(wrapped) {
		t.Error("IsValidation should unwrap error chains")
	}
}

func TestIsValidation_False(t *testing.T) {
	if runtime.IsValidation(errors.New("other")) {
		t.Error("IsValidation should be false for non-validation errors")
	}
}

// --- BusinessError ---

func TestBusinessError_Error(t *testing.T) {
	be := &runtime.BusinessError{Code: "invoice.duplicate", Message: "already exists", Status: 409}
	if be.Error() == "" {
		t.Error("Error() should not be empty")
	}
	if be.Code != "invoice.duplicate" {
		t.Errorf("Code mismatch: %q", be.Code)
	}
}

func TestIsBusiness_True(t *testing.T) {
	be := &runtime.BusinessError{Code: "x", Message: "y", Status: 400}
	if !runtime.IsBusiness(be) {
		t.Error("IsBusiness should return true for *BusinessError")
	}
}

func TestIsBusiness_Wrapped(t *testing.T) {
	be := &runtime.BusinessError{Code: "x", Message: "y", Status: 400}
	wrapped := fmt.Errorf("service: %w", be)
	if !runtime.IsBusiness(wrapped) {
		t.Error("IsBusiness should unwrap error chains")
	}
}

// --- NotFoundError ---

func TestNotFoundError_WithID(t *testing.T) {
	nfe := &runtime.NotFoundError{EntityName: "invoice", ID: "inv-123"}
	msg := nfe.Error()
	if msg == "" {
		t.Error("Error() should not be empty")
	}
}

func TestNotFoundError_WithoutID(t *testing.T) {
	nfe := &runtime.NotFoundError{EntityName: "invoice"}
	msg := nfe.Error()
	if msg == "" {
		t.Error("Error() should not be empty")
	}
}

func TestIsNotFound_True(t *testing.T) {
	nfe := &runtime.NotFoundError{EntityName: "invoice", ID: "x"}
	if !runtime.IsNotFound(nfe) {
		t.Error("IsNotFound should return true for *NotFoundError")
	}
}

func TestIsNotFound_Wrapped(t *testing.T) {
	nfe := &runtime.NotFoundError{EntityName: "invoice"}
	wrapped := fmt.Errorf("repo.Get: %w", nfe)
	if !runtime.IsNotFound(wrapped) {
		t.Error("IsNotFound should unwrap error chains")
	}
}

func TestIsNotFound_False(t *testing.T) {
	if runtime.IsNotFound(errors.New("other")) {
		t.Error("IsNotFound should be false for other errors")
	}
}

// --- PermissionError ---

func TestPermissionError_Error(t *testing.T) {
	pe := &runtime.PermissionError{Action: "delete", EntityName: "invoice"}
	if pe.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

func TestIsPermission_True(t *testing.T) {
	pe := &runtime.PermissionError{Action: "write", EntityName: "invoice"}
	if !runtime.IsPermission(pe) {
		t.Error("IsPermission should return true for *PermissionError")
	}
}

func TestIsPermission_Wrapped(t *testing.T) {
	pe := &runtime.PermissionError{Action: "delete", EntityName: "contact"}
	wrapped := fmt.Errorf("authz: %w", pe)
	if !runtime.IsPermission(wrapped) {
		t.Error("IsPermission should unwrap error chains")
	}
}

// --- ImmutableFieldError ---

func TestImmutableFieldError_Error(t *testing.T) {
	ife := &runtime.ImmutableFieldError{EntityName: "invoice", Field: "number"}
	if ife.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

// --- HTTPStatus ---

func TestHTTPStatus_Nil(t *testing.T) {
	if runtime.HTTPStatus(nil) != 200 {
		t.Errorf("HTTPStatus(nil) should be 200")
	}
}

func TestHTTPStatus_NotFound(t *testing.T) {
	err := &runtime.NotFoundError{EntityName: "invoice"}
	if runtime.HTTPStatus(err) != 404 {
		t.Errorf("HTTPStatus(NotFoundError) should be 404, got %d", runtime.HTTPStatus(err))
	}
}

func TestHTTPStatus_Validation(t *testing.T) {
	err := runtime.NewValidationError("f", "m")
	if runtime.HTTPStatus(err) != 422 {
		t.Errorf("HTTPStatus(ValidationError) should be 422, got %d", runtime.HTTPStatus(err))
	}
}

func TestHTTPStatus_Permission(t *testing.T) {
	err := &runtime.PermissionError{Action: "read", EntityName: "invoice"}
	if runtime.HTTPStatus(err) != 403 {
		t.Errorf("HTTPStatus(PermissionError) should be 403, got %d", runtime.HTTPStatus(err))
	}
}

func TestHTTPStatus_Business(t *testing.T) {
	err := &runtime.BusinessError{Code: "x", Message: "y", Status: 409}
	if runtime.HTTPStatus(err) != 409 {
		t.Errorf("HTTPStatus(BusinessError{409}) should be 409, got %d", runtime.HTTPStatus(err))
	}
}

func TestHTTPStatus_Unknown(t *testing.T) {
	err := errors.New("unexpected")
	if runtime.HTTPStatus(err) != 500 {
		t.Errorf("HTTPStatus(unknown) should be 500, got %d", runtime.HTTPStatus(err))
	}
}

func TestHTTPStatus_WrappedErrors(t *testing.T) {
	nfe := &runtime.NotFoundError{EntityName: "invoice"}
	wrapped := fmt.Errorf("service.Get: %w", nfe)
	if runtime.HTTPStatus(wrapped) != 404 {
		t.Errorf("HTTPStatus should unwrap chains, got %d", runtime.HTTPStatus(wrapped))
	}
}
