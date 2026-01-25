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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Strike represents a standard weapon attack using one of the attacks granted
// by the Attack ability. It consumes one AttacksRemaining from the action economy.
// Unlike FlurryStrike or OffHandStrike, Strike is a permanent action that is
// always available when the character has attacks remaining.
type Strike struct {
	id       string
	ownerID  string
	weaponID weapons.WeaponID
}

// StrikeConfig contains configuration for creating a Strike action
type StrikeConfig struct {
	ID       string
	OwnerID  string
	WeaponID weapons.WeaponID
}

// NewStrike creates a new Strike action
func NewStrike(config StrikeConfig) *Strike {
	return &Strike{
		id:       config.ID,
		ownerID:  config.OwnerID,
		weaponID: config.WeaponID,
	}
}

// GetID implements core.Entity
func (s *Strike) GetID() string {
	return s.id
}

// GetType implements core.Entity
func (s *Strike) GetType() core.EntityType {
	return EntityTypeAction
}

// GetWeaponID returns the weapon ID for this strike
func (s *Strike) GetWeaponID() weapons.WeaponID {
	return s.weaponID
}

// CanActivate implements core.Action[ActionInput]
// Strike can be activated when there are attacks remaining in the action economy.
func (s *Strike) CanActivate(_ context.Context, _ core.Entity, input ActionInput) error {
	if input.ActionEconomy == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "action economy required")
	}

	if !input.ActionEconomy.CanUseAttack() {
		return rpgerr.New(rpgerr.CodeResourceExhausted, "no attacks remaining")
	}

	if input.Target == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "strike requires a target")
	}

	return nil
}

// Activate implements core.Action[ActionInput]
// Strike consumes one attack and publishes a StrikeExecutedEvent for the game
// layer to resolve the actual attack roll and damage.
func (s *Strike) Activate(ctx context.Context, owner core.Entity, input ActionInput) error {
	if err := s.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	// Consume the attack
	if err := input.ActionEconomy.UseAttack(); err != nil {
		return rpgerr.Wrapf(err, "failed to use attack")
	}

	// Publish the strike executed event for the game server to resolve
	if input.Bus != nil {
		topic := dnd5eEvents.StrikeExecutedTopic.On(input.Bus)
		err := topic.Publish(ctx, dnd5eEvents.StrikeExecutedEvent{
			AttackerID: owner.GetID(),
			TargetID:   input.Target.GetID(),
			WeaponID:   string(s.weaponID),
			ActionID:   s.id,
		})
		if err != nil {
			return rpgerr.Wrapf(err, "failed to publish strike executed event")
		}
	}

	return nil
}

// Apply implements Action - Strike is a permanent action and does not need
// to subscribe to any events.
func (s *Strike) Apply(_ context.Context, _ events.EventBus) error {
	// Strike is permanent and doesn't need event subscriptions
	return nil
}

// Remove implements Action - Strike is a permanent action and does not need
// to unsubscribe from any events.
func (s *Strike) Remove(_ context.Context, _ events.EventBus) error {
	// Strike is permanent and doesn't need cleanup
	return nil
}

// IsTemporary returns false - Strike is a permanent action
func (s *Strike) IsTemporary() bool {
	return false
}

// UsesRemaining returns UnlimitedUses - Strike can be used as long as attacks remain
func (s *Strike) UsesRemaining() int {
	return UnlimitedUses
}

// ToJSON converts the action to JSON for persistence
func (s *Strike) ToJSON() (json.RawMessage, error) {
	data := map[string]interface{}{
		"id":        s.id,
		"owner_id":  s.ownerID,
		"weapon_id": s.weaponID,
		"type":      "strike",
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal strike: %w", err)
	}

	return bytes, nil
}

// ActionType returns the action economy cost (free - uses come from Attack ability)
func (s *Strike) ActionType() coreCombat.ActionType {
	return coreCombat.ActionFree
}

// CapacityType returns that Strike consumes attack capacity
func (s *Strike) CapacityType() combat.CapacityType {
	return combat.CapacityAttack
}

// Compile-time check that Strike implements Action
var _ Action = (*Strike)(nil)
