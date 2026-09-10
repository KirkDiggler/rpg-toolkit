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
	ended    []dnd5eEvents.ConcentrationEndedEvent
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
	s.ended = nil

	_, err := dnd5eEvents.ConditionRemovedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			s.removals = append(s.removals, event)
			return nil
		})
	s.Require().NoError(err)

	_, err = dnd5eEvents.ConcentrationEndedTopic.On(s.bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConcentrationEndedEvent) error {
			s.ended = append(s.ended, event)
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

// EVERY end path publishes exactly one fact, and it is the only thing in the
// train that says why.
//
// Six reasons, and each one happens somewhere different: a failed check inside
// a strike, a recast inside the next cast, the clock and the fight on a
// boundary, the last child leaving, the caster going down. Nothing outside this
// condition can see all six, and a removal landing on a skeleton's sheet with
// no beat near it reads as a random drop. Three of the six arrive here as a
// removal somebody ELSE published, which is why this table covers both shapes.
func (s *ConcentratingConditionSuite) TestEveryEndPathPublishesOneFact() {
	skeleton := dnd5eEvents.ChildRef{MemberID: "skeleton-1", ConditionRef: refs.Conditions.Charmed().String()}

	cases := []struct {
		name   string
		reason string
		end    func(condition *ConcentratingCondition)
	}{
		{"the caster drops to 0", ConcentrationEndedCasterDown, func(*ConcentratingCondition) {
			s.damage(testCasterID, 30, true)
		}},
		{"the clock runs out", ConcentrationEndedDuration, func(*ConcentratingCondition) {
			s.endTurn(testCasterID)
			s.endTurn(testCasterID)
		}},
		{"the fight ends", ConcentrationEndedCombatEnd, func(*ConcentratingCondition) {
			s.Require().NoError(dnd5eEvents.CombatEndTopic.On(s.bus).Publish(s.ctx,
				dnd5eEvents.CombatEndEvent{SubjectID: testCasterID}))
		}},
		{"a failed check strips it", ConcentrationEndedDamage, func(condition *ConcentratingCondition) {
			// What the consequence's delivery does: the owner's address, then
			// the children it named.
			s.publishRemoval(dnd5eEvents.ChildRef{
				MemberID:     condition.MemberID,
				ConditionRef: condition.Ref().String(),
			}, ConcentrationEndedDamage)
		}},
		{"another concentration cast displaces it", ConcentrationEndedRecast, func(condition *ConcentratingCondition) {
			s.publishRemoval(dnd5eEvents.ChildRef{
				MemberID:     condition.MemberID,
				ConditionRef: condition.Ref().String(),
			}, ConcentrationEndedRecast)
		}},
		{"a long rest takes it", "long rest", func(*ConcentratingCondition) {
			s.Require().NoError(dnd5eEvents.RestTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.RestEvent{
				CharacterID: testCasterID,
				RestType:    coreResources.ResetLongRest,
			}))
		}},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			condition := s.applied()
			s.Require().NoError(condition.AddChild(s.ctx, skeleton))

			tc.end(condition)

			s.Require().Len(s.ended, 1, "one fact per end, never two and never none")
			fact := s.ended[0]
			s.Equal(testCasterID, fact.CasterID)
			s.Equal(s.spellRef, fact.SpellRef)
			s.Equal(TrueStrikeName, fact.SpellName)
			s.Equal(tc.reason, fact.Reason)
			s.Equal([]dnd5eEvents.ChildRef{s.child(), skeleton}, fact.Removed,
				"and it names every address that came off with it")
			s.False(condition.IsApplied())
		})
	}
}

// The sixth reason, which is the one nobody else can publish: the hold ending
// because its last child ended on its own account.
func (s *ConcentratingConditionSuite) TestBaneClockSkipsCastingTurnThenCountsTenSubsequentCasterEndsAcrossReload() {
	condition := NewConcentratingConditionWithInput(NewConcentratingConditionInput{
		MemberID:         s.casterID,
		SourceID:         s.casterID,
		SpellRef:         refs.Spells.Bane().String(),
		SpellName:        "Bane",
		TurnEnds:         10,
		SkipFirstTurnEnd: true,
	})
	child := dnd5eEvents.ConditionAddress{
		MemberID: "target-1", ConditionRef: refs.Conditions.Baned().String(), SourceID: s.casterID,
	}
	s.Require().NoError(condition.AddChild(s.ctx, child))
	s.Require().NoError(condition.Apply(s.ctx, s.bus))

	s.endTurn(s.casterID)
	s.Equal(10, condition.TurnEndsLeft, "the casting turn consumes only the persisted grace")
	s.False(condition.SkipNextTurnEnd)
	s.endTurn("another-member")
	s.Equal(10, condition.TurnEndsLeft, "recipient and other member turns do not own this clock")

	for range 5 {
		s.endTurn(s.casterID)
	}
	s.Equal(5, condition.TurnEndsLeft)
	raw, err := condition.ToJSON()
	s.Require().NoError(err)
	s.Require().NoError(condition.Remove(s.ctx, s.bus))

	loadedBehavior, err := LoadJSON(raw)
	s.Require().NoError(err)
	loaded := loadedBehavior.(*ConcentratingCondition)
	s.False(loaded.SkipNextTurnEnd, "consumed grace stays consumed after reload")
	s.Equal(5, loaded.TurnEndsLeft)
	s.Require().NoError(loaded.Apply(s.ctx, s.bus))

	for range 4 {
		s.endTurn(s.casterID)
	}
	s.Empty(s.removals)
	s.endTurn(s.casterID)
	s.Require().Len(s.removals, 2, "the tenth subsequent caster end removes child then owner")
	s.Equal(child, s.removals[0].Address())
	s.Equal(loaded.ConditionAddress(), s.removals[1].Address())
}

func (s *ConcentratingConditionSuite) TestBaneOwnerRejectsUnqualifiedOrForeignChildren() {
	condition := NewConcentratingConditionWithInput(NewConcentratingConditionInput{
		MemberID: s.casterID, SourceID: s.casterID, SpellRef: refs.Spells.Bane().String(),
		SpellName: "Bane", TurnEnds: 10, SkipFirstTurnEnd: true,
	})

	err := condition.AddChild(s.ctx, dnd5eEvents.ConditionAddress{
		MemberID: "target-1", ConditionRef: refs.Conditions.Baned().String(),
	})
	s.Require().ErrorContains(err, "source")
	err = condition.AddChild(s.ctx, dnd5eEvents.ConditionAddress{
		MemberID: "target-1", ConditionRef: refs.Conditions.Baned().String(), SourceID: "bard-b",
	})
	s.Require().ErrorContains(err, "source")
}

func (s *ConcentratingConditionSuite) TestTheLastChildLeavingPublishesSpellEnded() {
	condition := s.applied()

	s.publishRemoval(s.child(), "consumed")

	s.Require().Len(s.ended, 1)
	s.Equal(ConcentrationEndedSpellEnded, s.ended[0].Reason)
	s.Empty(s.ended[0].Removed, "the child was already gone; the hold stripped nothing")
	s.False(condition.IsApplied())
}

// A child that came off BECAUSE the hold ended is not the last child leaving.
// Publishing the children first and the owner second must still produce one
// fact reading `damage`, not a `spell_ended` followed by silence.
func (s *ConcentratingConditionSuite) TestChildrenStrippedFirstStillEndWithTheRealReason() {
	condition := s.applied()

	s.publishRemoval(s.child(), ConcentrationEndedDamage)
	s.Require().Empty(s.ended, "a child removed by the strip is not the spell ending on its own")
	s.Require().True(condition.IsApplied())

	s.publishRemoval(dnd5eEvents.ChildRef{
		MemberID:     condition.MemberID,
		ConditionRef: condition.Ref().String(),
	}, ConcentrationEndedDamage)

	s.Require().Len(s.ended, 1)
	s.Equal(ConcentrationEndedDamage, s.ended[0].Reason)
	s.False(condition.IsApplied())
}
