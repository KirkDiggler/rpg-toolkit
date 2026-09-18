// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// table_test.go is THE CREATURE'S TABLE ON THE FORM (rpg-project#465): the
// `time` key, the conditions, the words a creature does with time, the
// selectors, and the temperament — each refused in the author's own words when
// it is written somewhere it does not belong.
//
// THE DEFAULT TABLE THE RULEBOOK SHIPS IS COMPILED HERE TOO, through
// [dungeonspec.CompileTable] and therefore through the SAME validator an
// authored dungeon goes through. Two grammars would drift, and the first thing
// to drift would be a refusal the author sees and the rulebook does not.

// thugDefaultTable is the rulebook's default table, VERBATIM as rulebooks/dnd5e
// ships it in monster/table (design §2) — character for character, so this
// module's validator is pinned against the real thing rather than a paraphrase
// of it. A paraphrase is the one fixture that can pass while the table it
// stands for is refused, which is the whole failure this test exists to catch.
//
// ONE GENERIC TABLE stands behind every monster kind this slice ships, so this
// is equally what the rulebook answers for a goblin or a skeleton. The name
// keeps the kind it was first written for.
const thugDefaultTable = `time:
  - { when: { fled: { within: 3 } },     away: actor,      weight: 3 }
  - { when: { attacked: { within: 3 } }, attack: attacker, weight: 3 }
  - { when: { enemy: reach },            attack: enemy }
  - { when: { enemy: seen },             toward: enemy }
  - { when: { enemy: remembered },       toward: enemy }
  - { when: { enemy: none },             hold: {} }
`

// TestTheRulebooksDefaultTableCompiles is layer one of three: a monster kind's
// own orders, shipped as content, read by the same decoder and judged by the
// same validator as anything an author writes.
func TestTheRulebooksDefaultTableCompiles(t *testing.T) {
	table, err := dungeonspec.CompileTable(thugDefaultTable)
	require.NoError(t, err)

	entries := table[encounter.AnswerTime]
	require.Len(t, entries, 6, "six rows, in the order the rulebook wrote them")

	require.Equal(t, 3, entries[0].Weight, "the run is weighted as heavily as the retaliation")
	require.Equal(t, "fled", entries[0].When.Deed,
		"the author's own past tense: a condition is written from the creature's side")
	require.Equal(t, encounter.DeedFled, encounter.DeedVerbFor(entries[0].When.Deed),
		"and it names the deed the store files under")
	require.Equal(t, 3, entries[0].When.Within, "three rounds of running, in the author's sight")
	require.Equal(t, encounter.SelectorActor, entries[0].Away.Word,
		"away from whoever made it run: `flee: {}` lands the deed and THIS row does the walking")

	require.Equal(t, 3, entries[1].Weight, "the retaliation is the other heavy one")
	require.Equal(t, "attacked", entries[1].When.Deed,
		"the author's own past tense, again")
	require.Equal(t, encounter.DeedAttack, encounter.DeedVerbFor(entries[1].When.Deed),
		"and it names the deed the store files under")
	require.Equal(t, 3, entries[1].When.Within, "the preset's patience, moved into the author's sight")
	require.Equal(t, encounter.SelectorAttacker, entries[1].Attack.Word)

	require.Equal(t, 1, entries[2].Weight, "an omitted weight compiles to 1 here, once")
	require.Equal(t, encounter.EnemyReach, entries[2].When.Enemy)
	require.Equal(t, encounter.SelectorEnemy, entries[2].Attack.Word,
		"the swing is under `reach`, which is the band that can land it")

	require.Equal(t, encounter.EnemySeen, entries[3].When.Enemy)
	require.Equal(t, encounter.SelectorEnemy, entries[3].Toward.Word,
		"and `seen` — in sight, out of reach — is where it CLOSES, which the default could not do before")

	require.Equal(t, encounter.EnemyRemembered, entries[4].When.Enemy)
	require.Equal(t, encounter.SelectorEnemy, entries[4].Toward.Word)

	require.True(t, entries[5].Hold, "and a way out, so a thug with nothing to do stands there")
	require.Equal(t, encounter.EnemyNone, entries[5].When.Enemy,
		"CONDITIONAL, not standing: an unconditional hold is eligible on every roll, so it competes with the swing and a thug in reach stands there half its turns")
}

// TestADefaultTableCannotNameACell: a kind's table belongs to every creature
// of that kind, on every map. There is no floor to check a cell against, so
// `toward: { at: … }` is an authored dungeon's word and refused here.
func TestADefaultTableCannotNameACell(t *testing.T) {
	_, err := dungeonspec.CompileTable(`
time:
  - { toward: { at: [3, 4] } }
`)
	require.Error(t, err)
	require.ErrorIs(t, err, dungeonspec.ErrBadSpec)
	require.Contains(t, err.Error(), "belongs to a kind and not to a map")
}

// TestADefaultTableIsJudgedByTheSameValidator: the point of exporting the
// compiler is that a rulebook's mistake reads exactly like an author's.
func TestADefaultTableIsJudgedByTheSameValidator(t *testing.T) {
	_, err := dungeonspec.CompileTable(`
time:
  - { fact: camp-cowed }
`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "answers a social verdict")
}

// --- the grammar on a real dungeon -------------------------------------------

// withPlacement rewrites the heirloom tomb's captain line, which is the one
// monster placement these scenes need to edit. One edit per scene, so a defect
// names exactly one mistake.
func withPlacement(t *testing.T, line string) string {
	t.Helper()
	source := heirloomSource(t)
	const captain = `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map] }`
	require.Contains(t, source, captain, "the fixture still has the line these scenes edit")

	return strings.Replace(source, captain, line, 1)
}

// TestATimeTableOnAPlacementCompiles is the whole new grammar in one
// placement: a condition on a deed, a condition on what is in sight, an
// authored cell to walk to, a run from the actor of a deed, and a way out.
func TestATimeTableOnAPlacementCompiles(t *testing.T) {
	source := withPlacement(t, `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map], temper: coward,
      on: { time: [
        { when: { attacked: { within: 3 } }, attack: attacker, weight: 3 },
        { when: { enemy: seen }, attack: enemy },
        { when: { enemy: remembered }, toward: enemy },
        { when: { enemy: none }, toward: { at: [23, 3] } },
        { when: { fled: { within: 2 } }, away: actor, weight: 5 },
        { hold: {} },
      ] } }`)

	require.Empty(t, defectsIn(t, source), "every word of the new grammar, on one placement")

	compiled, err := dungeonspec.Load([]byte(source))
	require.NoError(t, err)

	var captain dungeonspec.MonsterPlacement
	for _, m := range compiled.Monsters {
		if m.ID == "captain" {
			captain = m
		}
	}
	require.Len(t, captain.Table[encounter.AnswerTime], 6)
	require.Equal(t, "coward", captain.Temper.Word, "the author named this creature's nerve, so nothing is dealt")
	require.Empty(t, captain.Temper.Mix)
	require.NotNil(t, captain.Table[encounter.AnswerTime][3].Toward.At,
		"and the authored cell reached the host as a dungeon-absolute position")
}

// TestEveryMisplacedWordIsRefusedByName is the legality table, per key. An
// author who writes a word under the wrong trigger has said something real and
// gets told where it belongs, not that it does not exist.
func TestEveryMisplacedWordIsRefusedByName(t *testing.T) {
	for _, tc := range []struct {
		name string
		on   string
		path string
		says []string
	}{
		{
			name: "a fact under time",
			on:   `on: { time: [ { fact: camp-cowed } ] }`,
			path: "place[18].on.time[0].fact",
			says: []string{"answers a social verdict"},
		},
		{
			name: "a flee under time",
			on:   `on: { time: [ { flee: {} } ] }`,
			path: "place[18].on.time[0].flee",
			says: []string{"answers a social verdict"},
		},
		{
			name: "a hold under a social verdict",
			on:   `on: { intimidated: [ { hold: {} } ] }`,
			path: "place[18].on.intimidated[0].hold",
			says: []string{"what a creature does with time"},
		},
		{
			name: "an attack under a social verdict",
			on:   `on: { intimidated: [ { attack: enemy } ] }`,
			path: "place[18].on.intimidated[0].attack",
			says: []string{"what a creature does with time"},
		},
		{
			name: "a condition under a social verdict",
			on:   `on: { intimidated: [ { say: "hah", when: { enemy: seen } } ] }`,
			path: "place[18].on.intimidated[0].when",
			says: []string{"already the condition"},
		},
		{
			name: "a cell on a word that acts on a creature",
			on:   `on: { time: [ { attack: { at: [23, 3] } } ] }`,
			path: "place[18].on.time[0].attack",
			says: []string{"somewhere to walk toward"},
		},
		{
			name: "`actor` with no deed to have been the actor of",
			on:   `on: { time: [ { when: { enemy: seen }, away: actor } ] }`,
			path: "place[18].on.time[0].away",
			says: []string{"this entry names no deed"},
		},
		{
			name: "a cell that is not floor",
			on:   `on: { time: [ { toward: { at: [99, 99] } } ] }`,
			path: "place[18].on.time[0].toward.at",
			says: []string{"which is not floor"},
		},
		{
			name: "a temperament this build does not ship",
			on:   `temper: brave`,
			path: "place[18].temper",
			says: []string{"not a temperament this build ships", "soldier, coward, aggressive"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := withPlacement(t, `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map], `+tc.on+` }`)
			requireDefect(t, defectsIn(t, source), append([]string{tc.path}, tc.says...)...)
		})
	}
}

// TestEveryMalformedConditionIsRefusedAtItsOwnLine: a `when` is refused by the
// decoder, which knows the line the author wrote it on.
func TestEveryMalformedConditionIsRefusedAtItsOwnLine(t *testing.T) {
	for _, tc := range []struct {
		name string
		when string
		says string
	}{
		{name: "two conditions in one", when: `{ enemy: seen, attacked: { within: 3 } }`, says: "one condition, and this names 2"},
		{name: "an enemy word nobody reads", when: `{ enemy: nearby }`, says: "not a condition this build reads"},
		{name: "and the refusal lists the four bands", when: `{ enemy: nearby }`, says: "reach, seen, remembered, none"},
		{name: "a deed nobody holds", when: `{ insulted: { within: 3 } }`, says: "not a deed this build holds"},
		{name: "a span counted from zero", when: `{ attacked: { within: 0 } }`, says: "counted from 1"},
		{name: "a deed with no span", when: `{ attacked: {} }`, says: "names no span"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := withPlacement(t, `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map], on: { time: [ { when: `+tc.when+`, hold: {} } ] } }`)
			_, err := dungeonspec.Decode([]byte(source))
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.says)
			require.Contains(t, err.Error(), "line ", "and it says which line")
		})
	}
}

// TestPatrolIsRefusedByNameWithWhereItWent: `patrol` joins `alarm`, `lure` and
// `pretend` — words the design NAMES and this build does not land. Refusing it
// as an unknown field would tell an author it does not exist, which is false
// and sends them looking for a typo.
func TestPatrolIsRefusedByNameWithWhereItWent(t *testing.T) {
	source := withPlacement(t, `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map], on: { time: [ { patrol: {} } ] } }`)

	_, err := dungeonspec.Decode([]byte(source))
	require.Error(t, err)
	require.Contains(t, err.Error(), "`patrol` is designed and not built yet")
}

// TestASelectorOutsideTheSealedThreeIsRefused: the vocabulary grows one word
// per use case, and an author finds out on the form.
func TestASelectorOutsideTheSealedThreeIsRefused(t *testing.T) {
	source := withPlacement(t, `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map], on: { time: [ { attack: weakest } ] } }`)

	_, err := dungeonspec.Decode([]byte(source))
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a selector this build resolves")
	require.Contains(t, err.Error(), "enemy, attacker, actor")
}

// --- the layers ---------------------------------------------------------------

// TestAFactionsOrdersAreInheritedAndAPlacementsKeyReplacesThem is design §1,
// on the form: layer two under layer three, nearest key wholesale.
func TestAFactionsOrdersAreInheritedAndAPlacementsKeyReplacesThem(t *testing.T) {
	source := heirloomSource(t)
	source = strings.Replace(source, "\nplace:\n", `
factions:
  - id: bandits
    temper: { coward: 1, soldier: 2 }
    on:
      time:
        - { when: { enemy: none }, toward: { at: [3, 3] } }
        - { hold: {} }
      intimidated:
        - { weight: 1, say: "the faction's line" }

place:
`, 1)
	source = withPlacementIn(t, source, `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map], faction: bandits,
      on: { time: [ { hold: {} } ] } }`)
	// A second member of the same faction, writing nothing of its own.
	source = strings.Replace(source,
		`  - { ref: "dnd5e:monsters:skeleton", at: [11,3], targeting: lowest-health }`,
		`  - { id: follower, ref: "dnd5e:monsters:skeleton", at: [11,3], targeting: lowest-health, faction: bandits }`, 1)

	require.Empty(t, defectsIn(t, source))

	compiled, err := dungeonspec.Load([]byte(source))
	require.NoError(t, err)

	byID := map[string]dungeonspec.MonsterPlacement{}
	for _, m := range compiled.Monsters {
		byID[m.ID] = m
	}

	follower := byID["follower"]
	require.Len(t, follower.Table[encounter.AnswerTime], 2, "it wrote nothing, so the faction's orders are its orders")
	require.Len(t, follower.Table[encounter.AnswerIntimidated], 1)
	require.Equal(t, map[string]int{"coward": 1, "soldier": 2}, follower.Temper.Mix,
		"and the faction's mix comes with them, for the composition to deal at the door")

	captain := byID["captain"]
	require.Len(t, captain.Table[encounter.AnswerTime], 1,
		"its own `time` key replaced the faction's wholesale — the entries under it are gone, visibly")
	require.True(t, captain.Table[encounter.AnswerTime][0].Hold)
	require.Len(t, captain.Table[encounter.AnswerIntimidated], 1,
		"and the key it was silent about is still the faction's")
	require.Equal(t, "the faction's line", captain.Table[encounter.AnswerIntimidated][0].Say)
}

// TestAPlacementsOwnTemperWinsAndTheMixIsNotDealtForIt: an author who named
// this creature's nerve has already answered the question the mix exists to
// ask (design §3).
func TestAPlacementsOwnTemperWinsAndTheMixIsNotDealtForIt(t *testing.T) {
	source := heirloomSource(t)
	source = strings.Replace(source, "\nplace:\n", `
factions:
  - id: bandits
    temper: { coward: 1, soldier: 2 }

place:
`, 1)
	source = withPlacementIn(t, source, `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map], faction: bandits, temper: aggressive }`)

	require.Empty(t, defectsIn(t, source))

	compiled, err := dungeonspec.Load([]byte(source))
	require.NoError(t, err)
	for _, m := range compiled.Monsters {
		if m.ID != "captain" {
			continue
		}
		require.Equal(t, "aggressive", m.Temper.Word)
		require.Empty(t, m.Temper.Mix, "and nothing is left for a die to decide")
	}
}

// withPlacementIn is withPlacement against a source a scene has already
// edited, so two edits can compose.
func withPlacementIn(t *testing.T, source, line string) string {
	t.Helper()
	const captain = `  - { id: captain, ref: "dnd5e:monsters:skeleton-captain", at: [23,5], targeting: closest,
      holds: [vault-map] }`
	require.Contains(t, source, captain)

	return strings.Replace(source, captain, line, 1)
}
