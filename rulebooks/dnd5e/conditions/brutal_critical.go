// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/core/chain"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// diceNotationRegex matches simple dice notation like "1d8", "2d6", etc.
var diceNotationRegex = regexp.MustCompile(`^(\d*)[dD](\d+)`)

// BrutalCriticalData is the JSON structure for persisting brutal critical condition state
type BrutalCriticalData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	Level     int       `json:"level"`
	ExtraDice int       `json:"extra_dice"`
}

// BrutalCriticalCondition represents the barbarian's brutal critical feature.
// It adds extra weapon damage dice on critical hits based on barbarian level.
// It implements the ConditionBehavior interface.
type BrutalCriticalCondition struct {
	MemberID        string
	Level           int
	ExtraDice       int
	subscriptionIDs []string
	bus             events.EventBus
	roller          dice.Roller
}

// Ensure BrutalCriticalCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*BrutalCriticalCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (b *BrutalCriticalCondition) Ref() *core.Ref { return refs.Conditions.BrutalCritical() }

// BrutalCriticalInput provides configuration for creating a brutal critical condition
type BrutalCriticalInput struct {
	MemberID string      // ID of the barbarian
	Level    int         // Barbarian level (determines extra dice)
	Roller   dice.Roller // Dice roller for rolling extra damage
}

// NewBrutalCriticalCondition creates a brutal critical condition from input
func NewBrutalCriticalCondition(input BrutalCriticalInput) *BrutalCriticalCondition {
	return &BrutalCriticalCondition{
		MemberID:  input.MemberID,
		Level:     input.Level,
		ExtraDice: calculateExtraDice(input.Level),
		roller:    input.Roller,
	}
}

// calculateExtraDice determines extra weapon dice based on barbarian level
func calculateExtraDice(level int) int {
	switch {
	case level >= 17:
		return 3
	case level >= 13:
		return 2
	case level >= 9:
		return 1
	default:
		return 0
	}
}

// IsApplied returns true if this condition is currently applied
func (b *BrutalCriticalCondition) IsApplied() bool {
	return b.bus != nil
}

// Apply subscribes this condition to relevant combat events
func (b *BrutalCriticalCondition) Apply(ctx context.Context, bus events.EventBus) error {
	b.bus = bus

	// Subscribe to damage chain to add extra dice on crits
	damageChain := dnd5eEvents.DamageChain.On(bus)
	subID, err := damageChain.SubscribeWithChain(ctx, b.onDamageChain)
	if err != nil {
		return rpgerr.Wrap(err, "failed to subscribe to damage chain")
	}
	b.subscriptionIDs = append(b.subscriptionIDs, subID)

	return nil
}

// Remove unsubscribes this condition from events
func (b *BrutalCriticalCondition) Remove(ctx context.Context, bus events.EventBus) error {
	if b.bus == nil {
		return nil // Not applied, nothing to remove
	}

	total := len(b.subscriptionIDs)
	var errs []error
	for _, subID := range b.subscriptionIDs {
		if err := bus.Unsubscribe(ctx, subID); err != nil {
			errs = append(errs, fmt.Errorf("unsubscribe %s: %w", subID, err))
		}
	}

	b.subscriptionIDs = nil
	b.bus = nil

	if len(errs) > 0 {
		return fmt.Errorf("failed to unsubscribe %d/%d subscriptions: %w", len(errs), total, errors.Join(errs...))
	}
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (b *BrutalCriticalCondition) ToJSON() (json.RawMessage, error) {
	data := BrutalCriticalData{
		Ref:       refs.Conditions.BrutalCritical(),
		MemberID:  b.MemberID,
		Level:     b.Level,
		ExtraDice: b.ExtraDice,
	}
	return json.Marshal(data)
}

// loadJSON loads brutal critical condition state from JSON
func (b *BrutalCriticalCondition) loadJSON(data json.RawMessage) error {
	var bcData BrutalCriticalData
	if err := json.Unmarshal(data, &bcData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal brutal critical data")
	}

	b.MemberID = bcData.MemberID
	b.Level = bcData.Level
	b.ExtraDice = bcData.ExtraDice

	return nil
}

// onDamageChain adds extra weapon damage dice on critical hits
func (b *BrutalCriticalCondition) onDamageChain(
	_ context.Context,
	event *dnd5eEvents.DamageChainEvent,
	c chain.Chain[*dnd5eEvents.DamageChainEvent],
) (chain.Chain[*dnd5eEvents.DamageChainEvent], error) {
	// Only add extra dice if:
	// 1. We're the attacker
	// 2. This is a critical hit
	// 3. We have extra dice to add (level 9+)
	if event.AttackerID != b.MemberID || !event.IsCritical || b.ExtraDice == 0 {
		return c, nil
	}

	// Parse marked weapon damage notation to get die size (e.g., "1d8" -> 8)
	dieSize, err := parseDieSize(event.WeaponDamageDice)
	if err != nil {
		return c, rpgerr.Wrapf(err, "failed to parse weapon damage notation: %s", event.WeaponDamageDice)
	}

	if dieSize == 0 {
		return c, nil // No dice to roll (shouldn't happen with valid weapons)
	}

	// Add brutal critical modifier at StageFeatures
	modifyDamage := func(modCtx context.Context, e *dnd5eEvents.DamageChainEvent) (*dnd5eEvents.DamageChainEvent, error) {
		// Roll extra dice
		roller := b.roller
		if roller == nil {
			roller = dice.NewRoller()
		}

		extraRolls, rollErr := roller.RollN(modCtx, b.ExtraDice, dieSize)
		if rollErr != nil {
			return e, rpgerr.Wrap(rollErr, "failed to roll brutal critical dice")
		}

		subtotal := 0
		for _, face := range extraRolls {
			subtotal += face
		}

		// Append brutal critical damage component. The trace records the dice
		// actually rolled — a critical's extra dice — so the notation describes
		// the physical pool, not the weapon's printed expression.
		e.Components = append(e.Components, dnd5eEvents.DamageComponent{
			Source: dnd5eEvents.DamageSourceFeature,
			Roll: dnd5eEvents.RollComponent{
				Source: dnd5eEvents.RollSource{
					Ref:  refs.Features.BrutalCritical(),
					Name: "Brutal Critical",
				},
				Dice: &dnd5eEvents.DiceTrace{
					Notation:      dice.SimplePool(len(extraRolls), dieSize, 0).Notation(),
					DieSize:       dieSize,
					OriginalRolls: extraRolls,
					FinalRolls:    slices.Clone(extraRolls),
					Subtotal:      subtotal,
				},
			},
			DamageType: e.WeaponDamageType,
			IsCritical: false,
		})
		return e, nil
	}

	err = c.Add(combat.StageFeatures, "brutal_critical", modifyDamage)
	if err != nil {
		return c, rpgerr.Wrapf(err, "failed to add brutal critical modifier for character %s", b.MemberID)
	}

	return c, nil
}

// parseDieSize extracts the die size from a dice notation string (e.g., "1d8" -> 8)
func parseDieSize(notation string) (int, error) {
	matches := diceNotationRegex.FindStringSubmatch(notation)
	if len(matches) < 3 {
		return 0, rpgerr.Newf(rpgerr.CodeInvalidArgument, "invalid dice notation: %s", notation)
	}

	dieSize, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, rpgerr.Wrapf(err, "invalid die size in notation: %s", notation)
	}

	return dieSize, nil
}
