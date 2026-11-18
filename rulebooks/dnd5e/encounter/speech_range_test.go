// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// speech_range_test.go is rpg-project#953: a line spoken at `speak` carries
// a short way — [encounter.SpeechCarryFeet] — out of the speaker's room
// through an OPEN doorway, measured along the way through the doorway, never
// in a straight line through a wall. On the noise hall: door a–b is the edge
// on row 5 between columns 9 and 10.
type SpeechRangeSuite struct {
	suite.Suite
}

func TestSpeechRangeSuite(t *testing.T) { suite.Run(t, new(SpeechRangeSuite)) }

const (
	// rangeNear stands in room b just through the doorway, on its row.
	rangeNear encounter.MemberID = "near"
	// rangeAround stands in room b off the doorway's row, out of the
	// speaker's sight, a short walk round through the doorway.
	rangeAround encounter.MemberID = "around"
	// rangeWall stands in room b just the other side of the seam wall from
	// a speaker at row 1, far from the doorway.
	rangeWall encounter.MemberID = "wall"
)

// scene is the noise hall with door a–b as given, alice at [col,row] in
// room a and the goblin beside her carrying one line at loudness, and the
// three listeners of room b.
func (s *SpeechRangeSuite) scene(ab encounter.DoorState, col, row int, loudness string) *encounter.Encounter {
	goblinIn := at(goblin, encounter.KindMonster, col-1, row)
	goblinIn.Options = []encounter.Option{{
		ID: "hello", Say: "Evening.", Loudness: loudness,
		Success: []encounter.Answer{{Weight: 1, Say: "Evening yourself."}},
	}}
	setup := noiseHallSetup(ab, encounter.DoorIsClosed(),
		sheetFacts{
			alice: {SpeedFeet: 30}, goblin: {SpeedFeet: 30},
			rangeNear: {SpeedFeet: 30}, rangeAround: {SpeedFeet: 30}, rangeWall: {SpeedFeet: 30},
		},
		tableDriver(),
		at(alice, encounter.KindPlayer, col, row),
		goblinIn,
		at(rangeNear, encounter.KindPlayer, 11, 5),
		at(rangeAround, encounter.KindPlayer, 11, 3),
		at(rangeWall, encounter.KindPlayer, 10, 1),
	)
	enc, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)
	_, err = enc.Speak(context.Background(), &encounter.SpeakInput{
		Range: speakReach, Actor: alice, Target: goblin, OptionID: "hello", Roller: rollsLowest{},
	})
	s.Require().NoError(err)
	return enc
}

// alicesLine is the version of alice's line a member was written — the
// utterance the goblin, who was spoken to, heard first — or nil.
func (s *SpeechRangeSuite) alicesLine(enc *encounter.Encounter, who encounter.MemberID) map[string]any {
	goblins := speechHeardBy(s.T(), enc, goblin)
	s.Require().NotEmpty(goblins, "the goblin caught alice's line")
	for _, v := range speechHeardBy(s.T(), enc, who) {
		if v["utterance"] == goblins[0]["utterance"] {
			return v
		}
	}
	return nil
}

// caught reports whether a member caught the words of alice's line.
func (s *SpeechRangeSuite) caught(enc *encounter.Encounter, who encounter.MemberID) bool {
	v := s.alicesLine(enc, who)
	return v != nil && v["words"] == "Evening."
}

// TestTheWordsCarryThroughAnOpenDoorway: alice beside the doorway; the
// listener through it on its row catches the words and sees her; the one
// round the corner, unseen, catches them from someone unnamed one room off.
func (s *SpeechRangeSuite) TestTheWordsCarryThroughAnOpenDoorway() {
	enc := s.scene(encounter.DoorIsOpen(), 8, 5, "")
	s.True(s.caught(enc, rangeNear), "through the open doorway, within the carry")
	s.Equal(string(alice), s.alicesLine(enc, rangeNear)["speaker"])

	around := s.alicesLine(enc, rangeAround)
	s.Require().NotNil(around)
	s.Equal("Evening.", around["words"], "round the corner, within the carry by the doorway")
	s.NotContains(around, "speaker", "from someone unseen")
	s.EqualValues(1, around["hops"], "one room off")
}

// TestTheWordsStopBeyondTheCarry: alice seven cells and more from the
// doorway; the listener just through it is beyond the carry and catches no
// words, though she still sees alice say something.
func (s *SpeechRangeSuite) TestTheWordsStopBeyondTheCarry() {
	enc := s.scene(encounter.DoorIsOpen(), 2, 5, "")
	s.Require().Greater(enc.Distance(cellAt(2, 5), cellAt(10, 5)), float64(encounter.CellsFromFeet(encounter.SpeechCarryFeet)),
		"precondition: the doorway is beyond the carry from alice")
	s.False(s.caught(enc, rangeNear), "beyond the carry, no words")
	s.False(s.caught(enc, rangeAround))
}

// TestAClosedDoorStopsTheWords: alice beside the doorway, the door shut:
// nobody in room b catches the words.
func (s *SpeechRangeSuite) TestAClosedDoorStopsTheWords() {
	enc := s.scene(encounter.DoorIsClosed(), 8, 5, "")
	for _, who := range []encounter.MemberID{rangeNear, rangeAround, rangeWall} {
		s.False(s.caught(enc, who), "%s, behind the closed door", who)
	}
}

// TestAWallIsNotADoorway: alice at row 1 beside the seam wall, the listener
// two cells off in a straight line through the wall; the way round by the
// open doorway is longer than the carry, so no words.
func (s *SpeechRangeSuite) TestAWallIsNotADoorway() {
	enc := s.scene(encounter.DoorIsOpen(), 8, 1, "")
	s.Require().LessOrEqual(enc.Distance(cellAt(8, 1), cellAt(10, 1)), 2.0, "precondition: near in a straight line")
	s.False(s.caught(enc, rangeWall), "a wall with no doorway near carries no words")
}

// TestOtherLoudnessesAreUnchanged: a whisper beside the doorway reaches only
// the goblin; a shout from beyond the carry still reaches all of room b.
func (s *SpeechRangeSuite) TestOtherLoudnessesAreUnchanged() {
	whisper := s.scene(encounter.DoorIsOpen(), 8, 5, "whisper")
	for _, who := range []encounter.MemberID{rangeNear, rangeAround, rangeWall} {
		s.False(s.caught(whisper, who), "%s catches no whisper", who)
	}
	shout := s.scene(encounter.DoorIsOpen(), 2, 1, "shout")
	for _, who := range []encounter.MemberID{rangeNear, rangeAround, rangeWall} {
		s.True(s.caught(shout, who), "%s catches a shout from the next room, however far", who)
	}
}
