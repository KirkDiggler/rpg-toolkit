package resolution

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

func validMeleeDefinition() combatActions.Definition {
	return combatActions.Definition{
		Ref:  core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "test-claw"},
		Name: "Test Claw",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Slashing, FlatBonus: 2}},
		},
	}
}

func TestNewActionDispatchesAnUnknownContentRefByProfile(t *testing.T) {
	definition := validMeleeDefinition()
	definition.Ref.ID = "never-registered-content"

	machine, err := NewAction(&ActionInput{Definition: definition, AttackerID: "attacker", TargetID: "target"})

	require.NoError(t, err)
	require.IsType(t, &strikeMachine{}, machine)
}

func TestNewActionRejectsInvalidDefinitions(t *testing.T) {
	_, err := NewAction(nil)
	require.ErrorIs(t, err, ErrNilInput)

	definition := validMeleeDefinition()
	definition.Attack = nil
	_, err = NewAction(&ActionInput{Definition: definition})
	require.ErrorIs(t, err, ErrBadAction)
}
