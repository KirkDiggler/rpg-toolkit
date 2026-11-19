// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// refusal_test.go is A REFUSAL SAYS WHERE (rpg-project#997): every refusal
// Compile makes is a *machine.Refusal whose Where is data a caller builds a
// path from, whose kind errors.Is still finds, and whose text is the
// sentence it always was.

// refusalSites is every place Compile refuses, one recipe each.
func refusalSites() []struct {
	name   string
	recipe machine.Recipe
} {
	pick := machine.Act{Pick: "x"}
	rung := machine.Guard{Flag: "rung"}
	scope := func(within int) *machine.Guard { return &machine.Guard{Scope: &machine.ScopeLeaf{Within: within}} }
	rounds := func(op machine.Op) *machine.Guard { return &machine.Guard{Rounds: &machine.RoundsLeaf{Op: op, N: 1}} }
	terminal := states("s", map[string]machine.State{"s": {Terminal: true, Transitions: []machine.Transition{on(beatTime, nil)}}})
	scalarBag := states("s", map[string]machine.State{"s": {}})
	scalarBag.Bag = machine.Bag{"f": 1.5}
	nested := one(on(beatTime, &machine.Guard{All: []machine.Guard{{Flag: "rung"}, {Not: &machine.Guard{Flag: "count"}}}}))
	return []struct {
		name   string
		recipe machine.Recipe
	}{
		{"unknown beat", one(on("smell", nil, pick))},
		{"unknown fact", one(on(beatTime, fact("mood", machine.OpEq, "sad")))},
		{"to names no state", one(machine.Transition{On: beatTime, To: "nowhere"})},
		{"initial missing", states("", map[string]machine.State{"s": {}})},
		{"initial names no state", states("gone", map[string]machine.State{"s": {}})},
		{"wake_in below zero", one(on(beatTime, nil, machine.Act{WakeIn: -1}))},
		{"act sets none", one(on(beatTime, nil, machine.Act{WakeIn: 0}))},
		{"act sets two fields", one(on(beatTime, nil, machine.Act{Pick: "x", Anchor: true}))},
		{"set of an undeclared key", one(on(beatTime, nil, pick, set("nope", true)))},
		{"set of the wrong kind", one(on(beatTime, nil, set("count", "two")))},
		{"clear of an undeclared key", one(on(beatTime, nil, machine.Act{Clear: []string{"nope"}}))},
		{"scope on a beat with no scope", one(on(beatBell, scope(2)))},
		{"rounds on a beat with no round", one(on(beatHeard, rounds(machine.OpGte)))},
		{"guard node sets two fields", one(on(beatTime, &machine.Guard{Flag: "rung", Not: &rung}))},
		{"guard node sets none", one(on(beatTime, &machine.Guard{}))},
		{"flag names no bool key", one(on(beatTime, &machine.Guard{Flag: "count"}))},
		{"flag nested under all and not", nested},
		{"fact value of the wrong kind", one(on(beatTime, fact("wounds", machine.OpEq, "two")))},
		{"gte on a string fact", one(on(beatTime, fact("enemy", machine.OpGte, "a")))},
		{"unknown op", one(on(beatTime, rounds("~")))},
		{"scope with no measure", one(on(beatHeard, &machine.Guard{Scope: &machine.ScopeLeaf{}}))},
		{"scope below zero", one(on(beatHeard, scope(-1)))},
		{"bag value not a scalar", scalarBag},
		{"terminal state with transitions", terminal},
	}
}

// wantRefusal is what one site's refusal is: its kind, its place and its
// sentence, pinned as it read before refusals carried their place.
var wantRefusal = map[string]struct {
	kind  error
	where machine.Where
	text  string
}{
	"unknown beat": {machine.ErrUnknownBeat, machine.Where{State: "s", Transition: 0},
		`machine: unknown beat: states.s.transitions[0]: beat "smell" is not in the vocabulary`},
	"unknown fact": {machine.ErrUnknownFact, machine.Where{State: "s", Transition: 0, Leaf: "when.fact"},
		`machine: unknown fact: states.s.transitions[0].when.fact: fact "mood" is not in the vocabulary`},
	"to names no state": {machine.ErrUnknownState, machine.Where{State: "s", Transition: 0},
		`machine: unknown state: states.s.transitions[0]: to "nowhere" is not a state of the recipe`},
	"initial missing": {machine.ErrNoInitial, machine.Where{Initial: true, Transition: -1},
		`machine: no initial state: initial: the recipe names no initial state`},
	"initial names no state": {machine.ErrUnknownState, machine.Where{Initial: true, Transition: -1},
		`machine: unknown state: initial: state "gone" is not in the recipe`},
	"wake_in below zero": {machine.ErrBadAct, machine.Where{State: "s", Transition: 0, Leaf: "do[0]"},
		`machine: bad act: states.s.transitions[0].do[0]: wake_in -1 is not above zero`},
	"act sets none": {machine.ErrBadAct, machine.Where{State: "s", Transition: 0, Leaf: "do[0]"},
		`machine: bad act: states.s.transitions[0].do[0]: an act sets exactly one of pick, wake_in, set, clear, anchor; this sets 0`},
	"act sets two fields": {machine.ErrBadAct, machine.Where{State: "s", Transition: 0, Leaf: "do[0]"},
		`machine: bad act: states.s.transitions[0].do[0]: an act sets exactly one of pick, wake_in, set, clear, anchor; this sets 2`},
	"set of an undeclared key": {machine.ErrBadAct, machine.Where{State: "s", Transition: 0, Leaf: "do[1]"},
		`machine: bad act: states.s.transitions[0].do[1]: set of key "nope" the bag does not declare`},
	"set of the wrong kind": {machine.ErrBadAct, machine.Where{State: "s", Transition: 0, Leaf: "do[0]"},
		`machine: bad act: states.s.transitions[0].do[0]: set of int key "count" to two (string)`},
	"clear of an undeclared key": {machine.ErrBadAct, machine.Where{State: "s", Transition: 0, Leaf: "do[0]"},
		`machine: bad act: states.s.transitions[0].do[0]: clear of key "nope" the bag does not declare`},
	"scope on a beat with no scope": {machine.ErrNoScope, machine.Where{State: "s", Transition: 0, Leaf: "when.scope"},
		`machine: beat has no scope: states.s.transitions[0].when.scope: the beat has no scope`},
	"rounds on a beat with no round": {machine.ErrNoRound, machine.Where{State: "s", Transition: 0, Leaf: "when.rounds"},
		`machine: beat carries no round: states.s.transitions[0].when.rounds: the beat carries no round`},
	"guard node sets two fields": {machine.ErrBadGuard, machine.Where{State: "s", Transition: 0, Leaf: "when"},
		`machine: bad guard: states.s.transitions[0].when: a guard node sets exactly one of all, any, not, fact, flag, scope, rounds; this sets 2`},
	"guard node sets none": {machine.ErrBadGuard, machine.Where{State: "s", Transition: 0, Leaf: "when"},
		`machine: bad guard: states.s.transitions[0].when: a guard node sets exactly one of all, any, not, fact, flag, scope, rounds; this sets 0`},
	"flag names no bool key": {machine.ErrBadGuard, machine.Where{State: "s", Transition: 0, Leaf: "when.flag"},
		`machine: bad guard: states.s.transitions[0].when.flag: flag "count" is not a bool the bag declares`},
	"flag nested under all and not": {machine.ErrBadGuard, machine.Where{State: "s", Transition: 0, Leaf: "when.all[1].not.flag"},
		`machine: bad guard: states.s.transitions[0].when.all[1].not.flag: flag "count" is not a bool the bag declares`},
	"fact value of the wrong kind": {machine.ErrBadGuard, machine.Where{State: "s", Transition: 0, Leaf: "when.fact"},
		`machine: bad guard: states.s.transitions[0].when.fact: fact "wounds" wants a int value, got two (string)`},
	"gte on a string fact": {machine.ErrBadGuard, machine.Where{State: "s", Transition: 0, Leaf: "when.fact"},
		`machine: bad guard: states.s.transitions[0].when.fact: op "gte" does not apply to a string`},
	"unknown op": {machine.ErrBadGuard, machine.Where{State: "s", Transition: 0, Leaf: "when.rounds"},
		`machine: bad guard: states.s.transitions[0].when.rounds: op "~" is not "eq" or "gte"`},
	"scope with no measure": {machine.ErrBadGuard, machine.Where{State: "s", Transition: 0, Leaf: "when.scope"},
		`machine: bad guard: states.s.transitions[0].when.scope: a scope sets within or regions`},
	"scope below zero": {machine.ErrBadGuard, machine.Where{State: "s", Transition: 0, Leaf: "when.scope"},
		`machine: bad guard: states.s.transitions[0].when.scope: within -1 and regions 0 are not at or above zero`},
	"bag value not a scalar": {machine.ErrBadBag, machine.Where{Bag: "f", Transition: -1},
		`machine: bad bag: bag.f: value 1.5 (float64) is not a bool, int or string`},
	"terminal state with transitions": {machine.ErrBadState, machine.Where{State: "s", Transition: -1},
		`machine: bad state: states.s: a terminal state carries no transitions`},
}

// refusals is every *machine.Refusal in a joined Compile error, in order.
func refusals(t *testing.T, err error) []*machine.Refusal {
	t.Helper()
	joined, ok := err.(interface{ Unwrap() []error })
	require.True(t, ok, "Compile joins its refusals: %v", err)
	errs := joined.Unwrap()
	out := make([]*machine.Refusal, 0, len(errs))
	for _, e := range errs {
		var r *machine.Refusal
		require.True(t, errors.As(e, &r), "every refusal is a *machine.Refusal: %v", e)
		out = append(out, r)
	}
	return out
}

func TestRefusal_EverySiteSaysWhereAsData(t *testing.T) {
	sites := refusalSites()
	require.Len(t, wantRefusal, len(sites), "every site has its pinned refusal")
	for _, site := range sites {
		t.Run(site.name, func(t *testing.T) {
			want, ok := wantRefusal[site.name]
			require.True(t, ok)
			_, err := site.recipe.Compile(testVocabulary())
			require.Error(t, err)
			assert.Equal(t, want.text, err.Error(), "the sentence is the one it always was")
			got := refusals(t, err)
			require.Len(t, got, 1)
			assert.Equal(t, want.where, got[0].Where)
			assert.Equal(t, want.kind, got[0].Kind)
			assert.ErrorIs(t, err, want.kind, "errors.Is still finds the kind")
			assert.Equal(t, want.text, fmt.Sprintf("%v: %s: %s", got[0].Kind, got[0].Where, got[0].Message),
				"the place the text names is Where's own rendering")
		})
	}
}

// TestRefusal_EveryKindIsCovered: the sites above trip every refusal kind
// Compile has, so a kind without a placed refusal cannot hide.
func TestRefusal_EveryKindIsCovered(t *testing.T) {
	seen := map[error]bool{}
	for _, w := range wantRefusal {
		seen[w.kind] = true
	}
	for _, kind := range []error{
		machine.ErrUnknownBeat, machine.ErrUnknownFact, machine.ErrUnknownState, machine.ErrNoInitial, machine.ErrBadAct,
		machine.ErrNoScope, machine.ErrNoRound, machine.ErrBadGuard, machine.ErrBadBag, machine.ErrBadState,
	} {
		assert.True(t, seen[kind], "a site trips %v", kind)
	}
}

// TestRefusal_EveryRefusalHasAPlace trips every site in one recipe set and
// walks the joined errors: each is a *machine.Refusal naming exactly one
// place — a bag key, the initial state, or a state (and a step when its
// transition is not -1). A refusal made without a place fails here.
func TestRefusal_EveryRefusalHasAPlace(t *testing.T) {
	for _, site := range refusalSites() {
		_, err := site.recipe.Compile(testVocabulary())
		for _, r := range refusals(t, err) {
			places := 0
			for _, set := range []bool{r.Where.Bag != "", r.Where.Initial, r.Where.State != ""} {
				if set {
					places++
				}
			}
			assert.Equal(t, 1, places, "%s: exactly one place: %+v", site.name, r.Where)
			if r.Where.State == "" || r.Where.Transition < 0 {
				assert.Empty(t, r.Where.Leaf, "%s: a leaf is only inside a step", site.name)
			}
			assert.NotEmpty(t, r.Message, site.name)
			assert.NotNil(t, r.Kind, site.name)
		}
	}
}

// TestRefusal_TwoBadStepsNameTheirOwn: two refused steps in one state are
// two refusals, each naming its own step and leaf.
func TestRefusal_TwoBadStepsNameTheirOwn(t *testing.T) {
	r := states("s", map[string]machine.State{"s": {Transitions: []machine.Transition{
		{On: "smell"},
		on(beatTime, fact("mood", machine.OpEq, "sad")),
	}}})
	_, err := r.Compile(testVocabulary())
	got := refusals(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, machine.Where{State: "s", Transition: 0}, got[0].Where)
	assert.ErrorIs(t, got[0], machine.ErrUnknownBeat)
	assert.Equal(t, machine.Where{State: "s", Transition: 1, Leaf: "when.fact"}, got[1].Where)
	assert.ErrorIs(t, got[1], machine.ErrUnknownFact)
}

func TestWhere_String(t *testing.T) {
	for _, tc := range []struct {
		where machine.Where
		want  string
	}{
		{machine.Where{Bag: "count", Transition: -1}, "bag.count"},
		{machine.Where{Initial: true, Transition: -1}, "initial"},
		{machine.Where{State: "s", Transition: -1}, "states.s"},
		{machine.Where{State: "s", Transition: 2}, "states.s.transitions[2]"},
		{machine.Where{State: "s", Transition: 0, Leaf: "do[1]"}, "states.s.transitions[0].do[1]"},
		{machine.Where{State: "s", Transition: 3, Leaf: "when.any[0].not.fact"}, "states.s.transitions[3].when.any[0].not.fact"},
	} {
		assert.Equal(t, tc.want, tc.where.String())
	}
}

// TestRefusal_EveryCallSiteIsPlacedByAConstructor reads compile.go: every
// refusal is made with a place from the Where constructors (atBag,
// atInitial, atState, atStep) or one derived from them (`where`), never a
// hand-spelled Where literal, so a new refusal cannot be made placeless.
func TestRefusal_EveryCallSiteIsPlacedByAConstructor(t *testing.T) {
	src, err := os.ReadFile("compile.go")
	require.NoError(t, err)
	calls := regexp.MustCompile(`c\.failf\(\s*\w+,\s*([^,]+),`).FindAllStringSubmatch(string(src), -1)
	require.NotEmpty(t, calls)
	placed := regexp.MustCompile(`^(atBag\(|atInitial\(\)|atState\(|atStep\(|where\b)`)
	for _, c := range calls {
		assert.Regexp(t, placed, c[1], "a refusal's place is made by a constructor: %s", c[0])
	}
}
