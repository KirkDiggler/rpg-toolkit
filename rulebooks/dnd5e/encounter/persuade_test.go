// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// persuade_test.go is the second social verb on a real board
// (rpg-project#458): the same audience, the same refusals, its own deed and
// its own half of the table. It is Intimidate's twin, and these scenes are
// what say so.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

type PersuadeTestSuite struct {
	suite.Suite
	ctx context.Context
}

func TestPersuadeSuite(t *testing.T) {
	suite.Run(t, new(PersuadeTestSuite))
}

func (s *PersuadeTestSuite) SetupTest() {
	s.ctx = context.Background()
}

// scene is IntimidateTestSuite's room, unchanged: alice and the goblin in
// sight of each other, billy behind a wall row who can see neither.
func (s *PersuadeTestSuite) scene(members ...encounter.MemberInput) *encounter.Encounter {
	if len(members) == 0 {
		members = []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4}, Mind: "coward"},
			{ID: billy, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 10}},
		}
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(outcomeRoom, 0, 0, 12, 12)},
			Props:   wallRow(6, 4, 8),
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	return enc
}

func (s *PersuadeTestSuite) deedOf(
	enc *encounter.Encounter, observer, actor encounter.MemberID,
) (deed.Deed, bool) {
	holdings, err := enc.View(&encounter.ViewInput{Member: observer})
	s.Require().NoError(err)
	for _, h := range holdings {
		if h.Subject != deed.Subject(actor) || h.Channel != deed.Channel {
			continue
		}
		saw, err := deed.Decode(h.Payload)
		s.Require().NoError(err)

		return saw, true
	}

	return deed.Deed{}, false
}

func (s *PersuadeTestSuite) beatsOfKind(
	enc *encounter.Encounter, member core.EntityID, kind string,
) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	out := make([]map[string]any, 0)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == kind {
			out = append(out, beat)
		}
	}

	return out
}

// A beaten appeal lands its OWN deed on exactly the witnesses. Its own verb,
// not a polarity on the threat's: a mind reads the verb, and the coward's
// fear must not key on being reasoned with.
func (s *PersuadeTestSuite) TestABeatenAppealLandsItsOwnDeedOnWhoeverSawIt() {
	enc := s.scene()

	out, err := enc.Persuade(s.ctx, &encounter.PersuadeInput{
		Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	s.True(out.Beaten)
	s.Equal([]encounter.MemberID{alice, goblin}, out.Witnesses, "billy is behind the wall")

	saw, ok := s.deedOf(enc, goblin, alice)
	s.Require().True(ok, "the goblin heard her out")
	s.Equal(encounter.DeedPersuade, saw.Verb, "its own verb, never the threat's")
	s.Equal(alice, saw.Actor)
	s.Equal(goblin, saw.Target)

	_, ok = s.deedOf(enc, billy, alice)
	s.False(ok, "billy, behind the wall, never learned anything was said")
}

// A missed appeal lands no deed — the threat's rule, for the threat's reason.
func (s *PersuadeTestSuite) TestAMissedAppealLandsNoDeed() {
	enc := s.scene()

	out, err := enc.Persuade(s.ctx, &encounter.PersuadeInput{
		Actor: alice, Target: goblin, Beaten: false, DC: 9, Total: 4, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	s.False(out.Beaten)

	_, ok := s.deedOf(enc, goblin, alice)
	s.False(ok)
}

// The roll is seen either way, under [encounter.BeatPersuaded] — its own beat
// name, which the session's decoder reads by the same constant.
func (s *PersuadeTestSuite) TestTheTableSeesTheDieWhetherItLandedOrNot() {
	for _, beaten := range []bool{true, false} {
		enc := s.scene()
		_, err := enc.Persuade(s.ctx, &encounter.PersuadeInput{
			Actor: alice, Target: goblin, Beaten: beaten, DC: 10, Total: 13, Roller: rollsLowest{},
		})
		s.Require().NoError(err)

		beats := s.beatsOfKind(enc, goblin, encounter.BeatPersuaded)
		s.Require().Len(beats, 1)
		s.Equal(string(alice), beats[0]["actor"])
		s.Equal(string(goblin), beats[0]["target"])
		s.EqualValues(10, beats[0]["dc"])
		s.EqualValues(13, beats[0]["total"])
		s.Equal(beaten, beats[0]["beaten"], "false beside a miss, never absent")

		s.Empty(s.beatsOfKind(enc, goblin, encounter.BeatIntimidated),
			"an appeal is not a threat, and does not write the threat's beat")
	}
}

// The refusals are the threat's, in the same order, with the same sentinels.
func (s *PersuadeTestSuite) TestRefusals() {
	enc := s.scene()

	_, err := enc.Persuade(s.ctx, nil)
	s.ErrorIs(err, encounter.ErrNilInput)

	_, err = enc.Persuade(s.ctx, &encounter.PersuadeInput{Actor: alice, Roller: rollsLowest{}})
	s.ErrorIs(err, encounter.ErrNoMember, "an appeal with no target")

	_, err = enc.Persuade(s.ctx, &encounter.PersuadeInput{
		Actor: "nobody", Target: goblin, Roller: rollsLowest{}})
	s.ErrorIs(err, encounter.ErrNotMember)

	_, err = enc.Persuade(s.ctx, &encounter.PersuadeInput{
		Actor: alice, Target: "nobody", Roller: rollsLowest{}})
	s.ErrorIs(err, encounter.ErrNotMember)

	_, err = enc.Persuade(s.ctx, &encounter.PersuadeInput{
		Actor: alice, Target: alice, Roller: rollsLowest{}})
	s.ErrorIs(err, encounter.ErrNotMember, "talking yourself round is a caller defect")

	_, err = enc.Persuade(s.ctx, &encounter.PersuadeInput{
		Actor: alice, Target: billy, Beaten: true, Roller: rollsLowest{}})
	s.ErrorIs(err, encounter.ErrUnwitnessed)
	s.Empty(s.beatsOfKind(enc, alice, encounter.BeatPersuaded), "a refusal writes no beat")
}

// The authored appeal crosses onto the member the way the threat's does, and
// survives a save and a reload.
func (s *PersuadeTestSuite) TestTheAuthoredAppealCrossesAndSurvivesAReload() {
	approaches := []encounter.CheckApproach{{Ability: "persuasion", DC: 10}}
	table := map[string][]encounter.Reaction{
		encounter.ReactionPersuadeFailed: {{Weight: 3, Say: "Go right."}, {Weight: 1, Say: "Go left."}},
	}
	enc := s.scene(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4},
			Persuade: approaches, Reactions: table},
	)

	read := func(enc *encounter.Encounter) encounter.Member {
		members, err := enc.Members()
		s.Require().NoError(err)
		for _, m := range members {
			if m.ID == goblin {
				return m
			}
		}
		s.Require().Fail("no goblin")

		return encounter.Member{}
	}

	s.Equal(approaches, read(enc).Persuade)
	s.Equal(table, read(enc).Reactions)

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)
	s.Equal(approaches, read(reloaded).Persuade, "and survives a reload")
	s.Equal(table, read(reloaded).Reactions, "weights included: a persisted 1 is a 1, not an absence")
}

// A persuade route with nothing to beat is the same defect an intimidate one
// is. An ABSENT list is not a defect — that is every monster.
func (s *PersuadeTestSuite) TestAnApproachWithNothingToBeatIsRefused() {
	_, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: encounter.FieldInput{
			Canvas:  openAir(),
			Regions: []encounter.RegionInput{rectRegion("yard", 0, 0, 6, 6)},
		},
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 1}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1},
				Persuade: []encounter.CheckApproach{{Ability: "persuasion"}}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().ErrorIs(err, encounter.ErrNoMember)
}
