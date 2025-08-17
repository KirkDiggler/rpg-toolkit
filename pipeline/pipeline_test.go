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

	// Create sequential pipeline with int input and int output
	p := pipeline.Sequential[int, int](ref, addFive, double)

	// Execute pipeline: (10 + 5) * 2 = 30
	result := p.Process(context.Background(), 10)

	// Verify result
	if !result.IsComplete() {
		t.Fatal("expected pipeline to complete")
	}

	output := result.GetOutput()
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

	// Create pipeline with int input and int output
	p := pipeline.Sequential[int, int](ref, damageStage)

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

// Define typed input/output for attack pipeline
type AttackInput struct {
	Attacker string
	Target   string
	Bonus    int
}

type AttackOutput struct {
	Hit    bool
	Damage int
}

// TestTypedPipeline verifies typed pipeline execution.
func TestTypedPipeline(t *testing.T) {
	ref, err := core.ParseString("test:pipeline:attack")
	if err != nil {
		t.Fatalf("failed to parse ref: %v", err)
	}

	// Create an attack stage with typed transformation
	attackStage := pipeline.NewStage("attack", func(_ context.Context, input any) (any, error) {
		attack := input.(AttackInput)

		// Simulate attack roll (fixed for test)
		roll := 15 + attack.Bonus
		hit := roll >= 10 // AC 10

		var damage int
		if hit {
			damage = 8 // Fixed damage for test
		}

		return AttackOutput{
			Hit:    hit,
			Damage: damage,
		}, nil
	})

	// Create typed pipeline
	p := pipeline.Sequential[AttackInput, AttackOutput](ref, attackStage)

	// Execute pipeline
	result := p.Process(context.Background(), AttackInput{
		Attacker: "fighter",
		Target:   "goblin",
		Bonus:    5,
	})

	// Verify result
	if !result.IsComplete() {
		t.Fatal("expected pipeline to complete")
	}

	output := result.GetOutput()
	if !output.Hit {
		t.Error("expected attack to hit")
	}
	if output.Damage != 8 {
		t.Errorf("expected 8 damage, got %d", output.Damage)
	}
}

// TestPipelineRegistry verifies the registry pattern with typed pipelines.
func TestPipelineRegistry(t *testing.T) {
	registry := pipeline.NewRegistry()

	// Create a pipeline ref
	attackRef, err := core.ParseString("test:pipeline:attack")
	if err != nil {
		t.Fatalf("failed to parse ref: %v", err)
	}

	// Register a typed pipeline factory
	registry.Register(attackRef, pipeline.Func[AttackInput, AttackOutput](func() pipeline.Pipeline[AttackInput, AttackOutput] {
		return pipeline.Sequential[AttackInput, AttackOutput](attackRef,
			pipeline.NewStage("roll", func(_ context.Context, _ any) (any, error) {
				return AttackOutput{
					Hit:    true, // Always hits for test
					Damage: 20,   // Natural 20!
				}, nil
			}),
		)
	}))

	// Get pipeline from registry with proper types
	p, err := pipeline.Get[AttackInput, AttackOutput](registry, attackRef)
	if err != nil {
		t.Fatalf("failed to get pipeline: %v", err)
	}

	// Execute pipeline
	result := p.Process(context.Background(), AttackInput{
		Attacker: "barbarian",
		Target:   "orc",
		Bonus:    7,
	})

	// Verify result
	if !result.IsComplete() {
		t.Fatal("expected pipeline to complete")
	}

	output := result.GetOutput()
	if !output.Hit {
		t.Error("expected hit")
	}
	if output.Damage != 20 {
		t.Errorf("expected 20 damage, got %d", output.Damage)
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