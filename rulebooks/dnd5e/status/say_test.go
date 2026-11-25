// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package status_test

import (
	"encoding/json"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/status"
)

// The castle's three statuses, as the-mad-king.yaml writes them.
const (
	castleFeasted  = `{"name":"Feasted","changes":[{"max_hit_points":2}],"when_given":[{"heal":2}],"maintenance":[{"ends_on":"run"}],"again":"ignored"}`
	castleWarmedUp = `{"name":"Warmed up","changes":[{"initiative":1}],"maintenance":[{"ends_on":"run"}],"again":"ignored"}`
	castleHeld     = `{"name":"Held","changes":[{"cannot":["move"]}],"maintenance":[{"ends_on":"fight starts"},{"ends_on":"run"}],"again":"ignored"}`
)

func (s *statusSuite) say(id, doc string) (*status.SayOutput, error) {
	return status.Say(&status.SayInput{ID: id, Status: json.RawMessage(doc)})
}

// THE CASTLE'S STATUSES SAY WHAT THEY DO, in the engine's own labels.
func (s *statusSuite) TestSayTheCastlesStatuses() {
	for _, tc := range []struct {
		id, doc, name, sentence string
		parts                   []status.SaidPart
	}{
		{"feasted", castleFeasted, "Feasted",
			"Feasted: raises the holder's hit point maximum by 2. Heals the holder by 2 when given. It ends when the run ends. A second giving is ignored.",
			[]status.SaidPart{
				{Path: "changes[0]", Text: "raises the holder's hit point maximum by 2"},
				{Path: "when_given[0]", Text: "heals the holder by 2 when given"},
				{Path: "maintenance[0]", Text: "ends when the run ends"},
				{Path: "again", Text: "a second giving is ignored"},
			}},
		{"warmed-up", castleWarmedUp, "Warmed up",
			"Warmed up: adds 1 to the holder's initiative. It ends when the run ends. A second giving is ignored.",
			[]status.SaidPart{
				{Path: "changes[0]", Text: "adds 1 to the holder's initiative"},
				{Path: "maintenance[0]", Text: "ends when the run ends"},
				{Path: "again", Text: "a second giving is ignored"},
			}},
		{"held", castleHeld, "Held",
			"Held: cannot move by its own will. It ends when a fight the holder is in starts, or ends when the run ends. A second giving is ignored.",
			[]status.SaidPart{
				{Path: "changes[0]", Text: "cannot move by its own will"},
				{Path: "maintenance[0]", Text: "ends when a fight the holder is in starts"},
				{Path: "maintenance[1]", Text: "ends when the run ends"},
				{Path: "again", Text: "a second giving is ignored"},
			}},
	} {
		s.Run(tc.id, func() {
			out, err := s.say(tc.id, tc.doc)
			s.Require().NoError(err)
			s.Equal(tc.name, out.Name)
			s.Equal(tc.sentence, out.Sentence)
			s.Equal(tc.parts, out.Parts)
		})
	}
}

// A when AND A LIST OF TWO say through the table too: the change, then each
// when label in word order; a list of values joined with "and".
func (s *statusSuite) TestSayJoinsWhensAndLists() {
	out, err := s.say("t", `{"name":"Charming","changes":[{"advantage":"checks","when":{"ability":"charisma"}},{"damage":-1,"when":{"attack":"melee","ability":"strength"}},{"resistance":["fire","cold"]}],"maintenance":[{"ends_on":"rest"}],"again":"ignored"}`)
	s.Require().NoError(err)
	s.Equal("advantage on checks, only when the ability is charisma", out.Parts[0].Text)
	s.Equal("-1 to the damage the holder deals, only when the ability is strength and only for melee attacks", out.Parts[1].Text)
	s.Equal("resistance to fire damage and resistance to cold damage", out.Parts[2].Text)
	s.Equal("ends at the next rest", out.Parts[3].Text)
}

// EVERY PART IS BUILT FROM THE TABLE'S LABELS: for every document the words
// table yields, each part's text is a label filled with the author's value or
// a value's own label, and the sentence holds every part.
func (s *statusSuite) TestSayUsesOnlyTheTablesLabels() {
	words := status.Words(&status.WordsInput{}).Words
	allowed := map[string]bool{}
	for _, w := range words {
		for _, v := range w.Values {
			allowed[v.Label] = true
		}
		for _, n := range []string{"1", "2", "-1"} {
			allowed[strings.ReplaceAll(w.Label, "{value}", n)] = true
		}
	}
	for name, doc := range documents(words) {
		s.Run(name, func() {
			out, err := s.say("t", doc)
			s.Require().NoError(err, doc)
			s.NotEmpty(out.Parts)
			for _, part := range out.Parts {
				// A part may be several labels: values of a list, whens.
				rest := part.Text
				for _, sep := range []string{", ", " and "} {
					rest = strings.ReplaceAll(rest, sep, "\x00")
				}
				for _, piece := range strings.Split(rest, "\x00") {
					s.True(allowed[piece], "%s: %q is not a label from Words", part.Path, piece)
				}
				// The sentence upper-cases the first letter of a sentence.
				s.True(strings.Contains(out.Sentence, part.Text) ||
					strings.Contains(out.Sentence, strings.ToUpper(part.Text[:1])+part.Text[1:]), "%s: %q is in %q", part.Path, part.Text, out.Sentence)
			}
		})
	}
}

// SAY REFUSES WHAT COMPILE REFUSES, with the same error and the same path.
func (s *statusSuite) TestSayRefusesWhatCompileRefuses() {
	for name, tc := range map[string]struct{ doc, path string }{
		"unknown change word": {strings.Replace(castleHeld, `"cannot":["move"]`, `"fly":1`, 1), "changes[0].fly"},
		"number out of range": {strings.Replace(castleWarmedUp, `"initiative":1`, `"initiative":5000`, 1), "changes[0].initiative"},
		"no maintenance":      {strings.Replace(castleHeld, `[{"ends_on":"fight starts"},{"ends_on":"run"}]`, `[]`, 1), "maintenance"},
	} {
		s.Run(name, func() {
			_, want := status.Compile(&status.CompileInput{ID: "held", Status: json.RawMessage(tc.doc)})
			s.Require().Error(want)
			out, err := s.say("held", tc.doc)
			s.Nil(out)
			s.Require().Error(err)
			s.Equal(want.Error(), err.Error())
			var refusal *status.Refusal
			s.Require().ErrorAs(err, &refusal)
			s.Equal(tc.path, refusal.Path)
			s.ErrorIs(err, status.ErrBadStatus)
		})
	}
}
