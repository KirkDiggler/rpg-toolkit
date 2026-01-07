package combatabilities

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// BaseCombatAbility provides common functionality for combat abilities.
// Concrete abilities (Attack, Dash, Dodge, etc.) embed this and override
// the activation behavior.
type BaseCombatAbility struct {
	id          string
	name        string
	description string
	actionType  coreCombat.ActionType
	ref         *core.Ref
}

// BaseCombatAbilityConfig contains configuration for creating a BaseCombatAbility
type BaseCombatAbilityConfig struct {
	// ID is the unique identifier for this ability instance
	ID string
	// Name is the display name of the ability
	Name string
	// Description is a brief description of what the ability does
	Description string
	// ActionType is the action economy cost (action, bonus action, reaction, free)
	ActionType coreCombat.ActionType
	// Ref is the reference to this ability type
	Ref *core.Ref
}

// NewBaseCombatAbility creates a new BaseCombatAbility with the given configuration.
// This is typically used by concrete ability constructors.
func NewBaseCombatAbility(config BaseCombatAbilityConfig) *BaseCombatAbility {
	return &BaseCombatAbility{
		id:          config.ID,
		name:        config.Name,
		description: config.Description,
		actionType:  config.ActionType,
		ref:         config.Ref,
	}
}

// GetID implements core.Entity
func (b *BaseCombatAbility) GetID() string {
	return b.id
}

// GetType implements core.Entity
func (b *BaseCombatAbility) GetType() core.EntityType {
	return EntityTypeCombatAbility
}

// Name returns the display name of this ability
func (b *BaseCombatAbility) Name() string {
	return b.name
}

// Description returns a brief description of what this ability does
func (b *BaseCombatAbility) Description() string {
	return b.description
}

// ActionType returns the action economy cost to use this ability
func (b *BaseCombatAbility) ActionType() coreCombat.ActionType {
	return b.actionType
}

// Ref returns the reference to this ability type
func (b *BaseCombatAbility) Ref() *core.Ref {
	return b.ref
}

// CanActivate checks if the ability can be activated based on action economy.
// Concrete abilities should call this first, then add their own validation.
func (b *BaseCombatAbility) CanActivate(_ context.Context, _ core.Entity, input CombatAbilityInput) error {
	if input.ActionEconomy == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "action economy required")
	}

	// Check action economy based on action type
	switch b.actionType {
	case coreCombat.ActionStandard:
		if !input.ActionEconomy.CanUseAction() {
			return rpgerr.ResourceExhausted("action")
		}
	case coreCombat.ActionBonus:
		if !input.ActionEconomy.CanUseBonusAction() {
			return rpgerr.ResourceExhausted("bonus action")
		}
	case coreCombat.ActionReaction:
		if !input.ActionEconomy.CanUseReaction() {
			return rpgerr.ResourceExhausted("reaction")
		}
	case coreCombat.ActionFree:
		// Free actions don't consume resources
	}

	return nil
}

// Activate consumes the action economy resource.
// Concrete abilities should call this first, then implement their specific effects.
func (b *BaseCombatAbility) Activate(_ context.Context, _ core.Entity, input CombatAbilityInput) error {
	if input.ActionEconomy == nil {
		return rpgerr.New(rpgerr.CodeInvalidArgument, "action economy required")
	}

	// Consume action economy based on action type
	switch b.actionType {
	case coreCombat.ActionStandard:
		if err := input.ActionEconomy.UseAction(); err != nil {
			return err
		}
	case coreCombat.ActionBonus:
		if err := input.ActionEconomy.UseBonusAction(); err != nil {
			return err
		}
	case coreCombat.ActionReaction:
		if err := input.ActionEconomy.UseReaction(); err != nil {
			return err
		}
	case coreCombat.ActionFree:
		// Free actions don't consume resources
	}

	return nil
}

// Apply is a no-op for the base implementation.
// Concrete abilities override if they need event subscriptions.
func (b *BaseCombatAbility) Apply(_ context.Context, _ events.EventBus) error {
	return nil
}

// Remove is a no-op for the base implementation.
// Concrete abilities override if they have subscriptions to clean up.
func (b *BaseCombatAbility) Remove(_ context.Context, _ events.EventBus) error {
	return nil
}

// BaseCombatAbilityData is the JSON structure for persisting combat ability state
type BaseCombatAbilityData struct {
	Ref         *core.Ref             `json:"ref"`
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	ActionType  coreCombat.ActionType `json:"action_type"`
}

// ToJSON converts the ability to JSON for persistence
func (b *BaseCombatAbility) ToJSON() (json.RawMessage, error) {
	data := BaseCombatAbilityData{
		Ref:         b.ref,
		ID:          b.id,
		Name:        b.name,
		Description: b.description,
		ActionType:  b.actionType,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal combat ability data: %w", err)
	}

	return bytes, nil
}

// LoadJSON loads a combat ability from JSON based on its ref.
// Routes to the appropriate ability type based on the ref ID.
func LoadJSON(data json.RawMessage) (CombatAbility, error) {
	// First, extract the ref to determine ability type
	var metadata struct {
		Ref core.Ref `json:"ref"`
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to extract combat ability ref: %w", err)
	}

	// Route based on Ref ID
	switch metadata.Ref.ID {
	case refs.CombatAbilities.Attack().ID:
		attack := &Attack{}
		if err := attack.loadJSON(data); err != nil {
			return nil, fmt.Errorf("failed to load attack ability: %w", err)
		}
		return attack, nil

	case refs.CombatAbilities.Dash().ID:
		dash := &Dash{}
		if err := dash.loadJSON(data); err != nil {
			return nil, fmt.Errorf("failed to load dash ability: %w", err)
		}
		return dash, nil

	case refs.CombatAbilities.Dodge().ID:
		dodge := &Dodge{}
		if err := dodge.loadJSON(data); err != nil {
			return nil, fmt.Errorf("failed to load dodge ability: %w", err)
		}
		return dodge, nil

	case refs.CombatAbilities.Disengage().ID:
		disengage := &Disengage{}
		if err := disengage.loadJSON(data); err != nil {
			return nil, fmt.Errorf("failed to load disengage ability: %w", err)
		}
		return disengage, nil

	case refs.CombatAbilities.Help().ID:
		// Help ability will be implemented in a future phase
		return nil, fmt.Errorf("help ability not yet implemented")

	case refs.CombatAbilities.Hide().ID:
		// Hide ability will be implemented in a future phase
		return nil, fmt.Errorf("hide ability not yet implemented")

	case refs.CombatAbilities.Ready().ID:
		// Ready ability will be implemented in a future phase
		return nil, fmt.Errorf("ready ability not yet implemented")

	default:
		return nil, fmt.Errorf("unknown combat ability type: %s", metadata.Ref.ID)
	}
}
