// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// MartialArtsBonusStrike represents an unarmed strike granted by Martial Arts.
// When a monk takes the Attack action with an unarmed strike or monk weapon,
// they can make one unarmed strike as a bonus action.
// It is a temporary action that removes itself when used or at turn end.
type MartialArtsBonusStrike struct {
	id             string
	ownerID        string
	bus            events.EventBus
	subscriptionID string
	removed        bool
	used           bool
}

// MartialArtsBonusStrikeConfig contains configuration for creating a MartialArtsBonusStrike action
type MartialArtsBonusStrikeConfig struct {
	ID      string
	OwnerID string
}

// NewMartialArtsBonusStrike creates a new MartialArtsBonusStrike action.
func NewMartialArtsBonusStrike(config MartialArtsBonusStrikeConfig) *MartialArtsBonusStrike {
	return &MartialArtsBonusStrike{
		id:      config.ID,
		ownerID: config.OwnerID,
	}
}

// GetID implements core.Entity
func (m *MartialArtsBonusStrike) GetID() string {
	return m.id
}

// GetType implements core.Entity
func (m *MartialArtsBonusStrike) GetType() core.EntityType {
	return EntityTypeAction
}

// CanActivate implements core.Action[ActionInput]
func (m *MartialArtsBonusStrike) CanActivate(_ context.Context, _ core.Entity, input ActionInput) error {
	if m.removed {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "martial arts bonus strike has been removed")
	}

	if m.used {
		return rpgerr.New(rpgerr.CodeResourceExhausted, "martial arts bonus strike already used")
	}

	if input.ActionEconomy == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "action economy required")
	}

	if !input.ActionEconomy.CanUseBonusAction() {
		return rpgerr.New(rpgerr.CodeResourceExhausted, "no bonus action available")
	}

	if input.Target == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "martial arts bonus strike requires a target")
	}

	return nil
}

// Activate implements core.Action[ActionInput]
func (m *MartialArtsBonusStrike) Activate(ctx context.Context, owner core.Entity, input ActionInput) error {
	if err := m.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Consume bonus action from ActionEconomy
	if err := input.ActionEconomy.UseBonusAction(); err != nil {
		return rpgerr.Wrapf(err, "failed to use bonus action for martial arts strike")
	}

	m.used = true

	// Publish the strike request event for the game server to resolve
	// Using FlurryStrikeRequestedTopic since it's the same type of unarmed strike
	if input.Bus != nil {
		topic := dnd5eEvents.FlurryStrikeRequestedTopic.On(input.Bus)
		err := topic.Publish(ctx, dnd5eEvents.FlurryStrikeRequestedEvent{
			AttackerID: owner.GetID(),
			TargetID:   input.Target.GetID(),
			ActionID:   m.id,
		})
		if err != nil {
			return rpgerr.Wrapf(err, "failed to publish martial arts strike event")
		}
	}

	// Remove self after use
	if m.bus != nil {
		if err := m.Remove(ctx, m.bus); err != nil {
			return rpgerr.Wrapf(err, "failed to remove martial arts bonus strike after use")
		}
	}

	return nil
}

// Apply subscribes to turn end events for automatic cleanup
func (m *MartialArtsBonusStrike) Apply(ctx context.Context, bus events.EventBus) error {
	if m.bus != nil {
		return rpgerr.New(rpgerr.CodeAlreadyExists, "martial arts bonus strike already applied")
	}

	m.bus = bus

	// Subscribe to turn end for cleanup
	turnEndTopic := dnd5eEvents.TurnEndTopic.On(bus)
	subID, err := turnEndTopic.Subscribe(ctx, m.onTurnEnd)
	if err != nil {
		m.bus = nil
		return rpgerr.Wrapf(err, "failed to subscribe to turn end")
	}
	m.subscriptionID = subID

	return nil
}

// Remove unsubscribes from events and marks as removed
func (m *MartialArtsBonusStrike) Remove(ctx context.Context, bus events.EventBus) error {
	if m.removed {
		return nil // Already removed
	}

	m.removed = true

	if m.subscriptionID != "" {
		if err := bus.Unsubscribe(ctx, m.subscriptionID); err != nil {
			return rpgerr.Wrapf(err, "failed to unsubscribe from turn end")
		}
		m.subscriptionID = ""
	}

	// Publish action removed event so the character can remove it from their list
	removedTopic := dnd5eEvents.ActionRemovedTopic.On(bus)
	err := removedTopic.Publish(ctx, dnd5eEvents.ActionRemovedEvent{
		ActionID: m.id,
		OwnerID:  m.ownerID,
	})
	if err != nil {
		return rpgerr.Wrapf(err, "failed to publish action removed event")
	}

	return nil
}

// onTurnEnd is called when the turn ends - removes unused strike
func (m *MartialArtsBonusStrike) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) error {
	// Only remove if this is the owner's turn ending
	if event.CharacterID != m.ownerID {
		return nil
	}

	// Remove self at end of turn
	if !m.removed && m.bus != nil {
		return m.Remove(ctx, m.bus)
	}

	return nil
}

// IsTemporary returns true - martial arts bonus strikes are always temporary
func (m *MartialArtsBonusStrike) IsTemporary() bool {
	return true
}

// UsesRemaining returns 1 if not used, 0 if used
func (m *MartialArtsBonusStrike) UsesRemaining() int {
	if m.used || m.removed {
		return 0
	}
	return 1
}

// ToJSON converts the action to JSON for persistence.
func (m *MartialArtsBonusStrike) ToJSON() (json.RawMessage, error) {
	data := map[string]interface{}{
		"id":       m.id,
		"owner_id": m.ownerID,
		"type":     "martial_arts_bonus_strike",
		"used":     m.used,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal martial arts bonus strike: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost (bonus action)
func (m *MartialArtsBonusStrike) ActionType() coreCombat.ActionType {
	return coreCombat.ActionBonus
}

// CapacityType returns that MartialArtsBonusStrike has no additional capacity requirement.
// The bonus action consumption is already tracked via ActionEconomy.UseBonusAction().
func (m *MartialArtsBonusStrike) CapacityType() combat.CapacityType {
	return combat.CapacityNone
}

// Compile-time check that MartialArtsBonusStrike implements Action
var _ Action = (*MartialArtsBonusStrike)(nil)
