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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
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

// TestADefectiveTableIsRefusedEvenWhenNobodyNamesIt is the other half of the
// pair above, and it is the half that PINS the claim `singleRoomSite`'s comment
// makes ("a table is owned by nobody: that is the whole reason it is at the
// root").
//
// A table no binding names is still JUDGED. Without this case the suite would
// only show that a GOOD unnamed table compiles, which passes just as well if
// unused tables were skipped entirely — so the comment would be asserting
// something no test could falsify (independent review, finding 3).
func TestADefectiveTableIsRefusedEvenWhenNobodyNamesIt(t *testing.T) {
	errs := refusals(t, v4With(
		"tables:\n  goblin-mind:\n    taunted:\n      - { hold: {} }\n", ""))

	requireExactDefect(t, errs, "tables.goblin-mind.on.taunted",
		`"taunted" is not a trigger this build rolls: they are intimidated, intimidate_failed, `+
			"persuaded, persuade_failed, time")
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

// TestTheThreeLayersStackInOrder is the whole stack, in order: the binding's
// own `on:` OVER the named table OVER the faction's.
//
// EACH LAYER CONTRIBUTES A DIFFERENT NUMBER OF ROWS, AND THAT IS THE TEST
// (independent review, finding 1). The first version of this asserted
// `Len == 1` on a creature in NO faction, which passes under every possible
// ordering — including ignoring the named table entirely — so its name
// promised an order it could not detect. Row counts 3/2/1 make each layer
// individually visible: drop the binding's `on:` and the table must still beat
// the faction (2), and that middle assertion is the only thing in this suite
// that pins table-over-faction at all.
//
// THE FIXTURE'S OWN GOBLIN CANNOT CARRY A FACTION — `v4With` declares none, so
// every other v4 test would earn an undeclared-faction refusal. Hence a
// test-local document rather than a tweak to the shared helper.
func TestTheThreeLayersStackInOrder(t *testing.T) {
	// faction `time` = 1 row, table `time` = 2 rows, binding `time` = 3 rows —
	// so the count SAYS which layer won.
	const doc = `version: 4
key: three-layers
play: { void: transparent, lighting: bright, standing: centre-covered }
factions:
  - id: sneaks
    on:
      time:
        - { when: { enemy: reach }, attack: enemy }
tables:
  goblin-mind:
    time:
      - { when: { enemy: seen }, toward: enemy }
      - { when: { enemy: none }, hold: {} }
room:
  version: 3
  id: room-1
  name: A Room
  coordinateFrame: { hexRadius: 1 }
  workspace: { hexRadius: 6 }
  scene:
    version: 1
    id: scene-1
    name: A Room
    items: []
    groups: []
  room:
    implicitRegionId: room-1-region
    walkableHexes: [{q: 0, r: 0}, {q: 1, r: 0}, {q: 2, r: 0}]
    propDeclarations: {}
    arrangementDeclarations: {}
    partyStart: { q: 0, r: 0 }
    monsters:
      - { id: goblin-1, ref: 'dnd5e:monsters:goblin', startingCell: { location: { q: 2, r: 0 } }, faction: sneaks }
    monsterBindings:
      goblin-1:
        table: goblin-mind
        on:
          time:
            - { when: { enemy: remembered }, toward: enemy }
            - { when: { enemy: reach }, attack: enemy }
            - { when: { enemy: none }, hold: {} }
`

	compiled, err := dungeonspec.Load([]byte(doc))
	require.NoError(t, err)
	require.Len(t, monsterNamed(t, compiled, "goblin-1").Table[encounter.AnswerKey("time")], 3,
		"the binding's own `on:` is the NEAREST layer and wins the trigger wholesale")

	// Remove ONLY the binding's `on:`. The named table must still beat the
	// faction's one row — the middle layer, which nothing else here pins.
	withoutBindingOn := strings.Replace(doc,
		`        on:
          time:
            - { when: { enemy: remembered }, toward: enemy }
            - { when: { enemy: reach }, attack: enemy }
            - { when: { enemy: none }, hold: {} }
`, "", 1)
	require.NotEqual(t, doc, withoutBindingOn, "the binding's `on:` was removed for this case")

	compiled2, err := dungeonspec.Load([]byte(withoutBindingOn))
	require.NoError(t, err)
	require.Len(t, monsterNamed(t, compiled2, "goblin-1").Table[encounter.AnswerKey("time")], 2,
		"with no `on:` of its own, the NAMED TABLE beats the faction's row")
}

// TestAFactionNamesItsTable is `factions[].table` — the same mechanism as a
// binding's, one level up (rpg-toolkit#1897).
//
// A FACTION'S TABLE IS THE BASE ITS OWN `on:` LAYS OVER, so a faction writing
// BOTH keeps its own key and takes the table's other keys — nearest layer wins
// wholesale, exactly as everywhere else in the stack.
//
// THE ROW COUNTS ARE THE ASSERTION, and the first draft of this test had them
// backwards: it expected the TABLE to win `time`, which would have been the
// layering inverted. The faction's `on:` is NEARER than the table it names, so
// `time` is the faction's 1 row and the table's `intimidated` key survives.
func TestAFactionNamesItsTable(t *testing.T) {
	const doc = `version: 4
key: faction-table
play: { void: transparent, lighting: bright, standing: centre-covered }
factions:
  - id: watch
    table: watch-drill
    on:
      time:
        - { when: { enemy: reach }, attack: enemy }
tables:
  watch-drill:
    time:
      - { when: { enemy: seen }, toward: enemy }
      - { when: { enemy: none }, hold: {} }
    intimidated:
      - { say: "Fine!" }
room:
  version: 3
  id: room-1
  name: A Room
  coordinateFrame: { hexRadius: 1 }
  workspace: { hexRadius: 6 }
  scene:
    version: 1
    id: scene-1
    name: A Room
    items: []
    groups: []
  room:
    implicitRegionId: room-1-region
    walkableHexes: [{q: 0, r: 0}, {q: 1, r: 0}, {q: 2, r: 0}]
    propDeclarations: {}
    arrangementDeclarations: {}
    partyStart: { q: 0, r: 0 }
    monsters:
      - { id: goblin-1, ref: 'dnd5e:monsters:goblin', startingCell: { location: { q: 2, r: 0 } }, faction: watch }
`

	compiled, err := dungeonspec.Load([]byte(doc))
	require.NoError(t, err)
	table := monsterNamed(t, compiled, "goblin-1").Table
	require.Len(t, table[encounter.AnswerKey("time")], 1,
		"the faction's own `on:` is NEARER than the table it names and wins `time` wholesale")
	require.Contains(t, table, encounter.AnswerKey("intimidated"),
		"and a key only the NAMED TABLE carries reaches the member — which is the table arriving at all")
}

// TestAFactionTableThatNamesNothingIsRefused: the name must resolve, and the
// sentence names the faction and the id it wrote — [bindingTable]'s shape.
func TestAFactionTableThatNamesNothingIsRefused(t *testing.T) {
	errs := refusals(t, v4With(
		"factions:\n  - { id: watch, table: watch-dril }\ntables:\n  watch-drill:\n    time:\n      - { hold: {} }\n",
		""))

	requireExactDefect(t, errs, "factions[0].table",
		`faction "watch" names the table "watch-dril", and no table in this dungeon has that id`)
}
