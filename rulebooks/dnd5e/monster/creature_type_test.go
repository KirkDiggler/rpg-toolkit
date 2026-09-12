// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/require"
)

func TestCreatureTypeReadsOldCatalogueSheetsWithoutRewritingThem(t *testing.T) {
	for _, tc := range []struct {
		data *Data
		want string
	}{
		{&Data{Ref: refs.Monsters.Skeleton()}, "undead"}, {&Data{Ref: refs.Monsters.AnimatedArmor()}, "construct"},
		{&Data{Ref: refs.Monsters.Wolf()}, "beast"}, {&Data{CreatureType: "dragon"}, "dragon"}, {&Data{}, ""},
	} {
		m, err := Load(context.Background(), tc.data)
		require.NoError(t, err)
		require.Equal(t, tc.want, m.CreatureType())
		require.Equal(t, tc.data.CreatureType, m.ToData().CreatureType)
	}
}

func TestNegativeHealingDoesNotMutateMonster(t *testing.T) {
	for _, traced := range []bool{false, true} {
		m := New(Config{ID: "target", HP: 20})
		m.hp = 5
		event := dnd5eEvents.HealingReceivedEvent{TargetID: "target", Amount: -2}
		if traced {
			amount := -2
			event.Calculation = &dnd5eEvents.RollCalculation{Total: -2, Components: []dnd5eEvents.RollComponent{{Source: dnd5eEvents.RollSource{Ref: refs.Spells.CureWounds(), Name: "Cure Wounds"}, Modifier: &amount}}}
		}
		require.Error(t, m.onHealingReceived(context.Background(), events.NewEventBus(), event))
		require.Equal(t, 5, m.hp)
		require.False(t, m.dirty)
	}
}
