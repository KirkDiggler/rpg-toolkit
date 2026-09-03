// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// scriptedRoller answers rolls from a fixed script and records every die size
// it was asked for. It is the interaction's roller in these tests: the one
// object that can prove a condition rolled THE INTERACTION'S dice rather than
// a process-global default.
type scriptedRoller struct {
	calls   []int
	results []int
}

// Roll serves the next scripted face, or fails when the script is exhausted —
// an unexpected roll is a failure this test wants to hear about loudly.
func (r *scriptedRoller) Roll(_ context.Context, size int) (int, error) {
	r.calls = append(r.calls, size)
	if len(r.results) == 0 {
		return 0, errors.New("scriptedRoller: script exhausted")
	}

	next := r.results[0]
	r.results = r.results[1:]

	return next, nil
}

// RollN serves count dice from the script, one Roll per die.
func (r *scriptedRoller) RollN(ctx context.Context, count, size int) ([]int, error) {
	rolls := make([]int, 0, count)
	for range count {
		roll, err := r.Roll(ctx, size)
		if err != nil {
			return nil, err
		}
		rolls = append(rolls, roll)
	}

	return rolls, nil
}

// RollerBindingTestSuite pins the generic roller-binding contract at the
// Character attach boundary: a persisted condition restores with no roller,
// and the attach entry that accepts one must bind it to every condition that
// wants it — without changing ordering, attribution, serialization, or the
// no-op rollback contract.
type RollerBindingTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestRollerBindingSuite(t *testing.T) {
	suite.Run(t, new(RollerBindingTestSuite))
}

func (s *RollerBindingTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// gwfBlob is a persisted Great Weapon Fighting condition in the exact bytes
// its own serializer produces, so the test is a claim about the load path
// rather than about hand-written JSON matching a serializer's field order.
func gwfBlob(s *suite.Suite, memberID string) json.RawMessage {
	gwf := &conditions.FightingStyleGreatWeaponFightingCondition{MemberID: memberID}

	raw, err := gwf.ToJSON()
	s.Require().NoError(err)

	return raw
}

// rollerBindingSheet is a minimal persisted fighter carrying the given
// conditions — enough sheet to load, attach, and swing a weapon.
func rollerBindingSheet(conditions ...json.RawMessage) *Data {
	return &Data{
		ID:               "roller-fighter",
		PlayerID:         "player-roller",
		Name:             "Roller Binding",
		Level:            4,
		ProficiencyBonus: 2,
		ClassID:          classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16,
			abilities.DEX: 10,
			abilities.CON: 14,
			abilities.INT: 10,
			abilities.WIS: 10,
			abilities.CHA: 10,
		},
		HitPoints:    20,
		MaxHitPoints: 30,
		ArmorClass:   16,
		Conditions:   conditions,
	}
}

// markedWeaponComponent builds the marked primary weapon component whose dice
// trace carries the given faces: the shape GWF's rule keys on.
func markedWeaponComponent(faces ...int) dnd5eEvents.DamageComponent {
	subtotal := 0
	for _, face := range faces {
		subtotal += face
	}

	return dnd5eEvents.DamageComponent{
		Source:     dnd5eEvents.DamageSourceWeapon,
		Properties: []damage.Property{damage.AddsAttackAbilityModifier},
		Roll: dnd5eEvents.RollComponent{
			Source: dnd5eEvents.RollSource{
				Ref:  refs.Weapons.Greatsword(),
				Name: "Greatsword",
			},
			Dice: &dnd5eEvents.DiceTrace{
				Notation:      dice.SimplePool(len(faces), 6, 0).Notation(),
				DieSize:       6,
				OriginalRolls: faces,
				FinalRolls:    append([]int(nil), faces...),
				Subtotal:      subtotal,
			},
		},
		DamageType: damage.Slashing,
	}
}

// runWeaponDamage publishes a real DamageChain — the attacker swinging a
// marked 2d6 weapon at a goblin — through the given bus and executes the
// chain, returning the final event.
func (s *RollerBindingTestSuite) runWeaponDamage(
	bus events.EventBus, faces ...int,
) *dnd5eEvents.DamageChainEvent {
	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: "roller-fighter",
		TargetID:   "goblin-1",
		Components: []dnd5eEvents.DamageComponent{markedWeaponComponent(faces...)},
	}

	damageChain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.DamageChain.On(bus).PublishWithChain(s.ctx, event, damageChain)
	s.Require().NoError(err)

	final, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)

	return final
}

// TestPersistedGWFBindsTheSuppliedRoller is the acceptance test for the
// generic binding: a persisted character carrying a serialized Great Weapon
// Fighting condition, loaded through the strict path and attached with the
// new generic input, rerolls the marked 2d6 weapon trace through the SUPPLIED
// roller — [1,5] becomes [4,5] with a canonical sourced 1→4 reroll and a
// subtotal of 9 — instead of falling back to process-global randomness.
func (s *RollerBindingTestSuite) TestPersistedGWFBindsTheSuppliedRoller() {
	char, err := Load(s.ctx, rollerBindingSheet(gwfBlob(&s.Suite, "roller-fighter")))
	s.Require().NoError(err)

	roller := &scriptedRoller{results: []int{4}}
	bus := events.NewEventBus()
	s.Require().NoError(AttachWithRoller(s.ctx, char, bus, roller))
	s.Empty(roller.calls, "attach binds the roller; it does not roll")

	final := s.runWeaponDamage(bus, 1, 5)

	s.Equal([]int{6}, roller.calls, "the supplied roller is called exactly once, with a d6")

	trace := final.Components[0].Roll.Dice
	s.Require().NotNil(trace)
	s.Equal([]int{1, 5}, trace.OriginalRolls, "original faces are immutable")
	s.Equal([]int{4, 5}, trace.FinalRolls)
	s.Equal(9, trace.Subtotal, "6 moves to 9 with the sourced 1→4")

	s.Require().Len(trace.Rerolls, 1)
	s.Equal(0, trace.Rerolls[0].DieIndex)
	s.Equal(1, trace.Rerolls[0].Before)
	s.Equal(4, trace.Rerolls[0].After)
	s.Require().NotNil(trace.Rerolls[0].Source.Ref)
	s.Equal(refs.Conditions.FightingStyleGreatWeaponFighting().String(),
		trace.Rerolls[0].Source.Ref.String(), "the reroll is sourced canonically")
	s.Equal("Great Weapon Fighting", trace.Rerolls[0].Source.Name)

	calc := &dnd5eEvents.RollCalculation{
		Components: []dnd5eEvents.RollComponent{final.Components[0].Roll},
		Total:      9,
	}
	s.Require().NoError(dnd5eEvents.ValidateRollCalculation(calc))
}

// TestLenientRouteBindsTheSuppliedRoller pins the lenient half of the
// contract: LoadFromData's own exported route receives the optional roller and
// binds it the same way, so a lenient caller never reimplements loading policy.
func (s *RollerBindingTestSuite) TestLenientRouteBindsTheSuppliedRoller() {
	roller := &scriptedRoller{results: []int{4}}
	bus := events.NewEventBus()
	char, err := LoadFromDataWithRoller(
		s.ctx, rollerBindingSheet(gwfBlob(&s.Suite, "roller-fighter")), bus, roller)
	s.Require().NoError(err)
	s.Require().NotEmpty(char.subscriptionIDs, "the sheet is attached")

	final := s.runWeaponDamage(bus, 1, 5)

	s.Equal([]int{6}, roller.calls)
	trace := final.Components[0].Roll.Dice
	s.Require().NotNil(trace)
	s.Equal([]int{1, 5}, trace.OriginalRolls)
	s.Equal([]int{4, 5}, trace.FinalRolls)
	s.Equal(9, trace.Subtotal)
}

// TestFailedStrictAttachLeavesNoRollerBoundAndRetryBindsOne pins the rollback
// contract with a roller in play: GWF applies, Brutal Critical's first
// subscription is refused, and the whole strict attach rolls back — the
// supplied roller must never have been called, because binding happens only
// after every effect has applied. The sheet is byte-identical, and a retry on
// a good bus binds the RETRY'S roller.
func (s *RollerBindingTestSuite) TestFailedStrictAttachLeavesNoRollerBoundAndRetryBindsOne() {
	data := rollerBindingSheet(gwfBlob(&s.Suite, "roller-fighter"))
	data.Conditions = append(data.Conditions, brutalCriticalBlob(&s.Suite))

	char, err := Load(s.ctx, data)
	s.Require().NoError(err)
	before := marshalData(&s.Suite, char.ToData())

	// Five keeper hooks, GWF's one damage-chain subscription, then Brutal
	// Critical's first subscription.
	failedRoller := &scriptedRoller{results: []int{4}}
	bus := newFailingBus(7)
	err = AttachWithRoller(s.ctx, char, bus, failedRoller)

	s.Require().Error(err)
	s.Empty(failedRoller.calls, "a failed attach binds nothing: the roller is never called")
	s.Equal(before, marshalData(&s.Suite, char.ToData()),
		"the sheet is byte-identical after the failed attach")
	s.Require().Len(char.pendingEffects, 2, "both conditions are pending again")

	// The retry binds its own roller, and the failed one stays untouched.
	retryRoller := &scriptedRoller{results: []int{4}}
	good := events.NewEventBus()
	s.Require().NoError(AttachWithRoller(s.ctx, char, good, retryRoller))

	final := s.runWeaponDamage(good, 1, 5)

	s.Empty(failedRoller.calls, "the failed attach's roller is still never called")
	s.Equal([]int{6}, retryRoller.calls, "the retry's roller is the one bound")
	trace := final.Components[0].Roll.Dice
	s.Require().NotNil(trace)
	s.Equal([]int{4, 5}, trace.FinalRolls, "the retry rerolled through its own roller")
	s.Equal([]int{1, 5}, trace.OriginalRolls)
}

// TestNilRollerAttachesWithoutBinding pins the nil half of the contract: a
// nil roller attaches the sheet exactly as Attach always has — every
// condition applied, no serialized state changed, and no condition's roller
// erased.
func (s *RollerBindingTestSuite) TestNilRollerAttachesWithoutBinding() {
	char, err := Load(s.ctx, rollerBindingSheet(gwfBlob(&s.Suite, "roller-fighter")))
	s.Require().NoError(err)
	before := marshalData(&s.Suite, char.ToData())

	bus := events.NewEventBus()
	s.Require().NoError(AttachWithRoller(s.ctx, char, bus, nil))

	for _, cond := range char.GetConditions() {
		s.Require().True(cond.IsApplied(), "a nil roller attaches the sheet as usual")
	}
	s.Equal(before, marshalData(&s.Suite, char.ToData()),
		"attaching with a roller changes no serialized state")
}

// TestExistingWrappersStillWork pins the source-compatible contract: the old
// Attach and LoadFromData signatures keep working, and attach with no roller.
func (s *RollerBindingTestSuite) TestExistingWrappersStillWork() {
	char, err := Load(s.ctx, rollerBindingSheet(gwfBlob(&s.Suite, "roller-fighter")))
	s.Require().NoError(err)
	bus := events.NewEventBus()
	s.Require().NoError(Attach(s.ctx, char, bus))
	for _, cond := range char.GetConditions() {
		s.Require().True(cond.IsApplied())
	}

	lenient, err := LoadFromData(
		s.ctx, rollerBindingSheet(gwfBlob(&s.Suite, "roller-fighter")), events.NewEventBus())
	s.Require().NoError(err)
	s.Require().NotEmpty(lenient.subscriptionIDs, "the lenient wrapper still attaches")
}

// TestNilInputsRefused pins the new entries' validation: same refusals, same
// messages, as the wrappers they back.
func (s *RollerBindingTestSuite) TestNilInputsRefused() {
	char, err := Load(s.ctx, rollerBindingSheet(gwfBlob(&s.Suite, "roller-fighter")))
	s.Require().NoError(err)

	s.Require().ErrorContains(AttachWithRoller(s.ctx, nil, events.NewEventBus(), nil),
		"character is required")
	s.Require().ErrorContains(AttachWithRoller(s.ctx, char, nil, nil),
		"event bus is required")

	_, err = LoadFromDataWithRoller(
		s.ctx, rollerBindingSheet(gwfBlob(&s.Suite, "roller-fighter")), nil, nil)
	s.Require().ErrorContains(err, "event bus is required")
}
