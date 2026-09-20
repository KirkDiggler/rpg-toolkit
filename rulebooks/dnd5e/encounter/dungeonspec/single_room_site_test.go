// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const v4SitePath = "testdata/world-builder-v4-site.yaml"

// theSameCampInVersionTwo is the v4 site fixture's sides and orders written in
// the OTHER dialect, on geometry of its own.
//
// Every authored fact about what the creatures ARE is identical to
// `testdata/world-builder-v4-site.yaml`: the same faction with the same mix
// and the same inherited table, the same disposition waiting on the same fact,
// the same membership, and the same three creatures with the same overrides in
// the same order. Only the GEOMETRY differs, because the two dialects describe
// space differently and nothing in this claim is about space.
const theSameCampInVersionTwo = `
version: 2
key: front-room-site
name: The Front Room
orientation: pointy
void: transparent
regions:
  - id: room-1-region
    name: Front Room
    archetype: crypt
    lighting: { intensity: 1 }
    cells:
      - [[0,0],[1,0],[2,0],[3,0],[4,0]]
      - [[0,1],[1,1],[2,1],[3,1],[4,1]]
start: { at: [0,0] }
factions:
  - id: goblins
    temper: { coward: 2, soldier: 1, aggressive: 1 }
    on:
      intimidated:
        - { weight: 70, say: 'Fine! The cellar door is behind the barrels.', fact: goblin-cowed }
        - { weight: 30, say: 'Boss! BOSS!', flee: {} }
      time:
        - { when: { enemy: reach }, attack: enemy }
        - { when: { enemy: seen }, toward: enemy }
dispositions:
  - { between: [goblins, party], stance: hostile, until: { fact: goblin-cowed } }
place:
  - id: goblin-1
    ref: "dnd5e:monsters:goblin"
    at: [2,0]
    faction: goblins
    temper: coward
    actions: ["dnd5e:weapons:scimitar", "dnd5e:weapons:shortbow"]
    on:
      time:
        - { when: { enemy: reach }, hold: {} }
  - { id: skeleton-a, ref: "dnd5e:monsters:skeleton", at: [1,1], actions: ["dnd5e:weapons:shortsword"] }
  - { id: skeleton-b, ref: "dnd5e:monsters:skeleton", at: [0,1] }
`

// TestTheTwoDialectsCompileTheSameOrders is the claim the site keys exist to
// make: an author who writes a faction, a membership and a creature's orders
// in the SINGLE-ROOM dialect gets exactly what the v2 dialect has always
// produced for the same words.
//
// It compares the compiled MonsterPlacement fields the orders fill — the
// membership, the layered table, the temperament and the arms — FIELD BY
// FIELD, and deliberately not the whole struct: the region and the cell are
// geometry, and the two documents describe different rooms on purpose.
//
// The layering is the interesting half. The faction writes `intimidated` and
// `time`; goblin-1 writes only `time`; so the compiled table must carry the
// faction's `intimidated` untouched and the creature's `time` INSTEAD of the
// faction's, which is what "nearest key wins wholesale" means.
func TestTheTwoDialectsCompileTheSameOrders(t *testing.T) {
	raw, err := os.ReadFile(v4SitePath)
	require.NoError(t, err)
	site, err := Load(raw)
	require.NoError(t, err)
	camp, err := Load([]byte(theSameCampInVersionTwo))
	require.NoError(t, err)

	require.Len(t, site.Monsters, 3)
	require.Len(t, camp.Monsters, 3)
	for i := range site.Monsters {
		got, want := site.Monsters[i], camp.Monsters[i]
		t.Run(want.ID, func(t *testing.T) {
			require.Equal(t, want.ID, got.ID)
			require.Equal(t, want.Ref, got.Ref)
			require.Equal(t, want.Faction, got.Faction, "the membership, as authored")
			require.Equal(t, want.Table, got.Table, "the faction's table with the creature's laid over")
			require.Equal(t, want.Temper, got.Temper, "the creature's word, else the faction's mix")
			require.Equal(t, want.Actions, got.Actions, "verbatim, in the author's order")
		})
	}
	require.Equal(t, camp.Factions, site.Factions, "the faction of one takes its member as its mind")
	require.Equal(t, camp.Dispositions, site.Dispositions, "the until compiles to the same trigger")

	// The overrides actually override, rather than the two documents agreeing
	// because neither carried anything.
	goblin := site.Monsters[0]
	require.Equal(t, "coward", goblin.Temper.Word, "the creature's word beat the faction's mix")
	require.Empty(t, goblin.Temper.Mix, "and the mix was not dealt for it")
	require.Len(t, goblin.Table["intimidated"], 2, "inherited from the faction")
	require.Len(t, goblin.Table["time"], 1, "replaced wholesale, not merged with the faction's two")
	require.Equal(t, []string{"dnd5e:weapons:scimitar", "dnd5e:weapons:shortbow"}, goblin.Actions)

	// And a creature nobody authored anything for carries nothing at all —
	// the zero values a pre-v4 document produced.
	bare := site.Monsters[2]
	require.Empty(t, bare.Faction)
	require.Nil(t, bare.Table)
	require.Nil(t, bare.Actions)
	require.Equal(t, "", bare.Temper.Word)
	require.Empty(t, bare.Temper.Mix)
}

// TestSingleRoomSiteRefusals pins the PATH and the SENTENCE of every refusal
// the site keys add, because the World Builder draws each one on the thing it
// names: a path that moved would put a refusal on the wrong field, and a
// sentence that changed would leave the web's copy of it stale.
func TestSingleRoomSiteRefusals(t *testing.T) {
	raw, err := os.ReadFile(v4SitePath)
	require.NoError(t, err)
	cases := []struct {
		name, old, repl string
		want            FieldError
	}{
		{
			name: "a member of a faction nobody declared",
			old:  "{id: skeleton-a, ref: 'dnd5e:monsters:skeleton', cell: {q: 1, r: -1}}",
			repl: "{id: skeleton-a, ref: 'dnd5e:monsters:skeleton', cell: {q: 1, r: -1}, faction: bandits}",
			want: FieldError{
				Path: "room.room.monsters[1].faction",
				Message: "\"dnd5e:monsters:skeleton\" is in faction \"bandits\", and no faction in this dungeon " +
					"has that id — declare it under `factions:`",
			},
		},
		{
			name: "orders for a creature this room does not place",
			old:  "      skeleton-a:\n        actions: ['dnd5e:weapons:shortsword']\n",
			repl: "      skeleton-a:\n        actions: ['dnd5e:weapons:shortsword']\n" +
				"      ghost-9:\n        actions: ['dnd5e:weapons:club']\n",
			want: FieldError{Path: "room.room.monsterBindings.ghost-9", Message: "must name a live room monster"},
		},
		{
			name: "a cell selector, which this dialect has no frame for",
			old:  "            - {when: {enemy: reach}, hold: {}}",
			repl: "            - {when: {enemy: none}, toward: {at: [3, 4]}}",
			want: FieldError{
				Path: "room.room.monsterBindings.goblin-1.on.time[0].toward.at",
				Message: "at is not a place a single room can name yet: its cells are axial and the selector's " +
					"cell is an offset in a document orientation this dialect does not have; " +
					"name enemy, attacker or actor, or wait for the sites layer",
			},
		},
		{
			name: "a cell under a word that is not `toward`, which is still the frame's defect",
			old:  "            - {when: {enemy: reach}, hold: {}}",
			repl: "            - {when: {enemy: none}, attack: {at: [3, 4]}}",
			want: FieldError{
				Path: "room.room.monsterBindings.goblin-1.on.time[0].attack.at",
				Message: "at is not a place a single room can name yet: its cells are axial and the selector's " +
					"cell is an offset in a document orientation this dialect does not have; " +
					"name enemy, attacker or actor, or wait for the sites layer",
			},
		},
		{
			name: "a temperament this build does not ship",
			old:  "        temper: coward",
			repl: "        temper: brave",
			want: FieldError{
				Path:    "room.room.monsterBindings.goblin-1.temper",
				Message: "\"brave\" is not a temperament this build ships: they are soldier, coward, aggressive",
			},
		},
		{
			name: "a trigger nothing rolls",
			old:  "        on:\n          time:\n",
			repl: "        on:\n          sunrise:\n",
			want: FieldError{
				Path: "room.room.monsterBindings.goblin-1.on.sunrise",
				Message: "\"sunrise\" is not a trigger this build rolls: they are " +
					"intimidated, intimidate_failed, persuaded, persuade_failed, time",
			},
		},
		{
			name: "an action that is not a weapon",
			old:  "actions: ['dnd5e:weapons:shortsword']",
			repl: "actions: ['dnd5e:monsters:goblin']",
			want: FieldError{
				Path:    "room.room.monsterBindings.skeleton-a.actions[0]",
				Message: "\"dnd5e:monsters:goblin\" is not a weapon: an action names a weapon as dnd5e:weapons:<id>",
			},
		},
		{
			name: "a creature in the players' side",
			old:  "cell: {q: 2, r: 0}, faction: goblins}",
			repl: "cell: {q: 2, r: 0}, faction: party}",
			want: FieldError{
				Path:    "room.room.monsters[0].faction",
				Message: "\"dnd5e:monsters:goblin\" cannot be in `party`: that is the players' side",
			},
		},
		{
			name: "a membership authored as nothing",
			old:  "cell: {q: 2, r: 0}, faction: goblins}",
			repl: "cell: {q: 2, r: 0}, faction: ''}",
			want: FieldError{Path: "room.room.monsters[0].faction", Message: "must not be empty"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(swapOneIn(t, raw, tc.old, tc.repl))
			require.Error(t, err)
			require.ErrorIs(t, err, ErrBadSpec)
			var validation *ValidationError
			require.ErrorAs(t, err, &validation)
			require.Contains(t, validation.Errors, tc.want)
		})
	}
}

// TestUnknownKeysInsideTheSiteKeysAreNamed is the strictness half, and it
// reports the same way everything else here does: the key at the author's own
// address for it, and the keys that shape does take (rpg-project#481, R2).
//
// It used to read differently on purpose — `KnownFields(true)` refuses at
// DECODE and the path was said to be gone by then — and that was the defect,
// not the design. The path is still in the source, which the decoder was
// handed; unknown_key.go walks it back. Both dialects run the same
// translation, so a misspelled key reads the same whether it was written in a
// site document or a region chain.
func TestUnknownKeysInsideTheSiteKeysAreNamed(t *testing.T) {
	raw, err := os.ReadFile(v4SitePath)
	require.NoError(t, err)
	cases := []struct {
		name, old, repl string
		want            FieldError
	}{
		{
			name: "inside an orders block",
			old:  "        temper: coward",
			repl: "        temper: coward\n        mind: goblin-1",
			want: FieldError{
				Path:    "room.room.monsterBindings.goblin-1.mind",
				Message: `"mind" is not a key this build reads: they are actions, arrives, holds, intimidate, on, persuade, temper`,
			},
		},
		{
			name: "inside a faction",
			old:  "  - id: goblins",
			repl: "  - id: goblins\n    stance: hostile",
			want: FieldError{
				Path:    "factions[0].stance",
				Message: `"stance" is not a key this build reads: they are id, mind, on, temper`,
			},
		},
		{
			name: "beside a creature",
			old:  "cell: {q: 0, r: -1}}",
			repl: "cell: {q: 0, r: -1}, temper: coward}",
			want: FieldError{
				Path:    "room.room.monsters[2].temper",
				Message: `"temper" is not a key this build reads: they are cell, faction, id, ref`,
			},
		},
		{
			name: "at the root of the site document",
			old:  "key: front-room-site",
			repl: "key: front-room-site\nheight: 8",
			want: FieldError{
				Path: "height",
				Message: `"height" is not a key this build reads: ` +
					"they are dispositions, factions, intel, key, play, room, version",
			},
		},
		{
			name: "in the play block",
			old:  "play: {void: transparent,",
			repl: "play: {viod: transparent,",
			want: FieldError{
				Path:    "play.viod",
				Message: `"viod" is not a key this build reads: they are lighting, standing, void`,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(swapOneIn(t, raw, tc.old, tc.repl))
			require.Error(t, err)
			require.ErrorIs(t, err, ErrBadSpec)
			var validation *ValidationError
			require.ErrorAs(t, err, &validation)
			require.Len(t, validation.Errors, 1)
			require.Equal(t, tc.want, validation.Errors[0])
			require.NotContains(t, validation.Errors[0].Message, "dungeonspec.",
				"an author is told about their file, not about Go")
			require.NotContains(t, validation.Errors[0].Message, "not found in type")
		})
	}
}

// TestTheSiteDocumentReportsEveryUnknownKey is the list half, in the dialect
// the World Builder actually authors: two misspellings come back as two
// defects, so fixing one does not send the author round again for the next.
func TestTheSiteDocumentReportsEveryUnknownKey(t *testing.T) {
	raw, err := os.ReadFile(v4SitePath)
	require.NoError(t, err)
	source := swapOneIn(t, raw, "key: front-room-site", "key: front-room-site\nheight: 8")
	source = swapOneIn(t, source, "        temper: coward", "        tempre: coward")

	_, err = Load(source)
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)
	require.Equal(t, []FieldError{
		{Path: "height", Message: `"height" is not a key this build reads: ` +
			"they are dispositions, factions, intel, key, play, room, version"},
		{Path: "room.room.monsterBindings.goblin-1.tempre",
			Message: `"tempre" is not a key this build reads: they are actions, arrives, holds, intimidate, on, persuade, temper`},
	}, validation.Errors, "both of them, in the order they were written")
}

// TestThePresentationStaysTheCodecsToJudge is the rule the translation must
// not have quietly broken. `scene`, `coordinateFrame` and `workspace` are the
// World Builder's own content and this decoder reads only the values play
// depends on out of them (rpg-project#479, R3), so an unknown key in there is
// still no defect of this package's.
func TestThePresentationStaysTheCodecsToJudge(t *testing.T) {
	raw, err := os.ReadFile(v4SitePath)
	require.NoError(t, err)
	for name, swap := range map[string][2]string{
		"in a scene item":         {"        label: Table", "        label: Table\n        wobble: 3"},
		"in the coordinate frame": {"{horizontalPlane: world-xz,", "{horizontalPlain: world-xz,"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(swapOneIn(t, raw, swap[0], swap[1]))
			require.NoError(t, err, "strictness over the presentation belongs to the codec that owns it")
		})
	}
}

// TestAbsenceRoundTripsAsAbsence is what every room authored before the site
// keys existed depends on: a document that names none of them decodes and
// marshals back with none of them, rather than with the empty placeholders a
// naive `omitempty`-less shape would write.
func TestAbsenceRoundTripsAsAbsence(t *testing.T) {
	raw, err := os.ReadFile("testdata/world-builder-v3.yaml")
	require.NoError(t, err)
	decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	require.NoError(t, err)
	require.Nil(t, decoded.Spec.Factions)
	require.Nil(t, decoded.Spec.Dispositions)
	require.Nil(t, decoded.Spec.Room.Gameplay.MonsterBindings)
	require.Empty(t, decoded.Spec.Room.Gameplay.Monsters[0].Faction)

	encoded, err := yaml.Marshal(decoded.Spec)
	require.NoError(t, err)
	for _, key := range []string{"factions:", "dispositions:", "faction:", "monsterBindings:"} {
		require.NotContains(t, string(encoded), key, "a document that authored none of these writes none of them")
	}
	// And it is still the same document: re-decoding the marshaled bytes gives
	// back what was decoded, so "absent" survived the round trip as absence
	// rather than as a value that happens to print as nothing.
	//
	// Compared as DOCUMENTS, because the presentation rides as a [yaml.Node]
	// that remembers where it was authored and a re-emitted file has its own
	// line numbers (rpg-project#479).
	again, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: encoded})
	require.NoError(t, err)
	againBytes, err := yaml.Marshal(again.Spec)
	require.NoError(t, err)
	require.Equal(t, string(encoded), string(againBytes))
	require.Nil(t, again.Spec.Factions)
	require.Nil(t, again.Spec.Room.Gameplay.MonsterBindings)
}

// TestTheV4FixtureRoundTripsItsSiteKeys is the other direction: everything the
// site fixture authored survives a decode and a re-encode, so a builder that
// reads a file and writes it back cannot quietly drop a faction or an orders
// block.
func TestTheV4FixtureRoundTripsItsSiteKeys(t *testing.T) {
	raw, err := os.ReadFile(v4SitePath)
	require.NoError(t, err)
	decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
	require.NoError(t, err)

	require.Equal(t, 4, decoded.Spec.Version)
	require.Equal(t, 3, decoded.Spec.Room.Version, "the embedded room draft is its own artifact")
	require.Len(t, decoded.Spec.Factions, 1)
	require.Equal(t, map[string]int{"coward": 2, "soldier": 1, "aggressive": 1}, decoded.Spec.Factions[0].Temper.Mix)
	require.Len(t, decoded.Spec.Dispositions, 1)
	require.Equal(t, "goblin-cowed", decoded.Spec.Dispositions[0].Until.Fact)
	require.Equal(t, "goblins", decoded.Spec.Room.Gameplay.Monsters[0].Faction)
	require.Empty(t, decoded.Spec.Room.Gameplay.Monsters[1].Faction, "unauthored stays unauthored")

	bindings := decoded.Spec.Room.Gameplay.MonsterBindings
	require.Len(t, bindings, 2)
	require.Equal(t, "coward", bindings["goblin-1"].Temper)
	require.Len(t, bindings["goblin-1"].On["time"], 1)
	require.Equal(t, []string{"dnd5e:weapons:scimitar", "dnd5e:weapons:shortbow"}, bindings["goblin-1"].Actions)
	require.Empty(t, bindings["skeleton-a"].Temper)
	require.Nil(t, bindings["skeleton-a"].On)

	// The keys reach the bytes. A FULL re-decode of this marshal is not
	// asserted, and the reason is a PRE-EXISTING asymmetry in the v2 answer
	// types rather than anything the site keys did: [WhenSpec], [SelectorSpec]
	// and [TemperSpec] each read a compact spelling through a custom
	// UnmarshalYAML and have no matching MarshalYAML, so yaml.Marshal writes
	// their Go fields (`{enemy: reach, deed: "", within: 0, line: 12}`) and
	// the decoder refuses that as four conditions in one `when`. It is true of
	// a v2 dungeon with an `on:` block today, nothing in this repository
	// marshals one, and closing it is its own change.
	encoded, err := yaml.Marshal(decoded.Spec)
	require.NoError(t, err)
	for _, key := range []string{"factions:", "dispositions:", "faction: goblins", "monsterBindings:", "temper: coward"} {
		require.Contains(t, string(encoded), key)
	}
}

// TestTheSiteKeysAreNotGatedOnVersionFour records the versioning rule the seam
// established (rpg-toolkit#1824): a version says what a file MAY contain, and
// the keys arrive INSIDE it. There is no "v4 only" branch anywhere, so the
// same document declared at 3 is read the same way — which is what keeps the
// next key from needing a version of its own.
func TestTheSiteKeysAreNotGatedOnVersionFour(t *testing.T) {
	raw, err := os.ReadFile(v4SitePath)
	require.NoError(t, err)
	atV4, err := Load(raw)
	require.NoError(t, err)

	atV3, err := Load(swapOneIn(t, raw, "version: 4\nkey: front-room-site", "version: 3\nkey: front-room-site"))
	require.NoError(t, err)
	require.Equal(t, atV4, atV3, "the shape walk and the compiler are version-blind")
	require.Equal(t, "goblins", atV3.Monsters[0].Faction)
}

// TestTheSiteScopeShapeWalkNamesItsDefects covers the strict read of the new
// keys, and covers exactly what the TYPED DECODE CANNOT.
//
// A value of the wrong kind never reaches the shape walk — yaml.v3 refuses
// `on: []` outright, naming the line and the Go type, and [DecodeSingleRoom]
// returns on that error before the walk runs. What yaml.v3 reads WITHOUT A
// WORD is an authored null: `on: null` is a nil map, `temper: null` is the
// empty string, `monsterBindings: {goblin-1: null}` is an orders block that
// orders nothing. Each of those is what this walk is for, and each is named
// at its own path here.
func TestTheSiteScopeShapeWalkNamesItsDefects(t *testing.T) {
	raw, err := os.ReadFile(v4SitePath)
	require.NoError(t, err)
	cases := []struct {
		name, old, repl string
		want            FieldError
	}{
		{
			name: "an orders block authored as nothing",
			old:  "      skeleton-a:\n        actions: ['dnd5e:weapons:shortsword']",
			repl: "      skeleton-a: null",
			want: FieldError{Path: "room.room.monsterBindings.skeleton-a", Message: "must not be null"},
		},
		{
			name: "an answer table authored as nothing",
			old:  "        on:\n          time:\n            - {when: {enemy: reach}, hold: {}}",
			repl: "        on: null",
			want: FieldError{Path: "room.room.monsterBindings.goblin-1.on", Message: "must not be null"},
		},
		{
			name: "an arms list authored as nothing",
			old:  "        actions: ['dnd5e:weapons:scimitar', 'dnd5e:weapons:shortbow']",
			repl: "        actions: null",
			want: FieldError{Path: "room.room.monsterBindings.goblin-1.actions", Message: "must not be null"},
		},
		{
			name: "a site scope authored as nothing",
			old:  "dispositions:\n  - {between: [goblins, party], stance: hostile, until: {fact: goblin-cowed}}",
			repl: "dispositions: null",
			want: FieldError{Path: "dispositions", Message: "must not be null"},
		},
		{
			name: "a temperament authored as nothing",
			old:  "        temper: coward",
			repl: "        temper: null",
			want: FieldError{Path: "room.room.monsterBindings.goblin-1.temper", Message: "must not be null"},
		},
		{
			name: "a mind authored as nothing",
			old:  "  - id: goblins",
			repl: "  - id: goblins\n    mind: ''",
			want: FieldError{Path: "factions[0].mind", Message: "must not be empty"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: swapOneIn(t, raw, tc.old, tc.repl)})
			require.Error(t, err)
			var validation *ValidationError
			require.ErrorAs(t, err, &validation)
			require.Contains(t, validation.Errors, tc.want)
		})
	}
}

// TestTheFactionLevelCellSelectorIsRefusedToo is the `at` refusal one layer
// up (rpg-project#484, design §3). A faction's inherited `on:` block is judged
// by the same grammar a creature's own is, and a single room has no more of a
// frame for a cell there than it has inside a binding — so the sentence and
// the shape of the path are the same, with `factions[0]` in front of it
// instead of the creature's id.
//
// It was reachable and pinned by nothing until the grammar split; #1834's
// review named it and deferred it here.
func TestTheFactionLevelCellSelectorIsRefusedToo(t *testing.T) {
	raw, err := os.ReadFile(v4SitePath)
	require.NoError(t, err)
	source := swapOneIn(t, raw,
		"        - {when: {enemy: reach}, attack: enemy}",
		"        - {when: {enemy: none}, toward: {at: [3, 4]}}")

	_, err = Load(source)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBadSpec)
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)
	require.Contains(t, validation.Errors, FieldError{
		Path: "factions[0].on.time[0].toward.at",
		Message: "at is not a place a single room can name yet: its cells are axial and the selector's " +
			"cell is an offset in a document orientation this dialect does not have; " +
			"name enemy, attacker or actor, or wait for the sites layer",
	})
}

// TestTheV2DialectStillResolvesCellSelectors is the other side of the `at`
// refusal, and the reason the frame is an input the dialect supplies rather
// than a check in the shared grammar: the dialect that HAS an orientation
// still resolves a cell against its floor, and still refuses one that is not
// floor.
func TestTheV2DialectStillResolvesCellSelectors(t *testing.T) {
	// Legal in v2: the cell is floor, the selector is on `toward`.
	source := strings.Replace(theSameCampInVersionTwo,
		"        - { when: { enemy: reach }, hold: {} }",
		"        - { when: { enemy: none }, toward: { at: [3,1] } }", 1)
	compiled, err := Load([]byte(source))
	require.NoError(t, err)
	require.NotNil(t, compiled.Monsters[0].Table["time"][0].Toward.At)

	// And the v2 refusal is still the floor one, not the single room's.
	offFloor := strings.Replace(source, "toward: { at: [3,1] }", "toward: { at: [9,9] }", 1)
	_, err = Load([]byte(offFloor))
	require.Error(t, err)
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)
	require.Contains(t, validation.Errors, FieldError{
		Path: "place[0].on.time[0].toward.at", Message: "this walks to [9,9], which is not floor",
	})
}
