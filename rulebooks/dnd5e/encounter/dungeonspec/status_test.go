// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// status_test.go is the authored half of A LINE THAT GIVES A STATUS
// (rpg-project#1024): the castle's `statuses:` and the cook's `help` line
// compiled, and every refusal the shape has, at the author's own path, on the
// small tomb vault.

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func TestDungeonspec_AStatusCompilesInTheAuthorsWords(t *testing.T) {
	c := loadSource(t, castleSource(t))
	var help *encounter.Option
	for _, o := range monsterNamed(t, c, "cook").Options {
		if o.ID == "help" {
			help = &o
		}
	}
	require.NotNil(t, help, "the cook has the help line")
	require.NotEmpty(t, help.Grants)
	for _, g := range help.Grants {
		require.False(t, g.Failure, "the failure row gives nothing")
		require.NotNil(t, g.Consequence.ApplyStatus)
		require.Equal(t, "well-fed", g.Consequence.ApplyStatus.ID)
		require.JSONEq(t, `{"name":"Well fed","changes":[{"advantage":"checks","when":{"ability":"charisma"}}],`+
			`"maintenance":[{"ends_on":"run"}],"again":"ignored"}`, string(g.Consequence.ApplyStatus.Status))
		var keys map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(g.Consequence.ApplyStatus.Status, &keys))
		require.Contains(t, string(g.Consequence.ApplyStatus.Status), `{"again":"ignored","changes":`,
			"canonical JSON: keys in order")
	}
}

// vaultWithAStatus is the tomb vault with one declared status and a line on
// its captain whose answer gives it: the small site the refusals are cut
// from, rewritten once with the fragment given.
func vaultWithAStatus(t *testing.T, old, replacement string) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/world-builder-v4-tomb-vault.yaml")
	require.NoError(t, err)
	source := strings.Replace(string(raw), "      captain: {faction: monsters,  holds: [vault-map] }\n",
		"      captain:\n        faction: monsters\n        holds: [vault-map]\n        options:\n          - id: ask\n            say: Where?\n"+
			"            success:\n              - say: Behind you.\n"+vaultApply, 1)
	require.Equal(t, 1, strings.Count(source, "\nroom:\n"))
	source = strings.Replace(source, "\nroom:\n", "\n"+vaultStatus+"room:\n", 1)
	require.Equal(t, 1, strings.Count(source, old), "the vault says %q once", old)
	return strings.Replace(source, old, replacement, 1)
}

const (
	vaultApply  = "                apply_status: rested\n"
	vaultStatus = "statuses:\n  rested:\n    name: Rested\n    changes:\n      - advantage: saves\n    maintenance:\n      - ends_on: rest\n    again: ignored\n"
)

func TestDungeonspec_AStatusIsRefusedWhereItIsWrong(t *testing.T) {
	const asked = "room.room.monsterBindings.captain.options[0].success[0]"
	_ = loadSource(t, vaultWithAStatus(t, vaultApply, vaultApply))
	for name, tc := range map[string]struct {
		old, replacement, path, message string
	}{
		"a status the site does not have": {vaultApply, "                apply_status: weary\n",
			asked + ".apply_status", `"weary" is not a status of this site: the statuses declared are rested`},
		"a status with no name": {"    name: Rested\n", "",
			"statuses.rested.name", "a status needs a name: the words a player sees"},
		"a status with an empty name": {"    name: Rested\n", "    name: \"\"\n",
			"statuses.rested.name", "a status needs a name: the words a player sees"},
		"a status with no changes": {"    changes:\n      - advantage: saves\n", "",
			"statuses.rested.changes", "a status needs changes as a non-empty list: what the status changes"},
		"a status with empty changes": {"    changes:\n      - advantage: saves\n", "    changes: []\n",
			"statuses.rested.changes", "a status needs changes as a non-empty list: what the status changes"},
		"a status whose changes are not a list": {"    changes:\n      - advantage: saves\n", "    changes: { advantage: saves }\n",
			"statuses.rested.changes", "a status needs changes as a non-empty list: what the status changes"},
		"a status with no maintenance": {"    maintenance:\n      - ends_on: rest\n", "",
			"statuses.rested.maintenance", "a status needs maintenance as a non-empty list: what ends the status"},
		"a status with empty maintenance": {"    maintenance:\n      - ends_on: rest\n", "    maintenance: []\n",
			"statuses.rested.maintenance", "a status needs maintenance as a non-empty list: what ends the status"},
		"a status with no again": {"    again: ignored\n", "",
			"statuses.rested.again", "a status says what a second giving does: write again"},
		"a status with an unknown word": {"    again: ignored\n", "    again: ignored\n    lasts: forever\n",
			"statuses.rested.lasts", `"lasts" is not a word of a status: a status is written with name, changes, maintenance, again`},
		"a status holding NaN": {"      - advantage: saves\n", "      - advantage: saves\n        weight: .nan\n",
			"statuses.rested", "the value NaN at changes[0].weight cannot be written: a status holds only text, whole numbers, yes/no, and lists and maps of those"},
		"a status holding infinity": {"      - ends_on: rest\n", "      - ends_on: rest\n        after: .inf\n",
			"statuses.rested", "the value +Inf at maintenance[0].after cannot be written: a status holds only text, whole numbers, yes/no, and lists and maps of those"},
		"a status beside another grant": {vaultApply, vaultApply + "                tells: vault-map\n",
			asked, "a row grants one thing: `give`, `tells`, `relocate` or `apply_status`, not two"},
	} {
		t.Run(name, func(t *testing.T) {
			requireExactDefect(t, refusals(t, vaultWithAStatus(t, tc.old, tc.replacement)), tc.path, tc.message)
		})
	}
}

// A STATUS IS NOT A TABLE ROW'S: a routine's row gives nothing.
func TestDungeonspec_AStatusIsNotATableRow(t *testing.T) {
	errs := refusals(t, castleWith(t, "        - say: Not one step further without my say.\n",
		"        - say: Not one step further without my say.\n          apply_status: well-fed\n"))
	require.NotEmpty(t, errs)
	require.Equal(t, "apply_status is only on a line's success or failure row: a table row grants nothing", errs[0].Message)
	require.True(t, strings.HasSuffix(errs[0].Path, ".apply_status"), "refused at the row's own key: %s", errs[0].Path)
	require.True(t, strings.HasPrefix(errs[0].Path, "behaviorDefinitions."), "on the routine's table row: %s", errs[0].Path)
}

// A SITE THAT DECLARES NO STATUS GIVES NONE: the refusal says there are none.
func TestDungeonspec_AStatusNeedsADeclaration(t *testing.T) {
	source := vaultWithAStatus(t, vaultStatus, "")
	requireExactDefect(t, refusals(t, source), "room.room.monsterBindings.captain.options[0].success[0].apply_status",
		`"rested" is not a status of this site: this site declares no statuses`)
}

// EVERY DECLARED STATUS A ROW NAMES IS ITS GRANT: in a site the grammar
// accepts, each row that names a status compiles to a grant carrying that
// status's block — never to a row with no grant.
func TestDungeonspec_ARowNamingAStatusAlwaysCarriesIt(t *testing.T) {
	c := loadSource(t, vaultWithAStatus(t, vaultApply, vaultApply))
	var grants []encounter.RowGrant
	for _, o := range monsterNamed(t, c, "captain").Options {
		grants = append(grants, o.Grants...)
	}
	require.NotEmpty(t, grants, "the row that names the status carries a grant")
	for _, g := range grants {
		require.NotNil(t, g.Consequence.ApplyStatus)
		require.Equal(t, "rested", g.Consequence.ApplyStatus.ID)
		require.JSONEq(t, `{"name":"Rested","changes":[{"advantage":"saves"}],"maintenance":[{"ends_on":"rest"}],"again":"ignored"}`,
			string(g.Consequence.ApplyStatus.Status))
	}
}
