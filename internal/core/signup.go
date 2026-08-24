package core

import (
	"fmt"
	"strings"
)

// Signup decides what should happen for a signup. Pure: returns effects,
// performs nothing.
func Signup(email string) ([]Effect, error) {
	if !strings.Contains(email, "@") {
		return nil, fmt.Errorf("invalid email: %q", email)
	}
	return []Effect{
		Log{Line: "signup " + email},
		SendEmail{To: email, Body: "Welcome!"},
	}, nil
}
