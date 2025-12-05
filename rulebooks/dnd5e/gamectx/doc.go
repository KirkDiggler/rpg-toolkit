// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package gamectx provides queryable game state through context.Context for event processing.
//
// THE PROBLEM: Conditions and features need to make decisions based on current
// game state (equipment, spatial positioning, etc.), but events shouldn't carry
// all possible data that might ever be needed. This creates a design tension:
// either bloat events with rarely-needed data, or give conditions no way to
// make intelligent decisions.
//
// THE SOLUTION: GameContext wraps context.Context with access to loaded game state.
// Events remain lightweight, but conditions/features can query whatever state they
// need during processing.
//
// Example - Dueling Fighting Style:
//
// The D&D 5e Dueling fighting style grants +2 damage when wielding a melee weapon
// in one hand with nothing in the other hand (a shield is allowed). This requires
// checking the character's equipped weapons during damage calculation.
//
// Without gamectx:
//
//	// ❌ BAD - Bloat every attack event with equipment data
//	type AttackEvent struct {
//	    AttackerID     string
//	    Damage         dice.Dice
//	    MainHandWeapon *Weapon      // Most conditions won't need this
//	    OffHandWeapon  *Weapon       // Most conditions won't need this
//	    AllEquipment   []Item        // Most conditions won't need this
//	    // ... dozens of other rarely-needed fields
//	}
//
// With gamectx:
//
//	// ✅ GOOD - Event remains focused, condition queries what it needs
//	type AttackEvent struct {
//	    AttackerID string
//	    Damage     dice.Dice
//	}
//
//	// In the Dueling condition's OnApply:
//	func (d *Dueling) OnApply(ctx context.Context, event events.Event) error {
//	    registry, ok := gamectx.Characters(ctx)
//	    if !ok {
//	        return nil  // No game context available, skip bonus
//	    }
//
//	    character := registry.GetCharacter(attackEvent.AttackerID)
//	    weapons := character.(*gamectx.CharacterWeapons)
//
//	    // Check if character meets Dueling requirements
//	    mainHand := weapons.MainHand()
//	    if mainHand != nil && mainHand.IsMelee && !mainHand.IsTwoHanded {
//	        if weapons.OffHand() == nil {  // No weapon in off-hand (shield is OK)
//	            attackEvent.Damage.Add(dice.Constant(2))  // Grant +2 damage
//	        }
//	    }
//	    return nil
//	}
//
// Usage Pattern:
//
//	// 1. Game server creates GameContext with loaded state
//	registry := gamectx.NewBasicCharacterRegistry()
//
//	// 2. Populate registry with character equipment
//	weapons := gamectx.NewCharacterWeapons(
//	    &gamectx.EquippedWeapon{
//	        ID:          "longsword-1",
//	        Name:        "Longsword",
//	        Slot:        "main_hand",
//	        IsMelee:     true,
//	        IsTwoHanded: false,
//	    },
//	    nil,  // No off-hand weapon
//	)
//	registry.Add("hero-1", weapons)
//
//	// 3. Wrap context with GameContext
//	gameCtx := gamectx.NewGameContext(gamectx.GameContextConfig{
//	    CharacterRegistry: registry,
//	})
//	ctx = gamectx.WithGameContext(ctx, gameCtx)
//
//	// 4. Process events with enriched context
//	eventBus.Publish(ctx, "attack.executed", attackEvent)
//
//	// 5. Conditions query state during event processing
//	// (shown in Dueling example above)
//
// Scope:
//   - Context wrapping for game state access
//   - CharacterRegistry interface for querying character data
//   - BasicCharacterRegistry implementation for weapon queries
//   - EquippedWeapon struct for weapon information
//   - CharacterWeapons helper for main/off-hand weapon queries
//
// Non-Goals:
//   - Event definitions: Events remain in their respective modules
//   - Game state storage: Game servers manage their own state
//   - Condition implementations: Conditions remain in rulebooks
//   - Serialization: GameContext is ephemeral per request
//   - Validation: Game servers validate state before creating GameContext
//
// Integration:
// This package integrates with:
//   - events: Provides context enrichment for event processing
//   - conditions: Enables state-aware condition evaluation
//   - features: Enables state-aware feature calculations
//
// Future Extensions:
// As more conditions require game state, GameContext can grow to include:
//   - SpatialRegistry: Query entity positions for range-dependent effects
//   - EffectsRegistry: Query active effects for stacking/interaction logic
//   - ResourceRegistry: Query spell slots, abilities for availability checks
//
// The pattern scales: add new registries as interfaces on GameContext,
// conditions opt-in by checking if the registry is available.
package gamectx
