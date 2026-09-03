// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

type interactionRoll struct {
	count int
	sides int
	faces []int
}

type interactionRoller struct {
	script []interactionRoll
	calls  []string
}

func (r *interactionRoller) Roll(_ context.Context, sides int) (int, error) {
	faces, err := r.next(1, sides, fmt.Sprintf("Roll(d%d)", sides))
	if err != nil {
		return 0, err
	}
	return faces[0], nil
}

func (r *interactionRoller) RollN(_ context.Context, count, sides int) ([]int, error) {
	return r.next(count, sides, fmt.Sprintf("RollN(%d,d%d)", count, sides))
}

func (r *interactionRoller) next(count, sides int, call string) ([]int, error) {
	r.calls = append(r.calls, call)
	if len(r.script) == 0 {
		return nil, errors.New("interaction roller: script exhausted")
	}

	next := r.script[0]
	r.script = r.script[1:]
	if next.count != count || next.sides != sides || len(next.faces) != count {
		return nil, fmt.Errorf("interaction roller: got %dd%d, scripted %dd%d with %d faces",
			count, sides, next.count, next.sides, len(next.faces))
	}
	return append([]int(nil), next.faces...), nil
}

func persistedGWFCharacter(t *testing.T) *character.Data {
	t.Helper()

	blob, err := (&conditions.FightingStyleGreatWeaponFightingCondition{MemberID: heroID}).ToJSON()
	require.NoError(t, err)

	hero := actionHero()
	hero.Conditions = []json.RawMessage{blob}
	return hero
}

func greatswordDefinition() combatActions.Definition {
	greatsword := *refs.Weapons.Greatsword()
	return combatActions.Definition{
		Ref:  greatsword,
		Name: "Greatsword",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: 4,
			Ability:     &combatActions.AbilityContribution{Ability: abilities.STR, Modifier: 3},
			Weapon:      &combatActions.WeaponContext{Ref: &greatsword, TwoHanded: true},
			Damage: []damage.Damage{{
				Dice:       "2d6",
				Type:       damage.Slashing,
				Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			}},
		},
	}
}

// TestResolveBindsOneRollerToTheMachineAndPersistedCharacter proves the strict
// route consumes one caller-authored script for every random decision: attack,
// original weapon pool, then the persisted Great Weapon Fighting reroll.
func TestResolveBindsOneRollerToTheMachineAndPersistedCharacter(t *testing.T) {
	roller := &interactionRoller{script: []interactionRoll{
		{count: 1, sides: 20, faces: []int{15}},
		{count: 2, sides: 6, faces: []int{1, 5}},
		{count: 1, sides: 6, faces: []int{4}},
	}}
	definition := greatswordDefinition()

	out, err := Resolve(context.Background(), &Input{
		World: actionWorld(t, 2),
		Participants: []Participant{
			{Character: persistedGWFCharacter(t)},
			{Monster: monsters.NewWolf(wolfID).ToData()},
		},
		Machine: NewStrike(&StrikeInput{
			AttackerID: heroID,
			TargetID:   wolfID,
			Definition: definition,
			Roller:     roller,
		}),
		Initiative: orderAsGiven{},
		Standing:   everyoneStanding{},
		Sight:      everyoneSeesTheWholeMap{},
		TurnDriver: passDriver{},
		Roller:     roller,
	})
	require.NoError(t, err)
	require.Equal(t, []string{"Roll(d20)", "RollN(2,d6)", "Roll(d6)"}, roller.calls)
	require.Empty(t, roller.script, "the interaction consumed the complete caller-authored script")

	outcome, ok := out.Outcome.(StrikeOutcome)
	require.True(t, ok)
	require.True(t, outcome.Hit)
	require.Equal(t, 12, outcome.Damage, "GWF subtotal 9 plus Strength 3")
	require.Equal(t, []damage.Instance{{Amount: 12, Type: damage.Slashing}}, outcome.DamageInstances)
	require.Len(t, outcome.DamageComponents, 2)

	weapon := outcome.DamageComponents[0]
	require.Equal(t, dnd5eEvents.DamageSourceWeapon, weapon.Source)
	require.Equal(t, []damage.Property{damage.AddsAttackAbilityModifier}, weapon.Properties)
	require.NotNil(t, weapon.Roll.Dice)
	trace := weapon.Roll.Dice
	require.Equal(t, []int{1, 5}, trace.OriginalRolls)
	require.Equal(t, []int{4, 5}, trace.FinalRolls)
	require.Equal(t, 9, trace.Subtotal)
	require.Equal(t, []dnd5eEvents.DiceReroll{{
		DieIndex: 0,
		Before:   1,
		After:    4,
		Source: dnd5eEvents.RollSource{
			Ref:  refs.Conditions.FightingStyleGreatWeaponFighting(),
			Name: "Great Weapon Fighting",
		},
	}}, trace.Rerolls)

	ability := outcome.DamageComponents[1]
	require.Equal(t, dnd5eEvents.DamageSourceAbility, ability.Source)
	require.NotNil(t, ability.Roll.Modifier)
	require.Equal(t, 3, *ability.Roll.Modifier)
}

// TestLenientCharacterAttachBindsTheRollerAndStillDropsUnreadable proves the
// projection policy and roller binding travel through the same attach helper:
// malformed persisted data is still dropped, while valid GWF uses the supplied
// roller rather than a hidden default.
func TestLenientCharacterAttachBindsTheRollerAndStillDropsUnreadable(t *testing.T) {
	data := persistedGWFCharacter(t)
	data.Conditions = append(data.Conditions, json.RawMessage(`{"ref":`))
	roller := &interactionRoller{script: []interactionRoll{{count: 1, sides: 6, faces: []int{4}}}}
	surf := newSurface(events.NewEventBus())

	cast, err := attachAll(context.Background(), surf, &attachAllInput{
		Participants:   []Participant{{Character: data}},
		Roller:         roller,
		DropUnreadable: true,
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, surf.teardown(context.Background())) }()
	_, ok := cast.Character(heroID)
	require.True(t, ok, "the lenient policy keeps the readable participant")

	trace := foldMarkedWeaponDamage(t, surf)
	require.Equal(t, []string{"Roll(d6)"}, roller.calls)
	require.Empty(t, roller.script)
	require.Equal(t, []int{1, 5}, trace.OriginalRolls)
	require.Equal(t, []int{4, 5}, trace.FinalRolls)
	require.Equal(t, 9, trace.Subtotal)
}

func foldMarkedWeaponDamage(t *testing.T, bus events.EventBus) *dnd5eEvents.DiceTrace {
	t.Helper()

	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: heroID,
		TargetID:   wolfID,
		Components: []dnd5eEvents.DamageComponent{{
			Source:     dnd5eEvents.DamageSourceWeapon,
			Properties: []damage.Property{damage.AddsAttackAbilityModifier},
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Greatsword(), Name: "Greatsword"},
				Dice: &dnd5eEvents.DiceTrace{
					Notation:      "2d6",
					DieSize:       6,
					OriginalRolls: []int{1, 5},
					FinalRolls:    []int{1, 5},
					Subtotal:      6,
				},
			},
			DamageType: damage.Slashing,
		}},
	}

	staged := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.DamageChain.On(bus).PublishWithChain(context.Background(), event, staged)
	require.NoError(t, err)
	final, err := modified.Execute(context.Background(), event)
	require.NoError(t, err)
	require.Len(t, final.Components, 1)
	require.NotNil(t, final.Components[0].Roll.Dice)
	return final.Components[0].Roll.Dice
}
