// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"context"
	jsonv1 "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/examples/effects/machine"
)

// ErrInvalidOutput wraps every refusal Compose makes of a recipe's outputs.
var ErrInvalidOutput = errors.New("invalid output")

// ErrNoAmount is returned by Bound.Amount for an output without an amount.
var ErrNoAmount = errors.New("output has no amount")

// Row is one output's information answer: its kind, its guard's verdict and
// reason, and the amount when the guard applies.
type Row struct {
	Kind    KindName
	Verdict machine.Verdict
	Reason  string
	Amount  int
}

// Bound is one output bound to its condition: what a kind's Attach and
// Assess are handed.
type Bound struct {
	id     string
	out    Output
	host   Host
	when   *machine.CompiledGuard
	amount AmountFn
	m      *machine.Machine
}

// ID returns an id unique to this output within its condition (ref, state
// and index), fit for a chain modifier.
func (b *Bound) ID() string { return b.id }

// Kind returns the output's kind.
func (b *Bound) Kind() KindName { return b.out.Kind }

// Params returns the output's params for the kind to decode.
func (b *Bound) Params() jsontext.Value { return b.out.Params }

// Host returns the condition as the output sees it.
func (b *Bound) Host() Host { return b.host }

// Answer evaluates the output's guard over frame and the condition's bag. A
// guard over an unknown fact answers machine.Depends.
func (b *Bound) Answer(frame any) (machine.Verdict, string) { return b.m.Answer(b.when, frame) }

// HasAmount reports whether the output carries an amount.
func (b *Bound) HasAmount() bool { return b.out.Amount != nil }

// Amount computes the output's amount from frame. It returns ErrNoAmount for
// an output without one, or the amount function's error.
func (b *Bound) Amount(frame any) (int, error) {
	if b.out.Amount == nil {
		return 0, ErrNoAmount
	}
	return b.amount(frame, *b.out.Amount)
}

// Composed is the condition built from a recipe: a machine plus the current
// state's outputs attached to the chains. It satisfies a rulebook's
// condition behaviour (Ref, IsApplied, Apply, Remove, ToJSON) structurally.
// Like its machine it is driven by bus handlers and is not safe for
// concurrent use.
type Composed struct {
	m        *machine.Machine
	ov       OutputVocabulary
	host     Host
	outputs  map[string][]*Bound
	attached []Detach
	bus      events.EventBus
}

type decoded struct {
	state string
	index int
	out   Output
	fn    AmountFn
}

// Compose validates r's outputs against ov and r against v, fills the
// instance's bag from params (ruling 3), and returns the condition for
// holder in state (empty for the recipe's initial state), not yet applied.
// It refuses, wrapping ErrInvalidOutput or the machine's errors: an output
// that does not decode; a kind not in ov, or one missing Attach or Assess;
// params the kind's Check refuses; an amount that sets other than one field
// or names a function ov does not hold; an output guard over an unknown fact
// or an undeclared flag; any refusal of machine.Validate; a parameter naming
// no bag key; and a state the recipe does not declare.
func Compose(
	r machine.Recipe, v machine.Vocabulary, ov OutputVocabulary,
	holder string, source *core.Ref, params machine.Bag, state string,
) (*Composed, error) {
	outs, keys, err := decodeOutputs(r, ov)
	if err != nil {
		return nil, err
	}
	p, err := r.Compile(v, keys...)
	if err != nil {
		return nil, err
	}
	if state != "" && !p.HasState(state) {
		return nil, fmt.Errorf("%w: %s: state %q", machine.ErrUnknownState, r.Ref.String(), state)
	}
	bag, err := p.NewBag(params)
	if err != nil {
		return nil, err
	}
	m := machine.New(p, holder, source, bag, state)
	c := &Composed{m: m, ov: ov, host: Host{Holder: holder, Ref: m.Ref(), Source: source}, outputs: map[string][]*Bound{}}
	for _, d := range outs {
		when, err := p.CompileGuard(d.out.When, "")
		if err != nil {
			return nil, fmt.Errorf("state %q output %d: %w", d.state, d.index, err)
		}
		c.outputs[d.state] = append(c.outputs[d.state], &Bound{
			id:   fmt.Sprintf("%s#%s.%d", r.Ref.String(), d.state, d.index),
			out:  d.out,
			host: c.host, when: when, amount: d.fn, m: m,
		})
	}
	m.OnTransition(c.onTransition)
	m.OnChanged(c.onChanged)
	return c, nil
}

// decodeOutputs decodes and checks every state's outputs, in sorted state
// order, and returns the bag keys their guards read.
func decodeOutputs(r machine.Recipe, ov OutputVocabulary) ([]decoded, []string, error) {
	var outs []decoded
	var keys []string
	refuse := func(state string, i int, format string, args ...any) error {
		return fmt.Errorf("%w: %s: state %q output %d: %s",
			ErrInvalidOutput, r.Ref.String(), state, i, fmt.Sprintf(format, args...))
	}
	for _, state := range sortedStates(r) {
		for i, raw := range r.States[state].Outputs {
			var o Output
			if err := json.Unmarshal(raw, &o); err != nil {
				return nil, nil, refuse(state, i, "%v", err)
			}
			spec, ok := ov.Kinds[o.Kind]
			if !ok || spec.Attach == nil || spec.Assess == nil {
				return nil, nil, refuse(state, i, "kind %q is not in the output vocabulary", o.Kind)
			}
			if spec.Check != nil {
				if err := spec.Check(o.Params); err != nil {
					return nil, nil, refuse(state, i, "kind %q refused its params: %v", o.Kind, err)
				}
			}
			d := decoded{state: state, index: i, out: o}
			if o.Amount != nil {
				name, err := o.Amount.Name()
				if err != nil {
					return nil, nil, refuse(state, i, "%v", err)
				}
				if d.fn = ov.Amounts[name]; d.fn == nil {
					return nil, nil, refuse(state, i, "amount %q is not in the output vocabulary", name)
				}
			}
			keys = append(keys, machine.GuardKeys(o.When)...)
			outs = append(outs, d)
		}
	}
	return outs, keys, nil
}

// Ref returns the recipe's ref; it is never nil.
func (c *Composed) Ref() *core.Ref { return c.m.Ref() }

// IsApplied reports whether the condition is attached to a bus.
func (c *Composed) IsApplied() bool { return c.m.IsApplied() }

// State returns the machine's current state.
func (c *Composed) State() string { return c.m.State() }

// Apply attaches the machine and the current state's outputs to bus. It
// returns machine.ErrAlreadyApplied when attached, and on any failure undoes
// everything it attached before returning the error.
func (c *Composed) Apply(ctx context.Context, bus events.EventBus) error {
	if err := c.m.Apply(ctx, bus); err != nil {
		return err
	}
	c.bus = bus
	if err := c.attach(ctx, c.m.State()); err != nil {
		return errors.Join(err, c.Remove(ctx, bus))
	}
	return nil
}

// Remove detaches the outputs and the machine from bus. It publishes no
// removal: that is the terminal state's End, or the caller's. It returns
// the joined detach errors and is a no-op when not applied.
func (c *Composed) Remove(ctx context.Context, bus events.EventBus) error {
	err := c.detach(ctx)
	c.bus = nil
	return errors.Join(err, c.m.Remove(ctx, bus))
}

// ToJSON writes the blob: ref, member_id, source, state and the bag's keys
// in one flat object with a stable key order.
func (c *Composed) ToJSON() (jsonv1.RawMessage, error) {
	b := c.m.ToBlob()
	return b.Marshal()
}

// Rows answers the information rows of the current state's outputs for
// frame, one per output in recipe order. A guard that depends is a row with
// machine.Depends, not an error; a kind's Assess error is returned.
func (c *Composed) Rows(frame any) ([]Row, error) {
	outs := c.outputs[c.m.State()]
	rows := make([]Row, 0, len(outs))
	for _, o := range outs {
		row, err := c.ov.Kinds[o.Kind()].Assess(frame, o)
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (c *Composed) attach(ctx context.Context, state string) error {
	for _, o := range c.outputs[state] {
		d, err := c.ov.Kinds[o.Kind()].Attach(ctx, c.bus, o)
		if err != nil {
			return fmt.Errorf("attach %s: %w", o.ID(), err)
		}
		c.attached = append(c.attached, d)
	}
	return nil
}

func (c *Composed) detach(ctx context.Context) error {
	var err error
	for _, d := range c.attached {
		err = errors.Join(err, d(ctx))
	}
	c.attached = nil
	return err
}

// onTransition swaps the outputs: on a terminal state End runs first, while
// the outputs still answer, then everything detaches (E5); otherwise the
// old state's outputs detach and the new state's attach.
func (c *Composed) onTransition(ctx context.Context, t machine.Transitioned) error {
	if c.m.Program().IsTerminal(t.To) {
		var err error
		if c.ov.End != nil {
			err = c.ov.End(ctx, c.bus, c.host, t.Reason)
		}
		err = errors.Join(err, c.detach(ctx))
		c.bus = nil
		return err
	}
	if err := c.detach(ctx); err != nil {
		return err
	}
	return c.attach(ctx, t.To)
}

func (c *Composed) onChanged(ctx context.Context) error {
	if c.ov.Changed == nil {
		return nil
	}
	return c.ov.Changed(ctx, c.bus, c.host)
}

func sortedStates(r machine.Recipe) []string {
	names := make([]string, 0, len(r.States))
	for name := range r.States {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
