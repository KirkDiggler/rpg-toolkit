// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func leafGuard(name string, op machine.Op, value any) *machine.Guard {
	return &machine.Guard{Fact: &machine.FactLeaf{Name: machine.FactName(name), Op: op, Value: value}}
}

// conditionsOf is the [ConnCondition] connections among conns.
func conditionsOf(conns []Connection) []Connection {
	var out []Connection
	for _, c := range conns {
		if c.Kind == ConnCondition {
			out = append(out, c)
		}
	}
	return out
}

// TestAStepsGuardWordsAreDrawn: every guard word the machine and the words
// slices touched (rpg-project#1000, #1005) is said in plain words and
// connects to what it reads: its condition, and the thing in the file it
// reads where it reads one.
func TestAStepsGuardWordsAreDrawn(t *testing.T) {
	cond := func(id, is string) Connection {
		return Connection{Kind: ConnCondition, ID: id, Role: "asks", Is: is, At: "when"}
	}
	for _, tc := range []struct {
		name     string
		guard    *machine.Guard
		sentence string
		conns    []Connection
	}{
		{"noted, eq true", leafGuard("noted:admitted", machine.OpEq, true),
			"X has noted `admitted` about the one X heard (X knows who only by seeing them)",
			[]Connection{cond("noted:admitted", "true"), {Kind: "topic", ID: "admitted", Role: "asks"},
				{Kind: ConnWho, ID: "the_one_named", Role: "about", At: "admitted"}}},
		{"noted, eq false", leafGuard("noted:admitted", machine.OpEq, false),
			"X has not noted `admitted` about the one X heard (X knows who only by seeing them)",
			[]Connection{cond("noted:admitted", "false"), {Kind: "topic", ID: "admitted", Role: "asks", Not: true},
				{Kind: ConnWho, ID: "the_one_named", Role: "about", At: "admitted"}}},
		{"noted.anyone, eq true, as the engine spells it", leafGuard("topic:admitted", machine.OpEq, true),
			"X has noted `admitted` about anyone",
			[]Connection{cond("noted.anyone:admitted", "true"), {Kind: "topic", ID: "admitted", Role: "wakes-on"},
				{Kind: ConnWho, ID: "anyone", Role: "about", At: "admitted"}}},
		{"heard.claim, eq failed", leafGuard("heard.claim", machine.OpEq, "failed"),
			"what X heard was a claim that failed", []Connection{cond("heard.claim", "failed")}},
		{"flag", &machine.Guard{Flag: "lit"}, "X's flag `lit` is set",
			[]Connection{{Kind: ConnCondition, ID: "flag:lit", Role: "asks", At: "when"}, {Kind: ConnFlag, ID: "lit", Role: "asks"}}},
		{"fact", leafGuard("fact:gate-cowed", machine.OpEq, true),
			"X knows the fact `gate-cowed`",
			[]Connection{cond("fact:gate-cowed", "true"), {Kind: "fact", ID: "gate-cowed", Role: "wakes-on"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, conns := guardWords(tc.guard, "X", "heard")
			require.Equal(t, tc.sentence, s)
			require.Equal(t, tc.conns, conns)
		})
	}
}

// TestAConditionKeepsItsPolarityAndPlace: a `not` is carried on the
// condition, an odd count of them only; `all` and `any` differ in At.
func TestAConditionKeepsItsPolarityAndPlace(t *testing.T) {
	held := leafGuard("heard.claim", machine.OpEq, "held")
	_, conns := guardWords(&machine.Guard{Not: held}, "X", "heard")
	require.Equal(t, []Connection{{Kind: ConnCondition, ID: "heard.claim", Role: "asks", Is: "held", Not: true, At: "when.not"}}, conns)

	_, conns = guardWords(&machine.Guard{Not: &machine.Guard{Not: held}}, "X", "heard")
	require.Equal(t, []Connection{{Kind: ConnCondition, ID: "heard.claim", Role: "asks", Is: "held", At: "when.not.not"}}, conns,
		"two nots cancel")

	other := leafGuard("noted:admitted", machine.OpEq, true)
	_, all := guardWords(&machine.Guard{All: []machine.Guard{*other, *held}}, "X", "heard")
	_, any := guardWords(&machine.Guard{Any: []machine.Guard{*other, *held}}, "X", "heard")
	require.Equal(t, "when.all[1]", conditionsOf(all)[1].At)
	require.Equal(t, "when.any[1]", conditionsOf(any)[1].At)

	_, conns = guardWords(&machine.Guard{Rounds: &machine.RoundsLeaf{Op: machine.OpGte, N: 3}}, "X", "time")
	require.Equal(t, []Connection{{Kind: ConnCondition, ID: "rounds", Role: "asks", Is: "3", Op: "gte", At: "when"}}, conns)

	_, conns = guardWords(&machine.Guard{Scope: &machine.ScopeLeaf{Within: 2, Regions: 1}}, "X", "heard")
	require.Equal(t, []Connection{
		{Kind: ConnCondition, ID: "within", Role: "asks", Is: "2", Op: "lte", At: "when"},
		{Kind: ConnCondition, ID: "regions", Role: "asks", Is: "1", Op: "lte", At: "when"},
	}, conns, "within and regions are at most")

	_, conns = guardWords(&machine.Guard{Flag: "lit"}, "X", "time")
	require.Equal(t, []Connection{{Kind: ConnCondition, ID: "flag:lit", Role: "asks", At: "when"}, {Kind: ConnFlag, ID: "lit", Role: "asks"}}, conns,
		"a flag's name is in its id, as every word's open part is")

	two := leafGuard("noted:admitted", machine.OpEq, true)
	_, conns = guardWords(&machine.Guard{Not: &machine.Guard{All: []machine.Guard{*held, *two}}}, "X", "heard")
	require.Equal(t, "when.not.all[0]", conns[0].At)
	require.True(t, conns[0].Not, "each leaf under one not carries it; At says the not wraps the all")
}

// sampleInner is the condition the node samples hold.
var sampleInner = leafGuard("heard.claim", machine.OpEq, "held")

// conditionSamples is one authored sample of every condition word in the
// table, with the id and place its connection must have.
var conditionSamples = map[string]struct {
	g      *machine.Guard
	id, at string
}{
	"enemy":           {leafGuard("enemy", machine.OpEq, "seen"), "enemy", "when"},
	"fact:":           {leafGuard("fact:gate-cowed", machine.OpEq, true), "fact:gate-cowed", "when"},
	"heard.kind":      {leafGuard("heard.kind", machine.OpEq, "voice"), "heard.kind", "when"},
	"heard.loudness:": {leafGuard("heard.loudness:shout", machine.OpEq, true), "heard.loudness:shout", "when"},
	"heard.claim":     {sampleInner, "heard.claim", "when"},
	"noted:":          {leafGuard("noted:admitted", machine.OpEq, true), "noted:admitted", "when"},
	"noted.as:":       {leafGuard("noted.as:admitted", machine.OpEq, "warned"), "noted.as:admitted", "when"},
	"noted.anyone:":   {leafGuard("topic:admitted", machine.OpEq, true), "noted.anyone:admitted", "when"},
	"flag:":           {&machine.Guard{Flag: "lit"}, "flag:lit", "when"},
	"within":          {&machine.Guard{Scope: &machine.ScopeLeaf{Within: 2}}, "within", "when"},
	"regions":         {&machine.Guard{Scope: &machine.ScopeLeaf{Regions: 1}}, "regions", "when"},
	"rounds":          {&machine.Guard{Rounds: &machine.RoundsLeaf{Op: machine.OpGte, N: 3}}, "rounds", "when"},
	"all":             {&machine.Guard{All: []machine.Guard{*sampleInner}}, "heard.claim", "when.all[0]"},
	"any":             {&machine.Guard{Any: []machine.Guard{*sampleInner}}, "heard.claim", "when.any[0]"},
	"not":             {&machine.Guard{Not: sampleInner}, "heard.claim", "when.not"},
}

// TestEveryConditionWordIsAConnection walks the machine words table: every
// condition word yields a connection of its own (a leaf) or places the
// conditions under it (a node), so a new word cannot ship without one.
func TestEveryConditionWordIsAConnection(t *testing.T) {
	sample := conditionSamples
	for _, w := range encounter.MachineWords().Conditions {
		tc, ok := sample[w.Word]
		require.True(t, ok, "a sample for the condition word %q: a new word needs its connection", w.Word)
		_, conns := guardWords(tc.g, "X", "heard")
		got := conditionsOf(conns)
		require.Len(t, got, 1, w.Word)
		require.Equal(t, tc.id, got[0].ID, w.Word)
		require.Equal(t, tc.at, got[0].At, w.Word)
		if w.Open == encounter.SlotNone && w.Join == "" && w.Word != "not" {
			require.Equal(t, w.Word, got[0].ID, "a whole word is its own id")
		} else if w.Open != encounter.SlotNone {
			require.True(t, strings.HasPrefix(got[0].ID, w.Word), "%s fills its open part", w.Word)
		}
	}
}

// TestRoundsWithNoCountIsTheVeryRound: `rounds: { op: eq }` with no n (n 0,
// the table's default) is said as the very round the anchor was set, not
// "exactly 0 rounds"; `gte` with no n keeps the template.
func TestRoundsWithNoCountIsTheVeryRound(t *testing.T) {
	s, conns := guardWords(&machine.Guard{Rounds: &machine.RoundsLeaf{Op: machine.OpEq}}, "X", "time")
	require.Equal(t, "it is the very round X set its anchor", s)
	require.Equal(t, []Connection{{Kind: ConnCondition, ID: "rounds", Role: "asks", Is: "0", At: "when"}}, conns)
	s, _ = guardWords(&machine.Guard{Rounds: &machine.RoundsLeaf{Op: machine.OpGte}}, "X", "time")
	require.Equal(t, "at least 0 rounds have passed since X set its anchor", s)
}
