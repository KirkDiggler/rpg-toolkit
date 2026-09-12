// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/stretchr/testify/require"
)

func TestCureWoundsUsesLoadedClassAbilityAndLifeDomain(t *testing.T) {
	for _, tc := range []struct {
		class                classes.Class
		subclass             classes.Subclass
		modifier, components int
	}{
		{classes.Cleric, "", 3, 1}, {classes.Cleric, classes.LifeDomain, 3, 2}, {classes.Bard, "", -1, 1},
	} {
		data := &Data{ID: "caster", Name: "Caster", Level: 1, ClassID: tc.class, SubclassID: tc.subclass,
			AbilityScores: shared.AbilityScores{abilities.WIS: 16, abilities.CHA: 8}, HitPoints: 10, MaxHitPoints: 10}
		ch, err := Load(context.Background(), data)
		require.NoError(t, err)
		definition := ch.CastDefinition(spells.CureWounds)
		require.NotNil(t, definition)
		require.NoError(t, definition.Validate())
		require.Len(t, definition.Cast.Healing.Modifiers, tc.components)
		require.Equal(t, tc.modifier, definition.Cast.Healing.Modifiers[0].Amount)
		require.Nil(t, definition.Cast.Save)
		require.Nil(t, definition.Cast.Concentration)
		clone := definition.Clone()
		clone.Cast.Healing.Modifiers[0].Source.Ref.ID = "changed"
		clone.Cast.HealingExcludes[0] = "changed"
		require.NotEqual(t, "changed", definition.Cast.Healing.Modifiers[0].Source.Ref.ID)
		require.Equal(t, "undead", definition.Cast.HealingExcludes[0])
	}
}

func TestNegativeHealingDoesNotMutateCharacter(t *testing.T) {
	for _, traced := range []bool{false, true} {
		ch := &Character{id: "hero", hitPoints: 5, maxHitPoints: 20}
		event := dnd5eEvents.HealingReceivedEvent{TargetID: "hero", Amount: -2}
		if traced {
			amount := -2
			event.Calculation = &dnd5eEvents.RollCalculation{Total: -2, Components: []dnd5eEvents.RollComponent{{Source: dnd5eEvents.RollSource{Ref: refs.Spells.CureWounds(), Name: "Cure Wounds"}, Modifier: &amount}}}
		}
		require.Error(t, ch.onHealingReceived(context.Background(), events.NewEventBus(), event))
		require.Equal(t, 5, ch.hitPoints)
		require.False(t, ch.dirty)
	}
}
