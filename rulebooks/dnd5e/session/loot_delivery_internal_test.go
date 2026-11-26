// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
)

// Test 5. An item's id on the looter's sheet is the site's item.
func TestSiteItemID(t *testing.T) {
	id := siteItemID("castle", "lieutenants-cloak")
	require.Equal(t, id, siteItemID("castle", "lieutenants-cloak"), "the same when asked twice: every session and every body")
	require.NotEqual(t, id, siteItemID("keep", "lieutenants-cloak"), "another site's item of the same id")
	require.NotEqual(t, id, siteItemID("castle", "captains-cloak"), "another item")
	require.Regexp(t, regexp.MustCompile(`^lieutenants-cloak-[0-9a-f]{8}$`), id)
	require.Regexp(t, regexp.MustCompile(`^[A-Za-z0-9_-]+$`), siteItemID("run:1/a b", "x_y"),
		"only letters, digits, '-' and '_' whatever the site is called")
	empty := siteItemID("", "lieutenants-cloak")
	require.Regexp(t, regexp.MustCompile(`^lieutenants-cloak-[0-9a-f]{8}$`), empty, "an empty site key is its own site")
	require.NotEqual(t, id, empty)
	require.NotEqual(t, siteItemID("a\x00b", "c"), siteItemID("a", "b\x00c"),
		"the site key and the item id are joined so they cannot be confused")
}

// Test 6. What a looter leaves is read from their sheet alone.
func TestLeaveIsDerivedFromTheSheetAlone(t *testing.T) {
	siteItem := func(id, owned string) magicitems.Data {
		return magicitems.Data{ID: owned, Description: magicitems.Description{
			Ref: core.Ref{Module: "site", Type: "items", ID: id}, Name: id}}
	}
	cloak := siteItem("lieutenants-cloak", siteItemID("castle", "lieutenants-cloak"))
	sheet := &character.Data{ID: "alice", Items: []magicitems.Data{cloak}}

	require.Equal(t, []string{"lieutenants-cloak"}, leaveFor("castle", sheet), "the site's own cloak is left")
	require.Empty(t, leaveFor("keep", sheet), "another site's cloak is another item")
	require.Empty(t, leaveFor("", sheet))

	old := siteItem("lieutenants-cloak", "lieutenants-cloak-1268cede")
	require.Empty(t, leaveFor("castle", &character.Data{Items: []magicitems.Data{old}}),
		"an item under an old id made from a run and a body is another item")

	catalogue := magicitems.Data{ID: siteItemID("castle", "lieutenants-cloak"), Description: magicitems.Description{
		Ref: core.Ref{Module: "dnd5e", Type: "items", ID: "lieutenants-cloak"}, Name: "x"}}
	require.Empty(t, leaveFor("castle", &character.Data{Items: []magicitems.Data{catalogue}}),
		"a catalogue item is not a site item")

	require.Empty(t, leaveFor("castle", &character.Data{}), "no items, nothing to leave")
	require.Empty(t, leaveFor("castle", nil), "no sheet, nothing to leave")

	two := &character.Data{Items: []magicitems.Data{cloak, siteItem("ring", siteItemID("castle", "ring"))}}
	require.Equal(t, []string{"lieutenants-cloak", "ring"}, leaveFor("castle", two), "every owned site item, in sheet order")
}

// Test 6, one level down: the delivering function refuses a grant the rules
// refuse, names the item, and leaves the sheet as it was.
func TestDeliverTakenRefusesWhatTheRulesRefuse(t *testing.T) {
	sheet := &character.Data{ID: "alice"}
	bad := encounter.ItemGrant{ID: "lieutenants-cloak", Name: "Lieutenant's cloak",
		Item: json.RawMessage(`{"name":"Lieutenant's cloak","slot":"cape","lets":[{"use":"second wind","with":{"level":1}}]}`)}
	err := deliverTaken("castle", "guard", sheet, []encounter.TakenItem{{Member: "alice", Grant: bad}})
	require.ErrorIs(t, err, ErrUnknownContent)
	require.Contains(t, err.Error(), "lieutenants-cloak")
	require.Empty(t, sheet.Items)
}

// emptyCharacters holds no sheet and refuses every save.
type emptyCharacters struct{ saved int }

func (e *emptyCharacters) GetCharacter(context.Context, string) (*character.Data, error) {
	return nil, ErrNotFound
}

func (e *emptyCharacters) SaveCharacter(context.Context, *character.Data) error {
	e.saved++
	return nil
}

// A looter with no sheet of its own (not a player) takes nothing from the
// encounter, so Loot loads and saves no sheet for it and behaves as it did
// before items: the encounter alone is saved. A sheet that is missing while
// something WAS taken is an error.
func TestALooterWithNoSheetSavesNoSheet(t *testing.T) {
	repo := &emptyCharacters{}
	m := &Manager{characters: repo}
	in := &LootInput{Session: "sess", Member: "guard-2", Target: "guard"}

	sheet, err := m.loadLooter(context.Background(), in.Member)
	require.NoError(t, err, "a looter with no sheet is not an error to load")
	require.Nil(t, sheet)
	require.Empty(t, leaveFor("castle", sheet), "and leaves nothing")
	require.NoError(t, m.deliverLoot(context.Background(), nil, in, sheet, nil))
	require.Zero(t, repo.saved, "no sheet was written")

	err = m.deliverLoot(context.Background(), nil, in, sheet, []encounter.TakenItem{{Member: "guard-2",
		Grant: encounter.ItemGrant{ID: "cloak", Item: json.RawMessage(`{}`)}}})
	require.ErrorIs(t, err, ErrNoCharacter)
}
