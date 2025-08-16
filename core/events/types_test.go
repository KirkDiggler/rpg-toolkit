// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core/events"
)

func TestTypedConstants(t *testing.T) {
	// This test demonstrates how rulebooks will use these types

	// Event types
	const (
		AttackRoll  events.EventType = "combat.attack.roll"
		DamageRoll  events.EventType = "combat.damage.roll"
		SavingThrow events.EventType = "save.throw"
	)

	// Modifier types
	const (
		ModifierAttackBonus events.ModifierType = "attack_bonus"
		ModifierDamageBonus events.ModifierType = "damage_bonus"
		ModifierAdvantage   events.ModifierType = "advantage"
	)

	// Modifier sources
	const (
		SourceRage        events.ModifierSource = "rage"
		SourceBless       events.ModifierSource = "bless"
		SourceProficiency events.ModifierSource = "proficiency"
	)

	// Verify they work as expected
	if AttackRoll != "combat.attack.roll" {
		t.Error("EventType constant not working correctly")
	}

	if ModifierAttackBonus != "attack_bonus" {
		t.Error("ModifierType constant not working correctly")
	}

	if SourceRage != "rage" {
		t.Error("ModifierSource constant not working correctly")
	}

	// Verify type safety - these should be different types
	eventType := AttackRoll
	modType := ModifierAttackBonus
	modSource := SourceRage

	// These should work
	_ = eventType
	_ = modType
	_ = modSource
}
