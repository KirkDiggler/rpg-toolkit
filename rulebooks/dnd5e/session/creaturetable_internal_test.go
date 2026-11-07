// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/table"
	"github.com/stretchr/testify/require"
	"sort"
	"testing"
)

func TestLaunchDoesNotInventAnUnauthoredPolicy(t *testing.T) {
	resolved, err := resolveLaunchMonsters(&dungeonspec.Compiled{Monsters: []dungeonspec.MonsterPlacement{{MemberID: "guard", Ref: "dnd5e:monsters:goblin"}}})
	require.NoError(t, err)
	require.Len(t, resolved, 1)
	require.Nil(t, resolved[0].placement.Behaviors, "sheet defaults do not become an implicit behavior")
}

func TestLaunchRefusesOldTablesRatherThanNormalizingThem(t *testing.T) {
	_, err := resolveLaunchMonsters(&dungeonspec.Compiled{Monsters: []dungeonspec.MonsterPlacement{{MemberID: "guard", Ref: "dnd5e:monsters:goblin", Table: encounter.Table{encounter.AnswerTime: {{Weight: 1, Hold: true}}}}}})
	require.ErrorIs(t, err, ErrInvalidWorld)
	require.ErrorContains(t, err, "canonical behaviors")
}

func TestATemperamentsWordIsResolvedToItsProfile(t *testing.T) {
	resolved, err := resolvedTemper(encounter.Temper{Word: string(table.TemperCoward)})
	require.NoError(t, err)
	require.Equal(t, "coward", resolved.Word)
	require.Nil(t, resolved.Mix)
	require.Equal(t, encounter.TemperProfile{Attack: 50, Toward: 50, Away: 300, Flee: 300, Hold: 100}, resolved.Profile)
}

func TestNoTemperamentIsNothingAtAll(t *testing.T) {
	resolved, err := resolvedTemper(encounter.Temper{})
	require.NoError(t, err)
	require.Equal(t, encounter.Temper{}, resolved)
}

func TestAMixCarriesEveryWordsProfileForTheCompositionToDealFrom(t *testing.T) {
	resolved, err := resolvedTemper(encounter.Temper{Mix: map[string]int{"coward": 1, "soldier": 2, "aggressive": 1}})
	require.NoError(t, err)
	require.Empty(t, resolved.Word, "nothing is dealt in the seam")
	require.Equal(t, map[string]int{"coward": 1, "soldier": 2, "aggressive": 1}, resolved.Mix)
	require.Len(t, resolved.Profiles, 3)
	require.Equal(t, 300, resolved.Profiles["aggressive"].Attack)
	require.Equal(t, 300, resolved.Profiles["coward"].Away)
	require.Equal(t, 100, resolved.Profiles["soldier"].Hold)
}

func TestAnUnknownTemperamentRefuses(t *testing.T) {
	for _, in := range []encounter.Temper{{Word: "solider"}, {Word: "brave"}, {Mix: map[string]int{"soldier": 2, "cowardly": 1}}} {
		out, err := resolvedTemper(in)
		require.Error(t, err)
		require.Equal(t, encounter.Temper{}, out)
	}
}

func TestTheTemperVocabulariesAgree(t *testing.T) {
	composition := append([]string(nil), encounter.TemperWords...)
	sort.Strings(composition)
	rulebook := []string{string(table.TemperSoldier), string(table.TemperCoward), string(table.TemperAggressive)}
	sort.Strings(rulebook)
	require.Equal(t, rulebook, composition)
	for _, word := range composition {
		p, err := profileOf(word)
		require.NoError(t, err)
		require.NotEqual(t, encounter.TemperProfile{}, p)
		require.True(t, encounter.ValidTemperWord(word))
	}
	require.False(t, encounter.ValidTemperWord("brave"))
	_, err := profileOf("brave")
	require.Error(t, err)
}
