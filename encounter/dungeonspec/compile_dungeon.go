package dungeonspec

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// CompileMode selects structural authoring preview or runnable compilation.
type CompileMode string

const (
	// CompileModeDraft permits structurally valid non-runnable projections.
	CompileModeDraft CompileMode = "draft"
	// CompileModeStrict requires a runnable candidate.
	CompileModeStrict CompileMode = "strict"
)

const fieldCanvasFloorSource = "canvas.floor_source"

// FieldError is ordered provider-authored validation feedback.
type FieldError struct{ Field, Message, Code string }

// CompileDungeonInput is one complete standalone authored candidate.
type CompileDungeonInput struct {
	Source              []byte
	Mode                CompileMode
	PartyStartSeatCount int
	PreviewSeed         int64
}

// CompileDungeonOutput is either a complete result or FieldErrors. Infrastructure
// failures use CompileDungeon's Go error return.
type CompileDungeonOutput struct {
	Compiled    CompiledDungeon
	FloorPlan   *FloorPlan
	FieldErrors []FieldError
}

// CompileDungeon is the protobuf-free complete-candidate provider seam used by
// API adapters. It never accepts previous compiled state or source metadata.
func CompileDungeon(ctx context.Context, in CompileDungeonInput) (*CompileDungeonOutput, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if in.Mode != CompileModeDraft && in.Mode != CompileModeStrict {
		return nil, fmt.Errorf("dungeonspec: unknown compile mode %q", in.Mode)
	}
	if in.PartyStartSeatCount < 1 {
		return nil, fmt.Errorf("dungeonspec: PartyStartSeatCount must be at least 1")
	}
	spec, err := Decode(in.Source)
	if err != nil {
		return validationOutput(fieldErrorFor(err)), nil
	}
	if err := Validate(spec); err != nil {
		return validationOutput(fieldErrorFor(err)), nil
	}
	compiled, err := compileWithConfig(spec, LoadConfig{PartyStartSeatCount: in.PartyStartSeatCount})
	if err != nil {
		return nil, fmt.Errorf("dungeonspec: compile candidate: %w", err)
	}
	if in.Mode == CompileModeStrict && compiled.canvas != nil && compiled.canvas.floorSource == FloorSourceRegions {
		if validation := strictRegionValidation(compiled); validation != nil {
			return validationOutput(*validation), nil
		}
		if err := validateCompiledRuntime(ctx, compiled, in.PreviewSeed); err != nil {
			return validationOutput(fieldErrorFor(err)), nil
		}
	}
	plan, err := BuildFloorPlan(ctx, BuildFloorPlanInput{Compiled: compiled, Seed: in.PreviewSeed})
	if err != nil {
		// Init/generator rejections describe authored content for this complete
		// candidate; cancellations and provider failures remain Go errors.
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return validationOutput(fieldErrorFor(err)), nil
	}
	return &CompileDungeonOutput{Compiled: compiled, FloorPlan: &plan}, nil
}

func validationOutput(field FieldError) *CompileDungeonOutput {
	return &CompileDungeonOutput{FieldErrors: []FieldError{field}}
}

func strictRegionValidation(compiled CompiledDungeon) *FieldError {
	canvas := compiled.canvas
	if len(canvas.floorCells) == 0 {
		return &FieldError{
			Field:   fieldCanvasFloorSource,
			Message: "region-union floor must not be empty in strict mode",
			Code:    "floor_empty",
		}
	}
	if canvas.entrance == nil {
		field := fieldCanvasFloorSource
		if compiled.Params.PartyStart.Anchor != nil {
			field = "start"
		}
		return &FieldError{
			Field: field, Message: "no floor anchor has a complete same-component party start envelope",
			Code: "entrance_unavailable",
		}
	}
	floor := make(map[[2]int]struct{}, len(canvas.floorCells))
	for _, cell := range canvas.floorCells {
		floor[[2]int{cell.Column, cell.Row}] = struct{}{}
	}
	anchor := [2]int{canvas.entrance.Column, canvas.entrance.Row}
	component := map[[2]int]struct{}{anchor: {}}
	queue := [][2]int{anchor}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		h := canvasHex(FloorPlanCell{Column: current[0], Row: current[1]})
		for _, cube := range h.ToCube().GetNeighbors() {
			pos := core.HexFromCube(cube).ToPosition()
			next := [2]int{int(pos.X), int(pos.Y)}
			if _, ok := floor[next]; !ok {
				continue
			}
			if _, seen := component[next]; seen {
				continue
			}
			component[next] = struct{}{}
			queue = append(queue, next)
		}
	}
	if len(component) != len(floor) {
		return &FieldError{
			Field: fieldCanvasFloorSource, Message: "region-union floor must be connected in strict mode",
			Code: "floor_disconnected",
		}
	}
	return nil
}

var sourcePathPattern = regexp.MustCompile(
	`(?:^|: |\b)((?:canvas\.floor_source|start|` +
		`place\[\d+\](?:\.[a-z_]+)?|walls\[\d+\](?:\.[a-z_]+)?|` +
		`regions\[\d+\](?:\.[a-z_]+)?|rooms\[\d+\](?:\.[a-z_]+)?))`,
)

func fieldErrorFor(err error) FieldError {
	message := err.Error()
	field := ""
	if match := sourcePathPattern.FindStringSubmatch(message); len(match) > 1 {
		field = match[1]
	}
	code := "invalid_candidate"
	switch {
	case strings.Contains(message, "decode dungeon spec"),
		strings.Contains(message, "empty dungeon spec"),
		strings.Contains(message, "multi-document"):
		code = "invalid_yaml"
	case strings.Contains(message, "duplicates regions"), strings.Contains(message, "equal cell set"):
		code = "duplicate_region"
	case strings.Contains(message, "outside structural floor"), strings.Contains(message, "not a semantic floor cell"):
		code = "outside_floor"
	}
	return FieldError{Field: field, Message: message, Code: code}
}
