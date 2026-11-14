// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// secretpassage_test.go is A SECRET PASSAGE (rpg-project#958, Passages S1 of #848)
// on the shut noise hall, a | b | c: a scullion in room a whose answer tells
// the way — an intel record that reveals the passage — a grate in room a the
// passage hides, with one line to
// room c, and an open stair beside it whose lines also go to room c and to a
// one-cell nook east of c. Alice and Bob stand in a, Dan in b behind the shut
// door, out of earshot.

const (
	psgScullion encounter.MemberID      = "scullion"
	psgGrate    encounter.PropID        = "grate"
	psgStair    encounter.PropID        = "stair"
	psgPassage  encounter.ConcealmentID = "passage"
	psgWay      encounter.IntelID       = "the-way"
	psgRoomC    encounter.RegionID      = "room-c"
	psgNook     encounter.RegionID      = "nook"
)

// psgDown is the grate's one line: down, and up again in room c.
var psgDown = encounter.Option{
	ID: "down", Say: "(lift the grate)", Success: []encounter.Answer{{Weight: 1}},
	Grants: []encounter.RowGrant{{Consequence: encounter.Consequence{
		Relocate: &encounter.RelocateGrant{To: psgRoomC, Words: "You crawl, and come up in room c.", Rounds: 1},
	}}},
}

// psgTo is a stair line to a region.
func psgTo(id string, to encounter.RegionID) encounter.Option {
	return encounter.Option{
		ID: id, Say: "(take the stair)", Success: []encounter.Answer{{Weight: 1}},
		Grants: []encounter.RowGrant{{Consequence: encounter.Consequence{
			Relocate: &encounter.RelocateGrant{To: to, Words: "The stair winds and lets you out."},
		}}},
	}
}

type SecretPassageSuite struct {
	suite.Suite
}

func TestSecretPassageSuite(t *testing.T) { suite.Run(t, new(SecretPassageSuite)) }

// hall is the shut hall with the scullion, the grate, the stair and the nook,
// and the given members besides.
func (s *SecretPassageSuite) hall(members ...encounter.MemberInput) *encounter.Encounter {
	scullion := at(psgScullion, encounter.KindMonster, 5, 2)
	scullion.Options = []encounter.Option{{
		ID: "favour", Say: "A way out?",
		Success: []encounter.Answer{{Weight: 1, Say: "The grate."}},
		Grants:  []encounter.RowGrant{{Consequence: encounter.Consequence{Tells: psgWay}}},
	}}
	members = append(members, scullion)
	sheets := sheetFacts{}
	for _, m := range members {
		sheets[m.ID] = encounter.SheetFacts{SpeedFeet: 30}
	}
	setup := noiseHallSetup(encounter.DoorIsClosed(), encounter.DoorIsClosed(), sheets, passDriver{}, members...)
	setup.Field.Regions = append(setup.Field.Regions, rectRegion(string(psgNook), 30, 0, 1, 1))
	no := false
	setup.Field.Props = []encounter.PropInput{
		{ID: psgGrate, Ref: "dnd5e:props:test:grate", At: spatial.Position{X: 5, Y: 4}, BlocksMovement: &no, BlocksLineOfSight: &no,
			Options: []encounter.Option{psgDown}},
		{ID: psgStair, Ref: "dnd5e:props:test:stair", At: spatial.Position{X: 3, Y: 4}, BlocksMovement: &no, BlocksLineOfSight: &no,
			Options: []encounter.Option{psgTo("to-c", psgRoomC), psgTo("to-nook", psgNook)}},
	}
	setup.Field.Intel = []encounter.IntelRecord{{ID: psgWay, Reveals: encounter.RevealTargets{Concealment: psgPassage}}}
	setup.Field.Concealments = []encounter.ConcealmentInput{{
		ID: psgPassage, Checks: []encounter.CheckApproach{{Ability: "investigation", DC: 30}}, Props: []encounter.PropID{psgGrate},
	}}
	setup.CheckResolver = findsNothing{}
	setup.Witness = nobodyPerceives{}
	enc, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)
	return enc
}

// party is Alice and Bob in a beside the grate, Dan in b.
func (s *SecretPassageSuite) party(more ...encounter.MemberInput) *encounter.Encounter {
	return s.hall(append([]encounter.MemberInput{
		at(relAlice, encounter.KindPlayer, 4, 4), at(relBob, encounter.KindPlayer, 6, 4), at(relDan, encounter.KindPlayer, 15, 5),
	}, more...)...)
}

// speak says a plain line to the scullion (target) or to a prop.
func (s *SecretPassageSuite) speak(enc *encounter.Encounter, actor encounter.MemberID, target encounter.MemberID,
	prop encounter.PropID, option string) (*encounter.SpeakOutput, error) {
	return enc.Speak(context.Background(), &encounter.SpeakInput{
		Actor: actor, Target: target, Prop: prop, OptionID: option, Range: speakReach, Roller: rollsLowest{},
	})
}

// told has alice ask the scullion the way.
func (s *SecretPassageSuite) told(enc *encounter.Encounter) {
	_, err := s.speak(enc, relAlice, psgScullion, "", "favour")
	s.Require().NoError(err)
}

// sees is whether member's map shows the prop.
func (s *SecretPassageSuite) sees(enc *encounter.Encounter, member encounter.MemberID, prop encounter.PropID) bool {
	atlas, err := enc.AtlasFor(member)
	s.Require().NoError(err)
	for _, p := range atlas.Props {
		if p.ID == prop {
			return true
		}
	}
	return false
}

func (s *SecretPassageSuite) regionOf(enc *encounter.Encounter, m encounter.MemberID) encounter.RegionID {
	region, ok := enc.RegionAt(s.where(enc, m))
	s.Require().True(ok)
	return region
}

func (s *SecretPassageSuite) where(enc *encounter.Encounter, id encounter.MemberID) spatial.Position {
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	s.FailNow("no such member", string(id))
	return spatial.Position{}
}

// holds is how many times member was written holding the way.
func (s *SecretPassageSuite) holds(enc *encounter.Encounter, member encounter.MemberID) int {
	n := 0
	for _, f := range enc.ToData().Journal.Facts {
		if f.Kind == "holds:intel:"+string(psgWay) && f.Actor == string(member) {
			n++
		}
	}
	return n
}

// learnedBecause is the cause of every fact that taught member the passage.
func (s *SecretPassageSuite) learnedBecause(enc *encounter.Encounter, member encounter.MemberID) []string {
	var out []string
	for _, f := range enc.ToData().Journal.Facts {
		if strings.HasPrefix(f.Kind, "known:") && strings.HasSuffix(f.Kind, string(psgPassage)) && f.Actor == string(member) {
			out = append(out, f.Detail)
		}
	}
	return out
}

// beats is every beat of kind in member's story.
func (s *SecretPassageSuite) beats(enc *encounter.Encounter, member encounter.MemberID, kind string) []map[string]any {
	var out []map[string]any
	for _, b := range storyBeats(s.T(), enc, member) {
		if b["beat"] == kind {
			out = append(out, b)
		}
	}
	return out
}

// TestUnknownItIsUnseenAndUnsayable: before anyone is told, the grate is on
// nobody's map and its line names nothing — while the open stair beside it
// is shown and offered.
func (s *SecretPassageSuite) TestUnknownItIsUnseenAndUnsayable() {
	enc := s.party()
	s.False(s.sees(enc, relAlice, psgGrate))
	s.True(s.sees(enc, relAlice, psgStair), "precondition: what stands beside it is shown")
	_, err := enc.PropOptionsFor(relAlice, psgGrate)
	s.ErrorIs(err, encounter.ErrNoProp)
	_, err = s.speak(enc, relAlice, "", psgGrate, "down")
	s.ErrorIs(err, encounter.ErrNoProp)
	s.Equal(cellAt(4, 4), s.where(enc, relAlice))
}

// TestTheAnswerLandsTheRecordOnWhoCaughtIt: Alice asks; she and Bob beside
// her catch the answer, each comes to hold the way, and the record shows each
// the grate with a reveal beat. Dan, behind the shut door, caught nothing,
// holds nothing and sees nothing.
func (s *SecretPassageSuite) TestTheAnswerLandsTheRecordOnWhoCaughtIt() {
	enc := s.party()
	s.told(enc)
	for _, m := range []encounter.MemberID{relAlice, relBob} {
		s.Equal(1, s.holds(enc, m), "%s holds the way", m)
		s.Equal([]string{"told the way to it"}, s.learnedBecause(enc, m), "%s was told it, not looted it", m)
		s.True(s.sees(enc, m, psgGrate), "%s caught the answer", m)
		offered, err := enc.PropOptionsFor(m, psgGrate)
		s.Require().NoError(err)
		s.Len(offered, 1)
		s.Len(s.beats(enc, m, encounter.BeatConcealmentRevealed), 1)
	}
	s.Zero(s.holds(enc, relDan), "out of earshot, Dan holds nothing")
	s.False(s.sees(enc, relDan, psgGrate))
	_, err := enc.PropOptionsFor(relDan, psgGrate)
	s.ErrorIs(err, encounter.ErrNoProp, "and can say nothing to it")
	s.Empty(s.beats(enc, relDan, encounter.BeatConcealmentRevealed))

	// Asked again, nobody is told twice.
	s.told(enc)
	s.Equal(1, s.holds(enc, relAlice))
	s.Len(s.beats(enc, relAlice, encounter.BeatConcealmentRevealed), 1)
}

// TestTheSpeakerLandsInTheRegion: Alice says the grate's line and is set
// down in room c, one round passing; Bob, beside her, stays. She is told the
// words; Bob is told she went, never where; the cause names the grant, never
// a place.
func (s *SecretPassageSuite) TestTheSpeakerLandsInTheRegion() {
	enc := s.party()
	s.told(enc)
	before := enc.ToData().Clock.HighWater
	_, err := s.speak(enc, relAlice, "", psgGrate, "down")
	s.Require().NoError(err)

	s.Equal(psgRoomC, s.regionOf(enc, relAlice))
	s.Equal(cellAt(6, 4), s.where(enc, relBob), "a companion who did not say it stays")
	s.Equal(before+1, enc.ToData().Clock.HighWater, "the line's round passes")

	alice := s.beats(enc, relAlice, encounter.BeatRelocated)
	s.Require().Len(alice, 1)
	s.Equal(psgDown.Grants[0].Consequence.Relocate.Words, alice[0]["words"])
	bob := s.beats(enc, relBob, encounter.BeatRelocated)
	s.Require().Len(bob, 1)
	s.Equal(encounter.RelocatedLeft, bob[0]["end"])
	s.NotContains(bob[0], "to", "who saw her go is never told where")
	s.NotContains(bob[0], "words")
	s.Equal("dnd5e:consequences:relocate", bob[0]["cause"], "the cause names no place")
}

// TestTwoTravellersLandApart: Alice goes, then Bob; each lands in room c,
// on cells of their own.
func (s *SecretPassageSuite) TestTwoTravellersLandApart() {
	enc := s.party()
	s.told(enc)
	_, err := s.speak(enc, relAlice, "", psgGrate, "down")
	s.Require().NoError(err)
	_, err = s.speak(enc, relBob, "", psgGrate, "down")
	s.Require().NoError(err)
	s.Equal(psgRoomC, s.regionOf(enc, relAlice))
	s.Equal(psgRoomC, s.regionOf(enc, relBob))
	s.NotEqual(s.where(enc, relAlice), s.where(enc, relBob))
}

// TestItIsOneWay: from room c there is no line back — the grate is in room
// a, beyond the shut doors — and she stays where she was set down.
func (s *SecretPassageSuite) TestItIsOneWay() {
	enc := s.party()
	s.told(enc)
	_, err := s.speak(enc, relAlice, "", psgGrate, "down")
	s.Require().NoError(err)
	landed := s.where(enc, relAlice)
	_, err = s.speak(enc, relAlice, "", psgGrate, "down")
	s.Error(err)
	s.Equal(landed, s.where(enc, relAlice))
}

// TestRefusedWhole: a line that would relocate its speaker is refused before
// it writes anything — no spoke beat, nobody moved — when the far end has no
// free cell, and when the speaker is in a fight.
func (s *SecretPassageSuite) TestRefusedWhole() {
	full := s.party(at(relCarol, encounter.KindPlayer, 30, 0))
	orc := encounter.MemberInput{ID: relOrc, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 5}}
	fight := s.party(orc)
	clock, err := fight.ClockOf(&encounter.ClockOfInput{Member: relAlice})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockTurn, clock.Kind, "precondition: the orc opened a fight")

	for name, tc := range map[string]struct {
		enc    *encounter.Encounter
		option string
		want   error
	}{
		"the nook is full": {full, "to-nook", encounter.ErrBadPlacement},
		"in a fight":       {fight, "to-c", encounter.ErrInBubble},
	} {
		s.Run(name, func() {
			story := len(storyBeats(s.T(), tc.enc, relAlice))
			_, err := s.speak(tc.enc, relAlice, "", psgStair, tc.option)
			s.ErrorIs(err, tc.want)
			s.Equal(cellAt(4, 4), s.where(tc.enc, relAlice), "nobody moved")
			s.Len(storyBeats(s.T(), tc.enc, relAlice), story, "the line wrote nothing")
		})
	}
}

// TestReloadRepeatsNothing: saved after the reveal and the move, the run
// loads with Alice where she landed, still knowing the passage, and the
// story exactly as long.
func (s *SecretPassageSuite) TestReloadRepeatsNothing() {
	enc := s.party()
	s.told(enc)
	_, err := s.speak(enc, relAlice, "", psgGrate, "down")
	s.Require().NoError(err)
	before, landed := len(storyBeats(s.T(), enc, relAlice)), s.where(enc, relAlice)

	blob, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	var data encounter.EncounterData
	s.Require().NoError(json.Unmarshal(blob, &data))
	setup := noiseHallSetup(encounter.DoorIsClosed(), encounter.DoorIsClosed(), sheetFacts{
		relAlice: {SpeedFeet: 30}, relBob: {SpeedFeet: 30}, relDan: {SpeedFeet: 30}, psgScullion: {SpeedFeet: 30},
	}, passDriver{})
	setup.CheckResolver = findsNothing{}
	setup.Witness = nobodyPerceives{}
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: setup.Capabilities})
	s.Require().NoError(err)
	s.Equal(landed, s.where(loaded, relAlice))
	s.Len(storyBeats(s.T(), loaded, relAlice), before, "nothing fires again")
	_, err = loaded.PropOptionsFor(relBob, psgGrate)
	s.NoError(err, "Bob still knows the way")
}

// TestArrivingInHostileSightFormsAFight: an orc alone in room c sees nobody
// until Alice comes up beside it; the fight forms and the line returns it.
func (s *SecretPassageSuite) TestArrivingInHostileSightFormsAFight() {
	orc := encounter.MemberInput{ID: relOrc, Kind: encounter.KindMonster, Position: spatial.Position{X: 28, Y: 8}}
	enc := s.party(orc)
	clock, err := enc.ClockOf(&encounter.ClockOfInput{Member: relAlice})
	s.Require().NoError(err)
	s.Require().Equal(encounter.ClockWorld, clock.Kind, "precondition: the walls keep them apart")

	out, err := s.speak(enc, relAlice, "", psgStair, "to-c")
	s.Require().NoError(err)
	s.NotNil(out.Formed, "the arrival in hostile sight forms the fight")
}
