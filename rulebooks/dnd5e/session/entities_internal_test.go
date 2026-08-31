// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// TestProjectCharacterFoldsFromTheRecord is the last of three generations of
// this test, and the shrinking is the story.
//
// It began as TestProjectCharacterRefusesAnUnattachedSheet: an unattached sheet
// made the projection refuse, because EffectiveAC refuses (rpg-toolkit#1276)
// and a refusal flattened into base armour is how a monk fought at 10+DEX for
// the life of the character. That mattered while the fold ran HERE, on whatever
// bus this seam had attached the sheet to.
//
// It then became TestProjectCharacterFoldsWithoutAnAttachedSheet, when the fold
// moved to resolution and an unattached sheet stopped being the fold's problem.
// The sheet was still passed, so the test still had one to leave unattached.
//
// Now there is no sheet to pass. A record goes down and numbers come back, so
// the failure mode is not guarded, not tolerated, but UNREPRESENTABLE — which
// is the shape a fix takes when it stops being a check.
//
// The fixture keeps its trap through all three. 13 is the stale scalar on the
// record; 15 is what the chain folds to — 10 base + 3 DEX + 2 WIS, Unarmored
// Defense for a monk. A projection that flattened back to the record answers 13
// and says so.
func TestProjectCharacterFoldsFromTheRecord(t *testing.T) {
	ud := conditions.NewUnarmoredDefenseCondition(conditions.UnarmoredDefenseInput{
		MemberID: "unattached", Type: conditions.UnarmoredDefenseMonk,
		Source: "dnd5e:classes:monk",
	})
	raw, err := ud.ToJSON()
	require.NoError(t, err)

	record := &character.Data{
		ID: "unattached", PlayerID: "p1", Name: "Unattached Monk",
		Level: 1, ProficiencyBonus: 2,
		RaceID: races.Human, ClassID: classes.Monk,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12, abilities.DEX: 16, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 14, abilities.CHA: 8,
		},
		HitPoints: 9, MaxHitPoints: 9,
		// The stale scalar a flattening projection would hand back.
		ArmorClass:     13,
		EquipmentSlots: character.EquipmentSlots{},
		Conditions:     []json.RawMessage{raw},
	}

	projected, err := projectCharacter(context.Background(), "unattached", record)
	require.NoError(t, err)

	state := characterStateFrom(projected)
	require.Equal(t, 15, state.ArmorClass,
		"10 base + 3 DEX + 2 WIS: resolution folded the condition in")
	require.NotEqual(t, 13, state.ArmorClass, "and did not fall back to the record's scalar")

	require.Equal(t, 30, state.Speed,
		"and Speed came off the loaded sheet — it is on no record, so echoing bytes cannot produce it")
}

// A record this seam cannot get a character out of is ErrBadCharacter, which is
// the vocabulary a host branches on.
//
// The guard this replaces was richer and is now unrepresentable: projectCharacter
// used to take a sheet AND a record, and checked they described the same
// character, because one caller passing both from one load is a habit rather
// than a guarantee. There is one argument now, so there is nothing to disagree
// with itself.
func TestProjectCharacterRefusesARecordItCannotUse(t *testing.T) {
	t.Run("no record at all", func(t *testing.T) {
		projected, err := projectCharacter(context.Background(), "hero", nil)

		require.ErrorIs(t, err, ErrBadCharacter)
		require.Nil(t, projected)
	})

	t.Run("a record resolution refuses", func(t *testing.T) {
		// No ID: the entry validates its participant, and a sheet with no ID
		// cannot be read back out of the cast it was just put into.
		projected, err := projectCharacter(context.Background(), "hero", &character.Data{})

		require.ErrorIs(t, err, ErrBadCharacter)
		require.Nil(t, projected)
		require.Contains(t, err.Error(), "hero", "the error names who was being projected")
	})
}

// characterStateFrom answers nil for nil, so a caller that has no projection
// publishes no state rather than a zero-valued one.
//
// A zero CharacterState would be a character with no name, no hit points and an
// armour class of 0 — which reads like a real answer about a very dead person.
func TestCharacterStateFromTreatsNilAsAbsence(t *testing.T) {
	require.Nil(t, characterStateFrom(nil))
}
