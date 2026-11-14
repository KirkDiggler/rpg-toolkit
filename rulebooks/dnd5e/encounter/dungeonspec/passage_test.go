// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// passage_test.go is the authored half of A SECRET PASSAGE (rpg-project#958):
// a line's row `tells:` and `relocate:`, compiled from the castle's kitchen
// passage, and refused on the small tomb vault at the author's own path when
// they name what the site does not have.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func TestDungeonspec_APassageCompiles(t *testing.T) {
	c := loadSource(t, castleSource(t))
	favour := monsterNamed(t, c, "scullion").Options
	require.Len(t, favour, 1)
	require.Equal(t, []encounter.RowGrant{{Consequence: encounter.Consequence{Tells: "the-mad-king/kitchen-way"}}},
		favour[0].Grants, "the told record is named as the site qualifies it")
	var way *encounter.IntelRecord
	for i := range c.Field.Intel {
		if c.Field.Intel[i].ID == "the-mad-king/kitchen-way" {
			way = &c.Field.Intel[i]
		}
	}
	require.NotNil(t, way)
	require.Equal(t, encounter.ConcealmentID("the-mad-king/kitchen-passage"), way.Reveals.Concealment, "the record is the secret")

	var grate *encounter.PlacedPropInput
	for i := range c.Field.Placed {
		if c.Field.Placed[i].ID == "grate" {
			grate = &c.Field.Placed[i]
		}
	}
	require.NotNil(t, grate)
	require.Len(t, grate.Options, 2)
	require.Equal(t, &encounter.RelocateGrant{
		To: "region-chapel", Words: "The tunnel climbs, and you come up through a flagstone behind the chapel's altar.", Rounds: 1,
	}, grate.Options[0].Grants[0].Consequence.Relocate)

	var hides []encounter.PropID
	for _, k := range c.Field.Concealments {
		if k.ID == "the-mad-king/kitchen-passage" {
			hides = k.Props
		}
	}
	require.Equal(t, []encounter.PropID{"grate"}, hides, "the passage hides the grate")
}

// vaultWithAPassage is the tomb vault with a line on its captain whose
// answer tells the vault map and a line on its scroll that sets the reader
// down in the tomb: the small site the refusals are cut from, rewritten once
// with the fragment given.
func vaultWithAPassage(t *testing.T, old, replacement string) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/world-builder-v4-tomb-vault.yaml")
	require.NoError(t, err)
	source := strings.Replace(string(raw), "      captain: {faction: monsters,  holds: [vault-map] }\n",
		"      captain:\n        faction: monsters\n        holds: [vault-map]\n        options:\n          - id: ask\n            say: Where?\n"+
			"            success:\n              - say: Behind you.\n"+vaultTells, 1)
	source = strings.Replace(source, "      hall-scroll: { holdable: true, holds: [hall-notes] }\n",
		"      hall-scroll:\n        holdable: true\n        holds: [hall-notes]\n        options:\n          - id: read\n            say: (read it aloud)\n"+
			"            success:\n"+vaultRelocate, 1)
	require.Equal(t, 1, strings.Count(source, old), "the vault says %q once", old)
	return strings.Replace(source, old, replacement, 1)
}

const (
	vaultTells    = "                tells: vault-map\n"
	vaultRelocate = "              - relocate: { to: room-1-region, words: The floor turns. }\n"
)

func TestDungeonspec_APassageIsRefusedWhereItIsWrong(t *testing.T) {
	const asked = "room.room.monsterBindings.captain.options[0].success[0]"
	const read = "room.room.propBindings.hall-scroll.options[0].success[0]"
	_ = loadSource(t, vaultWithAPassage(t, vaultTells, vaultTells))
	for name, tc := range map[string]struct {
		old, replacement, path, message string
	}{
		"a told record the site does not have": {vaultTells, "                tells: cellar\n",
			asked + ".tells", `"cellar" is not an intel record of this site`},
		"a relocate to no region": {vaultRelocate, "              - relocate: { to: region-moon, words: Up. }\n",
			read + ".relocate.to", `"region-moon" is not a region of this site`},
		"a relocate with no words": {vaultRelocate, "              - relocate: { to: room-1-region }\n",
			read + ".relocate.words", "a relocate tells the speaker what happens: write its words"},
		"a relocate of negative rounds": {vaultRelocate, "              - relocate: { to: room-1-region, words: Up., rounds: -1 }\n",
			read + ".relocate.rounds", "a relocate's rounds are zero or more"},
		"two grants on a row": {vaultTells, vaultTells + "                relocate: { to: room-1-region, words: Up. }\n",
			asked, "a row grants one thing: `recognize`, `tells` or `relocate`, not two"},
	} {
		t.Run(name, func(t *testing.T) {
			requireExactDefect(t, refusals(t, vaultWithAPassage(t, tc.old, tc.replacement)), tc.path, tc.message)
		})
	}
}

// TestDungeonspec_APassageIsNotATableRow: a routine's row grants nothing.
func TestDungeonspec_APassageIsNotATableRow(t *testing.T) {
	errs := refusals(t, castleWith(t, "        - say: Not one step further without my say.\n",
		"        - say: Not one step further without my say.\n          tells: kitchen-way\n"))
	require.NotEmpty(t, errs)
	require.Equal(t, "tells is only on a line's success or failure row: a table row grants nothing", errs[0].Message)
}
