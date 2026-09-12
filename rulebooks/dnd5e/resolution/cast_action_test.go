// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// CastActionTestSuite drives the cast door end to end: content compiles to a
// definition, NewAction reads the profile's arms, and the machine that runs is
// one that already existed.
//
// The two cantrips are the two halves of every cantrip — roll to see whether
// you deliver it, and just deliver it — so between them they are the whole
// branch.
type CastActionTestSuite struct {
	suite.Suite

	ctx context.Context
}

func TestCastActionSuite(t *testing.T) {
	suite.Run(t, new(CastActionTestSuite))
}

func (s *CastActionTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// castDamage reuses the damage suite's world, sheets and roller: the cast door
// is a second way into the same interaction, and a second set of fixtures would
// be a second thing to keep true.
func (s *CastActionTestSuite) fixtures() *ContestDamageTestSuite {
	fixtures := &ContestDamageTestSuite{}
	fixtures.SetT(s.T())
	fixtures.ctx = s.ctx

	return fixtures
}

// oneAction is a cantrip's whole price.
func oneAction() *combat.SpendProfile {
	return &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}}
}

func castCost() *Cost {
	return &Cost{
		PayerID: bardID,
		Profile: oneAction(),
		Turn:    &Turn{Number: mockeryTurn, Speed: mockerySpeed},
	}
}

// cast builds the machine the door would build for one cantrip.
func (s *CastActionTestSuite) cast(id spells.Spell, casterID, targetID string, roll int) Machine {
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: id, SpellSaveDC: spellSaveDC})
	s.Require().NotNil(definition, "this build has cast content for %s", id)
	definition.Cost = oneAction()

	machine, err := NewAction(&ActionInput{
		Definition: *definition,
		AttackerID: casterID,
		TargetIDs:  []string{targetID},
		Roller:     facedRoller{d20: roll, other: psychicFace},
	})
	s.Require().NoError(err)

	return machine
}

// castParams reads one persisted condition's raw configuration off a sheet, so
// the counterpart binding is asserted against what was actually stored rather
// than against what was passed in.
func (s *CastActionTestSuite) castParams(data *character.Data, conditionRef string) map[string]any {
	for _, raw := range data.Conditions {
		var peek map[string]any
		s.Require().NoError(json.Unmarshal(raw, &peek))
		if ref, _ := peek["ref"].(string); ref == conditionRef {
			return peek
		}
	}
	s.Require().Failf("condition not persisted", "%s is not on %s", conditionRef, data.ID)

	return nil
}

// castOutcome is what every scene here reads: ONE outcome type for both halves
// of the door, which is what the session switches on.
func (s *CastActionTestSuite) castOutcome(out *Output) CastOutcome {
	outcome, ok := out.Outcome.(CastOutcome)
	s.Require().True(ok, "a cast produces a CastOutcome, gated or not")

	return outcome
}

// THE HEADLINE FOR THE GATED HALF. Vicious Mockery, compiled from content,
// entered through the door at a price: the save fails, 1d4 psychic lands, and
// the rider goes on the target carrying the bard who imposed it.
type countingCastRoller struct {
	calls int
	facedRoller
}

func (r *countingCastRoller) Roll(ctx context.Context, size int) (int, error) {
	r.calls++
	return r.facedRoller.Roll(ctx, size)
}

func (r *countingCastRoller) RollN(ctx context.Context, count, size int) ([]int, error) {
	r.calls++
	return r.facedRoller.RollN(ctx, count, size)
}

func baneDefinition() *combatActions.Definition {
	return spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Bane, SpellSaveDC: spellSaveDC})
}

func baneCost() *Cost {
	return &Cost{
		PayerID: bardID,
		Profile: baneDefinition().Cost,
		Turn:    &Turn{Number: mockeryTurn, Speed: mockerySpeed},
	}
}

func baneCaster(actions, slots int) *character.Data {
	caster := (&ContestDamageTestSuite{}).bard(actions)
	caster.Resources = map[coreResources.ResourceKey]character.RecoverableResourceData{
		resources.SpellSlotLevel1: {
			Current: slots, Maximum: 2, ResetType: coreResources.ResetLongRest,
		},
	}
	return caster
}

func (s *CastActionTestSuite) TestBanePaysOnceAndResolvesTargetsInCallerOrder() {
	for _, targets := range [][]string{
		{heroID},
		{wolfID, heroID},
		{heroID, wolfID, bardID},
	} {
		s.Run(fmt.Sprintf("%d targets", len(targets)), func() {
			fixtures := s.fixtures()
			roller := &countingCastRoller{facedRoller: facedRoller{d20: 1, other: 4}}
			machine, err := NewAction(&ActionInput{
				Definition: *baneDefinition(), AttackerID: bardID,
				TargetIDs: targets, Roller: roller,
			})
			s.Require().NoError(err)

			out, err := fixtures.resolve(fixtures.saver(14), machine, baneCost(), baneCaster(1, 2))
			s.Require().NoError(err)
			outcome := s.castOutcome(out)
			gotTargets := make([]string, len(outcome.Targets))
			for i, target := range outcome.Targets {
				gotTargets[i] = target.TargetID
				s.Require().False(target.Save.Succeeded)
				s.Require().Len(target.Applied, 1)
			}
			s.Equal(targets, gotTargets)

			payer := fixtures.sheet(out, bardID)
			s.Zero(payer.ActionEconomy.ActionsRemaining)
			s.Equal(1, payer.Resources[resources.SpellSlotLevel1].Current,
				"one ordered fan-out pays one level-1 slot")
		})
	}
}

func (s *CastActionTestSuite) TestBaneRefusesMalformedWholeTargetListsBeforeRNG() {
	for _, tc := range []struct {
		name    string
		targets []string
	}{
		{name: "empty list", targets: nil},
		{name: "empty id", targets: []string{""}},
		{name: "duplicate", targets: []string{heroID, heroID}},
		{name: "fourth target", targets: []string{heroID, wolfID, bardID, "fourth"}},
	} {
		s.Run(tc.name, func() {
			roller := &countingCastRoller{facedRoller: facedRoller{d20: 1, other: 4}}
			caster := baneCaster(1, 2)
			caster.Conditions = []json.RawMessage{baneOwnerJSON(s.T(), bardID, 5)}
			before, err := json.Marshal(caster)
			s.Require().NoError(err)
			machine, err := NewAction(&ActionInput{
				Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: tc.targets, Roller: roller,
			})
			s.Error(err)
			s.Nil(machine)
			s.Zero(roller.calls)
			after, marshalErr := json.Marshal(caster)
			s.Require().NoError(marshalErr)
			s.JSONEq(string(before), string(after), "action, slot pool, and old concentration are unchanged")
		})
	}
}

func (s *CastActionTestSuite) TestBaneRejectsAnInvalidLaterTargetBeforeRNGOrMutation() {
	fixtures := s.fixtures()
	roller := &countingCastRoller{facedRoller: facedRoller{d20: 1, other: 4}}
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID,
		TargetIDs: []string{heroID, wolfID, "absent"}, Roller: roller,
	})
	s.Require().NoError(err)
	caster := baneCaster(1, 2)
	caster.Conditions = []json.RawMessage{baneOwnerJSON(s.T(), bardID, 5)}
	before, err := json.Marshal(caster)
	s.Require().NoError(err)

	out, err := fixtures.resolve(fixtures.saver(14), machine, baneCost(), caster)
	s.Require().Error(err)
	s.Nil(out)
	s.Zero(roller.calls)
	after, marshalErr := json.Marshal(caster)
	s.Require().NoError(marshalErr)
	s.JSONEq(string(before), string(after))
}

func baneWorld(t *testing.T, targetX float64) encounter.EncounterData {
	t.Helper()
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: noAttacksExpected{},
		Mover: encounter.RefusingMover{}, Announcer: quietAnnouncer{},
		Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		Field: encounter.FieldInput{Canvas: hexCanvas(), Regions: []encounter.RegionInput{
			rectRegion("room", 0, 0, 20, 10),
		}},
		Members: []encounter.MemberInput{
			{ID: bardID, Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: heroID, Kind: encounter.KindPlayer, Position: spatial.Position{X: targetX, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
	})
	require.NoError(t, err)
	return enc.ToData()
}

func (s *CastActionTestSuite) TestBaneRefusesStaleAndOutOfRangeTargetsBeforeDropOrRNG() {
	for _, tc := range []struct {
		name    string
		targetX float64
		dead    bool
		want    error
	}{
		{name: "stale defeated target", targetX: 2, dead: true, want: ErrBadAction},
		{name: "out of range", targetX: 9, want: ErrOutOfRange},
	} {
		s.Run(tc.name, func() {
			roller := &countingCastRoller{facedRoller: facedRoller{d20: 1, other: 4}}
			machine, err := NewAction(&ActionInput{
				Definition: *baneDefinition(), AttackerID: bardID,
				TargetIDs: []string{heroID}, Roller: roller,
			})
			s.Require().NoError(err)
			caster := baneCaster(1, 2)
			caster.Conditions = []json.RawMessage{baneOwnerJSON(s.T(), bardID, 5)}
			before, marshalErr := json.Marshal(caster)
			s.Require().NoError(marshalErr)
			target := s.fixtures().saver(14)
			if tc.dead {
				target.HitPoints = 0
				target.DeathSaveState = &saves.DeathSaveState{Dead: true}
			}
			bus := events.NewEventBus()
			removals := 0
			_, err = dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(s.ctx,
				func(context.Context, dnd5eEvents.ConditionRemovedEvent) error { removals++; return nil })
			s.Require().NoError(err)
			out, err := resolveOn(s.ctx, &Input{
				World:        baneWorld(s.T(), tc.targetX),
				Participants: []Participant{{Character: caster}, {Character: target}},
				Machine:      machine, Cost: baneCost(), Initiative: orderAsGiven{},
				Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
				TurnDriver: passDriver{}, Roller: dice.NewRoller(),
			}, newSurface(bus))
			s.ErrorIs(err, tc.want)
			s.Nil(out)
			s.Zero(roller.calls)
			s.Zero(removals)
			after, marshalErr := json.Marshal(caster)
			s.Require().NoError(marshalErr)
			s.JSONEq(string(before), string(after), "action, slot pool, and old concentration are unchanged")
		})
	}
}

func (s *CastActionTestSuite) TestViciousMockeryLandsDamageAndItsRider() {
	fixtures := s.fixtures()

	out, err := fixtures.resolve(
		fixtures.saver(14), s.cast(spells.ViciousMockery, bardID, heroID, straightRoll),
		castCost(), fixtures.bard(1),
	)
	s.Require().NoError(err)

	outcome := s.castOutcome(out)
	s.Require().Equal(refs.Spells.ByID(string(spells.ViciousMockery)).String(), outcome.Spell.String())
	s.Require().Equal(bardID, outcome.CasterID)
	s.Require().Equal(heroID, outcome.Targets[0].TargetID)

	s.Require().NotNil(outcome.Targets[0].Save, "a gated cast carries its whole contest")
	s.Require().False(outcome.Targets[0].Save.Succeeded, "WIS +1 on a 3 is 4 against DC 13")
	s.Require().Equal(spellSaveDC, outcome.Targets[0].Save.DC)
	s.Require().Equal(abilities.WIS, outcome.Targets[0].Save.Ability)
	s.Require().NotNil(outcome.Targets[0].Save.Save.Result, "and the roll the player must see")

	s.Require().Len(outcome.Targets[0].Applied, 2)
	s.Require().Equal(ImposedDamage, outcome.Targets[0].Applied[0].Kind)
	s.Require().Equal(psychicFace, outcome.Targets[0].Applied[0].Amount)
	s.Require().Equal(heroID, outcome.Targets[0].Applied[0].RecipientID)
	s.Require().Equal(damage.Psychic, outcome.Targets[0].Applied[0].Components[0].DamageType)

	// Everything a damage-applied beat needs, from the real content path.
	s.Require().Equal(psychicFace, outcome.Targets[0].Applied[0].Requested)
	s.Require().Equal(14, outcome.Targets[0].Applied[0].Before)
	s.Require().Equal(11, outcome.Targets[0].Applied[0].After)
	s.Require().NotNil(outcome.Targets[0].Applied[0].Calculation)
	s.Require().Equal(outcome.Targets[0].Applied[0].Requested, outcome.Targets[0].Applied[0].Calculation.Total)
	s.Require().NoError(dnd5eEvents.ValidateRollCalculation(outcome.Targets[0].Applied[0].Calculation))
	s.Require().Equal("Vicious Mockery", outcome.Targets[0].Applied[0].Calculation.Components[0].Source.Name,
		"the compiled definition is the provenance pair, ref and name")
	s.Require().Equal(ImposedCondition, outcome.Targets[0].Applied[1].Kind)
	s.Require().Equal(refs.Conditions.ViciousMockery().String(), outcome.Targets[0].Applied[1].Ref.String())
	s.Require().Equal(heroID, outcome.Targets[0].Applied[1].RecipientID)

	target := fixtures.sheet(out, heroID)
	s.Require().Equal(11, target.HitPoints, "14 - 3 psychic")
	params := s.castParams(target, refs.Conditions.ViciousMockery().String())
	s.Require().Equal(bardID, params[spells.ViciousMockeryCasterParameter],
		"the counterpart the content named is the CASTER, written by resolution")
	s.Require().Equal(refs.Spells.ByID(string(spells.ViciousMockery)).String(), params["source_ref"],
		"and the condition itself names WHICH spell applied it")

	payer := fixtures.sheet(out, bardID)
	s.Require().Zero(payer.ActionEconomy.ActionsRemaining, "the action was charged at the door")
}

// The control: the same cantrip, a made save, and nothing at all is delivered.
func (s *CastActionTestSuite) TestAMadeSaveAgainstViciousMockeryDeliversNothing() {
	fixtures := s.fixtures()

	out, err := fixtures.resolve(
		fixtures.saver(14), s.cast(spells.ViciousMockery, bardID, heroID, advantageRoll),
		castCost(), fixtures.bard(1),
	)
	s.Require().NoError(err)

	outcome := s.castOutcome(out)
	s.Require().NotNil(outcome.Targets[0].Save)
	s.Require().True(outcome.Targets[0].Save.Succeeded)
	s.Require().Empty(outcome.Targets[0].Applied, "no damage, no rider")
	s.Require().Equal(spellSaveDC, outcome.Targets[0].Save.DC, "and the save is still on the record")

	for _, sheet := range out.DirtyCharacters {
		if sheet.ID == heroID {
			s.Require().Equal(14, sheet.HitPoints)
			s.Require().Empty(sheet.Conditions)
		}
	}

	payer := fixtures.sheet(out, bardID)
	s.Require().Zero(payer.ActionEconomy.ActionsRemaining,
		"the action is spent whether or not the save was made")
}

// THE HEADLINE FOR THE GATELESS HALF. True Strike has no gate, so there is
// nothing to roll: the condition goes on the CASTER, keyed to the creature the
// advantage is good against, and the collector reports the one effect.
func (s *CastActionTestSuite) TestTrueStrikeDeliversToTheCasterWithNoRoll() {
	fixtures := s.fixtures()

	out, err := fixtures.resolve(
		fixtures.saver(14), s.cast(spells.TrueStrike, bardID, heroID, straightRoll),
		castCost(), fixtures.bard(1),
	)
	s.Require().NoError(err)

	outcome := s.castOutcome(out)
	s.Require().Equal(refs.Spells.ByID(string(spells.TrueStrike)).String(), outcome.Spell.String(),
		"the outcome echoes the spell that ran")
	s.Require().Nil(outcome.Targets[0].Save, "no gate means no saved beat to write")
	s.Require().Len(outcome.Targets[0].Applied, 1, "one Gather, one condition, one effect")
	s.Require().Equal(ImposedCondition, outcome.Targets[0].Applied[0].Kind)
	s.Require().Equal(bardID, outcome.Targets[0].Applied[0].RecipientID, "on the caster, not the creature named")
	s.Require().Equal(refs.Conditions.TrueStrike().String(), outcome.Targets[0].Applied[0].Ref.String())

	caster := fixtures.sheet(out, bardID)
	params := s.castParams(caster, refs.Conditions.TrueStrike().String())
	s.Require().Equal(heroID, params[spells.TrueStrikeTargetParameter],
		"the counterpart the content named is the TARGET, written by resolution")
	s.Require().Equal(refs.Spells.ByID(string(spells.TrueStrike)).String(), params["source_ref"],
		"and the condition itself names WHICH spell applied it")
	s.Require().Zero(caster.ActionEconomy.ActionsRemaining, "charged at the door like any cast")

	for _, sheet := range out.DirtyCharacters {
		s.Require().NotEqual(heroID, sheet.ID, "the creature named is untouched: nobody resists True Strike")
	}
}

// A cast with an empty action budget is refused at the door: nothing is
// published, nothing is rolled, and no sheet comes back.
func (s *CastActionTestSuite) TestACastWithNoActionLeftIsRefusedAtTheDoor() {
	fixtures := s.fixtures()

	out, err := fixtures.resolve(
		fixtures.saver(14), s.cast(spells.TrueStrike, bardID, heroID, straightRoll),
		castCost(), fixtures.bard(0),
	)

	s.Require().ErrorIs(err, ErrCannotPay)
	s.Require().Nil(out, "a refused cast hands back nothing to store")
	s.Require().Contains(err.Error(), bardID)
}

// The same refusal for the gated half, and it happens before the save is
// rolled — the door is the only place a cantrip's price moves.
func (s *CastActionTestSuite) TestAGatedCastWithNoActionLeftIsRefusedBeforeTheSave() {
	fixtures := s.fixtures()

	_, err := fixtures.resolve(
		fixtures.saver(14), s.cast(spells.ViciousMockery, bardID, heroID, straightRoll),
		castCost(), fixtures.bard(0),
	)

	s.Require().ErrorIs(err, ErrCannotPay)
}

// NewAction reads the arm, and nothing below it reads a spell id. Two profiles
// from the same content table pick two different machines.
func (s *CastActionTestSuite) TestTheProfileArmPicksTheMachine() {
	gated, err := NewAction(&ActionInput{
		Definition: *spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.ViciousMockery, SpellSaveDC: spellSaveDC}),
		AttackerID: bardID, TargetIDs: []string{heroID}, Roller: facedRoller{d20: straightRoll, other: psychicFace},
	})
	s.Require().NoError(err)
	s.Require().IsType(&castMachine{}, gated)
	s.Require().IsType(&contestMachine{}, gated.(*castMachine).targets[0].inner,
		"a gate is a save, and a save is a contest")

	gateless, err := NewAction(&ActionInput{
		Definition: *spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.TrueStrike, SpellSaveDC: spellSaveDC}),
		AttackerID: bardID, TargetIDs: []string{heroID},
	})
	s.Require().NoError(err)
	s.Require().IsType(&castMachine{}, gateless)
	s.Require().IsType(&activationMachine{}, gateless.(*castMachine).targets[0].inner, "no gate is a delivery")
}

// Every refusal the cast branch makes, and each says what content got wrong.
func (s *CastActionTestSuite) TestTheCastBranchRefusesWhatItCannotDeliver() {
	trueStrike := func() combatActions.Definition {
		return *spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.TrueStrike, SpellSaveDC: spellSaveDC})
	}

	s.Run("no caster", func() {
		_, err := NewAction(&ActionInput{Definition: trueStrike(), TargetIDs: []string{heroID}})
		s.Require().ErrorIs(err, ErrBadAction)
		s.Require().Contains(err.Error(), "cast by nobody")
	})

	s.Run("a one-creature cast with no creature", func() {
		_, err := NewAction(&ActionInput{Definition: trueStrike(), AttackerID: bardID})
		s.Require().ErrorIs(err, ErrBadAction)
		s.Require().Contains(err.Error(), "requires 1..1 targets")
	})

	s.Run("a self-targeted cast handed a target", func() {
		definition := trueStrike()
		definition.Cast.Target = combatActions.CastTargetSelf
		definition.Cast.MinTargets = 0
		definition.Cast.MaxTargets = 0
		definition.Cast.Effects[0].CounterpartKey = ""

		_, err := NewAction(&ActionInput{Definition: definition, AttackerID: bardID, TargetIDs: []string{heroID}})
		s.Require().ErrorIs(err, ErrBadAction)
		s.Require().Contains(err.Error(), "requires 0..0 targets")
	})

	s.Run("a gated cast delivering to its caster", func() {
		definition := *spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.ViciousMockery, SpellSaveDC: spellSaveDC})
		definition.Cast.Effects[0].Recipient = combatActions.CastRecipientCaster

		_, err := NewAction(&ActionInput{Definition: definition, AttackerID: bardID, TargetIDs: []string{heroID}})
		s.Require().ErrorIs(err, ErrBadAction)
		s.Require().Contains(err.Error(), "delivers to its caster")
	})

	s.Run("a gated cast with two conditions", func() {
		definition := *spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.ViciousMockery, SpellSaveDC: spellSaveDC})
		definition.Cast.Effects = append(definition.Cast.Effects, definition.Cast.Effects[0])

		_, err := NewAction(&ActionInput{Definition: definition, AttackerID: bardID, TargetIDs: []string{heroID}})
		s.Require().ErrorIs(err, ErrBadAction)
		s.Require().Contains(err.Error(), "a contest delivers one")
	})

	s.Run("undefended damage", func() {
		definition := trueStrike()
		definition.Cast.Damage = []damage.Damage{{Dice: "1d4", Type: damage.Psychic}}

		_, err := NewAction(&ActionInput{Definition: definition, AttackerID: bardID, TargetIDs: []string{heroID}})
		s.Require().ErrorIs(err, ErrBadAction)
		s.Require().Contains(err.Error(), "deals damage with no save")
	})

	s.Run("a recipient who is not in the cast", func() {
		fixtures := s.fixtures()
		_, err := fixtures.resolve(
			fixtures.saver(14), s.cast(spells.TrueStrike, "nobody", heroID, straightRoll), nil, nil,
		)
		s.Require().ErrorIs(err, ErrBadActivation)
	})
}

// The counterpart binding is content's key, not a convention: the two cantrips
// spell it differently and the writer is told which by the profile.
func (s *CastActionTestSuite) TestTheCounterpartKeyIsWrittenWhereContentSaid() {
	forEach := []struct {
		name          string
		effect        combatActions.CastEffect
		counterpartID string
		wantKey       string
	}{
		{
			"true strike names the target", combatActions.CastEffect{
				Recipient: combatActions.CastRecipientCaster, Ref: *refs.Conditions.TrueStrike(),
				CounterpartKey: spells.TrueStrikeTargetParameter,
			}, heroID, spells.TrueStrikeTargetParameter,
		},
		{
			"vicious mockery names the caster", combatActions.CastEffect{
				Recipient: combatActions.CastRecipientTarget, Ref: *refs.Conditions.ViciousMockery(),
				CounterpartKey: spells.ViciousMockeryCasterParameter,
			}, bardID, spells.ViciousMockeryCasterParameter,
		},
	}

	for _, tc := range forEach {
		s.Run(tc.name, func() {
			bound, err := bindCounterpart(tc.effect, tc.counterpartID)
			s.Require().NoError(err)

			var fields map[string]string
			s.Require().NoError(json.Unmarshal(bound, &fields))
			s.Require().Equal(map[string]string{tc.wantKey: tc.counterpartID}, fields)
		})
	}

	s.Run("an effect that binds nothing keeps its parameters", func() {
		bound, err := bindCounterpart(combatActions.CastEffect{
			Ref: *refs.Conditions.TrueStrike(), Parameters: json.RawMessage(`{"kept":true}`),
		}, "")
		s.Require().NoError(err)
		s.Require().JSONEq(`{"kept":true}`, string(bound))
	})

	s.Run("a binding with no counterpart is refused", func() {
		_, err := bindCounterpart(combatActions.CastEffect{
			Ref: *refs.Conditions.TrueStrike(), CounterpartKey: spells.TrueStrikeTargetParameter,
		}, "")
		s.Require().Error(err)
		s.Require().Contains(err.Error(), "the cast's other party")
	})
}

// A self-targeted cast names THE CASTER, because the outcome's target list is
// the list of RECIPIENTS and a recipient has to be somebody.
//
// This reverses the reasoning that stood here before, and the reasoning was
// sound for the shape it was written against: when a cast reported one
// TargetID beside a CasterID, repeating the caster into both really was a
// second way to say the same thing. The multi-target work replaced that pair
// with Targets []CastTargetOutcome, and the list stopped meaning "who did the
// player name" and started meaning "who received what" -- each entry carrying
// the effects delivered to it. An entry naming nobody has nowhere to hang them.
//
// Downstream is where it bites. encounter.RecordCastInput has exactly Actor,
// Spell and Targets []CastTargetResult; there is no caster-side results lane,
// so a self cast's condition can only be recorded against a target entry. An
// empty MemberID is refused there with ErrNoMember -- and refused AFTER the
// sheet writes are already durable, leaving the condition applied and the beat
// missing.
func (s *CastActionTestSuite) TestASelfTargetedCastNamesTheCaster() {
	definition := *spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.TrueStrike, SpellSaveDC: spellSaveDC})
	definition.Cast.Target = combatActions.CastTargetSelf
	definition.Cast.MinTargets = 0
	definition.Cast.MaxTargets = 0
	definition.Cast.Effects[0].CounterpartKey = ""
	definition.Cast.Effects[0].Parameters = json.RawMessage(`{"target_id":"` + heroID + `"}`)
	definition.Cost = oneAction()

	machine, err := NewAction(&ActionInput{Definition: definition, AttackerID: bardID})
	s.Require().NoError(err)

	fixtures := s.fixtures()
	out, err := fixtures.resolve(fixtures.saver(14), machine, castCost(), fixtures.bard(1))
	s.Require().NoError(err)

	outcome := s.castOutcome(out)
	s.Require().Len(outcome.Targets, 1, "one recipient, because one creature received the spell")
	s.Require().Equal(bardID, outcome.Targets[0].TargetID, "and the recipient is the caster, named as a real member")
	s.Require().Nil(outcome.Targets[0].Save)
	s.Require().Len(outcome.Targets[0].Applied, 1)
	s.Require().Equal(bardID, outcome.Targets[0].Applied[0].RecipientID)
}

// THE ORDERING THAT MATTERS. The cast's identity WRAPS the machine, and the
// wrapper preflights it at its OWN Start rather than letting the driver do it
// when the Request is reached — so a cast that cannot run is refused while
// Resolve is still pure preflight, before the door charges anybody.
//
// Detectable rather than asserted: the payer here has no action left either,
// so the two refusals are both available and only their ORDER decides which one
// comes back. A wrapper that preflighted late would answer ErrCannotPay.
func (s *CastActionTestSuite) TestACastIsPreflightedBeforeTheDoorCharges() {
	fixtures := s.fixtures()

	out, err := fixtures.resolve(
		fixtures.saver(14), s.cast(spells.TrueStrike, "nobody", heroID, straightRoll),
		castCost(), fixtures.bard(0),
	)

	s.Require().ErrorIs(err, ErrBadActivation, "the recipient is missing, and that is found first")
	s.Require().NotErrorIs(err, ErrCannotPay, "the door was never reached")
	s.Require().Nil(out, "and nothing comes back to be stored")
}

// sourcesSeen records the ConditionSource of every condition published on an
// interaction's bus, keyed by the condition's type.
//
// The event is what a keeper and a record both read, so the assertion is made
// against the published fact rather than against the argument that produced it.
func (s *CastActionTestSuite) sourcesSeen(
	bus events.EventBus,
) map[dnd5eEvents.ConditionType]dnd5eEvents.ConditionSource {
	seen := map[dnd5eEvents.ConditionType]dnd5eEvents.ConditionSource{}
	_, err := dnd5eEvents.ConditionAppliedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
			seen[event.Type] = event.Source
			return nil
		})
	s.Require().NoError(err)

	return seen
}

// resolveOnBus runs a cast on a bus the scene can listen to.
func (s *CastActionTestSuite) resolveOnBus(
	fixtures *ContestDamageTestSuite, machine Machine, bus events.EventBus,
) *Output {
	out, err := resolveOn(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		Equipment: noHandsAreObserved{},
		World:     fixtures.world(),
		Participants: []Participant{
			{Character: fixtures.saver(14)}, {Monster: fixtures.wolfData()}, {Character: fixtures.bard(1)},
		},
		Machine: machine,
		Cost:    castCost(),
	}, newSurface(bus))
	s.Require().NoError(err)

	return out
}

// A cast's condition says a SPELL applied it, on both halves of the door. The
// kind is all the event carries; which spell travels with the condition itself,
// as the source ref it was built with.
func (s *CastActionTestSuite) TestACastsConditionsCarryTheSpellAsTheirSource() {
	s.Run("gateless", func() {
		bus := events.NewEventBus()
		seen := s.sourcesSeen(bus)

		out := s.resolveOnBus(s.fixtures(), s.cast(spells.TrueStrike, bardID, heroID, straightRoll), bus)

		s.Require().Len(s.castOutcome(out).Targets[0].Applied, 1)
		s.Require().Equal(dnd5eEvents.ConditionSourceSpell, seen[dnd5eEvents.ConditionTrueStrike])
	})

	s.Run("gated", func() {
		bus := events.NewEventBus()
		seen := s.sourcesSeen(bus)

		out := s.resolveOnBus(s.fixtures(), s.cast(spells.ViciousMockery, bardID, heroID, straightRoll), bus)

		s.Require().Len(s.castOutcome(out).Targets[0].Applied, 2)
		s.Require().Equal(dnd5eEvents.ConditionSourceSpell, seen[dnd5eEvents.ConditionViciousMockery],
			"the contest reads the cause it was given, which says a spell raised the save")
	})
}

// The control. A contest raised by something other than a cast keeps the answer
// this package has always given, so the wolf's knockdown is unchanged.
func (s *CastActionTestSuite) TestAContestWithNoSpellCauseStillSaysDamage() {
	bus := events.NewEventBus()
	seen := s.sourcesSeen(bus)
	fixtures := s.fixtures()

	out, err := resolveOn(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(),
		Equipment:    noHandsAreObserved{},
		World:        fixtures.world(),
		Participants: []Participant{{Character: fixtures.saver(14)}, {Monster: fixtures.wolfData()}},
		Machine: NewContest(&ContestInput{
			Gate:        mockeryGate(),
			SaverID:     heroID,
			Application: prone(),
			Roller:      facedRoller{d20: straightRoll, other: psychicFace},
		}),
	}, newSurface(bus))
	s.Require().NoError(err)
	s.Require().Len(out.Outcome.(ContestOutcome).Imposed, 1)

	s.Require().Equal(dnd5eEvents.ConditionSourceDamage, seen[dnd5eEvents.ConditionProne])
}

// commandDefinition is Command as content compiles it: a WIS gate, a menu of
// three words, and one effect that reads both the caster and the word.
func commandDefinition() *combatActions.Definition {
	return spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Command, SpellSaveDC: spellSaveDC})
}

func commandCost() *Cost {
	return &Cost{
		PayerID: bardID,
		Profile: commandDefinition().Cost,
		Turn:    &Turn{Number: mockeryTurn, Speed: mockerySpeed},
	}
}

// THE HEADLINE FOR THE OPTION. Command is the first spell whose parameters are
// not fully written by content: the caster comes from the cast and the WORD
// comes from the player, and both have to be on the sheet for the compulsion to
// mean anything.
//
// Asserted on a GATED cast, which is the half that nearly lost the binding: the
// contest builds its one application through a different branch from the
// gateless delivery's loop, so a binding wired on one path only would resolve
// Command with an empty word and still look fine.
func (s *CastActionTestSuite) TestTheChosenWordIsWrittenOnAGatedCast() {
	for _, word := range []string{spells.CommandWordApproach, spells.CommandWordFlee, spells.CommandWordGrovel} {
		s.Run(word, func() {
			fixtures := s.fixtures()
			machine, err := NewAction(&ActionInput{
				Definition: *commandDefinition(), AttackerID: bardID,
				TargetIDs: []string{heroID}, Option: word,
				Roller: facedRoller{d20: 1, other: psychicFace},
			})
			s.Require().NoError(err)

			out, err := fixtures.resolve(fixtures.saver(14), machine, commandCost(), baneCaster(1, 2))
			s.Require().NoError(err)
			outcome := s.castOutcome(out)
			s.Require().Len(outcome.Targets, 1)
			s.Require().False(outcome.Targets[0].Save.Succeeded, "a 1 misses a DC 13 Wisdom save")

			stored := s.castParams(fixtures.sheet(out, heroID), refs.Conditions.Commanded().String())
			s.Equal(word, stored[spells.CommandWordParameter], "the word the player chose")
			s.Equal(bardID, stored[spells.CommandCasterParameter], "the caster the words are measured from")
		})
	}
}

// A made save leaves nothing behind, so there is no word on any sheet: the
// option is bound at construction and the gate still decides whether it lands.
func (s *CastActionTestSuite) TestAMadeSaveLeavesNoWordBehind() {
	fixtures := s.fixtures()
	machine, err := NewAction(&ActionInput{
		Definition: *commandDefinition(), AttackerID: bardID,
		TargetIDs: []string{heroID}, Option: spells.CommandWordGrovel,
		Roller: facedRoller{d20: 20, other: psychicFace},
	})
	s.Require().NoError(err)

	out, err := fixtures.resolve(fixtures.saver(14), machine, commandCost(), baneCaster(1, 2))
	s.Require().NoError(err)
	s.Require().True(s.castOutcome(out).Targets[0].Save.Succeeded)
	s.Empty(s.castOutcome(out).Targets[0].Applied, "Command has no damage, so a made save delivers nothing")
	for _, data := range out.DirtyCharacters {
		s.NotContains(fixtures.conditionRefs(data), refs.Conditions.Commanded().String(),
			"%s came back to be saved carrying a compulsion nobody imposed", data.ID)
	}
}

// The menu is re-checked HERE even though the host checked it against the offer
// it drew, and each of these is a different way for the choice to go missing.
func (s *CastActionTestSuite) TestACastIsRefusedForAChoiceItsProfileDoesNotOffer() {
	for _, tc := range []struct {
		name       string
		definition *combatActions.Definition
		option     string
		contains   string
	}{
		{
			name: "a menu and no choice", definition: commandDefinition(), option: "",
			contains: "chose none",
		},
		{
			name: "a word the spell does not know", definition: commandDefinition(), option: "halt",
			contains: `does not offer the option "halt"`,
		},
		{
			name: "a choice for a spell with no menu", definition: baneDefinition(),
			option: spells.CommandWordFlee, contains: `does not offer the option "flee"`,
		},
	} {
		s.Run(tc.name, func() {
			roller := &countingCastRoller{facedRoller: facedRoller{d20: 1, other: psychicFace}}
			machine, err := NewAction(&ActionInput{
				Definition: *tc.definition, AttackerID: bardID,
				TargetIDs: []string{heroID}, Option: tc.option, Roller: roller,
			})
			s.Require().ErrorIs(err, ErrBadAction)
			s.Require().Contains(err.Error(), tc.contains)
			s.Nil(machine)
			s.Zero(roller.calls, "a choice the profile cannot answer for is refused before any die")
		})
	}
}

// The option key, like the counterpart key, is content's own: an effect that
// names none keeps exactly the parameters it was authored with.
func (s *CastActionTestSuite) TestTheOptionKeyIsWrittenWhereContentSaidAndNowhereElse() {
	command := commandDefinition().Cast.Effects[0]

	s.Run("both bindings land on one configuration", func() {
		bound, err := bindCast(command, bardID, spells.CommandWordFlee)
		s.Require().NoError(err)

		var fields map[string]any
		s.Require().NoError(json.Unmarshal(bound, &fields))
		s.Equal(bardID, fields[spells.CommandCasterParameter])
		s.Equal(spells.CommandWordFlee, fields[spells.CommandWordParameter])
		s.Equal(float64(spells.CommandTurnEnds), fields["turn_ends"],
			"content's own parameters survive both bindings")
	})

	s.Run("an effect that reads no option is untouched", func() {
		mockery := combatActions.CastEffect{
			Recipient: combatActions.CastRecipientTarget, Ref: *refs.Conditions.ViciousMockery(),
			CounterpartKey: spells.ViciousMockeryCasterParameter,
		}
		bound, err := bindCast(mockery, bardID, spells.CommandWordFlee)
		s.Require().NoError(err)
		s.JSONEq(fmt.Sprintf(`{%q:%q}`, spells.ViciousMockeryCasterParameter, bardID), string(bound))
	})

	s.Run("an effect that reads the option and gets none is refused", func() {
		_, err := bindOption(command, json.RawMessage(`{}`), "")
		s.Require().Error(err)
		s.Require().Contains(err.Error(), "the cast's chosen option")
	})
}

// conditionTraffic is an ordered log of what landed and what came off, so the
// ORDER of a replacement can be asserted rather than assumed.
func (s *CastActionTestSuite) conditionTraffic(bus events.EventBus, ref *core.Ref) *[]string {
	log := &[]string{}
	_, err := dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionRemovedEvent) error {
			if event.ConditionRef == ref.String() {
				*log = append(*log, "removed:"+event.MemberID+":"+event.SourceID+":"+event.Reason)
			}
			return nil
		})
	s.Require().NoError(err)
	_, err = dnd5eEvents.ConditionAppliedTopic.On(bus).Subscribe(s.ctx,
		func(_ context.Context, event dnd5eEvents.ConditionAppliedEvent) error {
			if event.Type == dnd5eEvents.ConditionType(ref.ID) {
				*log = append(*log, "applied:"+event.Target.GetID())
			}
			return nil
		})
	s.Require().NoError(err)

	return log
}

// countingRef is how many instances of one ref a sheet came back carrying. The
// rule under test is "one per member", so the number IS the claim.
func (s *CastActionTestSuite) countingRef(stored []json.RawMessage, ref *core.Ref) int {
	found := 0
	for _, raw := range stored {
		var peek struct {
			Ref core.Ref `json:"ref"`
		}
		s.Require().NoError(json.Unmarshal(raw, &peek))
		if peek.Ref.Equals(ref) {
			found++
		}
	}

	return found
}

// THE EFFECTS OF THE SAME SPELL DO NOT STACK, and Bane is the proof because
// Bane is where the stacking was: two casters each put a −1d4 on one creature
// and the first one to end took both off.
//
// Command's second word replacing the first is the use case that brought the
// rule, and the rule is general because the stacking was never Command's bug.
func (s *CastActionTestSuite) TestASecondInstanceOfOneRefReplacesTheFirst() {
	fixtures := s.fixtures()
	target := fixtures.saver(14, baneConditionJSON(s.T(), heroID, "other-caster"))
	bus := events.NewEventBus()
	traffic := s.conditionTraffic(bus, refs.Conditions.Baned())
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID,
		TargetIDs: []string{heroID}, Roller: facedRoller{d20: 1, other: 4},
	})
	s.Require().NoError(err)

	out, err := resolveOn(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Equipment: noHandsAreObserved{},
		World:        fixtures.world(),
		Participants: []Participant{{Character: target}, {Character: baneCaster(1, 2)}},
		Machine:      machine, Cost: baneCost(),
	}, newSurface(bus))
	s.Require().NoError(err)

	s.Equal([]string{"removed:" + heroID + ":other-caster:replaced", "applied:" + heroID}, *traffic,
		"the old instance comes off BEFORE the new one lands")
	s.Equal(1, s.countingRef(fixtures.sheet(out, heroID).Conditions, refs.Conditions.Baned()),
		"one instance of a ref per member")

	imposed := s.castOutcome(out).Targets[0].Applied
	s.Require().Len(imposed, 2, "the trace says the replacement happened as well as the application")
	s.Equal(ImposedConditionRemoved, imposed[0].Kind)
	s.Contains(imposed[0].Description, "replaced by")
	s.Equal(ImposedCondition, imposed[1].Kind)
}

// A monster's sheet is the same sheet for this rule. The check reads whatever
// the recipient is holding, and nothing about it knows which kind of sheet it
// came off.
func (s *CastActionTestSuite) TestAMonsterRecipientIsReplacedTheSameWay() {
	fixtures := s.fixtures()
	wolf := fixtures.wolfData()
	wolf.Conditions = []json.RawMessage{baneConditionJSON(s.T(), wolfID, "other-caster")}
	bus := events.NewEventBus()
	traffic := s.conditionTraffic(bus, refs.Conditions.Baned())
	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID,
		TargetIDs: []string{wolfID}, Roller: facedRoller{d20: 1, other: 4},
	})
	s.Require().NoError(err)

	out, err := resolveOn(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Equipment: noHandsAreObserved{},
		World:        fixtures.world(),
		Participants: []Participant{{Monster: wolf}, {Character: baneCaster(1, 2)}},
		Machine:      machine, Cost: baneCost(),
	}, newSurface(bus))
	s.Require().NoError(err)

	s.Equal([]string{"removed:" + wolfID + ":other-caster:replaced", "applied:" + wolfID}, *traffic)
	s.Equal(1, s.countingRef(s.monsterSheet(out, wolfID).Conditions, refs.Conditions.Baned()),
		"one instance of a ref per member, on a monster's sheet too")
}

// monsterSheet is the monster half of the damage suite's sheet lookup, and it
// fails rather than skipping: a recipient that never came back to be saved is a
// missing assertion, not an absent one.
func (s *CastActionTestSuite) monsterSheet(out *Output, id string) *monster.Data {
	for _, data := range out.DirtyMonsters {
		if data.ID == id {
			return data
		}
	}
	s.Require().Failf("no dirty sheet", "%q did not come back to be saved", id)

	return nil
}

// Replacement is by REF and only by ref. A creature holding somebody else's
// spell keeps it when a different one lands, which is what makes this a rule
// about one spell rather than about conditions in general.
func (s *CastActionTestSuite) TestADifferentRefIsLeftWhereItIs() {
	fixtures := s.fixtures()
	mockery := conditions.NewViciousMockeryCondition(heroID, bardID, refs.Spells.ViciousMockery().String())
	stored, err := mockery.ToJSON()
	s.Require().NoError(err)

	bus := events.NewEventBus()
	removals := 0
	_, err = dnd5eEvents.ConditionRemovedTopic.On(bus).Subscribe(s.ctx,
		func(context.Context, dnd5eEvents.ConditionRemovedEvent) error { removals++; return nil })
	s.Require().NoError(err)

	machine, err := NewAction(&ActionInput{
		Definition: *baneDefinition(), AttackerID: bardID,
		TargetIDs: []string{heroID}, Roller: facedRoller{d20: 1, other: 4},
	})
	s.Require().NoError(err)

	out, err := resolveOn(s.ctx, &Input{
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{},
		Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Equipment: noHandsAreObserved{},
		World:        fixtures.world(),
		Participants: []Participant{{Character: fixtures.saver(14, stored)}, {Character: baneCaster(1, 2)}},
		Machine:      machine, Cost: baneCost(),
	}, newSurface(bus))
	s.Require().NoError(err)

	s.Zero(removals, "Bane landing takes nothing else off")
	sheet := fixtures.sheet(out, heroID)
	s.Equal(1, s.countingRef(sheet.Conditions, refs.Conditions.ViciousMockery()))
	s.Equal(1, s.countingRef(sheet.Conditions, refs.Conditions.Baned()))
}
