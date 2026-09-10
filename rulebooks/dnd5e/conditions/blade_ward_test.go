// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const wardedID = "bard-1"

type BladeWardConditionSuite struct {
	suite.Suite
	ctx      context.Context
	bus      events.EventBus
	removals []dnd5eEvents.ConditionRemovedEvent
}

func TestBladeWardConditionSuite(t *testing.T) {
	suite.Run(t, new(BladeWardConditionSuite))
}

func (s *BladeWardConditionSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.removals = nil

	_, err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			s.removals = append(s.removals, event)
			return nil
		})
	s.Require().NoError(err)
}

// warded returns a live ward on the bard for its full two turn ends.
func (s *BladeWardConditionSuite) warded() *BladeWardCondition {
	condition := NewBladeWardCondition(wardedID, "", 2)
	s.Require().NoError(condition.Apply(s.ctx, s.bus))
	return condition
}

func (s *BladeWardConditionSuite) endTurn(subjectID string) {
	s.Require().NoError(dnd5eEvents.TurnEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.TurnEndEvent{SubjectID: subjectID, Round: 1}))
}

// strike drives one damage chain at targetID and returns the settled total,
// which is what a player actually feels. Source says who dealt it -- a swing or
// a spell -- and that distinction is the whole point of this condition.
func (s *BladeWardConditionSuite) strike(
	targetID string, source dnd5eEvents.DamageSourceType, amount int, damageType damage.Type,
) int {
	component := dnd5eEvents.DamageComponent{
		Source: source,
		Roll: dnd5eEvents.RollComponent{
			Source: dnd5eEvents.RollSource{Ref: refs.Weapons.Longsword(), Name: "Longsword"},
			Dice:   testDiceTrace(8, amount),
		},
		DamageType: damageType,
	}
	event := &dnd5eEvents.DamageChainEvent{
		AttackerID: "skeleton-1",
		TargetID:   targetID,
		Components: []dnd5eEvents.DamageComponent{component},
	}

	chain := events.NewStagedChain[*dnd5eEvents.DamageChainEvent](combat.ModifierStages)
	modified, err := dnd5eEvents.DamageChain.On(s.bus).PublishWithChain(s.ctx, event, chain)
	s.Require().NoError(err)
	settled, err := modified.Execute(s.ctx, event)
	s.Require().NoError(err)

	_, total := combat.FinalDamage(settled.Components)
	return total
}

// The promise, asserted as the number rather than as the mechanism: a warded
// bard takes half. combat.FinalDamage is what folds the multiplier, so folding
// it here is what proves the component was appended in a form that actually
// counts.
func (s *BladeWardConditionSuite) TestAWardedCasterTakesHalfFromAWeaponSwing() {
	s.warded()

	s.Equal(4, s.strike(wardedID, dnd5eEvents.DamageSourceWeapon, 9, damage.Slashing),
		"9 slashing from a weapon lands as 4, halved and rounded down")
}

// THE SCOPING TEST, and the reason this condition does not simply copy rage's
// predicate. RAW resists damage "dealt by weapon attacks"; rage resists all
// B/P/S whatever dealt it. Same damage type, same amount, different source.
func (s *BladeWardConditionSuite) TestItLeavesNonWeaponDamageAlone() {
	s.warded()

	s.Equal(9, s.strike(wardedID, dnd5eEvents.DamageSourceSpell, 9, damage.Slashing),
		"a spell's slashing is not a weapon attack, and the ward does not reach it")
}

func (s *BladeWardConditionSuite) TestItLeavesNonPhysicalWeaponDamageAlone() {
	s.warded()

	s.Equal(9, s.strike(wardedID, dnd5eEvents.DamageSourceWeapon, 9, damage.Fire),
		"the ward is against blade, point and bludgeon, not against fire")
}

func (s *BladeWardConditionSuite) TestItLeavesSomebodyElsesDamageAlone() {
	s.warded()

	s.Equal(9, s.strike("fighter-1", dnd5eEvents.DamageSourceWeapon, 9, damage.Slashing),
		"the ward is on the bard and reads the defender side")
}

func (s *BladeWardConditionSuite) TestItWardsEveryPhysicalType() {
	s.warded()

	for _, physical := range []damage.Type{damage.Slashing, damage.Piercing, damage.Bludgeoning} {
		s.Equal(4, s.strike(wardedID, dnd5eEvents.DamageSourceWeapon, 9, physical), "%s", physical)
	}
}

// THE OFF-BY-ONE, and the reason the count is two rather than one.
//
// Blade Ward costs an action, so it is always traced during the caster's own
// turn -- and the very next turn end on the bus is that same turn's. A ward
// that spent its only count there would expire before any enemy could swing,
// which is the failure this asserts against rather than describes.
func (s *BladeWardConditionSuite) TestItSurvivesTheTurnItWasTracedOn() {
	condition := s.warded()

	s.endTurn(wardedID)
	s.Empty(s.removals, "the casting turn's own end must not spend the ward")
	s.True(condition.IsApplied())
	s.Equal(4, s.strike(wardedID, dnd5eEvents.DamageSourceWeapon, 9, damage.Slashing),
		"and it is still warding when the swing finally comes")

	s.endTurn(wardedID)
	s.Require().Len(s.removals, 1, "the end of the caster's NEXT turn is what ends it")
	s.Equal(refs.Conditions.BladeWard().String(), s.removals[0].ConditionRef)
	s.Equal("expired", s.removals[0].Reason)
	s.Equal(9, s.strike(wardedID, dnd5eEvents.DamageSourceWeapon, 9, damage.Slashing),
		"and once it is gone the swing lands whole")
}

func (s *BladeWardConditionSuite) TestSomebodyElsesTurnEndDoesNotSpendIt() {
	s.warded()

	for range 5 {
		s.endTurn("skeleton-1")
	}

	s.Empty(s.removals, "other members' turns do not own this clock")
}

func (s *BladeWardConditionSuite) TestCombatEndTakesIt() {
	s.warded()

	s.Require().NoError(dnd5eEvents.CombatEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.CombatEndEvent{SubjectID: wardedID}))

	s.Require().Len(s.removals, 1)
	s.Equal("combat ended", s.removals[0].Reason)
}

// The write side, key for key. A round trip cannot catch a rename; this can.
func (s *BladeWardConditionSuite) TestItPersistsTheExactWireShape() {
	condition := NewBladeWardCondition(wardedID, "", 2)

	raw, err := condition.ToJSON()
	s.Require().NoError(err)

	s.JSONEq(`{
		"ref":"dnd5e:conditions:blade_ward",
		"member_id":"bard-1",
		"source_ref":"dnd5e:spells:blade-ward",
		"turn_ends_left":2
	}`, string(raw))
}

// The read side, judged by WHEN THE WARD ENDS rather than by what a field
// reads -- the half a symmetric round trip structurally cannot reach.
func (s *BladeWardConditionSuite) TestAHandAuthoredBlobKeepsItsRemainingCount() {
	loaded, err := LoadJSON(json.RawMessage(`{
		"ref":"dnd5e:conditions:blade_ward",
		"member_id":"bard-1","source_ref":"dnd5e:spells:blade-ward","turn_ends_left":1
	}`))
	s.Require().NoError(err)
	s.Require().NoError(loaded.Apply(s.ctx, s.bus))

	s.endTurn(wardedID)

	s.Require().Len(s.removals, 1, "one left on the blob means the next caster turn end is its last")
}

func (s *BladeWardConditionSuite) TestTheFactoryRefusesAWardWithNoClock() {
	for _, config := range []json.RawMessage{nil, json.RawMessage(`{}`), json.RawMessage(`{"turn_ends":0}`)} {
		_, err := CreateFromRef(&CreateFromRefInput{
			Ref:      refs.Conditions.BladeWard().String(),
			MemberID: wardedID,
			Config:   config,
		})
		s.Require().Error(err, "config %s", config)
		s.Require().ErrorContains(err, "turn_ends", "config %s", config)
	}
}

func (s *BladeWardConditionSuite) TestTheFactoryBuildsItFromContentParameters() {
	built, err := CreateFromRef(&CreateFromRefInput{
		Ref:       refs.Conditions.BladeWard().String(),
		MemberID:  wardedID,
		SourceRef: refs.Spells.BladeWard().String(),
		Config:    json.RawMessage(`{"turn_ends":2}`),
	})
	s.Require().NoError(err)

	ward, ok := built.Condition.(*BladeWardCondition)
	s.Require().True(ok)
	s.Equal(wardedID, ward.MemberID)
	s.Equal(2, ward.TurnEndsLeft)
	s.Equal(refs.Spells.BladeWard().String(), ward.SourceRef)
}

func (s *BladeWardConditionSuite) TestItIsInTheDisplayCatalog() {
	display, ok := DisplayFor(*refs.Conditions.BladeWard())

	s.Require().True(ok)
	s.Equal(BladeWardName, display.Name)
}
