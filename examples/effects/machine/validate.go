// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// ErrInvalidRecipe wraps every refusal Validate and Compile return.
var ErrInvalidRecipe = errors.New("invalid recipe")

// ErrInvalidBag wraps every refusal NewBag returns.
var ErrInvalidBag = errors.New("invalid bag")

// Validate checks r against v and returns the first refusal by name,
// wrapping ErrInvalidRecipe. It refuses: a stacks value other than by_ref or
// by_source; by_source without a source_id key; a bag key of a type other
// than bool, int or string, or one that collides with the blob header; an
// initial state that is missing or terminal; a beat or fact not in the
// vocabulary; a selector the beat does not support; a To naming no state; a
// transition into a terminal state without a reason; a guard node, rounds
// leaf or act that sets other than exactly one field; a flag or written key
// not declared in the bag, or of the wrong kind; a fact value or op that does
// not fit the fact's kind; a rounds leaf on a beat that carries no round; a
// state no transition reaches; and a bag key nothing reads.
//
// The machine cannot see inside outputs, so readElsewhere names the bag keys
// the caller's outputs read; they count as read.
func Validate(r Recipe, v Vocabulary, readElsewhere ...string) error {
	_, err := compile(r, v, readElsewhere)
	return err
}

// Compile validates r against v exactly as Validate does and binds it into a
// Program: fact readers and beat specs resolved once. readElsewhere is as
// for Validate. The recipe must not be changed after Compile.
func (r Recipe) Compile(v Vocabulary, readElsewhere ...string) (*Program, error) {
	return compile(r, v, readElsewhere)
}

// Program is a recipe bound to its vocabulary, ready to run machines.
type Program struct {
	recipe   Recipe
	vocab    Vocabulary
	states   map[string]*compiledState
	trackers []compiledTracker
	beats    []Beat
}

type compiledState struct {
	terminal    bool
	outputs     []jsontext.Value
	transitions []compiledTransition
}

type compiledTransition struct {
	Transition
	spec  BeatSpec
	guard *CompiledGuard
}

type compiledTracker struct {
	Tracker
	spec BeatSpec
}

// Ref returns the recipe's ref.
func (p *Program) Ref() core.Ref { return p.recipe.Ref }

// Initial returns the recipe's initial state.
func (p *Program) Initial() string { return p.recipe.Initial }

// HasState reports whether the recipe declares state.
func (p *Program) HasState(state string) bool { return p.states[state] != nil }

// IsTerminal reports whether state is a terminal state of the recipe; an
// unknown state is not terminal.
func (p *Program) IsTerminal(state string) bool {
	st := p.states[state]
	return st != nil && st.terminal
}

// Outputs returns the opaque outputs of state, nil for an unknown state.
func (p *Program) Outputs(state string) []jsontext.Value {
	if st := p.states[state]; st != nil {
		return st.outputs
	}
	return nil
}

// States returns the recipe's state names in sorted order.
func (p *Program) States() []string {
	names := make([]string, 0, len(p.states))
	for name := range p.states {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// NewBag returns the recipe's declared bag with params written over it: the
// instance's bag (ruling 3). A parameter naming no declared key, or a value
// whose kind differs from the key's zero value, is refused by name, wrapping
// ErrInvalidBag. Whole float64 numbers (JSON's) land as int.
func (p *Program) NewBag(params map[string]any) (Bag, error) {
	bag := p.recipe.Bag.Clone()
	if bag == nil {
		bag = Bag{}
	}
	for _, key := range sortedKeys(params) {
		zero, declared := p.recipe.Bag[key]
		if !declared {
			return nil, fmt.Errorf("%w: %s: parameter %q names no bag key", ErrInvalidBag, p.recipe.Ref.String(), key)
		}
		val := wholeNumber(params[key])
		want, _ := kindOf(zero)
		got, ok := kindOf(val)
		if !ok || got != want {
			return nil, fmt.Errorf("%w: %s: bag key %q wants a %s value, got %T",
				ErrInvalidBag, p.recipe.Ref.String(), key, want, params[key])
		}
		bag[key] = val
	}
	return bag, nil
}

// CompileGuard validates and binds a guard against the program's vocabulary
// and bag. on names the beat whose events the guard will be asked about; an
// empty on is a guard read outside a transition, such as an output's, where
// a rounds leaf is refused because only a beat carries a round. A nil guard
// compiles to nil, which always applies. Refusals wrap ErrInvalidRecipe.
func (p *Program) CompileGuard(g *Guard, on Beat) (*CompiledGuard, error) {
	c := &checker{r: p.recipe, v: p.vocab, read: map[string]bool{}}
	var spec *BeatSpec
	if on != "" {
		s, ok := p.vocab.Beats[on]
		if !ok {
			return nil, fmt.Errorf("%w: %s: beat %q is not in the vocabulary", ErrInvalidRecipe, p.recipe.Ref.String(), on)
		}
		spec = &s
	}
	cg := c.guard(g, spec, "guard", on)
	if c.err != nil {
		return nil, c.err
	}
	return cg, nil
}

// GuardKeys returns the bag keys a guard tree reads, sorted, so a caller can
// name them to Validate as read elsewhere.
func GuardKeys(g *Guard) []string {
	seen := map[string]bool{}
	var walk func(g *Guard)
	walk = func(g *Guard) {
		if g == nil {
			return
		}
		for i := range g.All {
			walk(&g.All[i])
		}
		for i := range g.Any {
			walk(&g.Any[i])
		}
		walk(g.Not)
		if g.Flag != "" {
			seen[g.Flag] = true
		}
		if g.Rounds != nil {
			seen[KeyRoundActivated] = true
		}
	}
	walk(g)
	return sortedKeys(seen)
}

// checker accumulates the first refusal while it walks a recipe.
type checker struct {
	r    Recipe
	v    Vocabulary
	read map[string]bool
	err  error
}

func (c *checker) failf(format string, args ...any) {
	if c.err == nil {
		c.err = fmt.Errorf("%w: %s: %s", ErrInvalidRecipe, c.r.Ref.String(), fmt.Sprintf(format, args...))
	}
}

func compile(r Recipe, v Vocabulary, readElsewhere []string) (*Program, error) {
	c := &checker{r: r, v: v, read: map[string]bool{}}
	for _, k := range readElsewhere {
		c.read[k] = true
	}
	p := &Program{recipe: r, vocab: v, states: map[string]*compiledState{}}

	c.header()
	p.trackers = c.trackers()
	beats := map[Beat]bool{}
	for _, name := range sortedKeys(r.States) {
		st := r.States[name]
		cs := &compiledState{terminal: st.Terminal, outputs: st.Outputs}
		for i, t := range st.Transitions {
			ct := c.transition(name, i, t)
			cs.transitions = append(cs.transitions, ct)
			beats[t.On] = true
		}
		p.states[name] = cs
	}
	c.reachable()
	c.unread()
	if c.err != nil {
		return nil, c.err
	}
	p.beats = sortedKeys(beats)
	return p, nil
}

// header checks stacks, the bag's keys and the initial state.
func (c *checker) header() {
	switch c.r.Stacks {
	case "", StacksByRef:
	case StacksBySource:
		if _, ok := c.r.Bag[KeySourceID]; !ok {
			c.failf("stacks %q needs a %q bag key", StacksBySource, KeySourceID)
		}
		c.read[KeySourceID] = true
	default:
		c.failf("stacks %q is neither %q nor %q", c.r.Stacks, StacksByRef, StacksBySource)
	}
	for _, key := range sortedKeys(c.r.Bag) {
		if headerKeys[key] {
			c.failf("bag key %q collides with the blob header", key)
		}
		if _, ok := kindOf(c.r.Bag[key]); !ok {
			c.failf("bag key %q has unsupported type %T, want bool, int or string", key, c.r.Bag[key])
		}
	}
	st, ok := c.r.States[c.r.Initial]
	switch {
	case !ok:
		c.failf("initial state %q names no state", c.r.Initial)
	case st.Terminal:
		c.failf("initial state %q is terminal", c.r.Initial)
	}
}

// beat resolves a beat and selector, refusing either by name.
func (c *checker) beat(where string, b Beat, sel Selector) BeatSpec {
	spec, ok := c.v.Beats[b]
	switch {
	case !ok:
		c.failf("%s: beat %q is not in the vocabulary", where, b)
	case spec.Subscribe == nil || spec.About == nil:
		c.failf("%s: vocabulary beat %q has no Subscribe or About", where, b)
	case !spec.supports(sel):
		c.failf("%s: beat %q does not support selector %q", where, b, sel)
	}
	return spec
}

// key checks that a key the recipe names is declared with kind want, and
// records it as read.
func (c *checker) key(where, what, key string, want ScalarKind) {
	zero, ok := c.r.Bag[key]
	if !ok {
		c.failf("%s: %s %q is not declared in the bag", where, what, key)
		return
	}
	if got, _ := kindOf(zero); got != want {
		c.failf("%s: %s %q is declared %s, want %s", where, what, key, got, want)
	}
	c.read[key] = true
}

func (c *checker) trackers() []compiledTracker {
	out := make([]compiledTracker, 0, len(c.r.Trackers))
	for i, t := range c.r.Trackers {
		where := fmt.Sprintf("tracker %d", i)
		spec := c.beat(where, t.On, t.About)
		c.key(where, "flag", t.Set, KindBool)
		out = append(out, compiledTracker{Tracker: t, spec: spec})
	}
	return out
}

func (c *checker) transition(state string, i int, t Transition) compiledTransition {
	where := fmt.Sprintf("state %q transition %d", state, i)
	spec := c.beat(where, t.On, t.About)
	if t.To != "" {
		to, ok := c.r.States[t.To]
		switch {
		case !ok:
			c.failf("%s: to %q names no state", where, t.To)
		case to.Terminal && t.Reason == "":
			c.failf("%s: transition into terminal state %q has no reason", where, t.To)
		}
	}
	for j, a := range t.Do {
		c.act(fmt.Sprintf("%s act %d", where, j), a)
	}
	return compiledTransition{Transition: t, spec: spec, guard: c.guard(t.When, &spec, where, t.On)}
}

func (c *checker) act(where string, a Act) {
	n := 0
	if a.AnchorRound {
		n++
		c.key(where, "anchor", KeyRoundActivated, KindInt)
		c.key(where, "anchor", KeySawTurnEnd, KindBool)
	}
	if a.DeriveTurnsActive {
		n++
		c.key(where, "anchor", KeyRoundActivated, KindInt)
		c.key(where, "derived key", KeyTurnsActive, KindInt)
	}
	if len(a.ClearFlags) > 0 {
		n++
		for _, f := range a.ClearFlags {
			c.key(where, "flag", f, KindBool)
		}
	}
	if a.Set != nil {
		n++
		zero, ok := c.r.Bag[a.Set.Key]
		want, _ := kindOf(zero)
		got, valid := kindOf(a.Set.Value)
		switch {
		case !ok:
			c.failf("%s: set key %q is not declared in the bag", where, a.Set.Key)
		case !valid || got != want:
			c.failf("%s: set key %q wants a %s value, got %T", where, a.Set.Key, want, a.Set.Value)
		}
		c.read[a.Set.Key] = true
	}
	if n != 1 {
		c.failf("%s: act sets %d fields, want exactly 1", where, n)
	}
}

// reachable refuses a state no transition reaches from the initial state.
func (c *checker) reachable() {
	seen := map[string]bool{c.r.Initial: true}
	queue := []string{c.r.Initial}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		for _, t := range c.r.States[name].Transitions {
			if t.To != "" && !seen[t.To] {
				seen[t.To] = true
				queue = append(queue, t.To)
			}
		}
	}
	for _, name := range sortedKeys(c.r.States) {
		if !seen[name] {
			c.failf("state %q is reached by no transition", name)
		}
	}
}

// unread refuses a declared bag key nothing reads or writes.
func (c *checker) unread() {
	for _, key := range sortedKeys(c.r.Bag) {
		if !c.read[key] {
			c.failf("bag key %q is read by no guard, act or tracker", key)
		}
	}
}

// sortedKeys returns m's keys in sorted order.
func sortedKeys[K ~string, V any](m map[K]V) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// wholeNumber lands a whole float64 (JSON's number) as int; any other value
// passes through.
func wholeNumber(v any) any {
	f, ok := v.(float64)
	if ok && f == float64(int(f)) {
		return int(f)
	}
	return v
}
