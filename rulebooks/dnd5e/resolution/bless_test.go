// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
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
