package main

import (
	"errors"
	"fmt"
	"strings"
)

// usageError is a bad flag, argument, or command. main exits 2.
// A scan, config, or write failure is a plain error and exits 1.
type usageError struct {
	err error
}

func (e *usageError) Error() string {
	if e == nil || e.err == nil {
		return "usage"
	}
	return e.err.Error()
}

func (e *usageError) Unwrap() error { return e.err }

func usagef(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...)}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if isUsage(err) {
		return 2
	}
	return 1
}

func isUsage(err error) bool {
	var usage *usageError
	if errors.As(err, &usage) {
		return true
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "unknown command"),
		strings.HasPrefix(msg, "unknown flag"),
		strings.HasPrefix(msg, "unknown shorthand"),
		strings.HasPrefix(msg, "accepts "),
		strings.HasPrefix(msg, "requires "):
		return true
	default:
		return false
	}
}
