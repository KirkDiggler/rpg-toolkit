// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
)

// The guard probe: a frame that is a map of fact values, where a missing
// fact is unknown, and one beat that carries a round.

type probeFrame map[machine.FactName]any

type probeTick struct{ Round int }

const (
	beatTick machine.Beat = "tick"

	factA      machine.FactName = "a"
	factB      machine.FactName = "b"
	factLevel  machine.FactName = "level"
	factAbil   machine.FactName = "ability"
	factClass  machine.FactName = "class.barbarian"
	flagA                       = "flag_a"
	abilityStr                  = "str"
)

func probeVocabulary() machine.Vocabulary {
	read := func(name machine.FactName) func(any) (any, bool) {
		return func(frame any) (any, bool) {
			f, ok := frame.(probeFrame)
			if !ok {
				return nil, false
			}
			v, ok := f[name]
			return v, ok
		}
	}
	return machine.Vocabulary{
		Facts: map[machine.FactName]machine.FactSpec{
			"a":       {Kind: machine.KindBool, Read: read(factA)},
			"b":       {Kind: machine.KindBool, Read: read(factB)},
			factLevel: {Kind: machine.KindInt, Read: read(factLevel)},
			factAbil:  {Kind: machine.KindString, Read: read(factAbil)},
			factClass: {Kind: machine.KindInt, Read: read(factClass)},
		},
		Beats: map[machine.Beat]machine.BeatSpec{
			beatTick: {
				Selectors: []machine.Selector{selSubject},
				Subscribe: func(context.Context, events.EventBus, func(context.Context, any) error) (string, error) {
					return "", nil // the guard tests never attach
				},
				About: func(any, machine.Selector) (string, bool) { return "holder", true },
				Round: func(ev any) (int, bool) {
					t, ok := ev.(probeTick)
					return t.Round, ok && t.Round > 0
				},
			},
		},
	}
}

func probeProgram(s *suite.Suite) *machine.Program {
	r := machine.Recipe{
		Ref:     core.Ref{Module: "test", Type: "conditions", ID: "probe"},
		Initial: "on",
		Bag:     machine.Bag{flagA: false, machine.KeyRoundActivated: 0},
		States: map[string]machine.State{
			"on": {Transitions: []machine.Transition{
				{On: beatTick, About: selSubject, When: &machine.Guard{Flag: flagA}},
				{On: beatTick, About: selSubject, When: &machine.Guard{Rounds: &machine.RoundsLeaf{WentBackwards: true}}},
			}},
		},
	}
	p, err := r.Compile(probeVocabulary())
	s.Require().NoError(err)
	return p
}

const (
	A = machine.Applies
	N = machine.DoesNotApply
	D = machine.Depends
)

// leafFor returns a fact leaf over name and the frame value that makes it
// answer v.
func leafFor(name machine.FactName, v machine.Verdict) (machine.Guard, any, bool) {
	g := machine.Guard{Fact: &machine.FactLeaf{Name: name, Op: machine.OpEq, Value: true}}
	switch v {
	case machine.Applies:
		return g, true, true
	case machine.DoesNotApply:
		return g, false, true
	default:
		return g, nil, false
	}
}

type guardTruthSuite struct {
	suite.Suite
	p *machine.Program
}

func (s *guardTruthSuite) SetupTest() { s.p = probeProgram(&s.Suite) }

func (s *guardTruthSuite) answer(g machine.Guard, frame probeFrame) machine.Verdict {
	cg, err := s.p.CompileGuard(&g, "")
	s.Require().NoError(err)
	v, _ := cg.Answer(frame, nil, nil)
	return v
}

func (s *guardTruthSuite) TestAllAnyOverTwoLeaves() {
	verdicts := []machine.Verdict{A, N, D}
	// Written out by hand rather than derived, so the table is the spec.
	wantAll := map[[2]machine.Verdict]machine.Verdict{
		{A, A}: A, {A, N}: N, {A, D}: D,
		{N, A}: N, {N, N}: N, {N, D}: N,
		{D, A}: D, {D, N}: N, {D, D}: D,
	}
	wantAny := map[[2]machine.Verdict]machine.Verdict{
		{A, A}: A, {A, N}: A, {A, D}: A,
		{N, A}: A, {N, N}: N, {N, D}: D,
		{D, A}: A, {D, N}: D, {D, D}: D,
	}
	for _, x := range verdicts {
		for _, y := range verdicts {
			ga, va, ka := leafFor(factA, x)
			gb, vb, kb := leafFor(factB, y)
			frame := probeFrame{}
			if ka {
				frame["a"] = va
			}
			if kb {
				frame["b"] = vb
			}
			s.Equal(wantAll[[2]machine.Verdict{x, y}], s.answer(machine.Guard{All: []machine.Guard{ga, gb}}, frame),
				"all(%s, %s)", x, y)
			s.Equal(wantAny[[2]machine.Verdict{x, y}], s.answer(machine.Guard{Any: []machine.Guard{ga, gb}}, frame),
				"any(%s, %s)", x, y)
		}
	}
}

func (s *guardTruthSuite) TestNotSwapsAndLeavesDepends() {
	want := map[machine.Verdict]machine.Verdict{A: N, N: A, D: D}
	for in, out := range want {
		g, v, known := leafFor(factA, in)
		frame := probeFrame{}
		if known {
			frame["a"] = v
		}
		s.Equal(out, s.answer(machine.Guard{Not: &g}, frame), "not(%s)", in)
	}
}

func (s *guardTruthSuite) TestReasonIsTheLeafThatDecided() {
	ga, _, _ := leafFor(factA, A)
	gb, _, _ := leafFor(factB, N)
	cg, err := s.p.CompileGuard(&machine.Guard{All: []machine.Guard{ga, gb}}, "")
	s.Require().NoError(err)
	v, reason := cg.Answer(probeFrame{factA: true, factB: false}, nil, nil)
	s.Equal(N, v)
	s.Equal("fact b", reason)

	v, reason = cg.Answer(probeFrame{factB: true}, nil, nil)
	s.Equal(D, v)
	s.Equal("fact a", reason, "the unknown fact is named")
}

func (s *guardTruthSuite) TestNilGuardAlwaysApplies() {
	cg, err := s.p.CompileGuard(nil, "")
	s.Require().NoError(err)
	v, _ := cg.Answer(nil, nil, nil)
	s.Equal(A, v)
}

func (s *guardTruthSuite) TestFlagReadsTheBag() {
	cg, err := s.p.CompileGuard(&machine.Guard{Flag: flagA}, "")
	s.Require().NoError(err)
	v, reason := cg.Answer(nil, machine.Bag{flagA: true}, nil)
	s.Equal(A, v)
	s.Equal("flag flag_a", reason)
	v, _ = cg.Answer(nil, machine.Bag{flagA: false}, nil)
	s.Equal(N, v)
}

func TestGuardTruthTable(t *testing.T) {
	suite.Run(t, new(guardTruthSuite))
}

type guardOpsSuite struct {
	suite.Suite
	p *machine.Program
}

func (s *guardOpsSuite) SetupTest() { s.p = probeProgram(&s.Suite) }

func (s *guardOpsSuite) verdict(g machine.Guard, frame probeFrame) machine.Verdict {
	cg, err := s.p.CompileGuard(&g, "")
	s.Require().NoError(err)
	v, _ := cg.Answer(frame, nil, nil)
	return v
}

func (s *guardOpsSuite) TestEqOnIntAndString() {
	lvl := machine.Guard{Fact: &machine.FactLeaf{Name: factLevel, Op: machine.OpEq, Value: 3}}
	s.Equal(A, s.verdict(lvl, probeFrame{factLevel: 3}))
	s.Equal(N, s.verdict(lvl, probeFrame{factLevel: 4}))
	s.Equal(D, s.verdict(lvl, probeFrame{}))

	str := machine.Guard{Fact: &machine.FactLeaf{Name: factAbil, Op: machine.OpEq, Value: abilityStr}}
	s.Equal(A, s.verdict(str, probeFrame{factAbil: abilityStr}))
	s.Equal(N, s.verdict(str, probeFrame{factAbil: "dex"}))
	s.Equal(D, s.verdict(str, probeFrame{}))
}

func (s *guardOpsSuite) TestGteOnInt() {
	g := machine.Guard{Fact: &machine.FactLeaf{Name: factLevel, Op: machine.OpGte, Value: 9}}
	s.Equal(N, s.verdict(g, probeFrame{factLevel: 8}))
	s.Equal(A, s.verdict(g, probeFrame{factLevel: 9}))
	s.Equal(A, s.verdict(g, probeFrame{factLevel: 16}))
}

// gte has no meaning on a string fact; the recipe is refused at load rather
// than answering by lexical order.
func (s *guardOpsSuite) TestGteOnStringIsRefused() {
	g := machine.Guard{Fact: &machine.FactLeaf{Name: factAbil, Op: machine.OpGte, Value: abilityStr}}
	_, err := s.p.CompileGuard(&g, "")
	s.Require().ErrorIs(err, machine.ErrInvalidRecipe)
	s.Contains(err.Error(), `op "gte" does not apply to a string fact`)
}

// A reader that reports known with the wrong Go type is the plug's bug; the
// leaf answers Depends rather than guessing.
func (s *guardOpsSuite) TestAKnownValueOfTheWrongTypeDepends() {
	g := machine.Guard{Fact: &machine.FactLeaf{Name: factLevel, Op: machine.OpGte, Value: 1}}
	s.Equal(D, s.verdict(g, probeFrame{factLevel: "high"}))
}

func (s *guardOpsSuite) TestClassLevel() {
	g := machine.Guard{ClassLevel: &machine.ClassLevelLeaf{Class: string(factClass), Op: machine.OpGte, N: 9}}
	s.Equal(A, s.verdict(g, probeFrame{factClass: 9}))
	s.Equal(N, s.verdict(g, probeFrame{factClass: 5}))
	s.Equal(D, s.verdict(g, probeFrame{}), "a frame that does not carry the class depends")

	wizard := machine.Guard{ClassLevel: &machine.ClassLevelLeaf{Class: "class.wizard", Op: machine.OpGte, N: 1}}
	_, err := s.p.CompileGuard(&wizard, "")
	s.Require().ErrorIs(err, machine.ErrInvalidRecipe)
	s.Contains(err.Error(), `class "class.wizard" is not an int fact in the vocabulary`)
}

func TestGuardOps(t *testing.T) {
	suite.Run(t, new(guardOpsSuite))
}

type roundsLeafSuite struct {
	suite.Suite
	p *machine.Program
}

func (s *roundsLeafSuite) SetupTest() { s.p = probeProgram(&s.Suite) }

func (s *roundsLeafSuite) answer(leaf machine.RoundsLeaf, anchor, round int) machine.Verdict {
	cg, err := s.p.CompileGuard(&machine.Guard{Rounds: &leaf}, beatTick)
	s.Require().NoError(err)
	v, _ := cg.Answer(nil, machine.Bag{machine.KeyRoundActivated: anchor}, probeTick{Round: round})
	return v
}

func (s *roundsLeafSuite) TestSinceAnchor() {
	cap9 := machine.RoundsLeaf{SinceAnchor: &machine.IntOp{Op: machine.OpGte, N: 9}}
	s.Equal(D, s.answer(cap9, 0, 5), "anchor 0: nothing to measure from")
	s.Equal(D, s.answer(cap9, 1, 0), "event round 0: the beat carries no round")
	s.Equal(N, s.answer(cap9, 1, 9), "round 9 is eight rounds after anchor 1")
	s.Equal(A, s.answer(cap9, 1, 10), "round 10 is nine rounds after anchor 1")
}

func (s *roundsLeafSuite) TestWentBackwards() {
	back := machine.RoundsLeaf{WentBackwards: true}
	s.Equal(A, s.answer(back, 4, 1), "round below the anchor")
	s.Equal(N, s.answer(back, 4, 4))
	s.Equal(N, s.answer(back, 4, 7))
	s.Equal(D, s.answer(back, 0, 1), "anchor 0")
	s.Equal(D, s.answer(back, 4, 0), "event round 0")
}

func (s *roundsLeafSuite) TestRoundsOutsideABeatIsRefused() {
	_, err := s.p.CompileGuard(&machine.Guard{Rounds: &machine.RoundsLeaf{WentBackwards: true}}, "")
	s.Require().ErrorIs(err, machine.ErrInvalidRecipe)
	s.Contains(err.Error(), "a rounds leaf needs a beat")
}

func (s *roundsLeafSuite) TestRoundsLeafSetsExactlyOneField() {
	_, err := s.p.CompileGuard(&machine.Guard{Rounds: &machine.RoundsLeaf{}}, beatTick)
	s.Require().ErrorIs(err, machine.ErrInvalidRecipe)
	s.Contains(err.Error(), "rounds leaf sets 0 fields, want exactly 1")
}

func TestRoundsLeaf(t *testing.T) {
	suite.Run(t, new(roundsLeafSuite))
}
