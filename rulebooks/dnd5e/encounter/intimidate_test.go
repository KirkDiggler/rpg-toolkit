// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

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

// IntimidateTestSuite is the first shenanigan on a real board
// (rpg-project#454): a threat reaches exactly who could see it, a beaten one
// lands a deed, a missed one lands nothing, and the world learns only what
// the author planted.
type IntimidateTestSuite struct {
	suite.Suite
	ctx context.Context
}

func (s *IntimidateTestSuite) SetupTest() {
	s.ctx = context.Background()
}

func TestIntimidateSuite(t *testing.T) {
	suite.Run(t, new(IntimidateTestSuite))
}

// scene is the deed suite's room: alice and the goblin in sight of each
// other, billy on the far side of a wall row who can see neither.
func (s *IntimidateTestSuite) scene(members ...encounter.MemberInput) *encounter.Encounter {
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

func (s *IntimidateTestSuite) deedOf(
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

func (s *IntimidateTestSuite) beatsOfKind(
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

// A beaten threat lands the deed on EXACTLY the witnesses: the goblin, who
// can see alice, holds it; billy, behind the wall, never learned it happened.
func (s *IntimidateTestSuite) TestABeatenThreatLandsOnWhoeverSawIt() {
	enc := s.scene()

	out, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
		Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	s.True(out.Beaten)
	s.Equal([]encounter.MemberID{alice, goblin}, out.Witnesses, "billy is behind the wall")

	saw, ok := s.deedOf(enc, goblin, alice)
	s.Require().True(ok, "the goblin heard the threat")
	s.Equal(encounter.DeedIntimidate, saw.Verb)
	s.Equal(alice, saw.Actor, "afraid of HER, and the mind keys fear by actor")
	s.Equal(goblin, saw.Target)

	_, ok = s.deedOf(enc, billy, alice)
	s.False(ok, "billy, behind the wall, never learned a threat was made")
}

// The witness read a caller prices against is the same set the verb lands
// on, and it answers before anything is spent.
func (s *IntimidateTestSuite) TestTheWitnessReadIsTheAudience() {
	enc := s.scene()

	witnesses, err := enc.Witnesses(alice)
	s.Require().NoError(err)
	s.Equal([]encounter.MemberID{alice, goblin}, witnesses, "billy is behind the wall")

	out, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
		Actor: alice, Target: goblin, Beaten: true, DC: 9, Total: 14, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	s.Equal(witnesses, out.Witnesses, "what the caller was told, and what the verb used")

	_, err = enc.Witnesses("nobody")
	s.ErrorIs(err, encounter.ErrNoMember)
}

// A missed threat lands NOTHING — no deed at all, which is the difference
// between this verb and an attack, where a miss is still a shot at you.
func (s *IntimidateTestSuite) TestAMissedThreatLandsNothing() {
	enc := s.scene()

	out, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
		Actor: alice, Target: goblin, Beaten: false, DC: 9, Total: 4, Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	s.False(out.Beaten)

	_, ok := s.deedOf(enc, goblin, alice)
	s.False(ok, "a threat nobody was frightened by is a sentence in the air")
}

// The roll is seen either way: a beat carries the actor, the target, the DC,
// the total and whether it landed, to every witness — under the name
// [encounter.BeatIntimidated], which the session's decoder reads by the same
// constant so a rename cannot part them silently.
func (s *IntimidateTestSuite) TestTheTableSeesTheDieWhetherItLandedOrNot() {
	for _, beaten := range []bool{true, false} {
		enc := s.scene()
		_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
			Actor: alice, Target: goblin, Beaten: beaten, DC: 9, Total: 14, Roller: rollsLowest{},
		})
		s.Require().NoError(err)

		beats := s.beatsOfKind(enc, goblin, encounter.BeatIntimidated)
		s.Require().Len(beats, 1)
		s.Equal(string(alice), beats[0]["actor"])
		s.Equal(string(goblin), beats[0]["target"])
		s.EqualValues(9, beats[0]["dc"])
		s.EqualValues(14, beats[0]["total"])
		s.Equal(beaten, beats[0]["beaten"], "false beside a miss, never absent")

		s.Empty(s.beatsOfKind(enc, billy, encounter.BeatIntimidated), "billy saw nothing to narrate")
	}
}

// A target that cannot see the actor is refused, and nothing is written.
func (s *IntimidateTestSuite) TestAThreatThroughAWallIsRefused() {
	enc := s.scene()

	_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
		Actor: alice, Target: billy, Beaten: true, DC: 9, Total: 20, Roller: rollsLowest{},
	})
	s.Require().ErrorIs(err, encounter.ErrUnwitnessed)
	s.Empty(s.beatsOfKind(enc, alice, encounter.BeatIntimidated), "a refusal writes no beat")
}

// The rest of the refusals, in validation order.
func (s *IntimidateTestSuite) TestRefusals() {
	enc := s.scene()

	_, err := enc.Intimidate(s.ctx, nil)
	s.ErrorIs(err, encounter.ErrNilInput)

	_, err = enc.Intimidate(s.ctx, &encounter.IntimidateInput{Actor: alice, Roller: rollsLowest{}})
	s.ErrorIs(err, encounter.ErrNoMember, "a threat with no target")

	_, err = enc.Intimidate(s.ctx, &encounter.IntimidateInput{Actor: "nobody", Target: goblin, Roller: rollsLowest{}})
	s.ErrorIs(err, encounter.ErrNotMember)

	_, err = enc.Intimidate(s.ctx, &encounter.IntimidateInput{Actor: alice, Target: "nobody", Roller: rollsLowest{}})
	s.ErrorIs(err, encounter.ErrNotMember)

	_, err = enc.Intimidate(s.ctx, &encounter.IntimidateInput{Actor: alice, Target: alice, Roller: rollsLowest{}})
	s.ErrorIs(err, encounter.ErrNotMember, "frightening yourself is a caller defect")
}

// The world half: the fact is learned by every witness, and only when the
// author planted it. The camp is hostile until it knows, so the stance
// turning is the proof the fact arrived.
func (s *IntimidateTestSuite) TestTheCampLearnsOnlyWhatTheAuthorPlanted() {
	const sergeant = core.EntityID("sergeant")

	open := func(fact encounter.FactID) *encounter.Encounter {
		var table map[string][]encounter.Reaction
		if fact != "" {
			table = map[string][]encounter.Reaction{
				encounter.ReactionIntimidated: {{Weight: 1, Fact: fact}},
			}
		}
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight:     everyoneSeesTheWholeMap{},
			Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
			Field: encounter.FieldInput{
				Canvas:   openAir(),
				Regions:  []encounter.RegionInput{rectRegion("yard", 0, 0, 6, 6)},
				Factions: []encounter.FactionInput{{ID: campFaction, Mind: sergeant}},
				Dispositions: []encounter.DispositionInput{{
					Between: [2]encounter.FactionID{campFaction, encounter.FactionParty},
					Stance:  encounter.StanceHostile, Until: encounter.TriggerFact{Fact: campFact},
				}},
			},
			Members: []encounter.MemberInput{
				{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 1}},
				{ID: sergeant, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 1},
					Faction: campFaction, Reactions: table},
			},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		s.Require().NoError(err)
		return enc
	}
	stance := func(enc *encounter.Encounter) encounter.Stance {
		out, err := enc.Stance(campFaction, encounter.FactionParty)
		s.Require().NoError(err)
		return out
	}

	s.Run("authored: cowing the sergeant in front of the camp turns it", func() {
		enc := open(campFact)
		s.Require().Equal(encounter.StanceHostile, stance(enc), "precondition: nobody knows yet")

		_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
			Actor: alice, Target: sergeant, Beaten: true, DC: 10, Total: 17, Roller: rollsLowest{},
		})
		s.Require().NoError(err)
		s.Equal(encounter.StanceNeutral, stance(enc))
	})

	s.Run("authored but missed: the camp learns nothing", func() {
		enc := open(campFact)
		_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
			Actor: alice, Target: sergeant, Beaten: false, DC: 10, Total: 3, Roller: rollsLowest{},
		})
		s.Require().NoError(err)
		s.Equal(encounter.StanceHostile, stance(enc))
	})

	s.Run("unauthored: a scared sergeant does not turn a camp nobody told", func() {
		enc := open("")
		_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
			Actor: alice, Target: sergeant, Beaten: true, DC: 10, Total: 17, Roller: rollsLowest{},
		})
		s.Require().NoError(err)
		s.Equal(encounter.StanceHostile, stance(enc))
	})
}

// The authored check and the authored fact cross into the member the way
// Targeting and Actions do, and survive a save and a reload — including the
// reload's own trust boundary, which has to accept the fact this placement
// mints.
func (s *IntimidateTestSuite) TestTheAuthoredCheckCrossesLikeTargeting() {
	approaches := []encounter.CheckApproach{
		{Ability: "intimidation", DC: 12},
		{Ability: "str", DC: 15},
	}
	enc := s.scene(
		encounter.MemberInput{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
		encounter.MemberInput{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 4},
			Mind: "coward", Intimidate: approaches, Reactions: map[string][]encounter.Reaction{
				encounter.ReactionIntimidated: {{Weight: 1, Fact: campFact}},
			}},
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

	s.Equal(approaches, read(enc).Intimidate)
	s.Equal(campFact, read(enc).Reactions[encounter.ReactionIntimidated][0].Fact)

	// Learn the fact first, so the blob carries a `known:fact` this field
	// mints ONLY because the placement authored it — the trust boundary has
	// to accept it on the way back in.
	_, err := enc.Intimidate(s.ctx, &encounter.IntimidateInput{
		Actor: alice, Target: goblin, Beaten: true, DC: 12, Total: 18, Roller: rollsLowest{},
	})
	s.Require().NoError(err)

	reloaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Sight: everyoneSeesTheWholeMap{}, Equipment: noHandsAreObserved{},
		Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{},
		Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
	})
	s.Require().NoError(err)
	s.Equal(approaches, read(reloaded).Intimidate, "and survives a reload")
	s.Equal(campFact, read(reloaded).Reactions[encounter.ReactionIntimidated][0].Fact,
		"the table survives the reload, and the blob's trust boundary accepts the fact it mints")
}

// A route with nothing to beat is the same defect on a member that it is on
// a lock. An ABSENT list is not a defect — that is every monster.
func (s *IntimidateTestSuite) TestAnApproachWithNothingToBeatIsRefused() {
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
				Intimidate: []encounter.CheckApproach{{Ability: "intimidation"}}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().ErrorIs(err, encounter.ErrNoMember)
}
