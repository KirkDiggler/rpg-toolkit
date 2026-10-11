// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monstertraits

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// A saved In Fog condition names a removed type: the monster's conditions load
// refuses it as an unknown ref instead of skipping it.
func TestLoadMonsterConditionsRefusesARemovedInFog(t *testing.T) {
	ctx := context.Background()
	bus := events.NewEventBus()
	m, err := monster.LoadFromData(ctx, &monster.Data{
		ID: "skeleton-fog", Name: "Skeleton", Ref: refs.Monsters.Skeleton(),
		HitPoints: 13, MaxHitPoints: 13, ArmorClass: 13, AbilityScores: shared.AbilityScores{},
	}, bus)
	require.NoError(t, err)

	immunity, err := conditions.ImmunityJSON("skeleton-fog", damage.Poison)
	require.NoError(t, err)
	inFog := json.RawMessage(`{"ref":{"module":"dnd5e","type":"conditions","id":"in_fog"},` +
		`"member_id":"skeleton-fog","source_id":"area-1","source_ref":{"module":"dnd5e","type":"spells","id":"fog-cloud"}}`)

	err = LoadMonsterConditions(ctx, m, []json.RawMessage{inFog, immunity}, bus, dice.NewRoller())
	require.ErrorIs(t, err, conditions.ErrUnknownCondition)
}
