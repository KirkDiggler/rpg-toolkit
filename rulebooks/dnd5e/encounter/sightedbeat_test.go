// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// SightedBeatTestSuite guards the per-recipient sighting beat: one observer
// being told that WHO THEY CAN SEE changed.
//
// Every case runs on one set — the same 12x12 hall split by a wall across
// y=6 that beatorder_test.go uses — because the beat is about crossing a
// sightline, and that set is the one place in this package where a sightline
// can genuinely be broken and restored by walking.
//
// Alice starts at (6,2) and the goblin at (6,10), directly across the wall's
// span from each other: they cannot see each other at first light, so every
// transition below is caused by the verb under test rather than inherited
// from setup.
type SightedBeatTestSuite struct {
	suite.Suite
}

func TestSightedBeatSuite(t *testing.T) {
	suite.Run(t, new(SightedBeatTestSuite))
}

// sighting is one decoded sighted beat, with the audience it was written for.
type sighting struct {
	audience []encounter.MemberID
	gained   []string
	lost     []string
	seq      uint64
}

// sightingsOf reads every sighted beat in one member's story.
func (s *SightedBeatTestSuite) sightingsOf(
	enc *encounter.Encounter, audience encounter.MemberID,
) []sighting {
	s.T().Helper()

	story, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)

	out := make([]sighting, 0)
	for _, entry := range story {
		var beat struct {
			Beat   string   `json:"beat"`
			Gained []string `json:"gained"`
			Lost   []string `json:"lost"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat.Beat != encounter.BeatSighted {
			continue
		}
		out = append(out, sighting{
			audience: entry.Audience, gained: beat.Gained, lost: beat.Lost, seq: entry.Seq,
		})
	}
	return out
}

// keys reads the raw JSON object keys of one member's sighted beats, which is
// how the "omitted, not empty" claim is checked — a decode into a struct
// cannot tell an absent key from a null one.
func (s *SightedBeatTestSuite) sightedKeys(
	enc *encounter.Encounter, audience encounter.MemberID,
) []map[string]any {
	s.T().Helper()

	story, err := enc.Story(&encounter.StoryInput{Audience: audience})
	s.Require().NoError(err)

	out := make([]map[string]any, 0)
	for _, entry := range story {
		var raw map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &raw))
		if raw["beat"] == encounter.BeatSighted {
			out = append(out, raw)
		}
	}
	return out
}

// blocked is the shared set: the two of them across the wall from each other,
// seeing nothing.
func (s *SightedBeatTestSuite) blocked() *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: wallRoom(),
		Members: []encounter.MemberInput{
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 6, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 6, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// NOTHING CHANGED, SO NOTHING WAS SAID. The wall keeps them apart at first
// light, so neither of them is owed a beat — and this is the case that makes
// the beat worth receiving at all. Without it the composition would append
// one per observer per refresh forever and silence would mean nothing.
func (s *SightedBeatTestSuite) TestNoTransitionAppendsNoBeat() {
	enc := s.blocked()

	s.Empty(s.sightingsOf(enc, alice), "alice saw nobody arrive and nobody leave")
	s.Empty(s.sightingsOf(enc, goblin))
}

// FIRST LIGHT DOES EMIT, when there is something to say. Standing in the open
// they see each other from the opening frame, and each is told about their own
// side of it.
func (s *SightedBeatTestSuite) TestFirstLightTellsEachObserverWhatTheySee() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: wallRoom(),
		Members: []encounter.MemberInput{
			// Both clear of the wall's span.
			{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
			{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 0, Y: 10}},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)

	hers := s.sightingsOf(enc, alice)
	s.Require().Len(hers, 1, "one beat, for the one thing that changed")
	s.Equal([]string{string(goblin)}, hers[0].gained)
	s.Empty(hers[0].lost)

	its := s.sightingsOf(enc, goblin)
	s.Require().Len(its, 1)
	s.Equal([]string{string(alice)}, its[0].gained)
}

// THE AUDIENCE IS THE OBSERVER, AND NOBODY ELSE. This is the property the
// whole beat exists for: what alice can see is news for alice. A roster-wide
// audience here would publish the sight graph to everyone, which is the one
// thing a perception beat must never do.
func (s *SightedBeatTestSuite) TestTheBeatIsAddressedToItsObserverAlone() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)

	hers := s.sightingsOf(enc, alice)
	s.Require().NotEmpty(hers)
	for _, beat := range hers {
		s.Equal([]encounter.MemberID{alice}, beat.audience,
			"alice's sighting beat goes to alice and stops there")
	}

	its := s.sightingsOf(enc, goblin)
	s.Require().NotEmpty(its, "the goblin now sees her too, and is told separately")
	for _, beat := range its {
		s.Equal([]encounter.MemberID{goblin}, beat.audience)
	}
}

// STEPPING INTO VIEW IS A GAIN — the first-contact half.
func (s *SightedBeatTestSuite) TestSteppingIntoViewIsGained() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)

	hers := s.sightingsOf(enc, alice)
	s.Require().Len(hers, 1)
	s.Equal([]string{string(goblin)}, hers[0].gained, "she walked past the wall and there it was")
	s.Empty(hers[0].lost)
}

// STEPPING OUT OF VIEW IS A LOSS, and the subject becomes a ghost rather than
// vanishing — which is what makes the return below a return.
func (s *SightedBeatTestSuite) TestSteppingOutOfViewIsLost() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(6, 2)})
	s.Require().NoError(err)

	hers := s.sightingsOf(enc, alice)
	s.Require().Len(hers, 2, "one beat for arriving in view, one for leaving it")
	s.Equal([]string{string(goblin)}, hers[1].lost, "back behind the wall")
	s.Empty(hers[1].gained)
}

// THE RETURN. A ghost becoming real again is its own beat, and this is the
// case the whole slice was built for — Kirk's "when the ghost becomes a
// reality again, how do I know what they're holding?". Without
// intel.SurveilOutput.Reacquired this pass is an ordinary refresh and says
// nothing at all.
func (s *SightedBeatTestSuite) TestSteppingBackIntoViewIsGainedAgain() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(6, 2)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)

	hers := s.sightingsOf(enc, alice)
	s.Require().Len(hers, 3, "into view, out of view, back into view")
	s.Equal([]string{string(goblin)}, hers[2].gained,
		"the ghost is current again, and she is told so")
	s.Empty(hers[2].lost)
}

// A WALK THAT CHANGES NOTHING SAYS NOTHING. She moves one cell while the wall
// still stands between them — a refresh happened, and no beat came of it.
func (s *SightedBeatTestSuite) TestAStepThatChangesNoPerceptionIsSilent() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(5, 2)})
	s.Require().NoError(err)

	s.Empty(s.sightingsOf(enc, alice), "still behind the wall, still nothing to report")
	s.Empty(s.sightingsOf(enc, goblin))
}

// OMITTED, NOT EMPTY. An absent key is "this did not happen"; an empty list
// invites a reader to think the composition looked and found none, which is a
// different claim and one this beat never makes.
func (s *SightedBeatTestSuite) TestTheHalfThatDidNotHappenIsOmitted() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)

	raw := s.sightedKeys(enc, alice)
	s.Require().Len(raw, 1)
	s.Contains(raw[0], "gained", "somebody came into view")
	s.NotContains(raw[0], "lost", "nobody left it — so the key is absent, not empty")
}

// CAUSE BEFORE EFFECT, the law refreshSight states, applied to this beat: the
// fight forms BECAUSE she saw it, so the seeing has to be readable first.
func (s *SightedBeatTestSuite) TestTheSightingPrecedesTheFightItCauses() {
	enc := s.blocked()

	out, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "walking into the open puts them in contact")

	hers := s.sightingsOf(enc, alice)
	s.Require().Len(hers, 1)
	s.Greater(out.Formed.Seq, hers[0].seq, "she sees it, THEN the fight starts")
	s.Greater(hers[0].seq, out.Seq, "and the step that caused it is ahead of both")
}

// THE STORY ENTRY IS TAGGED AS ONE, so a host can route perception beats
// without decoding every payload — the same courtesy the reveal beats extend.
func (s *SightedBeatTestSuite) TestTheBeatCarriesItsOwnTag() {
	enc := s.blocked()

	_, err := enc.Step(&encounter.StepInput{Member: alice, To: cellAt(1, 2)})
	s.Require().NoError(err)

	story, err := enc.Story(&encounter.StoryInput{Audience: alice})
	s.Require().NoError(err)

	tagged := make([]record.Entry, 0)
	for _, entry := range story {
		if entry.Tags["tag"] == "sight" {
			tagged = append(tagged, entry)
		}
	}
	s.Require().Len(tagged, 1)
	var beat map[string]any
	s.Require().NoError(json.Unmarshal(tagged[0].Payload, &beat))
	s.Equal(encounter.BeatSighted, beat["beat"])
}
