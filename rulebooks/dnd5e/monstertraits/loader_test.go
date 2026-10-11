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

// The legacy load path drops a saved In Fog condition rather than failing the
// monster: the type retired, and the monster's traits still apply.
func TestLoadMonsterConditionsDropsARetiredInFog(t *testing.T) {
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

	require.NoError(t, LoadMonsterConditions(ctx, m, []json.RawMessage{inFog, immunity}, bus, dice.NewRoller()))
	saved, err := json.Marshal(m.ToData())
	require.NoError(t, err)
	require.NotContains(t, string(saved), `"in_fog"`)
	require.Contains(t, string(saved), refs.MonsterTraits.Immunity().ID, "the trait beside it still loads")
}
