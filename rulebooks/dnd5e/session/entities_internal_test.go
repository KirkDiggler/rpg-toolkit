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

// projectCharacter folds through resolution now, so the sheet it is handed no
// longer has to be attached — and the number is right either way.
//
// This test used to pin the OPPOSITE, and the change is the point of the slice
// rather than a relaxation. It pinned that an unattached sheet made the
// projection refuse, because EffectiveAC refuses (rpg-toolkit#1276) and a
// refusal flattened into base armour is how a monk fought at 10+DEX for the
// life of the character. That guard mattered while the fold ran HERE, on
// whatever bus this seam had happened to attach the sheet to.
//
// The fold now runs in resolution, off the RECORD, on a bus resolution builds
// and tears down with the truth installed. So the failure the old test guarded
// cannot occur rather than being caught when it does — and an unattached sheet,
// which used to be the broken case, is simply not the fold's problem.
//
// The fixture keeps its trap. 13 is the stale scalar on the sheet; 15 is what
// the chain folds to — 10 base + 3 DEX + 2 WIS, Unarmored Defense for a monk.
// A projection that flattened back to the sheet answers 13 and says so.
func TestProjectCharacterFoldsWithoutAnAttachedSheet(t *testing.T) {
	ud := conditions.NewUnarmoredDefenseCondition(conditions.UnarmoredDefenseInput{
		CharacterID: "unattached", Type: conditions.UnarmoredDefenseMonk,
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

	// Load, deliberately WITHOUT Attach: no bus, nothing subscribed here.
	sheet, err := character.Load(context.Background(), record)
	require.NoError(t, err)

	state, err := projectCharacter(context.Background(), sheet, record)

	require.NoError(t, err, "the fold does not need this seam's bus any more")
	require.Equal(t, 15, state.ArmorClass,
		"10 base + 3 DEX + 2 WIS: resolution folded the condition in")
	require.NotEqual(t, 13, state.ArmorClass, "and did not fall back to the sheet's scalar")
}

// The sheet and the record have to describe the same character. One caller
// passing both from one load is a habit, not a guarantee, and a mismatch would
// fold one character's conditions into another's state and look plausible on
// the way out.
func TestProjectCharacterRefusesASheetAndRecordThatDisagree(t *testing.T) {
	sheet, err := character.Load(context.Background(), &character.Data{
		ID: "hero", PlayerID: "p1", Name: "Hero",
		Level: 1, ProficiencyBonus: 2,
		RaceID: races.Human, ClassID: classes.Monk,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 12, abilities.DEX: 16, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 14, abilities.CHA: 8,
		},
		HitPoints: 9, MaxHitPoints: 9, EquipmentSlots: character.EquipmentSlots{},
	})
	require.NoError(t, err)

	t.Run("a different character's record", func(t *testing.T) {
		state, err := projectCharacter(context.Background(), sheet, &character.Data{ID: "somebody-else"})

		require.ErrorIs(t, err, ErrBadCharacter)
		require.Nil(t, state)
		require.Contains(t, err.Error(), "somebody-else", "the error names the record it was handed")
	})

	t.Run("no record at all", func(t *testing.T) {
		state, err := projectCharacter(context.Background(), sheet, nil)

		require.ErrorIs(t, err, ErrBadCharacter)
		require.Nil(t, state)
	})
}

// A nil character is absence, not failure, and stays a nil projection — Join
// relies on this for members that carry no sheet.
func TestProjectCharacterTreatsNilAsAbsenceNotFailure(t *testing.T) {
	state, err := projectCharacter(context.Background(), nil, nil)

	require.NoError(t, err)
	require.Nil(t, state)
}
