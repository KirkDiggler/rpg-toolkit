// Package pipeline provides the core abstraction for game mechanics as pure transformations.
// Pipelines transform input to output through stages, returning data for the game server to persist.
package pipeline

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Pipeline represents a game mechanic transformation.
// Pipelines are pure functions that return data, not side effects.
type Pipeline[I, O any] interface {
	// GetRef returns the pipeline's unique reference
	GetRef() *core.Ref

	// Process executes the pipeline with the given input
	Process(ctx context.Context, input I) Result[O]

	// Resume continues a suspended pipeline with a decision
	Resume(continuation ContinuationData, decision any) Result[O]
}

// Result represents the output of a pipeline execution.
// Can be either completed or suspended awaiting a decision.
type Result[O any] interface {
	// IsComplete returns true if the pipeline finished
	IsComplete() bool

	// GetData returns state changes to apply
	GetData() []Data

	// GetOutput returns the pipeline's output (if complete)
	GetOutput() O

	// GetContinuation returns suspension data (if suspended)
	GetContinuation() *ContinuationData
}

// Stage transforms a value as part of a pipeline.
type Stage interface {
	// Name returns the stage's name for debugging
	Name() string

	// Process transforms the input value
	Process(ctx context.Context, value any) (any, error)
}

// Context carries execution state through pipelines.
type Context struct {
	// Registry for accessing other pipelines
	Registry *Registry

	// Game state
	Round       int
	CurrentTurn string

	// Nesting support
	Parent    *Context
	Depth     int
	CallStack []string
}

// Nest creates a child context for nested pipeline execution.
func (c *Context) Nest(name string) *Context {
	return &Context{
		Registry:    c.Registry,
		Round:       c.Round,
		CurrentTurn: c.CurrentTurn,
		Parent:      c,
		Depth:       c.Depth + 1,
		CallStack:   append(c.CallStack, name),
	}
}

// Data represents a state change to be applied.
type Data interface {
	// GetEntityID returns the entity this data affects
	GetEntityID() string

	// GetOperation returns the type of operation
	GetOperation() DataOperation

	// Apply applies the data to a store
	Apply(store any) error
}

// DataOperation defines how data should be applied.
type DataOperation string

const (
	// OpUpdate modifies existing data
	OpUpdate DataOperation = "update"

	// OpAppend adds to a collection
	OpAppend DataOperation = "append"

	// OpRemove deletes data
	OpRemove DataOperation = "remove"

	// OpReplace replaces data entirely
	OpReplace DataOperation = "replace"
)

// ContinuationData holds the state of a suspended pipeline.
type ContinuationData struct {
	// PipelineRef identifies which pipeline to resume
	PipelineRef string `json:"pipeline_ref"`

	// Stage is the stage index where we suspended
	Stage int `json:"stage"`

	// Input is the original pipeline input
	Input any `json:"input"`

	// Intermediate is the state at suspension
	Intermediate any `json:"intermediate"`

	// Context is the execution context
	Context map[string]any `json:"context"`
}
