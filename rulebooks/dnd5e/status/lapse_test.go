// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package status_test

import (
	"encoding/json"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/status"
)

const heldStatus = `{"name":"Held",
 "changes":[{"cannot":["move"]}],
 "maintenance":[{"ends_on":"run"}],
 "again":"ignored"}`

type libraryDoc struct {
	Definitions map[string]struct {
		Maintenance []struct{ Key, Part string } `json:"maintenance"`
	} `json:"definitions"`
	Parts map[string]struct {
		Kind string `json:"kind"`
	} `json:"parts"`
}

// Rounds of world time pass only when someone acts or waits (a Wait act is its
// own story, rpg-project#1079); these tests only compile, none counts rounds.
func (s *statusSuite) TestCompileWithUntilAddsALapsePart() {
	out, err := status.Compile(&status.CompileInput{ID: "held", Status: json.RawMessage(heldStatus), Until: &status.Until{Rounds: 3}})
	s.Require().NoError(err)
	var doc libraryDoc
	s.Require().NoError(json.Unmarshal(out.Library, &doc))
	var kinds []string
	for _, m := range doc.Definitions["held"].Maintenance {
		kinds = append(kinds, doc.Parts[m.Part].Kind)
	}
	s.ElementsMatch([]string{"end_on", "lapse"}, kinds)
}

func (s *statusSuite) TestCompileWithoutUntilIsUnchanged() {
	without, err := status.Compile(&status.CompileInput{ID: "held", Status: json.RawMessage(heldStatus)})
	s.Require().NoError(err)
	s.NotContains(string(without.Library), "lapse")
	explicit, err := status.Compile(&status.CompileInput{ID: "held", Status: json.RawMessage(heldStatus), Until: nil})
	s.Require().NoError(err)
	s.Equal(string(without.Library), string(explicit.Library))
}

func (s *statusSuite) TestCompileRefusesUntilOutsideItsBounds() {
	for _, rounds := range []int{0, -1, 1001} {
		out, err := status.Compile(&status.CompileInput{ID: "held", Status: json.RawMessage(heldStatus), Until: &status.Until{Rounds: rounds}})
		s.Nil(out)
		s.Require().Error(err)
		s.True(errors.Is(err, status.ErrBadStatus))
		s.Contains(err.Error(), "until.rounds must be a whole number of one or more, at most 1000")
	}
	for _, rounds := range []int{1, 1000} {
		_, err := status.Compile(&status.CompileInput{ID: "held", Status: json.RawMessage(heldStatus), Until: &status.Until{Rounds: rounds}})
		s.NoError(err)
	}
}

func (s *statusSuite) TestAStatusWithOnlyALapseStillNeedsItsOwnMaintenance() {
	bare := `{"name":"Held","changes":[{"cannot":["move"]}],"again":"ignored"}`
	_, today := status.Compile(&status.CompileInput{ID: "held", Status: json.RawMessage(bare)})
	s.Require().Error(today)
	_, err := status.Compile(&status.CompileInput{ID: "held", Status: json.RawMessage(bare), Until: &status.Until{Rounds: 3}})
	s.Require().Error(err)
	s.Equal(today.Error(), err.Error())
	s.Contains(err.Error(), "the status has no maintenance")
}
