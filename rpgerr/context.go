package rpgerr

import (
	"context"
)

// contextKey is a private type to avoid collisions
type contextKey string

const metadataKey contextKey = "rpgerr-metadata"

// MetadataScope holds accumulated metadata for errors
type MetadataScope struct {
	fields map[string]any
}

// MetaField represents a single metadata field
type MetaField struct {
	Key   string
	Value any
}

// Meta creates a metadata field for use with WithMetadata
func Meta(key string, value any) MetaField {
	return MetaField{Key: key, Value: value}
}

// WithMetadata adds metadata to context that will be automatically included
// in any errors created with Ctx functions. Metadata is inherited and can
// be overwritten by child contexts.
//
// Example:
//
//	ctx = rpgerr.WithMetadata(ctx,
//	    rpgerr.Meta("pipeline", "AttackPipeline"),
//	    rpgerr.Meta("attacker_id", "barbarian-1"),
//	    rpgerr.Meta("weapon", "greataxe"),
//	)
//	// Any errors created with WrapCtx will include this metadata
func WithMetadata(ctx context.Context, fields ...MetaField) context.Context {
	scope := &MetadataScope{
		fields: make(map[string]any),
	}

	// Inherit parent metadata if exists
	if parent, ok := ctx.Value(metadataKey).(*MetadataScope); ok && parent != nil {
		for k, v := range parent.fields {
			scope.fields[k] = v
		}
	}

	// Add new metadata (overwrites on conflict)
	for _, field := range fields {
		scope.fields[field.Key] = field.Value
	}

	return context.WithValue(ctx, metadataKey, scope)
}

// getMetadata extracts metadata from context
func getMetadata(ctx context.Context) map[string]any {
	if ctx == nil {
		return nil
	}

	if scope, ok := ctx.Value(metadataKey).(*MetadataScope); ok && scope != nil {
		return scope.fields
	}

	return nil
}

// applyContextMetadata applies metadata from context to an error
func applyContextMetadata(ctx context.Context, err *Error) *Error {
	if metadata := getMetadata(ctx); metadata != nil {
		for k, v := range metadata {
			if err.Meta == nil {
				err.Meta = make(map[string]any)
			}
			err.Meta[k] = v
		}
	}
	return err
}

// WrapCtx wraps an error with message and metadata from context
//
// Example:
//
//	if err != nil {
//	    return rpgerr.WrapCtx(ctx, err, "attack failed")
//	}
func WrapCtx(ctx context.Context, err error, message string) *Error {
	wrapped := Wrap(err, message)
	return applyContextMetadata(ctx, wrapped)
}

// WrapfCtx wraps an error with formatted message and metadata from context
func WrapfCtx(ctx context.Context, err error, format string, args ...any) *Error {
	wrapped := Wrapf(err, format, args...)
	return applyContextMetadata(ctx, wrapped)
}

// WrapWithCodeCtx wraps an error with specific code and metadata from context
func WrapWithCodeCtx(ctx context.Context, err error, code Code, message string) *Error {
	wrapped := WrapWithCode(err, code, message)
	return applyContextMetadata(ctx, wrapped)
}

// NewCtx creates a new error with code, message and metadata from context
func NewCtx(ctx context.Context, code Code, message string) *Error {
	err := New(code, message)
	return applyContextMetadata(ctx, err)
}

// NewfCtx creates a new error with formatted message and metadata from context
func NewfCtx(ctx context.Context, code Code, format string, args ...any) *Error {
	err := Newf(code, format, args...)
	return applyContextMetadata(ctx, err)
}

// Context-aware game rule error constructors

// NotAllowedCtx creates a not allowed error with metadata from context
func NotAllowedCtx(ctx context.Context, action string) *Error {
	err := NotAllowed(action)
	return applyContextMetadata(ctx, err)
}

// NotAllowedfCtx creates a formatted not allowed error with metadata from context
func NotAllowedfCtx(ctx context.Context, format string, args ...any) *Error {
	err := NotAllowedf(format, args...)
	return applyContextMetadata(ctx, err)
}

// PrerequisiteNotMetCtx creates a prerequisite error with metadata from context
func PrerequisiteNotMetCtx(ctx context.Context, requirement string) *Error {
	err := PrerequisiteNotMet(requirement)
	return applyContextMetadata(ctx, err)
}

// PrerequisiteNotMetfCtx creates a formatted prerequisite error with metadata from context
func PrerequisiteNotMetfCtx(ctx context.Context, format string, args ...any) *Error {
	err := PrerequisiteNotMetf(format, args...)
	return applyContextMetadata(ctx, err)
}

// ResourceExhaustedCtx creates a resource error with metadata from context
func ResourceExhaustedCtx(ctx context.Context, resource string) *Error {
	err := ResourceExhausted(resource)
	return applyContextMetadata(ctx, err)
}

// ResourceExhaustedfCtx creates a formatted resource error with metadata from context
func ResourceExhaustedfCtx(ctx context.Context, format string, args ...any) *Error {
	err := ResourceExhaustedf(format, args...)
	return applyContextMetadata(ctx, err)
}

// OutOfRangeCtx creates a range error with metadata from context
func OutOfRangeCtx(ctx context.Context, action string) *Error {
	err := OutOfRange(action)
	return applyContextMetadata(ctx, err)
}

// OutOfRangefCtx creates a formatted range error with metadata from context
func OutOfRangefCtx(ctx context.Context, format string, args ...any) *Error {
	err := OutOfRangef(format, args...)
	return applyContextMetadata(ctx, err)
}

// InvalidTargetCtx creates a target error with metadata from context
func InvalidTargetCtx(ctx context.Context, reason string) *Error {
	err := InvalidTarget(reason)
	return applyContextMetadata(ctx, err)
}

// InvalidTargetfCtx creates a formatted target error with metadata from context
func InvalidTargetfCtx(ctx context.Context, format string, args ...any) *Error {
	err := InvalidTargetf(format, args...)
	return applyContextMetadata(ctx, err)
}

// ConflictingStateCtx creates a state error with metadata from context
func ConflictingStateCtx(ctx context.Context, conflict string) *Error {
	err := ConflictingState(conflict)
	return applyContextMetadata(ctx, err)
}

// ConflictingStatefCtx creates a formatted state error with metadata from context
func ConflictingStatefCtx(ctx context.Context, format string, args ...any) *Error {
	err := ConflictingStatef(format, args...)
	return applyContextMetadata(ctx, err)
}

// TimingRestrictionCtx creates a timing error with metadata from context
func TimingRestrictionCtx(ctx context.Context, reason string) *Error {
	err := TimingRestriction(reason)
	return applyContextMetadata(ctx, err)
}

// TimingRestrictionfCtx creates a formatted timing error with metadata from context
func TimingRestrictionfCtx(ctx context.Context, format string, args ...any) *Error {
	err := TimingRestrictionf(format, args...)
	return applyContextMetadata(ctx, err)
}

// CooldownActiveCtx creates a cooldown error with metadata from context
func CooldownActiveCtx(ctx context.Context, ability string) *Error {
	err := CooldownActive(ability)
	return applyContextMetadata(ctx, err)
}

// CooldownActivefCtx creates a formatted cooldown error with metadata from context
func CooldownActivefCtx(ctx context.Context, format string, args ...any) *Error {
	err := CooldownActivef(format, args...)
	return applyContextMetadata(ctx, err)
}

// ImmuneCtx creates an immunity error with metadata from context
func ImmuneCtx(ctx context.Context, immunity string) *Error {
	err := Immune(immunity)
	return applyContextMetadata(ctx, err)
}

// ImmunefCtx creates a formatted immunity error with metadata from context
func ImmunefCtx(ctx context.Context, format string, args ...any) *Error {
	err := Immunef(format, args...)
	return applyContextMetadata(ctx, err)
}

// BlockedCtx creates a blocked error with metadata from context
func BlockedCtx(ctx context.Context, blocker string) *Error {
	err := Blocked(blocker)
	return applyContextMetadata(ctx, err)
}

// BlockedfCtx creates a formatted blocked error with metadata from context
func BlockedfCtx(ctx context.Context, format string, args ...any) *Error {
	err := Blockedf(format, args...)
	return applyContextMetadata(ctx, err)
}

// InterruptedCtx creates an interrupted error with metadata from context
func InterruptedCtx(ctx context.Context, interruptor string) *Error {
	err := Interrupted(interruptor)
	return applyContextMetadata(ctx, err)
}

// InterruptedfCtx creates a formatted interrupted error with metadata from context
func InterruptedfCtx(ctx context.Context, format string, args ...any) *Error {
	err := Interruptedf(format, args...)
	return applyContextMetadata(ctx, err)
}
