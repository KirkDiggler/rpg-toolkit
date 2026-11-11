// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/stretchr/testify/suite"
)

type LinkedApplicationSuite struct {
	suite.Suite
	ctx context.Context
}

func TestLinkedApplicationSuite(t *testing.T) { suite.Run(t, new(LinkedApplicationSuite)) }
func (s *LinkedApplicationSuite) SetupTest()  { s.ctx = context.Background() }
func (s *LinkedApplicationSuite) fixtures() *ContestDamageTestSuite {
	f := &ContestDamageTestSuite{}
	f.SetT(s.T())
	f.ctx = s.ctx
	return f
}
func (s *LinkedApplicationSuite) resolveLinked(world encounter.EncounterData, participants []Participant, machine Machine, cost *Cost) *Output {
	out, err := Resolve(s.ctx, &Input{World: world, Participants: participants, Machine: machine, Cost: cost, Capabilities: encounter.Capabilities{
		Initiative: orderAsGiven{}, Driver: passDriver{}, Participation: everyoneParticipates{}, Sight: everyoneSeesTheWholeMap{}, Roller: dice.NewRoller(), Equipment: noHandsAreObserved{}, Conditions: noHandsAreObserved{}, Sheets: noSheetsAsked{}, Actors: Actors,
	}})
	s.Require().NoError(err)
	return out
}
func (s *LinkedApplicationSuite) linkedOwner(stored []json.RawMessage) *conditions.LinkedCondition {
	for _, raw := range stored {
		condition, err := conditions.LoadJSON(raw)
		s.Require().NoError(err)
		if owner, ok := condition.(*conditions.LinkedCondition); ok && owner.LinkedApplicationInfo().Role == conditions.LinkedCoordinator {
			return owner
		}
	}
	s.Require().Fail("missing current linked coordinator")
	return nil
}
func (s *LinkedApplicationSuite) savedParticipants(out *Output, old []Participant) []Participant {
	next := append([]Participant(nil), old...)
	for i, participant := range next {
		for _, sheet := range out.DirtyCharacters {
			if sheet.ID == participant.ID() {
				next[i] = Participant{Character: sheet}
			}
		}
		for _, sheet := range out.DirtyMonsters {
			if sheet.ID == participant.ID() {
				next[i] = Participant{Monster: sheet}
			}
		}
	}
	return next
}

func (s *LinkedApplicationSuite) TestCastSealReloadAndExactRemoval() {
	f := s.fixtures()
	resistance, err := conditions.NewResistanceCondition(conditions.NewResistanceConditionInput{MemberID: heroID, SourceID: "cleric", SourceRef: refs.Spells.Resistance()})
	s.Require().NoError(err)
	raw, err := resistance.ToJSON()
	s.Require().NoError(err)
	participants := []Participant{{Character: f.saver(14, raw)}, {Monster: f.wolfData()}, {Character: baneCaster(1)}}
	machine, err := NewAction(&ActionInput{ApplicationID: "cast-pause-1", Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{wolfID, heroID}, Roller: facedRoller{d20: 1, other: 1}})
	s.Require().NoError(err)
	paused := s.resolveLinked(f.world(), participants, machine, baneCost())
	s.Require().NotNil(paused.Posed)
	s.Equal(heroID, paused.Posed.Ask.Audience)
	owner := s.linkedOwner(f.sheet(paused, bardID).Conditions)
	s.Equal("cast-pause-1", owner.LinkedApplicationInfo().ID)
	s.Len(owner.LinkedApplicationInfo().LiveMembers, 1)
	s.Len(owner.LinkedApplicationInfo().PendingMembers, 1)
	s.Require().Len(paused.DirtyMonsters, 1, "the earlier Bane is durable while another target answers")
	saved := s.savedParticipants(paused, participants)
	s.Run("frozen maintenance cannot disappear", func() {
		header, err := readFrozen(paused.Posed.Frozen)
		s.Require().NoError(err)
		var state frozenCast
		s.Require().NoError(decodeState(header, &state))
		state.Definition.Cast.Maintenance = nil
		changed := *paused.Posed
		changed.Frozen, err = writeFrozen(header.Machine, header.Kind, state)
		s.Require().NoError(err)
		_, err = Resume(&ResumeInput{Pause: changed, Answer: Decline(), Roller: facedRoller{d20: 1, other: 1}})
		s.Require().ErrorIs(err, ErrBadFrozen)
	})
	resumed, err := Resume(&ResumeInput{Pause: *paused.Posed, Answer: Decline(), Roller: facedRoller{d20: 1, other: 1}})
	s.Require().NoError(err)
	finished := s.resolveLinked(paused.World, saved, resumed, nil)
	s.Nil(finished.Posed)
	owner = s.linkedOwner(f.sheet(finished, bardID).Conditions)
	s.Equal("cast-pause-1", owner.LinkedApplicationInfo().ID)
	s.Len(owner.LinkedApplicationInfo().LiveMembers, 2)
	s.Empty(owner.LinkedApplicationInfo().PendingMembers)
	saved = s.savedParticipants(finished, saved)
	struck := s.resolveLinked(finished.World, saved, NewStrike(&StrikeInput{AttackerID: wolfID, TargetID: bardID, Definition: claw("10d6"), Roller: facedRoller{d20: 20, other: 3}}), nil)
	s.Require().Len(struck.ConcentrationBreaks, 1)
	saved = s.savedParticipants(struck, saved)
	for _, participant := range saved {
		var blobs []json.RawMessage
		if participant.Character != nil {
			blobs = participant.Character.Conditions
		} else {
			blobs = participant.Monster.Conditions
		}
		for _, blob := range blobs {
			condition, err := conditions.LoadJSON(blob)
			s.Require().NoError(err)
			if linked, ok := condition.(*conditions.LinkedCondition); ok {
				s.NotEqual("cast-pause-1", linked.LinkedApplicationInfo().ID)
			}
		}
	}
}

func (s *LinkedApplicationSuite) TestSelfBlessConcentrationUsesDieBeforeCleanup() {
	f := s.fixtures()
	definition := spells.CastDefinition(spells.CastDefinitionInput{Spell: spells.Bless})
	machine, err := NewAction(&ActionInput{ApplicationID: "self-bless", Definition: *definition, AttackerID: bardID, TargetIDs: []string{bardID, heroID}, StaleTargetPolicy: StaleTargetRefuse})
	s.Require().NoError(err)
	participants := []Participant{{Character: f.saver(14)}, {Character: baneCaster(1)}, {Monster: f.wolfData()}}
	cast := s.resolveLinked(f.world(), participants, machine, baneCost())
	saved := s.savedParticipants(cast, participants)
	struck := s.resolveLinked(cast.World, saved, NewStrike(&StrikeInput{AttackerID: wolfID, TargetID: bardID, Definition: claw("1d6"), Roller: facedRoller{d20: 3, other: 4}}), nil)
	outcome := struck.Outcome.(StrikeOutcome)
	s.Require().Len(outcome.FollowUps, 1, "two recipients still produce only one concentration check")
	calculation := outcome.FollowUps[0].Save.Result.Calculation
	s.Equal(8, calculation.Total, "3 + CON 1 + Bless 4, evaluated before cleanup")
	s.Require().Len(calculation.Components, 3)
	s.Equal(bardID, calculation.Components[2].Source.SourceID)
	s.Equal([]int{4}, calculation.Components[2].Dice.FinalRolls)
	s.Require().Len(struck.ConcentrationBreaks, 1)
	s.Len(struck.ConcentrationBreaks[0].Removed, 2)
	s.Equal(5, f.sheet(struck, bardID).HitPoints, "the strike applies damage once")
	s.Empty(f.sheet(struck, heroID).Conditions)
}

func (s *LinkedApplicationSuite) TestLongRestPersistsEveryAffectedHolder() {
	f := s.fixtures()
	machine, err := NewAction(&ActionInput{ApplicationID: "rest-application", Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{heroID, wolfID}, Roller: facedRoller{d20: 1, other: 1}})
	s.Require().NoError(err)
	cast := s.resolveLinked(f.world(), []Participant{{Character: f.saver(14)}, {Character: baneCaster(1)}, {Monster: f.wolfData()}}, machine, baneCost())
	caster := f.sheet(cast, bardID)
	refused, err := LongRest(s.ctx, &LongRestInput{Character: caster})
	s.Require().ErrorIs(err, ErrBadParticipant)
	s.Nil(refused)
	out, err := LongRest(s.ctx, &LongRestInput{Character: caster, Others: []Participant{{Character: f.sheet(cast, heroID)}, {Monster: cast.DirtyMonsters[0]}}})
	s.Require().NoError(err)
	s.Require().Len(out.ConcentrationBreaks, 1)
	s.Require().Len(out.DirtyCharacters, 1)
	s.Equal(heroID, out.DirtyCharacters[0].ID)
	s.Require().Len(out.DirtyMonsters, 1)
	s.Equal(wolfID, out.DirtyMonsters[0].ID)
	s.Empty(out.DirtyCharacters[0].Conditions)
	s.Empty(out.DirtyMonsters[0].Conditions)
	s.Equal("rest-application", s.linkedOwner(caster.Conditions).LinkedApplicationInfo().ID, "input custody is unchanged")
}

func (s *LinkedApplicationSuite) TestUnsupportedNonSpellNativePresentationIsRefusedByName() {
	f := s.fixtures()
	definition := baneDefinition().Clone()
	definition.Ref = core.Ref{Module: "example", Type: "items", ID: "ward-ring"}
	machine, err := NewAction(&ActionInput{ApplicationID: "item-application", Definition: definition, AttackerID: bardID, TargetIDs: []string{heroID}, Roller: facedRoller{d20: 1, other: 1}})
	s.Require().NoError(err)
	caster := baneCaster(1)
	before, err := json.Marshal(caster)
	s.Require().NoError(err)
	out, err := f.resolve(f.saver(14), machine, baneCost(), caster)
	s.Require().ErrorContains(err, "unsupported non-spell native maintenance source")
	s.Nil(out)
	after, err := json.Marshal(caster)
	s.Require().NoError(err)
	s.Equal(before, after)
}

func (s *LinkedApplicationSuite) TestMaintainedCastRequiresCallerApplicationIdentity() {
	_, err := NewAction(&ActionInput{Definition: *baneDefinition(), AttackerID: bardID, TargetIDs: []string{heroID}, Roller: facedRoller{d20: 1}})
	s.Require().Error(err)
}
