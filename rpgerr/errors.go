// Package rpgerr provides structured error handling for RPG game mechanics.
// It enables clear communication of why game actions cannot proceed, with full
// context about the game state when rules are evaluated.
package rpgerr

import (
	"context"
	"errors"
	"fmt"
)

// Code represents a game rule or system error code that explains why an action failed
type Code string

const (
	// CodeUnknown indicates an unknown error occurred
	CodeUnknown Code = "unknown"
	// CodeInternal indicates an internal system error
	CodeInternal Code = "internal"
	// CodeCanceled indicates the operation was canceled
	CodeCanceled Code = "canceled"

	// CodeNotAllowed indicates action not permitted by game rules
	CodeNotAllowed Code = "not_allowed"
	// CodePrerequisiteNotMet indicates missing requirements (level, class, feat)
	CodePrerequisiteNotMet Code = "prerequisite_not_met"
	// CodeResourceExhausted indicates out of resources (HP, spell slots, energy, actions)
	CodeResourceExhausted Code = "resource_exhausted"
	// CodeOutOfRange indicates target too far away
	CodeOutOfRange Code = "out_of_range"
	// CodeInvalidTarget indicates cannot target that entity
	CodeInvalidTarget Code = "invalid_target"
	// CodeConflictingState indicates states conflict (rage + concentration)
	CodeConflictingState Code = "conflicting_state"
	// CodeTimingRestriction indicates wrong phase/turn for this action
	CodeTimingRestriction Code = "timing_restriction"
	// CodeCapacityExceeded indicates too many items, effects, etc.
	CodeCapacityExceeded Code = "capacity_exceeded"
	// CodeCooldownActive indicates ability still on cooldown
	CodeCooldownActive Code = "cooldown_active"
	// CodeImmune indicates target immune to this effect
	CodeImmune Code = "immune"
	// CodeBlocked indicates action blocked by another effect
	CodeBlocked Code = "blocked"
	// CodeInterrupted indicates action interrupted by reaction/trigger
	CodeInterrupted Code = "interrupted"
	// CodeInvalidState indicates entity in wrong state for action
	CodeInvalidState Code = "invalid_state"
	// CodeNotFound indicates requested entity/resource not found
	CodeNotFound Code = "not_found"
	// CodeAlreadyExists indicates entity/resource already exists
	CodeAlreadyExists Code = "already_exists"
	// CodeInvalidArgument indicates invalid input provided
	CodeInvalidArgument Code = "invalid_argument"
)

// Error represents a game error with code, message, and metadata
type Error struct {
	// Code categorizes the error type
	Code Code

	// Message describes what happened
	Message string

	// Cause is the wrapped error if any
	Cause error

	// Meta contains game state context
	Meta map[string]any

	// CallStack tracks execution path through nested systems
	CallStack []string
}

// Error returns the error message
func (e *Error) Error() string {
	if e == nil {
		return "rpgerr: nil error"
	}

	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}

	return e.Message
}

// Unwrap returns the wrapped error
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Option is a functional option for configuring errors
type Option func(*Error)

// WithMeta adds metadata to the error
func WithMeta(key string, value any) Option {
	return func(e *Error) {
		if e.Meta == nil {
			e.Meta = make(map[string]any)
		}
		e.Meta[key] = value
	}
}

// WithCallStack sets the call stack for the error
func WithCallStack(stack []string) Option {
	return func(e *Error) {
		e.CallStack = stack
	}
}

// AddToCallStack appends to the call stack
func AddToCallStack(frame string) Option {
	return func(e *Error) {
		e.CallStack = append(e.CallStack, frame)
	}
}

// New creates a new error with the given code and message
func New(code Code, message string, opts ...Option) *Error {
	err := &Error{
		Code:    code,
		Message: message,
	}

	for _, opt := range opts {
		opt(err)
	}

	return err
}

// Newf creates a new error with formatted message
func Newf(code Code, format string, args ...any) *Error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// NewfWithOpts creates a new error with formatted message and options
func NewfWithOpts(code Code, opts []Option, format string, args ...any) *Error {
	err := &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}

	for _, opt := range opts {
		opt(err)
	}

	return err
}

// Wrap wraps an error with additional context
func Wrap(err error, message string, opts ...Option) *Error {
	if err == nil {
		return New(CodeInternal, fmt.Sprintf("rpgerr.Wrap called with nil: %s", message))
	}

	var wrapped *Error

	// Preserve code if it's already our error type
	var rpgErr *Error
	if errors.As(err, &rpgErr) {
		wrapped = &Error{
			Code:      rpgErr.Code,
			Message:   message,
			Cause:     err,
			Meta:      copyMeta(rpgErr.Meta),
			CallStack: copyCallStack(rpgErr.CallStack),
		}
	} else {
		wrapped = &Error{
			Code:    CodeUnknown,
			Message: message,
			Cause:   err,
		}
	}

	for _, opt := range opts {
		opt(wrapped)
	}

	return wrapped
}

// Wrapf wraps an error with formatted message
func Wrapf(err error, format string, args ...any) *Error {
	return Wrap(err, fmt.Sprintf(format, args...))
}

// WrapWithCode wraps an error with a specific code
func WrapWithCode(err error, code Code, message string, opts ...Option) *Error {
	if err == nil {
		return New(CodeInternal, fmt.Sprintf("rpgerr.WrapWithCode called with nil: %s", message))
	}

	var meta map[string]any
	var stack []string

	// Preserve metadata and stack if it's our error
	var rpgErr *Error
	if errors.As(err, &rpgErr) {
		meta = copyMeta(rpgErr.Meta)
		stack = copyCallStack(rpgErr.CallStack)
	}

	wrapped := &Error{
		Code:      code,
		Message:   message,
		Cause:     err,
		Meta:      meta,
		CallStack: stack,
	}

	for _, opt := range opts {
		opt(wrapped)
	}

	return wrapped
}

// copyMeta creates a shallow copy of metadata
func copyMeta(meta map[string]any) map[string]any {
	if meta == nil {
		return nil
	}

	copied := make(map[string]any, len(meta))
	for k, v := range meta {
		copied[k] = v
	}
	return copied
}

// copyCallStack creates a copy of the call stack
func copyCallStack(stack []string) []string {
	if stack == nil {
		return nil
	}

	copied := make([]string, len(stack))
	copy(copied, stack)
	return copied
}

// GetCode extracts the error code from any error
func GetCode(err error) Code {
	var rpgErr *Error
	if errors.As(err, &rpgErr) {
		if rpgErr == nil {
			return CodeUnknown
		}

		// Check standard context errors
		if rpgErr.Code == CodeUnknown {
			if errors.Is(err, context.Canceled) {
				return CodeCanceled
			}
		}

		return rpgErr.Code
	}

	// Check standard errors
	switch {
	case errors.Is(err, context.Canceled):
		return CodeCanceled
	default:
		return CodeUnknown
	}
}

// GetMeta extracts metadata from an error
func GetMeta(err error) map[string]any {
	var rpgErr *Error
	if errors.As(err, &rpgErr) && rpgErr != nil {
		return rpgErr.Meta
	}
	return nil
}

// GetCallStack extracts the call stack from an error
func GetCallStack(err error) []string {
	var rpgErr *Error
	if errors.As(err, &rpgErr) && rpgErr != nil {
		return rpgErr.CallStack
	}
	return nil
}

// Common game rule error constructors

// NotAllowed creates an error for actions not permitted by game rules
func NotAllowed(action string, opts ...Option) *Error {
	return New(CodeNotAllowed, fmt.Sprintf("%s not allowed", action), opts...)
}

// NotAllowedf creates a formatted not allowed error
func NotAllowedf(format string, args ...any) *Error {
	return Newf(CodeNotAllowed, format, args...)
}

// PrerequisiteNotMet creates an error for missing requirements
func PrerequisiteNotMet(requirement string, opts ...Option) *Error {
	return New(CodePrerequisiteNotMet, fmt.Sprintf("prerequisite not met: %s", requirement), opts...)
}

// PrerequisiteNotMetf creates a formatted prerequisite error
func PrerequisiteNotMetf(format string, args ...any) *Error {
	return Newf(CodePrerequisiteNotMet, format, args...)
}

// ResourceExhausted creates an error for depleted resources
func ResourceExhausted(resource string, opts ...Option) *Error {
	return New(CodeResourceExhausted, fmt.Sprintf("insufficient %s", resource), opts...)
}

// ResourceExhaustedf creates a formatted resource exhausted error
func ResourceExhaustedf(format string, args ...any) *Error {
	return Newf(CodeResourceExhausted, format, args...)
}

// OutOfRange creates an error for range restrictions
func OutOfRange(action string, opts ...Option) *Error {
	return New(CodeOutOfRange, fmt.Sprintf("%s out of range", action), opts...)
}

// OutOfRangef creates a formatted out of range error
func OutOfRangef(format string, args ...any) *Error {
	return Newf(CodeOutOfRange, format, args...)
}

// InvalidTarget creates an error for invalid targeting
func InvalidTarget(reason string, opts ...Option) *Error {
	return New(CodeInvalidTarget, fmt.Sprintf("invalid target: %s", reason), opts...)
}

// InvalidTargetf creates a formatted invalid target error
func InvalidTargetf(format string, args ...any) *Error {
	return Newf(CodeInvalidTarget, format, args...)
}

// ConflictingState creates an error for conflicting game states
func ConflictingState(conflict string, opts ...Option) *Error {
	return New(CodeConflictingState, fmt.Sprintf("conflicting state: %s", conflict), opts...)
}

// ConflictingStatef creates a formatted conflicting state error
func ConflictingStatef(format string, args ...any) *Error {
	return Newf(CodeConflictingState, format, args...)
}

// TimingRestriction creates an error for timing violations
func TimingRestriction(reason string, opts ...Option) *Error {
	return New(CodeTimingRestriction, fmt.Sprintf("timing restriction: %s", reason), opts...)
}

// TimingRestrictionf creates a formatted timing restriction error
func TimingRestrictionf(format string, args ...any) *Error {
	return Newf(CodeTimingRestriction, format, args...)
}

// CooldownActive creates an error for abilities on cooldown
func CooldownActive(ability string, opts ...Option) *Error {
	return New(CodeCooldownActive, fmt.Sprintf("%s on cooldown", ability), opts...)
}

// CooldownActivef creates a formatted cooldown error
func CooldownActivef(format string, args ...any) *Error {
	return Newf(CodeCooldownActive, format, args...)
}

// Immune creates an error for immunity
func Immune(immunity string, opts ...Option) *Error {
	return New(CodeImmune, fmt.Sprintf("immune to %s", immunity), opts...)
}

// Immunef creates a formatted immunity error
func Immunef(format string, args ...any) *Error {
	return Newf(CodeImmune, format, args...)
}

// Blocked creates an error for blocked actions
func Blocked(blocker string, opts ...Option) *Error {
	return New(CodeBlocked, fmt.Sprintf("blocked by %s", blocker), opts...)
}

// Blockedf creates a formatted blocked error
func Blockedf(format string, args ...any) *Error {
	return Newf(CodeBlocked, format, args...)
}

// Interrupted creates an error for interrupted actions
func Interrupted(interruptor string, opts ...Option) *Error {
	return New(CodeInterrupted, fmt.Sprintf("interrupted by %s", interruptor), opts...)
}

// Interruptedf creates a formatted interrupted error
func Interruptedf(format string, args ...any) *Error {
	return Newf(CodeInterrupted, format, args...)
}

// Helper functions for checking error codes

// IsNotAllowed checks if error is CodeNotAllowed
func IsNotAllowed(err error) bool {
	return GetCode(err) == CodeNotAllowed
}

// IsPrerequisiteNotMet checks if error is CodePrerequisiteNotMet
func IsPrerequisiteNotMet(err error) bool {
	return GetCode(err) == CodePrerequisiteNotMet
}

// IsResourceExhausted checks if error is CodeResourceExhausted
func IsResourceExhausted(err error) bool {
	return GetCode(err) == CodeResourceExhausted
}

// IsOutOfRange checks if error is CodeOutOfRange
func IsOutOfRange(err error) bool {
	return GetCode(err) == CodeOutOfRange
}

// IsInvalidTarget checks if error is CodeInvalidTarget
func IsInvalidTarget(err error) bool {
	return GetCode(err) == CodeInvalidTarget
}

// IsConflictingState checks if error is CodeConflictingState
func IsConflictingState(err error) bool {
	return GetCode(err) == CodeConflictingState
}

// IsTimingRestriction checks if error is CodeTimingRestriction
func IsTimingRestriction(err error) bool {
	return GetCode(err) == CodeTimingRestriction
}

// IsCooldownActive checks if error is CodeCooldownActive
func IsCooldownActive(err error) bool {
	return GetCode(err) == CodeCooldownActive
}

// IsImmune checks if error is CodeImmune
func IsImmune(err error) bool {
	return GetCode(err) == CodeImmune
}

// IsBlocked checks if error is CodeBlocked
func IsBlocked(err error) bool {
	return GetCode(err) == CodeBlocked
}

// IsInterrupted checks if error is CodeInterrupted
func IsInterrupted(err error) bool {
	return GetCode(err) == CodeInterrupted
}
