package combat_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

func TestFormatDamageComponent(t *testing.T) {
	tests := []struct {
		name      string
		component dnd5eEvents.DamageComponent
		want      string
	}{
		{
			name:      "includes dice rolls and positive bonus",
			component: dnd5eEvents.DamageComponent{DiceNotation: "1d6", FinalDiceRolls: []int{4}, FlatBonus: 2, DamageType: damage.Acid},
			want:      "1d6 (4) + 2 acid = 6",
		},
		{
			name:      "joins multiple dice rolls",
			component: dnd5eEvents.DamageComponent{DiceNotation: "2d6", FinalDiceRolls: []int{5, 3}, FlatBonus: 3, DamageType: damage.Bludgeoning},
			want:      "2d6 (5 + 3) + 3 bludgeoning = 11",
		},
		{
			name:      "formats negative bonus",
			component: dnd5eEvents.DamageComponent{DiceNotation: "1d6", FinalDiceRolls: []int{4}, FlatBonus: -2, DamageType: damage.Acid},
			want:      "1d6 (4) - 2 acid = 2",
		},
		{
			name:      "omits zero bonus",
			component: dnd5eEvents.DamageComponent{DiceNotation: "1d6", FinalDiceRolls: []int{4}, DamageType: damage.Acid},
			want:      "1d6 (4) acid = 4",
		},
		{
			name:      "formats flat only component",
			component: dnd5eEvents.DamageComponent{FlatBonus: 2, DamageType: damage.Acid},
			want:      "+ 2 acid = 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, combat.FormatDamageComponent(tt.component))
		})
	}
}

func TestFormatDamageBreakdownDisplay(t *testing.T) {
	acid := dnd5eEvents.DamageComponent{DiceNotation: "1d6", FinalDiceRolls: []int{4}, FlatBonus: 2, DamageType: damage.Acid}
	bludgeoning := dnd5eEvents.DamageComponent{DiceNotation: "2d6", FinalDiceRolls: []int{5, 3}, FlatBonus: 3, DamageType: damage.Bludgeoning}

	breakdown := &combat.DamageBreakdown{
		Components:  []dnd5eEvents.DamageComponent{acid, bludgeoning},
		TotalDamage: 17,
	}

	require.Equal(t, "1d6 (4) + 2 acid = 6; 2d6 (5 + 3) + 3 bludgeoning = 11. Total: 17 damage.", breakdown.Display())
}
