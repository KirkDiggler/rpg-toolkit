// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// speech_fold_internal_test.go is each version of something said, read into
// the acts its reader is shown (rpg-project#933): never a part the version
// does not hold.

// speechVersion is one speech version's payload, the fields given over its place.
func speechVersion(fields map[string]any) map[string]any {
	out := map[string]any{"beat": encounter.BeatSpeech, "utterance": "u1", "place": map[string]any{"x": 4, "y": 2}, "loudness": "speak"}
	for k, v := range fields {
		out[k] = v
	}
	return out
}

// TestAnotherCharactersLineCaughtAndSeenIsTheirSpeakAct: bob, who saw alice
// and caught her words, reads her line, whom it was said to, and its roll.
func (s *ActFoldSuite) TestAnotherCharactersLineCaughtAndSeenIsTheirSpeakAct() {
	acts := foldActs("sess", "bob", []record.Entry{s.beat("speak:one", speechVersion(map[string]any{
		"speaker": "alice", "to": "goblin", "words": "Evening.", "option": "hello",
		"check": map[string]any{"skill": "persuasion", "dc": 10, "total": 15, "beaten": true},
	}))})
	s.Require().Len(acts, 1)
	act, ok := acts[0].Body.(SpeakAct)
	s.Require().True(ok, "a %T", acts[0].Body)
	s.Equal(SpeakAct{
		By: member("alice"), To: member("goblin"), Option: "hello", Line: "Evening.", Skill: "persuasion",
		Roll: act.Roll,
	}, act)
	s.Require().NotNil(act.Roll, "the roll travels with the words")
	s.Equal(15, act.Roll.Total)
	s.Equal(true, *act.Roll.Beaten)
}

// TestALineWithoutWhomItWasSaidToNamesNobody: a version that does not
// see the addressee names nobody spoken to.
func (s *ActFoldSuite) TestALineWithoutWhomItWasSaidToNamesNobody() {
	acts := foldActs("sess", "bob", []record.Entry{s.beat("speak:one", speechVersion(map[string]any{
		"speaker": "alice", "words": "Evening.", "option": "hello",
	}))})
	s.Require().Len(acts, 1)
	act := acts[0].Body.(SpeakAct)
	s.Equal(Party{}, act.To)
	s.Nil(act.Roll, "a plain line carries no roll")
}

// TestAVersionWithNoWordsGivesNoLine: the speaker's own version (their spoke
// record carries it) and a version that only saw them speak give nothing.
func (s *ActFoldSuite) TestAVersionWithNoWordsGivesNoLine() {
	own := s.beat("speak:one", speechVersion(map[string]any{"speaker": "alice", "words": "Evening.", "option": "hello"}))
	seen := s.beat("speak:one", speechVersion(map[string]any{"speaker": "alice", "to": "goblin"}))
	s.Empty(foldActs("sess", "alice", []record.Entry{own}), "her own line is her spoke record's")
	s.Empty(foldActs("sess", "bob", []record.Entry{seen}), "bob saw her speak and learns no line")
}

// TestWordsFromNobodySeenAreAVoice: words caught from a speaker not seen, and
// a voice only noticed, are each a voice where it was said and how many
// rooms off; no words, no speaker, no roll.
func (s *ActFoldSuite) TestWordsFromNobodySeenAreAVoice() {
	for name, fields := range map[string]map[string]any{
		"words unseen": {"hops": 0, "words": "Evening.", "option": "hello", "check": map[string]any{"skill": "persuasion", "dc": 10, "total": 15, "beaten": true}},
		"noticed":      {"hops": 1},
	} {
		s.Run(name, func() {
			acts := foldActs("sess", "dana", []record.Entry{s.beat("speak:one", speechVersion(fields))})
			s.Require().Len(acts, 1)
			p, ok := acts[0].Body.(Perception)
			s.Require().True(ok, "a %T", acts[0].Body)
			s.Equal(SenseHearing, p.Sense)
			s.Equal(&Sound{Kind: encounter.SoundVoice, Cell: cell(4, 2), Hops: fields["hops"].(int)}, p.Heard)
		})
	}
}

// TestACreaturesWordsFoldIntoItsPick: the words a creature said complete
// the pick that said them — a time pick's table answer, and the answer
// inside the line it answers — with whom they were said to.
func (s *ActFoldSuite) TestACreaturesWordsFoldIntoItsPick() {
	pick := s.beat("time:one", map[string]any{"beat": encounter.BeatAnswered, "creature": "guard", "key": "behavior/6:policy/time", "say": "", "to": ""})
	said := s.beat("time:one", speechVersion(map[string]any{"speaker": "guard", "to": "alice", "words": "To arms!"}))
	acts := foldActs("sess", "alice", []record.Entry{pick, said})
	s.Require().Len(acts, 1, "one table answer, its words folded in")
	answer := acts[0].Body.(TableAnswer)
	s.Equal("To arms!", answer.Say)
	s.Equal(member("alice"), answer.To)

	spoke := s.beat("speak:two", map[string]any{"beat": encounter.BeatSpoke, "actor": "alice", "target": "goblin", "option": "hello", "line": "Evening."})
	answered := s.beat("speak:two", map[string]any{"beat": encounter.BeatAnswered, "creature": "goblin", "say": "", "to": ""})
	reply := s.beat("speak:two", speechVersion(map[string]any{"speaker": "goblin", "to": "alice", "words": "Evening yourself."}))
	acts = foldActs("sess", "alice", []record.Entry{spoke, answered, reply})
	s.Require().Len(acts, 1, "one speak act, the answer and its words folded in")
	line := acts[0].Body.(SpeakAct)
	s.Require().NotNil(line.Answer)
	s.Equal("Evening yourself.", line.Answer.Say)
	s.Equal(member("alice"), line.Answer.To)
}

// TestACreaturesWordsWithNoPickOpenStandAlone: words from a creature whose
// pick the reader was not shown stand as its answer, with nothing more.
func (s *ActFoldSuite) TestACreaturesWordsWithNoPickOpenStandAlone() {
	acts := foldActs("sess", "alice", []record.Entry{s.beat("time:one", speechVersion(map[string]any{"speaker": "guard", "words": "Halt."}))})
	s.Require().Len(acts, 1)
	s.Equal(TableAnswer{Who: member("guard"), Say: "Halt."}, acts[0].Body)
}

// TestAnEntitledReaderIsToldWhatThePickNoLongerCarries (rpg-project#933): a
// reader who holds the line and catches the creature's words is told the
// pick's option, its table (how the line went) and that it taught a fact;
// one who caught the answer's words but not the line is told the fact and
// not the line; one who caught no words is told none of the three.
func (s *ActFoldSuite) TestAnEntitledReaderIsToldWhatThePickNoLongerCarries() {
	spoke := s.beat("speak:one", map[string]any{
		"beat": encounter.BeatSpoke, "actor": "alice", "target": "goblin", "option": "threaten", "line": "Drop it.",
		"skill": "intimidation", "dc": 12, "total": 4, "beaten": false,
	})
	answered := s.beat("speak:one", map[string]any{"beat": encounter.BeatAnswered, "creature": "goblin", "say": "", "to": "", "word": ""})
	reply := s.beat("speak:one", speechVersion(map[string]any{"speaker": "goblin", "to": "alice", "words": "Thief!", "fact": "caught"}))

	s.Run("the line held and the words caught", func() {
		acts := foldActs("sess", "alice", []record.Entry{spoke, answered, reply})
		s.Require().Len(acts, 1)
		answer := acts[0].Body.(SpeakAct).Answer
		s.Require().NotNil(answer)
		s.Equal("threaten", answer.Option, "the line it answered")
		s.Equal(string(encounter.AnswerFailure), answer.Key, "the missed line's table")
		s.Equal("fact", answer.Word, "and that it taught a fact")
		s.Equal("Thief!", answer.Say)
	})
	s.Run("the words caught, not the line", func() {
		acts := foldActs("sess", "bob", []record.Entry{answered, reply})
		s.Require().Len(acts, 1)
		answer := acts[0].Body.(TableAnswer)
		s.Equal("Thief!", answer.Say)
		s.Equal("fact", answer.Word, "the fact is in the words he caught")
		s.Empty(answer.Option, "but not which line it answered")
		s.Empty(answer.Key, "nor how that went")
	})
	s.Run("no words caught", func() {
		seen := s.beat("speak:one", speechVersion(map[string]any{"speaker": "goblin"}))
		acts := foldActs("sess", "cole", []record.Entry{answered, seen})
		s.Require().Len(acts, 1)
		answer := acts[0].Body.(TableAnswer)
		s.Empty(answer.Say)
		s.Empty(answer.Option)
		s.Empty(answer.Key)
		s.Empty(answer.Word)
	})
	s.Run("a plain line's answer is its success table", func() {
		plain := s.beat("speak:two", map[string]any{"beat": encounter.BeatSpoke, "actor": "alice", "target": "goblin", "option": "greet", "line": "Evening."})
		picked := s.beat("speak:two", map[string]any{"beat": encounter.BeatAnswered, "creature": "goblin", "say": "", "to": ""})
		words := s.beat("speak:two", speechVersion(map[string]any{"speaker": "goblin", "to": "alice", "words": "Evening."}))
		acts := foldActs("sess", "alice", []record.Entry{plain, picked, words})
		answer := acts[0].Body.(SpeakAct).Answer
		s.Equal(string(encounter.AnswerSuccess), answer.Key)
		s.Equal("greet", answer.Option)
		s.Empty(answer.Word, "it taught nothing")
	})
}
