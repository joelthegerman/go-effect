package core

import "fmt"

// ValidationError is a domain-level rejection: the input is not something the
// business logic considers valid (empty title, over-long title, …). It is the
// kind of check the logic wants for its OWN correctness, so it lives in core.
//
// The API layer maps this to HTTP 422. Guardrails imposed ON the code (not by
// it) live in a feature's shell policy instead — see internal/shell/todos/policy.go.
type ValidationError struct {
	Field string
	Msg   string
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return e.Msg
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Msg)
}

// Invalid is a small constructor for a ValidationError, exported so feature
// logic packages (core/todos, …) can build one without redeclaring the type.
func Invalid(field, msg string) error { return ValidationError{Field: field, Msg: msg} }
