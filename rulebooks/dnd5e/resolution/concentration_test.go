// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// ConcentrationTestSuite drives the check that runs INSIDE the interaction
// that caused it: damage lands, the caster's hold answers with a follow-up,
// and the machine that applied the damage runs it before it finishes.
//
// Nothing here starts a second interaction, and nothing outside a machine
// rolls.
type ConcentrationTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestConcentrationSuite(t *testing.T) {
	suite.Run(t, new(ConcentrationTestSuite))
}

func (s *ConcentrationTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// clawBonus is high enough that every scripted d20 hits, so the attack roll
// never competes with the save roll for a face.
const clawBonus = 20

func (s *ConcentrationTestSuite) fixtures() *ContestDamageTestSuite {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())
	fixtures.ctx = s.ctx

	return fixtures
}

// claw is a swing with no rider and no ability contribution, so the only thing
// the damage number depends on is the scripted dice.
func claw(dice string) combatActions.Definition {
	return combatActions.Definition{
		Ref:  *refs.Weapons.Greatsword(),
		Name: "Claw",
		Attack: &combatActions.AttackProfile{
			Category:    combatActions.AttackCategoryWeapon,
			Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
			AttackBonus: clawBonus,
			Damage:      []damage.Damage{{Dice: dice, Type: damage.Slashing}},
		},
	}
}

// trueStrikeAddress is where True Strike's child sits: on the CASTER, keyed to
// the creature it was aimed at.
func trueStrikeAddress(casterID string) dnd5eEvents.ChildRef {
	return dnd5eEvents.ChildRef{
		MemberID:     casterID,
		ConditionRef: refs.Conditions.TrueStrike().String(),
	}
}

func concentratingAddress(casterID string) dnd5eEvents.ChildRef {
	return dnd5eEvents.ChildRef{
		MemberID:     casterID,
		ConditionRef: refs.Conditions.Concentrating().String(),
	}
}

// holding builds the two blobs a caster mid-True-Strike carries: the hold, and
// the child it owns.
func (s *ConcentrationTestSuite) holding(casterID, targetID string) []json.RawMessage {
	hold := conditions.NewConcentratingCondition(
		casterID, refs.Spells.TrueStrike().String(), "True Strike", spells.TrueStrikeTurnEnds)
	s.Require().NoError(hold.AddChild(s.ctx, trueStrikeAddress(casterID)))

	holdJSON, err := hold.ToJSON()
	s.Require().NoError(err)

	child := conditions.NewTrueStrikeCondition(casterID, targetID, refs.Spells.TrueStrike().String())
	childJSON, err := child.ToJSON()
	s.Require().NoError(err)

	return []json.RawMessage{holdJSON, childJSON}
}

// removalLog records every removal fact in publication order, which is the
// only place the strip is visible: nothing reaches across to another sheet.
func (s *ConcentrationTestSuite) removalLog(bus events.EventBus) *[]dnd5eEvents.ConditionRemovedEvent {
	seen := &[]dnd5eEvents.ConditionRemovedEvent{}
	_, err := dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			*seen = append(*seen, event)
			return nil
		})
	s.Require().NoError(err)

	return seen
}

// strike runs one claw at the hero on a bus the scene can listen to.
func (s *ConcentrationTestSuite) strike(
	hero *character.Data, definition combatActions.Definition, roller dice.Roller,
	bus events.EventBus, extra ...Participant,
) (*Output, error) {
	fixtures := s.fixtures()
	participants := append(
		[]Participant{{Character: hero}, {Monster: fixtures.wolfData()}}, extra...)

	return resolveOn(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World:        fixtures.world(),
		Participants: participants,
		Machine: NewStrike(&StrikeInput{
			AttackerID: wolfID,
			TargetID:   heroID,
			Definition: definition,
			Roller:     roller,
		}),
	}, newSurface(bus))
}

func (s *ConcentrationTestSuite) struck(out *Output) StrikeOutcome {
	outcome, ok := out.Outcome.(StrikeOutcome)
	s.Require().True(ok, "a strike produces a StrikeOutcome")

	return outcome
}

func (s *ConcentrationTestSuite) conditionRefs(out *Output, id string) []string {
	for _, data := range out.DirtyCharacters {
		if data.ID != id {
			continue
		}
		found := make([]string, 0, len(data.Conditions))
		for _, raw := range data.Conditions {
			var peek struct {
				Ref string `json:"ref"`
			}
			s.Require().NoError(json.Unmarshal(raw, &peek))
			found = append(found, peek.Ref)
		}

		return found
	}
	s.Require().Failf("sheet not dirty", "%s did not come back changed", id)

	return nil
}

// THE HEADLINE. A skeleton hits the bard, and the Constitution check happens
// inside the swing: one publish, one nested Request, and the failed check ends
// the hold and every child — all inside one Resolve, with no second
// interaction started by anyone.
func (s *ConcentrationTestSuite) TestAStrikeOnAConcentratingCasterRunsTheCheckInside() {
	bus := events.NewEventBus()
	removals := s.removalLog(bus)

	// singles: the claw's d20, then the concentration save's. CON +2 against
	// DC 10 fails on a 3.
	out, err := s.strike(
		s.fixtures().saver(40, s.holding(heroID, wolfID)...),
		claw("1d6"),
		&sequenceRoller{singles: []int{straightRoll, straightRoll}, pair: []int{6}},
		bus,
	)
	s.Require().NoError(err)

	struck := s.struck(out)
	s.Require().Equal(6, struck.Damage)
	s.Require().Len(struck.FollowUps, 1, "one hold, one check")

	followUp := struck.FollowUps[0]
	s.Equal(heroID, followUp.SaverID)
	s.Equal(abilities.CON, followUp.Ability)
	s.Require().NotNil(followUp.Save.Result)
	s.Equal(conditions.ConcentrationDCFloor, followUp.Save.Result.DC, "6 damage asks for the floor")
	s.False(followUp.Save.Result.Success)

	s.Require().NotNil(followUp.Ended, "a failed check ends the hold")
	s.Equal(heroID, followUp.Ended.CasterID)
	s.Equal(conditions.ConcentrationEndedDamage, followUp.Ended.Reason)
	s.Require().NotNil(followUp.Ended.Spell)
	s.Equal(refs.Spells.TrueStrike().String(), followUp.Ended.Spell.String())
	s.Equal("True Strike", followUp.Ended.SpellName,
		"the name is read off the hold before the strip takes it away")
	s.Equal([]dnd5eEvents.ChildRef{trueStrikeAddress(heroID)}, followUp.Ended.Removed)

	// The strip, as facts on the interaction's own bus: the owner first, then
	// its child. Exactly one owner removal — publishing the children first
	// would let the hold end itself a second time with the wrong reason.
	s.Require().Len(*removals, 2)
	s.Equal(concentratingAddress(heroID).ConditionRef, (*removals)[0].ConditionRef)
	s.Equal(conditions.ConcentrationEndedDamage, (*removals)[0].Reason)
	s.Equal(trueStrikeAddress(heroID).ConditionRef, (*removals)[1].ConditionRef)

	// And on the sheet: both are gone.
	s.Empty(s.conditionRefs(out, heroID), "the child went with the parent")
}

// A made check removes nothing and still records roll, total and DC.
func (s *ConcentrationTestSuite) TestAMadeCheckRemovesNothing() {
	bus := events.NewEventBus()
	removals := s.removalLog(bus)

	out, err := s.strike(
		s.fixtures().saver(40, s.holding(heroID, wolfID)...),
		claw("1d6"),
		&sequenceRoller{singles: []int{straightRoll, 18}, pair: []int{6}},
		bus,
	)
	s.Require().NoError(err)

	struck := s.struck(out)
	s.Require().Len(struck.FollowUps, 1)
	s.Require().NotNil(struck.FollowUps[0].Save.Result)
	s.True(struck.FollowUps[0].Save.Result.Success)
	s.Equal(18, struck.FollowUps[0].Save.Result.Roll)
	s.Equal(20, struck.FollowUps[0].Save.Result.Total, "18 plus CON +2")
	s.Equal(conditions.ConcentrationDCFloor, struck.FollowUps[0].Save.Result.DC)
	s.Nil(struck.FollowUps[0].Ended, "nothing ended")

	s.Empty(*removals, "a made check publishes no removal")
	s.Len(s.conditionRefs(out, heroID), 2, "the hold and its child are both still there")
}

// The DC is half the damage when that is higher than the floor, settled by the
// condition from the APPLIED amount and travelling as a static number.
func (s *ConcentrationTestSuite) TestTheDCIsHalfTheDamageWhenThatIsHigher() {
	out, err := s.strike(
		s.fixtures().saver(40, s.holding(heroID, wolfID)...),
		claw("4d6"),
		&sequenceRoller{singles: []int{straightRoll, 18}, pair: []int{6, 6, 6, 6}},
		events.NewEventBus(),
	)
	s.Require().NoError(err)

	struck := s.struck(out)
	s.Require().Equal(24, struck.Damage)
	s.Require().Len(struck.FollowUps, 1)
	s.Require().NotNil(struck.FollowUps[0].Save.Result)
	s.Equal(12, struck.FollowUps[0].Save.Result.DC, "half of 24")
}

// A member who is not concentrating produces zero follow-ups and zero nested
// steps. The fact is still published; nobody answers it.
func (s *ConcentrationTestSuite) TestADefenderHoldingNothingOwesNothing() {
	bus := events.NewEventBus()
	removals := s.removalLog(bus)

	out, err := s.strike(
		s.fixtures().saver(40),
		claw("1d6"),
		&sequenceRoller{singles: []int{straightRoll}, pair: []int{6}},
		bus,
	)
	s.Require().NoError(err)

	s.Empty(s.struck(out).FollowUps)
	s.Empty(*removals)
}

// Two follow-ups from one interaction are two nested checks, in append order,
// with no special case — the shape movementMachine.react has for its triggers.
//
// The second is appended by a subscriber standing in for a second hold, which
// is the only way one blow reaches two holders today: damage lands on one
// sheet, and the fact is what anybody else answers.
func (s *ConcentrationTestSuite) TestTwoFollowUpsAreTwoNestedChecks() {
	bus := events.NewEventBus()
	fixtures := s.fixtures()

	_, err := dnd5eEvents.DamageTakenTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, event *dnd5eEvents.DamageTakenEvent) error {
			event.FollowUps = append(event.FollowUps, dnd5eEvents.FollowUp{
				SaverID: bardID,
				Ability: abilities.CON,
				DC:      conditions.ConcentrationDCFloor,
				Cause: dnd5eEvents.SaveCause{
					Trigger:   dnd5eEvents.SaveTriggerConcentration,
					EffectRef: refs.Spells.TrueStrike(),
				},
				OnFailure: dnd5eEvents.Consequence{
					Owner:  concentratingAddress(bardID),
					Reason: conditions.ConcentrationEndedDamage,
				},
			})

			return nil
		})
	s.Require().NoError(err)

	out, err := s.strike(
		fixtures.saver(40, s.holding(heroID, wolfID)...),
		claw("1d6"),
		&sequenceRoller{singles: []int{straightRoll, 18, 18}, pair: []int{6}},
		bus,
		Participant{Character: fixtures.bard(1)},
	)
	s.Require().NoError(err)

	struck := s.struck(out)
	s.Require().Len(struck.FollowUps, 2, "two follow-ups, two checks")

	// APPEND ORDER, which is subscription order: this scene subscribed before
	// Resolve attached the hero's hold, so its follow-up is first. That is the
	// honest reading and it is not a rule — nothing about the outcome depends
	// on which check ran first, which is exactly why one step per follow-up is
	// enough and no ordering policy is needed.
	s.Equal(bardID, struck.FollowUps[0].SaverID)
	s.Equal(heroID, struck.FollowUps[1].SaverID)
}

// R2's recursion bound, stated rather than discovered: a contest built from a
// follow-up may not declare damage, so the depth is exactly one.
//
// The day a consequence damages, THIS is what has to be replaced with an
// explicit bound rather than deleted.
func (s *ConcentrationTestSuite) TestAFollowUpContestMayNotDeclareDamage() {
	s.Require().Error(refuseFollowUpDamage(&ContestInput{
		Damage: []damage.Damage{{Dice: "1d4", Type: damage.Psychic}},
	}), "damage on a follow-up's contest is refused")

	built, err := followUpContest(dnd5eEvents.FollowUp{
		SaverID: heroID,
		Ability: abilities.CON,
		DC:      conditions.ConcentrationDCFloor,
		OnFailure: dnd5eEvents.Consequence{
			Owner:  concentratingAddress(heroID),
			Reason: conditions.ConcentrationEndedDamage,
		},
	}, nil)
	s.Require().NoError(err)
	s.Empty(built.Damage, "and none is ever built")
	s.Require().NotNil(built.Removal)
	s.Equal(concentratingAddress(heroID), built.Removal.Owner)
}

// A follow-up naming no owner is refused: a check with nothing to end is a roll
// the player is asked for and nothing happens.
func (s *ConcentrationTestSuite) TestAFollowUpWithNoOwnerIsRefused() {
	_, err := followUpContest(dnd5eEvents.FollowUp{
		SaverID: heroID,
		Ability: abilities.CON,
		DC:      conditions.ConcentrationDCFloor,
	}, nil)
	s.Require().ErrorIs(err, ErrBadAction)
}

// castingBard is a bard with an action to spend and whatever it is already
// holding.
func (s *ConcentrationTestSuite) castingBard(actions int, conds ...json.RawMessage) *character.Data {
	data := s.fixtures().bard(actions)
	data.Conditions = conds

	return data
}

// resolveCast runs one cast at the door's price, on a bus the scene can hear.
func (s *ConcentrationTestSuite) resolveCast(
	bard *character.Data, machine Machine, bus events.EventBus,
) (*Output, error) {
	fixtures := s.fixtures()

	return resolveOn(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		World: fixtures.world(),
		Participants: []Participant{
			{Character: fixtures.saver(14)}, {Monster: fixtures.wolfData()}, {Character: bard},
		},
		Machine: machine,
		Cost:    castCost(),
	}, newSurface(bus))
}

func (s *ConcentrationTestSuite) trueStrike() Machine {
	definition := spells.CastDefinition(spells.TrueStrike, spellSaveDC)
	s.Require().NotNil(definition)
	definition.Cost = oneAction()

	machine, err := NewAction(&ActionInput{
		Definition: *definition, AttackerID: bardID, TargetID: wolfID,
	})
	s.Require().NoError(err)

	return machine
}

// held reads the hold off a sheet, which is where the address list lives.
func (s *ConcentrationTestSuite) held(out *Output, id string) conditions.ConcentratingConditionData {
	var found []conditions.ConcentratingConditionData
	for _, data := range out.DirtyCharacters {
		if data.ID != id {
			continue
		}
		for _, raw := range data.Conditions {
			var blob conditions.ConcentratingConditionData
			s.Require().NoError(json.Unmarshal(raw, &blob))
			if blob.Ref != nil && blob.Ref.ID == refs.Conditions.Concentrating().ID {
				found = append(found, blob)
			}
		}
	}
	s.Require().Len(found, 1, "exactly one hold at a time")

	return found[0]
}

// A concentration cast registers what it delivered on the owner, so ending the
// hold can end the child.
func (s *ConcentrationTestSuite) TestTheCastRegistersItsDeliveredChildOnTheOwner() {
	out, err := s.resolveCast(s.castingBard(1), s.trueStrike(), events.NewEventBus())
	s.Require().NoError(err)

	outcome, ok := out.Outcome.(CastOutcome)
	s.Require().True(ok)
	s.Nil(outcome.Dropped, "a caster holding nothing displaces nothing")

	hold := s.held(out, bardID)
	s.Equal(refs.Spells.TrueStrike().String(), hold.SpellRef)
	s.Equal(spells.TrueStrikeTurnEnds, hold.TurnEndsLeft)
	s.Equal([]dnd5eEvents.ChildRef{trueStrikeAddress(bardID)}, hold.Children)
}

// The recast drop is the FIRST yielded step after the charge: the old spell is
// gone before the new one delivers anything.
func (s *ConcentrationTestSuite) TestASecondConcentrationCastDropsTheFirst() {
	bus := events.NewEventBus()

	// One ordered log of both facts, because the ORDER is the ruling:
	// preflight -> charge -> drop the old spell -> resolve the new one.
	var order []string
	_, err := dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			order = append(order, "removed:"+event.ConditionRef+":"+event.Reason)
			return nil
		})
	s.Require().NoError(err)
	_, err = dnd5eEvents.ConditionAppliedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
			order = append(order, "applied:"+string(event.Type))
			return nil
		})
	s.Require().NoError(err)

	out, err := s.resolveCast(
		s.castingBard(1, s.holding(bardID, heroID)...), s.trueStrike(), bus)
	s.Require().NoError(err)

	s.Equal([]string{
		"removed:" + concentratingAddress(bardID).ConditionRef + ":" + conditions.ConcentrationEndedRecast,
		"removed:" + trueStrikeAddress(bardID).ConditionRef + ":" + conditions.ConcentrationEndedRecast,
		"applied:" + refs.Conditions.TrueStrike().ID,
		"applied:" + refs.Conditions.Concentrating().ID,
	}, order)

	hold := s.held(out, bardID)
	s.Equal([]dnd5eEvents.ChildRef{trueStrikeAddress(bardID)}, hold.Children,
		"exactly one hold, owning exactly the new cast's child")

	// The drop rides the outcome, so the record can say WHICH spell ended and
	// why. There is no check to report: a recast ends the first spell outright.
	outcome, ok := out.Outcome.(CastOutcome)
	s.Require().True(ok)
	s.Require().NotNil(outcome.Dropped)
	s.Equal(bardID, outcome.Dropped.CasterID)
	s.Equal("True Strike", outcome.Dropped.SpellName)
	s.Equal(conditions.ConcentrationEndedRecast, outcome.Dropped.Reason)
	s.Equal([]dnd5eEvents.ChildRef{trueStrikeAddress(bardID)}, outcome.Dropped.Removed)
}

// A cast refused at the door drops nothing, which is what RAW means by "when
// you cast another spell that requires concentration".
func (s *ConcentrationTestSuite) TestACastRefusedAtTheDoorDropsNothing() {
	bus := events.NewEventBus()
	removals := s.removalLog(bus)

	_, err := s.resolveCast(
		s.castingBard(0, s.holding(bardID, heroID)...), s.trueStrike(), bus)
	s.Require().Error(err)
	s.Empty(*removals, "nothing was cast, so nothing was displaced")
}
