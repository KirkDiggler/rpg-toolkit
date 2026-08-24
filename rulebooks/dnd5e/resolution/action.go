package resolution

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
)

// ActionInput identifies a shared action definition and the participants it targets.
type ActionInput struct {
	Definition combatActions.Definition
	AttackerID string
	TargetID   string
	Roller     dice.Roller
}

// NewAction validates an inert definition and dispatches by populated profile arm.
func NewAction(in *ActionInput) (Machine, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if err := in.Definition.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadAction, err)
	}
	if in.Definition.Attack != nil {
		return NewStrike(&StrikeInput{
			AttackerID: in.AttackerID,
			TargetID:   in.TargetID,
			Definition: in.Definition.Clone(),
			Roller:     in.Roller,
		}), nil
	}
	return nil, fmt.Errorf("%w: definition %q has no supported profile", ErrBadAction, in.Definition.Ref.String())
}
