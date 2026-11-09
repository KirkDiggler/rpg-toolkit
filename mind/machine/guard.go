// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// Guard is a node of a guard tree: all, any or not over child nodes, or one
// leaf. Exactly one field is set per node; Compile refuses any other count.
type Guard struct {
	All []Guard `json:"all,omitzero"`
	Any []Guard `json:"any,omitzero"`
	Not *Guard  `json:"not,omitzero"`

	Fact   *FactLeaf   `json:"fact,omitzero"`
	Flag   string      `json:"flag,omitzero"`
	Scope  *ScopeLeaf  `json:"scope,omitzero"`
	Rounds *RoundsLeaf `json:"rounds,omitzero"`
}

// FactLeaf compares the vocabulary fact Name against Value with Op. A fact
// the vocabulary's reader reports unknown answers Depends.
type FactLeaf struct {
	Name  FactName `json:"name"`
	Op    Op       `json:"op"`
	Value any      `json:"value"`
}

// ScopeLeaf asks the beat's InScope whether its event is near enough to the
// creature asking: within Within hexes, within Regions regions, or both. A
// zero measure is not asked; a leaf sets at least one. What a hex or a
// region is belongs to the host.
type ScopeLeaf struct {
	Within  int `json:"within,omitzero"`
	Regions int `json:"regions,omitzero"`
}

// RoundsLeaf compares the rounds since the blob's anchor (the beat's round
// less the anchor) against N with Op. It answers Depends when the beat
// carries no round or nothing is anchored.
type RoundsLeaf struct {
	Op Op  `json:"op"`
	N  int `json:"n"`
}

// Op is a comparison in a guard leaf.
type Op string

const (
	// OpEq is equality; it applies to bool, int and string values.
	OpEq Op = "eq"
	// OpGte is greater-than-or-equal; it applies to int values only.
	OpGte Op = "gte"
)

// UnmarshalJSONFrom decodes a fact leaf, landing a whole JSON number Value
// as int so it compares with an int fact.
func (f *FactLeaf) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	type plain FactLeaf
	return json.UnmarshalDecode(dec, (*plain)(f), wholeNumbers)
}

// Verdict is a guard's three-valued answer. Depends is not false: a
// transition whose guard depends does not fire, and Not keeps it Depends.
type Verdict int

const (
	// Applies means the guard holds.
	Applies Verdict = iota + 1
	// DoesNotApply means the guard is known not to hold.
	DoesNotApply
	// Depends means the guard cannot be answered from what is known.
	Depends
)

// String names the verdict.
func (v Verdict) String() string {
	switch v {
	case Applies:
		return "applies"
	case DoesNotApply:
		return "does not apply"
	case Depends:
		return "depends"
	}
	return "unanswered"
}

type nodeKind int

const (
	nodeAll nodeKind = iota
	nodeAny
	nodeNot
	nodeFact
	nodeFlag
	nodeScope
	nodeRounds
)

// guard is a Guard bound to its vocabulary and beat. A nil *guard always
// applies.
type guard struct {
	kind   nodeKind
	kids   []*guard
	reason string

	read  func(any) (any, bool)
	op    Op
	value any

	flag string

	scope   ScopeLeaf
	inScope func(any, ScopeLeaf) bool

	round func(any) (uint64, bool)
	n     int
}

// asked is what a guard is answered over: the host's env, the beat's event,
// the blob and the recipe's declared zeros.
type asked struct {
	env, ev any
	blob    *Blob
	zero    Bag
}

// answer evaluates the guard and returns the verdict with the leaf that
// decided it.
//
//   - all is DoesNotApply if any child is, else Depends if any is, else
//     Applies; any is Applies if any child is, else Depends if any is, else
//     DoesNotApply; not swaps Applies and DoesNotApply and keeps Depends.
//   - A flag reads the blob's bag, falling back to the declared zero.
func (g *guard) answer(a *asked, trace *GuardTrace) (verdict Verdict, reason string) {
	if trace != nil {
		trace.Evaluated = true
		if g != nil && len(g.kids) > 0 {
			trace.Children = make([]GuardTrace, len(g.kids))
		}
		defer func() { trace.Verdict, trace.Reason = verdict, reason }()
	}
	if g == nil {
		return Applies, "always"
	}
	switch g.kind {
	case nodeAll:
		return g.fold(a, DoesNotApply, Applies, trace)
	case nodeAny:
		return g.fold(a, Applies, DoesNotApply, trace)
	case nodeNot:
		v, reason := g.kids[0].answer(a, trace.child(0))
		switch v {
		case Applies:
			return DoesNotApply, reason
		case DoesNotApply:
			return Applies, reason
		}
		return v, reason
	case nodeFact:
		got, known := g.read(a.env)
		trace.observe(got, known)
		if !known {
			return Depends, g.reason
		}
		return compare(g.op, got, g.value), g.reason
	case nodeFlag:
		v, ok := a.blob.Bag[g.flag]
		if !ok {
			v = a.zero[g.flag]
		}
		trace.observe(v, true)
		set, ok := v.(bool)
		if !ok {
			return Depends, g.reason
		}
		return verdictOf(set), g.reason
	case nodeScope:
		inside := g.inScope(a.ev, g.scope)
		trace.observe(inside, true)
		return verdictOf(inside), g.reason
	case nodeRounds:
		round, ok := g.round(a.ev)
		if !ok || a.blob.Anchor == nil {
			trace.observe(nil, false)
			return Depends, g.reason
		}
		since := int(int64(round) - int64(*a.blob.Anchor)) //nolint:gosec // rounds are far below the int64 range
		trace.observe(since, true)
		return compare(g.op, since, g.n), g.reason
	}
	return Depends, g.reason
}

// fold runs all (decisive DoesNotApply, otherwise Applies) or any (decisive
// Applies, otherwise DoesNotApply): the first decisive child wins, else the
// first Depends, else the default with the first child's reason.
func (g *guard) fold(a *asked, decisive, otherwise Verdict, trace *GuardTrace) (Verdict, string) {
	dependsOn := ""
	for i, kid := range g.kids {
		v, reason := kid.answer(a, trace.child(i))
		if v == decisive {
			return v, reason
		}
		if v == Depends && dependsOn == "" {
			dependsOn = reason
		}
	}
	if dependsOn != "" {
		return Depends, dependsOn
	}
	return otherwise, g.kids[0].reason
}

// compare applies op to a known value and the leaf's operand. Operands of
// different kinds answer Depends.
func compare(op Op, got, want any) Verdict {
	switch op {
	case OpEq:
		gk, ok1 := kindOf(got)
		wk, ok2 := kindOf(want)
		if !ok1 || !ok2 || gk != wk {
			return Depends
		}
		return verdictOf(got == want)
	case OpGte:
		g, ok1 := got.(int)
		w, ok2 := want.(int)
		if !ok1 || !ok2 {
			return Depends
		}
		return verdictOf(g >= w)
	}
	return Depends
}

func verdictOf(b bool) Verdict {
	if b {
		return Applies
	}
	return DoesNotApply
}

// necessaryScope returns the scope a guard requires of every event it
// applies to: its own scope leaf, or one under a top-level all. A scope
// under any or not is not necessary, and nil is returned.
func necessaryScope(g *Guard) *ScopeLeaf {
	if g == nil {
		return nil
	}
	if g.Scope != nil {
		s := *g.Scope
		return &s
	}
	for i := range g.All {
		if s := necessaryScope(&g.All[i]); s != nil {
			return s
		}
	}
	return nil
}
