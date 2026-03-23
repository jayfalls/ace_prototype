// Package validator provides request validation utilities using go-playground/validator.
package validator

import (
	"github.com/go-playground/validator/v10"
)

// Validate is a global validator instance that can be used across handlers.
// It is pre-configured with standard validation tags.
var Validate *validator.Validate

func init() {
	Validate = validator.New()
}

// ValidateStruct validates a struct using the global validator instance.
// Returns validation errors if any, or nil if validation passes.
func ValidateStruct[T any](s T) error {
	return Validate.Struct(s)
}
