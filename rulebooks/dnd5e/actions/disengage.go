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
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// Disengage represents the D&D 5e Disengage combat ability. When activated,
// the character's movement does not provoke opportunity attacks for the rest
// of the current turn.
//
// Wave 2.11e (#666): full-action variant only. Monk's Step of the Wind
// provides the bonus-action variant via the features package; this action
// is the standard once-per-turn Disengage available to all characters.
//
// Activate consumes one action from the ActionEconomy, applies the
// DisengagingCondition to the owner's encounter bus (so MovementChain
// subscribers add OAPreventionSources during the next move), and publishes
// DisengageActivatedEvent for game-server telemetry. The condition removes
// itself automatically on the owner's TurnEnd.
type Disengage struct {
	id      string
	ownerID string
}

// DisengageConfig contains configuration for creating a Disengage action.
type DisengageConfig struct {
	// ID is the unique action ID for this Disengage instance on the
	// owner's action list.
	ID string

	// OwnerID is the character that owns this action; the DisengagingCondition
	// will be Apply()'d under this ID.
	OwnerID string
}

// NewDisengage creates a new Disengage action.
func NewDisengage(config DisengageConfig) *Disengage {
	return &Disengage{
		id:      config.ID,
		ownerID: config.OwnerID,
	}
}

// GetID implements core.Entity.
func (d *Disengage) GetID() string {
	return d.id
}

// GetType implements core.Entity.
func (d *Disengage) GetType() core.EntityType {
	return EntityTypeAction
}

// CanActivate implements core.Action[ActionInput].
// Disengage can be activated when the owner has an action remaining.
// Disengage requires no target — it affects the owner only.
func (d *Disengage) CanActivate(_ context.Context, _ core.Entity, input ActionInput) error {
	if input.ActionEconomy == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "action economy required")
	}
	if !input.ActionEconomy.CanUseAction() {
		return rpgerr.New(rpgerr.CodeResourceExhausted, "no actions remaining")
	}
	return nil
}

// Activate implements core.Action[ActionInput].
// Consumes one action from the economy, applies the DisengagingCondition to
// the owner on input.Bus, and publishes DisengageActivatedEvent for the
// game server.
//
// Toolkit-as-product framing: the rule application (Apply'ing the condition)
// lives toolkit-side. The orchestrator (rpg-api) only consumes the activation
// event for streaming/UI; it does not need to know about the condition
// mechanism.
func (d *Disengage) Activate(ctx context.Context, owner core.Entity, input ActionInput) error {
	if err := d.CanActivate(ctx, owner, input); err != nil {
		return err
	}

	if err := input.ActionEconomy.UseAction(); err != nil {
		return rpgerr.Wrap(err, "failed to use action")
	}

	// Apply the DisengagingCondition to the owner's bus. The condition
	// subscribes to MovementChain (adds OAPreventionSources when the owner
	// moves) and TurnEndTopic (self-removes when the owner's turn ends).
	if input.Bus != nil {
		condition := conditions.NewDisengagingCondition(d.ownerID)
		if err := condition.Apply(ctx, input.Bus); err != nil {
			return rpgerr.Wrap(err, "failed to apply disengaging condition")
		}

		// Telemetry event for the game server. The condition is already
		// applied; this is the activation signal for stream consumers.
		topic := dnd5eEvents.DisengageActivatedTopic.On(input.Bus)
		if err := topic.Publish(ctx, dnd5eEvents.DisengageActivatedEvent{
			CharacterID: d.ownerID,
		}); err != nil {
			return rpgerr.Wrap(err, "failed to publish disengage activated event")
		}
	}

	return nil
}

// Apply implements Action — Disengage is a permanent action and does not
// need to subscribe to events at character-load time. The DisengagingCondition
// it creates on Activate handles its own subscriptions.
func (d *Disengage) Apply(_ context.Context, _ events.EventBus) error {
	return nil
}

// Remove implements Action — Disengage is a permanent action; nothing to
// unsubscribe at character-unload time.
func (d *Disengage) Remove(_ context.Context, _ events.EventBus) error {
	return nil
}

// IsTemporary returns false — Disengage is always available.
func (d *Disengage) IsTemporary() bool {
	return false
}

// UsesRemaining returns UnlimitedUses — Disengage can be activated whenever
// the owner has an action remaining.
func (d *Disengage) UsesRemaining() int {
	return UnlimitedUses
}

// ToJSON converts the action to JSON for persistence.
func (d *Disengage) ToJSON() (json.RawMessage, error) {
	data := map[string]interface{}{
		"id":       d.id,
		"owner_id": d.ownerID,
		"type":     "disengage",
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal disengage: %w", err)
	}
	return bytes, nil
}

// ActionType returns the action economy cost (1 action).
func (d *Disengage) ActionType() coreCombat.ActionType {
	return coreCombat.ActionStandard
}

// CapacityType returns that Disengage consumes no specialized capacity
// (only the action itself).
func (d *Disengage) CapacityType() combat.CapacityType {
	return combat.CapacityNone
}

// Compile-time check that Disengage implements Action.
var _ Action = (*Disengage)(nil)
