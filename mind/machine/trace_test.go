// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

type TraceSuite struct{ suite.Suite }

func TestTraceSuite(t *testing.T) { suite.Run(t, new(TraceSuite)) }

func traceFact(name machine.FactName) machine.Guard {
	return machine.Guard{Fact: &machine.FactLeaf{Name: name, Op: machine.OpEq, Value: true}}
}

func traceRecipe(transitions ...machine.Transition) machine.Recipe {
	return machine.Recipe{
		Ref: testRef("trace"), Initial: "s",
		States: map[string]machine.State{
			"s": {Transitions: transitions}, "done": {Terminal: true},
		},
	}
}

// An eager diagnostic rerun, evaluating the next transition, or treating an
// unknown guard as false breaks the observed reads or their recorded verdicts.
func (s *TraceSuite) TestFirstMatchAndShortCircuitReadOnce() {
	var reads []string
	facts := map[machine.FactName]machine.FactSpec{}
	for _, name := range []machine.FactName{"no", "unknown", "yes", "unread"} {
		facts[name] = machine.FactSpec{Kind: machine.KindBool, Read: func(any) (any, bool) {
			reads = append(reads, string(name))
			s.Require().NotEqual(machine.FactName("unread"), name, "a later transition must not be queried for its trace")
			switch name {
			case "unknown":
				return "withheld", false
			case "yes":
				return true, true
			default:
				return false, true
			}
		}}
	}
	no, unknown, yes, unread := traceFact("no"), traceFact("unknown"), traceFact("yes"), traceFact("unread")
	r := traceRecipe(
		machine.Transition{On: beatBell, When: &unread},
		machine.Transition{On: beatTime, When: &no},
		machine.Transition{On: beatTime, When: &unknown},
		machine.Transition{On: beatTime, When: &yes, To: "done", Reason: "chosen", Do: []machine.Act{{Pick: "winner"}}},
		machine.Transition{On: beatTime, When: &unread},
	)
	p, err := r.Compile(machine.Vocabulary{Facts: facts, Beats: map[machine.Beat]machine.BeatSpec{beatTime: {Timed: true}, beatBell: {}}})
	s.Require().NoError(err)
	b := p.NewBlob()
	out, err := p.TraceStep(&machine.TraceStepInput{Blob: &b, On: beatTime, Now: 7})
	s.Require().NoError(err)
	s.Equal([]string{"no", "unknown", "yes"}, reads)
	s.Equal("done", b.State)
	s.Equal([]machine.Act{{Pick: "winner"}}, out.Step.Acts)
	s.Equal("chosen", out.Step.Reason)
	s.Equal("s", out.Trace.Before)
	s.Equal("done", out.Trace.After)
	s.Equal(beatTime, out.Trace.On)
	s.Equal(uint64(7), out.Trace.Now)
	s.Require().Len(out.Trace.Transitions, 5)
	for i, tr := range out.Trace.Transitions {
		s.Equal(i, tr.Index)
		s.Equal(i == 3, tr.Selected)
		s.Equal(i >= 1 && i <= 3, tr.Evaluated)
	}
	s.Equal(beatBell, out.Trace.Transitions[0].On)
	s.Equal(machine.DoesNotApply, out.Trace.Transitions[1].Guard.Verdict)
	s.Equal(machine.Depends, out.Trace.Transitions[2].Guard.Verdict)
	s.False(out.Trace.Transitions[2].Guard.Known)
	s.Nil(out.Trace.Transitions[2].Guard.Value, "an unknown reader's ignored return is not evidence")
	s.Equal(machine.Applies, out.Trace.Transitions[3].Guard.Verdict)
	s.False(out.Trace.Transitions[4].Guard.Evaluated)
}

func (s *TraceSuite) TestNestedGuardTraceKeepsUnvisitedAndUnknownDistinct() {
	tests := []struct {
		name    string
		guard   machine.Guard
		want    machine.Verdict
		reads   []string
		visited []bool
	}{
		{"all stops on false", machine.Guard{All: []machine.Guard{traceFact("no"), traceFact("unread")}}, machine.DoesNotApply, []string{"no"}, []bool{true, false}},
		{"any stops on true", machine.Guard{Any: []machine.Guard{traceFact("yes"), traceFact("unread")}}, machine.Applies, []string{"yes"}, []bool{true, false}},
		{"not keeps unknown", machine.Guard{Not: ptrGuard(traceFact("unknown"))}, machine.Depends, []string{"unknown"}, []bool{true}},
		{"all checks after unknown", machine.Guard{All: []machine.Guard{traceFact("unknown"), traceFact("no")}}, machine.DoesNotApply, []string{"unknown", "no"}, []bool{true, true}},
		{"any checks after unknown", machine.Guard{Any: []machine.Guard{traceFact("unknown"), traceFact("yes")}}, machine.Applies, []string{"unknown", "yes"}, []bool{true, true}},
		{"all remains unknown", machine.Guard{All: []machine.Guard{traceFact("unknown"), traceFact("yes")}}, machine.Depends, []string{"unknown", "yes"}, []bool{true, true}},
		{"any remains unknown", machine.Guard{Any: []machine.Guard{traceFact("unknown"), traceFact("no")}}, machine.Depends, []string{"unknown", "no"}, []bool{true, true}},
		{"not flips true", machine.Guard{Not: ptrGuard(traceFact("yes"))}, machine.DoesNotApply, []string{"yes"}, []bool{true}},
	}
	for _, tc := range tests {
		s.Run(tc.name, func() {
			var reads []string
			facts := map[machine.FactName]machine.FactSpec{}
			for _, name := range []machine.FactName{"no", "unknown", "yes", "unread"} {
				facts[name] = machine.FactSpec{Kind: machine.KindBool, Read: func(any) (any, bool) {
					reads = append(reads, string(name))
					s.Require().NotEqual(machine.FactName("unread"), name)
					return name == "yes", name != "unknown"
				}}
			}
			p, err := traceRecipe(machine.Transition{On: beatTime, When: &tc.guard}).Compile(machine.Vocabulary{Facts: facts, Beats: map[machine.Beat]machine.BeatSpec{beatTime: {}}})
			s.Require().NoError(err)
			b := p.NewBlob()
			out, err := p.TraceStep(&machine.TraceStepInput{Blob: &b, On: beatTime})
			s.Require().NoError(err)
			s.Equal(tc.reads, reads)
			g := out.Trace.Transitions[0].Guard
			s.True(g.Evaluated)
			s.Equal(tc.want, g.Verdict)
			s.Equal(tc.want == machine.Applies, out.Step.Fired)
			s.Require().Len(g.Children, len(tc.visited))
			for i, visited := range tc.visited {
				s.Equal(visited, g.Children[i].Evaluated)
			}
		})
	}
}

func ptrGuard(g machine.Guard) *machine.Guard { return &g }

func (s *TraceSuite) TestTracedAndOrdinaryStepsMatch() {
	r := machine.Recipe{
		Ref: testRef("parity"), Initial: "s", Bag: machine.Bag{"armed": false},
		States: map[string]machine.State{
			"s":     {Transitions: []machine.Transition{{On: beatHeard, When: &machine.Guard{Scope: &machine.ScopeLeaf{Within: 3}}, To: "alert", Do: []machine.Act{{Set: &machine.SetAct{Key: "armed", Value: true}}, {Anchor: true}, {WakeIn: 2}}}}},
			"alert": {Transitions: []machine.Transition{{On: beatTime, When: &machine.Guard{Rounds: &machine.RoundsLeaf{Op: machine.OpGte, N: 3}}, To: "s", Do: []machine.Act{{Clear: []string{"armed"}}}}}},
		},
	}
	calls := [2]int{}
	programs := [2]*machine.Program{}
	for i := range programs {
		p, err := r.Compile(machine.Vocabulary{Beats: map[machine.Beat]machine.BeatSpec{
			beatHeard: {InScope: func(any, machine.ScopeLeaf) bool { calls[i]++; return true }},
			beatTime:  {Timed: true, Round: func(ev any) (uint64, bool) { calls[i]++; return ev.(uint64), true }},
		}})
		s.Require().NoError(err)
		programs[i] = p
	}
	a, b := programs[0].NewBlob(), programs[1].NewBlob()
	for _, in := range []struct {
		on  machine.Beat
		now uint64
	}{{beatHeard, 1}, {beatTime, 3}, {beatTime, 4}} {
		ordinary, err := programs[0].Step(&a, in.on, in.now, nil, in.now)
		s.Require().NoError(err)
		traced, err := programs[1].TraceStep(&machine.TraceStepInput{Blob: &b, On: in.on, Event: in.now, Now: in.now})
		s.Require().NoError(err)
		s.Equal(ordinary, traced.Step)
		s.Equal(a, b)
		s.Equal(a.State, traced.Trace.After)
		s.Equal(calls[0], calls[1], "tracing cannot duplicate any vocabulary read")
	}
	s.Equal([2]int{3, 3}, calls)
	s.Equal("s", b.State)
	s.Empty(b.Bag)
	s.Nil(b.WakeAt)
	s.Require().NotNil(b.Anchor)
	s.Equal(uint64(1), *b.Anchor)
}

func (s *TraceSuite) TestObservedValuesAreDetachedAndUnknownStaysUnknown() {
	for _, tc := range []struct {
		name          string
		kind          machine.ScalarKind
		got, want     any
		known, scalar bool
	}{
		{"false", machine.KindBool, false, false, true, true},
		{"zero", machine.KindInt, 0, 0, true, true},
		{"empty", machine.KindString, "", "", true, true},
		{"unknown", machine.KindString, "private ignored value", "", false, false},
		{"opaque", machine.KindBool, map[string]any{"mutable": true}, true, true, false},
	} {
		s.Run(tc.name, func() {
			r := traceRecipe(machine.Transition{On: beatTime, When: &machine.Guard{Fact: &machine.FactLeaf{Name: "value", Op: machine.OpEq, Value: tc.want}}})
			p, err := r.Compile(machine.Vocabulary{Beats: map[machine.Beat]machine.BeatSpec{beatTime: {}}, Facts: map[machine.FactName]machine.FactSpec{"value": {Kind: tc.kind, Read: func(any) (any, bool) { return tc.got, tc.known }}}})
			s.Require().NoError(err)
			b := p.NewBlob()
			out, err := p.TraceStep(&machine.TraceStepInput{Blob: &b, On: beatTime})
			s.Require().NoError(err)
			g := out.Trace.Transitions[0].Guard
			s.True(g.Read)
			s.Equal(tc.known, g.Known)
			s.Equal(tc.scalar, g.Scalar)
			if tc.scalar {
				s.Equal(tc.got, g.Value)
			} else {
				s.Nil(g.Value)
				s.Equal(machine.Depends, g.Verdict)
			}
			out.Trace.Transitions[0].Guard.Value = "changed by reader"
			again, err := p.TraceStep(&machine.TraceStepInput{Blob: &b, On: beatTime})
			s.Require().NoError(err)
			if tc.scalar {
				s.Equal(tc.got, again.Trace.Transitions[0].Guard.Value)
			} else {
				s.Nil(again.Trace.Transitions[0].Guard.Value)
			}
		})
	}
}

func (s *TraceSuite) TestErrorsAndTerminalState() {
	p := mustCompile(traceRecipe(machine.Transition{On: beatTime, To: "done"}))
	out, err := p.TraceStep(nil)
	s.ErrorIs(err, machine.ErrNilInput)
	s.Nil(out)
	out, err = p.TraceStep(&machine.TraceStepInput{On: beatTime})
	s.ErrorIs(err, machine.ErrNilBlob)
	s.Nil(out)
	b := p.NewBlob()
	out, err = p.TraceStep(&machine.TraceStepInput{Blob: &b, On: "unknown"})
	s.ErrorIs(err, machine.ErrUnknownBeat)
	s.Nil(out)
	b.State = "invalid"
	out, err = p.TraceStep(&machine.TraceStepInput{Blob: &b, On: beatTime})
	s.ErrorIs(err, machine.ErrUnknownState)
	s.Nil(out)
	b.State = "done"
	out, err = p.TraceStep(&machine.TraceStepInput{Blob: &b, On: beatTime})
	s.Require().NoError(err)
	s.False(out.Step.Fired)
	s.Equal("done", out.Trace.Before)
	s.Equal("done", out.Trace.After)
	s.Empty(out.Trace.Transitions)
	b.State = ""
	out, err = p.TraceStep(&machine.TraceStepInput{Blob: &b, On: beatTime})
	s.Require().NoError(err)
	s.Equal("s", out.Trace.Before, "the effective initial state supplies symbols")
	s.True(out.Trace.Transitions[0].Guard.Evaluated)
	s.Equal(machine.Applies, out.Trace.Transitions[0].Guard.Verdict, "an absent guard is always")
}

func (s *TraceSuite) TestFlagScopeAndRoundsReadOnce() {
	r := traceRecipe(machine.Transition{On: beatTime, When: &machine.Guard{All: []machine.Guard{
		{Flag: "armed"}, {Scope: &machine.ScopeLeaf{Within: 2}}, {Rounds: &machine.RoundsLeaf{Op: machine.OpGte, N: 3}},
	}}})
	r.Bag = machine.Bag{"armed": false}
	scopes, rounds := 0, 0
	p, err := r.Compile(machine.Vocabulary{Beats: map[machine.Beat]machine.BeatSpec{beatTime: {
		InScope: func(_ any, scope machine.ScopeLeaf) bool { scopes++; s.Equal(2, scope.Within); return true },
		Round:   func(any) (uint64, bool) { rounds++; return 12, true },
	}}})
	s.Require().NoError(err)
	b := p.NewBlob()
	out, err := p.TraceStep(&machine.TraceStepInput{Blob: &b, On: beatTime})
	s.Require().NoError(err)
	s.Equal(false, out.Trace.Transitions[0].Guard.Children[0].Value)
	s.Zero(scopes)
	s.Zero(rounds)
	b.Bag = machine.Bag{"armed": true}
	b.Anchor = u64(9)
	out, err = p.TraceStep(&machine.TraceStepInput{Blob: &b, On: beatTime})
	s.Require().NoError(err)
	s.True(out.Step.Fired)
	s.Equal(1, scopes)
	s.Equal(1, rounds)
	children := out.Trace.Transitions[0].Guard.Children
	s.Equal(true, children[0].Value)
	s.Equal(true, children[1].Value)
	s.Equal(3, children[2].Value)
}
