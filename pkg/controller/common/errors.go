// Package common provides shared utilities and types used across controllers.
package common

import (
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// ErrorReason represents the category of a reconciliation error,
// used to determine whether the reconciler should retry or not.
type ErrorReason string

const (
	// IrrecoverableError indicates an error that cannot be resolved by retrying.
	// Examples include invalid configuration, permission errors, or bad requests.
	// The reconciler should not requeue when encountering this error type.
	IrrecoverableError ErrorReason = "IrrecoverableError"

	// RetryRequiredError indicates a transient error that may be resolved by retrying.
	// Examples include temporary network issues or resource conflicts.
	// The reconciler should requeue when encountering this error type.
	RetryRequiredError ErrorReason = "RetryRequiredError"
)

// ReconcileError represents an error that occurred during reconciliation.
// It includes the error reason, a descriptive message, and the underlying error.
type ReconcileError struct {
	// Reason categorizes the error as either irrecoverable or requiring retry.
	Reason ErrorReason `json:"reason,omitempty"`
	// Message provides a human-readable description of the error context.
	Message string `json:"message,omitempty"`
	// Err is the underlying error that caused this reconciliation error.
	Err error `json:"error,omitempty"`
}

// Ensure ReconcileError implements the error interface.
var _ error = &ReconcileError{}

// NewIrrecoverableError creates a new ReconcileError with IrrecoverableError reason.
// Returns nil if the provided error is nil.
// The message supports fmt.Sprintf-style formatting with the provided args.
func NewIrrecoverableError(err error, message string, args ...any) *ReconcileError {
	if err == nil {
		return nil
	}
	return &ReconcileError{
		Reason:  IrrecoverableError,
		Message: fmt.Sprintf(message, args...),
		Err:     err,
	}
}

// NewRetryRequiredError creates a new ReconcileError with RetryRequiredError reason.
// Returns nil if the provided error is nil.
// The message supports fmt.Sprintf-style formatting with the provided args.
func NewRetryRequiredError(err error, message string, args ...any) *ReconcileError {
	if err == nil {
		return nil
	}
	return &ReconcileError{
		Reason:  RetryRequiredError,
		Message: fmt.Sprintf(message, args...),
		Err:     err,
	}
}

// IsIrrecoverableError checks if the given error is a ReconcileError
// with IrrecoverableError reason. Returns false if err is nil or
// not a ReconcileError.
func IsIrrecoverableError(err error) bool {
	rerr := &ReconcileError{}
	if errors.As(err, &rerr) {
		return rerr.Reason == IrrecoverableError
	}
	return false
}

// Error implements the error interface, returning a formatted string
// containing both the message and the underlying error.
func (e *ReconcileError) Error() string {
	return fmt.Sprintf("%s: %s", e.Message, e.Err)
}

// FromClientError creates a ReconcileError from a Kubernetes API client error.
// It automatically determines the error reason based on the API error type:
//   - IrrecoverableError: Unauthorized, Forbidden, Invalid, BadRequest, ServiceUnavailable
//   - RetryRequiredError: All other errors (e.g., NotFound, Conflict, Timeout)
//
// Returns nil if the provided error is nil.
// The message supports fmt.Sprintf-style formatting with the provided args.
func FromClientError(err error, message string, args ...any) *ReconcileError {
	if err == nil {
		return nil
	}
	if apierrors.IsUnauthorized(err) || apierrors.IsForbidden(err) || apierrors.IsInvalid(err) ||
		apierrors.IsBadRequest(err) || apierrors.IsServiceUnavailable(err) {
		return NewIrrecoverableError(err, message, args...)
	}

	return NewRetryRequiredError(err, message, args...)
}
