package actions

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// ActionData represents the common data structure for loading actions.
// Actions share a consistent schema, so we use LoadFromData instead of LoadJSON.
// Capacity for temporary actions (OffHandStrike, FlurryStrike) is tracked via
// ActionEconomy, not serialized with the action.
type ActionData struct {
	// Ref identifies the action type (e.g., refs.Actions.Strike())
	Ref *core.Ref

	// ID is the unique identifier for this action instance
	ID string

	// OwnerID is the character who owns this action
	OwnerID string

	// WeaponID is the weapon used for this action (empty for Move, FlurryStrike)
	WeaponID weapons.WeaponID
}

// LoadFromData creates an Action from the given data.
// Routes to the appropriate action constructor based on the Ref.
func LoadFromData(data ActionData) (Action, error) {
	if data.Ref == nil {
		return nil, fmt.Errorf("action data requires a ref")
	}

	switch data.Ref.ID {
	case refs.Actions.Move().ID:
		return NewMove(MoveConfig{
			ID:      data.ID,
			OwnerID: data.OwnerID,
		}), nil

	case refs.Actions.Strike().ID:
		return NewStrike(StrikeConfig{
			ID:       data.ID,
			OwnerID:  data.OwnerID,
			WeaponID: data.WeaponID,
		}), nil

	case refs.Actions.OffHandStrike().ID:
		return NewOffHandStrike(OffHandStrikeConfig{
			ID:       data.ID,
			OwnerID:  data.OwnerID,
			WeaponID: data.WeaponID,
		}), nil

	case refs.Actions.FlurryStrike().ID:
		return NewFlurryStrike(FlurryStrikeConfig{
			ID:      data.ID,
			OwnerID: data.OwnerID,
		}), nil

	case refs.Actions.UnarmedStrike().ID:
		// UnarmedStrike uses Strike with no weapon
		return NewStrike(StrikeConfig{
			ID:       data.ID,
			OwnerID:  data.OwnerID,
			WeaponID: "", // No weapon for unarmed
		}), nil

	default:
		return nil, fmt.Errorf("unknown action type: %s", data.Ref.ID)
	}
}
