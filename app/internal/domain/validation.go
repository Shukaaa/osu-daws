package domain

import (
	"fmt"
	"strings"
)

type ValidationError struct {
	Code    string
	Message string
	Field   string
	Details map[string]any
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s (field=%s)", e.Code, e.Message, e.Field)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewValidationError(code, field, message string) *ValidationError {
	return &ValidationError{Code: code, Field: field, Message: message}
}

type ValidationResult struct {
	Errors []*ValidationError
}

func (r *ValidationResult) Add(err *ValidationError) {
	if err == nil {
		return
	}
	r.Errors = append(r.Errors, err)
}

func (r *ValidationResult) Addf(code, field, format string, args ...any) {
	r.Errors = append(r.Errors, NewValidationError(code, field, fmt.Sprintf(format, args...)))
}

func (r *ValidationResult) OK() bool {
	return len(r.Errors) == 0
}

func (r *ValidationResult) Error() string {
	if r.OK() {
		return ""
	}
	parts := make([]string, 0, len(r.Errors))
	for _, e := range r.Errors {
		parts = append(parts, e.Error())
	}
	return strings.Join(parts, "; ")
}
