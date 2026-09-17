// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// intimidate_test.go is the shenanigans' authoring surface (rpg-project#454,
// rpg-project#458): what the author writes on a monster placement to price a
// threat and an appeal, what the creature does and says about each outcome,
// and what the compiler hands the host.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// The front room goblin's own placement, in the design's words: two priced
// verbs and a table under every outcome, weights, lines, one `fact` and one
// `flee`.
const talkativeChief = `  - { id: chief,  ref: "dnd5e:monsters:skeleton-captain", at: [12,4], faction: raiders, ` +
	`intimidate: [{ ability: intimidation, dc: 12 }, { ability: str, dc: 15 }], ` +
	`persuade: [{ ability: persuasion, dc: 10 }], ` +
	`on: { ` +
	`intimidated: [ { weight: 70, say: "Fine, fine!", fact: sergeant-cowed }, { weight: 30, say: "BOSS!", flee: {} } ], ` +
	`intimidate_failed: [ { say: "Big talk." } ], ` +
	`persuaded: [ { fact: sergeant-cowed } ], ` +
	`persuade_failed: [ { say: "Nothing down there, friend." } ] } }`

// A fixture carrying every key, every weight form and both outcome words
// compiles onto the placement the host spawns from, in authored order.
func TestAnAuthoredTableReachesTheHost(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(edited(t, chiefLine, talkativeChief)))
	require.NoError(t, err)

	chief := compiled.Monsters[0]
	require.Equal(t, "chief", chief.ID, "the fixture's first monster is the one edited")
	require.Equal(t, []encounter.CheckApproach{
		{Ability: "intimidation", DC: 12},
		{Ability: "str", DC: 15},
	}, chief.Intimidate, "every route the author priced, in the order they wrote them")
	require.Equal(t, []encounter.CheckApproach{
		{Ability: "persuasion", DC: 10},
	}, chief.Persuade)

	require.Equal(t, map[string][]encounter.Reaction{
		dungeonspec.OnIntimidated: {
			{Weight: 70, Say: "Fine, fine!", Fact: "sergeant-cowed"},
			{Weight: 30, Say: "BOSS!", Flee: true},
		},
		dungeonspec.OnIntimidateFailed: {{Weight: 1, Say: "Big talk."}},
		dungeonspec.OnPersuaded:        {{Weight: 1, Fact: "sergeant-cowed"}},
		dungeonspec.OnPersuadeFailed:   {{Weight: 1, Say: "Nothing down there, friend."}},
	}, chief.Reactions)
}

// An omitted weight compiles to 1, resolved HERE so nothing downstream has to
// know what "omitted" meant. A table whose entries all omit it is an even
// split, and a single entry is the certainty it looks like.
func TestAnOmittedWeightCompilesToOne(t *testing.T) {
	const evenly = `  - { id: chief,  ref: "dnd5e:monsters:skeleton-captain", at: [12,4], faction: raiders, ` +
		`on: { intimidated: [ { say: "one" }, { say: "two" }, { say: "three" } ] } }`

	compiled, err := dungeonspec.Load([]byte(edited(t, chiefLine, evenly)))
	require.NoError(t, err)

	require.Equal(t, []encounter.Reaction{
		{Weight: 1, Say: "one"},
		{Weight: 1, Say: "two"},
		{Weight: 1, Say: "three"},
	}, compiled.Monsters[0].Reactions[dungeonspec.OnIntimidated])
}

// Absent is the common case and it compiles to nil, not to an empty map — the
// rulebook derives the DC from the stat block's own passive Insight, and nil
// is how it knows to.
func TestAMonsterNobodyPricedCarriesNothing(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(campSource(t)))
	require.NoError(t, err)

	for _, m := range compiled.Monsters {
		require.Nil(t, m.Intimidate, "%s prices no threat", m.ID)
		require.Nil(t, m.Persuade, "%s prices no appeal", m.ID)
		require.Nil(t, m.Reactions, "%s answers nothing", m.ID)
	}
}

// TestAFactNothingElseMentionsIsAllowed: the same R8 rule `arrives: { fact }`
// keeps — the dungeon shows the cost rather than refusing a fact no record
// reveals and no disposition waits for.
func TestAFactNothingElseMentionsIsAllowed(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(edited(t, chiefLine,
		strings.Replace(talkativeChief, "fact: sergeant-cowed }, { weight: 30",
			"fact: nobody-waits-for-this }, { weight: 30", 1))))
	require.NoError(t, err)
	require.Equal(t, encounter.FactID("nobody-waits-for-this"),
		compiled.Monsters[0].Reactions[dungeonspec.OnIntimidated][0].Fact)
}

// TestTheNewKeysAreKnownToTheDecoder: a yaml tag alone is not enough —
// PlaceSpec's hand-written key list is what decides, and a typo'd sibling
// must still be refused by name.
func TestTheNewKeysAreKnownToTheDecoder(t *testing.T) {
	t.Run("all three keys decode", func(t *testing.T) {
		_, err := dungeonspec.Load([]byte(edited(t, chiefLine, talkativeChief)))
		require.NoError(t, err)
	})
	t.Run("a typo is still refused by name", func(t *testing.T) {
		_, err := dungeonspec.Load([]byte(edited(t, chiefLine,
			strings.Replace(talkativeChief, "persuade:", "persuasion:", 1))))
		require.Error(t, err)
		require.Contains(t, err.Error(), "field persuasion not found")
	})
}

// onTable is the chief with one `on:` body, so a scene can name exactly the
// table it is about.
func onTable(t *testing.T, body string) string {
	t.Helper()

	return edited(t, chiefLine,
		`  - { id: chief,  ref: "dnd5e:monsters:skeleton-captain", at: [12,4], faction: raiders, `+
			`on: `+body+` }`)
}

// An outcome key nobody designed is refused with the four there are, so the
// author can see what they meant to write.
func TestAnUnknownOutcomeKeyIsRefused(t *testing.T) {
	requireDefect(t, defectsIn(t, onTable(t, `{ bribed: [ { say: "ok" } ] }`)),
		"place[0].on.bribed", `"bribed" is not an outcome this build lands`,
		"intimidated, intimidate_failed, persuaded, persuade_failed")
}

// A word the design NAMES and this build does not land is refused BY NAME,
// with where it went — never as a bare unknown field, which would tell the
// author the word does not exist.
func TestADesignedButUnbuiltWordIsRefusedByName(t *testing.T) {
	for word, want := range map[string]string{
		"alarm":   "designed as the Alarm slice and not built yet",
		"lure":    "designed and not built yet",
		"pretend": "designed as the Insight slice and not built yet",
		"tell":    "not a word: a fact taught to whoever was there is `fact`",
	} {
		t.Run(word, func(t *testing.T) {
			_, err := dungeonspec.Load([]byte(onTable(t,
				`{ intimidated: [ { `+word+`: { toward: bandit-hall } } ] }`)))
			require.Error(t, err)
			require.Contains(t, err.Error(), "`"+word+"` is "+want)
			require.Contains(t, err.Error(), "rpg-project#458")
		})
	}
}

// A key that is neither an outcome word nor a designed one is still a plain
// unknown field, so a typo reads as a typo.
func TestATypoInAnEntryIsAnUnknownField(t *testing.T) {
	_, err := dungeonspec.Load([]byte(onTable(t, `{ intimidated: [ { sez: "hi" } ] }`)))
	require.Error(t, err)
	require.Contains(t, err.Error(), "field sez not found in type dungeonspec.ReactionSpec")
}

// Two words in one entry is refused, so an author never has to guess which
// happens first.
func TestTwoWordsInOneEntryAreRefused(t *testing.T) {
	requireDefect(t, defectsIn(t, onTable(t,
		`{ intimidated: [ { fact: cowed, flee: {} } ] }`)),
		"place[0].on.intimidated[0]", "an entry does one thing")
}

// A weight below 1 is a row that can never fire, and the message says what to
// write instead.
func TestAWeightBelowOneIsRefused(t *testing.T) {
	requireDefect(t, defectsIn(t, onTable(t,
		`{ intimidated: [ { weight: 0, say: "never" } ] }`)),
		"place[0].on.intimidated[0].weight", "a weight of 0 can never be rolled")
}

// An entry that does nothing and says nothing is a row written for no reason.
func TestAnEmptyEntryIsRefused(t *testing.T) {
	requireDefect(t, defectsIn(t, onTable(t, `{ intimidated: [ { weight: 3 } ] }`)),
		"place[0].on.intimidated[0]", "this entry does nothing and says nothing")
}

// `fact:` with nothing after it says the world learns something and does not
// say what — its own sentence, not the empty-entry one.
func TestAnEmptyFactIsRefusedAsAMissingValue(t *testing.T) {
	requireDefect(t, defectsIn(t, onTable(t, `{ intimidated: [ { fact: "" } ] }`)),
		"place[0].on.intimidated[0].fact", "does not say what")
}

// An outcome key with no entries is a table that cannot be rolled.
func TestAnOutcomeWithNoEntriesIsRefused(t *testing.T) {
	requireDefect(t, defectsIn(t, onTable(t, `{ intimidated: [] }`)),
		"place[0].on.intimidated", "lists nothing that happens on it")
}

// Neither social key belongs on a prop: nothing about a barrel can be
// threatened or talked round.
func TestSocialKeysAreRefusedOnAProp(t *testing.T) {
	const barrel = `  - { id: barrel, ref: "dnd5e:props:barrel", at: [11,4], ` +
		`blocks_movement: true, blocks_los: false, persuade: [{ ability: persuasion, dc: 5 }] }`

	requireDefect(t, defectsIn(t, edited(t, chiefLine, chiefLine+"\n"+barrel)),
		"place[1].persuade", "is not a monster and cannot be persuaded")
}

// A priced verb still needs a way through it, on both verbs.
func TestAPricedVerbWithNoWayThroughIsRefused(t *testing.T) {
	requireDefect(t, defectsIn(t, edited(t, chiefLine,
		`  - { id: chief,  ref: "dnd5e:monsters:skeleton-captain", at: [12,4], faction: raiders, `+
			`persuade: [] }`)),
		"place[0].persuade", "declares a persuade check with no way through it")
}
