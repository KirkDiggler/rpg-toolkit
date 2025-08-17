package pipeline

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Sequential creates a pipeline that executes stages in order.
func Sequential[I, O any](ref *core.Ref, stages ...Stage) Pipeline[I, O] {
	return &sequentialPipeline[I, O]{
		ref:    ref,
		stages: stages,
	}
}

type sequentialPipeline[I, O any] struct {
	ref    *core.Ref
	stages []Stage
}

// GetRef returns the pipeline's reference.
func (p *sequentialPipeline[I, O]) GetRef() *core.Ref {
	return p.ref
}

// Process executes all stages in sequence.
func (p *sequentialPipeline[I, O]) Process(ctx context.Context, input I) Result[O] {
	var value any = input
	data := []Data{}

	// Execute each stage
	for i, stage := range p.stages {
		output, err := stage.Process(ctx, value)
		if err != nil {
			// For now, return error as completed with zero output
			var zero O
			return CompletedResult[O]{
				Output: zero,
				Data:   data,
			}
		}

		// Check if stage returned data
		if stageData, ok := output.(StageResult); ok {
			value = stageData.Value
			data = append(data, stageData.Data...)
		} else {
			value = output
		}

		// Check for suspension (future feature)
		// For now, always continue
		_ = i
	}

	// Type assert the final value to output type
	finalOutput, ok := value.(O)
	if !ok {
		var zero O
		return CompletedResult[O]{
			Output: zero,
			Data:   data,
		}
	}

	return CompletedResult[O]{
		Output: finalOutput,
		Data:   data,
	}
}

// Resume is not yet implemented for sequential pipelines.
func (p *sequentialPipeline[I, O]) Resume(_ ContinuationData, _ any) Result[O] {
	// TODO: Implement resumption
	var zero O
	return CompletedResult[O]{
		Output: zero,
		Data:   []Data{},
	}
}

// StageResult allows stages to return both a value and data.
type StageResult struct {
	Value any    // The transformed value
	Data  []Data // State changes from this stage
}

// SimpleStage provides a base implementation for stages.
type SimpleStage struct {
	name string
	fn   func(context.Context, any) (any, error)
}

// NewStage creates a simple stage with a transformation function.
func NewStage(name string, fn func(context.Context, any) (any, error)) Stage {
	return &SimpleStage{
		name: name,
		fn:   fn,
	}
}

// Name returns the stage name.
func (s *SimpleStage) Name() string {
	return s.name
}

// Process executes the stage's transformation.
func (s *SimpleStage) Process(ctx context.Context, value any) (any, error) {
	return s.fn(ctx, value)
}

// Func adapts a function to the Factory interface.
type Func[I, O any] func() Pipeline[I, O]

// Create calls the function to create a pipeline.
func (f Func[I, O]) Create() Pipeline[I, O] {
	return f()
}

// StaticFactory always returns the same pipeline instance.
type StaticFactory[I, O any] struct {
	pipeline Pipeline[I, O]
}

// NewStaticFactory creates a factory that returns a pre-created pipeline.
func NewStaticFactory[I, O any](p Pipeline[I, O]) Factory[I, O] {
	return &StaticFactory[I, O]{pipeline: p}
}

// Create returns the static pipeline.
func (f *StaticFactory[I, O]) Create() Pipeline[I, O] {
	return f.pipeline
}

// Error creates a result representing an error.
func Error[O any](err error, data []Data) Result[O] {
	var zero O
	return CompletedResult[O]{
		Output: zero,
		Data:   data,
	}
}