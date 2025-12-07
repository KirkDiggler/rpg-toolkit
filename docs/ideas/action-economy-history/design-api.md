# Design: Game Server (rpg-api) Changes

## Overview

Game server changes to support passing ActionType through, including feature choices in character responses, and tracking action history in TurnState.

**Key principle:** rpg-api stores data and orchestrates. rpg-toolkit handles rules.

## 1. Handler: ActivateFeature

Handler validates request and calls orchestrator:

```go
// internal/handlers/dnd5e/v1alpha1/encounter/handler.go

// ActivateFeature handles feature activation requests
func (h *Handler) ActivateFeature(
    ctx context.Context,
    req *dnd5ev1alpha1.ActivateFeatureRequest,
) (*dnd5ev1alpha1.ActivateFeatureResponse, error) {
    // 1. Validate request
    if req.GetEncounterId() == "" {
        return nil, status.Error(codes.InvalidArgument, "encounter_id is required")
    }
    if req.GetCharacterId() == "" {
        return nil, status.Error(codes.InvalidArgument, "character_id is required")
    }
    if req.GetFeatureId() == "" {
        return nil, status.Error(codes.InvalidArgument, "feature_id is required")
    }

    // 2. Create orchestrator input
    input := &encounter.ActivateFeatureInput{
        EncounterID: req.GetEncounterId(),
        CharacterID: req.GetCharacterId(),
        FeatureID:   req.GetFeatureId(),
        ActionType:  req.GetActionType(),  // Pass through proto enum
    }

    // 3. Call orchestrator
    output, err := h.encounterService.ActivateFeature(ctx, input)
    if err != nil {
        return nil, errors.ToGRPCError(err)  // Uses internal/errors package
    }

    // 4. Convert to proto response
    return &dnd5ev1alpha1.ActivateFeatureResponse{
        Success:            true,
        Message:            output.Message,
        UpdatedCharacter:   toProtoCharacter(output.Character),
        UpdatedCombatState: toProtoCombatState(output.CombatState),
    }, nil
}
```

## 2. Orchestrator: ActivateFeature

Orchestrator loads data, calls toolkit, saves results:

```go
// internal/orchestrators/encounter/orchestrator.go

// ActivateFeatureInput is the input for feature activation
type ActivateFeatureInput struct {
    EncounterID string
    CharacterID string
    FeatureID   string
    ActionType  dnd5ev1alpha1.ActionType  // Proto enum, passed through
}

// ActivateFeatureOutput is the output for feature activation
type ActivateFeatureOutput struct {
    Message     string
    Character   *entities.Character
    CombatState *entities.CombatState
}

func (o *Orchestrator) ActivateFeature(
    ctx context.Context,
    input *ActivateFeatureInput,
) (*ActivateFeatureOutput, error) {
    if input == nil {
        return nil, errors.InvalidArgument("input is required")
    }

    // 1. Load encounter
    encounter, err := o.encounterRepo.Get(ctx, input.EncounterID)
    if err != nil {
        return nil, errors.Wrapf(err, "failed to load encounter %s", input.EncounterID)
    }

    // 2. Load character
    character, err := o.characterRepo.Get(ctx, input.CharacterID)
    if err != nil {
        return nil, errors.Wrapf(err, "failed to load character %s", input.CharacterID)
    }

    // 3. Get feature from character
    feature := character.GetFeature(input.FeatureID)
    if feature == nil {
        return nil, errors.NotFound("feature %s not found", input.FeatureID)
    }

    // 4. Get turn state and convert to ActionEconomy
    turnState := encounter.GetTurnState(input.CharacterID)
    actionEconomy := o.toActionEconomy(turnState)

    // 5. Build feature input - pass ActionType through without interpretation
    featureInput := features.FeatureInput{
        Bus:           o.eventBus,
        ActionEconomy: actionEconomy,
        ActionType:    fromProtoActionType(input.ActionType),
    }

    // 6. Activate feature (toolkit does validation)
    if err := feature.Activate(ctx, character, featureInput); err != nil {
        // Toolkit returns typed errors - preserve them
        return nil, err
    }

    // 7. Update turn state from action economy
    encounter.UpdateTurnState(input.CharacterID, o.toTurnState(actionEconomy, turnState))

    // 8. Save encounter
    if err := o.encounterRepo.Save(ctx, encounter); err != nil {
        return nil, errors.Wrapf(err, "failed to save encounter")
    }

    // 9. Save character if modified
    if character.IsDirty() {
        if err := o.characterRepo.Save(ctx, character); err != nil {
            return nil, errors.Wrapf(err, "failed to save character")
        }
    }

    return &ActivateFeatureOutput{
        Message:     fmt.Sprintf("%s activated", feature.Name()),
        Character:   character,
        CombatState: encounter.CombatState,
    }, nil
}
```

## 3. Conversion Helpers

### ActionType Conversion

```go
// internal/handlers/dnd5e/v1alpha1/encounter/converters.go

import (
    dnd5ev1alpha1 "github.com/KirkDiggler/rpg-api-protos/gen/go/dnd5e/api/v1alpha1"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

func fromProtoActionType(pt dnd5ev1alpha1.ActionType) combat.ActionType {
    switch pt {
    case dnd5ev1alpha1.ActionType_ACTION_TYPE_ATTACK:
        return combat.ActionAttack
    case dnd5ev1alpha1.ActionType_ACTION_TYPE_DASH:
        return combat.ActionDash
    case dnd5ev1alpha1.ActionType_ACTION_TYPE_DISENGAGE:
        return combat.ActionDisengage
    case dnd5ev1alpha1.ActionType_ACTION_TYPE_DODGE:
        return combat.ActionDodge
    default:
        return ""
    }
}

func toProtoActionType(at combat.ActionType) dnd5ev1alpha1.ActionType {
    switch at {
    case combat.ActionAttack:
        return dnd5ev1alpha1.ActionType_ACTION_TYPE_ATTACK
    case combat.ActionDash:
        return dnd5ev1alpha1.ActionType_ACTION_TYPE_DASH
    case combat.ActionDisengage:
        return dnd5ev1alpha1.ActionType_ACTION_TYPE_DISENGAGE
    case combat.ActionDodge:
        return dnd5ev1alpha1.ActionType_ACTION_TYPE_DODGE
    default:
        return dnd5ev1alpha1.ActionType_ACTION_TYPE_UNSPECIFIED
    }
}
```

### ActionEconomy Conversion

```go
// internal/orchestrators/encounter/converters.go

func (o *Orchestrator) toActionEconomy(ts *dnd5ev1alpha1.TurnState) *combat.ActionEconomy {
    ae := combat.NewActionEconomy()
    ae.ActionsRemaining = int(ts.GetActionsRemaining())
    ae.BonusActionsRemaining = int(ts.GetBonusActionsRemaining())
    ae.ReactionsRemaining = int(ts.GetReactionsRemaining())

    // Convert history
    for _, record := range ts.GetActionsTaken() {
        ae.Taken.Actions = append(ae.Taken.Actions, combat.ActionRecord{
            Source:     core.ParseRef(record.GetSourceRef()),
            ActionType: fromProtoActionType(record.GetActionType()),
        })
    }
    for _, record := range ts.GetBonusActionsTaken() {
        ae.Taken.BonusActions = append(ae.Taken.BonusActions, combat.ActionRecord{
            Source:     core.ParseRef(record.GetSourceRef()),
            ActionType: fromProtoActionType(record.GetActionType()),
        })
    }
    for _, record := range ts.GetReactionsTaken() {
        ae.Taken.Reactions = append(ae.Taken.Reactions, combat.ActionRecord{
            Source:     core.ParseRef(record.GetSourceRef()),
            ActionType: fromProtoActionType(record.GetActionType()),
        })
    }

    return ae
}

func (o *Orchestrator) toTurnState(ae *combat.ActionEconomy, existing *dnd5ev1alpha1.TurnState) *dnd5ev1alpha1.TurnState {
    ts := &dnd5ev1alpha1.TurnState{
        EntityId:              existing.GetEntityId(),
        MovementUsed:          existing.GetMovementUsed(),
        MovementMax:           existing.GetMovementMax(),
        Position:              existing.GetPosition(),
        ActionsRemaining:      int32(ae.ActionsRemaining),
        BonusActionsRemaining: int32(ae.BonusActionsRemaining),
        ReactionsRemaining:    int32(ae.ReactionsRemaining),
    }

    // Convert history
    for _, record := range ae.Taken.Actions {
        ts.ActionsTaken = append(ts.ActionsTaken, &dnd5ev1alpha1.ActionRecord{
            SourceRef:  record.Source.String(),
            ActionType: toProtoActionType(record.ActionType),
        })
    }
    for _, record := range ae.Taken.BonusActions {
        ts.BonusActionsTaken = append(ts.BonusActionsTaken, &dnd5ev1alpha1.ActionRecord{
            SourceRef:  record.Source.String(),
            ActionType: toProtoActionType(record.ActionType),
        })
    }
    for _, record := range ae.Taken.Reactions {
        ts.ReactionsTaken = append(ts.ReactionsTaken, &dnd5ev1alpha1.ActionRecord{
            SourceRef:  record.Source.String(),
            ActionType: toProtoActionType(record.ActionType),
        })
    }

    return ts
}
```

## 4. GetCharacter with Activation Choices

Include feature choices in character response:

```go
// internal/handlers/dnd5e/v1alpha1/character/converters.go

func toProtoFeature(f toolkitfeatures.Feature) *dnd5ev1alpha1.Feature {
    protoFeature := &dnd5ev1alpha1.Feature{
        Id:          f.GetID(),
        Ref:         toProtoRef(f.Ref()),
        Name:        f.Name(),
        Description: f.Description(),
    }

    // Check if feature provides activation choices
    if provider, ok := f.(toolkitfeatures.ActivationChoiceProvider); ok {
        protoFeature.ActivationChoices = toProtoActivationChoices(provider.GetActivationChoices())
    }

    return protoFeature
}

func toProtoActivationChoices(choices []toolkitfeatures.ActivationChoice) []*dnd5ev1alpha1.ActivationChoice {
    if len(choices) == 0 {
        return nil
    }

    result := make([]*dnd5ev1alpha1.ActivationChoice, len(choices))
    for i, choice := range choices {
        options := make([]*dnd5ev1alpha1.ActivationOption, len(choice.Options))
        for j, opt := range choice.Options {
            options[j] = &dnd5ev1alpha1.ActivationOption{
                ActionType:  toProtoActionType(opt.ActionType),
                Label:       opt.Label,
                Description: opt.Description,
            }
        }

        result[i] = &dnd5ev1alpha1.ActivationChoice{
            Id:          choice.ID,
            Description: choice.Description,
            Options:     options,
        }
    }
    return result
}
```

## 5. Movement with AoO Check

When processing movement, publish AoO check events:

```go
// internal/orchestrators/encounter/movement.go

func (o *Orchestrator) processMovement(ctx context.Context, mover *entities.Character, path []Position) error {
    for i := 1; i < len(path); i++ {
        previousPos := path[i-1]
        currentPos := path[i]

        // Check for opportunity attacks when leaving threatened squares
        for _, enemy := range o.getAdjacentEnemies(previousPos) {
            if o.isLeavingThreat(previousPos, currentPos, enemy) {
                // Publish check event - conditions can cancel
                checkEvent := &dnd5eEvents.OpportunityAttackCheckEvent{
                    AttackerID: enemy.GetID(),
                    TargetID:   mover.GetID(),
                }

                topic := dnd5eEvents.OpportunityAttackCheckTopic.On(o.eventBus)
                if err := topic.Publish(ctx, checkEvent); err != nil {
                    return errors.Wrapf(err, "failed to publish AoO check event")
                }

                // If not cancelled, trigger AoO
                if !checkEvent.IsCancelled() {
                    if err := o.triggerOpportunityAttack(ctx, enemy, mover); err != nil {
                        return errors.Wrapf(err, "failed to trigger opportunity attack")
                    }
                }
            }
        }

        // Move to new position
        if err := o.moveEntity(mover, currentPos); err != nil {
            return errors.Wrapf(err, "failed to move entity to %v", currentPos)
        }
    }

    return nil
}
```

## File Summary

| Layer | File | Changes |
|-------|------|---------|
| Handler | handlers/dnd5e/v1alpha1/encounter/handler.go | ActivateFeature handler |
| Handler | handlers/dnd5e/v1alpha1/encounter/converters.go | ActionType converters |
| Handler | handlers/dnd5e/v1alpha1/character/converters.go | Feature with choices |
| Orchestrator | orchestrators/encounter/orchestrator.go | ActivateFeature implementation |
| Orchestrator | orchestrators/encounter/converters.go | ActionEconomy ↔ TurnState |
| Orchestrator | orchestrators/encounter/movement.go | AoO check events |

## Key Principles

1. **Handler validates request** - Required fields, format
2. **Orchestrator orchestrates** - Load, call toolkit, save
3. **Toolkit validates logic** - Features validate their own requirements
4. **Game server doesn't interpret ActionType** - Just passes it through
5. **Events decouple** - AoO checks go through event bus, conditions can cancel
6. **Use internal/errors package** - Consistent error handling
