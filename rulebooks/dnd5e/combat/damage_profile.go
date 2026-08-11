package combat

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// DamageProfileComponent describes unrolled damage promised by one attack.
// It is deliberately distinct from events.DamageComponent, which records
// actual dice rolls and final modifiers after resolution.
type DamageProfileComponent struct {
	Dice                   string      `json:"dice"`
	DamageType             damage.Type `json:"damage_type"`
	AppliesAbilityModifier bool        `json:"applies_ability_modifier,omitempty"`
}

// RollDamageProfile rolls every component independently. Critical hits double
// dice only; an ability modifier is added once to opted-in components.
func RollDamageProfile(ctx context.Context, profile []DamageProfileComponent, abilityModifier int, critical bool, roller dice.Roller) ([]dnd5eEvents.DamageComponent, error) {
	if len(profile) == 0 {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "damage profile is required")
	}
	if roller == nil {
		roller = dice.NewRoller()
	}
	out := make([]dnd5eEvents.DamageComponent, 0, len(profile))
	for _, part := range profile {
		pool, err := dice.ParseNotation(part.Dice)
		if err != nil {
			return nil, rpgerr.Wrap(err, "invalid damage profile dice")
		}
		times := 1
		if critical {
			times = 2
		}
		rolls, err := rollDamageDice(ctx, pool, roller, times)
		if err != nil {
			return nil, err
		}
		flat := 0
		if part.AppliesAbilityModifier {
			flat = abilityModifier
		}
		out = append(out, dnd5eEvents.DamageComponent{Source: dnd5eEvents.DamageSourceWeapon, OriginalDiceRolls: rolls, FinalDiceRolls: rolls, FlatBonus: flat, DamageType: part.DamageType, IsCritical: critical})
	}
	return out, nil
}
