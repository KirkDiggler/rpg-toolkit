package combat

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/stretchr/testify/require"
)

type fixedRoller struct {
	rolls []int
	at    int
}

func (r *fixedRoller) Roll(context.Context, int) (int, error) {
	v := r.rolls[r.at]
	r.at++
	return v, nil
}
func (r *fixedRoller) RollN(ctx context.Context, n, size int) ([]int, error) {
	out := make([]int, n)
	for i := range out {
		v, e := r.Roll(ctx, size)
		if e != nil {
			return nil, e
		}
		out[i] = v
	}
	return out, nil
}

var _ dice.Roller = (*fixedRoller)(nil)

func TestRollDamageProfileRetainsDeclaredDiceNotation(t *testing.T) {
	components, err := RollDamageProfile(context.Background(), []DamageProfileComponent{
		{Dice: "1d6", DamageType: damage.Acid},
		{Dice: "2d6", DamageType: damage.Bludgeoning},
	}, 0, false, &fixedRoller{rolls: []int{3, 2, 5}})
	require.NoError(t, err)
	require.Equal(t, "1d6", components[0].DiceNotation)
	require.Equal(t, "2d6", components[1].DiceNotation)
}

func TestRollDamageProfileCriticalDoublesEachComponent(t *testing.T) {
	components, err := RollDamageProfile(context.Background(), []DamageProfileComponent{
		{Dice: "1d6", DamageType: damage.Bludgeoning, AppliesAbilityModifier: true},
		{Dice: "2d6", DamageType: damage.Acid},
	}, 3, true, &fixedRoller{rolls: []int{2, 4, 1, 3, 5, 6}})
	if err != nil {
		t.Fatal(err)
	}
	if len(components) != 2 || len(components[0].OriginalDiceRolls) != 2 || len(components[1].OriginalDiceRolls) != 4 {
		t.Fatalf("unexpected critical components: %#v", components)
	}
	if components[0].FlatBonus != 3 || components[1].FlatBonus != 0 {
		t.Fatalf("ability modifier applied incorrectly: %#v", components)
	}
}
