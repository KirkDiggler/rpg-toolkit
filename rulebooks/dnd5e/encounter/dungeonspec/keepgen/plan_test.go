// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package keepgen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLoadPlan_Embedded: the embedded keep loads, names every room of
// FadedPez's keep on rpg-project#739, and is 10,000 hexes at scale 1 (plan
// G1), so the keep fixture is scale 1 exactly.
func TestLoadPlan_Embedded(t *testing.T) {
	p := DefaultPlan()
	var ids []string
	for _, r := range p.Rooms {
		ids = append(ids, r.ID)
	}
	require.ElementsMatch(t, []string{
		"gatehouse", "forecourt", "throne-room", "audience-approach", "bedrooms", "treasury", "hidden-stash",
		"chapel", "rectory", "crypt", "library", "apothecary", "enclosed-passage", "ballroom", "mausoleum",
		"banquet-hall", "cemetery", "kitchen", "larder", "servants-quarters", "mess-hall", "barracks", "tower",
		"armory", "training-yard", "shared-service-passage", "prison", "jailer", "stables",
	}, ids)
	require.Equal(t, "courtyard", p.Courtyard.ID)
	require.Equal(t, "west-grounds", p.Grounds.West.ID)
	require.Equal(t, "south-grounds", p.Grounds.South.ID)
	require.Equal(t, "gatehouse", p.PartyStart)
	require.Equal(t, []Exit{{ID: "gate", Room: "gatehouse"}, {ID: "postern", Room: "enclosed-passage"}}, p.Exits)
	n, err := planHexes(p)
	require.NoError(t, err)
	require.Equal(t, 10000, n)
}

// planWith is the embedded plan's text with one substitution, which must
// apply.
func planWith(t *testing.T, old, replacement string) string {
	t.Helper()
	require.Contains(t, string(embeddedPlan), old)
	return strings.Replace(string(embeddedPlan), old, replacement, 1)
}

// TestLoadPlan_RefusesUnknownField: strict decode names the key.
func TestLoadPlan_RefusesUnknownField(t *testing.T) {
	doc := planWith(t, "{ id: bedrooms, name: Bedrooms,", "{ id: bedrooms, colour: red, name: Bedrooms,")
	_, err := LoadPlan(strings.NewReader(doc))
	require.ErrorContains(t, err, "field colour not found")
}

// TestLoadPlan_RefusesDoorToUnknownRoom: a door names a region that is not
// in the plan, by its path.
func TestLoadPlan_RefusesDoorToUnknownRoom(t *testing.T) {
	doc := planWith(t, "- [tower, mess-hall]", "- [tower, wine-cellar]")
	_, err := LoadPlan(strings.NewReader(doc))
	require.ErrorContains(t, err, `[1]: "wine-cellar" is not a room, the courtyard or a grounds`)
}

// TestLoadPlan_RefusesMissingField: an absent required field is named, and
// every defect is reported, not only the first.
func TestLoadPlan_RefusesMissingField(t *testing.T) {
	doc := planWith(t, "kind: quarters, width: 12, depth: 16, side: north, lighting: 0.8 }",
		"width: 12, depth: 16, side: north }")
	_, err := LoadPlan(strings.NewReader(doc))
	require.ErrorContains(t, err, "rooms[0].kind: is required")
	require.ErrorContains(t, err, "rooms[0].lighting: is required")
}

// TestLoadPlan_RefusesBadValues: an unknown kind and side, a duplicate id,
// a door written twice, and a footprint outside the dialect's bounds.
func TestLoadPlan_RefusesBadValues(t *testing.T) {
	doc := string(embeddedPlan)
	for _, sub := range [][2]string{
		{"kind: quarters, width: 12", "kind: dungeon, width: 12"},
		{"side: north, lighting: 0.8 }", "side: up, lighting: 0.8 }"},
		{"{ id: treasury,", "{ id: bedrooms,"},
		{"- [tower, mess-hall]", "- [tower, mess-hall]\n  - [mess-hall, tower]"},
		{"table_fancy_01\", width: 1.4,", "table_fancy_01\", width: 13,"},
	} {
		require.Contains(t, doc, sub[0])
		doc = strings.Replace(doc, sub[0], sub[1], 1)
	}
	_, err := LoadPlan(bytes.NewReader([]byte(doc)))
	require.ErrorContains(t, err, `rooms[0].kind: "dungeon" is not a room kind`)
	require.ErrorContains(t, err, `rooms[0].side: "up" is not a side`)
	require.ErrorContains(t, err, `rooms[1].id: "bedrooms" is already rooms[0]`)
	require.ErrorContains(t, err, `"mess-hall" and "tower" already have a door`)
	require.ErrorContains(t, err, "palettes.hall.props[0].width: must be between 0.1 and 12")
}

// TestLoadPlan_RefusesMonsterWithoutStatBlock: a palette monster whose id
// has no entry under templates is refused at its path, naming the ref,
// because the launch resolves monsters only from the site's own templates.
func TestLoadPlan_RefusesMonsterWithoutStatBlock(t *testing.T) {
	doc := planWith(t, "    monsters: [dnd5e:monsters:guard]\n\n# The stat blocks",
		"    monsters: [dnd5e:monsters:dragon]\n\n# The stat blocks")
	_, err := LoadPlan(strings.NewReader(doc))
	require.ErrorContains(t, err, `palettes.grounds.monsters[0]: "dnd5e:monsters:dragon" has no stat block in templates`)

	cut := strings.Index(string(embeddedPlan), "\ntemplates:\n")
	require.Positive(t, cut)
	_, err = LoadPlan(strings.NewReader(string(embeddedPlan[:cut+1])))
	require.ErrorContains(t, err, `palettes.hall.monsters[0]: "dnd5e:monsters:guard" has no stat block in templates`)
}

// TestLoadPlan_RefusalsInOrder: two unknown palette kinds are reported in
// the same order every time.
func TestLoadPlan_RefusalsInOrder(t *testing.T) {
	doc := planWith(t, "palettes:\n", "palettes:\n  zeta: { props: [], monsters: [] }\n  alpha: { props: [], monsters: [] }\n")
	_, first := LoadPlan(strings.NewReader(doc))
	require.ErrorContains(t, first, "palettes.alpha: is not a room kind; palettes.zeta: is not a room kind")
	for range 20 {
		_, again := LoadPlan(strings.NewReader(doc))
		require.Equal(t, first.Error(), again.Error())
	}
}

// TestLoadPlan_SightBlockersAreG3: the embedded palettes block sight with
// exactly the kinds plan G3 names (shelves, statues, clothes racks, crate
// stacks) and nothing else.
func TestLoadPlan_SightBlockersAreG3(t *testing.T) {
	g3 := []string{"shelf_", "statue_", "clothes_rack_", "crate_stack_"}
	for kind, pal := range DefaultPlan().Palettes {
		for _, e := range pal.Props {
			named := false
			for _, prefix := range g3 {
				named = named || strings.HasPrefix(assetName(e.AssetRef), prefix)
			}
			require.Equal(t, named, *e.BlocksLineOfSight, "%s %s", kind, e.AssetRef)
			if named {
				require.True(t, *e.BlocksMovement, "%s %s: a sight blocker is a blocker", kind, e.AssetRef)
			}
		}
	}
}
