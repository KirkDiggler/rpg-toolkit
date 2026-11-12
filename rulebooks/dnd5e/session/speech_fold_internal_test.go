// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"
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
	acts := retiredActs(foldActs("sess", "bob", []record.Entry{s.beat("speak:one", speechVersion(map[string]any{
		"speaker": "alice", "to": "goblin", "words": "Evening.", "option": "hello",
		"check": map[string]any{"skill": "persuasion", "dc": 10, "total": 15, "beaten": true},
	}))}))
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
	acts := retiredActs(foldActs("sess", "bob", []record.Entry{s.beat("speak:one", speechVersion(map[string]any{
		"speaker": "alice", "words": "Evening.", "option": "hello",
	}))}))
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
	s.Empty(retiredActs(foldActs("sess", "alice", []record.Entry{own})), "her own line is her spoke record's")
	s.Empty(retiredActs(foldActs("sess", "bob", []record.Entry{seen})), "bob saw her speak and learns no line")
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
			acts := retiredActs(foldActs("sess", "dana", []record.Entry{s.beat("speak:one", speechVersion(fields))}))
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
	acts := retiredActs(foldActs("sess", "alice", []record.Entry{pick, said}))
	s.Require().Len(acts, 1, "one table answer, its words folded in")
	answer := acts[0].Body.(TableAnswer)
	s.Equal("To arms!", answer.Say)
	s.Equal(member("alice"), answer.To)

	spoke := s.beat("speak:two", map[string]any{"beat": encounter.BeatSpoke, "actor": "alice", "target": "goblin", "option": "hello", "line": "Evening."})
	answered := s.beat("speak:two", map[string]any{"beat": encounter.BeatAnswered, "creature": "goblin", "say": "", "to": ""})
	reply := s.beat("speak:two", speechVersion(map[string]any{"speaker": "goblin", "to": "alice", "words": "Evening yourself."}))
	acts = retiredActs(foldActs("sess", "alice", []record.Entry{spoke, answered, reply}))
	s.Require().Len(acts, 1, "one speak act, the answer and its words folded in")
	line := acts[0].Body.(SpeakAct)
	s.Require().NotNil(line.Answer)
	s.Equal("Evening yourself.", line.Answer.Say)
	s.Equal(member("alice"), line.Answer.To)
}

// TestACreaturesWordsWithNoPickOpenStandAlone: words from a creature whose
// pick the reader was not shown stand as its answer, with nothing more.
func (s *ActFoldSuite) TestACreaturesWordsWithNoPickOpenStandAlone() {
	acts := retiredActs(foldActs("sess", "alice", []record.Entry{s.beat("time:one", speechVersion(map[string]any{"speaker": "guard", "words": "Halt."}))}))
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
		acts := retiredActs(foldActs("sess", "alice", []record.Entry{spoke, answered, reply}))
		s.Require().Len(acts, 1)
		answer := acts[0].Body.(SpeakAct).Answer
		s.Require().NotNil(answer)
		s.Equal("threaten", answer.Option, "the line it answered")
		s.Equal(string(encounter.AnswerFailure), answer.Key, "the missed line's table")
		s.Equal("fact", answer.Word, "and that it taught a fact")
		s.Equal("Thief!", answer.Say)
	})
	s.Run("the words caught, not the line", func() {
		acts := retiredActs(foldActs("sess", "bob", []record.Entry{answered, reply}))
		s.Require().Len(acts, 1)
		answer := acts[0].Body.(TableAnswer)
		s.Equal("Thief!", answer.Say)
		s.Equal("fact", answer.Word, "the fact is in the words he caught")
		s.Empty(answer.Option, "but not which line it answered")
		s.Empty(answer.Key, "nor how that went")
	})
	s.Run("no words caught", func() {
		seen := s.beat("speak:one", speechVersion(map[string]any{"speaker": "goblin"}))
		acts := retiredActs(foldActs("sess", "cole", []record.Entry{answered, seen}))
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
		acts := retiredActs(foldActs("sess", "alice", []record.Entry{plain, picked, words}))
		answer := acts[0].Body.(SpeakAct).Answer
		s.Equal(string(encounter.AnswerSuccess), answer.Key)
		s.Equal("greet", answer.Option)
		s.Empty(answer.Word, "it taught nothing")
	})
}

// retiredActs is every act but the [SpeechAct]s: what the retired messages
// fold into, still sent beside the speech act (rpg-project#934).
func retiredActs(acts []Act) []Act {
	var out []Act
	for _, a := range acts {
		if _, speech := a.Body.(SpeechAct); !speech {
			out = append(out, a)
		}
	}
	return out
}

// speechActsOf is the Speech acts a recipient is sent of the entries.
func speechActsOf(recipient string, entries ...record.Entry) []Act {
	var out []Act
	for _, a := range foldActs("sess", recipient, entries) {
		if _, speech := a.Body.(SpeechAct); speech {
			out = append(out, a)
		}
	}
	return out
}

// TestEveryVersionIsOneSpeechAct is the facet table as the act carries it
// (rpg-project#934, R1–R4, R6): each version of one spoken thing is one
// Speech act, family act, with exactly the parts it holds, and its
// Perceived part names the senses that delivered them.
func (s *ActFoldSuite) TestEveryVersionIsOneSpeechAct() {
	check := map[string]any{"skill": "persuasion", "dc": 10, "total": 15, "beaten": true}
	beaten := true
	roll := &Roll{Total: 15, Against: intp(10), Beaten: &beaten}
	place := cell(4, 2)
	words := func(w string) *string { return &w }
	hops := func(h int) *int { return &h }
	sight, hearing := []Sense{SenseSight}, []Sense{SenseHearing}
	both := []Sense{SenseSight, SenseHearing}
	for _, tc := range []struct {
		name      string
		recipient string
		version   map[string]any
		want      SpeechAct
		senses    []Sense
	}{
		{
			name: "sees and hears", recipient: "bob",
			version: map[string]any{"speaker": "alice", "to": "goblin", "words": "Evening.", "option": "hello", "check": check},
			want: SpeechAct{Utterance: "u1", Speaker: member("alice"), SpokenTo: member("goblin"), Words: words("Evening."),
				Option: "hello", Place: place, Loudness: "speak", Skill: "persuasion", Roll: roll},
			senses: both,
		},
		{
			name: "the speaker themself", recipient: "alice",
			version: map[string]any{"speaker": "alice", "to": "goblin", "words": "Evening.", "option": "hello", "check": check},
			want: SpeechAct{Utterance: "u1", Speaker: member("alice"), SpokenTo: member("goblin"), Words: words("Evening."),
				Option: "hello", Place: place, Loudness: "speak", Skill: "persuasion", Roll: roll},
			senses: both,
		},
		{
			name: "hears, does not see: words and someone's roll from a place", recipient: "dana",
			version: map[string]any{"hops": 0, "words": "Evening.", "option": "hello", "check": check},
			want: SpeechAct{Utterance: "u1", Words: words("Evening."), Option: "hello", Place: place, Hops: hops(0),
				Loudness: "speak", Skill: "persuasion", Roll: roll},
			senses: hearing,
		},
		{
			name: "one room further at a shout, teaching a fact", recipient: "cole",
			version: map[string]any{"hops": 1, "words": "To arms!", "loudness": "shout", "fact": "alarm-raised"},
			want: SpeechAct{Utterance: "u1", Words: words("To arms!"), Place: place, Hops: hops(1), Loudness: "shout",
				Fact: "alarm-raised"},
			senses: hearing,
		},
		{
			name: "beyond the words' reach: a voice", recipient: "ed",
			version: map[string]any{"hops": 2, "loudness": "shout"},
			want:    SpeechAct{Utterance: "u1", Place: place, Hops: hops(2), Loudness: "shout"},
			senses:  hearing,
		},
		{
			name: "a whisper watched, not heard", recipient: "bea",
			version: map[string]any{"speaker": "alice", "to": "goblin", "loudness": "whisper"},
			want:    SpeechAct{Utterance: "u1", Speaker: member("alice"), SpokenTo: member("goblin"), Place: place, Loudness: "whisper"},
			senses:  sight,
		},
		{
			name: "seen, the one spoken to unseen", recipient: "bea",
			version: map[string]any{"speaker": "alice", "words": "Evening."},
			want:    SpeechAct{Utterance: "u1", Speaker: member("alice"), Words: words("Evening."), Place: place, Loudness: "speak"},
			senses:  both,
		},
		{
			name: "a line to a prop rides spoken-to as a prop", recipient: "alice",
			version: map[string]any{"speaker": "alice", "prop": "altar", "words": "I kneel.", "option": "kneel"},
			want: SpeechAct{Utterance: "u1", Speaker: member("alice"), SpokenTo: prop("altar"), Words: words("I kneel."),
				Option: "kneel", Place: place, Loudness: "speak"},
			senses: both,
		},
	} {
		s.Run(tc.name, func() {
			fields := speechVersion(tc.version)
			for _, k := range []string{"loudness", "fact"} {
				if v, ok := tc.version[k]; ok {
					fields[k] = v
				}
			}
			acts := speechActsOf(tc.recipient, s.beat("speak:one", fields))
			s.Require().Len(acts, 1, "one Speech act per version")
			s.Equal(FamilyAct, acts[0].Family, "a Speech act is an act, whoever spoke")
			s.Zero(acts[0].Segment, "beside the act's own body, not a segment of it")
			s.Equal(tc.want, acts[0].Body)
			s.Equal(&Perceived{Senses: tc.senses}, acts[0].Perceived, "via is empty until the party-shared view")
		})
	}
}

// TestASpeechActStandsBesideTheLineWithoutPartingIt: the Speech acts of a
// line and its answer stand beside the speak act the answer still folds
// into, in record order, and acts sharing an id stay adjacent (R28).
func (s *ActFoldSuite) TestASpeechActStandsBesideTheLineWithoutPartingIt() {
	spoke := s.beat("speak:one", map[string]any{"beat": encounter.BeatSpoke, "actor": "alice", "target": "goblin", "option": "hello", "line": "Evening."})
	line := s.beat("speak:one", speechVersion(map[string]any{"speaker": "alice", "to": "goblin", "words": "Evening.", "option": "hello"}))
	answered := s.beat("speak:one", map[string]any{"beat": encounter.BeatAnswered, "creature": "goblin", "say": "", "to": ""})
	reply := s.beat("speak:one", speechVersion(map[string]any{"speaker": "goblin", "to": "alice", "words": "Evening yourself."}))
	acts := foldActs("sess", "alice", []record.Entry{spoke, line, answered, reply})

	var kinds []string
	for _, a := range acts {
		kinds = append(kinds, fmt.Sprintf("%T", a.Body))
	}
	s.Equal([]string{"session.SpeechAct", "session.SpeakAct", "session.SpeechAct"}, kinds,
		"her line's speech act, the speak act, his answer's")
	speak := acts[1].Body.(SpeakAct)
	s.Require().NotNil(speak.Answer, "the answer still folds into the line")
	s.Equal("Evening yourself.", speak.Answer.Say)
	for i := 1; i < len(acts); i++ {
		s.LessOrEqual(acts[i-1].ID, acts[i].ID, "record order")
	}
	s.Equal(acts[1].ID, acts[2].ID, "the speak act and the answer's speech act share the reply's id, side by side")
}

// TestAFactRidesOnlyWithTheWords: the fact a line taught is told to a
// perceiver who caught its words, and to nobody who only saw the speaker or
// only noticed a voice — even were a version to name it without words
// (rpg-project#934).
func (s *ActFoldSuite) TestAFactRidesOnlyWithTheWords() {
	for _, tc := range []struct {
		name    string
		version map[string]any
		fact    string
	}{
		{"caught the words", map[string]any{"speaker": "goblin", "words": "To arms!", "fact": "alarm-raised"}, "alarm-raised"},
		{"only saw the speaker", map[string]any{"speaker": "goblin", "fact": "alarm-raised"}, ""},
		{"only noticed a voice", map[string]any{"hops": 2, "fact": "alarm-raised"}, ""},
	} {
		s.Run(tc.name, func() {
			fields := speechVersion(tc.version)
			fields["fact"] = tc.version["fact"]
			acts := speechActsOf("bob", s.beat("time:one", fields))
			s.Require().Len(acts, 1)
			s.Equal(tc.fact, acts[0].Body.(SpeechAct).Fact)
		})
	}
}

// TestASpeechVersionMissingWhatItOwesIsRefusedLoudly: a version that did not
// see its speaker and names no hops, and one naming a prop spoken to with no
// words, are refused when read (the entry stands undecoded, with its
// reason) rather than read as "in your room" or as a prop guessed at.
func (s *ActFoldSuite) TestASpeechVersionMissingWhatItOwesIsRefusedLoudly() {
	for _, tc := range []struct {
		name    string
		version map[string]any
		says    string
	}{
		{"unseen, no hops", map[string]any{"words": "Evening."}, "names no hops"},
		{"a prop, no words", map[string]any{"speaker": "alice", "prop": "altar"}, "without the words"},
	} {
		s.Run(tc.name, func() {
			acts := foldActs("sess", "bob", []record.Entry{s.beat("speak:one", speechVersion(tc.version))})
			s.Require().Len(acts, 1, "the entry stands in the stream")
			s.Nil(acts[0].Body, "with no body")
			s.Contains(acts[0].Undecoded, tc.says)
		})
	}
}
