# Design: Toolkit Changes for Action Economy History

## Overview

Toolkit changes to support ActionType, ActionEconomy history, feature activation choices, turn-scoped conditions, and consolidated event topics.

## 1. ActionType in core/combat

Type definition only (constants in rulebook):

```go
// core/combat/types.go
package combat

// ActionType represents a type of action that can be taken
type ActionType string
```

## 2. ActionType Constants in rulebooks/dnd5e/combat

```go
// rulebooks/dnd5e/combat/action_types.go
package combat

import corecombat "github.com/KirkDiggler/rpg-toolkit/core/combat"

// Action type constants for D&D 5e
// Only add constants when there's a concrete use case
const (
    ActionAttack    corecombat.ActionType = "attack"
    ActionDash      corecombat.ActionType = "dash"
    ActionDisengage corecombat.ActionType = "disengage"
    ActionDodge     corecombat.ActionType = "dodge"

    // Future action types - add when use cases arise:
    // ActionHelp      corecombat.ActionType = "help"
    // ActionHide      corecombat.ActionType = "hide"
    // ActionReady     corecombat.ActionType = "ready"
    // ActionSearch    corecombat.ActionType = "search"
    // ActionUseObject corecombat.ActionType = "use_object"
)
```

## 3. ActionRecord

```go
// rulebooks/dnd5e/combat/action_record.go
package combat

import (
    "github.com/KirkDiggler/rpg-toolkit/core"
    corecombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
)

// ActionRecord tracks what consumed an action slot
type ActionRecord struct {
    // Source is the feature/ability that used this action
    Source *core.Ref

    // ActionType is what action was taken
    ActionType corecombat.ActionType
}
```

## 4. ActionHistory

Group history slices together:

```go
// rulebooks/dnd5e/combat/action_history.go
package combat

import corecombat "github.com/KirkDiggler/rpg-toolkit/core/combat"

// ActionHistory tracks what actions have been taken during a turn
type ActionHistory struct {
    Actions      []ActionRecord
    BonusActions []ActionRecord
    Reactions    []ActionRecord
}

// NewActionHistory creates an empty ActionHistory
func NewActionHistory() *ActionHistory {
    return &ActionHistory{
        Actions:      make([]ActionRecord, 0),
        BonusActions: make([]ActionRecord, 0),
        Reactions:    make([]ActionRecord, 0),
    }
}

// HasTakenAction returns true if an action of the given type was taken
func (h *ActionHistory) HasTakenAction(actionType corecombat.ActionType) bool {
    for _, record := range h.Actions {
        if record.ActionType == actionType {
            return true
        }
    }
    return false
}

// HasTakenBonusAction returns true if a bonus action of the given type was taken
func (h *ActionHistory) HasTakenBonusAction(actionType corecombat.ActionType) bool {
    for _, record := range h.BonusActions {
        if record.ActionType == actionType {
            return true
        }
    }
    return false
}

// HasTakenReaction returns true if a reaction of the given type was taken
func (h *ActionHistory) HasTakenReaction(actionType corecombat.ActionType) bool {
    for _, record := range h.Reactions {
        if record.ActionType == actionType {
            return true
        }
    }
    return false
}

// Reset clears all history
func (h *ActionHistory) Reset() {
    h.Actions = make([]ActionRecord, 0)
    h.BonusActions = make([]ActionRecord, 0)
    h.Reactions = make([]ActionRecord, 0)
}
```

## 5. ActionEconomy Updates

```go
// rulebooks/dnd5e/combat/action_economy.go
package combat

import "github.com/KirkDiggler/rpg-toolkit/rpgerr"

// ActionEconomy tracks action budget and history for a combatant's turn
type ActionEconomy struct {
    ActionsRemaining      int
    BonusActionsRemaining int
    ReactionsRemaining    int

    Taken *ActionHistory
}

// NewActionEconomy creates a new ActionEconomy with default values (1/1/1)
func NewActionEconomy() *ActionEconomy {
    return &ActionEconomy{
        ActionsRemaining:      1,
        BonusActionsRemaining: 1,
        ReactionsRemaining:    1,
        Taken:                 NewActionHistory(),
    }
}

// UseActionFor consumes an action and records what used it
func (ae *ActionEconomy) UseActionFor(record ActionRecord) error {
    if ae.ActionsRemaining <= 0 {
        return rpgerr.ResourceExhausted("action")
    }
    ae.ActionsRemaining--
    ae.Taken.Actions = append(ae.Taken.Actions, record)
    return nil
}

// UseBonusActionFor consumes a bonus action and records what used it
func (ae *ActionEconomy) UseBonusActionFor(record ActionRecord) error {
    if ae.BonusActionsRemaining <= 0 {
        return rpgerr.ResourceExhausted("bonus action")
    }
    ae.BonusActionsRemaining--
    ae.Taken.BonusActions = append(ae.Taken.BonusActions, record)
    return nil
}

// UseReactionFor consumes a reaction and records what used it
func (ae *ActionEconomy) UseReactionFor(record ActionRecord) error {
    if ae.ReactionsRemaining <= 0 {
        return rpgerr.ResourceExhausted("reaction")
    }
    ae.ReactionsRemaining--
    ae.Taken.Reactions = append(ae.Taken.Reactions, record)
    return nil
}

// CanUseAction returns whether an action is available
func (ae *ActionEconomy) CanUseAction() bool {
    return ae.ActionsRemaining > 0
}

// CanUseBonusAction returns whether a bonus action is available
func (ae *ActionEconomy) CanUseBonusAction() bool {
    return ae.BonusActionsRemaining > 0
}

// CanUseReaction returns whether a reaction is available
func (ae *ActionEconomy) CanUseReaction() bool {
    return ae.ReactionsRemaining > 0
}

// Reset restores budget to defaults and clears history
func (ae *ActionEconomy) Reset() {
    ae.ActionsRemaining = 1
    ae.BonusActionsRemaining = 1
    ae.ReactionsRemaining = 1
    ae.Taken.Reset()
}

// GrantExtraAction grants an additional action
func (ae *ActionEconomy) GrantExtraAction() {
    ae.ActionsRemaining++
}

// GrantExtraBonusAction grants an additional bonus action
func (ae *ActionEconomy) GrantExtraBonusAction() {
    ae.BonusActionsRemaining++
}
```

## 6. FeatureInput Updates

```go
// rulebooks/dnd5e/features/types.go
package features

import (
    corecombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// FeatureInput provides input data for feature activation.
type FeatureInput struct {
    // Bus is provided by the character/owner during activation
    Bus events.EventBus `json:"-"`

    // ActionEconomy is provided for features that consume actions
    ActionEconomy *combat.ActionEconomy `json:"-"`

    // ActionType is the chosen action for features with choices
    ActionType corecombat.ActionType `json:"action_type,omitempty"`
}
```

## 7. Activation Choices Interface

```go
// rulebooks/dnd5e/features/activation.go
package features

import corecombat "github.com/KirkDiggler/rpg-toolkit/core/combat"

// ActivationOption represents one selectable option for feature activation
type ActivationOption struct {
    ActionType  corecombat.ActionType
    Label       string
    Description string
}

// ActivationChoice represents a choice required to activate a feature
type ActivationChoice struct {
    ID          string
    Description string
    Options     []ActivationOption
}

// ActivationChoiceProvider is implemented by features that require choices
type ActivationChoiceProvider interface {
    // GetActivationChoices returns choices needed for activation
    // Returns nil or empty slice if no choices required
    GetActivationChoices() []ActivationChoice
}
```

## 8. Step of the Wind Implementation

```go
// rulebooks/dnd5e/features/step_of_the_wind.go

import (
    "context"

    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/rpgerr"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
    dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// GetActivationChoices implements ActivationChoiceProvider
func (s *StepOfTheWind) GetActivationChoices() []ActivationChoice {
    return []ActivationChoice{
        {
            ID:          "action_type",
            Description: "Choose your action",
            Options: []ActivationOption{
                {ActionType: combat.ActionDisengage, Label: "Disengage", Description: "Avoid opportunity attacks"},
                {ActionType: combat.ActionDash, Label: "Dash", Description: "Double your movement"},
            },
        },
    }
}

func (s *StepOfTheWind) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    // Validate ActionEconomy provided
    if input.ActionEconomy == nil {
        return rpgerr.New(rpgerr.CodeInvalidArgument, "ActionEconomy is required")
    }

    // Validate choice
    if input.ActionType != combat.ActionDisengage && input.ActionType != combat.ActionDash {
        return rpgerr.New(rpgerr.CodeInvalidArgument, "action_type must be disengage or dash")
    }

    // Consume Ki
    accessor, ok := owner.(ResourceAccessor)
    if !ok {
        return rpgerr.New(rpgerr.CodeInvalidArgument, "owner must implement ResourceAccessor")
    }
    ki := accessor.GetResource(resources.Ki)
    if err := ki.Use(1); err != nil {
        return rpgerr.Wrapf(err, "failed to use ki for step of the wind")
    }

    // Consume bonus action with record
    record := combat.ActionRecord{
        Source:     refs.Features.StepOfTheWind(),
        ActionType: input.ActionType,
    }
    if err := input.ActionEconomy.UseBonusActionFor(record); err != nil {
        return rpgerr.Wrapf(err, "failed to use bonus action")
    }

    // Apply turn-scoped condition based on choice
    if input.Bus != nil {
        if input.ActionType == combat.ActionDisengage {
            // Apply Disengaged condition - cancels AoO for this turn
            condition := conditions.NewDisengaged(owner.GetID())
            if err := condition.Apply(ctx, input.Bus); err != nil {
                return rpgerr.Wrapf(err, "failed to apply disengaged condition")
            }
        }
        // Note: Dash effect handled by querying ActionEconomy.Taken.HasTakenBonusAction(ActionDash)

        // Publish generic feature activated event
        topic := dnd5eEvents.FeatureActivatedTopic.On(input.Bus)
        err := topic.Publish(ctx, dnd5eEvents.FeatureActivatedEvent{
            CharacterID: owner.GetID(),
            FeatureRef:  refs.Features.StepOfTheWind(),
            ActionType:  input.ActionType,
        })
        if err != nil {
            return rpgerr.Wrapf(err, "failed to publish feature activated event")
        }
    }

    return nil
}
```

## 9. Consolidated Event Topic

Replace per-feature topics with single topic:

```go
// rulebooks/dnd5e/events/events.go

import (
    "github.com/KirkDiggler/rpg-toolkit/core"
    corecombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
    "github.com/KirkDiggler/rpg-toolkit/events"
)

// FeatureActivatedEvent is published when any feature is activated
type FeatureActivatedEvent struct {
    CharacterID string
    FeatureRef  *core.Ref
    ActionType  corecombat.ActionType  // What action was taken (if applicable)
}

// FeatureActivatedTopic provides typed pub/sub for all feature activation events
var FeatureActivatedTopic = events.DefineTypedTopic[FeatureActivatedEvent]("dnd5e.feature.activated")

// DEPRECATED: Remove these per-feature topics
// var FlurryOfBlowsActivatedTopic = ...
// var PatientDefenseActivatedTopic = ...
// var StepOfTheWindActivatedTopic = ...
```

## 10. Disengaged Condition (Turn-Scoped)

```go
// rulebooks/dnd5e/conditions/disengaged.go
package conditions

import (
    "context"

    "github.com/KirkDiggler/rpg-toolkit/events"
    dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// Disengaged is a turn-scoped condition that prevents opportunity attacks
type Disengaged struct {
    characterID  string
    subscription events.Subscription
}

// NewDisengaged creates a new Disengaged condition for the character
func NewDisengaged(characterID string) *Disengaged {
    return &Disengaged{
        characterID: characterID,
    }
}

// Apply subscribes to opportunity attack check events
func (d *Disengaged) Apply(ctx context.Context, bus events.EventBus) error {
    topic := dnd5eEvents.OpportunityAttackCheckTopic.On(bus)
    sub, err := topic.Subscribe(ctx, d.onOpportunityAttackCheck)
    if err != nil {
        return err
    }
    d.subscription = sub

    // Subscribe to turn end to clean up
    turnTopic := dnd5eEvents.TurnEndTopic.On(bus)
    _, err = turnTopic.Subscribe(ctx, d.onTurnEnd)
    return err
}

func (d *Disengaged) onOpportunityAttackCheck(ctx context.Context, event *dnd5eEvents.OpportunityAttackCheckEvent) {
    if event.TargetID == d.characterID {
        event.Cancel("target disengaged this turn")
    }
}

func (d *Disengaged) onTurnEnd(ctx context.Context, event dnd5eEvents.TurnEndEvent) {
    if event.CharacterID == d.characterID {
        // Clean up subscription
        if d.subscription != nil {
            d.subscription.Unsubscribe()
        }
    }
}
```

## 11. Opportunity Attack Check Event

```go
// rulebooks/dnd5e/events/events.go

// OpportunityAttackCheckEvent is published when checking if an AoO should trigger
type OpportunityAttackCheckEvent struct {
    AttackerID string  // Entity that would make the AoO
    TargetID   string  // Entity leaving threatened square
    cancelled  bool
    reason     string
}

// Cancel prevents the opportunity attack from triggering
func (e *OpportunityAttackCheckEvent) Cancel(reason string) {
    e.cancelled = true
    e.reason = reason
}

// IsCancelled returns true if the AoO was cancelled
func (e *OpportunityAttackCheckEvent) IsCancelled() bool {
    return e.cancelled
}

// CancelReason returns why the AoO was cancelled
func (e *OpportunityAttackCheckEvent) CancelReason() string {
    return e.reason
}

var OpportunityAttackCheckTopic = events.DefineTypedTopic[*OpportunityAttackCheckEvent](
    "dnd5e.combat.opportunity_attack.check")
```

**Note:** Event uses pointer so condition can mutate `cancelled` field.

## File Summary

| File | Changes |
|------|---------|
| core/combat/types.go | Add ActionType type |
| rulebooks/dnd5e/combat/action_types.go | New - ActionType constants (only ones with use cases) |
| rulebooks/dnd5e/combat/action_record.go | New - ActionRecord struct |
| rulebooks/dnd5e/combat/action_history.go | New - ActionHistory struct with query helpers |
| rulebooks/dnd5e/combat/action_economy.go | Add Taken *ActionHistory, replace Use* with UseFor* |
| rulebooks/dnd5e/features/types.go | Add ActionType to FeatureInput |
| rulebooks/dnd5e/features/activation.go | New - ActivationChoice, ActivationChoiceProvider |
| rulebooks/dnd5e/features/step_of_the_wind.go | Implement choices, apply condition |
| rulebooks/dnd5e/features/patient_defense.go | Update to use new API |
| rulebooks/dnd5e/events/events.go | Add FeatureActivatedTopic, OpportunityAttackCheckTopic |
| rulebooks/dnd5e/conditions/disengaged.go | New - turn-scoped condition |
