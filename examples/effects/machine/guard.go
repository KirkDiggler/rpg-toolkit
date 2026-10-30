// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

// Verdict is a guard's three-valued answer (design E3). Depends is not
// false: a transition whose guard depends does not fire, and an output whose
// guard depends cannot answer.
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
		return "does_not_apply"
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
	nodeRounds
)

// CompiledGuard is a guard bound to its vocabulary: fact readers and the
// beat's round resolved once. A nil *CompiledGuard always applies.
type CompiledGuard struct {
	kind   nodeKind
	kids   []*CompiledGuard
	reason string

	read  func(any) (any, bool)
	op    Op
	value any

	flag string

	rounds RoundsLeaf
	round  func(any) (int, bool)
}

// Answer evaluates the guard over frame (facts), bag (flags and the round
// anchor) and ev (the beat's event, for rounds), and returns the verdict
// with the reason: the leaf that decided it.
//
//   - all is DoesNotApply if any child is, else Depends if any is, else
//     Applies; any is Applies if any child is, else Depends if any is, else
//     DoesNotApply; not swaps Applies and DoesNotApply and keeps Depends.
//   - A fact the reader reports unknown, or known with a value of the wrong
//     type, answers Depends.
//   - A rounds leaf answers Depends when ev carries no round above zero or
//     the bag is not anchored.
func (g *CompiledGuard) Answer(frame any, bag Bag, ev any) (Verdict, string) {
	if g == nil {
		return Applies, "always"
	}
	switch g.kind {
	case nodeAll:
		return g.fold(frame, bag, ev, DoesNotApply, Applies)
	case nodeAny:
		return g.fold(frame, bag, ev, Applies, DoesNotApply)
	case nodeNot:
		v, reason := g.kids[0].Answer(frame, bag, ev)
		switch v {
		case Applies:
			return DoesNotApply, reason
		case DoesNotApply:
			return Applies, reason
		}
		return v, reason
	case nodeFact:
		got, known := g.read(frame)
		if !known {
			return Depends, g.reason
		}
		return compare(g.op, got, g.value), g.reason
	case nodeFlag:
		set, ok := bag[g.flag].(bool)
		if !ok {
			return Depends, g.reason
		}
		return verdictOf(set), g.reason
	case nodeRounds:
		return g.answerRounds(bag, ev), g.reason
	}
	return Depends, g.reason
}

// fold runs all (decisive DoesNotApply, otherwise Applies) or any (decisive
// Applies, otherwise DoesNotApply): the first decisive child wins, else the
// first Depends, else the default with the first child's reason.
func (g *CompiledGuard) fold(frame any, bag Bag, ev any, decisive, otherwise Verdict) (Verdict, string) {
	dependsOn := ""
	for _, kid := range g.kids {
		v, reason := kid.Answer(frame, bag, ev)
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

func (g *CompiledGuard) answerRounds(bag Bag, ev any) Verdict {
	round, ok := g.round(ev)
	anchor, anchored := bag[KeyRoundActivated].(int)
	if !ok || round <= 0 || !anchored || anchor <= 0 {
		return Depends
	}
	if g.rounds.WentBackwards {
		return verdictOf(round < anchor)
	}
	return compare(g.rounds.SinceAnchor.Op, round-anchor, g.rounds.SinceAnchor.N)
}

// compare applies op to a known value and the leaf's operand. Operands of
// different types answer Depends.
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

// guard validates g and compiles it. spec is the beat the guard is asked
// about, nil outside a transition.
func (c *checker) guard(g *Guard, spec *BeatSpec, where string, on Beat) *CompiledGuard {
	if g == nil {
		return nil
	}
	n := 0
	for _, set := range []bool{
		len(g.All) > 0, len(g.Any) > 0, g.Not != nil,
		g.Fact != nil, g.Flag != "", g.Rounds != nil, g.ClassLevel != nil,
	} {
		if set {
			n++
		}
	}
	if n != 1 {
		c.failf("%s: guard node sets %d fields, want exactly 1", where, n)
		return nil
	}
	switch {
	case len(g.All) > 0:
		return c.node(nodeAll, g.All, spec, where, on)
	case len(g.Any) > 0:
		return c.node(nodeAny, g.Any, spec, where, on)
	case g.Not != nil:
		return &CompiledGuard{kind: nodeNot, kids: []*CompiledGuard{c.guard(g.Not, spec, where, on)}}
	case g.Fact != nil:
		return c.fact(g.Fact.Name, g.Fact.Op, g.Fact.Value, where, "fact "+string(g.Fact.Name))
	case g.Flag != "":
		c.key(where, "flag", g.Flag, KindBool)
		return &CompiledGuard{kind: nodeFlag, flag: g.Flag, reason: "flag " + g.Flag}
	case g.Rounds != nil:
		return c.roundsLeaf(*g.Rounds, spec, where, on)
	default:
		cl := g.ClassLevel
		if fs, ok := c.v.Facts[FactName(cl.Class)]; !ok || fs.Kind != KindInt {
			c.failf("%s: class %q is not an int fact in the vocabulary", where, cl.Class)
			return nil
		}
		return c.fact(FactName(cl.Class), cl.Op, cl.N, where, "class_level "+cl.Class)
	}
}

func (c *checker) node(kind nodeKind, kids []Guard, spec *BeatSpec, where string, on Beat) *CompiledGuard {
	cg := &CompiledGuard{kind: kind, kids: make([]*CompiledGuard, 0, len(kids))}
	for i := range kids {
		cg.kids = append(cg.kids, c.guard(&kids[i], spec, where, on))
	}
	if c.err == nil {
		cg.reason = cg.kids[0].reason
	}
	return cg
}

func (c *checker) fact(name FactName, op Op, value any, where, reason string) *CompiledGuard {
	fs, ok := c.v.Facts[name]
	if !ok || fs.Read == nil {
		c.failf("%s: fact %q is not in the vocabulary", where, name)
		return nil
	}
	if got, valid := kindOf(value); !valid || got != fs.Kind {
		c.failf("%s: fact %q wants a %s value, got %T", where, name, fs.Kind, value)
	}
	c.op(where, op, fs.Kind)
	return &CompiledGuard{kind: nodeFact, read: fs.Read, op: op, value: value, reason: reason}
}

// op refuses an op the kind does not support.
func (c *checker) op(where string, op Op, kind ScalarKind) {
	switch {
	case op != OpEq && op != OpGte:
		c.failf("%s: op %q is not %q or %q", where, op, OpEq, OpGte)
	case op == OpGte && kind != KindInt:
		c.failf("%s: op %q does not apply to a %s fact", where, op, kind)
	}
}

func (c *checker) roundsLeaf(leaf RoundsLeaf, spec *BeatSpec, where string, on Beat) *CompiledGuard {
	switch {
	case spec == nil:
		c.failf("%s: a rounds leaf needs a beat", where)
		return nil
	case spec.Round == nil:
		c.failf("%s: beat %q carries no round", where, on)
		return nil
	}
	n := 0
	if leaf.SinceAnchor != nil {
		n++
		c.op(where, leaf.SinceAnchor.Op, KindInt)
	}
	if leaf.WentBackwards {
		n++
	}
	if n != 1 {
		c.failf("%s: rounds leaf sets %d fields, want exactly 1", where, n)
		return nil
	}
	c.key(where, "anchor", KeyRoundActivated, KindInt)
	reason := "rounds.since_anchor"
	if leaf.WentBackwards {
		reason = "rounds.went_backwards"
	}
	return &CompiledGuard{kind: nodeRounds, rounds: leaf, round: spec.Round, reason: reason}
}
