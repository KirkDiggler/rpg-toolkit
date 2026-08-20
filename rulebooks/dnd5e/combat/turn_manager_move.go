package combat

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// MoveInput provides input for movement.
type MoveInput struct {
	Path []spatial.Position
}

// Move executes movement along a path, consuming movement from the economy.
func (tm *TurnManager) Move(ctx context.Context, input *MoveInput) (*MoveEntityResult, error) {
	if tm.turnEnded {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn already ended")
	}
	if !tm.turnStarted {
		return nil, rpgerr.New(rpgerr.CodeInvalidState, "turn not started")
	}
	if input == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "MoveInput is nil")
	}
	if len(input.Path) < 2 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "Path must have at least 2 positions")
	}

	currentPos, exists := tm.room.GetEntityPosition(tm.character.GetID())
	if !exists {
		return nil, rpgerr.Newf(rpgerr.CodeNotFound, "entity not found in room: %s", tm.character.GetID())
	}
	if input.Path[0].X != currentPos.X || input.Path[0].Y != currentPos.Y {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"Path[0] must be current position (%d,%d), got (%d,%d)",
			int(currentPos.X), int(currentPos.Y), int(input.Path[0].X), int(input.Path[0].Y))
	}

	steps := len(input.Path) - 1
	cost := steps * int(FeetPerGridUnit)
	if err := tm.economy.UseMovement(cost); err != nil {
		return nil, err
	}

	combatCtx := tm.buildContext(ctx)
	result, err := MoveEntity(combatCtx, &MoveEntityInput{
		EntityID:   tm.character.GetID(),
		EntityType: "character",
		Path:       input.Path,
		EventBus:   tm.bus,
		Roller:     tm.roller,
	})
	if err != nil {
		return nil, err
	}

	if result.MovementStopped {
		unusedSteps := steps - result.StepsCompleted
		if unusedSteps > 0 {
			tm.economy.AddMovement(unusedSteps * int(FeetPerGridUnit))
		}
	}
	return result, nil
}
