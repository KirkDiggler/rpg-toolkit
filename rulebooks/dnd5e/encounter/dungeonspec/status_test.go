// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// status_test.go is the authored half of A LINE THAT GIVES A STATUS
// (rpg-project#1024): the castle's `statuses:` and the cook's `help` line
// compiled, and every refusal the shape has, at the author's own path, on the
// small tomb vault.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

func TestDungeonspec_AStatusCompilesInTheAuthorsWords(t *testing.T) {
	c := loadSource(t, castleSource(t))
	lines := map[string]*encounter.Option{}
	for _, o := range monsterNamed(t, c, "cook").Options {
		lines[o.ID] = &o
	}
	for _, tc := range []struct{ line, status, block string }{
		// The Keep design's two kitchen rewards (rpg-project#1024).
		{"help", "warmed-up", `{"name":"Warmed up","changes":[{"initiative":1}],"maintenance":[{"ends_on":"run"}],"again":"ignored"}`},
		{"taste", "feasted", `{"name":"Feasted","changes":[{"max_hit_points":2}],"when_given":[{"heal":2}],` +
			`"maintenance":[{"ends_on":"run"}],"again":"ignored"}`},
	} {
		t.Run(tc.line, func(t *testing.T) {
			line := lines[tc.line]
			require.NotNil(t, line, "the cook has the %s line", tc.line)
			require.NotEmpty(t, line.Grants)
			for _, g := range line.Grants {
				require.False(t, g.Failure, "the failure row gives nothing")
				require.NotNil(t, g.Consequence.ApplyStatus)
				require.Equal(t, tc.status, g.Consequence.ApplyStatus.ID)
				require.JSONEq(t, tc.block, string(g.Consequence.ApplyStatus.Status), "carried in the author's words, when_given and all")
				require.Contains(t, string(g.Consequence.ApplyStatus.Status), `{"again":"ignored","changes":`,
					"canonical JSON: keys in order")
			}
		})
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

// receiverWords is how a refusal lists who can receive a status: the table's
// words at `apply_status.to`, each with its label.
const receiverWords = "`speaker`, the speaker, seen or not, or `party_in_sight`, the speaker's party it sees, or " +
	"`spoken_to`, the one the line is said to"

const (
	vaultApply  = "                apply_status: { status: rested, to: speaker }\n"
	vaultStatus = "statuses:\n  rested:\n    name: Rested\n    changes:\n      - advantage: saves\n    maintenance:\n      - ends_on: rest\n    again: ignored\n"
)

func TestDungeonspec_AStatusIsRefusedWhereItIsWrong(t *testing.T) {
	const asked = "room.room.monsterBindings.captain.options[0].success[0]"
	_ = loadSource(t, vaultWithAStatus(t, vaultApply, vaultApply))
	for name, tc := range map[string]struct {
		old, replacement, path, message string
	}{
		"a status the site does not have": {vaultApply, "                apply_status: { status: weary, to: speaker }\n",
			asked + ".apply_status.status", `"weary" is not a status of this site: the statuses declared are rested`},
		"the short form": {vaultApply, "                apply_status: rested\n",
			asked + ".apply_status", "apply_status names who receives it: write apply_status: { status: rested, to: speaker }"},
		"no status": {vaultApply, "                apply_status: { to: speaker }\n",
			asked + ".apply_status.status", "apply_status names the status given: write `status: <id>` (the statuses declared are rested)"},
		"no receiver": {vaultApply, "                apply_status: { status: rested }\n",
			asked + ".apply_status.to", "apply_status names who receives it: write `to:` as " + receiverWords},
		"an unknown receiver": {vaultApply, "                apply_status: { status: rested, to: the_taken }\n",
			asked + ".apply_status.to", `"the_taken" is not who can receive a status: write ` + "`to:` as " + receiverWords},
		"another key": {vaultApply, "                apply_status: { status: rested, to: speaker, for: ever }\n",
			asked + ".apply_status.for", "apply_status takes `status` and `to`, not `for`"},
		"a list of receivers": {vaultApply, "                apply_status: { status: rested, to: [speaker, spoken_to] }\n",
			asked + ".apply_status.to", "apply_status's `to` is one word, not a list or a mapping"},
		"a null receiver": {vaultApply, "                apply_status: { status: rested, to: ~ }\n",
			asked + ".apply_status.to", "apply_status names who receives it: write `to:` as " + receiverWords},
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
		"a status whose when_given is not a list": {"    again: ignored\n", "    again: ignored\n    when_given: { heal: 2 }\n",
			"statuses.rested.when_given", "when_given, when written, is a non-empty list: what the status does once, when it is first given"},
		"a status with empty when_given": {"    again: ignored\n", "    again: ignored\n    when_given: []\n",
			"statuses.rested.when_given", "when_given, when written, is a non-empty list: what the status does once, when it is first given"},
		"a status with an unknown word": {"    again: ignored\n", "    again: ignored\n    lasts: forever\n",
			"statuses.rested.lasts", `"lasts" is not a word of a status: a status is written with name, changes, maintenance, again, when_given`},
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

// A STATUS IS NOT A TABLE ROW'S: a routine's row gives nothing, even a status
// the site DECLARES — so the one refusal is "not on a table row", never "no
// such status".
func TestDungeonspec_AStatusIsNotATableRow(t *testing.T) {
	errs := refusals(t, castleWith(t, "        - say: Not one step further without my say.\n",
		"        - say: Not one step further without my say.\n          apply_status: { status: warmed-up, to: speaker }\n"))
	require.NotEmpty(t, errs)
	for _, e := range errs {
		require.NotContains(t, e.Message, "is not a status of this site", "warmed-up is declared: %s", e.Message)
	}
	require.Equal(t, "apply_status is only on a line's success or failure row: a table row grants nothing", errs[0].Message)
	require.True(t, strings.HasSuffix(errs[0].Path, ".apply_status"), "refused at the row's own key: %s", errs[0].Path)
	require.True(t, strings.HasPrefix(errs[0].Path, "behaviorDefinitions."), "on the routine's table row: %s", errs[0].Path)
	for _, e := range errs {
		require.Equal(t, errs[0], e, "the table row is the one thing refused")
	}
}

// A SITE THAT DECLARES NO STATUS GIVES NONE: the refusal says there are none.
func TestDungeonspec_AStatusNeedsADeclaration(t *testing.T) {
	source := vaultWithAStatus(t, vaultStatus, "")
	requireExactDefect(t, refusals(t, source), "room.room.monsterBindings.captain.options[0].success[0].apply_status.status",
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

// ON A THING'S LINE ONLY THE SPEAKER RECEIVES (rpg-project#990): a thing has
// nobody spoken to, so `party_in_sight` and `spoken_to` are refused at the
// author's path there, and `speaker` is the speaker.
func TestDungeonspec_OnAThingsLineOnlyTheSpeakerReceives(t *testing.T) {
	const relocate = "              - relocate:\n                  to: region-chapel\n" +
		"                  words: The tunnel climbs, and you come up through a flagstone behind the chapel's altar.\n" +
		"                  rounds: 1\n"
	give := func(to string) string {
		return "              - apply_status: { status: warmed-up, to: " + to + " }\n"
	}
	_ = loadSource(t, castleWith(t, relocate, give("speaker")))
	const at = "room.room.propBindings.grate.options[0].success[0].apply_status.to"
	for _, to := range []string{"party_in_sight", "spoken_to"} {
		t.Run(to, func(t *testing.T) {
			requireExactDefect(t, refusals(t, castleWith(t, relocate, give(to))), at,
				"on a thing's line nobody is spoken to, so nobody receives it but the speaker: only `to: speaker` can be used here")
		})
	}
}

// THE AUTHOR IS TOLD WHAT TO WRITE, NEVER THE PLAYER'S SENTENCE
// (rpg-project#990): the short form is refused by the compiler at the
// author's path; the engine's refusal of a grant with no receiver — written
// for a player whose saved game is too old — never reaches an author.
func TestDungeonspec_TheShortFormTellsTheAuthorWhatToWrite(t *testing.T) {
	errs := refusals(t, vaultWithAStatus(t, vaultApply, "                apply_status: rested\n"))
	requireExactDefect(t, errs, "room.room.monsterBindings.captain.options[0].success[0].apply_status",
		"apply_status names who receives it: write apply_status: { status: rested, to: speaker }")
	for _, e := range errs {
		require.NotContains(t, e.Message, "saved by an older version", "an author is never told the player's sentence")
	}
}

// A KEY WRITTEN TWICE IN apply_status IS REFUSED, at its line in yaml.v3's
// own sentence, as every hand-read mapping in this package refuses one
// ([repeatedKey]): the second `to` never quietly wins.
func TestDungeonspec_ApplyStatusRefusesAKeyWrittenTwice(t *testing.T) {
	_, err := dungeonspec.Load([]byte(vaultWithAStatus(t, vaultApply,
		"                apply_status: { status: rested, to: speaker, to: party_in_sight }\n")))
	require.Error(t, err)
	require.Regexp(t, `line \d+: mapping key "to" already defined at line \d+`, err.Error())
}
