// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/require"
)

const statesRoot = `behaviorDefinitions:
  posted:
    recipe:
      ref: example:behaviors:posted
      initial: posted
      states:
        posted:
          transitions:
            - {on: heard, when: {scope: {within: 6}}, to: alert, do: [{anchor: true}], reason: heard something}
        alert:
          transitions:
            - {on: time, when: {rounds: {op: gte, n: 3}}, to: posted}
            - {on: time, do: [{pick: time}]}
    table: {time: [{hold: {}}]}
  waiting:
    recipe:
      ref: example:behaviors:waiting
      initial: waiting
      states:
        waiting: {transitions: [{on: clock:fight, to: marching}]}
        marching: {transitions: [{on: time, do: [{pick: march}]}]}
    table: {march: [{toward: {at: {q: 0, r: 1}}}]}
factions:
  - id: garrison
    behaviors: [{id: body, use: waiting}]
`

const statesBindings = `    monsterBindings:
      goblin-1:
        faction: garrison
        behaviors: [{id: body, use: posted}]
      goblin-2:
        faction: garrison
`

func TestDungeonspec_StatesCompile(t *testing.T) {
	c := loadSource(t, v4With(statesRoot, statesBindings))
	own := monsterNamed(t, c, "goblin-1").Behaviors.Instances[0].Instance
	inherited := monsterNamed(t, c, "goblin-2").Behaviors.Instances[0].Instance
	require.Equal(t, "example:behaviors:posted", own.Recipe.Ref.String())
	require.Equal(t, "example:behaviors:waiting", inherited.Recipe.Ref.String())
	tr := own.Recipe.States["posted"].Transitions[0]
	require.Equal(t, "heard", string(tr.On))
	require.Equal(t, 6, tr.When.Scope.Within)
	require.True(t, tr.Do[0].Anchor)
	require.Equal(t, "alert", tr.To)
	require.Equal(t, 3, own.Recipe.States["alert"].Transitions[0].When.Rounds.N)
	require.Contains(t, inherited.Table, encounter.AnswerKey("march"))
	members := []encounter.MemberInput{{ID: "hero", Kind: encounter.KindPlayer, Position: c.PartyStart[0].At}}
	for _, m := range c.Monsters {
		members = append(members, encounter.MemberInput{ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At, Faction: encounter.FactionID(m.Faction), Behaviors: m.Behaviors})
	}
	_, err := encounter.NewEncounter(&encounter.SetupInput{Field: c.Field, Members: members, Endings: []encounter.EndingInput{{Key: "stop", Trigger: encounter.TriggerExternal{}}}, Capabilities: facingCapabilities()})
	require.NoError(t, err, "the real installation accepts the compiled definitions")
}

func TestDungeonspec_NativeStateRefusals(t *testing.T) {
	good := `ref: example:behaviors:test
initial: ready
states:
  ready: {transitions: [{on: time, do: [{pick: time}]}]}`
	for _, tc := range []struct{ name, old, replacement string }{
		{"unknown beat", "on: time", "on: imaginary"},
		{"missing state", "initial: ready", "initial: absent"},
		{"absent picked table", "pick: time", "pick: absent"},
		{"multi pick", "do: [{pick: time}]", "do: [{pick: time}, {pick: time}]"},
		{"scope on time", "on: time, do:", "on: time, when: {scope: {within: 2}}, do:"},
		{"runtime blob", "initial: ready", "initial: ready\nblob: {}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			native := strings.Replace(good, tc.old, tc.replacement, 1)
			root := "behaviorDefinitions:\n  test:\n    recipe:\n      " + strings.ReplaceAll(native, "\n", "\n      ") + "\n    table: {time: [{hold: {}}]}\n"
			_, err := dungeonspec.Load([]byte(v4With(root, bindingOnGoblin1("        behaviors: [{id: body, use: test}]\n"))))
			require.ErrorIs(t, err, dungeonspec.ErrBadSpec)
			require.ErrorContains(t, err, "test")
		})
	}
}

func TestDungeonspec_BlocksCompilesToBlockingStates(t *testing.T) {
	bindings := strings.Replace(statesBindings, "        behaviors: [{id: body, use: posted}]", "        behaviors: [{id: body, use: posted}]\n        behaviorState: body\n        blockingStates: [alert, posted]", 1)
	c := loadSource(t, v4With(statesRoot, bindings))
	own := monsterNamed(t, c, "goblin-1")
	require.Equal(t, []string{"alert", "posted"}, own.BlockingStates)
	require.Equal(t, "body", own.Behaviors.BodyInstance)
	require.Empty(t, monsterNamed(t, c, "goblin-2").BlockingStates)
	for _, bad := range []string{"absent", "marching"} {
		_, err := dungeonspec.Load([]byte(v4With(statesRoot, strings.Replace(bindings, "blockingStates: [alert, posted]", "blockingStates: ["+bad+"]", 1))))
		require.ErrorIs(t, err, dungeonspec.ErrBadSpec)
		require.ErrorContains(t, err, "behaviorState")
	}
}

func TestDungeonspec_HoldsCompilesToPosts(t *testing.T) {
	raw, err := os.ReadFile("testdata/world-builder-v4-castle.yaml")
	require.NoError(t, err)
	c := loadSource(t, string(raw))
	gate := encounter.Post{Door: "the-mad-king/gate-wall-door", Cells: []spatial.Position{{X: -3, Y: 7}, {X: -4, Y: 7}}, Against: encounter.PostEntering, Pass: []encounter.Trigger{encounter.TriggerRecognized{Topic: "admitted"}}}
	require.Equal(t, map[string]encounter.Post{"posted": gate, "alert": gate, "challenged": gate}, monsterNamed(t, c, "gate-1").Posts)
	// Every gate guard holds the gate (rpg-project#928), each from his own
	// cells, each letting through whom he recognises.
	recognized := []encounter.Trigger{encounter.TriggerRecognized{Topic: "admitted"}}
	require.Equal(t, map[string]encounter.Post{"ready": {Door: gate.Door, Cells: []spatial.Position{{X: -3, Y: 10}, {X: -2, Y: 8}, {X: -4, Y: 10}, {X: -2, Y: 10}}, Against: encounter.PostEntering, Pass: recognized}}, monsterNamed(t, c, "gate-2").Posts)
	require.Equal(t, map[string]encounter.Post{"ready": {Door: gate.Door, Cells: []spatial.Position{{X: -4, Y: 8}}, Against: encounter.PostEntering, Pass: recognized}}, monsterNamed(t, c, "captain").Posts)
	require.True(t, slices.ContainsFunc(c.Field.Doors, func(d encounter.DoorInput) bool { return d.ID == gate.Door }))
}
