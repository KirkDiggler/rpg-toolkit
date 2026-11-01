// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import "fmt"

// Stepped is what one Step did. Fired is false when no transition of the
// current state on the beat applied; Reason then says why (the first guard
// that depended, or that nothing listens). When a transition fired, Acts are
// its acts in recipe order, for the host to execute the ones that are its
// own (Pick); Reason is the transition's. From and To are set when the
// transition named a state, and are empty otherwise.
type Stepped struct {
	Fired  bool
	Acts   []Act
	From   string
	To     string
	Reason string
}

// Listen is one beat a state listens for, with the scope every transition on
// it requires, or nil when the beat is unscoped for this state.
type Listen struct {
	Beat  Beat
	Scope *ScopeLeaf
}

// Step runs b's current state's transitions on the beat on, in recipe order.
// The first whose On is on and whose guard answers Applies fires: the
// machine applies its own acts to b (Set, Clear, Anchor stamps now, WakeIn
// arms now plus n), enters To when it names a state, and returns the acts
// for the host, which executes Pick. Step never publishes and never calls
// the host except through the vocabulary's readers. ev is the beat's event,
// handed to InScope and Round; env is handed to the fact readers; now is the
// round of the clock the machine is seated on.
//
// A Timed beat asked at or after b's wake clears it before the transitions
// are asked, whether or not one fires, so a wake is never due twice; a
// WakeIn on the firing transition arms it again. A terminal state fires
// nothing. An empty b.State is set to the initial state.
//
// It returns ErrNilBlob for a nil b, ErrUnknownState when b names a state
// the recipe lacks and ErrUnknownBeat when on is not in the vocabulary.
func (p *Program) Step(b *Blob, on Beat, ev any, env any, now uint64) (Stepped, error) {
	if b == nil {
		return Stepped{}, ErrNilBlob
	}
	if b.State == "" {
		b.State = p.initial
	}
	st, ok := p.states[b.State]
	if !ok {
		return Stepped{}, fmt.Errorf("%w: blob state %q", ErrUnknownState, b.State)
	}
	spec, ok := p.beats[on]
	if !ok {
		return Stepped{}, fmt.Errorf("%w: %q", ErrUnknownBeat, on)
	}
	if spec.Timed && b.WakeAt != nil && now >= *b.WakeAt {
		b.WakeAt = nil
	}

	reason := fmt.Sprintf("state %s does not listen for %s", b.State, on)
	undecided := false
	a := &asked{env: env, ev: ev, blob: b, zero: p.zero}
	for i := range st.transitions {
		t := &st.transitions[i]
		if t.on != on {
			continue
		}
		v, why := t.when.answer(a)
		if v != Applies {
			if !undecided {
				reason = fmt.Sprintf("%s: %s", why, v)
				undecided = true
			}
			continue
		}
		p.apply(b, t.do, now)
		out := Stepped{Fired: true, Acts: cloneActs(t.do), Reason: t.reason}
		if t.to != "" {
			out.From, out.To = b.State, t.to
			b.State = t.to
		}
		return out, nil
	}
	return Stepped{Reason: reason}, nil
}

// apply runs the machine's own acts on b. A key set to its declared zero,
// or cleared, is dropped from the blob's bag, which carries only what
// differs.
func (p *Program) apply(b *Blob, do []Act, now uint64) {
	for _, a := range do {
		switch {
		case a.Set != nil:
			if p.zero[a.Set.Key] == a.Set.Value {
				delete(b.Bag, a.Set.Key)
				continue
			}
			if b.Bag == nil {
				b.Bag = Bag{}
			}
			b.Bag[a.Set.Key] = a.Set.Value
		case len(a.Clear) > 0:
			for _, k := range a.Clear {
				delete(b.Bag, k)
			}
		case a.Anchor:
			at := now
			b.Anchor = &at
		case a.WakeIn > 0:
			at := now + uint64(a.WakeIn)
			b.WakeAt = &at
		}
	}
	if len(b.Bag) == 0 {
		b.Bag = nil
	}
}

// Listens answers what b's current state listens for, so a host can index
// it: every beat a transition of the state is on, except the Timed ones
// (the due heap answers those), once each in recipe order with the scope
// its transitions require. A state with a wake still listens. It returns
// nil for a nil b, an unknown state or a state that listens for nothing.
// The slice and its scopes are the caller's.
func (p *Program) Listens(b *Blob) []Listen {
	st := p.stateOf(b)
	if st == nil || len(st.listens) == 0 {
		return nil
	}
	out := make([]Listen, len(st.listens))
	for i, l := range st.listens {
		if l.Scope != nil {
			s := *l.Scope
			l.Scope = &s
		}
		out[i] = l
	}
	return out
}

// DueAt answers the next round b wants to be asked the time: its wake when
// it has one; else now + 1 when its current state has a transition on a
// Timed beat; else not due. A nil b or an unknown state is not due.
func (p *Program) DueAt(b *Blob, now uint64) (uint64, bool) {
	st := p.stateOf(b)
	switch {
	case st == nil:
		return 0, false
	case b.WakeAt != nil:
		return *b.WakeAt, true
	case st.timed:
		return now + 1, true
	}
	return 0, false
}

// stateOf returns b's compiled state, reading an empty state as the
// initial one, or nil.
func (p *Program) stateOf(b *Blob) *program {
	if b == nil {
		return nil
	}
	name := b.State
	if name == "" {
		name = p.initial
	}
	return p.states[name]
}
