// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package table

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTemperRoundTripsTheVocabulary(t *testing.T) {
	for _, word := range []string{"soldier", "coward", "aggressive"} {
		t.Run(word, func(t *testing.T) {
			got, err := ParseTemper(word)

			require.NoError(t, err)
			require.Equal(t, word, got.String(), "String is the inverse of ParseTemper")
		})
	}
}

func TestParseTemperRefusesWhatNoAuthorWrote(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "unknown word", input: "brave"},
		{name: "a retired mind word", input: "berserker"},
		{name: "wrong case", input: "Coward"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTemper(tc.input)

			require.Error(t, err)
			require.Equal(t, Temper(""), got)
			require.Contains(t, err.Error(), "invalid temper")
			for _, word := range authorable {
				require.Contains(t, err.Error(), word.String(),
					"a refusal names the whole vocabulary, so an author can see what they meant")
			}
		})
	}
}

// An absent temperament is the caller's zero, and the design says that zero is
// a soldier. What it must never be is an all-zero profile, which would not
// weight the creature's table but silence it.
func TestAnAbsentTemperIsASoldier(t *testing.T) {
	require.Equal(t, TemperSoldier.Profile(), Temper("").Profile())
}

func TestSoldierLeavesEveryAuthoredWeightAsWritten(t *testing.T) {
	require.Equal(t, Profile{Attack: 100, Toward: 100, Away: 100, Flee: 100, Hold: 100}, TemperSoldier.Profile())
}

// Reflection rather than five named asserts: a word added to Profile without a
// number in every temperament is exactly the mistake worth catching, and a
// hand-written list of fields would not catch it.
func TestEveryProfileLoadsEveryWord(t *testing.T) {
	for temper, profile := range profiles {
		value := reflect.ValueOf(profile)
		for i := 0; i < value.NumField(); i++ {
			require.NotZero(t, value.Field(i).Int(),
				"%s leaves %s at zero, which deletes that word from every table it touches rather than weighting it",
				temper, value.Type().Field(i).Name)
		}
	}
}

func TestEveryAuthorableWordHasNumbers(t *testing.T) {
	require.Len(t, profiles, len(authorable), "a word an author can write and a word with numbers are the same set")
	for _, temper := range authorable {
		require.Contains(t, profiles, temper)
	}
}

// The words mean something, and what they mean is which way the die leans.
func TestCowardAndAggressiveLeanOppositeWays(t *testing.T) {
	coward := TemperCoward.Profile()
	aggressive := TemperAggressive.Profile()
	soldier := TemperSoldier.Profile()

	require.Greater(t, coward.Away, coward.Attack, "a coward would rather be elsewhere than in reach")
	require.Greater(t, coward.Flee, soldier.Flee, "and runs where a soldier would not")
	require.Less(t, coward.Attack, soldier.Attack)

	require.Greater(t, aggressive.Attack, coward.Attack, "the aggressive one comes at you off a table that only suggested it")
	require.Less(t, aggressive.Away, soldier.Away, "and gives ground where a soldier would")
	require.Less(t, aggressive.Hold, soldier.Hold, "standing still is the one thing it is worse at")
}
