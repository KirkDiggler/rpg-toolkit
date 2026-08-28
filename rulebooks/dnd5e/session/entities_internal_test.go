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

// projectCharacter carries a refusal out instead of flattening it into a
// plausible number.
//
// Join always hands it an attached sheet, so this cannot be reached through the
// public verb today — which is exactly why it is worth pinning here. The value
// of EffectiveAC refusing (rpg-toolkit#1276) is entirely lost if the one caller
// in this package quietly turns the refusal back into base armour, and nothing
// about Join's current loading would fail if someone did.
func TestProjectCharacterCarriesTheRefusalOut(t *testing.T) {
	ud := conditions.NewUnarmoredDefenseCondition(conditions.UnarmoredDefenseInput{
		CharacterID: "unattached", Type: conditions.UnarmoredDefenseMonk,
		Source: "dnd5e:classes:monk",
	})
	raw, err := ud.ToJSON()
	require.NoError(t, err)

	// Load, deliberately WITHOUT Attach: no bus, so the AC chain cannot fold.
	sheet, err := character.Load(context.Background(), &character.Data{
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
	})
	require.NoError(t, err)

	state, err := projectCharacter(context.Background(), sheet)

	require.Error(t, err, "a sheet that cannot fold its AC must not be projected")
	require.Nil(t, state, "no half-built state may escape alongside the error")
	require.Contains(t, err.Error(), "unattached", "the error names the character")
}

// A nil character is absence, not failure, and stays a nil projection — Join
// relies on this for members that carry no sheet.
func TestProjectCharacterTreatsNilAsAbsenceNotFailure(t *testing.T) {
	state, err := projectCharacter(context.Background(), nil)

	require.NoError(t, err)
	require.Nil(t, state)
}
