// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

// conditions_test.go holds a step's condition connections (rpg-project#997)
// to the condition they stand for: a reader of connections alone rebuilds
// every castle step's `when` exactly as it compiled.

import (
	"fmt"
	"regexp"
	"strconv"
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
		case "flag":
			node.Flag = c.Is
		default:
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
			}
		}
	}
	require.Positive(t, guarded, "the castle has guarded steps (%d steps)", steps)
}
