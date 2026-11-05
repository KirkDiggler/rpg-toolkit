// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// This is an executable design probe for rpg-project#859, not keep content.
// The caller supplies sound kind and report relevance through its vocabulary;
// the live encounter does not yet deliver this vocabulary. Information changes
// the fixture's state, while explicit time inputs request a table pick. That
// is authored fixture policy, not a new rule about encounter scheduling.
//
// A Pick is only the name of behavior to select, not an executed action or
// a rolled result. The machine has no die; selection and its controlled-roll
// tests belong to the consumer. This probe establishes neither simultaneous
// roles nor perception routing, memory policy, dialogue or operation recovery.
const (
	investigationIntel  machine.Beat     = "intel"
	investigationKind   machine.FactName = "heard.kind"
	investigationReport machine.FactName = "report.party_escaped"
)

type guardInvestigationSuite struct {
	suite.Suite
	program *machine.Program
}

func (s *guardInvestigationSuite) SetupTest() {
	program, err := investigationRecipe().Compile(investigationVocabulary())
	s.Require().NoError(err)
	s.program = program
}

// TestGuardInvestigationSuite exercises a supplied guard recipe against the
// real machine, with explicit inputs, literal expected picks and no randomness.
func TestGuardInvestigationSuite(t *testing.T) {
	suite.Run(t, new(guardInvestigationSuite))
}

func investigationRecipe() machine.Recipe {
	return machine.Recipe{
		Ref:     testRef("guard-investigation"),
		Initial: "patrolling",
		States: map[string]machine.State{
			"patrolling": {Transitions: []machine.Transition{
				{On: beatTime, Do: []machine.Act{{Pick: "patrol"}}},
				{
					On: beatHeard,
					When: &machine.Guard{All: []machine.Guard{
						{Scope: &machine.ScopeLeaf{Within: 6}},
						{Fact: &machine.FactLeaf{Name: investigationKind, Op: machine.OpEq, Value: "clang"}},
					}},
					To:     "searching",
					Reason: "heard a nearby clang",
				},
			}},
			"searching": {Transitions: []machine.Transition{
				{On: beatTime, Do: []machine.Act{{Pick: "search"}}},
				{
					On: investigationIntel,
					When: &machine.Guard{Fact: &machine.FactLeaf{
						Name: investigationReport, Op: machine.OpEq, Value: true,
					}},
					To:     "questioning",
					Reason: "received an escaped-party report",
				},
			}},
			"questioning": {Transitions: []machine.Transition{
				{On: beatTime, Do: []machine.Act{{Pick: "challenge"}}},
			}},
		},
	}
}

func investigationVocabulary() machine.Vocabulary {
	v := testVocabulary()
	v.Beats[investigationIntel] = machine.BeatSpec{}
	v.Facts[investigationKind] = machine.FactSpec{
		Kind: machine.KindString, Read: investigationFact(investigationKind),
	}
	v.Facts[investigationReport] = machine.FactSpec{
		Kind: machine.KindBool, Read: investigationFact(investigationReport),
	}
	return v
}

// investigationFact reads only the facts this fixture's caller supplied.
// Absence stays unknown; this is not a report store or a truth lookup.
func investigationFact(name machine.FactName) func(any) (any, bool) {
	return func(value any) (any, bool) {
		facts, ok := value.(env)
		if !ok {
			return nil, false
		}
		fact, known := facts[name]
		return fact, known
	}
}

// A missing state write must fail at the next selected behavior, even if the
// transition result still claims it entered the right state.
func (s *guardInvestigationSuite) TestClangThenIntelChangesSelectedBehavior() {
	blob := s.program.NewBlob()
	step, err := s.program.Step(&blob, beatTime, tick{Round: 10}, nil, 10)
	s.Require().NoError(err)
	s.Assert().Equal([]machine.Act{{Pick: "patrol"}}, step.Acts)

	step, err = s.program.Step(&blob, beatHeard, sound{Hexes: 3}, env{investigationKind: "clang"}, 10)
	s.Require().NoError(err)
	s.Assert().True(step.Fired)
	s.Assert().Equal("patrolling", step.From)
	s.Assert().Equal("searching", step.To)
	s.Assert().Equal("heard a nearby clang", step.Reason)
	s.Assert().Empty(step.Acts, "this recipe's information input requests no table execution")
	s.Assert().Equal([]machine.Listen{{Beat: investigationIntel}}, s.program.Listens(&blob))

	step, err = s.program.Step(&blob, beatTime, tick{Round: 11}, nil, 11)
	s.Require().NoError(err)
	s.Assert().Equal([]machine.Act{{Pick: "search"}}, step.Acts)

	step, err = s.program.Step(&blob, investigationIntel, nil, env{investigationReport: true}, 11)
	s.Require().NoError(err)
	s.Assert().True(step.Fired)
	s.Assert().Equal("searching", step.From)
	s.Assert().Equal("questioning", step.To)
	s.Assert().Equal("received an escaped-party report", step.Reason)
	s.Assert().Empty(step.Acts, "receiving the report does not execute a challenge")
	s.Assert().Empty(s.program.Listens(&blob), "this authored state only listens for time")

	step, err = s.program.Step(&blob, beatTime, tick{Round: 12}, nil, 12)
	s.Require().NoError(err)
	s.Assert().Equal([]machine.Act{{Pick: "challenge"}}, step.Acts)
}

// Ignoring a known-false kind or scope guard would wrongly select search;
// interpreting a missing kind as a match would do the same.
func (s *guardInvestigationSuite) TestSoundMustMatchKindAndScope() {
	cases := []struct {
		name     string
		hexes    int
		facts    env
		fired    bool
		wantPick string
	}{
		{"clang at the boundary", 6, env{investigationKind: "clang"}, true, "search"},
		{"clang beyond the boundary", 7, env{investigationKind: "clang"}, false, "patrol"},
		{"nearby voice", 3, env{investigationKind: "voice"}, false, "patrol"},
		{"unknown kind", 3, env{}, false, "patrol"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			blob := s.program.NewBlob()
			step, err := s.program.Step(&blob, beatHeard, sound{Hexes: tc.hexes}, tc.facts, 1)
			s.Require().NoError(err)
			s.Assert().Equal(tc.fired, step.Fired)
			s.Assert().Empty(step.Acts)

			step, err = s.program.Step(&blob, beatTime, tick{Round: 2}, nil, 2)
			s.Require().NoError(err)
			s.Assert().Equal([]machine.Act{{Pick: tc.wantPick}}, step.Acts)
		})
	}
}

// Known false and unknown are both non-firing, but unknown remains Depends.
// A change that accepts DoesNotApply must fail on the nonmatching report.
func (s *guardInvestigationSuite) TestOnlyKnownMatchingReportChangesSearch() {
	cases := []struct {
		name     string
		facts    env
		fired    bool
		depends  bool
		wantPick string
	}{
		{"matching report", env{investigationReport: true}, true, false, "challenge"},
		{"nonmatching report", env{investigationReport: false}, false, false, "search"},
		{"unknown report", env{}, false, true, "search"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			blob := s.program.NewBlob()
			step, err := s.program.Step(&blob, beatHeard, sound{Hexes: 3}, env{investigationKind: "clang"}, 1)
			s.Require().NoError(err)
			s.Require().True(step.Fired)

			step, err = s.program.Step(&blob, investigationIntel, nil, tc.facts, 1)
			s.Require().NoError(err)
			s.Assert().Equal(tc.fired, step.Fired)
			s.Assert().Empty(step.Acts)
			if tc.depends {
				s.Assert().Contains(step.Reason, "depends")
			}

			step, err = s.program.Step(&blob, beatTime, tick{Round: 2}, nil, 2)
			s.Require().NoError(err)
			s.Assert().Equal([]machine.Act{{Pick: tc.wantPick}}, step.Acts)
		})
	}
}

// Losing the saved state would resume patrol, and losing its listener would
// prevent the later intel from changing the resumed behavior.
func (s *guardInvestigationSuite) TestReloadContinuesSearchAndAcceptsAReport() {
	blob := s.program.NewBlob()
	_, err := s.program.Step(&blob, beatHeard, sound{Hexes: 3}, env{investigationKind: "clang"}, 1)
	s.Require().NoError(err)

	data, err := json.Marshal(blob)
	s.Require().NoError(err)
	var restored machine.Blob
	s.Require().NoError(json.Unmarshal(data, &restored))
	s.Assert().Equal([]machine.Listen{{Beat: investigationIntel}}, s.program.Listens(&restored))

	step, err := s.program.Step(&restored, beatTime, tick{Round: 2}, nil, 2)
	s.Require().NoError(err)
	s.Assert().Equal([]machine.Act{{Pick: "search"}}, step.Acts)

	_, err = s.program.Step(&restored, investigationIntel, nil, env{investigationReport: true}, 2)
	s.Require().NoError(err)
	step, err = s.program.Step(&restored, beatTime, tick{Round: 3}, nil, 3)
	s.Require().NoError(err)
	s.Assert().Equal([]machine.Act{{Pick: "challenge"}}, step.Acts)
}

// Storing current state on the shared Program instead of the Blob would let
// one guard's input change another guard's behavior.
func (s *guardInvestigationSuite) TestInstancesKeepIndependentBehavior() {
	first, second := s.program.NewBlob(), s.program.NewBlob()
	_, err := s.program.Step(&first, beatHeard, sound{Hexes: 3}, env{investigationKind: "clang"}, 1)
	s.Require().NoError(err)

	step, err := s.program.Step(&second, beatTime, tick{Round: 2}, nil, 2)
	s.Require().NoError(err)
	s.Assert().Equal([]machine.Act{{Pick: "patrol"}}, step.Acts)
	step, err = s.program.Step(&first, beatTime, tick{Round: 2}, nil, 2)
	s.Require().NoError(err)
	s.Assert().Equal([]machine.Act{{Pick: "search"}}, step.Acts)
}
