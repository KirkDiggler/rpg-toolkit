// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// TestStatesPath_ReadsTheDialectsWrapperBack: the states dialect wraps a
// step's within, rounds and when in an `all`; a machine leaf inside it is
// reported at the key the author wrote.
func TestStatesPath_ReadsTheDialectsWrapperBack(t *testing.T) {
	s := &StatesSpec{Initial: "p", States: map[string]StateSpec{"p": {Transitions: []TransitionSpec{
		{On: "heard", Within: 2, Rounds: 3, When: &WhenSpec{Not: &WhenSpec{Enemy: "seen"}}},
		{On: "fact", When: &WhenSpec{Fact: "gate-cowed"}, Do: []ActSpec{{Pick: "time"}}},
	}}}}
	at := func(i int, leaf ...machine.Segment) string {
		return placedRefusal{kind: machine.PlaceStep, state: "p", transition: i, leaf: leaf}.statesPath("b.states", s)
	}
	when := machine.Segment{Name: "when"}
	all := func(i int) machine.Segment { return machine.Segment{Name: "all", Index: i, Indexed: true} }
	require.Equal(t, "b.states.p.transitions[0].within", at(0, when, all(0), machine.Segment{Name: "scope"}))
	require.Equal(t, "b.states.p.transitions[0].rounds", at(0, when, all(1), machine.Segment{Name: "rounds"}))
	require.Equal(t, "b.states.p.transitions[0].when.not.enemy", at(0, when, all(2), machine.Segment{Name: "not"}, machine.Segment{Name: "fact"}))
	require.Equal(t, "b.states.p.transitions[1].when.fact", at(1, when, machine.Segment{Name: "fact"}), "one part: no wrapper")
	require.Equal(t, "b.states.p.transitions[1].do[0]", at(1, machine.Segment{Name: "do", Index: 0, Indexed: true}))
	require.Equal(t, "b.states.p.transitions[1]", at(1))
	require.Equal(t, "b.states.p", placedRefusal{kind: machine.PlaceState, state: "p"}.statesPath("b.states", s))
	require.Equal(t, "b.states.initial", placedRefusal{kind: machine.PlaceInitial}.statesPath("b.states", s))
}
