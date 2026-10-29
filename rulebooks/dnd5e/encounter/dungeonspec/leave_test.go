// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// leave_test.go is THE RUN ENDS AT THE DOOR in the dialect (rpg-project#716):
// `{ left: <exit id> }` on an ending, with an `if` and an `outcome`, and
// nowhere else.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// leaveRoot is a front door and the operator's two endings at it, with a
// goblin in the king's place.
const leaveRoot = `exits:
  - { id: front-door, cell: { q: 0, r: 0 } }
endings:
  - { id: goblin-slain, when: { left: front-door }, if: { down: goblin-1 }, outcome: success }
  - { id: goblin-lives, when: { left: front-door }, outcome: failure }`

// TestTheLeaveEndingsCompile: both endings reach the engine as a leave
// trigger at the door, the first with its `if`, each with its outcome.
func TestTheLeaveEndingsCompile(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(v4With(leaveRoot, "")))
	require.NoError(t, err)

	require.Equal(t, []encounter.EndingInput{
		{Key: "goblin-slain", Trigger: encounter.TriggerLeft{Exit: "front-door"},
			If: encounter.TriggerMemberDown{Member: "goblin-1"}, Result: encounter.ResultSuccess},
		{Key: "goblin-lives", Trigger: encounter.TriggerLeft{Exit: "front-door"}, Result: encounter.ResultFailure},
	}, compiled.Endings)
}

// TestTheLeaveEndingIsRefusedLineByLine: each wrong line at its own path.
func TestTheLeaveEndingIsRefusedLineByLine(t *testing.T) {
	const door = "exits:\n  - { id: front-door, cell: { q: 0, r: 0 } }\n"
	for _, tc := range []struct {
		name, root, path string
		says             []string
	}{
		{name: "an exit nobody declared",
			root: door + "endings:\n  - { id: out, when: { left: back-door }, outcome: failure }",
			path: "endings[0].when.left", says: []string{`"back-door" is not a way out this dungeon declares`}},
		{name: "no outcome",
			root: door + "endings:\n  - { id: out, when: { left: front-door } }",
			path: "endings[0].outcome", says: []string{"`outcome: success` or `outcome: failure`"}},
		{name: "an outcome that is neither",
			root: door + "endings:\n  - { id: out, when: { left: front-door }, outcome: draw }",
			path: "endings[0].outcome", says: []string{"`outcome: draw`", "success or failure"}},
		{name: "an if beside anything but left",
			root: door + "endings:\n  - { id: out, when: { round: 3 }, if: { down: goblin-1 } }",
			path: "endings[0].if", says: []string{"only an ending that waits for the party to leave"}},
		{name: "an if that is a holding",
			root: door + "endings:\n  - { id: out, when: { left: front-door }, if: { holds: table }, outcome: success }",
			path: "endings[0].if", says: []string{"`holds` is none of them"}},
		{name: "an if that is a left",
			root: door + "endings:\n  - { id: out, when: { left: front-door }, if: { left: front-door }, outcome: success }",
			path: "endings[0].if", says: []string{"`left` is none of them"}},
		{name: "an if on nobody placed",
			root: door + "endings:\n  - { id: out, when: { left: front-door }, if: { down: king }, outcome: success }",
			path: "endings[0].if.down", says: []string{`"king" is not a placement in this dungeon`}},
		{name: "left on a disposition's until",
			root: door + "factions:\n  - { id: goblins }\ndispositions:\n" +
				"  - { between: [goblins, party], stance: neutral, until: { left: front-door } }",
			path: "dispositions[0].until.left", says: []string{"only an ending waits for that"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireRefusedAt(t, refusals(t, v4With(tc.root, "")), tc.path, tc.says...)
		})
	}
}

// TestLeftIsRefusedOnAnArrivalAndARequirement: a reserve cannot wait for
// the party to have gone, and nobody speaks after they have; each is refused
// by the word's name.
func TestLeftIsRefusedOnAnArrivalAndARequirement(t *testing.T) {
	for _, tc := range []struct {
		name, gameplay, says string
	}{
		{name: "an arrival", gameplay: "    monsterBindings:\n      goblin-2: { arrives: { left: front-door } }",
			says: "only an ending waits for that"},
		{name: "an option's requires",
			gameplay: "    monsterBindings:\n      goblin-1: { options: [ { id: hail, say: Hi, requires: { left: front-door } } ] }",
			says:     "`left` is neither"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errs := refusals(t, v4With("exits:\n  - { id: front-door, cell: { q: 0, r: 0 } }", tc.gameplay))
			var said bool
			for _, e := range errs {
				said = said || strings.Contains(e.Message, tc.says)
			}
			require.True(t, said, "refused by name: %v", errs)
		})
	}
}
