// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// captureHandler collects the WARNINGS written to it, so a test can assert on
// what a lenient load said rather than on whether it stayed quiet.
//
// Warnings only, deliberately. These tests are about one claim — a drop is
// audible — and a handler that collected every level would make them fail the
// day somebody adds an unrelated Info line to a loader, which is a failure that
// teaches nothing. It also lets TestACleanLoadSaysNothing mean what it says:
// nothing was DROPPED, rather than nothing was logged at all.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn
}

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())

	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(string) slog.Handler      { return h }

// attrs flattens one captured record into a map, so an assertion can name a
// key instead of walking the record.
func attrs(r slog.Record) map[string]string {
	out := make(map[string]string, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		out[a.Key] = a.Value.String()

		return true
	})

	return out
}

// captureWarnings redirects the default logger for one test and hands back the
// records it collected.
func captureWarnings(t *testing.T) *captureHandler {
	t.Helper()

	handler := &captureHandler{}
	previous := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return handler
}

// TestALenientDropIsAudible is the pin under D10: a lenient load still drops
// what it cannot read, and is no longer quiet about it.
//
// The whole failure this closes is one of OBSERVABILITY, not of behaviour. A
// character carrying a blob this build cannot parse must still enter play —
// refusing puts one bad condition between a player and the game — but dropping
// in silence is how a sheet loses a condition with nobody able to see why. So
// the assertion is on what was SAID, and a test that only checked the character
// still loaded would have passed before this change and after it.
func TestALenientDropIsAudible(t *testing.T) {
	logs := captureWarnings(t)

	// Truncated JSON: broken before the ref, which is the case that matters.
	data := &Data{
		ID: "noisy", PlayerID: "p1", Name: "Noisy", Level: 1, ProficiencyBonus: 2,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 9, MaxHitPoints: 9,
		EquipmentSlots: EquipmentSlots{},
		Conditions:     []json.RawMessage{json.RawMessage(`{"ref":"nonsense","x":`)},
	}

	loaded, err := LoadFromData(context.Background(), data, events.NewEventBus())

	require.NoError(t, err, "lenient still means the character enters play")
	require.NotNil(t, loaded)

	require.Len(t, logs.records, 1, "exactly one drop, exactly one warning")

	record := logs.records[0]
	require.Equal(t, slog.LevelWarn, record.Level)

	got := attrs(record)
	require.Equal(t, "noisy", got["character"], "the line names whose sheet lost something")
	require.Equal(t, "condition", got["dropped"], "and what kind of thing it was")
	require.Equal(t, "0", got["index"], "and which entry, which is always knowable")
	require.Contains(t, got["reason"], "unexpected end of JSON input",
		"and why, in the loader's own words")

	// THE REF IS ABSENT, and that is the fact rather than a gap. This blob is
	// too broken to name itself; a placeholder here would read like a ref that
	// exists and send somebody looking for it.
	_, hasRef := got["ref"]
	require.False(t, hasRef, "a blob that cannot be parsed has no ref to report")
}

// TestALenientDropNamesTheRefWhenThereIsOne is the other half: when the blob
// parses far enough to say what it is, the warning says so too.
//
// Same drop, different knowledge. The ref is what turns "something was lost"
// into "the hexed condition was lost", and it is the difference between a line
// somebody can act on and a line they can only notice.
func TestALenientDropNamesTheRefWhenThereIsOne(t *testing.T) {
	logs := captureWarnings(t)

	data := &Data{
		ID: "named", PlayerID: "p1", Name: "Named", Level: 1, ProficiencyBonus: 2,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 9, MaxHitPoints: 9,
		EquipmentSlots: EquipmentSlots{},
		Conditions: []json.RawMessage{
			json.RawMessage(`{"ref":"homebrew:conditions:hexed","character_id":"named","stacks":3}`),
		},
	}

	loaded, err := LoadFromData(context.Background(), data, events.NewEventBus())

	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Len(t, logs.records, 1)

	got := attrs(logs.records[0])
	require.Equal(t, "homebrew:conditions:hexed", got["ref"],
		"the blob named itself, so the warning names it too")
	require.Contains(t, got["reason"], "hexed")
}

// TestACleanLoadSaysNothing keeps the warnings meaningful.
//
// A log line that fires on a healthy character is a line people learn to
// ignore, and the next real drop scrolls past with it.
func TestACleanLoadSaysNothing(t *testing.T) {
	logs := captureWarnings(t)

	data := &Data{
		ID: "clean", PlayerID: "p1", Name: "Clean", Level: 1, ProficiencyBonus: 2,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 9, MaxHitPoints: 9,
		EquipmentSlots: EquipmentSlots{},
	}

	_, err := LoadFromData(context.Background(), data, events.NewEventBus())

	require.NoError(t, err)
	require.Empty(t, logs.records, "nothing was dropped, so nothing is said")
}
