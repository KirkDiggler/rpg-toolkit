// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// ErrAlreadyApplied is returned by Apply on a machine that is attached.
var ErrAlreadyApplied = errors.New("machine already applied")

// ErrUnknownState is returned by Apply when the machine's state is not a
// live state of its recipe: unknown, or terminal.
var ErrUnknownState = errors.New("unknown or terminal state")

// Transitioned is the event a machine fires each time it enters a state:
// where it was, where it is now, and the transition's reason (required when
// To is terminal).
type Transitioned struct {
	From   string
	To     string
	Reason string
}

// Machine runs one Program for one holder over one bag. It subscribes to
// the beats its recipe names, runs the current state's transitions in
// recipe order on each, and fires Transitioned to its listeners on every
// state it enters. It knows no chain and no output. A Machine is driven by
// its bus's handlers and is not safe for concurrent use.
type Machine struct {
	p       *Program
	holder  string
	source  *core.Ref
	bag     Bag
	current string

	bus  events.EventBus
	subs []string

	onTransition []func(context.Context, Transitioned) error
	onChanged    []func(context.Context) error
}

// New returns a machine for holder in state over bag. An empty state is the
// recipe's initial state; a nil bag is the recipe's declared zero bag. bag
// should come from Program.NewBag; New does not copy it. The machine is not
// attached until Apply.
func New(p *Program, holder string, source *core.Ref, bag Bag, state string) *Machine {
	if state == "" {
		state = p.Initial()
	}
	if bag == nil {
		bag = p.recipe.Bag.Clone()
	}
	return &Machine{p: p, holder: holder, source: source, bag: bag, current: state}
}

// OnTransition registers a listener called, in registration order, each
// time the machine enters a state. On a terminal state every listener runs
// before the machine detaches, so a listener can still publish while the
// condition is attached. A listener's error is returned to the publisher of
// the beat.
func (m *Machine) OnTransition(f func(context.Context, Transitioned) error) {
	m.onTransition = append(m.onTransition, f)
}

// OnChanged registers a listener called when a tracker or a staying
// transition's acts change the bag. Entering a state fires OnTransition
// instead.
func (m *Machine) OnChanged(f func(context.Context) error) {
	m.onChanged = append(m.onChanged, f)
}

// Apply subscribes the recipe's trackers and one handler per beat its
// transitions name. It returns ErrAlreadyApplied when attached and
// ErrUnknownState when the current state is unknown or terminal; if any
// subscription fails, every one made so far is undone and the error is
// returned.
func (m *Machine) Apply(ctx context.Context, bus events.EventBus) error {
	if m.bus != nil {
		return ErrAlreadyApplied
	}
	if !m.p.HasState(m.current) || m.p.IsTerminal(m.current) {
		return fmt.Errorf("%w: %q", ErrUnknownState, m.current)
	}
	subs := make([]string, 0, len(m.p.trackers)+len(m.p.beats))
	rollback := func(err error) error {
		for _, id := range subs {
			err = errors.Join(err, bus.Unsubscribe(ctx, id))
		}
		return err
	}
	for _, t := range m.p.trackers {
		id, err := t.spec.Subscribe(ctx, bus, m.trackerHandler(t))
		if err != nil {
			return rollback(fmt.Errorf("tracker on %q: %w", t.On, err))
		}
		subs = append(subs, id)
	}
	for _, b := range m.p.beats {
		id, err := m.p.vocab.Beats[b].Subscribe(ctx, bus, m.beatHandler(b))
		if err != nil {
			return rollback(fmt.Errorf("beat %q: %w", b, err))
		}
		subs = append(subs, id)
	}
	m.bus, m.subs = bus, subs
	return nil
}

// Remove unsubscribes everything Apply attached from bus. It is a no-op on
// a detached machine and returns the joined unsubscribe errors.
func (m *Machine) Remove(ctx context.Context, bus events.EventBus) error {
	if m.bus == nil {
		return nil
	}
	var err error
	for _, id := range m.subs {
		err = errors.Join(err, bus.Unsubscribe(ctx, id))
	}
	m.bus, m.subs = nil, nil
	return err
}

// IsApplied reports whether the machine is attached to a bus.
func (m *Machine) IsApplied() bool { return m.bus != nil }

// State returns the current state's name.
func (m *Machine) State() string { return m.current }

// Holder returns the id of the entity the machine runs for.
func (m *Machine) Holder() string { return m.holder }

// Source returns the ref that applied the condition, possibly nil.
func (m *Machine) Source() *core.Ref { return m.source }

// Ref returns a copy of the recipe's ref; it is never nil.
func (m *Machine) Ref() *core.Ref {
	ref := m.p.Ref()
	return &ref
}

// Program returns the program the machine runs.
func (m *Machine) Program() *Program { return m.p }

// Bag returns a copy of the bag.
func (m *Machine) Bag() Bag { return m.bag.Clone() }

// Answer evaluates g over frame and the live bag, outside any beat. It is
// how the composer asks an output's guard without copying the bag.
func (m *Machine) Answer(g *CompiledGuard, frame any) (Verdict, string) {
	return g.Answer(frame, m.bag, nil)
}

// ToBlob returns what the condition persists: the recipe's ref, holder,
// source, state and a copy of the bag.
func (m *Machine) ToBlob() Blob {
	return Blob{Ref: m.p.Ref(), MemberID: m.holder, Source: m.source, State: m.current, Bag: m.bag.Clone()}
}

// isAbout reports whether the event names the holder under sel.
func (m *Machine) isAbout(spec BeatSpec, sel Selector, ev any) bool {
	id, ok := spec.About(ev, sel)
	return ok && id == m.holder
}

func (m *Machine) trackerHandler(t compiledTracker) func(context.Context, any) error {
	return func(ctx context.Context, ev any) error {
		if m.bus == nil || !m.isAbout(t.spec, t.About, ev) {
			return nil
		}
		if set, _ := m.bag[t.Set].(bool); set {
			return nil // once per turn: already set, no change
		}
		m.bag[t.Set] = true
		return m.changed(ctx)
	}
}

func (m *Machine) beatHandler(b Beat) func(context.Context, any) error {
	return func(ctx context.Context, ev any) error { return m.onBeat(ctx, b, ev) }
}

// onBeat runs the current state's transitions on b in recipe order; the
// first that is about the holder and whose guard applies runs its acts and,
// if it names a state, enters it. A guard that depends does not fire.
func (m *Machine) onBeat(ctx context.Context, b Beat, ev any) error {
	if m.bus == nil {
		return nil
	}
	for _, t := range m.p.states[m.current].transitions {
		if t.On != b || !m.isAbout(t.spec, t.About, ev) {
			continue
		}
		if v, _ := t.guard.Answer(ev, m.bag, ev); v != Applies {
			continue
		}
		changed := m.run(t.Do, t.spec, ev)
		if t.To != "" {
			return m.enter(ctx, t.To, t.Reason)
		}
		if changed {
			return m.changed(ctx)
		}
		return nil
	}
	return nil
}

// enter moves to state, fires the transition listeners, and on a terminal
// state detaches the machine after they have run.
func (m *Machine) enter(ctx context.Context, state, reason string) error {
	t := Transitioned{From: m.current, To: state, Reason: reason}
	m.current = state
	var err error
	for _, f := range m.onTransition {
		if err = f(ctx, t); err != nil {
			break
		}
	}
	if m.p.IsTerminal(state) {
		err = errors.Join(err, m.Remove(ctx, m.bus))
	}
	return err
}

func (m *Machine) changed(ctx context.Context) error {
	for _, f := range m.onChanged {
		if err := f(ctx); err != nil {
			return err
		}
	}
	return nil
}
