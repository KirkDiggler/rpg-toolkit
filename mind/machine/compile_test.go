// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// one is a recipe of one state holding one transition.
func one(t machine.Transition) machine.Recipe {
	return machine.Recipe{
		Ref: testRef("one"), Initial: "s",
		Bag:    machine.Bag{"rung": false, "count": 0},
		States: map[string]machine.State{"s": {Transitions: []machine.Transition{t}}},
	}
}

// on is a transition on beat b with guard g and acts do.
func on(b machine.Beat, g *machine.Guard, do ...machine.Act) machine.Transition {
	return machine.Transition{On: b, When: g, Do: do}
}

func fact(name machine.FactName, op machine.Op, value any) *machine.Guard {
	return &machine.Guard{Fact: &machine.FactLeaf{Name: name, Op: op, Value: value}}
}

func set(key string, value any) machine.Act {
	return machine.Act{Set: &machine.SetAct{Key: key, Value: value}}
}

func states(initial string, s map[string]machine.State) machine.Recipe {
	return machine.Recipe{Ref: testRef("x"), Initial: initial, States: s}
}

func TestCompile_Refusals(t *testing.T) {
	pick := machine.Act{Pick: "x"}
	rung := machine.Guard{Flag: "rung"}
	scope := func(within int) *machine.Guard { return &machine.Guard{Scope: &machine.ScopeLeaf{Within: within}} }
	rounds := func(op machine.Op) *machine.Guard { return &machine.Guard{Rounds: &machine.RoundsLeaf{Op: op, N: 1}} }
	terminal := states("s", map[string]machine.State{
		"s": {Terminal: true, Transitions: []machine.Transition{on(beatTime, nil)}},
	})
	scalarBag := states("s", map[string]machine.State{"s": {}})
	scalarBag.Bag = machine.Bag{"f": 1.5}
	cases := []struct {
		name   string
		recipe machine.Recipe
		want   error
	}{
		// The seven refusals of the contract.
		{"unknown beat", one(on("smell", nil, pick)), machine.ErrUnknownBeat},
		{"unknown fact", one(on(beatTime, fact("mood", machine.OpEq, "sad"))), machine.ErrUnknownFact},
		{"to names no state", one(machine.Transition{On: beatTime, To: "nowhere"}), machine.ErrUnknownState},
		{"initial missing", states("", map[string]machine.State{"s": {}}), machine.ErrNoInitial},
		{"wake_in below zero", one(on(beatTime, nil, machine.Act{WakeIn: -1})), machine.ErrBadAct},
		{"scope on a beat with no scope", one(on(beatBell, scope(2))), machine.ErrNoScope},
		{"rounds on a beat with no round", one(on(beatHeard, rounds(machine.OpGte))), machine.ErrNoRound},

		// The shape around them.
		{"empty act, the wake_in zero", one(on(beatTime, nil, machine.Act{WakeIn: 0})), machine.ErrBadAct},
		{"act sets two fields", one(on(beatTime, nil, machine.Act{Pick: "x", Anchor: true})), machine.ErrBadAct},
		{"set of an undeclared key", one(on(beatTime, nil, set("nope", true))), machine.ErrBadAct},
		{"set of the wrong kind", one(on(beatTime, nil, set("count", "two"))), machine.ErrBadAct},
		{"clear of an undeclared key", one(on(beatTime, nil, machine.Act{Clear: []string{"nope"}})), machine.ErrBadAct},
		{"initial names no state", states("gone", map[string]machine.State{"s": {}}), machine.ErrUnknownState},
		{"guard node sets two fields", one(on(beatTime, &machine.Guard{Flag: "rung", Not: &rung})), machine.ErrBadGuard},
		{"guard node sets none", one(on(beatTime, &machine.Guard{})), machine.ErrBadGuard},
		{"flag names no bool key", one(on(beatTime, &machine.Guard{Flag: "count"})), machine.ErrBadGuard},
		{"fact value of the wrong kind", one(on(beatTime, fact("wounds", machine.OpEq, "two"))), machine.ErrBadGuard},
		{"gte on a string fact", one(on(beatTime, fact("enemy", machine.OpGte, "a"))), machine.ErrBadGuard},
		{"unknown op", one(on(beatTime, rounds("~"))), machine.ErrBadGuard},
		{"scope with no measure", one(on(beatHeard, &machine.Guard{Scope: &machine.ScopeLeaf{}})), machine.ErrBadGuard},
		{"scope below zero", one(on(beatHeard, scope(-1))), machine.ErrBadGuard},
		{"bag value not a scalar", scalarBag, machine.ErrBadBag},
		{"terminal state with transitions", terminal, machine.ErrBadState},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := tc.recipe.Compile(testVocabulary())
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.want)
			assert.Nil(t, p)
		})
	}
}

func TestCompile_ReportsEveryProblem(t *testing.T) {
	r := states("s", map[string]machine.State{"s": {Transitions: []machine.Transition{
		{On: "smell"},
		{On: beatTime, To: "nowhere"},
	}}})
	_, err := r.Compile(testVocabulary())
	assert.ErrorIs(t, err, machine.ErrUnknownBeat)
	assert.ErrorIs(t, err, machine.ErrUnknownState)
	assert.Contains(t, err.Error(), `states.s.transitions[1]`, "an error names where it is")
}

func TestCompile_SinksAndTerminalsCompile(t *testing.T) {
	r := states("s", map[string]machine.State{
		"s":    {Transitions: []machine.Transition{{On: beatBell, To: "done"}, {On: beatHeard, To: "sink"}}},
		"sink": {},
		"done": {Terminal: true},
	})
	p, err := r.Compile(testVocabulary())
	require.NoError(t, err)

	b := p.NewBlob()
	got, err := p.Step(&b, beatBell, nil, nil, 1)
	require.NoError(t, err)
	assert.Equal(t, "done", got.To)

	got, err = p.Step(&b, beatBell, nil, nil, 2)
	require.NoError(t, err)
	assert.False(t, got.Fired, "a terminal state runs nothing")
	_, due := p.DueAt(&b, 2)
	assert.False(t, due)
}

func TestCompile_VocabularyIsCopied(t *testing.T) {
	v := testVocabulary()
	r := one(on(beatTime, fact("wounds", machine.OpGte, 1), machine.Act{Pick: "x"}))
	p, err := r.Compile(v)
	require.NoError(t, err)

	delete(v.Beats, beatTime)
	delete(v.Facts, "wounds")
	b := p.NewBlob()
	got, err := p.Step(&b, beatTime, tick{Round: 1}, env{"wounds": 2}, 1)
	require.NoError(t, err, "a beat deleted from the vocabulary after Compile is still the program's")
	assert.True(t, got.Fired, "and so is a fact")
}

func TestCompile_RecipeIsCopied(t *testing.T) {
	r := guardRecipe()
	p, err := r.Compile(testVocabulary())
	require.NoError(t, err)

	r.States["posted"].Transitions[1].When.Scope.Within = 1
	r.Bag["alarmed"] = true
	b := p.NewBlob()
	assert.Equal(t, 6, p.Listens(&b)[0].Scope.Within, "a compiled program does not see the recipe change after")
	assert.Empty(t, b.Bag)
}
