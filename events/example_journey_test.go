// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events_test

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/events"
)

// ExampleJourney_AttackFlow demonstrates how an attack event journeys through
// the entire combat system, accumulating modifiers from multiple features.
//
// THE MAGIC: Watch the '.On(bus)' pattern connect each feature to the journey.
func Example_journey_attackFlow() {
	bus := events.NewEventBus()
	ctx := context.Background()

	// Define our journey stages - the order matters!
	stages := []chain.Stage{
		StageBase,       // Base calculations
		StageFeatures,   // Class features apply
		StageConditions, // Status effects apply
		StageEquipment,  // Gear bonuses apply
		StageFinal,      // Last-minute adjustments
	}

	// Feature 1: Rage - adds damage when active
	rage := &RageFeature{ownerID: "barbarian", bonus: 2}
	_ = rage.Apply(bus)

	// Feature 2: Bless - adds to attack and damage
	bless := &BlessSpell{targets: []string{"barbarian"}, bonus: 4}
	_ = bless.Apply(bus)

	// Feature 3: Magic Weapon - enhances weapon damage
	magicWeapon := &MagicWeaponFeature{ownerID: "barbarian", bonus: 1}
	_ = magicWeapon.Apply(bus)

	// Now watch the journey unfold...

	// 1. Create the attack event - just data!
	attack := AttackEvent{
		AttackerID: "barbarian",
		TargetID:   "goblin",
		Damage:     10, // Base damage
	}

	// 2. Create a chain to collect modifiers
	attackChain := events.NewStagedChain[AttackEvent](stages)

	// 3. THE MAGIC MOMENT - connect to the bus
	attacks := AttackChain.On(bus)

	// 4. Publish the event - it journeys through all features
	modifiedChain, _ := attacks.PublishWithChain(ctx, attack, attackChain)

	// 5. Execute the accumulated modifications
	result, _ := modifiedChain.Execute(ctx, attack)

	fmt.Printf("Attack Journey:\n")
	fmt.Printf("  Base damage: %d\n", attack.Damage)
	fmt.Printf("  After rage (+2): %d\n", attack.Damage+2)
	fmt.Printf("  After bless (+4): %d\n", attack.Damage+2+4)
	fmt.Printf("  After magic weapon (+1): %d\n", attack.Damage+2+4+1)
	fmt.Printf("  Final damage: %d\n", result.Damage)

	// Output:
	// Attack Journey:
	//   Base damage: 10
	//   After rage (+2): 12
	//   After bless (+4): 16
	//   After magic weapon (+1): 17
	//   Final damage: 17
}

// ExampleJourney_SaveFlow shows how a saving throw accumulates modifiers
// from different sources as it flows through the system.
func Example_journey_saveFlow() {
	bus := events.NewEventBus()
	ctx := context.Background()

	// Topic definition
	SaveChain := events.DefineChainedTopic[SaveEvent]("combat.save")

	// Feature: Resistance gives advantage (we'll simulate with +5)
	resistance := &ResistanceFeature{targetID: "hero", bonus: 5}
	saves := SaveChain.On(bus) // Connect to journey
	_, _ = saves.SubscribeWithChain(ctx, resistance.ModifySave)

	// Feature: Bane spell imposes penalty
	bane := &BaneSpell{targets: []string{"hero"}, penalty: -2}
	_, _ = saves.SubscribeWithChain(ctx, bane.ModifySave)

	// Create save event
	save := SaveEvent{
		TargetID: "hero",
		SaveType: "wisdom",
		DC:       15,
		BaseRoll: 10,
	}

	// Journey through modifiers
	stages := []chain.Stage{StageBase, StageFeatures, StageConditions}
	saveChain := events.NewStagedChain[SaveEvent](stages)
	modifiedChain, _ := saves.PublishWithChain(ctx, save, saveChain)
	result, _ := modifiedChain.Execute(ctx, save)

	fmt.Printf("Save Journey:\n")
	fmt.Printf("  DC: %d\n", save.DC)
	fmt.Printf("  Base roll: %d\n", save.BaseRoll)
	fmt.Printf("  With resistance (+5): %d\n", save.BaseRoll+5)
	fmt.Printf("  With bane (-2): %d\n", save.BaseRoll+5-2)
	fmt.Printf("  Final roll: %d\n", result.TotalRoll)
	fmt.Printf("  Success: %v\n", result.TotalRoll >= save.DC)

	// Output:
	// Save Journey:
	//   DC: 15
	//   Base roll: 10
	//   With resistance (+5): 15
	//   With bane (-2): 13
	//   Final roll: 13
	//   Success: false
}

// ExampleJourney_DamageReduction shows how damage flows through
// resistances and vulnerabilities.
func Example_journey_damageReduction() {
	bus := events.NewEventBus()
	ctx := context.Background()

	// Topic definition
	DamageChain := events.DefineChainedTopic[DamageEvent]("combat.damage")

	// Feature: Fire Resistance
	fireResist := &ResistanceFeature{targetID: "dragon", damageType: "fire"}
	damages := DamageChain.On(bus) // THE MAGIC CONNECTION
	_, _ = damages.SubscribeWithChain(ctx, fireResist.ModifyDamage)

	// Feature: Stoneskin spell
	stoneskin := &StoneskinSpell{targetID: "dragon", reduction: 3}
	_, _ = damages.SubscribeWithChain(ctx, stoneskin.ModifyDamage)

	// Create damage events
	fireDamage := DamageEvent{
		SourceID:   "wizard",
		TargetID:   "dragon",
		Amount:     20,
		DamageType: "fire",
	}

	slashDamage := DamageEvent{
		SourceID:   "fighter",
		TargetID:   "dragon",
		Amount:     15,
		DamageType: "slashing",
	}

	stages := []chain.Stage{StageBase, StageConditions, StageFinal}

	// Fire damage journey
	fireChain := events.NewStagedChain[DamageEvent](stages)
	modifiedChain, _ := damages.PublishWithChain(ctx, fireDamage, fireChain)
	fireResult, _ := modifiedChain.Execute(ctx, fireDamage)

	// Slashing damage journey
	slashChain := events.NewStagedChain[DamageEvent](stages)
	modifiedChain2, _ := damages.PublishWithChain(ctx, slashDamage, slashChain)
	slashResult, _ := modifiedChain2.Execute(ctx, slashDamage)

	fmt.Printf("Damage Journeys:\n")
	fmt.Printf("Fire damage:\n")
	fmt.Printf("  Base: %d\n", fireDamage.Amount)
	fmt.Printf("  After resistance (half): %d\n", fireDamage.Amount/2)
	fmt.Printf("  After stoneskin (-3): %d\n", fireResult.Amount)
	fmt.Printf("\nSlashing damage:\n")
	fmt.Printf("  Base: %d\n", slashDamage.Amount)
	fmt.Printf("  After stoneskin (-3): %d\n", slashResult.Amount)

	// Output:
	// Damage Journeys:
	// Fire damage:
	//   Base: 20
	//   After resistance (half): 10
	//   After stoneskin (-3): 7
	//
	// Slashing damage:
	//   Base: 15
	//   After stoneskin (-3): 12
}

// Supporting types for the journey examples

type SaveEvent struct {
	TargetID  string
	SaveType  string
	DC        int
	BaseRoll  int
	TotalRoll int
}

type ResistanceFeature struct {
	targetID   string
	damageType string
	bonus      int
}

func (r *ResistanceFeature) ModifySave(_ context.Context, e SaveEvent,
	c chain.Chain[SaveEvent]) (chain.Chain[SaveEvent], error) {
	if e.TargetID == r.targetID {
		_ = c.Add(StageFeatures, "resistance", func(_ context.Context, e SaveEvent) (SaveEvent, error) {
			e.TotalRoll = e.BaseRoll + r.bonus
			return e, nil
		})
	}
	return c, nil
}

func (r *ResistanceFeature) ModifyDamage(_ context.Context, e DamageEvent,
	c chain.Chain[DamageEvent]) (chain.Chain[DamageEvent], error) {
	if e.TargetID == r.targetID && e.DamageType == r.damageType {
		_ = c.Add(StageConditions, "resistance-"+r.damageType, func(_ context.Context, e DamageEvent) (DamageEvent, error) {
			e.Amount /= 2 // Resistance halves damage
			return e, nil
		})
	}
	return c, nil
}

type BaneSpell struct {
	targets []string
	penalty int
}

func (b *BaneSpell) ModifySave(_ context.Context, e SaveEvent,
	c chain.Chain[SaveEvent]) (chain.Chain[SaveEvent], error) {
	for _, target := range b.targets {
		if e.TargetID == target {
			_ = c.Add(StageConditions, "bane", func(_ context.Context, e SaveEvent) (SaveEvent, error) {
				e.TotalRoll += b.penalty
				return e, nil
			})
			break
		}
	}
	return c, nil
}

type BlessSpell struct {
	targets []string
	bonus   int
}

func (b *BlessSpell) Apply(bus events.EventBus) error {
	// THE MAGIC: Connect to the bus explicitly
	attacks := AttackChain.On(bus)

	_, err := attacks.SubscribeWithChain(
		context.Background(),
		func(_ context.Context, e AttackEvent, c chain.Chain[AttackEvent]) (chain.Chain[AttackEvent], error) {
			for _, target := range b.targets {
				if e.AttackerID == target {
					_ = c.Add(StageConditions, "bless", func(_ context.Context, e AttackEvent) (AttackEvent, error) {
						e.Damage += b.bonus
						return e, nil
					})
					break
				}
			}
			return c, nil
		},
	)
	return err
}

type MagicWeaponFeature struct {
	ownerID string
	bonus   int
}

func (m *MagicWeaponFeature) Apply(bus events.EventBus) error {
	// THE MAGIC: See the connection!
	attacks := AttackChain.On(bus)

	_, err := attacks.SubscribeWithChain(
		context.Background(),
		func(_ context.Context, e AttackEvent, c chain.Chain[AttackEvent]) (chain.Chain[AttackEvent], error) {
			if e.AttackerID == m.ownerID {
				_ = c.Add(StageEquipment, "magic-weapon", func(_ context.Context, e AttackEvent) (AttackEvent, error) {
					e.Damage += m.bonus
					return e, nil
				})
			}
			return c, nil
		},
	)
	return err
}

type StoneskinSpell struct {
	targetID  string
	reduction int
}

func (s *StoneskinSpell) ModifyDamage(_ context.Context, e DamageEvent,
	c chain.Chain[DamageEvent]) (chain.Chain[DamageEvent], error) {
	if e.TargetID == s.targetID {
		// Stoneskin affects all physical damage in this example
		_ = c.Add(StageFinal, "stoneskin", func(_ context.Context, e DamageEvent) (DamageEvent, error) {
			e.Amount -= s.reduction
			if e.Amount < 0 {
				e.Amount = 0
			}
			return e, nil
		})
	}
	return c, nil
}
