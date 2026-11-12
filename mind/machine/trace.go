// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

// TraceStepInput supplies the same blob, occurrence and clock as Program.Step.
// TraceStep mutates Blob exactly as Step does; a caller previewing a proposal
// supplies its own candidate blob and decides whether to commit it.
type TraceStepInput struct {
	Blob  *Blob
	On    Beat
	Event any
	Env   any
	Now   uint64
}

// TraceStepOutput pairs the ordinary result with its one-pass explanation.
// The trace owns its values; modifying it cannot change the program or blob.
type TraceStepOutput struct {
	Step  Stepped
	Trace StepTrace
}

// StepTrace identifies the effective state and every transition in that state,
// in recipe order. Before uses the initial state when the supplied state was
// empty. After describes the supplied blob after this step, not a promise that
// an outer composer committed a preview. No configuration or host object is copied.
type StepTrace struct {
	Before      string
	After       string
	On          Beat
	Now         uint64
	Transitions []TransitionTrace
}

// TransitionTrace addresses a transition by its recipe index. Evaluated is
// false for a different beat and for matching transitions after the winner;
// neither means a rejected guard. Selected marks the first applying transition.
type TransitionTrace struct {
	Index     int
	On        Beat
	Evaluated bool
	Selected  bool
	Guard     GuardTrace
}

// GuardTrace records a node evaluated in the actual guard walk. Children keep
// authored positions, including unevaluated children after a short circuit.
// Their descendants are supplied by the configuration symbols, not guessed here.
// Verdict zero with Evaluated false is not DoesNotApply or Depends.
//
// Read marks a leaf observation. Known is the reader's knowledge answer; Scalar
// says its known value was a bool, int or string. Value is present only then:
// unknown returns and opaque host objects are never retained. A known wrong-kind
// scalar is still the actual observation, even when comparison answers Depends.
// For scope Value is the returned scope verdict; for rounds it is elapsed rounds.
type GuardTrace struct {
	Evaluated bool
	Verdict   Verdict
	Reason    string
	Read      bool
	Known     bool
	Value     any
	Scalar    bool
	Children  []GuardTrace
}

// TraceStep evaluates once through the same implementation as Step and returns
// its observed alternatives. It never asks an unvisited guard for diagnostics.
// Errors are Step's refusals plus ErrNilInput; a refusal returns no output.
func (p *Program) TraceStep(in *TraceStepInput) (*TraceStepOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	out := &TraceStepOutput{}
	step, err := p.step(in.Blob, in.On, in.Event, in.Env, in.Now, &out.Trace)
	if err != nil {
		return nil, err
	}
	out.Step = step
	out.Trace.After = in.Blob.State
	return out, nil
}

// child is nil on the ordinary, untraced evaluation path.
func (t *GuardTrace) child(i int) *GuardTrace {
	if t == nil {
		return nil
	}
	return &t.Children[i]
}

// observe keeps only values the scalar vocabulary can describe. An unknown
// callback's ignored return must never become an accidental information leak.
func (t *GuardTrace) observe(value any, known bool) {
	if t == nil {
		return
	}
	t.Read, t.Known = true, known
	if !known {
		return
	}
	_, t.Scalar = kindOf(value)
	if t.Scalar {
		t.Value = value
	}
}
