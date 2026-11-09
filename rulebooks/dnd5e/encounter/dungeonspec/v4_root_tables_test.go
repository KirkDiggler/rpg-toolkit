// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// Canonical definitions replace actor-level table layering. The reusable row
// grammar remains the table's owner; attachment inheritance replaces a whole
// list, not selected keys of another actor policy.

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/stretchr/testify/require"
	"testing"
)

const goblinMind = `behaviorDefinitions:
  goblin-mind:
    recipe:
      ref: example:behaviors:goblin-mind
      initial: ready
      states:
        ready: {transitions: [{on: time, do: [{pick: time}]}]}
    table:
      time:
        - {when: {enemy: reach}, attack: enemy}
        - {when: {enemy: none}, hold: {}}
  scout:
    recipe:
      ref: example:behaviors:scout
      initial: ready
      states:
        ready: {transitions: [{on: time, do: [{pick: walk}]}]}
    table:
      walk: [{toward: {at: {q: 0, r: 1}}}]
`

func bindingOnGoblin1(binding string) string {
	return "    monsterBindings:\n      goblin-1:\n" + binding
}

func TestARootDefinitionSuppliesItsBoundTable(t *testing.T) {
	c := loadSource(t, v4With(goblinMind, bindingOnGoblin1("        behaviors: [{id: duty, use: goblin-mind}]\n")))
	b := monsterNamed(t, c, "goblin-1").Behaviors
	require.NotNil(t, b)
	require.Equal(t, "duty", b.Instances[0].Instance.ID)
	require.Len(t, b.Instances[0].Instance.Table["time"], 2)
}

func TestOneDefinitionServesManyCreatures(t *testing.T) {
	c := loadSource(t, v4With(goblinMind, "    monsterBindings:\n      goblin-1: {behaviors: [{id: duty, use: goblin-mind}]}\n      goblin-2: {behaviors: [{id: duty, use: goblin-mind}]}\n"))
	first := monsterNamed(t, c, "goblin-1").Behaviors.Instances[0]
	second := monsterNamed(t, c, "goblin-2").Behaviors.Instances[0]
	require.Equal(t, first.Instance.Recipe.Ref, second.Instance.Recipe.Ref)
	first.Instance.Table["time"][0].Weight = 8
	require.Equal(t, 1, second.Instance.Table["time"][0].Weight, "one definition, detached bindings")
}

func TestABindingReplacesItsFactionsWholeAttachmentList(t *testing.T) {
	root := goblinMind + "factions:\n  - id: watch\n    behaviors: [{id: ordinary, use: goblin-mind}]\n"
	inherited := loadSource(t, v4With(root, bindingOnGoblin1("        faction: watch\n")))
	require.Equal(t, "ordinary", monsterNamed(t, inherited, "goblin-1").Behaviors.Instances[0].Instance.ID)
	replaced := loadSource(t, v4With(root, bindingOnGoblin1("        faction: watch\n        behaviors: [{id: scout, use: scout}]\n")))
	instances := monsterNamed(t, replaced, "goblin-1").Behaviors.Instances
	require.Len(t, instances, 1)
	require.Equal(t, "scout", instances[0].Instance.ID)
	require.Contains(t, instances[0].Instance.Table, encounter.AnswerKey("walk"))
	require.NotContains(t, instances[0].Instance.Table, encounter.AnswerKey("time"), "old policy keys do not leak through a replacement")
	empty := loadSource(t, v4With(root, bindingOnGoblin1("        faction: watch\n        behaviors: []\n")))
	require.Empty(t, monsterNamed(t, empty, "goblin-1").Behaviors.Instances)
}

func TestADefinitionNameThatNamesNothingIsRefused(t *testing.T) {
	_, err := dungeonspec.Load([]byte(v4With(goblinMind, bindingOnGoblin1("        behaviors: [{id: duty, use: typo}]\n"))))
	require.ErrorContains(t, err, "room.room.monsterBindings.goblin-1.behaviors[0].use")
	require.ErrorContains(t, err, `definition "typo"`)
}

func TestADefinitionDeclaredAndNeverNamedIsLegal(t *testing.T) {
	c := loadSource(t, v4With(goblinMind, ""))
	require.Len(t, c.Monsters, 2)
	require.Nil(t, c.Monsters[0].Behaviors, "a declaration alone installs no policy")
}

func TestADefectiveTableIsRefusedEvenWhenNobodyNamesIt(t *testing.T) {
	errs := refusals(t, v4With("tables:\n  goblin-mind:\n    taunted:\n      - {hold: {}}\n", ""))
	requireExactDefect(t, errs, "tables.goblin-mind.on.taunted", `"taunted" is not a trigger this build rolls: they are time`)
}

func TestANullTableIsRefusedRatherThanReadAsTheZeroValue(t *testing.T) {
	requireExactDefect(t, refusals(t, v4With("tables:\n  goblin-mind:\n", "")), "tables.goblin-mind", "must not be null")
}

func TestATableWithNoIdIsRefused(t *testing.T) {
	requireExactDefect(t, refusals(t, v4With("tables:\n  \"\":\n    time:\n      - {hold: {}}\n", "")), "tables", "a table has no id")
}

func TestActorTableLayeringIsRefusedNotEmulated(t *testing.T) {
	for _, old := range []string{"table: goblin-mind", "on: {time: [{hold: {}}]}"} {
		_, err := dungeonspec.Load([]byte(v4With(goblinMind, bindingOnGoblin1("        "+old+"\n"))))
		require.ErrorContains(t, err, "not imported")
	}
}
