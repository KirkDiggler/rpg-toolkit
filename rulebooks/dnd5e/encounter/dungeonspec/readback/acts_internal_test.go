// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

// acts_internal_test.go holds every act of a step to one connection shape
// (rpg-project#1001), driven from the machine words table: a new act word
// with no connection fails here.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// actOf is a step's one act connection with the given id prefix.
func actConn(id, is, at string) Connection {
	return Connection{Kind: ConnAct, ID: id, Role: "does", Is: is, At: at}
}

// TestEveryActIsAConnection: one sample step per act word in the table;
// each writes its act connection, and a flag act its flag edge.
func TestEveryActIsAConnection(t *testing.T) {
	samples := map[string]struct {
		tr   machine.Transition
		want []Connection
	}{
		"to":      {machine.Transition{To: "alert"}, []Connection{actConn("to:alert", "", "to")}},
		"pick":    {machine.Transition{Do: []machine.Act{{Pick: "post"}}}, []Connection{actConn("pick:post", "", "do[0]")}},
		"wake_in": {machine.Transition{Do: []machine.Act{{WakeIn: 3}}}, []Connection{actConn("wake_in", "3", "do[0]")}},
		"anchor":  {machine.Transition{Do: []machine.Act{{Anchor: true}}}, []Connection{actConn("anchor", "", "do[0]")}},
		"set": {machine.Transition{Do: []machine.Act{
			{Set: &machine.SetAct{Key: "warned", Value: true}},
			{Set: &machine.SetAct{Key: "count", Value: 2}},
			{Set: &machine.SetAct{Key: "mood", Value: "idle"}},
		}}, []Connection{
			actConn("set:warned", "true", "do[0]"), {Kind: ConnFlag, ID: "warned", Role: "sets"},
			actConn("set:count", "2", "do[1]"), {Kind: ConnFlag, ID: "count", Role: "sets"},
			actConn("set:mood", `"idle"`, "do[2]"), {Kind: ConnFlag, ID: "mood", Role: "sets"},
		}},
		"clear": {machine.Transition{Do: []machine.Act{{Clear: []string{"warned", "count"}}}}, []Connection{
			actConn("clear:warned", "", "do[0].clear[0]"), {Kind: ConnFlag, ID: "warned", Role: "clears"},
			actConn("clear:count", "", "do[0].clear[1]"), {Kind: ConnFlag, ID: "count", Role: "clears"},
		}},
	}
	for _, w := range encounter.MachineWords().Acts {
		sample, ok := samples[w.Word]
		require.True(t, ok, "a sample for the act word %q: a new act needs its connection", w.Word)
		got := actConnections(sample.tr)
		require.Equal(t, sample.want, got, w.Word)
		require.True(t, strings.HasPrefix(got[0].ID, w.Word), "%s: the id begins with the act's word", w.Word)
	}
}

// TestAFlagIsNamedAlikeByItsReadersAndWriters: the step that reads a flag
// and the steps that set and clear it carry one edge kind and id, so a
// reader joins them.
func TestAFlagIsNamedAlikeByItsReadersAndWriters(t *testing.T) {
	_, reads := guardWords(&machine.Guard{Flag: "warned"}, "X", "time")
	sets := actConnections(machine.Transition{Do: []machine.Act{{Set: &machine.SetAct{Key: "warned", Value: true}}}})
	clears := actConnections(machine.Transition{Do: []machine.Act{{Clear: []string{"warned"}}}})
	edge := func(conns []Connection) Connection {
		for _, c := range conns {
			if c.Kind == ConnFlag {
				return c
			}
		}
		t.Fatalf("no flag edge in %v", conns)
		return Connection{}
	}
	require.Equal(t, Connection{Kind: ConnFlag, ID: "warned", Role: "asks"}, edge(reads))
	require.Equal(t, Connection{Kind: ConnFlag, ID: "warned", Role: "sets"}, edge(sets))
	require.Equal(t, Connection{Kind: ConnFlag, ID: "warned", Role: "clears"}, edge(clears))
}

// ActConnectionsOf is [actConnections], for the rebuild test that reads
// connections as an outside reader does (conditions_test.go).
var ActConnectionsOf = actConnections
