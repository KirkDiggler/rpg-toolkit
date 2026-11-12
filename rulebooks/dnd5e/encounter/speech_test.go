// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// speech_test.go is perceived speech (rpg-project#933) on the noise hall's
// three rooms: room a holds the speaker alice and the goblin she speaks to;
// door a–b stands on row 5, so bea in room b on that row sees into room a
// and cole in room b off it does not; dana in room a is short-sighted and
// sees nobody; ed is two rooms away in room c. Walls block sight, so a room
// beyond is seen only through an open doorway.
type SpeechSuite struct {
	suite.Suite
	ctx context.Context
}

func TestSpeechSuite(t *testing.T) { suite.Run(t, new(SpeechSuite)) }

func (s *SpeechSuite) SetupTest() { s.ctx = context.Background() }

const (
	speechBea  encounter.MemberID = "bea"
	speechCole encounter.MemberID = "cole"
	speechDana encounter.MemberID = "dana"
	speechEd   encounter.MemberID = "ed"
)

// speechOptions is the goblin's one checked line and its answer, the line
// and the answer at the loudness given, the answer teaching a fact.
func speechOptions(line, answer string) []encounter.Option {
	return []encounter.Option{{
		ID: "hello", Say: "Evening.", Loudness: line,
		Check:   []encounter.CheckApproach{{Ability: "persuasion", DC: 10}},
		Success: []encounter.Answer{{Weight: 1, Say: "Evening yourself.", Loudness: answer, Fact: "met-the-goblin"}},
		Failure: []encounter.Answer{{Weight: 1, Say: "Move along.", Loudness: answer}},
	}}
}

// hall opens the noise hall with the door a–b in the state given and the
// goblin's line and answer at the loudness given.
func (s *SpeechSuite) hall(ab encounter.DoorState, line, answer string) *encounter.Encounter {
	return s.hallWithAliceAt(5, ab, line, answer)
}

// hallWithAliceAt is [SpeechSuite.hall] with alice at a column of row 5.
func (s *SpeechSuite) hallWithAliceAt(col int, ab encounter.DoorState, line, answer string) *encounter.Encounter {
	return s.hallWith(col, ab, line, answer, nil)
}

// hallWith is the hall with alice at a column of row 5 and, when given, a
// participation that reports members down.
func (s *SpeechSuite) hallWith(
	col int, ab encounter.DoorState, line, answer string, participation encounter.Participation,
) *encounter.Encounter {
	goblinIn := at(goblin, encounter.KindMonster, 6, 5)
	goblinIn.Options = speechOptions(line, answer)
	setup := noiseHallSetup(ab, encounter.DoorIsOpen(),
		sheetFacts{
			alice: {SpeedFeet: 30}, goblin: {SpeedFeet: 30},
			speechBea: {SpeedFeet: 30, PassivePerception: 10}, speechCole: {SpeedFeet: 30, PassivePerception: 10},
			speechDana: {SpeedFeet: 30, PassivePerception: 10}, speechEd: {SpeedFeet: 30, PassivePerception: 10},
		},
		tableDriver(),
		at(alice, encounter.KindPlayer, col, 5),
		goblinIn,
		at(speechBea, encounter.KindPlayer, 12, 5),
		at(speechCole, encounter.KindPlayer, 12, 1),
		at(speechDana, encounter.KindPlayer, 2, 1),
		at(speechEd, encounter.KindPlayer, 25, 8),
	)
	setup.Sight = sightRanges{speechDana: 1}
	if participation != nil {
		setup.Participation = participation
	}
	enc, err := encounter.NewEncounter(setup)
	s.Require().NoError(err)
	return enc
}

// say has alice say the goblin's line, beaten or not.
func (s *SpeechSuite) say(enc *encounter.Encounter, beaten bool) error {
	_, err := enc.Speak(s.ctx, &encounter.SpeakInput{
		Range: speakReach, Actor: alice, Target: goblin, OptionID: "hello", Skill: "persuasion",
		Beaten: beaten, DC: 10, Total: map[bool]int{true: 15, false: 4}[beaten], Roller: rollsLowest{},
	})
	return err
}

// versions is every speech version a member was written, in order.
func (s *SpeechSuite) versions(enc *encounter.Encounter, who encounter.MemberID) []map[string]any {
	return speechHeardBy(s.T(), enc, who)
}

// TestEveryObserverGetsTheirVersionOfALine is the facet table for a
// player's line (R8): the goblin sees and catches it; bea sees it through
// the doorway and catches nothing; cole, off the doorway, notices a voice
// one room away; dana, in the room but seeing nobody, catches the words from
// someone unnamed; ed, two rooms off, perceives nothing. The roll travels
// only with the words, and the goblin is named only to those who see him.
func (s *SpeechSuite) TestEveryObserverGetsTheirVersionOfALine() {
	enc := s.hall(encounter.DoorIsOpen(), "", "")
	s.Require().NoError(s.say(enc, true))

	line := func(who encounter.MemberID) map[string]any {
		vs := s.versions(enc, who)
		s.Require().NotEmpty(vs, "%s was written a version of alice's line", who)
		return vs[0]
	}
	g := line(goblin)
	s.Equal(string(alice), g["speaker"])
	s.Equal("Evening.", g["words"])
	s.Contains(g, "check", "the roll travels with the words")
	s.Equal(string(goblin), g["to"], "the goblin is the one spoken to")

	b := line(speechBea)
	s.Equal(string(alice), b["speaker"], "bea sees alice through the doorway")
	s.NotContains(b, "words", "but a spoken line does not reach the next room")
	s.NotContains(b, "check", "and no words, no roll")
	s.Equal(string(goblin), b["to"], "she sees whom it was said to")
	s.NotContains(b, "hops", "a seen speaker's place needs no distance")

	c := line(speechCole)
	s.NotContains(c, "speaker", "cole sees nobody")
	s.NotContains(c, "words")
	s.NotContains(c, "to", "nor whom it was said to")
	s.EqualValues(1, c["hops"], "he noticed a voice one room off: DC 5 plus 5, against passive 10")

	d := line(speechDana)
	s.NotContains(d, "speaker", "dana does not see who spoke")
	s.Equal("Evening.", d["words"], "but the words reached her in the room")
	s.Contains(d, "check", "and the roll with them, someone's")
	s.NotContains(d, "to", "she does not see whom it was said to")
	s.EqualValues(0, d["hops"], "the same room")

	s.Empty(s.versions(enc, speechEd), "ed, two rooms off, perceives nothing")
	for _, who := range []encounter.MemberID{goblin, speechBea, speechCole, speechDana} {
		s.Equal(line(goblin)["utterance"], line(who)["utterance"], "%s's version is of the same utterance", who)
	}
}

// TestEveryObserverGetsTheirVersionOfAnAnswer is the same table for the
// goblin's answer: it is the goblin's utterance, said to alice.
func (s *SpeechSuite) TestEveryObserverGetsTheirVersionOfAnAnswer() {
	enc := s.hall(encounter.DoorIsOpen(), "", "")
	s.Require().NoError(s.say(enc, true))

	answer := func(who encounter.MemberID) map[string]any {
		vs := s.versions(enc, who)
		s.Require().Len(vs, 2, "%s was written the line and the answer", who)
		return vs[1]
	}
	a := answer(alice)
	s.Equal(string(goblin), a["speaker"])
	s.Equal("Evening yourself.", a["words"])
	s.Equal(string(alice), a["to"])
	s.NotContains(a, "check", "an answer rolls no check")

	b := answer(speechBea)
	s.Equal(string(goblin), b["speaker"])
	s.NotContains(b, "words")

	c := answer(speechCole)
	s.NotContains(c, "speaker")
	s.NotContains(c, "words")
	s.EqualValues(1, c["hops"])

	d := answer(speechDana)
	s.NotContains(d, "speaker")
	s.Equal("Evening yourself.", d["words"])
}

// TestAFactGoesToWhoCatchesTheWordsNotWhoSees is R9: the goblin's answer
// teaches whoever its words reach — dana, who saw nothing — and not bea, who
// watched it said through the doorway out of earshot.
func (s *SpeechSuite) TestAFactGoesToWhoCatchesTheWordsNotWhoSees() {
	enc := s.hall(encounter.DoorIsOpen(), "", "")
	s.Require().NoError(s.say(enc, true))

	s.True(knows(s.T(), enc, alice, "met-the-goblin"), "the one it was said to")
	s.True(knows(s.T(), enc, speechDana, "met-the-goblin"), "a hearer who saw nothing")
	s.False(knows(s.T(), enc, speechBea, "met-the-goblin"), "a watcher out of earshot")
	s.False(knows(s.T(), enc, speechCole, "met-the-goblin"), "one who only noticed a voice")
}

// TestADeedGoesToWhoCatchesTheWordsNotWhoSees is the deed's half of R9: a
// persuasion that worked lands `persuade` on whoever caught its words — the
// verdict travels with them — and not on bea, who watched it said through
// the doorway, nor cole, who only noticed a voice.
func (s *SpeechSuite) TestADeedGoesToWhoCatchesTheWordsNotWhoSees() {
	enc := s.hall(encounter.DoorIsOpen(), "", "")
	s.Require().NoError(s.say(enc, true))

	held := func(who encounter.MemberID) bool {
		holdings, err := enc.View(&encounter.ViewInput{Member: who})
		s.Require().NoError(err)
		for _, h := range holdings {
			if h.Channel == deed.Channel && h.Subject == deed.Subject(alice, encounter.DeedPersuade) {
				return true
			}
		}
		return false
	}
	s.True(held(goblin), "the one it was said to")
	s.True(held(speechDana), "a hearer who saw nothing")
	s.False(held(speechBea), "a watcher out of earshot does not know it worked")
	s.False(held(speechCole), "nor one who only noticed a voice")
}

// TestAClosedDoorCostsARoom: a shout reaches the next room through an open
// door and stops at a closed one; at speak, the closed door also stops cole
// noticing the voice (DC 5, plus 10 for two rooms, against passive 10).
func (s *SpeechSuite) TestAClosedDoorCostsARoom() {
	open := s.hall(encounter.DoorIsOpen(), "shout", "")
	s.Require().NoError(s.say(open, true))
	s.Equal("Evening.", s.versions(open, speechCole)[0]["words"], "a shout crosses an open door")

	shut := s.hall(encounter.DoorIsClosed(), "shout", "")
	s.Require().NoError(s.say(shut, true))
	s.Require().NotEmpty(s.versions(shut, speechCole), "noticed: a shout is DC 0, plus 10 for the closed door's two rooms")
	s.NotContains(s.versions(shut, speechCole)[0], "words", "but a closed door costs a room, and a shout reaches one")

	spoken := s.hall(encounter.DoorIsClosed(), "", "")
	s.Require().NoError(s.say(spoken, true))
	s.Empty(s.versions(spoken, speechCole), "spoken behind a closed door, cole notices nothing")
}

// TestAWhisperReachesOnlyTheOneAddressed: a whispered exchange is caught by
// the one each line is said to alone, noticed by nobody, seen and not heard
// by bea through the doorway; a whispered line is refused unless said from
// beside the one it is said to.
func (s *SpeechSuite) TestAWhisperReachesOnlyTheOneAddressed() {
	enc := s.hall(encounter.DoorIsOpen(), "whisper", "whisper")
	s.Require().NoError(s.say(enc, true))
	s.Equal("Evening.", s.versions(enc, goblin)[0]["words"])
	s.Equal("Evening yourself.", s.versions(enc, alice)[1]["words"], "and his whispered answer reaches alice")
	s.Empty(s.versions(enc, speechDana), "dana, in the room but seeing nobody, perceives none of it")
	s.Empty(s.versions(enc, speechCole), "and nobody notices it")
	for _, v := range s.versions(enc, speechBea) {
		s.NotContains(v, "words", "bea watches them whisper and learns nothing said")
	}

	far := s.hallWithAliceAt(4, encounter.DoorIsOpen(), "whisper", "")
	s.ErrorIs(s.say(far, true), encounter.ErrOutOfRange, "a whisper is said only from beside the one it is said to")
}

// TestAWhisperedAnswerReachesOnlyFromBeside (R10): a whispered answer row
// said to a member who spoke from across the room reaches nobody — not even
// the one it is said to — and teaches nothing; she sees him say something.
func (s *SpeechSuite) TestAWhisperedAnswerReachesOnlyFromBeside() {
	enc := s.hallWithAliceAt(4, encounter.DoorIsOpen(), "", "whisper")
	s.Require().NoError(s.say(enc, true))
	vs := s.versions(enc, alice)
	s.Require().Len(vs, 2, "her line and his answer")
	s.Equal(string(goblin), vs[1]["speaker"], "she sees him answer")
	s.NotContains(vs[1], "words", "but a whisper from two cells off does not reach her")
	s.False(knows(s.T(), enc, alice, "met-the-goblin"), "and teaches her nothing")

	near := s.hallWithAliceAt(5, encounter.DoorIsOpen(), "", "whisper")
	s.Require().NoError(s.say(near, true))
	s.Equal("Evening yourself.", s.versions(near, alice)[1]["words"], "from beside, it does")
}

// TestNoBeatCarriesALineBeyondItsHearers: the record's other beats carry no
// words — `spoke` is the speaker's own, `answered` keeps its shape with
// `say` and `to` empty — so bea, who saw everything and heard nothing, holds
// no beat with the words.
func (s *SpeechSuite) TestNoBeatCarriesALineBeyondItsHearers() {
	enc := s.hall(encounter.DoorIsOpen(), "", "")
	s.Require().NoError(s.say(enc, true))
	for _, beat := range storyBeats(s.T(), enc, speechBea) {
		for _, key := range []string{"words", "line"} {
			s.NotContains(beat, key, "bea holds no %s: %v", key, beat)
		}
		if say, ok := beat["say"]; ok {
			s.Empty(say, "no beat bea holds says the words: %v", beat)
		}
	}
	s.Empty(speechBeatsTagged(s.T(), enc, speechBea, encounter.BeatSpoke), "alice's own record is hers")
}

// speechBeatsTagged is a member's beats of one kind.
func speechBeatsTagged(t *testing.T, enc *encounter.Encounter, who encounter.MemberID, kind string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, b := range storyBeats(t, enc, who) {
		if b["beat"] == kind {
			out = append(out, b)
		}
	}
	return out
}

// TestADownMemberPerceivesNothing: dana, down in the room, is written no
// version of the line and learns nothing it teaches (R2: a member who is
// down catches and notices nothing).
func (s *SpeechSuite) TestADownMemberPerceivesNothing() {
	enc := s.hallWith(5, encounter.DoorIsOpen(), "", "", &downList{down: []encounter.MemberID{speechDana}})
	s.Require().NoError(s.say(enc, true))
	s.Empty(s.versions(enc, speechDana), "a body perceives nothing")
	s.False(knows(s.T(), enc, speechDana, "met-the-goblin"))
}

// TestAReloadReplaysTheSameVersions: every member's versions read the same
// after the encounter is saved and loaded back: they are record entries, and
// the utterance id is the record's own sequence.
func (s *SpeechSuite) TestAReloadReplaysTheSameVersions() {
	enc := s.hall(encounter.DoorIsOpen(), "", "")
	s.Require().NoError(s.say(enc, true))
	setup := noiseHallSetup(encounter.DoorIsOpen(), encounter.DoorIsOpen(), nil, tableDriver())
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: enc.ToData(), Capabilities: setup.Capabilities,
	})
	s.Require().NoError(err)
	for _, who := range []encounter.MemberID{alice, goblin, speechBea, speechCole, speechDana, speechEd} {
		s.Equal(s.versions(enc, who), s.versions(loaded, who), "%s's versions", who)
	}
}

// TestAWatcherReadsNothingOfTheLineFromAnyBeat is the review's probe made a
// test (rpg-project#933): after alice's line and the goblin's answer, bea —
// who watched through the doorway — and cole — who noticed a voice — are
// told nothing of which line it was, how it went, what it taught or what
// was said, on any beat at all; dana, who caught the words unseen, learns
// them only through her version.
func (s *SpeechSuite) TestAWatcherReadsNothingOfTheLineFromAnyBeat() {
	enc := s.hall(encounter.DoorIsOpen(), "", "")
	watchers := []encounter.MemberID{speechBea, speechCole, speechDana}
	before := map[encounter.MemberID]int{}
	for _, who := range watchers {
		entries, err := enc.Story(&encounter.StoryInput{Audience: who})
		s.Require().NoError(err)
		before[who] = len(entries)
	}
	s.Require().NoError(s.say(enc, true))

	for _, who := range watchers {
		entries, err := enc.Story(&encounter.StoryInput{Audience: who})
		s.Require().NoError(err)
		for _, entry := range entries[before[who]:] {
			var b map[string]any
			s.Require().NoError(json.Unmarshal(entry.Payload, &b))
			if b["beat"] == encounter.BeatSpeech {
				continue // their version, which the facet tables hold
			}
			s.NotEqual(encounter.BeatTable, b["beat"], "%s: a table is told to nobody", who)
			for _, key := range []string{"option", "beaten", "fact", "verb", "check", "words"} {
				s.NotContains(b, key, "%s: a %s beat says the line's %s", who, b["beat"], key)
			}
			s.Empty(b["say"], "%s: a %s beat says the words", who, b["beat"])
			k, _ := b["key"].(string)
			s.NotContains([]string{string(encounter.AnswerSuccess), string(encounter.AnswerFailure)}, k,
				"%s: a %s beat says how the line went by its table", who, b["beat"])
		}
	}
}
