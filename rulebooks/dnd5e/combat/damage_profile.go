package combat

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

func rollAttackDamage(
	ctx context.Context,
	spec damage.DamageSpec,
	source dnd5eEvents.DamageSourceType,
	sourceRef *core.Ref,
	critical bool,
	roller dice.Roller,
) ([]dnd5eEvents.DamageComponent, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}
	if roller == nil {
		roller = dice.NewRoller()
	}

	components := make([]dnd5eEvents.DamageComponent, 0, len(spec.Pools))
	for _, damagePool := range spec.Pools {
		times := 1
		if critical && hasDamageProperty(damagePool, damage.PropertyCritEligible) {
			times = 2
		}

		terms := damagePool.Terms
		if len(terms) == 0 {
			expression, err := damage.ParseExpression(damagePool.Dice)
			if err != nil {
				return nil, rpgerr.Wrap(err, "invalid attack damage dice")
			}
			terms = expression.Terms
		}

		rolledTerms := make([]dnd5eEvents.RolledDiceTerm, 0, len(terms))
		rolls := make([]int, 0)
		for _, term := range terms {
			pool, err := dice.ParseNotation(term.Dice)
			if err != nil {
				return nil, rpgerr.Wrap(err, "invalid attack damage dice")
			}
			termRolls, err := rollDamageDice(ctx, pool, roller, times)
			if err != nil {
				return nil, err
			}
			rolledTerms = append(rolledTerms, dnd5eEvents.RolledDiceTerm{Dice: term.Dice, Sign: term.Sign, Original: termRolls, Final: termRolls})
			rolls = append(rolls, termRolls...)
		}

		components = append(components, dnd5eEvents.DamageComponent{
			Source:            source,
			SourceRef:         sourceRef,
			DiceNotation:      damagePool.Dice,
			Terms:             rolledTerms,
			OriginalDiceRolls: rolls,
			FinalDiceRolls:    rolls,
			FlatBonus:         damagePool.FlatBonus,
			DamageType:        damagePool.Type,
			IsCritical:        critical,
		})
	}

	return components, nil
}

func hasDamageProperty(pool damage.Damage, property damage.Property) bool {
	for _, candidate := range pool.Properties {
		if candidate == property {
			return true
		}
	}
	return false
}

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
		out = append(out, dnd5eEvents.DamageComponent{Source: dnd5eEvents.DamageSourceWeapon, DiceNotation: part.Dice, OriginalDiceRolls: rolls, FinalDiceRolls: rolls, FlatBonus: flat, DamageType: part.DamageType, IsCritical: critical})
	}
	return out, nil
}
