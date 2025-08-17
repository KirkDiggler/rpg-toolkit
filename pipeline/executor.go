package pipeline

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Sequential creates a pipeline that executes stages in order.
func Sequential(ref *core.Ref, stages ...Stage) Pipeline {
	return &sequentialPipeline{
		ref:    ref,
		stages: stages,
	}
}

type sequentialPipeline struct {
	ref    *core.Ref
	stages []Stage
}

// GetRef returns the pipeline's reference.
func (p *sequentialPipeline) GetRef() *core.Ref {
	return p.ref
}

// Process executes all stages in sequence.
func (p *sequentialPipeline) Process(ctx context.Context, input any) Result {
	value := input
	data := []Data{}

	// Execute each stage
	for i, stage := range p.stages {
		output, err := stage.Process(ctx, value)
		if err != nil {
			// For now, return error as completed with no output
			// In future, could handle errors differently
			return CompletedResult{
				Output: nil,
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

	return CompletedResult{
		Output: value,
		Data:   data,
	}
}

// Resume is not yet implemented for sequential pipelines.
func (p *sequentialPipeline) Resume(_ ContinuationData, _ any) Result {
	// TODO: Implement resumption
	return CompletedResult{
		Output: nil,
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

// Func adapts a function to the PipelineFactory interface.
type Func func() Pipeline

// Create calls the function to create a pipeline.
func (f Func) Create() Pipeline {
	return f()
}

// StaticFactory always returns the same pipeline instance.
type StaticFactory struct {
	pipeline Pipeline
}

// NewStaticFactory creates a factory that returns a pre-created pipeline.
func NewStaticFactory(p Pipeline) Factory {
	return &StaticFactory{pipeline: p}
}

// Create returns the static pipeline.
func (f *StaticFactory) Create() Pipeline {
	return f.pipeline
}

// Error creates a result representing an error.
func Error(err error, data []Data) Result {
	return CompletedResult{
		Output: fmt.Errorf("pipeline error: %w", err),
		Data:   data,
	}
}
