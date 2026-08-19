// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

func TestNewDamageChainEventPreservesExplicitPrimaryMetadata(t *testing.T) {
	event := dnd5eEvents.NewDamageChainEvent(dnd5eEvents.DamageChainInput{
		WeaponDamageDice: "1d8",
		WeaponDamageType: damage.Slashing,
	})

	require.Equal(t, "1d8", event.WeaponDamageDice)
	require.Equal(t, damage.Slashing, event.WeaponDamageType)
}
