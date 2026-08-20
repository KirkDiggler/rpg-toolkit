// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
)

func TestNewDamageChainEventCopiesMarkedWeaponMetadata(t *testing.T) {
	got := NewDamageChainEvent(DamageChainInput{
		WeaponDamageDice: "1d8",
		WeaponDamageType: damage.Fire,
	})

	if got.WeaponDamageDice != "1d8" {
		t.Fatalf("WeaponDamageDice = %q, want %q", got.WeaponDamageDice, "1d8")
	}
	if got.WeaponDamageType != damage.Fire {
		t.Fatalf("WeaponDamageType = %q, want %q", got.WeaponDamageType, damage.Fire)
	}
}
