package store

import (
	"errors"
	"fmt"
	"strings"
)

// Error is a teach-in failure (shown as MCP tool error text).
type Error struct {
	Msg  string
	Next string
}

func (e *Error) Error() string {
	if e.Next == "" {
		return e.Msg
	}
	return e.Msg + " Next: " + e.Next
}

func teach(msg, next string) error {
	return &Error{Msg: msg, Next: next}
}

// TeachText returns the user-facing message when err is a store.Error.
func TeachText(err error) (string, bool) {
	var te *Error
	if errors.As(err, &te) {
		return te.Error(), true
	}
	return "", false
}

func requireName(label, v string) (string, error) {
	s := strings.TrimSpace(v)
	if s == "" {
		return "", teach(label+" is required.", `roster_create(agent_name="Kit", user_name="Chris")`)
	}
	if len(s) > 40 {
		return "", teach(label+" is too long (max 40).", `roster_create(agent_name="Kit", user_name="Chris")`)
	}
	return s, nil
}

func formatOpenCap() string {
	return fmt.Sprintf("yard already has %d open challenges.", maxOpenChallenges)
}
