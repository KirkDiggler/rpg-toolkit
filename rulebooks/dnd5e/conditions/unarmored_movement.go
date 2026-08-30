// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"context"
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// UnarmoredMovementData is the JSON structure for persisting unarmored movement condition state
type UnarmoredMovementData struct {
	Ref       *core.Ref `json:"ref"`
	MemberID  string    `json:"member_id"`
	MonkLevel int       `json:"monk_level"`
}

// UnarmoredMovementCondition represents the Monk's Unarmored Movement feature.
// Grants a speed bonus when not wearing armor or using a shield.
// The bonus scales with monk level:
// - Level 2-5: +10 ft
// - Level 6-9: +15 ft
// - Level 10-13: +20 ft
// - Level 14-17: +25 ft
// - Level 18+: +30 ft
type UnarmoredMovementCondition struct {
	MemberID  string
	MonkLevel int
	bus       events.EventBus
}

// Ensure UnarmoredMovementCondition implements dnd5eEvents.ConditionBehavior
var _ dnd5eEvents.ConditionBehavior = (*UnarmoredMovementCondition)(nil)

// Ref returns the canonical ref this condition names itself by — the same ref
// its ToJSON embeds and its loader routes on.
func (u *UnarmoredMovementCondition) Ref() *core.Ref { return refs.Conditions.UnarmoredMovement() }

// UnarmoredMovementInput provides configuration for creating an unarmored movement condition
type UnarmoredMovementInput struct {
	MemberID  string // ID of the character
	MonkLevel int    // Monk level determines speed bonus
}

// NewUnarmoredMovementCondition creates an unarmored movement condition from input
func NewUnarmoredMovementCondition(input UnarmoredMovementInput) *UnarmoredMovementCondition {
	return &UnarmoredMovementCondition{
		MemberID:  input.MemberID,
		MonkLevel: input.MonkLevel,
	}
}

// IsApplied returns true if this condition is currently applied
func (u *UnarmoredMovementCondition) IsApplied() bool {
	return u.bus != nil
}

// Apply registers this condition with the event bus.
// Unarmored Movement is a passive feature that doesn't subscribe to events,
// but we store the bus reference for consistency with the interface.
func (u *UnarmoredMovementCondition) Apply(_ context.Context, bus events.EventBus) error {
	u.bus = bus
	return nil
}

// Remove unregisters this condition from the event bus.
func (u *UnarmoredMovementCondition) Remove(_ context.Context, _ events.EventBus) error {
	u.bus = nil
	return nil
}

// ToJSON converts the condition to JSON for persistence
func (u *UnarmoredMovementCondition) ToJSON() (json.RawMessage, error) {
	data := UnarmoredMovementData{
		Ref:       refs.Conditions.UnarmoredMovement(),
		MemberID:  u.MemberID,
		MonkLevel: u.MonkLevel,
	}
	return json.Marshal(data)
}

// loadJSON loads unarmored movement condition state from JSON
//
//nolint:unused // Used by loader.go
func (u *UnarmoredMovementCondition) loadJSON(data json.RawMessage) error {
	var umData UnarmoredMovementData
	if err := json.Unmarshal(data, &umData); err != nil {
		return rpgerr.Wrap(err, "failed to unmarshal unarmored movement data")
	}

	u.MemberID = umData.MemberID
	u.MonkLevel = umData.MonkLevel

	return nil
}

// SpeedBonus returns the speed bonus granted by this condition, and whether
// the question could be answered at all.
//
// Zero and NOT known is "nobody could tell me whose sheet this is"; zero and
// known is "this monk is carrying a shield, so the bonus does not apply". A
// caller that reads only the number cannot tell a rule from missing data, which
// is why the bool is here rather than an error the caller would have to swallow.
//
// The bonus is based on monk level:
// - Level 2-5: +10 ft
// - Level 6-9: +15 ft
// - Level 10-13: +20 ft
// - Level 14-17: +25 ft
// - Level 18+: +30 ft
func (u *UnarmoredMovementCondition) SpeedBonus(ctx context.Context) (bonus int, known bool) {
	unarmored, known := u.isUnarmored(ctx)
	if !known {
		return 0, false
	}
	if !unarmored {
		return 0, true
	}

	return u.calculateSpeedBonus(), true
}

// isUnarmored reports whether the character is not using a shield, and whether
// that could be answered at all.
//
// The second return is the whole reason this is not a plain bool: "wearing a
// shield" and "nobody could name this monk in the cast" call for different
// answers, and collapsing them would silently deny a monk their speed on
// missing data rather than on a rule.
//
// The shield question is asked of the member surface, which every combatant
// answers — a monster answers false, because its shield is baked into the stat
// block AC it already reports and there is nothing further for a rule to add.
//
// TODO(rpg-toolkit): armor is not checked, only shields. That predates this
// change — the registry this replaced could not see armor either, and said so.
// The member surface can grow the question the day a rule needs it.
func (u *UnarmoredMovementCondition) isUnarmored(ctx context.Context) (unarmored, known bool) {
	me, ok := member(ctx, u.MemberID)
	if !ok {
		return false, false
	}

	return !me.HasShieldEquipped(), true
}

// calculateSpeedBonus returns the speed bonus based on monk level
func (u *UnarmoredMovementCondition) calculateSpeedBonus() int {
	switch {
	case u.MonkLevel >= 18:
		return 30
	case u.MonkLevel >= 14:
		return 25
	case u.MonkLevel >= 10:
		return 20
	case u.MonkLevel >= 6:
		return 15
	default:
		// Level 2-5
		return 10
	}
}
