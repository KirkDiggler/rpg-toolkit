// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
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

// place is what a refusal's Where should say, field by field: the kind
// only the package's constructors can set, and the fields that kind uses.
type place struct {
	kind       machine.PlaceKind
	bag        string
	state      string
	transition int
	leaf       []machine.Segment
}

// placeOf reads a Where into a place.
func placeOf(w machine.Where) place {
	return place{kind: w.Kind(), bag: w.Bag, state: w.State, transition: w.Transition, leaf: w.Leaf}
}

// wantRefusal is what one site's refusal is: its kind, its place and its
// sentence, pinned as it read before refusals carried their place.
var wantRefusal = map[string]struct {
	kind  error
	where place
	text  string
}{
	"unknown beat": {machine.ErrUnknownBeat, place{kind: machine.PlaceStep, state: "s", transition: 0},
		`machine: unknown beat: states.s.transitions[0]: beat "smell" is not in the vocabulary`},
	"unknown fact": {machine.ErrUnknownFact, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "when"}, {Name: "fact"}}},
		`machine: unknown fact: states.s.transitions[0].when.fact: fact "mood" is not in the vocabulary`},
	"to names no state": {machine.ErrUnknownState, place{kind: machine.PlaceStep, state: "s", transition: 0},
		`machine: unknown state: states.s.transitions[0]: to "nowhere" is not a state of the recipe`},
	"initial missing": {machine.ErrNoInitial, place{kind: machine.PlaceInitial},
		`machine: no initial state: initial: the recipe names no initial state`},
	"initial names no state": {machine.ErrUnknownState, place{kind: machine.PlaceInitial},
		`machine: unknown state: initial: state "gone" is not in the recipe`},
	"wake_in below zero": {machine.ErrBadAct, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "do", Index: 0, Indexed: true}}},
		`machine: bad act: states.s.transitions[0].do[0]: wake_in -1 is not above zero`},
	"act sets none": {machine.ErrBadAct, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "do", Index: 0, Indexed: true}}},
		`machine: bad act: states.s.transitions[0].do[0]: an act sets exactly one of pick, wake_in, set, clear, anchor; this sets 0`},
	"act sets two fields": {machine.ErrBadAct, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "do", Index: 0, Indexed: true}}},
		`machine: bad act: states.s.transitions[0].do[0]: an act sets exactly one of pick, wake_in, set, clear, anchor; this sets 2`},
	"set of an undeclared key": {machine.ErrBadAct, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "do", Index: 1, Indexed: true}}},
		`machine: bad act: states.s.transitions[0].do[1]: set of key "nope" the bag does not declare`},
	"set of the wrong kind": {machine.ErrBadAct, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "do", Index: 0, Indexed: true}}},
		`machine: bad act: states.s.transitions[0].do[0]: set of int key "count" to two (string)`},
	"clear of an undeclared key": {machine.ErrBadAct, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "do", Index: 0, Indexed: true}}},
		`machine: bad act: states.s.transitions[0].do[0]: clear of key "nope" the bag does not declare`},
	"scope on a beat with no scope": {machine.ErrNoScope, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "when"}, {Name: "scope"}}},
		`machine: beat has no scope: states.s.transitions[0].when.scope: the beat has no scope`},
	"rounds on a beat with no round": {machine.ErrNoRound, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "when"}, {Name: "rounds"}}},
		`machine: beat carries no round: states.s.transitions[0].when.rounds: the beat carries no round`},
	"guard node sets two fields": {machine.ErrBadGuard, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "when"}}},
		`machine: bad guard: states.s.transitions[0].when: a guard node sets exactly one of all, any, not, fact, flag, scope, rounds; this sets 2`},
	"guard node sets none": {machine.ErrBadGuard, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "when"}}},
		`machine: bad guard: states.s.transitions[0].when: a guard node sets exactly one of all, any, not, fact, flag, scope, rounds; this sets 0`},
	"flag names no bool key": {machine.ErrBadGuard, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "when"}, {Name: "flag"}}},
		`machine: bad guard: states.s.transitions[0].when.flag: flag "count" is not a bool the bag declares`},
	"flag nested under all and not": {machine.ErrBadGuard, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "when"}, {Name: "all", Index: 1, Indexed: true}, {Name: "not"}, {Name: "flag"}}},
		`machine: bad guard: states.s.transitions[0].when.all[1].not.flag: flag "count" is not a bool the bag declares`},
	"fact value of the wrong kind": {machine.ErrBadGuard, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "when"}, {Name: "fact"}}},
		`machine: bad guard: states.s.transitions[0].when.fact: fact "wounds" wants a int value, got two (string)`},
	"gte on a string fact": {machine.ErrBadGuard, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "when"}, {Name: "fact"}}},
		`machine: bad guard: states.s.transitions[0].when.fact: op "gte" does not apply to a string`},
	"unknown op": {machine.ErrBadGuard, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "when"}, {Name: "rounds"}}},
		`machine: bad guard: states.s.transitions[0].when.rounds: op "~" is not "eq" or "gte"`},
	"scope with no measure": {machine.ErrBadGuard, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "when"}, {Name: "scope"}}},
		`machine: bad guard: states.s.transitions[0].when.scope: a scope sets within or regions`},
	"scope below zero": {machine.ErrBadGuard, place{kind: machine.PlaceStep, state: "s", transition: 0, leaf: []machine.Segment{{Name: "when"}, {Name: "scope"}}},
		`machine: bad guard: states.s.transitions[0].when.scope: within -1 and regions 0 are not at or above zero`},
	"bag value not a scalar": {machine.ErrBadBag, place{kind: machine.PlaceBag, bag: "f"},
		`machine: bad bag: bag.f: value 1.5 (float64) is not a bool, int or string`},
	"terminal state with transitions": {machine.ErrBadState, place{kind: machine.PlaceState, state: "s"},
		`machine: bad state: states.s: a terminal state carries no transitions`},
}

// refusals is every refusal in a Compile error, by the package's own walk,
// after checking each joined error is a *machine.Refusal.
func refusals(t *testing.T, err error) []*machine.Refusal {
	t.Helper()
	joined, ok := err.(interface{ Unwrap() []error })
	require.True(t, ok, "Compile joins its refusals: %v", err)
	for _, e := range joined.Unwrap() {
		var r *machine.Refusal
		require.True(t, errors.As(e, &r), "every refusal is a *machine.Refusal: %v", e)
	}
	return machine.Refusals(err)
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
			assert.Equal(t, want.where, placeOf(got[0].Where))
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

// TestRefusal_EveryRefusalHasAPlace trips every site and walks every
// refusal: each names a place the package made — never [machine.PlaceNone],
// never "(no place)" — and only the fields its kind uses.
func TestRefusal_EveryRefusalHasAPlace(t *testing.T) {
	for _, site := range refusalSites() {
		_, err := site.recipe.Compile(testVocabulary())
		for _, r := range refusals(t, err) {
			w := r.Where
			assert.NotEqual(t, machine.PlaceNone, w.Kind(), site.name)
			assert.NotEqual(t, "(no place)", w.String(), site.name)
			assert.Equal(t, w.Kind() == machine.PlaceBag, w.Bag != "", "%s: a bag key only on a bag place", site.name)
			assert.Equal(t, w.Kind() == machine.PlaceState || w.Kind() == machine.PlaceStep, w.State != "", "%s: a state only on a state or step", site.name)
			if w.Kind() != machine.PlaceStep {
				assert.Empty(t, w.Leaf, "%s: a leaf is only inside a step", site.name)
				assert.Zero(t, w.Transition, "%s: a step index only on a step", site.name)
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
	assert.Equal(t, place{kind: machine.PlaceStep, state: "s", transition: 0}, placeOf(got[0].Where))
	assert.ErrorIs(t, got[0], machine.ErrUnknownBeat)
	assert.Equal(t, place{kind: machine.PlaceStep, state: "s", transition: 1,
		leaf: []machine.Segment{{Name: "when"}, {Name: "fact"}}}, placeOf(got[1].Where))
	assert.ErrorIs(t, got[1], machine.ErrUnknownFact)
}

// TestWhere_NoPlace: a Where the package did not make is no place, however
// its fields are set: it renders as no path, and reads PlaceNone.
func TestWhere_NoPlace(t *testing.T) {
	for _, w := range []machine.Where{{}, {State: "s"}, {State: "s", Transition: 2}, {Bag: "count"}} {
		assert.Equal(t, machine.PlaceNone, w.Kind())
		assert.Equal(t, "(no place)", w.String())
	}
}

// TestSegment_String: each kind of segment renders as the path spells it.
func TestSegment_String(t *testing.T) {
	assert.Equal(t, "when", machine.Segment{Name: "when"}.String())
	assert.Equal(t, "all[1]", machine.Segment{Name: "all", Index: 1, Indexed: true}.String())
	assert.Equal(t, "do[0]", machine.Segment{Name: "do", Index: 0, Indexed: true}.String())
}

// TestRefusal_ADeepNestNamesEverySegment: a refusal deep in an all, an any
// and a not names each key it passed, and renders the path the text shows.
func TestRefusal_ADeepNestNamesEverySegment(t *testing.T) {
	deep := one(on(beatTime, &machine.Guard{All: []machine.Guard{
		{Flag: "rung"},
		{Any: []machine.Guard{{Flag: "rung"}, {Not: &machine.Guard{Flag: "count"}}}},
	}}))
	_, err := deep.Compile(testVocabulary())
	got := machine.Refusals(err)
	require.Len(t, got, 1)
	assert.Equal(t, []machine.Segment{
		{Name: "when"}, {Name: "all", Index: 1, Indexed: true}, {Name: "any", Index: 1, Indexed: true}, {Name: "not"}, {Name: "flag"},
	}, got[0].Where.Leaf)
	assert.Equal(t, "states.s.transitions[0].when.all[1].any[1].not.flag", got[0].Where.String())
}

// TestRefusals_AllOfThemInOrder: a five-refusal compile gives five, in the
// order its text lists them; nil and an error with none give none; a
// refusal wrapped by a caller is still found.
func TestRefusals_AllOfThemInOrder(t *testing.T) {
	r := machine.Recipe{Ref: testRef("five"), Initial: "gone", Bag: machine.Bag{"f": 1.5},
		States: map[string]machine.State{"s": {Transitions: []machine.Transition{
			{On: "smell"},
			on(beatTime, fact("mood", machine.OpEq, "sad")),
			{On: beatTime, To: "nowhere"},
		}}}}
	_, err := r.Compile(testVocabulary())
	got := machine.Refusals(err)
	require.Len(t, got, 5)
	lines := strings.Split(err.Error(), "\n")
	require.Len(t, lines, len(got))
	for i, ref := range got {
		assert.Equal(t, lines[i], ref.Error(), "refusal %d is the text's line %d", i, i)
	}
	assert.Empty(t, machine.Refusals(nil))
	assert.Empty(t, machine.Refusals(errors.New("not a machine refusal")))
	assert.Len(t, machine.Refusals(fmt.Errorf("behaviors %q: %w", "policy", err)), 5, "a caller's wrap is walked through")
	assert.Equal(t, []machine.PlaceKind{machine.PlaceBag, machine.PlaceInitial, machine.PlaceStep, machine.PlaceStep, machine.PlaceStep},
		[]machine.PlaceKind{got[0].Where.Kind(), got[1].Where.Kind(), got[2].Where.Kind(), got[3].Where.Kind(), got[4].Where.Kind()})
}

// TestRefusal_OnlyTheConstructorsMakeAPlace reads the package's source: a
// place's kind can be set only by a `Where{kind: …}` literal, and the only
// such literals are in the four constructors of refusal.go. Anything else
// that builds a Where gets PlaceNone, which TestRefusal_EveryRefusalHasAPlace
// refuses for every refusal the compiler makes.
func TestRefusal_OnlyTheConstructorsMakeAPlace(t *testing.T) {
	files, err := os.ReadDir(".")
	require.NoError(t, err)
	literal := regexp.MustCompile(`Where\{\s*kind:`)
	constructor := regexp.MustCompile(`^func at(Bag|Initial|State|Step)\(`)
	for _, f := range files {
		name := f.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		require.NoError(t, err)
		for n, line := range strings.Split(string(src), "\n") {
			if literal.MatchString(line) {
				assert.Regexp(t, constructor, line, "%s:%d: a place's kind is set only by a constructor", name, n+1)
			}
		}
	}
}
