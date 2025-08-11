// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resources_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

func TestGenericTriggers(t *testing.T) {
	owner := &MockEntity{id: "cleric-1", typ: "character"}

	// Create a resource with custom triggers
	channelDivinity := resources.NewSimpleResource(resources.SimpleResourceConfig{
		ID:      "channel-divinity",
		Type:    resources.ResourceTypeAbilityUse,
		Owner:   owner,
		Key:     core.MustNewRef("channel_divinity", "dnd5e", "resource"),
		Current: 0,
		Maximum: 2,
		RestoreTriggers: map[string]int{
			"game.short_rest": -1, // Full restore
			"game.long_rest":  -1, // Full restore
			"game.dawn":       1,  // Restore 1 at dawn
		},
	})

	// Test dawn trigger
	restoreAmount := channelDivinity.RestoreOnTrigger("game.dawn")
	if restoreAmount != 1 {
		t.Errorf("Expected dawn to restore 1, got %d", restoreAmount)
	}

	// Test short rest trigger
	restoreAmount = channelDivinity.RestoreOnTrigger("game.short_rest")
	expectedAmount := channelDivinity.Maximum() - channelDivinity.Current()
	if restoreAmount != expectedAmount {
		t.Errorf("Expected short rest to restore %d (to full), got %d", expectedAmount, restoreAmount)
	}

	// Test unknown trigger
	restoreAmount = channelDivinity.RestoreOnTrigger("unknown_trigger")
	if restoreAmount != 0 {
		t.Errorf("Expected unknown trigger to restore 0, got %d", restoreAmount)
	}
}

func TestPoolProcessRestoration(t *testing.T) {
	owner := &MockEntity{id: "char-1", typ: "character"}
	pool := resources.NewSimplePool(owner)
	bus := events.NewBus()

	// Create resources with different triggers
	divineResource := resources.NewSimpleResource(resources.SimpleResourceConfig{
		ID:      "divine-power",
		Type:    resources.ResourceTypeCustom,
		Owner:   owner,
		Key:     core.MustNewRef("divine_power", "dnd5e", "resource"),
		Current: 0,
		Maximum: 3,
		RestoreTriggers: map[string]int{
			"dawn":            -1, // Full at dawn
			"prayer_answered": 1,  // +1 when prayer answered
		},
	})

	arcaneResource := resources.NewSimpleResource(resources.SimpleResourceConfig{
		ID:      "arcane-surge",
		Type:    resources.ResourceTypeCustom,
		Owner:   owner,
		Key:     core.MustNewRef("arcane_surge", "dnd5e", "resource"),
		Current: 1,
		Maximum: 2,
		RestoreTriggers: map[string]int{
			"arcane_rest": -1, // Full on arcane rest
			"ley_line":    2,  // +2 when near ley line
		},
	})

	// Add resources to pool
	if err := pool.Add(divineResource); err != nil {
		t.Fatalf("Failed to add divine resource: %v", err)
	}
	if err := pool.Add(arcaneResource); err != nil {
		t.Fatalf("Failed to add arcane resource: %v", err)
	}

	// Test dawn trigger - only affects divine
	pool.ProcessRestoration("dawn", bus)
	if divineResource.Current() != 3 {
		t.Errorf("Expected divine power to be full (3) after dawn, got %d", divineResource.Current())
	}
	if arcaneResource.Current() != 1 {
		t.Errorf("Expected arcane surge to remain at 1 after dawn, got %d", arcaneResource.Current())
	}

	// Test custom trigger
	divineResource.SetCurrent(2) // Use one
	pool.ProcessRestoration("prayer_answered", bus)
	if divineResource.Current() != 3 {
		t.Errorf("Expected divine power to be 3 after prayer, got %d", divineResource.Current())
	}

	// Test trigger that affects only one resource
	arcaneResource.SetCurrent(0)
	pool.ProcessRestoration("arcane_rest", bus)
	if arcaneResource.Current() != 2 {
		t.Errorf("Expected arcane surge to be full (2) after arcane rest, got %d", arcaneResource.Current())
	}
}

func TestLegacyCompatibility(t *testing.T) {
	owner := &MockEntity{id: "char-1", typ: "character"}

	// Create resource with legacy configuration
	resource := resources.NewSimpleResource(resources.SimpleResourceConfig{
		ID:               "legacy-resource",
		Type:             resources.ResourceTypeAbilityUse,
		Owner:            owner,
		Key:              core.MustNewRef("legacy", "dnd5e", "resource"),
		Current:          0,
		Maximum:          3,
		RestoreType:      resources.RestoreShortRest,
		ShortRestRestore: -1,
	})

	// Should work with both old methods and new triggers
	if restore := resource.RestoreOnShortRest(); restore != 3 {
		t.Errorf("Expected RestoreOnShortRest to return 3, got %d", restore)
	}

	if restore := resource.RestoreOnTrigger("short_rest"); restore != 3 {
		t.Errorf("Expected RestoreOnTrigger(short_rest) to return 3, got %d", restore)
	}

	if restore := resource.RestoreOnLongRest(); restore != 3 {
		t.Errorf("Expected RestoreOnLongRest to return 3 (short rest resources restore on long too), got %d", restore)
	}
}

func TestMixedConfiguration(t *testing.T) {
	owner := &MockEntity{id: "char-1", typ: "character"}

	// Create resource with both legacy and trigger configuration
	// Triggers should take precedence
	resource := resources.NewSimpleResource(resources.SimpleResourceConfig{
		ID:              "mixed-resource",
		Type:            resources.ResourceTypeCustom,
		Owner:           owner,
		Key:             core.MustNewRef("mixed", "dnd5e", "resource"),
		Current:         0,
		Maximum:         5,
		RestoreType:     resources.RestoreLongRest,
		LongRestRestore: 2, // Legacy says restore 2
		RestoreTriggers: map[string]int{
			"long_rest": 3, // Trigger says restore 3
			"dawn":      1,
		},
	})

	// Trigger value should override legacy value
	if restore := resource.RestoreOnTrigger("long_rest"); restore != 3 {
		t.Errorf("Expected trigger to override legacy, got %d instead of 3", restore)
	}

	// Additional triggers work
	if restore := resource.RestoreOnTrigger("dawn"); restore != 1 {
		t.Errorf("Expected dawn trigger to restore 1, got %d", restore)
	}
}
