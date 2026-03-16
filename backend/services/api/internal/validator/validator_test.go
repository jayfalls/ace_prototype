// Package validator provides tests for the validator package.
package validator

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

// TestStruct is a test struct with validation tags.
type TestStruct struct {
	Name  string `validate:"required,min=2,max=10"`
	Email string `validate:"required,email"`
}

func TestValidateStruct_Valid(t *testing.T) {
	testStruct := TestStruct{
		Name:  "John",
		Email: "john@example.com",
	}

	err := ValidateStruct(testStruct)
	if err != nil {
		t.Errorf("expected no validation errors, got %v", err)
	}
}

func TestValidateStruct_Invalid(t *testing.T) {
	testStruct := TestStruct{
		Name:  "A",       // too short (min=2)
		Email: "invalid", // not an email
	}

	err := ValidateStruct(testStruct)
	if err == nil {
		t.Error("expected validation errors, got nil")
	}

	// Verify it's a validation error
	_, ok := err.(validator.ValidationErrors)
	if !ok {
		t.Errorf("expected ValidationErrors, got %T", err)
	}
}

func TestValidateStruct_MissingRequired(t *testing.T) {
	testStruct := TestStruct{
		Name:  "", // missing
		Email: "", // missing
	}

	err := ValidateStruct(testStruct)
	if err == nil {
		t.Error("expected validation errors for missing required fields, got nil")
	}
}

func TestValidateStruct_InvalidEmail(t *testing.T) {
	testStruct := TestStruct{
		Name:  "John",
		Email: "not-an-email",
	}

	err := ValidateStruct(testStruct)
	if err == nil {
		t.Error("expected validation error for invalid email, got nil")
	}
}

func TestValidateStruct_TooLong(t *testing.T) {
	testStruct := TestStruct{
		Name:  "ThisNameIsTooLong", // max=10
		Email: "john@example.com",
	}

	err := ValidateStruct(testStruct)
	if err == nil {
		t.Error("expected validation error for name too long, got nil")
	}
}
