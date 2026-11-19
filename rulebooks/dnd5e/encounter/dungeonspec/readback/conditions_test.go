// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

// conditions_test.go holds a step's condition connections (rpg-project#997)
// to the condition they stand for: a reader of connections alone rebuilds
// every castle step's `when` exactly as it compiled.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// atSegment is one step of a condition's At below `when`.
var atSegment = regexp.MustCompile(`\.(all|any)\[(\d+)\]|\.not`)

// rebuild is the guard a step's condition connections describe, as a reader
// who has only the connections would build it; nil for none.
func rebuild(t *testing.T, conns []readback.Connection) *machine.Guard {
	t.Helper()
	var root *machine.Guard
	for _, c := range conns {
		if c.Kind != readback.ConnCondition {
			continue
		}
		require.Regexp(t, `^when`, c.At)
		if root == nil {
			root = &machine.Guard{}
		}
		node := root
		for _, m := range atSegment.FindAllStringSubmatch(c.At[len("when"):], -1) {
			switch m[1] {
			case "all", "any":
				i, _ := strconv.Atoi(m[2])
				list := &node.All
				if m[1] == "any" {
					list = &node.Any
				}
				for len(*list) <= i {
					*list = append(*list, machine.Guard{})
				}
				node = &(*list)[i]
			default:
				if node.Not == nil {
					node.Not = &machine.Guard{}
				}
				node = node.Not
			}
		}
		op := machine.OpEq
		if c.Op != "" {
			op = machine.Op(c.Op)
		}
		n, _ := strconv.Atoi(c.Is)
		switch c.ID {
		case "within", "regions":
			if node.Scope == nil {
				node.Scope = &machine.ScopeLeaf{}
			}
			if c.ID == "within" {
				node.Scope.Within = n
			} else {
				node.Scope.Regions = n
			}
		case "rounds":
			node.Rounds = &machine.RoundsLeaf{Op: op, N: n}
		default:
			if name, ok := strings.CutPrefix(c.ID, "flag:"); ok {
				node.Flag = name
				continue
			}
			var value any = c.Is
			if b, err := strconv.ParseBool(c.Is); err == nil {
				value = b
			} else if n, err := strconv.Atoi(c.Is); err == nil {
				value = n
			}
			node.Fact = &machine.FactLeaf{Name: machine.FactName(c.ID), Op: op, Value: value}
		}
	}
	return root
}

// authored is g with every fact named as authored (the engine's `topic:`
// back to `noted.anyone:`), the spelling a condition connection uses.
func authored(g *machine.Guard) *machine.Guard {
	if g == nil {
		return nil
	}
	out := *g
	if g.Fact != nil {
		f := *g.Fact
		if w, id, ok := encounter.SplitConditionWord(string(f.Name)); ok && w.Open != encounter.SlotNone {
			f.Name = machine.FactName(w.Word + id)
		}
		out.Fact = &f
	}
	out.All, out.Any = nil, nil
	for i := range g.All {
		out.All = append(out.All, *authored(&g.All[i]))
	}
	for i := range g.Any {
		out.Any = append(out.Any, *authored(&g.Any[i]))
	}
	out.Not = authored(g.Not)
	return &out
}

// TestEveryCastleStepsWhenRebuildsFromItsConnections: for every step of
// every routine the castle runs, the guard rebuilt from the statement's
// condition connections is the compiled guard.
func TestEveryCastleStepsWhenRebuildsFromItsConnections(t *testing.T) {
	raw, rb := theCastle(t)
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
	require.NoError(t, err)
	compiled, err := dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: decoded.Spec})
	require.NoError(t, err)

	recipes := recipesOf(decoded, compiled)

	byPath := map[string]readback.Statement{}
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindRoutineWake {
			byPath[st.Path] = st
		}
	}
	steps, guarded := 0, 0
	for def, r := range recipes {
		for state, st := range r.States {
			for k, tr := range st.Transitions {
				path := fmt.Sprintf("behaviorDefinitions.%s.recipe.states.%s.transitions[%d]", def, state, k)
				stmt, ok := byPath[path]
				require.True(t, ok, "a statement for %s", path)
				steps++
				if tr.When != nil {
					guarded++
				}
				require.Equal(t, authored(tr.When), rebuild(t, stmt.Connects), path)
				requireActsRebuild(t, path, tr, stmt.Connects)
			}
		}
	}
	require.Positive(t, guarded, "the castle has guarded steps (%d steps)", steps)
}

// TestEveryCastleConditionIDResolves: every condition connection the castle
// says resolves to a table word that takes it (conditionids_internal_test.go
// holds every other content file).
func TestEveryCastleConditionIDResolves(t *testing.T) {
	_, rb := theCastle(t)
	seen := 0
	for _, st := range rb.Statements() {
		for _, c := range readback.ConditionsOf(st.Connects) {
			readback.RequireConditionAccepted(t, st.Path, c)
			seen++
		}
	}
	require.Positive(t, seen)
}

// recipesOf is each routine's compiled recipe, by its definition, as the
// first creature that runs it holds it.
func recipesOf(decoded *dungeonspec.SingleRoomDecodeResult, compiled dungeonspec.Compiled) map[string]*machine.Recipe {
	recipes := map[string]*machine.Recipe{}
	for i := range compiled.Monsters {
		m := &compiled.Monsters[i]
		bind := decoded.Spec.Room.Gameplay.MonsterBindings[m.MemberID]
		if bind.Behaviors == nil || m.Behaviors == nil {
			continue
		}
		for k, use := range *bind.Behaviors {
			if k < len(m.Behaviors.Instances) && recipes[use.Use] == nil {
				recipes[use.Use] = &m.Behaviors.Instances[k].Instance.Recipe
			}
		}
	}
	return recipes
}

// actsOf is a step's move and acts as a reader of its [readback.ConnAct]
// connections alone builds them.
func actsOf(t *testing.T, conns []readback.Connection) (string, []machine.Act) {
	t.Helper()
	to := ""
	var acts []machine.Act
	place := regexp.MustCompile(`^do\[(\d+)\]`)
	for _, c := range conns {
		if c.Kind != readback.ConnAct {
			continue
		}
		word, named, _ := strings.Cut(c.ID, ":")
		if c.At == "to" {
			require.Equal(t, "to", word)
			to = named
			continue
		}
		m := place.FindStringSubmatch(c.At)
		require.NotNil(t, m, "an act's place: %q", c.At)
		j, _ := strconv.Atoi(m[1])
		for len(acts) <= j {
			acts = append(acts, machine.Act{})
		}
		a := &acts[j]
		switch word {
		case "pick":
			a.Pick = named
		case "wake_in":
			n, err := strconv.Atoi(c.Is)
			require.NoError(t, err)
			a.WakeIn = n
		case "anchor":
			a.Anchor = true
		case "set":
			var v any
			require.NoError(t, json.Unmarshal([]byte(c.Is), &v))
			a.Set = &machine.SetAct{Key: named, Value: v}
		case "clear":
			a.Clear = append(a.Clear, named)
		default:
			t.Fatalf("an act word no reader knows: %q", word)
		}
	}
	return to, acts
}

// requireActsRebuild holds a step's act connections to its compiled move
// and acts (a set's value compared as JSON, the kind its literal says).
func requireActsRebuild(t *testing.T, path string, tr machine.Transition, conns []readback.Connection) {
	t.Helper()
	to, acts := actsOf(t, conns)
	require.Equal(t, tr.To, to, "%s: its move", path)
	want, err := json.Marshal(tr.Do)
	require.NoError(t, err)
	got, err := json.Marshal(acts)
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(got), "%s: its acts", path)
}

// TestEveryStepsActsRebuildFromItsConnections: in every content file but
// the castle (TestEveryCastleStepsWhenRebuildsFromItsConnections holds it),
// every routine step's move and acts rebuild from its act connections
// alone.
func TestEveryStepsActsRebuildFromItsConnections(t *testing.T) {
	files, err := filepath.Glob("../testdata/*.yaml")
	require.NoError(t, err)
	more, err := filepath.Glob("testdata/*.yaml")
	require.NoError(t, err)
	steps := 0
	for _, f := range append(files, more...) {
		if filepath.Base(f) == filepath.Base(castlePath) {
			continue
		}
		raw, err := os.ReadFile(f)
		require.NoError(t, err)
		rb, err := readback.Of(raw)
		if err != nil {
			continue // a refused file says no steps
		}
		decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: raw})
		require.NoError(t, err)
		compiled, err := dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: decoded.Spec})
		require.NoError(t, err)
		byPath := map[string]readback.Statement{}
		for _, st := range rb.Statements() {
			if st.Kind == readback.KindRoutineWake {
				byPath[st.Path] = st
			}
		}
		for def, r := range recipesOf(decoded, compiled) {
			for state, st := range r.States {
				for k, tr := range st.Transitions {
					path := fmt.Sprintf("behaviorDefinitions.%s.recipe.states.%s.transitions[%d]", def, state, k)
					stmt, ok := byPath[path]
					require.True(t, ok, "%s: a statement for %s", f, path)
					requireActsRebuild(t, f+" "+path, tr, stmt.Connects)
					steps++
				}
			}
		}
	}
	require.Positive(t, steps)
}

// TestAuthoredActsRebuildFromTheirConnections: a set of each flag kind,
// a clear of several flags, a wait and an anchor rebuild from their
// connections alone.
func TestAuthoredActsRebuildFromTheirConnections(t *testing.T) {
	tr := machine.Transition{To: "alert", Do: []machine.Act{
		{Set: &machine.SetAct{Key: "warned", Value: true}},
		{Set: &machine.SetAct{Key: "count", Value: 2}},
		{Set: &machine.SetAct{Key: "mood", Value: "idle"}},
		{Clear: []string{"warned", "count"}},
		{WakeIn: 3},
		{Anchor: true},
		{Pick: "post"},
	}}
	requireActsRebuild(t, "sample", tr, readback.ActConnectionsOf(tr))
}
