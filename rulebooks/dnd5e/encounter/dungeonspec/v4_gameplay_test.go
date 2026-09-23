// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// v4_gameplay_test.go is WHAT THE ROOM IS FOR, PROVED TWICE
// (rpg-project#488, R1/R3/R5; rpg-toolkit#1853).
//
// # The claim, and how it is tested
//
// The v4 dialect gained six gameplay keys in slices 2 and 3 — the root's
// `intel:`, four on a creature's orders and three on a prop's — and every one
// of them is a v2 key reaching this dialect unchanged. (Slice 1's three root
// keys are the same claim, proved the same way, in v4_run_test.go.) A claim
// of that shape is not proved by
// asserting that a field is populated — it is proved by authoring the SAME
// GAMEPLAY in both dialects and requiring the same compiled answer. So each
// of the two documents below is compiled beside its v2 counterpart and the
// fields the slice adds are required EQUAL, field by field.
//
// WHAT IS DELIBERATELY NOT COMPARED IS GEOMETRY. v2 paints regions and walls
// them; this dialect has one room and no walls. The two compile to different
// worlds on purpose, and comparing whole [dungeonspec.Compiled] values would
// be a test that cannot pass rather than one that cannot fail. What is
// compared is exactly what the slice claims to carry.
//
// NO COLLECTION LENGTH IS PINNED. Each assertion names the creatures it is
// about and compares their values; a creature added to either fixture changes
// what those assertions say, not a count somebody has to bump.

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// loadContent compiles one authored document off disk.
func loadContent(t *testing.T, path string) dungeonspec.Compiled {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	compiled, err := dungeonspec.Load(raw)
	require.NoError(t, err, "%s compiles", path)

	return compiled
}

// loadSource compiles one authored document held in the test.
func loadSource(t *testing.T, source string) dungeonspec.Compiled {
	t.Helper()
	compiled, err := dungeonspec.Load([]byte(source))
	require.NoError(t, err)

	return compiled
}

// monsterNamed is the compiled placement one id names, required to exist so a
// renamed fixture fails as a missing creature rather than as a nil field.
func monsterNamed(t *testing.T, compiled dungeonspec.Compiled, id string) dungeonspec.MonsterPlacement {
	t.Helper()
	for _, m := range compiled.Monsters {
		if m.ID == id {
			return m
		}
	}
	require.FailNowf(t, "no such creature", "%q is not in this compile", id)

	return dungeonspec.MonsterPlacement{}
}

// # The front room — the priced threat, the priced appeal, and an arrival

// frontRoomV2Gameplay is the gameplay half of
// `rpg-api/content/reference-front-room.yaml`, transcribed: the goblin's two
// priced verbs, and the bandits waiting on the lie.
//
// TRANSCRIBED RATHER THAN READ FROM THAT REPOSITORY, because the file is
// rpg-api's content and this package may not reach across a repository
// boundary for a fixture. The geometry here is the smallest that carries the
// placements; what is copied is the thing under test, and only that.
const frontRoomV2Gameplay = `
version: 2
key: reference-front-room
name: The Front Room
orientation: pointy
void: opaque
regions:
  - id: front-room
    name: The Front Room
    archetype: crypt
    lighting: { intensity: 0.7 }
    cells:
      - [[0,0],[1,0],[2,0],[3,0],[4,0],[5,0]]
      - [[0,1],[1,1],[2,1],[3,1],[4,1],[5,1]]
      - [[0,2],[1,2],[2,2],[3,2],[4,2],[5,2]]
      - [[0,3],[1,3],[2,3],[3,3],[4,3],[5,3]]
start: [1, 3]
factions:
  - { id: goblins }
  - { id: bandits }
dispositions:
  - { between: [goblins, party], stance: neutral }
  - { between: [bandits, party], stance: hostile }
place:
  - id: front-goblin
    ref: "dnd5e:monsters:goblin"
    faction: goblins
    at: [4, 3]
    actions: ["dnd5e:weapons:scimitar"]
    intimidate: [{ ability: intimidation, dc: 12 }]
    persuade: [{ ability: persuasion, dc: 10 }]
  - { id: front-goblin-2, ref: "dnd5e:monsters:goblin", faction: goblins, at: [4, 1] }
  - id: bandit-1
    ref: "dnd5e:monsters:bandit"
    faction: bandits
    at: [5, 0]
    actions: ["dnd5e:weapons:scimitar", "dnd5e:weapons:light-crossbow"]
    arrives: { fact: cellar-is-clear }
`

// The front room's priced verbs and its arrival compile identically in both
// dialects. The DCs are the whole point of the pair: DC 12 to lean on and DC
// 10 to talk round, where the goblin's own stat block would derive 9 for
// both.
func TestTheFrontRoomPricesTheSameCheckInBothDialects(t *testing.T) {
	v2 := loadSource(t, frontRoomV2Gameplay)
	v4 := loadContent(t, "testdata/world-builder-v4-front-room.yaml")

	t.Run("the priced creature", func(t *testing.T) {
		was, is := monsterNamed(t, v2, "front-goblin"), monsterNamed(t, v4, "front-goblin")
		require.Equal(t, []encounter.CheckApproach{{Ability: "intimidation", DC: 12}}, was.Intimidate,
			"the v2 reference prices the threat")
		require.Equal(t, was.Intimidate, is.Intimidate, "and v4 compiles the same approaches")
		require.Equal(t, []encounter.CheckApproach{{Ability: "persuasion", DC: 10}}, was.Persuade)
		require.Equal(t, was.Persuade, is.Persuade)
	})

	// OMITTED IS NIL, NOT EMPTY, and that distinction is what tells the
	// rulebook this creature carries no social verb (rpg-project#494 R1). A
	// creature nobody priced must compile to nothing in either dialect.
	t.Run("a creature nobody priced", func(t *testing.T) {
		was, is := monsterNamed(t, v2, "front-goblin-2"), monsterNamed(t, v4, "front-goblin-2")
		require.Nil(t, was.Intimidate)
		require.Nil(t, is.Intimidate, "v4 carries the same silence, and nil is how it says so")
		require.Nil(t, was.Persuade)
		require.Nil(t, is.Persuade)
	})

	t.Run("the bandits wait on the lie", func(t *testing.T) {
		was, is := monsterNamed(t, v2, "bandit-1"), monsterNamed(t, v4, "bandit-1")
		require.Equal(t, encounter.TriggerFact{Fact: "cellar-is-clear"}, was.Arrives)
		require.Equal(t, was.Arrives, is.Arrives, "the same predicate, compiled by the same [predicateOf]")
	})
}

// # The raider camp — the record, who holds it, and the two clocks

// Every record the v4 camp declares compiles to what the v2 camp's does,
// MINTED THE SAME WAY. The id is `<key>/<id>` in both, which is why the two
// keys differ in the comparison below and nothing else does: two dungeons in
// one process cannot collide, so the prefix is a fact about the file rather
// than about the record.
func TestTheRaiderCampDeclaresTheSameRecordInBothDialects(t *testing.T) {
	v2 := loadContent(t, "testdata/reference-raider-camp.yaml")
	v4 := loadContent(t, "testdata/world-builder-v4-raider-camp.yaml")

	require.Equal(t, []encounter.IntelRecord{{
		ID:      "reference-raider-camp/wisemans-letter",
		Reveals: encounter.RevealTargets{Fact: "saved-wiseman"},
	}}, v2.Intel, "the v2 camp's record")
	require.Equal(t, []encounter.IntelRecord{{
		ID:      "raider-camp-v4/wisemans-letter",
		Reveals: encounter.RevealTargets{Fact: "saved-wiseman"},
	}}, v4.Intel, "the same record, this file's key")

	// THE FIELD CARRIES IT, which is the copy that survives being stored: the
	// composition reads a record's reveals when it changes hands, so a table
	// kept only on Compiled would be lost the moment the dungeon was saved.
	require.Equal(t, v4.Intel, v4.Field.Intel, "the same list, surfaced twice")
}

// The holder carries the COMPILED record id, not the author's — the same
// minting on both sides, so a holder never names a record the composition
// does not have.
func TestAHolderCarriesTheCompiledRecordIDInBothDialects(t *testing.T) {
	v2 := loadContent(t, "testdata/reference-raider-camp.yaml")
	v4 := loadContent(t, "testdata/world-builder-v4-raider-camp.yaml")

	require.Equal(t, []string{"raider-camp-v4/wisemans-letter"},
		monsterNamed(t, v4, "messenger").Holds,
		"the v4 holder names the record by its compiled id")

	// TWO HOLDERS OF ONE RECORD IS A LAW, NOT A DUPLICATE: intel copies
	// rather than moving, so two guards may both know the way in and looting
	// either teaches it. Nothing in either dialect refuses it, and the two
	// carry the identical compiled id.
	require.Equal(t, monsterNamed(t, v4, "messenger").Holds, monsterNamed(t, v4, "chief").Holds,
		"the chief carries the same record, by the same id")

	// And a creature that holds nothing carries nil, which is what keeps a
	// document with no records picturing as it always did.
	require.Nil(t, monsterNamed(t, v4, "scout").Holds)

	// The v2 camp puts the record on a PROP rather than on a creature, which
	// is the one thing v4 cannot do yet (rpg-toolkit#1854). The minting is
	// what is claimed equal, and it is the same function on both paths.
	var v2Holder []encounter.IntelID
	for _, p := range v2.Field.Props {
		if p.ID == "letter" {
			v2Holder = p.Holds
		}
	}
	require.Equal(t, []encounter.IntelID{"reference-raider-camp/wisemans-letter"}, v2Holder,
		"the v2 holder names it the same way, one key over")
}

// Both predicate forms the camp authors compile to the composition's own
// triggers, and they are the triggers the v2 camp compiles.
func TestTheCampsTwoClocksCompileTheSameInBothDialects(t *testing.T) {
	v2 := loadContent(t, "testdata/reference-raider-camp.yaml")
	v4 := loadContent(t, "testdata/world-builder-v4-raider-camp.yaml")

	t.Run("a fall brings the reinforcements", func(t *testing.T) {
		was := monsterNamed(t, v2, "reinforcement-1")
		require.Equal(t, encounter.TriggerMemberDown{Member: "chief"}, was.Arrives)
		for _, id := range []string{"reinforcement-1", "reinforcement-2", "reinforcement-3"} {
			require.Equal(t, was.Arrives, monsterNamed(t, v4, id).Arrives, "%s waits for the chief", id)
		}
	})

	// The v2 camp's round-6 arrival is on the letter PROP; v4 authors the same
	// beat on the holder it can compile. What is claimed equal is the
	// TRIGGER — one predicate grammar, one [predicateOf], two dialects.
	t.Run("round six brings the record", func(t *testing.T) {
		var wasProp encounter.Trigger
		for _, p := range v2.Field.Props {
			if p.ID == "letter" {
				wasProp = p.Arrives
			}
		}
		require.Equal(t, encounter.TriggerRound{Round: 6}, wasProp, "the v2 letter arrives on round 6")
		require.Equal(t, wasProp, monsterNamed(t, v4, "messenger").Arrives,
			"and the v4 messenger arrives on the same round")
	})

	t.Run("a creature that is there from the first frame", func(t *testing.T) {
		require.Nil(t, monsterNamed(t, v4, "chief").Arrives)
		require.Nil(t, monsterNamed(t, v4, "scout").Arrives)
	})
}

// # The refusals, each at its exact path

// requireExactDefect asserts one exact path-and-sentence pair is in the list.
// EXACT, unlike [requireRefusedAt]'s substrings, because these sentences are
// the contract the World Builder draws on a form row: a paraphrase that still
// contains the words is a different sentence, and the author reads the whole
// thing.
//
// The list itself is not pinned. A second unrelated defect is a fixture
// problem, and pinning the whole list would make every test here fail when
// any one refusal changed.
func requireExactDefect(t *testing.T, errs []dungeonspec.FieldError, path, message string) {
	t.Helper()
	require.NotEmpty(t, errs, "this document must be refused")
	require.Contains(t, errs, dungeonspec.FieldError{Path: path, Message: message},
		"the author is told this, here")
}

// v4With builds a v4 document carrying the given root keys and room gameplay
// keys — the smallest legal single room, with a hole for whatever is being
// refused.
func v4With(root, gameplay string) string {
	return fmt.Sprintf(`
version: 4
key: refusal-fixture
play: { void: transparent, lighting: bright, standing: centre-covered }
%s
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
    items:
      - { id: table, kind: prop, transform: { x: 0, y: 0, z: 2, rotationY: 0 } }
    groups: []
  room:
    implicitRegionId: room-1-region
    walkableHexes: [{q: 0, r: 0}, {q: 1, r: 0}, {q: 2, r: 0}, {q: 0, r: 1}, {q: 1, r: 1}]
    propDeclarations:
      table: { blocksMovement: false, blocksLineOfSight: false, footprint: { width: 1, depth: 1, offsetX: 0, offsetZ: 0 } }
    arrangementDeclarations:
      nook:
        stamped-chair: { blocksMovement: false, blocksLineOfSight: false, footprint: { width: 1, depth: 1, offsetX: 0, offsetZ: 0 } }
    partyStart: { q: 0, r: 0 }
    monsters:
      - { id: goblin-1, ref: 'dnd5e:monsters:goblin', cell: { q: 2, r: 0 } }
      - { id: goblin-2, ref: 'dnd5e:monsters:goblin', cell: { q: 1, r: 1 } }
%s
`, root, gameplay)
}

// A record is refused exactly as v2 refuses one: an id it must have, an id no
// two may share, and something it must say it reveals.
func TestARecordIsRefusedAsV2RefusesOne(t *testing.T) {
	t.Run("no id", func(t *testing.T) {
		errs := refusals(t, v4With("intel:\n  - { reveals: { fact: a-fact } }", ""))
		requireExactDefect(t, errs, "intel[0].id", "the intel record has no id")
	})

	t.Run("two records under one id", func(t *testing.T) {
		errs := refusals(t, v4With(
			"intel:\n  - { id: letter, reveals: { fact: a-fact } }\n  - { id: letter, reveals: { fact: another } }", ""))
		requireExactDefect(t, errs, "intel[1].id", `intel "letter" is already declared at intel[0]`)
	})

	t.Run("reveals nothing", func(t *testing.T) {
		errs := refusals(t, v4With("intel:\n  - { id: letter, reveals: {} }", ""))
		requireExactDefect(t, errs, "intel[0].reveals",
			"intel \"letter\" does not say what it reveals — `concealment: <id>` or `fact: <id>`")
	})
}

// `reveals: { door }` is REFUSED in this dialect, RETARGETED rather than
// deleted (rpg-project#490, R7). The word names the hinge; what a record
// gives away is the secret holding it, and this dialect names that secret
// directly under root `concealments:`. So the author is told where the right
// word is rather than that the key does not exist.
func TestRevealingADoorPointsAtTheConcealmentInstead(t *testing.T) {
	errs := refusals(t, v4With("intel:\n  - { id: vault-map, reveals: { door: vault } }", ""))

	requireExactDefect(t, errs, "intel[0].reveals.door",
		"a door is not what a record gives away: reveal the concealment that holds it — "+
			"write `concealment: <id>` naming one of this room's `concealments`")

	// ONE DEFECT FOR ONE MISTAKE. A record naming both keys is an author who
	// wrote a word this dialect does not take beside one it does; telling
	// them the word is not built is the sentence that helps, and "a record
	// reveals exactly one thing" beside it is not.
	both := refusals(t, v4With("intel:\n  - { id: vault-map, reveals: { door: vault, fact: a-fact } }", ""))
	for _, e := range both {
		require.Equal(t, "intel[0].reveals.door", e.Path, "the door is the whole answer")
	}
}

// A holder naming a record nothing declares is refused by name, on either
// kind of binding.
func TestAHolderNamingNoRecordIsRefused(t *testing.T) {
	t.Run("on a creature", func(t *testing.T) {
		errs := refusals(t, v4With("", "    monsterBindings:\n      goblin-1:\n        holds: [vault-map]"))
		requireExactDefect(t, errs, "room.room.monsterBindings.goblin-1.holds[0]",
			`"goblin-1" holds intel "vault-map", and no record in this dungeon has that id`)
	})

	t.Run("on a prop", func(t *testing.T) {
		errs := refusals(t, v4With("", "    propBindings:\n      table:\n        holds: [vault-map]"))
		requireExactDefect(t, errs, "room.room.propBindings.table.holds[0]",
			`"table" holds intel "vault-map", and no record in this dungeon has that id`)
	})
}

// An authored check with no way through it is refused by name — nil is the
// author saying nothing, `[]` is the author saying there is a price and not
// saying what it is. [RoomDoorBinding.Locked]'s law, one binding kind over.
func TestAnEmptySocialCheckIsRefused(t *testing.T) {
	t.Run("intimidate", func(t *testing.T) {
		errs := refusals(t, v4With("", "    monsterBindings:\n      goblin-1:\n        intimidate: []"))
		requireExactDefect(t, errs, "room.room.monsterBindings.goblin-1.intimidate",
			"this monster declares an intimidate check with no way through it — an ability and a DC")
	})

	t.Run("persuade", func(t *testing.T) {
		errs := refusals(t, v4With("", "    monsterBindings:\n      goblin-1:\n        persuade: []"))
		requireExactDefect(t, errs, "room.room.monsterBindings.goblin-1.persuade",
			"this monster declares a persuade check with no way through it — an ability and a DC")
	})

	// AND EVERY PER-APPROACH REFUSAL IS THE ONE SHARED GRAMMAR'S, so a row
	// with no ability and a DC nothing can beat says what it says in either
	// dialect.
	t.Run("an approach that rolls nothing", func(t *testing.T) {
		errs := refusals(t, v4With("",
			"    monsterBindings:\n      goblin-1:\n        intimidate: [{ dc: 0 }]"))
		requireExactDefect(t, errs, "room.room.monsterBindings.goblin-1.intimidate[0].ability",
			"the approach does not say which ability it rolls")
		requireExactDefect(t, errs, "room.room.monsterBindings.goblin-1.intimidate[0].dc",
			"an approach with dc 0 has nothing to beat")
	})
}

// An `arrives:` predicate is judged by the one predicate grammar, and the two
// liveness rules v2 applies apply here: nothing waits on its own fall, and a
// ring of reserved creatures never arrives.
func TestAnArrivalThatCanNeverHoldIsRefused(t *testing.T) {
	t.Run("a round counted from zero", func(t *testing.T) {
		errs := refusals(t, v4With("",
			"    monsterBindings:\n      goblin-1:\n        arrives: { round: 0 }"))
		requireExactDefect(t, errs, "room.room.monsterBindings.goblin-1.arrives.round",
			"round 0: a round is counted from 1")
	})

	t.Run("a fall nobody in this room can take", func(t *testing.T) {
		errs := refusals(t, v4With("",
			"    monsterBindings:\n      goblin-1:\n        arrives: { down: nobody }"))
		requireExactDefect(t, errs, "room.room.monsterBindings.goblin-1.arrives.down",
			`"nobody" is not a placement in this dungeon`)
	})

	t.Run("its own fall", func(t *testing.T) {
		errs := refusals(t, v4With("",
			"    monsterBindings:\n      goblin-1:\n        arrives: { down: goblin-1 }"))
		requireExactDefect(t, errs, "room.room.monsterBindings.goblin-1.arrives.down",
			`"goblin-1" cannot wait for its own fall — it is not here to fall until it arrives`)
	})

	// A RING NEVER ARRIVES: each waits for a fall that cannot happen until it
	// has arrived itself. Reported at EVERY member, in the words of the one
	// the author is looking at.
	t.Run("a ring", func(t *testing.T) {
		errs := refusals(t, v4With("", "    monsterBindings:\n"+
			"      goblin-1:\n        arrives: { down: goblin-2 }\n"+
			"      goblin-2:\n        arrives: { down: goblin-1 }"))
		requireExactDefect(t, errs, "room.room.monsterBindings.goblin-1.arrives.down",
			`"goblin-1" waits for "goblin-2" to fall, and "goblin-2" is waiting to arrive on a fall of `+
				"its own that leads back here — none of them can ever arrive")
		requireExactDefect(t, errs, "room.room.monsterBindings.goblin-2.arrives.down",
			`"goblin-2" waits for "goblin-1" to fall, and "goblin-1" is waiting to arrive on a fall of `+
				"its own that leads back here — none of them can ever arrive")
	})

	// A creature waiting on a fall that CAN happen is legal: a creature
	// standing there from the first frame falls like anybody.
	t.Run("a fall that can happen", func(t *testing.T) {
		compiled := loadSource(t, v4With("",
			"    monsterBindings:\n      goblin-1:\n        arrives: { down: goblin-2 }"))
		require.Equal(t, encounter.TriggerMemberDown{Member: "goblin-2"},
			monsterNamed(t, compiled, "goblin-1").Arrives)
	})
}

// An authored null under any of the new keys is refused: absence means "not
// supplied" and null means "supplied nothing", and reading both as the Go
// zero value would lose the distinction this document keeps everywhere else.
func TestAnAuthoredNullUnderANewKeyIsRefused(t *testing.T) {
	for _, tc := range []struct {
		key  string
		path string
		root string
		room string
	}{
		{key: "intel", path: "intel[0]", root: "intel:\n  -"},
		{key: "exits", path: "exits[0]", root: "exits:\n  -"},
		{key: "an exit's cell", path: "exits[0].cell", root: "exits:\n  - { id: entrance, cell: }"},
		{key: "endings", path: "endings[0]", root: "endings:\n  -"},
		{key: "when", path: "endings[0].when", root: "endings:\n  - { id: held-out, when: }"},
		{key: "a scenario", path: "scenarios.recover-the-artifact",
			root: "scenarios:\n  recover-the-artifact:"},
		{key: "holds", path: "room.room.monsterBindings.goblin-1.holds",
			room: "    monsterBindings:\n      goblin-1:\n        holds:"},
		{key: "intimidate", path: "room.room.monsterBindings.goblin-1.intimidate",
			room: "    monsterBindings:\n      goblin-1:\n        intimidate:"},
		{key: "persuade", path: "room.room.monsterBindings.goblin-1.persuade",
			room: "    monsterBindings:\n      goblin-1:\n        persuade:"},
		{key: "arrives", path: "room.room.monsterBindings.goblin-1.arrives",
			room: "    monsterBindings:\n      goblin-1:\n        arrives:"},
		{key: "a prop binding", path: "room.room.propBindings.table",
			room: "    propBindings:\n      table:"},
		{key: "holdable", path: "room.room.propBindings.table.holdable",
			room: "    propBindings:\n      table:\n        holdable:"},
	} {
		t.Run(tc.key, func(t *testing.T) {
			requireExactDefect(t, refusals(t, v4With(tc.root, tc.room)), tc.path, "must not be null")
		})
	}
}

// # Unknown keys inside the new blocks

// A typo inside any of the new keys is named AT ITS V4 PATH, with the keys
// that shape does take (rpg-project#481, R2) — the contract the World Builder
// draws a refusal on.
//
// NOTHING IN unknown_key.go CHANGED TO MAKE THIS TRUE, and that is what this
// test records. The path comes from walking the author's own source and the
// key list is reflected off the two dialect roots, so a shape reachable from
// [dungeonspec.SingleRoomSpec] is mapped the day it exists. A list written by
// hand would be a second spelling of the grammar, and a second spelling
// drifts.
func TestATypoInsideTheNewKeysIsNamedAtItsV4Path(t *testing.T) {
	for _, tc := range []struct {
		name    string
		root    string
		room    string
		path    string
		message string
	}{
		{
			name: "inside a record's reveals",
			root: "intel:\n  - { id: letter, reveals: { dor: vault } }",
			path: "intel[0].reveals.dor",
			// THE LIST OFFERS `door`, WHICH THIS DIALECT THEN REFUSES (R3),
			// and that is the [RoomDoorBinding.Concealed] precedent rather
			// than a defect: the field stays on the shared shape so the
			// refusal can be a sentence at its own path, and the key list is
			// reflected off that shape.
			message: `"dor" is not a key this build reads: they are concealment, door, fact`,
		},
		{
			name:    "inside a way out",
			root:    "exits:\n  - { id: entrance, cel: { q: 0, r: 0 } }",
			path:    "exits[0].cel",
			message: `"cel" is not a key this build reads: they are cell, id`,
		},
		{
			name:    "inside an ending",
			root:    "endings:\n  - { id: held-out, whn: { round: 6 } }",
			path:    "endings[0].whn",
			message: `"whn" is not a key this build reads: they are id, when`,
		},
		{
			// A PREDICATE READS ITSELF BY HAND here too, so an ending's
			// `when` is named at its path with no list — [PredicateSpec]'s
			// own refusal, reached from the third sink that takes one.
			name:    "inside an ending's predicate",
			root:    "endings:\n  - { id: held-out, when: { rounds: 6 } }",
			path:    "endings[0].when.rounds",
			message: `"rounds" is not a key this build reads`,
		},
		{
			name:    "inside a priced check's approach",
			room:    "    monsterBindings:\n      goblin-1:\n        intimidate: [{ abilty: str, dc: 5 }]",
			path:    "room.room.monsterBindings.goblin-1.intimidate[0].abilty",
			message: `"abilty" is not a key this build reads: they are ability, dc, tool`,
		},
		{
			name:    "inside a prop's orders",
			room:    "    propBindings:\n      table: { holdabel: true }",
			path:    "room.room.propBindings.table.holdabel",
			message: `"holdabel" is not a key this build reads: they are arrives, holdable, holds`,
		},
		{
			// A PREDICATE READS ITSELF BY HAND, so the list is omitted rather
			// than guessed: a struct's fields are not that shape's grammar.
			// The PATH is still the author's own, which is the half that
			// matters for a form.
			name:    "inside a creature's arrival",
			room:    "    monsterBindings:\n      goblin-1:\n        arrives: { rounds: 2 }",
			path:    "room.room.monsterBindings.goblin-1.arrives.rounds",
			message: `"rounds" is not a key this build reads`,
		},
		{
			name:    "inside a prop's arrival",
			room:    "    propBindings:\n      table: { arrives: { rounds: 2 } }",
			path:    "room.room.propBindings.table.arrives.rounds",
			message: `"rounds" is not a key this build reads`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireExactDefect(t, refusals(t, v4With(tc.root, tc.room)), tc.path, tc.message)
		})
	}
}

// The root's own key list gained `intel`, then `exits`, `endings` and
// `scenarios` (rpg-project#488, slice 1), and a creature's orders gained four
// — reflected, not listed, so an author offered a key always gets one that
// exists.
func TestTheNewKeysAreOfferedByTheUnknownKeyRefusal(t *testing.T) {
	requireExactDefect(t, refusals(t, v4With("heigth: 3", "")), "heigth",
		`"heigth" is not a key this build reads: they are concealments, dispositions, endings, exits, factions, intel, key, play, room, scenarios, tables, version`)

	requireExactDefect(t, refusals(t, v4With("", "    monsterBindings:\n      goblin-1: { tempre: coward }")),
		"room.room.monsterBindings.goblin-1.tempre",
		`"tempre" is not a key this build reads: they are actions, arrives, holds, intimidate, on, persuade, table, temper`)
}

// A binding whose creature is gone is named ONCE. Its contents are not asked
// a second question: the declaration cannot outlive the thing it names, and
// two defects for one mistake sends an author looking for a second problem.
func TestABindingWithNoLiveCreatureIsNamedOnce(t *testing.T) {
	errs := refusals(t, v4With("", "    monsterBindings:\n      ghost:\n"+
		"        holds: [vault-map]\n        intimidate: []\n        arrives: { round: 0 }"))

	requireExactDefect(t, errs, "room.room.monsterBindings.ghost", "must name a live room monster")
	for _, e := range errs {
		require.Equal(t, "room.room.monsterBindings.ghost", e.Path,
			"nothing inside a dead binding is judged; got %q: %s", e.Path, e.Message)
	}
}
