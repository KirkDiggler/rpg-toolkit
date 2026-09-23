// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// v4_root_tables_test.go is the answer tables at the root (rpg-toolkit#1897):
// a table declared ONCE, named by any number of bindings, with the binding's
// own `on:` still winning over it key by key.
//
// THE POINT OF THE KEY is that six goblins sharing a table write it once. So
// every test below asserts a fact about the COMPILED table rather than about
// the decoder's tolerance — a document that parses but hands the creature
// nothing would pass a shape test and fail the author.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// goblinMind is one root table with two triggers, so a test can tell a
// SHARED trigger from an OVERRIDDEN one.
const goblinMind = `tables:
  goblin-mind:
    time:
      - { when: { enemy: reach }, attack: enemy }
      - { when: { enemy: none }, hold: {} }
`

// bindingOnGoblin1 writes gameplay keys onto the fixture's first creature,
// indented to sit under `monsterBindings:` beside it.
func bindingOnGoblin1(binding string) string {
	return "    monsterBindings:\n      goblin-1:\n" + binding
}

// TestARootTableIsWhatABindingNamingItAnswers proves the reference RESOLVES:
// the creature carries the table's rows, not an empty one.
func TestARootTableIsWhatABindingNamingItAnswers(t *testing.T) {
	compiled := loadSource(t, v4With(goblinMind,
		bindingOnGoblin1("        table: goblin-mind\n")))

	table := monsterNamed(t, compiled, "goblin-1").Table
	require.Contains(t, table, encounter.AnswerKey("time"),
		"a creature naming a root table answers with it")
	require.Len(t, table[encounter.AnswerKey("time")], 2,
		"both of the table's rows reach the creature, in authored order")
}

// TestOneTableServesManyCreatures is THE REASON the key exists: the same table
// written once reaches two creatures, which is what six goblins used to pay
// six copies for.
func TestOneTableServesManyCreatures(t *testing.T) {
	compiled := loadSource(t, v4With(goblinMind,
		"    monsterBindings:\n"+
			"      goblin-1: { table: goblin-mind }\n"+
			"      goblin-2: { table: goblin-mind }\n"))

	first := monsterNamed(t, compiled, "goblin-1").Table
	second := monsterNamed(t, compiled, "goblin-2").Table
	require.Len(t, first[encounter.AnswerKey("time")], 2)
	require.Len(t, second[encounter.AnswerKey("time")], 2,
		"the second creature answers with the same table, from one declaration")
}

// TestABindingsOwnOnWinsOverTheTableItNames is the layering, at the one key
// the binding wrote. A creature may share a table and override a trigger
// without copying the rest — which is the whole difference between a base and
// a replacement.
func TestABindingsOwnOnWinsOverTheTableItNames(t *testing.T) {
	compiled := loadSource(t, v4With(goblinMind,
		bindingOnGoblin1(
			"        table: goblin-mind\n"+
				"        on:\n"+
				"          time:\n"+
				"            - { when: { enemy: seen }, toward: enemy }\n"+
				"            - { when: { enemy: remembered }, toward: enemy }\n"+
				"            - { when: { enemy: reach }, attack: enemy }\n"+
				"          intimidated:\n"+
				"            - { say: \"Fine!\" }\n")))

	table := monsterNamed(t, compiled, "goblin-1").Table
	require.Len(t, table[encounter.AnswerKey("time")], 3,
		"the binding's own `time:` REPLACES the table's — nearer layer wins wholesale")
	require.Contains(t, table, encounter.AnswerKey("intimidated"),
		"and a trigger the binding added is there beside it")
}

// TestARootTableWithNoNameIsRefused, and refused BY NAME — the id an author
// wrote, at their own path, so the sentence says what to fix rather than that
// something is wrong somewhere.
func TestATableNameThatNamesNothingIsRefused(t *testing.T) {
	errs := refusals(t, v4With(goblinMind,
		bindingOnGoblin1("        table: goblin-mind-the-typo\n")))

	requireExactDefect(t, errs, "room.room.monsterBindings.goblin-1.table",
		`"goblin-1" names the table "goblin-mind-the-typo", and no table in this dungeon has that id`)
}

// TestATableDeclaredAndNeverNamedIsLegal: a table waiting for its second
// creature is the reason to lift one to the root, so nothing counts
// references and an unused table is not a defect.
func TestATableDeclaredAndNeverNamedIsLegal(t *testing.T) {
	compiled := loadSource(t, v4With(goblinMind, ""))
	require.Len(t, compiled.Monsters, 2,
		"a document declaring a table no creature names still compiles")
}

// TestTheGrammarInsideATableIsTheOneGrammar: a table is judged by the SAME
// validator a binding's own `on:` gets, so a bad trigger inside a root table
// is refused at the table's own path — once, not once per creature naming it.
func TestTheGrammarInsideATableIsRefusedAtTheTablesPath(t *testing.T) {
	errs := refusals(t, v4With(
		"tables:\n  goblin-mind:\n    taunted:\n      - { hold: {} }\n",
		bindingOnGoblin1("        table: goblin-mind\n")))

	requireExactDefect(t, errs, "tables.goblin-mind.on.taunted",
		`"taunted" is not a trigger this build rolls: they are intimidated, intimidate_failed, `+
			"persuaded, persuade_failed, time")
}

// TestANullTableIsRefusedRatherThanReadAsTheZeroValue: `goblin-mind:` with
// nothing under it is an author who emptied a table, not one who authored an
// empty one — and the typed decode cannot tell those apart on its own.
func TestANullTableIsRefusedRatherThanReadAsTheZeroValue(t *testing.T) {
	errs := refusals(t, v4With("tables:\n  goblin-mind:\n", ""))
	requireExactDefect(t, errs, "tables.goblin-mind", "must not be null")
}

// TestATableWithNoIdIsRefused: the id is the map key here, so an empty one is
// a table nothing can ever name.
func TestATableWithNoIdIsRefused(t *testing.T) {
	errs := refusals(t, v4With("tables:\n  \"\":\n    time:\n      - { hold: {} }\n", ""))
	requireExactDefect(t, errs, "tables", "a table has no id")
}

// TestATableAndAFactionStillLayer is the three-source stack, in order: the
// binding's own `on:` over the named table over the faction's. Asserted
// together because the ORDER is the fact, and a test of any two of them would
// pass with the third in the wrong place.
func TestATableAndAFactionStillLayer(t *testing.T) {
	compiled := loadSource(t, v4With(
		"factions:\n"+
			"  - id: sneaks\n"+
			"    on:\n"+
			"      time:\n"+
			"        - { when: { enemy: reach }, attack: enemy }\n"+
			"        - { when: { enemy: seen }, toward: enemy }\n"+
			"        - { when: { enemy: none }, hold: {} }\n"+
			"tables:\n"+
			"  goblin-mind:\n"+
			"    time:\n"+
			"      - { when: { enemy: none }, toward: enemy }\n",
		"    monsterBindings:\n"+
			"      goblin-1:\n"+
			"        table: goblin-mind\n"+
			"        on:\n"+
			"          time:\n"+
			"            - { when: { enemy: remembered }, toward: enemy }\n"))

	// goblin-1 names no faction, so only the table and its own `on:` apply —
	// and its own one row wins the `time` trigger wholesale.
	table := monsterNamed(t, compiled, "goblin-1").Table
	require.Len(t, table[encounter.AnswerKey("time")], 1,
		"the binding's `on:` is the nearest layer and wins the trigger")
}
