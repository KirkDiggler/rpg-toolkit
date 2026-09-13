// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

func (s *CastActionTestSuite) blessAttempt(world encounter.EncounterData, caster, target *character.Data, targets []string, policy StaleTargetPolicy, roll *countingCastRoller) (*Output, error) {
	d := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Bless})
	machine, err := NewAction(&ActionInput{Definition: *d, AttackerID: bardID, TargetIDs: targets, StaleTargetPolicy: policy, Roller: roll})
	if err != nil {
		return nil, err
	}
	return Resolve(s.ctx, &Input{World: world, Participants: []Participant{{Character: caster}, {Character: target}, {Monster: s.fixtures().wolfData()}}, Machine: machine,
		Cost:       &Cost{PayerID: bardID, Profile: d.Cost, SpellTurn: "scene/round-1/bard"},
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Roller: dice.NewRoller()})
}

func (s *CastActionTestSuite) blessRun(world encounter.EncounterData, participants []Participant, machine Machine, cost *Cost) *Output {
	out, err := Resolve(s.ctx, &Input{World: world, Participants: participants, Machine: machine, Cost: cost,
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{}, Roller: dice.NewRoller()})
	s.Require().NoError(err)
	return out
}

func (s *CastActionTestSuite) TestBlessTwoLiveCastersReloadRollsAndQualifiedRecast() {
	f := s.fixtures()
	d := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Bless})
	first, err := s.blessAttempt(f.world(), baneCaster(1, 2), f.saver(14), []string{heroID}, StaleTargetRefuse, &countingCastRoller{})
	s.Require().NoError(err)
	// Round-trip the actual delivered sheet before a second caster blesses it.
	raw, err := json.Marshal(f.sheet(first, heroID))
	s.Require().NoError(err)
	var target character.Data
	s.Require().NoError(json.Unmarshal(raw, &target))
	caster := f.sheet(first, bardID)
	machine, err := NewAction(&ActionInput{Definition: *d, AttackerID: wolfID, TargetIDs: []string{heroID}, StaleTargetPolicy: StaleTargetRefuse})
	s.Require().NoError(err)
	second := s.blessRun(f.world(), []Participant{{Character: caster}, {Character: &target}, {Monster: f.wolfData()}}, machine, nil)
	target = *f.sheet(second, heroID)
	s.Equal(2, s.countingRef(target.Conditions, refs.Conditions.Blessed()))
	s.Require().Len(second.DirtyMonsters, 1)
	wolf := second.DirtyMonsters[0]
	s.Empty(second.ConcentrationBreaks)
	// Both durations persist, while only the oldest source supplies a die.
	participants := []Participant{{Character: caster}, {Character: &target}, {Monster: wolf}}
	roll := facedRoller{d20: 8, other: 4}
	saveInput := &SaveInput{SaverID: heroID, Ability: abilities.WIS, DC: 13, Roller: roll,
		D20Source: dnd5eEvents.RollSource{Ref: refs.Spells.ViciousMockery(), Name: "Vicious Mockery"}}
	save := s.blessRun(f.world(), participants, NewSave(saveInput), nil).Outcome.(SaveOutcome)
	s.Equal(13, save.Result.Calculation.Total, "8 + Wisdom 1 + one Bless 4")
	s.Require().Len(save.Result.Calculation.Components, 3)
	s.Equal(bardID, save.Result.Calculation.Components[2].Source.SourceID)
	strike := s.blessRun(f.world(), participants, NewStrike(&StrikeInput{AttackerID: heroID, TargetID: wolfID, Definition: claw("1d4"), Roller: roll}), nil).Outcome.(StrikeOutcome)
	s.Equal(8+clawBonus+4, strike.Total)
	s.Require().Len(strike.Calculation.Components, 3)
	s.Equal(bardID, strike.Calculation.Components[2].Source.SourceID)
	withBane := target
	withBane.Conditions = append(append([]json.RawMessage(nil), target.Conditions...), baneConditionJSON(s.T(), heroID, wolfID))
	opposed := s.blessRun(f.world(), []Participant{{Character: caster}, {Character: &withBane}, {Monster: wolf}}, NewSave(saveInput), nil).Outcome.(SaveOutcome)
	s.Equal(9, opposed.Result.Calculation.Total, "one Bless +4 and Bane -4 both apply")
	s.Len(opposed.Result.Calculation.Components, 4)
	// Recasting to self strips only the bard's old child on the hero.
	caster.ActionEconomy.ActionsRemaining = 1
	machine, err = NewAction(&ActionInput{Definition: *d, AttackerID: bardID, TargetIDs: []string{bardID}, StaleTargetPolicy: StaleTargetRefuse})
	s.Require().NoError(err)
	recast := s.blessRun(f.world(), participants, machine, &Cost{PayerID: bardID, Profile: d.Cost, SpellTurn: "scene/round-2/bard"})
	s.Require().Len(recast.ConcentrationBreaks, 1)
	s.Require().Len(recast.ConcentrationBreaks[0].Removed, 1)
	s.Equal(bardID, recast.ConcentrationBreaks[0].Removed[0].Address.SourceID)
	target = *f.sheet(recast, heroID)
	s.Equal(1, s.countingRef(target.Conditions, refs.Conditions.Blessed()))
	s.Empty(recast.DirtyMonsters, "the second caster's owner is untouched")
	save = s.blessRun(f.world(), []Participant{{Character: f.sheet(recast, bardID)}, {Character: &target}, {Monster: wolf}},
		NewSave(saveInput), nil).Outcome.(SaveOutcome)
	s.Equal(13, save.Result.Calculation.Total)
	s.Equal(wolfID, save.Result.Calculation.Components[2].Source.SourceID)
}

func (s *CastActionTestSuite) TestBlessAllMissPaysAndDropsPreviousConcentration() {
	f := s.fixtures()
	first, err := s.blessAttempt(f.world(), baneCaster(1, 2), f.saver(14), []string{heroID}, StaleTargetRefuse, &countingCastRoller{})
	s.Require().NoError(err)
	caster := f.sheet(first, bardID)
	caster.ActionEconomy.ActionsRemaining = 1
	world := f.world()
	s.rememberedHero(&world, encounter.LocationKnown, spatial.Position{X: 1, Y: 2})
	before, err := json.Marshal(caster)
	s.Require().NoError(err)
	refused, err := s.blessAttempt(world, caster, f.sheet(first, heroID), []string{heroID}, StaleTargetRefuse, &countingCastRoller{})
	s.ErrorIs(err, ErrOutOfRange)
	s.Nil(refused)
	after, err := json.Marshal(caster)
	s.Require().NoError(err)
	s.Equal(before, after, "refusal preserves the existing hold and resources")
	caster.ActionEconomy.ActionsRemaining = 0
	refused, err = s.blessAttempt(world, caster, f.sheet(first, heroID), []string{heroID}, StaleTargetAttempt, &countingCastRoller{})
	s.ErrorIs(err, ErrCannotPay)
	s.Nil(refused)
	caster.ActionEconomy.ActionsRemaining = 1
	out, err := s.blessAttempt(world, caster, f.sheet(first, heroID), []string{heroID}, StaleTargetAttempt, &countingCastRoller{})
	s.Require().NoError(err)
	s.True(s.castOutcome(out).Targets[0].Missed)
	s.Empty(s.castOutcome(out).Targets[0].Applied)
	s.Require().Len(out.ConcentrationBreaks, 1)
	s.Zero(s.countingRef(f.sheet(out, heroID).Conditions, refs.Conditions.Blessed()))
	paid := f.sheet(out, bardID)
	s.Zero(paid.Resources[resources.SpellSlotLevel1].Current)
	for _, raw := range paid.Conditions {
		var hold conditions.ConcentratingConditionData
		s.Require().NoError(json.Unmarshal(raw, &hold))
		if hold.SpellRef == refs.Spells.Bless().String() {
			s.Empty(hold.Children)
			s.Equal(10, hold.TurnEndsLeft)
			return
		}
	}
	s.Fail("missing zero-child concentration owner")
}

func (s *CastActionTestSuite) TestBlessDurationExpiresAllDeliveredChildrenAfterReload() {
	f := s.fixtures()
	out, err := s.blessAttempt(f.world(), baneCaster(1, 2), f.saver(14), []string{heroID, bardID}, StaleTargetRefuse, &countingCastRoller{})
	s.Require().NoError(err)
	caster, target := f.sheet(out, bardID), f.sheet(out, heroID)
	for turn := 0; turn <= 10; turn++ {
		machine, err := NewBoundary(&BoundaryInput{Crossed: []encounter.Boundary{{Kind: encounter.TurnEnded, Subject: bardID, Round: turn + 1}}})
		s.Require().NoError(err)
		out = s.blessRun(f.world(), []Participant{{Character: caster}, {Character: target}, {Monster: f.wolfData()}}, machine, nil)
		caster = f.sheet(out, bardID)
		if turn < 10 {
			s.Empty(out.ConcentrationBreaks)
			continue
		}
		s.Require().Len(out.ConcentrationBreaks, 1)
		s.Equal(conditions.ConcentrationEndedDuration, out.ConcentrationBreaks[0].Reason)
		s.Len(out.ConcentrationBreaks[0].Removed, 2)
		s.Zero(s.countingRef(caster.Conditions, refs.Conditions.Blessed()))
		s.Zero(s.countingRef(f.sheet(out, heroID).Conditions, refs.Conditions.Blessed()))
	}
}

func (s *CastActionTestSuite) TestBlessLifeStatesAndSameAddressReplacement() {
	for _, state := range []string{"stabilized", "dead", "same address"} {
		s.Run(state, func() {
			f := s.fixtures()
			target := f.saver(0)
			switch state {
			case "stabilized":
				target.DeathSaveState = &saves.DeathSaveState{Stabilized: true}
			case "dead":
				target.DeathSaveState = &saves.DeathSaveState{Dead: true}
			case "same address":
				blessed, err := conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{MemberID: heroID, SourceID: bardID, SourceRef: refs.Spells.Bless()})
				s.Require().NoError(err)
				raw, err := blessed.ToJSON()
				s.Require().NoError(err)
				target.Conditions = []json.RawMessage{raw}
			}
			out, err := s.blessAttempt(f.world(), baneCaster(1, 2), target, []string{heroID}, StaleTargetRefuse, &countingCastRoller{})
			if state == "dead" {
				s.ErrorIs(err, ErrBadAction)
				s.Nil(out)
				return
			}
			s.Require().NoError(err)
			if state == "same address" {
				effects := s.castOutcome(out).Targets[0].Applied
				s.Require().Len(effects, 2)
				s.Equal(removalEffect(dnd5eEvents.ConditionAddress{MemberID: heroID, ConditionRef: refs.Conditions.Blessed().String(), SourceID: bardID}, ConditionReplacedReason), effects[0])
				s.Equal(ImposedCondition, effects[1].Kind)
			}
			s.Equal(1, s.countingRef(f.sheet(out, heroID).Conditions, refs.Conditions.Blessed()))
		})
	}
}

func (s *CastActionTestSuite) rememberedHero(world *encounter.EncounterData, state encounter.LocationState, pos spatial.Position) {
	holding := world.Perception.Intel.Holdings[bardID][heroID]
	holding.CurrentVia = nil
	var err error
	holding.Payload, err = encounter.EncodeSightTestimony(encounter.SightTestimony{State: state, Position: pos})
	s.Require().NoError(err)
	world.Perception.Intel.Holdings[bardID][heroID] = holding
}

func (s *CastActionTestSuite) TestBlessKnownSelfDyingAndMonsterPayOnce() {
	f := s.fixtures()
	world := f.world()
	s.rememberedHero(&world, encounter.LocationKnown, spatial.Position{X: 1, Y: 1})
	roll := &countingCastRoller{}
	out, err := s.blessAttempt(world, baneCaster(1, 2), f.saver(0), []string{heroID, bardID, wolfID}, StaleTargetRefuse, roll)
	s.Require().NoError(err)
	result := s.castOutcome(out)
	s.Require().Len(result.Targets, 3)
	for i, id := range []string{heroID, bardID, wolfID} {
		s.Equal(id, result.Targets[i].TargetID)
		s.False(result.Targets[i].Missed)
		s.Require().Len(result.Targets[i].Applied, 1)
		s.Equal(refs.Conditions.Blessed(), result.Targets[i].Applied[0].Ref)
		s.Equal(bardID, result.Targets[i].Applied[0].Address.SourceID)
	}
	s.Zero(roll.calls, "delivery does not roll the bonus dice")
	paid := f.sheet(out, bardID)
	s.Zero(paid.ActionEconomy.ActionsRemaining)
	s.Equal(1, paid.Resources[resources.SpellSlotLevel1].Current)
	s.Contains(f.conditionRefs(paid), refs.Conditions.Concentrating().String())
	s.Equal(0, f.sheet(out, heroID).HitPoints, "Bless does not heal or stabilize")
}

func (s *CastActionTestSuite) TestBlessStalePoliciesAndMixedRecipients() {
	for _, policy := range []StaleTargetPolicy{StaleTargetRefuse, StaleTargetAttempt} {
		s.Run(string(policy), func() {
			f := s.fixtures()
			world := f.world()
			s.rememberedHero(&world, encounter.LocationKnown, spatial.Position{X: 1, Y: 2})
			caster := baneCaster(1, 2)
			before, err := json.Marshal(caster)
			s.Require().NoError(err)
			roll := &countingCastRoller{}
			out, err := s.blessAttempt(world, caster, f.saver(14), []string{bardID, heroID, wolfID}, policy, roll)
			if policy == StaleTargetRefuse {
				s.ErrorIs(err, ErrOutOfRange)
				s.Nil(out)
			} else {
				s.Require().NoError(err)
				result := s.castOutcome(out)
				s.Require().Len(result.Targets, 3)
				s.False(result.Targets[0].Missed)
				s.True(result.Targets[1].Missed)
				s.Empty(result.Targets[1].Applied)
				s.False(result.Targets[2].Missed)
				paid := f.sheet(out, bardID)
				s.Zero(paid.ActionEconomy.ActionsRemaining)
				s.Equal(1, paid.Resources[resources.SpellSlotLevel1].Current)
				encoded, marshalErr := json.Marshal(result.Targets[1])
				s.Require().NoError(marshalErr)
				s.NotContains(string(encoded), "Position")
			}
			s.Zero(roll.calls)
			after, marshalErr := json.Marshal(caster)
			s.Require().NoError(marshalErr)
			s.Equal(before, after, "caller-owned inputs stay unchanged")
		})
	}
}

func (s *CastActionTestSuite) TestBlessRefusesUnknownBlockedOutOfRangeAndBadPolicy() {
	for _, reason := range []string{"unknown", "wall", "range", "no policy", "bad policy", "duplicate"} {
		s.Run(reason, func() {
			f := s.fixtures()
			world := f.world()
			policy := StaleTargetAttempt
			targets := []string{heroID}
			switch reason {
			case "unknown":
				s.rememberedHero(&world, encounter.LocationUnknown, spatial.Position{})
			case "range":
				s.rememberedHero(&world, encounter.LocationKnown, spatial.Position{X: 9, Y: 9})
			case "wall":
				world.Field.Walls = append(world.Field.Walls, encounter.BoundaryData{From: encounter.PositionData{X: 2, Y: 1}, To: encounter.PositionData{X: 3, Y: 1}, BlocksMovement: true, BlocksLineOfSight: true})
			case "no policy":
				policy = ""
			case "bad policy":
				policy = "guess"
			case "duplicate":
				targets = []string{heroID, heroID}
			}
			caster := baneCaster(1, 2)
			roll := &countingCastRoller{}
			out, err := s.blessAttempt(world, caster, f.saver(14), targets, policy, roll)
			s.Error(err)
			s.Nil(out)
			s.Zero(roll.calls)
			s.Equal(2, caster.Resources[resources.SpellSlotLevel1].Current)
			s.Equal(1, caster.ActionEconomy.ActionsRemaining)
		})
	}
}

func (s *CastActionTestSuite) TestBlessTargetProjectionHonorsPolicyWithoutRevealingMovedLocation() {
	f := s.fixtures()
	world := f.world()
	s.rememberedHero(&world, encounter.LocationKnown, spatial.Position{X: 1, Y: 2})
	run, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: world,
		Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Standing: everyoneStanding{}, Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encounter.RefusingAnnouncer{}})
	s.Require().NoError(err)
	room, err := run.Canvas()
	s.Require().NoError(err)
	input := &KnownCreatureTargetsInput{Encounter: run, Room: room, CasterID: bardID, RangeFeet: 30,
		Candidates: []string{bardID, heroID, wolfID}, Participants: []Participant{{Character: baneCaster(1, 2)}, {Character: f.saver(0)}, {Monster: f.wolfData()}}}
	for _, policy := range []StaleTargetPolicy{StaleTargetRefuse, StaleTargetAttempt} {
		input.StaleTargetPolicy = policy
		answers, err := KnownCreatureTargets(s.ctx, input)
		s.Require().NoError(err)
		s.Equal(map[string]bool{bardID: true, heroID: policy == StaleTargetAttempt, wolfID: true}, answers)
	}
}
