// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
)

func TestDurationType_String(t *testing.T) {
	tests := []struct {
		name     string
		duration events.DurationType
		expected string
	}{
		{"permanent", events.DurationTypePermanent, "permanent"},
		{"rounds", events.DurationTypeRounds, "rounds"},
		{"minutes", events.DurationTypeMinutes, "minutes"},
		{"hours", events.DurationTypeHours, "hours"},
		{"encounter", events.DurationTypeEncounter, "encounter"},
		{"concentration", events.DurationTypeConcentration, "concentration"},
		{"short rest", events.DurationTypeShortRest, "short_rest"},
		{"long rest", events.DurationTypeLongRest, "long_rest"},
		{"until damaged", events.DurationTypeUntilDamaged, "until_damaged"},
		{"until save", events.DurationTypeUntilSave, "until_save"},
		{"unknown", events.DurationType(999), "unknown"}, // Test default case
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.duration.String())
		})
	}
}

func TestPermanentDuration(t *testing.T) {
	d := &events.PermanentDuration{}

	assert.Equal(t, events.DurationTypePermanent, d.Type())
	assert.Equal(t, "Permanent", d.Description())

	// Should never expire
	assert.False(t, d.IsExpired(time.Now(), 0))
	assert.False(t, d.IsExpired(time.Now().Add(100*time.Hour), 1000))
}

func TestRoundsDuration(t *testing.T) {
	tests := []struct {
		name         string
		duration     *events.RoundsDuration
		currentRound int
		expired      bool
		description  string
	}{
		{
			name: "1 round not expired",
			duration: &events.RoundsDuration{
				Rounds:       1,
				StartRound:   1,
				IncludeStart: true,
			},
			currentRound: 1,
			expired:      false,
			description:  "1 round",
		},
		{
			name: "1 round expired",
			duration: &events.RoundsDuration{
				Rounds:       1,
				StartRound:   1,
				IncludeStart: true,
			},
			currentRound: 2,
			expired:      true,
			description:  "1 round",
		},
		{
			name: "3 rounds not expired",
			duration: &events.RoundsDuration{
				Rounds:       3,
				StartRound:   1,
				IncludeStart: true,
			},
			currentRound: 3,
			expired:      false,
			description:  "3 rounds",
		},
		{
			name: "3 rounds expired",
			duration: &events.RoundsDuration{
				Rounds:       3,
				StartRound:   1,
				IncludeStart: true,
			},
			currentRound: 4,
			expired:      true,
			description:  "3 rounds",
		},
		{
			name: "exclude start round",
			duration: &events.RoundsDuration{
				Rounds:       2,
				StartRound:   1,
				IncludeStart: false,
			},
			currentRound: 2,
			expired:      false,
			description:  "2 rounds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, events.DurationTypeRounds, tt.duration.Type())
			assert.Equal(t, tt.description, tt.duration.Description())
			assert.Equal(t, tt.expired, tt.duration.IsExpired(time.Now(), tt.currentRound))
		})
	}
}

func TestMinutesDuration(t *testing.T) {
	startTime := time.Now()
	d := &events.MinutesDuration{
		Minutes:   10,
		StartTime: startTime,
	}

	assert.Equal(t, events.DurationTypeMinutes, d.Type())
	assert.Equal(t, "10 minutes", d.Description())

	// Not expired at start
	assert.False(t, d.IsExpired(startTime, 0))

	// Not expired after 5 minutes
	assert.False(t, d.IsExpired(startTime.Add(5*time.Minute), 0))

	// Expired after 10 minutes
	assert.True(t, d.IsExpired(startTime.Add(10*time.Minute), 0))

	// Test single minute description
	d1 := &events.MinutesDuration{Minutes: 1, StartTime: startTime}
	assert.Equal(t, "1 minute", d1.Description())
}

func TestHoursDuration(t *testing.T) {
	startTime := time.Now()
	d := &events.HoursDuration{
		Hours:     8,
		StartTime: startTime,
	}

	assert.Equal(t, events.DurationTypeHours, d.Type())
	assert.Equal(t, "8 hours", d.Description())

	// Not expired at start
	assert.False(t, d.IsExpired(startTime, 0))

	// Not expired after 4 hours
	assert.False(t, d.IsExpired(startTime.Add(4*time.Hour), 0))

	// Expired after 8 hours
	assert.True(t, d.IsExpired(startTime.Add(8*time.Hour), 0))

	// Test single hour description
	d1 := &events.HoursDuration{Hours: 1, StartTime: startTime}
	assert.Equal(t, "1 hour", d1.Description())
}

func TestEncounterDuration(t *testing.T) {
	d := &events.EncounterDuration{EncounterActive: true}

	assert.Equal(t, events.DurationTypeEncounter, d.Type())
	assert.Equal(t, "Until end of encounter", d.Description())

	// Not expired while encounter active
	assert.False(t, d.IsExpired(time.Now(), 0))

	// Expired when encounter ends
	d.EncounterActive = false
	assert.True(t, d.IsExpired(time.Now(), 0))
}

func TestConcentrationDuration(t *testing.T) {
	d := &events.ConcentrationDuration{ConcentrationBroken: false}

	assert.Equal(t, events.DurationTypeConcentration, d.Type())
	assert.Equal(t, "Concentration", d.Description())

	// Not expired while concentration maintained
	assert.False(t, d.IsExpired(time.Now(), 0))

	// Expired when concentration broken
	d.ConcentrationBroken = true
	assert.True(t, d.IsExpired(time.Now(), 0))
}

func TestShortRestDuration(t *testing.T) {
	d := &events.ShortRestDuration{ShortRestTaken: false}

	assert.Equal(t, events.DurationTypeShortRest, d.Type())
	assert.Equal(t, "Until short rest", d.Description())

	// Not expired before rest
	assert.False(t, d.IsExpired(time.Now(), 0))

	// Expired after rest
	d.ShortRestTaken = true
	assert.True(t, d.IsExpired(time.Now(), 0))
}

func TestLongRestDuration(t *testing.T) {
	d := &events.LongRestDuration{LongRestTaken: false}

	assert.Equal(t, events.DurationTypeLongRest, d.Type())
	assert.Equal(t, "Until long rest", d.Description())

	// Not expired before rest
	assert.False(t, d.IsExpired(time.Now(), 0))

	// Expired after rest
	d.LongRestTaken = true
	assert.True(t, d.IsExpired(time.Now(), 0))
}

func TestUntilDamagedDuration(t *testing.T) {
	d := &events.UntilDamagedDuration{DamageTaken: false}

	assert.Equal(t, events.DurationTypeUntilDamaged, d.Type())
	assert.Equal(t, "Until damaged", d.Description())

	// Not expired before damage
	assert.False(t, d.IsExpired(time.Now(), 0))

	// Expired after damage
	d.DamageTaken = true
	assert.True(t, d.IsExpired(time.Now(), 0))
}

func TestUntilSaveDuration(t *testing.T) {
	d := &events.UntilSaveDuration{
		Ability:   "wisdom",
		DC:        15,
		SaveMade:  false,
		EndOfTurn: true,
	}

	assert.Equal(t, events.DurationTypeUntilSave, d.Type())
	assert.Equal(t, "Until wisdom save (DC 15) at end of turn", d.Description())

	// Not expired before save
	assert.False(t, d.IsExpired(time.Now(), 0))

	// Expired after save made
	d.SaveMade = true
	assert.True(t, d.IsExpired(time.Now(), 0))

	// Test start of turn description
	d2 := &events.UntilSaveDuration{
		Ability:   "constitution",
		DC:        10,
		EndOfTurn: false,
	}
	assert.Equal(t, "Until constitution save (DC 10) at start of turn", d2.Description())
}

func TestDurationInContext(t *testing.T) {
	ctx := events.NewEventContext()
	duration := &events.RoundsDuration{
		Rounds:     3,
		StartRound: 1,
	}

	// Set duration in context
	ctx.Set(events.ContextKeyDuration, duration)

	// Get duration back
	retrieved, ok := ctx.GetDuration(events.ContextKeyDuration)
	require.True(t, ok)
	require.NotNil(t, retrieved)
	assert.Equal(t, events.DurationTypeRounds, retrieved.Type())
}
