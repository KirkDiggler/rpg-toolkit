package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

func abilityWeaponDefinition(modifier int, offHand bool) combatActions.Definition {
	definition := validMeleeDefinition()
	definition.Ref = *refs.Weapons.Scimitar()
	definition.Name = "Scimitar"
	definition.Attack.AttackBonus = 5
	definition.Attack.Ability = &combatActions.AbilityContribution{
		Ability:  abilities.STR,
		Modifier: modifier,
	}
	definition.Attack.Weapon = &combatActions.WeaponContext{Ref: refs.Weapons.Scimitar()}
	definition.Attack.IsOffHandAttack = offHand
	definition.Attack.Damage = []damage.Damage{{
		Dice:       "1d6",
		Type:       damage.Slashing,
		Properties: []damage.Property{damage.AddsAttackAbilityModifier},
	}}
	return definition
}

func resolveHeroAttack(
	t *testing.T, definition combatActions.Definition, hero *character.Data, roller dice.Roller,
) (*Output, error) {
	t.Helper()
	machine, err := NewAction(&ActionInput{
		Definition: definition,
		AttackerID: heroID,
		TargetID:   wolfID,
		Roller:     roller,
	})
	require.NoError(t, err)

	return Resolve(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Monster: monsters.NewWolf(wolfID).ToData()},
			{Character: hero},
		},
		Machine:    machine,
		Initiative: orderAsGiven{},
		Standing:   everyoneStanding{},
		Sight:      everyoneSeesTheWholeMap{},
		TurnDriver: passDriver{},
		Roller:     dice.NewRoller(),
	})
}

func TestOffHandAttackKeepsNormalAttackRollBonus(t *testing.T) {
	definition := abilityWeaponDefinition(3, true)

	out, err := resolveHeroAttack(t, definition, actionHero(), &actionRoller{
		singles: []int{15},
		damage:  [][]int{{4}},
	})

	require.NoError(t, err)
	outcome := out.Outcome.(StrikeOutcome)
	require.Equal(t, 15, outcome.Roll)
	require.Equal(t, 20, outcome.Total)
}

func TestOffHandAttackAbilityModifierDamage(t *testing.T) {
	tests := []struct {
		name             string
		modifier         int
		wantDamage       int
		wantAbilityParts int
	}{
		{name: "positive omitted", modifier: 3, wantDamage: 4},
		{name: "zero omitted", modifier: 0, wantDamage: 4},
		{name: "negative retained", modifier: -2, wantDamage: 2, wantAbilityParts: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := resolveHeroAttack(t, abilityWeaponDefinition(tc.modifier, true), actionHero(), &actionRoller{
				singles: []int{15},
				damage:  [][]int{{4}},
			})

			require.NoError(t, err)
			outcome := out.Outcome.(StrikeOutcome)
			require.Equal(t, tc.wantDamage, outcome.Damage)

			abilityParts := 0
			for _, component := range outcome.DamageComponents {
				if component.Source == dnd5eEvents.DamageSourceAbility {
					abilityParts++
					require.Equal(t, tc.modifier, component.FlatBonus)
				}
			}
			require.Equal(t, tc.wantAbilityParts, abilityParts)
		})
	}
}

func TestTwoWeaponFightingStyleRestoresPositiveOffHandDamage(t *testing.T) {
	hero := actionHero()
	condition := conditions.NewFightingStyleTwoWeaponFightingCondition(heroID)
	blob, err := condition.ToJSON()
	require.NoError(t, err)
	hero.Conditions = []json.RawMessage{blob}

	out, err := resolveHeroAttack(t, abilityWeaponDefinition(3, true), hero, &actionRoller{
		singles: []int{15},
		damage:  [][]int{{4}},
	})

	require.NoError(t, err)
	outcome := out.Outcome.(StrikeOutcome)
	require.Equal(t, 7, outcome.Damage)
	require.Len(t, outcome.DamageComponents, 2)
	require.Equal(t, dnd5eEvents.DamageSourceFeature, outcome.DamageComponents[1].Source)
	require.Equal(t, refs.Conditions.FightingStyleTwoWeaponFighting(), outcome.DamageComponents[1].SourceRef)
	require.Equal(t, 3, outcome.DamageComponents[1].FlatBonus)
}

func TestOrdinaryWeaponAttackStillAddsPositiveAbilityDamage(t *testing.T) {
	out, err := resolveHeroAttack(t, abilityWeaponDefinition(3, false), actionHero(), &actionRoller{
		singles: []int{15},
		damage:  [][]int{{4}},
	})

	require.NoError(t, err)
	outcome := out.Outcome.(StrikeOutcome)
	require.Equal(t, 7, outcome.Damage)
	require.Len(t, outcome.DamageComponents, 2)
	require.Equal(t, dnd5eEvents.DamageSourceAbility, outcome.DamageComponents[1].Source)
	require.Equal(t, 3, outcome.DamageComponents[1].FlatBonus)
}
