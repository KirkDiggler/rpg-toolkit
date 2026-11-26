// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// custom_items_test.go is the authored half of A SITE FILE SAYS CUSTOM ITEMS
// (rpg-project#850, rpg-project#1166): the root's `customItems:`, a creature's
// `equipment:` and a prop's `item:` — judged for their SHAPE and carried,
// unread, into the compiled site. Nothing in a run uses them yet.
//
// The site is testdata/custom-items/site.yaml, a copy of
// world-builder-v4-raider-letter.yaml cut down to the three words. It lives in
// a directory of its own so the content-picture net, which takes every
// testdata/*.yaml, does not.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

const customItemsSite = "testdata/custom-items/site.yaml"

// cloakBlock is the cloak as canonical JSON: keys sorted at every depth, which
// is what encoding/json writes for a map.
const cloakBlock = `{"charges":{"max":1,"refills":{"on":["short rest","long rest"]}},` +
	`"lets":[{"cost":{"charges":1},"use":"second wind","with":{"level":1}}],` +
	`"name":"Lieutenant's cloak","slot":"cloak"}`

// customItemsWith is the site with the first place it writes old changed.
func customItemsWith(t *testing.T, old, replacement string) string {
	t.Helper()
	raw, err := os.ReadFile(customItemsSite) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	source := string(raw)
	require.Equal(t, 1, strings.Count(source, old), "the site says %q once", old)
	return strings.Replace(source, old, replacement, 1)
}

// placedNamed is the compiled placed prop one id names.
func placedNamed(t *testing.T, c dungeonspec.Compiled, id string) encounter.PlacedPropInput {
	t.Helper()
	for _, p := range c.Field.Placed {
		if p.ID == encounter.PropID(id) {
			return p
		}
	}
	require.FailNowf(t, "no such prop", "%q is not placed in this compile", id)
	return encounter.PlacedPropInput{}
}

// Test 1: the settled words decode and pass the grammar.
func TestDungeonspec_CustomItemsDecodeAndPassTheGrammar(t *testing.T) {
	raw, err := os.ReadFile(customItemsSite) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	require.Empty(t, refusals(t, string(raw)))
}

// Test 3: SHAPE ONLY. The format does not know which slots exist or which
// abilities a `use` names: that is the rules' table, which this module may not
// import, and they refuse these at launch. If this module refused them it
// would be a second, drifting copy of that table.
func TestDungeonspec_CustomItemsAreJudgedForShapeNotForWhatIsInside(t *testing.T) {
	require.Empty(t, refusals(t, customItemsWith(t, "    slot: cloak\n", "    slot: cape\n")),
		"a slot nobody has heard of is the rules' to refuse")
	require.Empty(t, refusals(t, customItemsWith(t, "      - use: second wind\n", "      - use: third wind\n")),
		"an ability nobody has heard of is the rules' to refuse")
	require.Empty(t, refusals(t, customItemsWith(t, "    charges: { max: 1, refills: { on: [short rest, long rest] } }\n",
		"    charges: { bananas: 3 }\n")), "what is inside charges is the rules' to judge")
}

// Test 2: refused, each at its own path, in words that name the item or the
// node and the word.
func TestDungeonspec_CustomItemsAreRefusedWhereTheyAreWrong(t *testing.T) {
	const (
		item    = "customItems.lieutenants-cloak"
		binding = "room.room.monsterBindings.lieutenant"
		prop    = "room.room.propBindings.cloak-on-the-peg"
		equip   = "        equipment: [lieutenants-cloak]\n"
		itemKey = "        item: lieutenants-cloak\n"
		letsOld = "    lets:\n      - use: second wind\n        with: { level: 1 }\n        cost: { charges: 1 }\n"
		lets    = "a custom item needs lets as a non-empty list: what it lets its wearer do"
		declare = "(the custom items declared are lieutenants-cloak)"
	)
	for name, tc := range map[string]struct {
		old, replacement, path, message string
	}{
		"an unknown word": {"    slot: cloak\n", "    slot: cloak\n    colour: red\n",
			item + ".colour", `"colour" is not a word of a custom item: a custom item is written with name, slot, unique, charges, lets`},
		"no name": {"    name: Lieutenant's cloak\n", "",
			item + ".name", "a custom item needs a name: the words a player sees"},
		"an empty name": {"    name: Lieutenant's cloak\n", "    name: \"\"\n",
			item + ".name", "a custom item needs a name: the words a player sees"},
		"a name that is a number": {"    name: Lieutenant's cloak\n", "    name: 7\n",
			item + ".name", "a custom item needs a name: the words a player sees"},
		"no slot": {"    slot: cloak\n", "",
			item + ".slot", "a custom item needs a slot: where it is worn"},
		"an empty slot": {"    slot: cloak\n", "    slot: \"\"\n",
			item + ".slot", "a custom item needs a slot: where it is worn"},
		"no lets":                 {letsOld, "", item + ".lets", lets},
		"empty lets":              {letsOld, "    lets: []\n", item + ".lets", lets},
		"lets that is not a list": {letsOld, "    lets: { use: second wind }\n", item + ".lets", lets},
		"charges that is not an object": {"    charges: { max: 1, refills: { on: [short rest, long rest] } }\n", "    charges: 1\n",
			item + ".charges", "charges, when written, is a mapping: how many uses the item has and when they come back"},
		"a value JSON cannot carry": {"        with: { level: 1 }\n", "        with: { level: .nan }\n",
			item, "the value NaN at lets[0].with.level cannot be written: a custom item holds only text, whole numbers, yes/no, and lists and maps of those"},
		"an id with a space": {"  lieutenants-cloak:\n    name", "  \"lieutenants cloak\":\n    name",
			"customItems.lieutenants cloak", `"lieutenants cloak" is not a custom item id: write letters, digits, - and _`},
		"equipment naming an item that is not declared": {equip, "        equipment: [cape]\n",
			binding + ".equipment[0]", `"lieutenant" has the item "cape" equipped, and no custom item in this site has that id ` + declare},
		"equipment repeating one": {equip, "        equipment: [lieutenants-cloak, lieutenants-cloak]\n",
			binding + ".equipment[1]", `"lieutenant" has the item "lieutenants-cloak" equipped twice: an item is equipped once`},
		"an item prop naming an item that is not declared": {itemKey, "        item: cape\n",
			prop + ".item", `"cloak-on-the-peg" is the item "cape", and no custom item in this site has that id ` + declare},
		"an item prop that is holdable": {itemKey, itemKey + "        holdable: true\n",
			prop + ".holdable", `"cloak-on-the-peg" is an item and is holdable: a thing is held in the hands or taken as an item, not both: write item or holdable`},
		"an item prop that holds intel": {itemKey, itemKey + "        holds: [wisemans-letter]\n",
			prop + ".holds", `"cloak-on-the-peg" is an item and holds intel: what an item teaches is not built: write item or holds`},
	} {
		t.Run(name, func(t *testing.T) {
			source := customItemsWith(t, tc.old, tc.replacement)
			if strings.Contains(tc.replacement, "wisemans-letter") {
				// A record the site does declare, so the one defect is the pairing.
				source = strings.Replace(source, "\nroom:\n", "\nintel:\n  - { id: wisemans-letter, reveals: { fact: saved } }\nroom:\n", 1)
			}
			requireExactDefect(t, refusals(t, source), tc.path, tc.message)
		})
	}
}

// A SITE THAT DECLARES NO ITEM SAYS SO when a creature has one equipped.
func TestDungeonspec_EquipmentNeedsADeclaration(t *testing.T) {
	raw, err := os.ReadFile(customItemsSite) //nolint:gosec // a test reading its own fixture
	require.NoError(t, err)
	i, j := strings.Index(string(raw), "customItems:"), strings.Index(string(raw), "\nroom:\n")
	source := string(raw)[:i] + string(raw)[j:]
	requireExactDefect(t, refusals(t, source), "room.room.monsterBindings.lieutenant.equipment[0]",
		`"lieutenant" has the item "lieutenants-cloak" equipped, and no custom item in this site has that id (this site declares no custom items)`)
}

// Test 4: compiled. The creature's placement and the prop's input each carry
// the grant: the id, and the author's block as canonical JSON, byte for byte.
func TestDungeonspec_ACustomItemCompilesIntoWhoHasItAndWhichPropIsOne(t *testing.T) {
	c := loadContent(t, customItemsSite)

	lieutenant := monsterNamed(t, c, "lieutenant")
	require.Len(t, lieutenant.Equipment, 1)
	require.Equal(t, "lieutenants-cloak", lieutenant.Equipment[0].ID)
	require.Equal(t, cloakBlock, string(lieutenant.Equipment[0].Item), "canonical JSON, byte for byte")

	cloak := placedNamed(t, c, "cloak-on-the-peg")
	require.NotNil(t, cloak.Item)
	require.Equal(t, encounter.ItemGrant{ID: "lieutenants-cloak", Name: "Lieutenant's cloak", Item: []byte(cloakBlock)}, *cloak.Item)
	require.False(t, cloak.Holdable, "an item prop is not held in the hands")
}

func TestDungeonspec_TwoCreaturesNamingOneItemEachCarryTheirOwnCopy(t *testing.T) {
	source := customItemsWith(t, "    monsterBindings:\n",
		"    monsterBindings:\n      sergeant:\n        faction: garrison\n        equipment: [lieutenants-cloak]\n")
	source = strings.Replace(source, "    monsterDeclarations:\n",
		"    monsterDeclarations:\n      - {id: sergeant, ref: 'dnd5e:monsters:skeleton-captain', startingCell: { location: { q: 1, r: 1 } }}\n", 1)
	c := loadSource(t, source)
	a, b := monsterNamed(t, c, "lieutenant"), monsterNamed(t, c, "sergeant")
	require.Equal(t, a.Equipment[0].Item, b.Equipment[0].Item)
	a.Equipment[0].Item[0] = 'X'
	require.Equal(t, cloakBlock, string(b.Equipment[0].Item), "a copy each: one cannot reach the other")
	require.Equal(t, cloakBlock, string(placedNamed(t, c, "cloak-on-the-peg").Item.Item), "nor the prop's")
}

// A creature with no equipment and a prop that is no item carry nothing: the
// shape every site before these words compiled to.
func TestDungeonspec_ASiteWithoutTheWordsCarriesNothing(t *testing.T) {
	c := loadContent(t, "testdata/world-builder-v4-raider-letter.yaml")
	for _, m := range c.Monsters {
		require.Nil(t, m.Equipment)
	}
	for _, p := range c.Field.Placed {
		require.Nil(t, p.Item)
	}
}
