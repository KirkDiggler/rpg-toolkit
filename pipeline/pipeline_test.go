package pipeline_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/pipeline"
)

// TestSimplePipeline verifies basic pipeline execution.
func TestSimplePipeline(t *testing.T) {
	// Create a simple pipeline ref
	ref, err := core.ParseString("test:pipeline:simple")
	if err != nil {
		t.Fatalf("failed to parse ref: %v", err)
	}

	// Create stages that add numbers
	addFive := pipeline.NewStage("add-five", func(_ context.Context, input any) (any, error) {
		value := input.(int)
		return value + 5, nil
	})

	double := pipeline.NewStage("double", func(_ context.Context, input any) (any, error) {
		value := input.(int)
		return value * 2, nil
	})

	// Create sequential pipeline
	p := pipeline.Sequential(ref, addFive, double)

	// Execute pipeline: (10 + 5) * 2 = 30
	result := p.Process(context.Background(), 10)

	// Verify result
	if !result.IsComplete() {
		t.Fatal("expected pipeline to complete")
	}

	output := result.GetOutput().(int)
	if output != 30 {
		t.Errorf("expected 30, got %d", output)
	}
}

// TestPipelineWithData verifies pipelines can return data.
func TestPipelineWithData(t *testing.T) {
	ref, err := core.ParseString("test:pipeline:damage")
	if err != nil {
		t.Fatalf("failed to parse ref: %v", err)
	}

	// Create a damage stage that returns HP data
	damageStage := pipeline.NewStage("damage", func(_ context.Context, input any) (any, error) {
		damage := input.(int)

		// Return both output and data
		return pipeline.StageResult{
			Value: damage,
			Data: []pipeline.Data{
				&pipeline.HPData{
					EntityID: "goblin",
					Amount:   -damage,
				},
				&pipeline.LogData{
					Message: "Goblin takes damage",
				},
			},
		}, nil
	})

	// Create pipeline
	p := pipeline.Sequential(ref, damageStage)

	// Execute pipeline
	result := p.Process(context.Background(), 10)

	// Verify result
	if !result.IsComplete() {
		t.Fatal("expected pipeline to complete")
	}

	// Check data was returned
	data := result.GetData()
	if len(data) != 2 {
		t.Fatalf("expected 2 data items, got %d", len(data))
	}

	// Verify HP data
	hpData, ok := data[0].(*pipeline.HPData)
	if !ok {
		t.Fatal("expected HPData")
	}
	if hpData.EntityID != "goblin" {
		t.Errorf("expected goblin, got %s", hpData.EntityID)
	}
	if hpData.Amount != -10 {
		t.Errorf("expected -10 damage, got %d", hpData.Amount)
	}

	// Verify log data
	logData, ok := data[1].(*pipeline.LogData)
	if !ok {
		t.Fatal("expected LogData")
	}
	if logData.Message != "Goblin takes damage" {
		t.Errorf("unexpected log message: %s", logData.Message)
	}
}

// TestPipelineRegistry verifies the registry pattern.
func TestPipelineRegistry(t *testing.T) {
	registry := pipeline.NewRegistry()

	// Create a pipeline ref
	attackRef, err := core.ParseString("test:pipeline:attack")
	if err != nil {
		t.Fatalf("failed to parse ref: %v", err)
	}

	// Register a pipeline factory
	registry.Register(attackRef, pipeline.Func(func() pipeline.Pipeline {
		return pipeline.Sequential(attackRef,
			pipeline.NewStage("roll", func(_ context.Context, _ any) (any, error) {
				return 20, nil // Natural 20!
			}),
		)
	}))

	// Get pipeline from registry
	p, err := registry.Get(attackRef)
	if err != nil {
		t.Fatalf("failed to get pipeline: %v", err)
	}

	// Execute pipeline
	result := p.Process(context.Background(), nil)

	// Verify result
	if !result.IsComplete() {
		t.Fatal("expected pipeline to complete")
	}

	output := result.GetOutput().(int)
	if output != 20 {
		t.Errorf("expected 20, got %d", output)
	}
}

// TestNestedContext verifies context nesting.
func TestNestedContext(t *testing.T) {
	ctx := &pipeline.Context{
		Round:       1,
		CurrentTurn: "player",
		CallStack:   []string{"combat"},
	}

	// Create nested context
	nested := ctx.Nest("attack")

	// Verify nesting
	if nested.Parent != ctx {
		t.Error("expected parent to be set")
	}
	if nested.Depth != 1 {
		t.Errorf("expected depth 1, got %d", nested.Depth)
	}
	if len(nested.CallStack) != 2 {
		t.Errorf("expected 2 items in call stack, got %d", len(nested.CallStack))
	}
	if nested.CallStack[1] != "attack" {
		t.Errorf("expected 'attack' in call stack, got %s", nested.CallStack[1])
	}

	// Verify parent values are inherited
	if nested.Round != ctx.Round {
		t.Error("expected round to be inherited")
	}
	if nested.CurrentTurn != ctx.CurrentTurn {
		t.Error("expected current turn to be inherited")
	}
}
