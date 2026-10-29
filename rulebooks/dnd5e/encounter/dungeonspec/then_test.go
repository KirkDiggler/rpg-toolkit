// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// then_test.go is a disposition's `then:` and the `ally` selector in the
// dialect (rpg-project#717): the camp's one disposition, edited one line at a
// time, and the castle's table words with `ally` among them.

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// TestThenTurnsTheCampToThePartysSide: `then: allied` compiles onto the
// disposition, and a stance the pair could never reach before is reachable
// now, so an arrival waiting on it is accepted.
func TestThenTurnsTheCampToThePartysSide(t *testing.T) {
	allied := strings.Replace(dispoLine, "} }", "}, then: allied }", 1)
	compiled, err := dungeonspec.Load([]byte(edited(t, dispoLine, allied)))
	require.NoError(t, err)
	require.Equal(t, []encounter.DispositionInput{{
		Between: [2]encounter.FactionID{"raiders", "party"},
		Stance:  encounter.StanceHostile,
		Until:   encounter.TriggerFact{Fact: "saved-wiseman"},
		Then:    encounter.StanceAllied,
	}}, compiled.Field.Dispositions)

	source := edited(t, dispoLine, allied)
	source = strings.Replace(source, "arrives: { down: chief } }",
		"arrives: { stance: { between: [raiders, party], is: allied } } }", 1)
	require.Empty(t, refusals(t, source), "the camp can become the party's side, so an arrival may wait on it")
}

// TestAThenThatCannotBeOneIsRefusedAtItsLine: each refusal at the
// disposition's own path, in words a form-filler can act on.
func TestAThenThatCannotBeOneIsRefusedAtItsLine(t *testing.T) {
	for _, tc := range []struct {
		name, replacement, path string
		want                    []string
	}{
		{
			name:        "then with no until",
			replacement: `  - { between: [raiders, party], stance: hostile, then: neutral }`,
			path:        "dispositions[0].then", want: []string{"has no until"},
		},
		{
			name:        "then that is not a stance",
			replacement: strings.Replace(dispoLine, "} }", "}, then: friendly }", 1),
			path:        "dispositions[0].then", want: []string{`"friendly" is not a stance`},
		},
		{
			name:        "then the stance it already holds",
			replacement: strings.Replace(dispoLine, "} }", "}, then: hostile }", 1),
			path:        "dispositions[0].then", want: []string{"which it already is"},
		},
		{
			name:        "an until on an allied pair with nowhere to go",
			replacement: strings.Replace(dispoLine, "stance: hostile", "stance: allied", 1),
			path:        "dispositions[0].until", want: []string{alliedUntil, "`then: hostile`"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireRefusedAt(t, refusals(t, edited(t, dispoLine, tc.replacement)), tc.path, tc.want...)
		})
	}
}

// TestAnAlliedPairThatNamesWhereItGoesCompiles: allied until a fact, then
// hostile, is a legal line now.
func TestAnAlliedPairThatNamesWhereItGoesCompiles(t *testing.T) {
	line := strings.Replace(strings.Replace(dispoLine, "stance: hostile", "stance: allied", 1),
		"} }", "}, then: hostile }", 1)
	compiled, err := dungeonspec.Load([]byte(edited(t, dispoLine, line)))
	require.NoError(t, err)
	require.Equal(t, encounter.StanceAllied, compiled.Field.Dispositions[0].Stance)
	require.Equal(t, encounter.StanceHostile, compiled.Field.Dispositions[0].Then)
}

// TestTheAllyWordIsReadOnTheWalkingWordsAndRefusedOnAttack: the castle's
// own rows with `ally` in them, through the v4 dialect: `toward`, `dash` and
// `away` compile it, `attack` is refused at the row with mind/behavior's
// sentence.
func TestTheAllyWordIsReadOnTheWalkingWordsAndRefusedOnAttack(t *testing.T) {
	for _, word := range []string{"toward", "dash", "away"} {
		t.Run(word, func(t *testing.T) {
			rows := stormTheKeepRows(t, castleWithCaptainRow(t, "      - when: { enemy: none }\n        "+word+": ally\n"))
			last := rows[len(rows)-1]
			sel := map[string]*encounter.Selector{"toward": last.Toward, "dash": last.Dash, "away": last.Away}[word]
			require.NotNil(t, sel, "the row compiled its word")
			require.Equal(t, encounter.SelectorAlly, sel.Word)
		})
	}

	_, err := dungeonspec.Load([]byte(castleWithCaptainRow(t, "      - when: { enemy: none }\n        attack: ally\n")))
	require.Error(t, err)
	require.Contains(t, err.Error(), "`attack` cannot take `ally`: a mind is never handed an ally to strike")
}

// castleWithCaptainRow is the castle with one row appended to the captain's
// table, storm-the-keep.
func castleWithCaptainRow(t *testing.T, row string) string {
	t.Helper()
	raw := castleSource(t)
	anchor := "  answer-the-gate:\n"
	require.Equal(t, 1, strings.Count(raw, anchor), "the table after storm-the-keep is there once")

	return strings.Replace(raw, anchor, row+anchor, 1)
}

// castleSource is the castle fixture's bytes.
func castleSource(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/world-builder-v4-castle.yaml")
	require.NoError(t, err)

	return strings.ReplaceAll(string(raw), "\r\n", "\n")
}

// stormTheKeepRows is the castle captain's compiled `time` rows.
func stormTheKeepRows(t *testing.T, source string) []encounter.Answer {
	t.Helper()
	compiled, err := dungeonspec.Load([]byte(source))
	require.NoError(t, err)
	for _, m := range compiled.Monsters {
		if m.MemberID == "captain" {
			return m.Table[encounter.AnswerTime]
		}
	}
	require.FailNow(t, "the castle has its captain")

	return nil
}
