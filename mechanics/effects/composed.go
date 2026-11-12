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
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

// ErrInvalidOutput wraps every refusal Compose makes of a recipe's outputs.
var ErrInvalidOutput = errors.New("invalid output")

// ErrNoAmount is returned by Bound.Amount for an output without an amount.
var ErrNoAmount = errors.New("output has no amount")

// Row is one output's information answer: its kind, its guard's verdict and
// reason, and the amount when the guard applies.
type Row struct {
	Kind    KindName        // the output's kind
	Verdict machine.Verdict // Applies, DoesNotApply, or Depends when the frame cannot answer
	Reason  string          // the guard leaf that decided ("always" for no guard)
	Amount  *AmountResult   // an answered amount; nil when absent or not applicable
}

// Bound is one output bound to its condition: what a kind's Attach and
// Assess are handed.
type Bound struct {
	id       string
	out      Output
	host     Host
	when     *machine.CompiledGuard
	amount   AmountFn
	m        *machine.Machine
	reactive func() bool
}

// ID returns an id unique to this output within its condition (ref, state
// and index), fit for a chain modifier.
func (b *Bound) ID() string { return b.id }

// Kind returns the output's kind.
func (b *Bound) Kind() KindName { return b.out.Kind }

// Params returns an owned copy of the output's params for the kind to decode.
func (b *Bound) Params() jsontext.Value { return slices.Clone(b.out.Params) }

// Host returns the condition as the output sees it, with detached ref values.
func (b *Bound) Host() Host {
	host := b.host
	host.Ref = cloneRef(host.Ref)
	host.Source = cloneRef(host.Source)
	return host
}

// Answer evaluates the output's guard over frame and the condition's bag. A
// guard over an unknown fact answers machine.Depends.
func (b *Bound) Answer(frame any) (machine.Verdict, string) { return b.m.Answer(b.when, frame) }

// CanReact reports whether this currently attached output may request new work.
// It is false for stale-state handles and during application end reporting, even
// while Answer/Amount still describe contributions for the event being resolved.
func (b *Bound) CanReact() bool { return b.reactive != nil && b.reactive() }

// HasAmount reports whether the output carries an amount.
func (b *Bound) HasAmount() bool { return b.out.Amount != nil }

// Amount returns an owned, validated scalar or unresolved dice description. It
// returns ErrNoAmount for an output without one and propagates evaluator errors.
func (b *Bound) Amount(frame any) (AmountResult, error) {
	if b.out.Amount == nil {
		return AmountResult{}, ErrNoAmount
	}
	result, err := b.amount(frame, *b.out.Amount)
	if err != nil {
		return AmountResult{}, err
	}
	if err := result.Validate(); err != nil {
		return AmountResult{}, err
	}
	return result.owned(), nil
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
	// endState is an observation cursor during End-before-detach, not saved state.
	endState *string
	// reactionsAllowed is optional assembly-owned dispatch permission, not state.
	reactionsAllowed func() bool
}

type decoded struct {
	state string
	index int
	out   Output
	fn    AmountFn
}

// ComposeInput names the recipe, consumer vocabulary and instance data used to
// construct a composed condition. It contains no bus: Apply owns attachment.
type ComposeInput struct {
	// InstanceKey namespaces output IDs for linked/local assemblies. Empty keeps
	// the standalone ref/state/index identity; it is not source provenance.
	InstanceKey string
	// Recipe declares the states, outputs, transitions and bag defaults.
	Recipe machine.Recipe
	// MachineVocabulary supplies the facts and beats used by the recipe.
	MachineVocabulary machine.Vocabulary
	// OutputVocabulary supplies output kinds, amounts and lifecycle hooks.
	OutputVocabulary OutputVocabulary
	// HolderID identifies the entity receiving the composed condition.
	HolderID string
	// SourceRef identifies the applying content; nil means no source.
	SourceRef *core.Ref
	// Parameters fills declared bag keys over the recipe defaults. Nil uses
	// those defaults; a saved bag can be supplied to restore an instance.
	Parameters machine.Bag
	// State restores a declared state. Empty selects the recipe's initial state.
	State string
}

// Compose validates the recipe and its outputs against the supplied vocabularies,
// fills the instance bag from Parameters, and returns a condition not yet applied.
// It refuses, wrapping ErrInvalidOutput or the machine's errors: an output that
// does not decode; an unknown kind, missing Assess or invalid execution/Attach mode; parameters
// the kind's Check refuses; an invalid or unknown amount; a guard over an unknown
// fact or undeclared flag; any machine.Validate refusal; a parameter naming no
// bag key; and a state the recipe does not declare.
func Compose(input ComposeInput) (*Composed, error) {
	outs, keys, err := decodeOutputs(input.Recipe, input.OutputVocabulary)
	if err != nil {
		return nil, err
	}
	p, err := input.Recipe.Compile(input.MachineVocabulary, keys...)
	if err != nil {
		return nil, err
	}
	if input.State != "" && !p.HasState(input.State) {
		return nil, fmt.Errorf("%w: %s: state %q", machine.ErrUnknownState, input.Recipe.Ref.String(), input.State)
	}
	bag, err := p.NewBag(input.Parameters)
	if err != nil {
		return nil, err
	}
	m := machine.New(machine.Config{
		Program: p, HolderID: input.HolderID, SourceRef: input.SourceRef, Bag: bag, State: input.State,
	})
	c := &Composed{
		m: m, ov: input.OutputVocabulary,
		host: Host{Holder: input.HolderID, Ref: m.Ref(), Source: input.SourceRef}, outputs: map[string][]*Bound{},
	}
	for _, d := range outs {
		when, err := p.CompileGuard(d.out.When, "")
		if err != nil {
			return nil, fmt.Errorf("state %q output %d: %w", d.state, d.index, err)
		}
		id := fmt.Sprintf("%s#%s.%d", input.Recipe.Ref.String(), d.state, d.index)
		if input.InstanceKey != "" {
			id = fmt.Sprintf("%d:%s/%s", len(input.InstanceKey), input.InstanceKey, id)
		}
		c.outputs[d.state] = append(c.outputs[d.state], &Bound{
			id:   id,
			out:  d.out,
			host: c.host, when: when, amount: d.fn, m: m,
			reactive: func() bool {
				return m.IsApplied() && m.State() == d.state &&
					(c.reactionsAllowed == nil || c.reactionsAllowed())
			},
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
			if !ok || spec.Assess == nil {
				return nil, nil, refuse(state, i, "kind %q is not in the output vocabulary", o.Kind)
			}
			switch spec.Execution {
			case KindSubscribed:
				if spec.Attach == nil {
					return nil, nil, refuse(state, i, "subscribed kind %q requires Attach", o.Kind)
				}
			case KindQuery:
				if spec.Attach != nil {
					return nil, nil, refuse(state, i, "query kind %q forbids Attach", o.Kind)
				}
			default:
				return nil, nil, refuse(state, i, "kind %q has unknown execution %q", o.Kind, spec.Execution)
			}
			if spec.Check != nil {
				if err := spec.Check(o); err != nil {
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
	c.endState = nil
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
	outs := c.CurrentOutputs()
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
		spec := c.ov.Kinds[o.Kind()]
		if spec.Execution == KindQuery {
			continue
		}
		d, err := spec.Attach(ctx, c.bus, o)
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
// old state's outputs detach and the new state's attach, then Changed reports
// the successful nonterminal state change.
func (c *Composed) onTransition(ctx context.Context, t machine.Transitioned) error {
	if c.m.Program().IsTerminal(t.To) {
		c.endState = &t.From
		defer func() { c.endState = nil }()
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
	if err := c.attach(ctx, t.To); err != nil {
		return err
	}
	return c.onChanged(ctx)
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
