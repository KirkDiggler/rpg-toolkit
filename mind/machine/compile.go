// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// Program is a compiled recipe: every name bound to its vocabulary spec,
// every guard built, and what each state listens to worked out once. It is
// immutable and safe to share across every blob that runs the recipe.
type Program struct {
	initial string
	zero    Bag
	beats   map[Beat]BeatSpec
	states  map[string]*program
}

// program is one compiled state.
type program struct {
	terminal    bool
	timed       bool // some transition is on a Timed beat
	transitions []transition
	listens     []Listen
}

// transition is one compiled transition.
type transition struct {
	on     Beat
	when   *guard
	to     string
	do     []Act
	reason string
}

// Compile checks the recipe against v and binds it into a Program. The
// program holds its own copy: changing the recipe afterwards changes
// nothing compiled.
//
// It refuses, joining every problem found into one error that names where
// each is: a beat v lacks (ErrUnknownBeat), a fact v lacks (ErrUnknownFact),
// a To or Initial naming no state (ErrUnknownState), no Initial
// (ErrNoInitial), an act that is not exactly one field or a WakeIn at or
// below zero or a Set or Clear of an undeclared key (ErrBadAct), a scope
// leaf on a beat with no InScope (ErrNoScope), a rounds leaf on a beat with
// no Round (ErrNoRound), a malformed guard (ErrBadGuard), a bag value that
// is not a scalar (ErrBadBag) and a terminal state with transitions
// (ErrBadState). A state with no transitions compiles; so does a recipe
// whose initial state has no timed transition, which is a machine never
// due until something it listens to fires.
//
// The machine's acts never raise a beat, so the cyclic do (a transition
// whose acts fire the beat it is on) cannot be written in a recipe; it is
// the host's bound to keep, on the beats a Pick lands.
func (r Recipe) Compile(v Vocabulary) (*Program, error) {
	c := &checker{v: v, bag: r.Bag}
	for _, k := range sortedKeys(r.Bag) {
		val := r.Bag[k]
		if _, ok := kindOf(val); !ok {
			c.failf(ErrBadBag, "bag."+k, "value %v (%T) is not a bool, int or string", val, val)
		}
	}
	switch _, ok := r.States[r.Initial]; {
	case r.Initial == "":
		c.failf(ErrNoInitial, "initial", "the recipe names no initial state")
	case !ok:
		c.failf(ErrUnknownState, "initial", "state %q is not in the recipe", r.Initial)
	}

	p := &Program{
		initial: r.Initial,
		zero:    r.Bag.Clone(),
		beats:   maps.Clone(v.Beats),
		states:  make(map[string]*program, len(r.States)),
	}
	for _, name := range sortedKeys(r.States) {
		p.states[name] = c.state(name, r.States[name], r.States)
	}
	if c.err != nil {
		return nil, c.err
	}
	return p, nil
}

// checker accumulates every refusal while it compiles.
type checker struct {
	v   Vocabulary
	bag Bag
	err error
}

// failf records one refusal of kind at where.
func (c *checker) failf(kind error, where, format string, args ...any) {
	c.err = errors.Join(c.err, fmt.Errorf("%w: %s: %s", kind, where, fmt.Sprintf(format, args...)))
}

func (c *checker) state(name string, s State, all map[string]State) *program {
	out := &program{terminal: s.Terminal}
	if s.Terminal && len(s.Transitions) > 0 {
		c.failf(ErrBadState, "states."+name, "a terminal state carries no transitions")
	}
	for i, t := range s.Transitions {
		where := fmt.Sprintf("states.%s.transitions[%d]", name, i)
		spec, ok := c.v.Beats[t.On]
		if !ok {
			c.failf(ErrUnknownBeat, where, "beat %q is not in the vocabulary", t.On)
		}
		if t.To != "" {
			if _, ok := all[t.To]; !ok {
				c.failf(ErrUnknownState, where, "to %q is not a state of the recipe", t.To)
			}
		}
		for j, a := range t.Do {
			c.act(fmt.Sprintf("%s.do[%d]", where, j), a)
		}
		ct := transition{
			on:     t.On,
			when:   c.guard(t.When, spec, ok, where+".when"),
			to:     t.To,
			do:     cloneActs(t.Do),
			reason: t.Reason,
		}
		out.transitions = append(out.transitions, ct)
		if spec.Timed {
			out.timed = true
		} else if ok {
			out.listens = listen(out.listens, t.On, necessaryScope(t.When))
		}
	}
	return out
}

// listen adds one transition's beat and scope to a state's listens: one
// listen per beat, in recipe order. Two scopes on one beat widen to the
// larger when they measure the same way; an unscoped transition, or scopes
// that measure differently, leave the beat unscoped, which wakes more
// listeners and lets the guards decide.
func listen(ls []Listen, on Beat, scope *ScopeLeaf) []Listen {
	i := slices.IndexFunc(ls, func(l Listen) bool { return l.Beat == on })
	if i < 0 {
		return append(ls, Listen{Beat: on, Scope: scope})
	}
	cur := ls[i].Scope
	switch {
	case cur == nil || scope == nil:
		ls[i].Scope = nil
	case cur.Regions == 0 && scope.Regions == 0:
		cur.Within = max(cur.Within, scope.Within)
	case cur.Within == 0 && scope.Within == 0:
		cur.Regions = max(cur.Regions, scope.Regions)
	case *cur != *scope:
		ls[i].Scope = nil
	}
	return ls
}

func (c *checker) act(where string, a Act) {
	n := 0
	for _, set := range []bool{a.Pick != "", a.WakeIn != 0, a.Set != nil, len(a.Clear) > 0, a.Anchor} {
		if set {
			n++
		}
	}
	if n != 1 {
		c.failf(ErrBadAct, where, "an act sets exactly one of pick, wake_in, set, clear, anchor; this sets %d", n)
		return
	}
	switch {
	case a.WakeIn < 0:
		c.failf(ErrBadAct, where, "wake_in %d is not above zero", a.WakeIn)
	case a.Set != nil:
		zero, declared := c.bag[a.Set.Key]
		zk, _ := kindOf(zero)
		vk, ok := kindOf(a.Set.Value)
		switch {
		case !declared:
			c.failf(ErrBadAct, where, "set of key %q the bag does not declare", a.Set.Key)
		case !ok || vk != zk:
			c.failf(ErrBadAct, where, "set of %s key %q to %v (%T)", zk, a.Set.Key, a.Set.Value, a.Set.Value)
		}
	case len(a.Clear) > 0:
		for _, k := range a.Clear {
			if _, declared := c.bag[k]; !declared {
				c.failf(ErrBadAct, where, "clear of key %q the bag does not declare", k)
			}
		}
	}
}

// guard checks g and compiles it for the beat spec; known is false when the
// beat itself was refused, so its leaves are not refused twice.
func (c *checker) guard(g *Guard, spec BeatSpec, known bool, where string) *guard {
	if g == nil {
		return nil
	}
	n := 0
	for _, set := range []bool{
		len(g.All) > 0, len(g.Any) > 0, g.Not != nil,
		g.Fact != nil, g.Flag != "", g.Scope != nil, g.Rounds != nil,
	} {
		if set {
			n++
		}
	}
	if n != 1 {
		c.failf(ErrBadGuard, where,
			"a guard node sets exactly one of all, any, not, fact, flag, scope, rounds; this sets %d", n)
		return nil
	}
	switch {
	case len(g.All) > 0:
		return c.node(nodeAll, g.All, spec, known, where+".all")
	case len(g.Any) > 0:
		return c.node(nodeAny, g.Any, spec, known, where+".any")
	case g.Not != nil:
		kid := c.guard(g.Not, spec, known, where+".not")
		reason := ""
		if kid != nil {
			reason = kid.reason
		}
		return &guard{kind: nodeNot, kids: []*guard{kid}, reason: reason}
	case g.Fact != nil:
		return c.fact(*g.Fact, where+".fact")
	case g.Flag != "":
		if _, ok := c.bag[g.Flag].(bool); !ok {
			c.failf(ErrBadGuard, where+".flag", "flag %q is not a bool the bag declares", g.Flag)
		}
		return &guard{kind: nodeFlag, flag: g.Flag, reason: "flag " + g.Flag}
	case g.Scope != nil:
		return c.scope(*g.Scope, spec, known, where+".scope")
	default:
		return c.rounds(*g.Rounds, spec, known, where+".rounds")
	}
}

func (c *checker) node(kind nodeKind, kids []Guard, spec BeatSpec, known bool, where string) *guard {
	g := &guard{kind: kind, kids: make([]*guard, 0, len(kids))}
	for i := range kids {
		g.kids = append(g.kids, c.guard(&kids[i], spec, known, fmt.Sprintf("%s[%d]", where, i)))
	}
	if g.kids[0] != nil {
		g.reason = g.kids[0].reason
	}
	return g
}

func (c *checker) fact(f FactLeaf, where string) *guard {
	fs, ok := c.v.Facts[f.Name]
	if !ok || fs.Read == nil {
		c.failf(ErrUnknownFact, where, "fact %q is not in the vocabulary", f.Name)
		return nil
	}
	if got, ok := kindOf(f.Value); !ok || got != fs.Kind {
		c.failf(ErrBadGuard, where, "fact %q wants a %s value, got %v (%T)", f.Name, fs.Kind, f.Value, f.Value)
	}
	c.op(where, f.Op, fs.Kind)
	return &guard{kind: nodeFact, read: fs.Read, op: f.Op, value: f.Value, reason: "fact " + string(f.Name)}
}

func (c *checker) scope(s ScopeLeaf, spec BeatSpec, known bool, where string) *guard {
	switch {
	case s.Within < 0 || s.Regions < 0:
		c.failf(ErrBadGuard, where, "within %d and regions %d are not at or above zero", s.Within, s.Regions)
	case s.Within == 0 && s.Regions == 0:
		c.failf(ErrBadGuard, where, "a scope sets within or regions")
	case known && spec.InScope == nil:
		c.failf(ErrNoScope, where, "the beat has no scope")
	}
	return &guard{kind: nodeScope, scope: s, inScope: spec.InScope, reason: "scope"}
}

func (c *checker) rounds(r RoundsLeaf, spec BeatSpec, known bool, where string) *guard {
	if known && spec.Round == nil {
		c.failf(ErrNoRound, where, "the beat carries no round")
	}
	c.op(where, r.Op, KindInt)
	return &guard{kind: nodeRounds, round: spec.Round, op: r.Op, n: r.N, reason: "rounds since anchor"}
}

// op refuses an op the kind does not support.
func (c *checker) op(where string, op Op, kind ScalarKind) {
	switch {
	case op != OpEq && op != OpGte:
		c.failf(ErrBadGuard, where, "op %q is not %q or %q", op, OpEq, OpGte)
	case op == OpGte && kind != KindInt:
		c.failf(ErrBadGuard, where, "op %q does not apply to a %s", op, kind)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// cloneActs copies acts deeply enough that a recipe changed after Compile
// does not reach the program.
func cloneActs(do []Act) []Act {
	if len(do) == 0 {
		return nil
	}
	out := make([]Act, len(do))
	for i, a := range do {
		if a.Set != nil {
			s := *a.Set
			a.Set = &s
		}
		a.Clear = slices.Clone(a.Clear)
		out[i] = a
	}
	return out
}
