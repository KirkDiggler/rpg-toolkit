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

// TestALineAndItsAnswerAreSpeechActsAndAPick: the speaker of a line is sent
// her own Speech act and nothing from the `spoke` record; the creature's
// answer is its pick, a table answer under the line's tag, and its words are
// his Speech act (rpg-project#934). Record order holds.
func (s *ActFoldSuite) TestALineAndItsAnswerAreSpeechActsAndAPick() {
	spoke := s.beat("speak:one", map[string]any{"beat": encounter.BeatSpoke, "actor": "alice", "target": "goblin",
		"option": "hello", "line": "Evening.", "skill": "persuasion", "dc": 10, "total": 15, "beaten": true})
	line := s.beat("speak:one", speechVersion(map[string]any{"speaker": "alice", "to": "goblin", "words": "Evening.", "option": "hello",
		"check": map[string]any{"skill": "persuasion", "dc": 10, "total": 15, "beaten": true}}))
	answered := s.beat("speak:one", map[string]any{"beat": encounter.BeatAnswered, "creature": "goblin", "word": "",
		"roll": 3, "of": 100, "entry": 0, "candidates": []any{map[string]any{"entry": 0, "weight": 1, "percent": 100, "loaded": 100}}})
	reply := s.beat("speak:one", speechVersion(map[string]any{"speaker": "goblin", "to": "alice", "words": "Evening yourself."}))
	acts := foldActs("sess", "alice", []record.Entry{spoke, line, answered, reply})

	var kinds []string
	for _, a := range acts {
		kinds = append(kinds, fmt.Sprintf("%T", a.Body))
	}
	s.Equal([]string{"session.SpeechAct", "session.TableAnswer", "session.SpeechAct"}, kinds,
		"her line, his pick, his words: the spoke record folds to nothing")
	own := acts[0].Body.(SpeechAct)
	s.Equal(member("alice"), own.Speaker)
	s.Require().NotNil(own.Roll, "her roll rides her own Speech act")
	s.Equal(15, own.Roll.Total)
	pick := acts[1].Body.(TableAnswer)
	s.Equal(member("goblin"), pick.Who)
	s.Require().NotNil(pick.Roll, "the pick keeps its die")
	s.Len(pick.Candidates, 1, "and its arithmetic")
	s.Equal(FamilyTableAnswer, acts[1].Family)
	s.Equal("speak:one", acts[1].Act, "under the line's tag")
	for i := 1; i < len(acts); i++ {
		s.Less(acts[i-1].ID, acts[i].ID, "record order")
	}
}

// TestAPickTellsNoReaderMoreThanTheFoldedFormDid: the line's answer standing
// on its own carries what the encounter wrote on it and nothing from any
// version: a reader who caught no words is told no words, no line, no table
// key and no fact by it (rpg-project#933, #934).
func (s *ActFoldSuite) TestAPickTellsNoReaderMoreThanTheFoldedFormDid() {
	answered := s.beat("speak:one", map[string]any{"beat": encounter.BeatAnswered, "creature": "goblin", "word": "",
		"roll": 3, "of": 100, "entry": 0})
	seen := s.beat("speak:one", speechVersion(map[string]any{"speaker": "goblin"}))
	acts := foldActs("sess", "bea", []record.Entry{answered, seen})
	s.Require().Len(acts, 2)
	pick := acts[0].Body.(TableAnswer)
	s.Empty(pick.Option, "no line")
	s.Empty(pick.Key, "no table that would say how it went")
	s.Empty(pick.Word, "no fact word")
	speech := acts[1].Body.(SpeechAct)
	s.Nil(speech.Words, "and no words")
	s.Empty(speech.Fact)
}

// TestARecordFromBeforeStillFolds: a record written while the retired
// messages were sent — a `spoke` beat, a speech version and an answered beat
// still naming `say` and `to` — decodes and folds: the speaker's Speech act,
// and the pick with no words and no addressee.
func (s *ActFoldSuite) TestARecordFromBeforeStillFolds() {
	spoke := s.beat("speak:one", map[string]any{"beat": encounter.BeatSpoke, "actor": "alice", "target": "goblin",
		"option": "hello", "line": "Evening."})
	line := s.beat("speak:one", speechVersion(map[string]any{"speaker": "alice", "to": "goblin", "words": "Evening.", "option": "hello"}))
	old := s.beat("speak:one", map[string]any{"beat": encounter.BeatAnswered, "creature": "goblin", "say": "Evening yourself.",
		"to": "alice", "option": "hello", "key": "success", "word": ""})
	acts := foldActs("sess", "alice", []record.Entry{spoke, line, old})
	s.Require().Len(acts, 2)
	for _, a := range acts {
		s.Empty(a.Undecoded, "every entry decodes")
	}
	s.IsType(SpeechAct{}, acts[0].Body)
	pick := acts[1].Body.(TableAnswer)
	s.Equal(member("goblin"), pick.Who)
	s.Equal("success", pick.Key, "what the old record said of the pick, as written")
}

// TestAClangIsStillASound: a heard clang reaches a player as a sound, as
// every sound but a voice always has.
func (s *ActFoldSuite) TestAClangIsStillASound() {
	acts := foldActs("sess", "alice", []record.Entry{s.beat("", map[string]any{
		"beat": encounter.BeatHeard, "kind": encounter.SoundClang, "position": map[string]any{"x": 3, "y": 4}, "hops": 1,
	})})
	s.Require().Len(acts, 1)
	p, ok := acts[0].Body.(Perception)
	s.Require().True(ok, "a %T", acts[0].Body)
	s.Equal(SenseHearing, p.Sense)
	s.Equal(&Sound{Kind: encounter.SoundClang, Cell: cell(3, 4), Hops: 1}, p.Heard)
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
