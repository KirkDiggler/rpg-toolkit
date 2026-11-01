// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

func u64(n uint64) *uint64 { return &n }

// twoOnTime is one state with two transitions on time, both of which would
// apply, behind a transition on another beat that is not asked.
func twoOnTime(first *machine.Guard) machine.Recipe {
	return machine.Recipe{
		Ref:     testRef("two"),
		Initial: "s",
		States: map[string]machine.State{
			"s": {Transitions: []machine.Transition{
				{On: beatBell, Do: []machine.Act{{Pick: "bell"}}},
				{On: beatTime, When: first, Do: []machine.Act{{Pick: "first"}}, Reason: "first"},
				{On: beatTime, To: "t", Do: []machine.Act{{Pick: "second"}}, Reason: "second"},
			}},
			"t": {},
		},
	}
}

func TestStep_FirstFiringWins(t *testing.T) {
	p := mustCompile(twoOnTime(nil))
	b := p.NewBlob()

	got, err := p.Step(&b, beatTime, tick{Round: 1}, nil, 1)
	require.NoError(t, err)

	assert.True(t, got.Fired)
	assert.Equal(t, []machine.Act{{Pick: "first"}}, got.Acts,
		"the first applying transition on the beat runs, the bell one is not asked")
	assert.Equal(t, "first", got.Reason)
	assert.Empty(t, got.From, "the first transition names no state")
	assert.Empty(t, got.To)
	assert.Equal(t, "s", b.State)
}

func TestStep_GuardDependsDoesNotFire(t *testing.T) {
	// enemy is unknown in an empty env: the first guard depends and is
	// passed over, the second fires.
	p := mustCompile(twoOnTime(&machine.Guard{Fact: &machine.FactLeaf{Name: "enemy", Op: machine.OpEq, Value: "seen"}}))
	b := p.NewBlob()

	got, err := p.Step(&b, beatTime, tick{Round: 1}, env{}, 1)
	require.NoError(t, err)
	assert.Equal(t, []machine.Act{{Pick: "second"}}, got.Acts)
	assert.Equal(t, "s", got.From)
	assert.Equal(t, "t", got.To)
	assert.Equal(t, "t", b.State)

	// A lone guard that depends fires nothing, and says why.
	only := machine.Recipe{
		Ref: testRef("only"), Initial: "s",
		States: map[string]machine.State{"s": {Transitions: []machine.Transition{{
			On:   beatTime,
			When: &machine.Guard{Fact: &machine.FactLeaf{Name: "wounds", Op: machine.OpGte, Value: 2}},
			Do:   []machine.Act{{Pick: "flee"}},
		}}}},
	}
	p = mustCompile(only)
	b = p.NewBlob()
	got, err = p.Step(&b, beatTime, tick{Round: 1}, env{}, 1)
	require.NoError(t, err)
	assert.False(t, got.Fired)
	assert.Empty(t, got.Acts)
	assert.Contains(t, got.Reason, "depends")
	assert.Contains(t, got.Reason, "wounds")

	// Known, the same guard fires.
	got, err = p.Step(&b, beatTime, tick{Round: 1}, env{"wounds": 3}, 1)
	require.NoError(t, err)
	assert.True(t, got.Fired)
}

func TestStep_WakeInArmsAndTimedClears(t *testing.T) {
	r := machine.Recipe{
		Ref: testRef("dwell"), Initial: "walk",
		States: map[string]machine.State{
			"walk": {Transitions: []machine.Transition{
				{On: beatBell, Do: []machine.Act{{WakeIn: 3}}},
				on(beatTime, fact("lit", machine.OpEq, true), machine.Act{Pick: "time"}),
			}},
		},
	}
	p := mustCompile(r)
	b := p.NewBlob()

	got, err := p.Step(&b, beatBell, nil, nil, 5)
	require.NoError(t, err)
	assert.Equal(t, []machine.Act{{WakeIn: 3}}, got.Acts, "wake_in is returned with the acts")
	require.NotNil(t, b.WakeAt)
	assert.Equal(t, uint64(8), *b.WakeAt, "wake_in arms now + n")

	_, err = p.Step(&b, beatTime, tick{Round: 7}, env{"lit": true}, 7)
	require.NoError(t, err)
	require.NotNil(t, b.WakeAt, "a timed beat before the wake leaves it armed")

	// At the wake round the timed beat clears it, even though the guard
	// does not apply: a stale wake would be due forever.
	got, err = p.Step(&b, beatTime, tick{Round: 8}, env{"lit": false}, 8)
	require.NoError(t, err)
	assert.False(t, got.Fired)
	assert.Nil(t, b.WakeAt, "a timed beat at or after the wake clears it")

	// A non-timed beat never clears a wake.
	_, err = p.Step(&b, beatBell, nil, nil, 10)
	require.NoError(t, err)
	_, err = p.Step(&b, beatBell, nil, nil, 20)
	require.NoError(t, err)
	require.NotNil(t, b.WakeAt)
	assert.Equal(t, uint64(23), *b.WakeAt, "a second wake_in re-arms from its own now")
}

func TestStep_AnchorSetClearAndRounds(t *testing.T) {
	p := mustCompile(guardRecipe())
	b := p.NewBlob()

	got, err := p.Step(&b, beatHeard, sound{Hexes: 4}, nil, 10)
	require.NoError(t, err)
	assert.Equal(t, "posted", got.From)
	assert.Equal(t, "alert", got.To)
	assert.Equal(t, "heard something", got.Reason)
	require.NotNil(t, b.Anchor)
	assert.Equal(t, uint64(10), *b.Anchor, "anchor stamps now")
	assert.Equal(t, true, b.Bag["alarmed"])

	got, err = p.Step(&b, beatTime, tick{Round: 12}, nil, 12)
	require.NoError(t, err)
	assert.Equal(t, []machine.Act{{Pick: "alert"}}, got.Acts, "two rounds since the anchor: still alert")

	got, err = p.Step(&b, beatTime, tick{Round: 13}, nil, 13)
	require.NoError(t, err)
	assert.Equal(t, "posted", got.To, "three rounds since the anchor: back to the post")
	_, stored := b.Bag["alarmed"]
	assert.False(t, stored, "clear returns the key to its declared zero, which the blob does not carry")
}

func TestStep_FlagReadsTheBag(t *testing.T) {
	r := machine.Recipe{
		Ref: testRef("flag"), Initial: "s",
		Bag: machine.Bag{"rung": false},
		States: map[string]machine.State{"s": {Transitions: []machine.Transition{
			{On: beatBell, Do: []machine.Act{{Set: &machine.SetAct{Key: "rung", Value: true}}}},
			{On: beatTime, When: &machine.Guard{Flag: "rung"}, Do: []machine.Act{{Pick: "run"}}},
		}}},
	}
	p := mustCompile(r)
	b := p.NewBlob()

	got, err := p.Step(&b, beatTime, tick{Round: 1}, nil, 1)
	require.NoError(t, err)
	assert.False(t, got.Fired, "a declared flag at its zero does not apply")

	_, err = p.Step(&b, beatBell, nil, nil, 1)
	require.NoError(t, err)
	got, err = p.Step(&b, beatTime, tick{Round: 2}, nil, 2)
	require.NoError(t, err)
	assert.True(t, got.Fired)
}

// Mutation-style: flipping the first guard to its Not changes which
// transition fires. If Not were ignored, or the first transition were not
// the one asked first, one of these two would fail.
func TestStep_NotFlipsTheFiredTransition(t *testing.T) {
	lit := machine.Guard{Fact: &machine.FactLeaf{Name: "lit", Op: machine.OpEq, Value: true}}
	e := env{"lit": true}

	p := mustCompile(twoOnTime(&lit))
	b := p.NewBlob()
	got, err := p.Step(&b, beatTime, tick{Round: 1}, e, 1)
	require.NoError(t, err)
	assert.Equal(t, "first", got.Reason)

	p = mustCompile(twoOnTime(&machine.Guard{Not: &lit}))
	b = p.NewBlob()
	got, err = p.Step(&b, beatTime, tick{Round: 1}, e, 1)
	require.NoError(t, err)
	assert.Equal(t, "second", got.Reason)
}

func TestStep_AllAnyFold(t *testing.T) {
	lit := machine.Guard{Fact: &machine.FactLeaf{Name: "lit", Op: machine.OpEq, Value: true}}
	unknown := machine.Guard{Fact: &machine.FactLeaf{Name: "enemy", Op: machine.OpEq, Value: "seen"}}
	dark := machine.Guard{Not: &lit}
	cases := []struct {
		name  string
		guard machine.Guard
		fires bool
	}{
		{"all of applies fires", machine.Guard{All: []machine.Guard{lit, lit}}, true},
		{"all with a does-not-apply does not", machine.Guard{All: []machine.Guard{unknown, dark}}, false},
		{"all with a depends does not", machine.Guard{All: []machine.Guard{lit, unknown}}, false},
		{"any with an applies fires past a depends", machine.Guard{Any: []machine.Guard{unknown, lit}}, true},
		{"any of does-not-apply does not", machine.Guard{Any: []machine.Guard{dark, dark}}, false},
		{"not of depends still depends", machine.Guard{Not: &unknown}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := tc.guard
			r := machine.Recipe{Ref: testRef("fold"), Initial: "s", States: map[string]machine.State{
				"s": {Transitions: []machine.Transition{{On: beatTime, When: &g, Do: []machine.Act{{Pick: "x"}}}}},
			}}
			p := mustCompile(r)
			b := p.NewBlob()
			got, err := p.Step(&b, beatTime, tick{Round: 1}, env{"lit": true}, 1)
			require.NoError(t, err)
			assert.Equal(t, tc.fires, got.Fired)
		})
	}
}

func TestStep_Refusals(t *testing.T) {
	p := mustCompile(guardRecipe())

	_, err := p.Step(nil, beatTime, nil, nil, 1)
	assert.ErrorIs(t, err, machine.ErrNilBlob)

	b := machine.Blob{State: "gone"}
	_, err = p.Step(&b, beatTime, nil, nil, 1)
	assert.ErrorIs(t, err, machine.ErrUnknownState)

	b = p.NewBlob()
	_, err = p.Step(&b, "smell", nil, nil, 1)
	assert.ErrorIs(t, err, machine.ErrUnknownBeat)

	// An empty state is the initial one, as a blob loaded before it had a
	// machine.
	b = machine.Blob{}
	got, err := p.Step(&b, beatTime, tick{Round: 1}, nil, 1)
	require.NoError(t, err)
	assert.True(t, got.Fired)
	assert.Equal(t, "posted", b.State)
}

func TestDueAt_NoTimedNoWake(t *testing.T) {
	p := mustCompile(guardRecipe())
	b := machine.Blob{State: "asleep"}
	at, ok := p.DueAt(&b, 4)
	assert.False(t, ok, "a state with no timed transition and no wake is never due")
	assert.Zero(t, at)

	_, ok = p.DueAt(nil, 4)
	assert.False(t, ok)
}

func TestDueAt_TerminalIsNeverDue(t *testing.T) {
	p := mustCompile(states("s", map[string]machine.State{
		"s":    {Transitions: []machine.Transition{on(beatTime, nil, machine.Act{Pick: "x"})}},
		"done": {Terminal: true},
	}))
	b := machine.Blob{State: "done", WakeAt: u64(7)}
	at, ok := p.DueAt(&b, 4)
	assert.False(t, ok, "a terminal state is never due, even with a wake")
	assert.Zero(t, at)
}

func TestDueAt_WakeBeatsTimed(t *testing.T) {
	p := mustCompile(guardRecipe())
	b := p.NewBlob()

	at, ok := p.DueAt(&b, 4)
	assert.True(t, ok)
	assert.Equal(t, uint64(5), at, "a timed state is due next round")

	b.WakeAt = u64(9)
	at, ok = p.DueAt(&b, 4)
	assert.True(t, ok)
	assert.Equal(t, uint64(9), at, "a wake wins over the next round")

	asleep := machine.Blob{State: "asleep", WakeAt: u64(7)}
	at, ok = p.DueAt(&asleep, 4)
	assert.True(t, ok, "a wake is due even where no timed transition is")
	assert.Equal(t, uint64(7), at)
}

func TestListens_ChangesWithState(t *testing.T) {
	p := mustCompile(guardRecipe())
	b := p.NewBlob()

	assert.Equal(t, []machine.Listen{{Beat: beatHeard, Scope: &machine.ScopeLeaf{Within: 6}}}, p.Listens(&b),
		"posted listens for heard within 6; time is the heap's, never a listen")

	b.State = "alert"
	assert.Empty(t, p.Listens(&b), "alert listens only to time")

	b.State = "asleep"
	b.WakeAt = u64(30)
	assert.Equal(t, []machine.Listen{{Beat: beatHeard, Scope: &machine.ScopeLeaf{Within: 2}}}, p.Listens(&b),
		"a state with a wake still listens")

	// The returned slice is the caller's.
	ls := p.Listens(&b)
	ls[0].Scope.Within = 99
	assert.Equal(t, 2, p.Listens(&b)[0].Scope.Within)

	assert.Nil(t, p.Listens(nil))
}

func TestListens_DedupesAndWidens(t *testing.T) {
	near := machine.Guard{Scope: &machine.ScopeLeaf{Within: 2}}
	far := machine.Guard{Scope: &machine.ScopeLeaf{Within: 6}}
	lit := machine.Guard{Fact: &machine.FactLeaf{Name: "lit", Op: machine.OpEq, Value: true}}
	cases := []struct {
		name  string
		whens []*machine.Guard
		want  *machine.ScopeLeaf
	}{
		{"one scope", []*machine.Guard{&near}, &machine.ScopeLeaf{Within: 2}},
		{"a scope in a top all is necessary", []*machine.Guard{{All: []machine.Guard{lit, far}}},
			&machine.ScopeLeaf{Within: 6}},
		{"a scope under any is not", []*machine.Guard{{Any: []machine.Guard{lit, far}}}, nil},
		{"two scopes widen to the larger", []*machine.Guard{&near, &far}, &machine.ScopeLeaf{Within: 6}},
		{"an unscoped transition unscopes the beat", []*machine.Guard{&near, nil}, nil},
		{"different measures unscope the beat", []*machine.Guard{&near, {Scope: &machine.ScopeLeaf{Regions: 1}}}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ts []machine.Transition
			for _, w := range tc.whens {
				ts = append(ts, machine.Transition{On: beatHeard, When: w, Do: []machine.Act{{Pick: "x"}}})
			}
			ts = append(ts, machine.Transition{On: beatBell, Do: []machine.Act{{Pick: "y"}}})
			p := mustCompile(states("s", map[string]machine.State{"s": {Transitions: ts}}))
			b := p.NewBlob()
			got := p.Listens(&b)
			require.Len(t, got, 2, "one listen per beat")
			assert.Equal(t, beatHeard, got[0].Beat, "listens keep recipe order")
			assert.Equal(t, tc.want, got[0].Scope)
			assert.Equal(t, beatBell, got[1].Beat)
		})
	}
}
