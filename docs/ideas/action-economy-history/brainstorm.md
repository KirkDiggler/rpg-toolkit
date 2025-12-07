# Action Economy History - Brainstorm

## Problem

Step of the Wind allows a Monk to spend 1 Ki and take Disengage **or** Dash as a bonus action. Later in the turn, game logic needs to know which one was chosen:

- **AoO check**: Did they Disengage? If so, no opportunity attacks.
- **Movement processing**: Did they Dash? If so, double speed.

Currently `ActionEconomy` only tracks "how many actions remain" - it has no knowledge of what those actions were used for.

## Current State (main branch)

### ActionEconomy

Budget tracking only:

```go
type ActionEconomy struct {
    ActionsRemaining      int
    BonusActionsRemaining int
    ReactionsRemaining    int
}

func (ae *ActionEconomy) UseBonusAction() error  // Decrements, no record of what used it
```

### FeatureInput

Provides optional ActionEconomy to features:

```go
type FeatureInput struct {
    Bus           events.EventBus
    ActionEconomy *combat.ActionEconomy
}
```

No way to pass action choices (like Disengage vs Dash).

### The Gaps

1. **No action choices**: Features with options (Step of Wind: Disengage or Dash) have no way to receive the choice
2. **No history**: After `UseBonusAction()`, no record of what consumed it
3. **No queryability**: Later logic cannot ask "did they Disengage this turn?"

## Solution

### 1. Add ActionType

Type in core, constants in rulebook:

```go
// core/combat/types.go
type ActionType string

// rulebooks/dnd5e/combat/action_types.go
const (
    ActionDodge     ActionType = "dodge"
    ActionDisengage ActionType = "disengage"
    ActionDash      ActionType = "dash"
    ActionAttack    ActionType = "attack"
    // ... more as needed
)
```

### 2. Add ActionRecord

```go
// rulebooks/dnd5e/combat/action_economy.go
type ActionRecord struct {
    Source     core.Ref   // refs.Features.StepOfTheWind()
    ActionType ActionType // ActionDisengage, ActionDash, etc.
}
```

### 3. Extend ActionEconomy with History

```go
type ActionEconomy struct {
    ActionsRemaining      int
    BonusActionsRemaining int
    ReactionsRemaining    int

    ActionsTaken      []ActionRecord
    BonusActionsTaken []ActionRecord
    ReactionsTaken    []ActionRecord
}
```

### 4. Replace API with Recording Versions

Remove:
```go
func (ae *ActionEconomy) UseAction() error
func (ae *ActionEconomy) UseBonusAction() error
func (ae *ActionEconomy) UseReaction() error
```

Add:
```go
func (ae *ActionEconomy) UseActionFor(record ActionRecord) error
func (ae *ActionEconomy) UseBonusActionFor(record ActionRecord) error
func (ae *ActionEconomy) UseReactionFor(record ActionRecord) error

func (ae *ActionEconomy) HasTakenAction(actionType ActionType) bool
func (ae *ActionEconomy) HasTakenBonusAction(actionType ActionType) bool
func (ae *ActionEconomy) HasTakenReaction(actionType ActionType) bool
```

### 5. Feature Integration

Features that cost actions consume via the new API:

```go
func (s *StepOfTheWind) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    // Consume Ki
    ki := accessor.GetResource(resources.Ki)
    if err := ki.Use(1); err != nil {
        return err
    }

    // Consume bonus action with record
    record := ActionRecord{
        Source:     refs.Features.StepOfTheWind(),
        ActionType: input.ActionType, // ActionDisengage or ActionDash
    }
    if err := input.ActionEconomy.UseBonusActionFor(record); err != nil {
        return err
    }

    // Publish event
    // ...
}
```

Note: `FeatureInput` gains an `ActionType` field for features with choices, replacing the untyped `Action string` that was attempted on the branch.

### 6. Query Example (Future AoO Check)

```go
func (c *OpportunityAttackCondition) shouldTrigger(economy *ActionEconomy) bool {
    if economy.HasTakenBonusAction(ActionDisengage) {
        return false // They disengaged, no AoO
    }
    if economy.HasTakenAction(ActionDisengage) {
        return false // Normal Disengage action also works
    }
    return true
}
```

## Open Questions

1. Should `ActionRecord` include timestamp or sequence number for ordering?
2. Should there be an `ActionRecord.Data` field for action-specific data?
3. How does this interact with reactions taken outside your turn (e.g., AoO you take on enemy's turn)?

## Next Steps

- Define concrete use cases to validate the design
- Implement ActionType in core/combat
- Extend ActionEconomy with history tracking
- Update features to use new API
