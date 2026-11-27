// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// hit_points_row_test.go: a row's `hit_points` condition through the
// encounter's verbs (rpg-project#1131, slice 4b). The host is the shared turn
// host (internal/turntest), which tells the two numbers.

func hurtWhen(n int) encounter.When {
	return encounter.When{HitPoints: &behavior.HitPointsWhen{AtMost: n, Percent: true}}
}

// woundedLieutenantTable is the issue's table: when hurt, second wind; else
// walk in, swing in reach.
func woundedLieutenantTable() encounter.Table {
	enemy := &encounter.Selector{Word: encounter.SelectorEnemy}

	return encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{hurtWhen(30)}, Activate: secondWind},
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyWalk}}, Toward: enemy},
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyReach}}, Attack: enemy},
	}}
}

// Test 9: it fires when he is hurt, and not before.
func TestAHurtLieutenantActivatesAtOrBelowThatShareOfHisMaximum(t *testing.T) {
	for _, tc := range []struct {
		name       string
		told       bool
		now, max   int
		activation int
	}{
		{"unhurt, 9 of 9", true, 9, 9, 0},
		{"3 of 9 is a third, over thirty percent", true, 3, 9, 0},
		{"2 of 9 is under thirty percent", true, 2, 9, 1},
		{"the host says nothing", false, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := abilityScene{give: true, activator: true, table: woundedLieutenantTable()}.build(t)
			if tc.told {
				sc.host.Hurt(lieutenant, tc.now, tc.max)
			}
			aroundTheTable(t, sc.enc)

			require.Len(t, sc.host.Activations, tc.activation)
			require.Len(t, beatsOf(t, sc.enc, encounter.BeatActivated), tc.activation)
			require.Len(t, sc.striker.at, 1, "and he swings")
		})
	}
}

// Test 10: on a round of the world it is read too.
func TestAHitPointsRowIsReadOnARoundOfTheWorld(t *testing.T) {
	table := encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{hurtWhen(50)}, Say: "I yield"},
		{Weight: 1, Hold: true},
	}}
	for _, tc := range []struct {
		name     string
		now, max int
		says     bool
	}{
		{"4 of 9 says it", 4, 9, true},
		{"9 of 9 does not", 9, 9, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := abilityScene{table: table, world: true}.build(t)
			sc.host.Hurt(lieutenant, tc.now, tc.max)
			require.NoError(t, encounter.ExportedSpendWorldAction(sc.enc, alice))

			all, err := sc.enc.ReadRecord(&encounter.ReadRecordInput{})
			require.NoError(t, err)
			var said bool
			for _, entry := range all.Entries {
				var b map[string]any
				require.NoError(t, json.Unmarshal(entry.Payload, &b))
				if b["beat"] == "speech" && b["words"] == "I yield" {
					said = true
				}
			}
			require.Equal(t, tc.says, said)
			require.NotEmpty(t, picksOf(t, sc.enc), "he had his round")
		})
	}
}

// Test 11: a sheet's hit points are judged at the door.
func TestASheetsHitPointsAreJudgedAtTheDoor(t *testing.T) {
	for _, tc := range []struct {
		name     string
		now, max int
	}{
		{"negative hit points", -1, 9},
		{"a negative maximum", 0, -9},
		{"hit points with no maximum", 4, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := abilityScene{table: woundedLieutenantTable()}.build(t)
			sc.host.Hurt(lieutenant, tc.now, tc.max)
			on, err := sc.enc.ClockOf(&encounter.ClockOfInput{Member: alice})
			require.NoError(t, err)
			_, err = sc.enc.EndTurn(&encounter.EndTurnInput{Member: on.Active})
			require.ErrorIs(t, err, encounter.ErrInvalidData)
			require.Contains(t, err.Error(), string(lieutenant))
			t.Log(err)
		})
	}

	t.Run("above the maximum is accepted and holds nothing", func(t *testing.T) {
		sc := abilityScene{give: true, activator: true, table: woundedLieutenantTable()}.build(t)
		sc.host.Hurt(lieutenant, 12, 9)
		aroundTheTable(t, sc.enc)
		require.Empty(t, sc.host.Activations)
	})
}

// Test 5, the encounter half: saved and loaded with the encounter, alone and
// inside `not`.
func TestAHitPointsRowIsSavedAndLoaded(t *testing.T) {
	inner := hurtWhen(30)
	table := silenceTable()
	table[encounter.AnswerTime] = append([]encounter.Answer{
		{Weight: 1, When: []encounter.When{hurtWhen(30)}, Hold: true},
		{Weight: 1, When: []encounter.When{{Not: &inner}}, Hold: true},
	}, table[encounter.AnswerTime]...)
	enc, members := symbolsSceneWith(t, table)
	goblinTurn(t, enc)

	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)
	require.Equal(t, 2, strings.Count(string(raw), `"HitPoints":{"AtMost":30,"Percent":true}`), "alone and inside not, and no other row")

	var saved encounter.EncounterData
	require.NoError(t, json.Unmarshal(raw, &saved))
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: saved, Capabilities: symbolsCaps(members)})
	require.NoError(t, err)
	for _, m := range loaded.ToData().Members {
		if m.ID != goblin {
			continue
		}
		rows := m.Behaviors.Instances[0].Instance.Table[encounter.AnswerTime]
		require.Equal(t, 30, rows[0].When[0].HitPoints.AtMost)
		require.Equal(t, 30, rows[1].When[0].Not.HitPoints.AtMost)
		require.Nil(t, rows[2].When[0].HitPoints)
	}
}

// hurtNumber and hurtShare are the two forms of the condition (rpg-project#1131,
// slice 4b-2): at most N hit points, and at most N percent of the maximum.
func hurtNumber(n int) encounter.When {
	return encounter.When{HitPoints: &behavior.HitPointsWhen{AtMost: n}}
}

func hurtShare(n int) encounter.When {
	return encounter.When{HitPoints: &behavior.HitPointsWhen{AtMost: n, Percent: true}}
}

// lieutenantTableWith is woundedLieutenantTable with the one condition chosen.
func lieutenantTableWith(hurt encounter.When) encounter.Table {
	enemy := &encounter.Selector{Word: encounter.SelectorEnemy}

	return encounter.Table{encounter.AnswerTime: {
		{Weight: 1, When: []encounter.When{hurt}, Activate: secondWind},
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyWalk}}, Toward: enemy},
		{Weight: 1, When: []encounter.When{{Enemy: encounter.EnemyReach}}, Attack: enemy},
	}}
}

// Test 4b-2.6 and 4b-2.7: a number fires on hit points and a share on a share,
// and a host that tells nothing fires neither.
func TestARowFiresOnHitPointsOrOnAShareAsWritten(t *testing.T) {
	for _, tc := range []struct {
		name       string
		hurt       encounter.When
		told       bool
		now, max   int
		activation int
	}{
		{"8 hit points, 9 of 27 is over", hurtNumber(8), true, 9, 27, 0},
		{"8 hit points, 8 of 27 is at", hurtNumber(8), true, 8, 27, 1},
		{"30%, 9 of 27 is over (900 against 810)", hurtShare(30), true, 9, 27, 0},
		{"30%, 8 of 27 is under", hurtShare(30), true, 8, 27, 1},
		{"8 hit points does not scale: 8 of 10 fires", hurtNumber(8), true, 8, 10, 1},
		{"30% scales: 8 of 10 does not fire", hurtShare(30), true, 8, 10, 0},
		{"a number, the host says nothing", hurtNumber(8), false, 0, 0, 0},
		{"a share, the host says nothing", hurtShare(30), false, 0, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := abilityScene{give: true, activator: true, table: lieutenantTableWith(tc.hurt)}.build(t)
			if tc.told {
				sc.host.Hurt(lieutenant, tc.now, tc.max)
			}
			aroundTheTable(t, sc.enc)

			require.Len(t, sc.host.Activations, tc.activation)
			require.Len(t, beatsOf(t, sc.enc, encounter.BeatActivated), tc.activation)
			require.Len(t, sc.striker.at, 1, "and he swings")
		})
	}
}

// Test 4b-2.8: on a round of the world, both forms are read.
func TestAHitPointsNumberAndAShareAreReadOnARoundOfTheWorld(t *testing.T) {
	for _, tc := range []struct {
		name     string
		hurt     encounter.When
		now, max int
		says     bool
	}{
		{"a number: 4 of 9 says it", hurtNumber(5), 4, 9, true},
		{"a number: 9 of 9 does not", hurtNumber(5), 9, 9, false},
		{"a share: 4 of 9 says it", hurtShare(50), 4, 9, true},
		{"a share: 9 of 9 does not", hurtShare(50), 9, 9, false},
		{"a number does not scale: 6 of 90 says it", hurtNumber(8), 6, 90, true},
		{"a share scales: 60 of 90 does not", hurtShare(50), 60, 90, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := encounter.Table{encounter.AnswerTime: {
				{Weight: 1, When: []encounter.When{tc.hurt}, Say: "I yield"},
				{Weight: 1, Hold: true},
			}}
			sc := abilityScene{table: table, world: true}.build(t)
			sc.host.Hurt(lieutenant, tc.now, tc.max)
			require.NoError(t, encounter.ExportedSpendWorldAction(sc.enc, alice))

			all, err := sc.enc.ReadRecord(&encounter.ReadRecordInput{})
			require.NoError(t, err)
			var said bool
			for _, entry := range all.Entries {
				var b map[string]any
				require.NoError(t, json.Unmarshal(entry.Payload, &b))
				if b["beat"] == "speech" && b["words"] == "I yield" {
					said = true
				}
			}
			require.Equal(t, tc.says, said)
			require.NotEmpty(t, picksOf(t, sc.enc), "he had his round")
		})
	}
}

// Test 4b-2.4: saved and loaded, each as itself, alone and inside `not`.
func TestAHitPointsNumberAndAShareAreSavedAndLoadedAsThemselves(t *testing.T) {
	number, share := hurtNumber(8), hurtShare(30)
	notNumber, notShare := hurtNumber(9), hurtShare(40)
	table := silenceTable()
	table[encounter.AnswerTime] = append([]encounter.Answer{
		{Weight: 1, When: []encounter.When{number}, Hold: true},
		{Weight: 1, When: []encounter.When{share}, Hold: true},
		{Weight: 1, When: []encounter.When{{Not: &notNumber}}, Hold: true},
		{Weight: 1, When: []encounter.When{{Not: &notShare}}, Hold: true},
	}, table[encounter.AnswerTime]...)
	enc, members := symbolsSceneWith(t, table)
	goblinTurn(t, enc)

	raw, err := json.Marshal(enc.ToData())
	require.NoError(t, err)

	var saved encounter.EncounterData
	require.NoError(t, json.Unmarshal(raw, &saved))
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: saved, Capabilities: symbolsCaps(members)})
	require.NoError(t, err)
	var seen bool
	for _, m := range loaded.ToData().Members {
		if m.ID != goblin {
			continue
		}
		seen = true
		rows := m.Behaviors.Instances[0].Instance.Table[encounter.AnswerTime]
		require.Equal(t, behavior.HitPointsWhen{AtMost: 8}, *rows[0].When[0].HitPoints)
		require.Equal(t, behavior.HitPointsWhen{AtMost: 30, Percent: true}, *rows[1].When[0].HitPoints)
		require.Equal(t, behavior.HitPointsWhen{AtMost: 9}, *rows[2].When[0].Not.HitPoints)
		require.Equal(t, behavior.HitPointsWhen{AtMost: 40, Percent: true}, *rows[3].When[0].Not.HitPoints)
		require.Nil(t, rows[4].When[0].HitPoints)
	}
	require.True(t, seen, "the goblin is in the loaded encounter")
	require.Equal(t, 2, strings.Count(string(raw), `"Percent":true`), "the share alone and inside not say it, the numbers do not")
}
