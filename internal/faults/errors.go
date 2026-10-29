package faults

import (
	"errors"
)

var (
	ErrNotFound  = errors.New("chatstore: not found")
	ErrNotMember = errors.New("chatstore: not a group member")
	ErrForbidden = ErrNotMember
	ErrConflict  = errors.New("chatstore: already exists")
	ErrInvalid   = errors.New("chatstore: invalid input")
	ErrLimit     = errors.New("chatstore: limit exceeded")
	ErrMuted     = errors.New("chatstore: muted in room")
	ErrBlocked   = errors.New("chatstore: invite recipient blocks actor")
)

// ErrorKind describes an application failure without selecting an HTTP status.
type ErrorKind string

const (
	Invalid         ErrorKind = "invalid"
	Unauthenticated ErrorKind = "unauthenticated"
	Forbidden       ErrorKind = "forbidden"
	NotFound        ErrorKind = "not_found"
	Conflict        ErrorKind = "conflict"
	Unavailable     ErrorKind = "unavailable"
	Limited         ErrorKind = "limited"
	Internal        ErrorKind = "internal"
)

type ServiceError struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

func (e *ServiceError) Error() string { return e.Message }
func (e *ServiceError) Unwrap() error { return e.Cause }
func NewError(kind ErrorKind, message string) error {
	return &ServiceError{Kind: kind, Message: message}
}

// WrapError keeps the public error message separate from its underlying cause.
// Callers can still identify cancellation and adapter errors with errors.Is/As.
func WrapError(kind ErrorKind, message string, cause error) error {
	return &ServiceError{Kind: kind, Message: message, Cause: cause}
}
