// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// ConcentratingConditionSuite is rpg-project#407's done-when: the owner answers
// damage with a check it does not roll, ends six ways, and takes its children
// with it.
// testCasterID is the one bard every case in this suite is about.
const testCasterID = "bard-1"

type ConcentratingConditionSuite struct {
	suite.Suite
	ctx      context.Context
	bus      events.EventBus
	casterID string
	spellRef string
	removals []dnd5eEvents.ConditionRemovedEvent
}

func TestConcentratingConditionSuite(t *testing.T) {
	suite.Run(t, new(ConcentratingConditionSuite))
}

func (s *ConcentratingConditionSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	s.casterID = testCasterID
	s.spellRef = refs.Spells.TrueStrike().String()
	s.removals = nil

	_, err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			s.removals = append(s.removals, event)
			return nil
		})
	s.Require().NoError(err)
}

// applied returns a live hold on True Strike with one child on a skeleton.
func (s *ConcentratingConditionSuite) applied() *ConcentratingCondition {
	condition := NewConcentratingCondition(s.casterID, s.spellRef, TrueStrikeName, 2)
	s.Require().NoError(condition.Apply(s.ctx, s.bus))
	s.Require().NoError(condition.AddChild(s.ctx, s.child()))
	return condition
}

func (s *ConcentratingConditionSuite) child() dnd5eEvents.ChildRef {
	return dnd5eEvents.ChildRef{
		MemberID:     s.casterID,
		ConditionRef: refs.Conditions.TrueStrike().String(),
	}
}

// damage publishes one damage-taken fact and hands back what subscribers wrote
// on it, which is the whole point of the return channel.
func (s *ConcentratingConditionSuite) damage(
	memberID string, amount int, droppedToZero bool,
) *dnd5eEvents.DamageTakenEvent {
	event := &dnd5eEvents.DamageTakenEvent{
		MemberID:      memberID,
		Amount:        amount,
		DroppedToZero: droppedToZero,
		Cause: dnd5eEvents.SaveCause{
			InstigatorID:   "skeleton-1",
			InstigatorType: "monster",
		},
	}
	s.Require().NoError(dnd5eEvents.DamageTakenTopic.On(s.bus).Publish(s.ctx, event))
	return event
}

// THE RETURN CHANNEL, and the reason the topic carries a pointer: a value topic
// would hand the subscriber a copy and this assertion would find nothing.
func (s *ConcentratingConditionSuite) TestDamageAppendsExactlyOneFollowUp() {
	condition := s.applied()

	event := s.damage(s.casterID, 9, false)

	s.Require().Len(event.FollowUps, 1)
	followUp := event.FollowUps[0]
	s.Equal(s.casterID, followUp.SaverID)
	s.Equal(abilities.CON, followUp.Ability)
	s.Equal(dnd5eEvents.SaveTriggerConcentration, followUp.Cause.Trigger)
	s.Require().NotNil(followUp.Cause.EffectRef)
	s.Equal(s.spellRef, followUp.Cause.EffectRef.String(), "the record says which spell was at stake")
	s.Equal("skeleton-1", followUp.Cause.InstigatorID, "and who threatened it")
	s.Equal([]dnd5eEvents.ChildRef{s.child()}, followUp.OnFailure.Remove)
	s.Equal(dnd5eEvents.ChildRef{
		MemberID:     s.casterID,
		ConditionRef: refs.Conditions.Concentrating().String(),
	}, followUp.OnFailure.Owner)
	s.Equal(ConcentrationEndedDamage, followUp.OnFailure.Reason)
	s.True(condition.IsApplied(), "appending a check is not failing one")
	s.Empty(s.removals, "and describing a check removes nothing")
}

// max(10, floor(damage/2)) — RAW 2014, and this slice's ruling. The floor only
// becomes visible on odd damage above 20, which is where a wrong one hides.
func (s *ConcentratingConditionSuite) TestTheDCIsHalfTheDamageWithAFloorOfTen() {
	cases := []struct {
		name     string
		damage   int
		expected int
	}{
		{"1 damage is the floor", 1, 10},
		{"9 damage is still the floor", 9, 10},
		{"20 damage is where the floor and the half agree", 20, 10},
		{"31 damage halves DOWN to 15, not up to 16", 31, 15},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.applied()

			event := s.damage(s.casterID, tc.damage, false)

			s.Require().Len(event.FollowUps, 1)
			s.Equal(tc.expected, event.FollowUps[0].DC, "%d damage", tc.damage)
		})
	}
}

func (s *ConcentratingConditionSuite) TestSomebodyElsesDamageIsNotItsProblem() {
	condition := s.applied()

	event := s.damage("fighter-2", 30, false)

	s.Empty(event.FollowUps, "the check belongs to whoever is holding the spell")
	s.True(condition.IsApplied())
}

// R7: a caster at 0 hit points does not roll to keep a spell.
func (s *ConcentratingConditionSuite) TestDroppingToZeroEndsItWithNoCheck() {
	condition := s.applied()

	event := s.damage(s.casterID, 30, true)

	s.Empty(event.FollowUps, "no check, so no save beat in the record either")
	s.False(condition.IsApplied())
	s.Require().Len(s.removals, 2)
	s.Equal(ConcentrationEndedCasterDown, s.removals[0].Reason)
	s.Equal(ConcentrationEndedCasterDown, s.removals[1].Reason)
}

// The children come off first, while their owner still names them.
func (s *ConcentratingConditionSuite) TestEndingPublishesEveryChildThenItself() {
	condition := NewConcentratingCondition(s.casterID, s.spellRef, TrueStrikeName, 1)
	s.Require().NoError(condition.Apply(s.ctx, s.bus))
	skeleton := dnd5eEvents.ChildRef{MemberID: "skeleton-1", ConditionRef: refs.Conditions.Charmed().String()}
	s.Require().NoError(condition.AddChild(s.ctx, s.child()))
	s.Require().NoError(condition.AddChild(s.ctx, skeleton))

	s.Require().NoError(dnd5eEvents.CombatEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.CombatEndEvent{SubjectID: s.casterID}))

	s.Require().Len(s.removals, 3)
	s.Equal(s.child().ConditionRef, s.removals[0].ConditionRef)
	s.Equal("skeleton-1", s.removals[1].MemberID)
	s.Equal(refs.Conditions.Concentrating().String(), s.removals[2].ConditionRef,
		"the owner goes last, so the children come off while it still names them")
	s.Equal(s.casterID, s.removals[2].MemberID)
	for _, removal := range s.removals {
		s.Equal(ConcentrationEndedCombatEnd, removal.Reason)
	}
	s.False(condition.IsApplied())
}

// R6: a child consumed by anything else leaves the list, and the last one out
// ends the hold — otherwise a bard whose True Strike was spent still reads as
// concentrating and still drops "nothing" on the next concentration cast.
func (s *ConcentratingConditionSuite) TestTheLastChildLeavingEndsIt() {
	condition := s.applied()
	skeleton := dnd5eEvents.ChildRef{MemberID: "skeleton-1", ConditionRef: refs.Conditions.Charmed().String()}
	s.Require().NoError(condition.AddChild(s.ctx, skeleton))

	s.publishRemoval(skeleton, "consumed")
	s.Require().True(condition.IsApplied(), "one child left, so the spell is still up")
	s.Require().Len(condition.Children, 1)

	s.publishRemoval(s.child(), "consumed")

	s.False(condition.IsApplied())
	s.Require().NotEmpty(s.removals)
	last := s.removals[len(s.removals)-1]
	s.Equal(refs.Conditions.Concentrating().String(), last.ConditionRef)
	s.Equal(ConcentrationEndedSpellEnded, last.Reason)
}

func (s *ConcentratingConditionSuite) TestSomebodyElsesRemovalIsIgnored() {
	condition := s.applied()

	s.publishRemoval(dnd5eEvents.ChildRef{
		MemberID:     "fighter-2",
		ConditionRef: refs.Conditions.TrueStrike().String(),
	}, "expired")

	s.True(condition.IsApplied(), "an address is a member AND a ref")
	s.Len(condition.Children, 1)
}

func (s *ConcentratingConditionSuite) TestItEndsOnItsOwnClock() {
	condition := s.applied()

	s.endTurn(s.casterID)
	s.True(condition.IsApplied(), "a cantrip is cast on the caster's own turn, so the first end is that turn's")

	s.endTurn(s.casterID)

	s.False(condition.IsApplied())
	s.Require().Len(s.removals, 2)
	s.Equal(ConcentrationEndedDuration, s.removals[1].Reason)
}

func (s *ConcentratingConditionSuite) TestSomebodyElsesTurnEndDoesNotCountDown() {
	condition := s.applied()

	s.endTurn("fighter-2")
	s.endTurn("skeleton-1")

	s.True(condition.IsApplied())
	s.Equal(2, condition.TurnEndsLeft)
}

func (s *ConcentratingConditionSuite) TestALongRestTakesItLikeAnyCondition() {
	condition := s.applied()

	s.Require().NoError(dnd5eEvents.RestTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.RestEvent{
		CharacterID: s.casterID,
		RestType:    coreResources.ResetLongRest,
	}))

	s.False(condition.IsApplied())
}

func (s *ConcentratingConditionSuite) TestItRoundTripsWithItsChildAddresses() {
	condition := NewConcentratingCondition(s.casterID, s.spellRef, TrueStrikeName, 2)
	skeleton := dnd5eEvents.ChildRef{MemberID: "skeleton-1", ConditionRef: refs.Conditions.Charmed().String()}
	s.Require().NoError(condition.AddChild(s.ctx, s.child()))
	s.Require().NoError(condition.AddChild(s.ctx, skeleton))

	raw, err := condition.ToJSON()
	s.Require().NoError(err)

	loaded, err := LoadJSON(raw)
	s.Require().NoError(err)

	back, ok := loaded.(*ConcentratingCondition)
	s.Require().True(ok)
	s.Equal(s.casterID, back.MemberID)
	s.Equal(s.spellRef, back.SpellRef)
	s.Equal(TrueStrikeName, back.SpellName)
	s.Equal(2, back.TurnEndsLeft)
	s.Equal([]dnd5eEvents.ChildRef{s.child(), skeleton}, back.Children,
		"addresses rather than pointers, which is what survives this trip")
}

func (s *ConcentratingConditionSuite) TestTheFactoryRefusesAHoldOnNothing() {
	_, err := CreateFromRef(&CreateFromRefInput{
		Ref:      refs.Conditions.Concentrating().String(),
		MemberID: s.casterID,
		Config:   json.RawMessage(`{"turn_ends":2}`),
	})

	s.Require().ErrorContains(err, "spell_ref")
}

func (s *ConcentratingConditionSuite) TestTheFactoryRefusesAClockThatAlreadyRanOut() {
	_, err := CreateFromRef(&CreateFromRefInput{
		Ref:       refs.Conditions.Concentrating().String(),
		MemberID:  s.casterID,
		SourceRef: s.spellRef,
	})

	s.Require().ErrorContains(err, "turn_ends")
}

func (s *ConcentratingConditionSuite) TestTheFactoryTakesTheSpellFromWhatGrantedIt() {
	out, err := CreateFromRef(&CreateFromRefInput{
		Ref:       refs.Conditions.Concentrating().String(),
		MemberID:  s.casterID,
		SourceRef: s.spellRef,
		Config:    json.RawMessage(`{"turn_ends":2,"spell_name":"True Strike"}`),
	})
	s.Require().NoError(err)

	built, ok := out.Condition.(*ConcentratingCondition)
	s.Require().True(ok)
	s.Equal(s.spellRef, built.SpellRef)
}

// A display that could not name this condition would be a hard error at
// projection time rather than a missing badge.
func (s *ConcentratingConditionSuite) TestItIsInTheDisplayCatalog() {
	display, ok := DisplayFor(*refs.Conditions.Concentrating())

	s.Require().True(ok)
	s.Equal(ConcentratingName, display.Name)
}

func (s *ConcentratingConditionSuite) TestAnUnparseableSpellRefFailsClosed() {
	condition := NewConcentratingCondition(s.casterID, "not-a-ref", "Mystery", 2)
	s.Require().NoError(condition.Apply(s.ctx, s.bus))

	event := &dnd5eEvents.DamageTakenEvent{MemberID: s.casterID, Amount: 12}
	err := dnd5eEvents.DamageTakenTopic.On(s.bus).Publish(s.ctx, event)

	s.Require().Error(err, "a check whose cause cannot name the spell has nothing at stake")
	s.Empty(event.FollowUps)
}

func (s *ConcentratingConditionSuite) publishRemoval(address dnd5eEvents.ChildRef, reason string) {
	s.Require().NoError(dnd5eEvents.ConditionRemovedTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.ConditionRemovedEvent{
			MemberID:     address.MemberID,
			ConditionRef: address.ConditionRef,
			Reason:       reason,
		}))
}

func (s *ConcentratingConditionSuite) endTurn(subjectID string) {
	s.Require().NoError(dnd5eEvents.TurnEndTopic.On(s.bus).Publish(s.ctx,
		dnd5eEvents.TurnEndEvent{SubjectID: subjectID, Round: 1}))
}
