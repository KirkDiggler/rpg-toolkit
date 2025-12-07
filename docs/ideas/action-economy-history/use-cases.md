# Action Economy History - Use Cases

End-to-end flows from UI to toolkit and back.

## Use Case 1: Step of the Wind Activation

### Context

A Monk wants to use Step of the Wind. This feature costs 1 Ki and grants either Disengage or Dash as a bonus action. The user must choose which one.

### Flow

#### 1. UI Gets Character (includes features with choices)

Character data includes features and their activation requirements:

```protobuf
// Part of GetCharacter response
message Feature {
  string id = 1;
  Ref ref = 2;
  string name = 3;
  repeated ActivationChoice choices = 4;  // Empty if no choices needed
}

message ActivationChoice {
  string id = 1;           // "action_type"
  string description = 2;  // "Choose your action"
  repeated ActivationOption options = 3;
}

message ActivationOption {
  ActionType action_type = 1;  // Enum value
  string label = 2;            // "Disengage"
  string description = 3;      // "Avoid opportunity attacks"
}
```

#### 2. UI Presents Choice

User sees:
- "Step of the Wind (1 Ki, Bonus Action)"
- Options: [Disengage] [Dash]

User clicks [Disengage].

#### 3. UI Sends Activation Request

```protobuf
message ActivateFeatureRequest {
  string encounter_id = 1;
  string character_id = 2;
  string feature_id = 3;
  ActionType action_type = 4;  // ACTION_TYPE_DISENGAGE
}
```

#### 4. Game Server Processes

Game server:
1. Loads character, encounter, room
2. Creates GameContext (read-only snapshot)
3. Gets ActionEconomy for this turn (from TurnState)
4. Calls feature.Activate() with FeatureInput

```go
input := FeatureInput{
    Bus:           bus,
    ActionEconomy: economy,
    ActionType:    req.ActionType,  // Passed through, not interpreted
}
err := feature.Activate(ctx, character, input)
```

#### 5. Feature Executes

```go
func (s *StepOfTheWind) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    // Validate choice - feature validates its own requirements
    if input.ActionType != ActionDisengage && input.ActionType != ActionDash {
        return rpgerr.InvalidArgument("action_type must be disengage or dash")
    }

    // Consume Ki
    ki := accessor.GetResource(resources.Ki)
    if err := ki.Use(1); err != nil {
        return err
    }

    // Consume bonus action with record
    record := ActionRecord{
        Source:     refs.Features.StepOfTheWind(),
        ActionType: input.ActionType,
    }
    if err := input.ActionEconomy.UseBonusActionFor(record); err != nil {
        return err
    }

    // Publish generic feature activated event
    FeatureActivatedTopic.On(input.Bus).Publish(ctx, FeatureActivatedEvent{
        CharacterID: owner.GetID(),
        FeatureRef:  refs.Features.StepOfTheWind(),
        ActionType:  input.ActionType,
    })

    return nil
}
```

**Note:** Uses `FeatureActivatedTopic` not `StepOfTheWindActivatedTopic`. One topic for all features.

#### 6. Game Server Returns Result

```protobuf
message ActivateFeatureResponse {
  bool success = 1;
  string message = 2;
  TurnState updated_turn_state = 3;  // Includes action history
}
```

---

## Resolved Questions

### Q1: How does a feature declare its choices?

**Answer:** Feature interface gains `GetActivationChoices() []ActivationChoice` method.

```go
func (s *StepOfTheWind) GetActivationChoices() []ActivationChoice {
    return []ActivationChoice{
        {
            ID:          "action_type",
            Description: "Choose your action",
            Options: []ActivationOption{
                {ActionType: ActionDisengage, Label: "Disengage", Description: "Avoid opportunity attacks"},
                {ActionType: ActionDash, Label: "Dash", Description: "Double your movement"},
            },
        },
    }
}
```

### Q2: How does game server know which features require choices?

**Answer:** Call `GetActivationChoices()` - returns empty slice if no choices needed.

### Q3: ActionType validation

**Answer:** Feature validates it has what it needs. Same pattern as resource validation.

### Q4: Event payload

**Answer:** Use `ActionType` enum, not string. One `FeatureActivatedTopic` for all features.

---

## Use Case 2: Movement After Disengage

### Context

Monk used Step of the Wind (Disengage). Now they move away from an adjacent enemy.

### Alternative Approaches

There are two ways to implement the "no AoO after Disengage" behavior:

#### Approach A: Query ActionEconomy History

Movement processor queries the action history directly:

```go
func (s *MovementService) shouldTriggerAoO(ctx context.Context, mover *Character, enemy *Character) bool {
    // Get mover's action economy from context
    economy := gamectx.GetActionEconomy(ctx, mover.ID)

    // Did they Disengage this turn?
    if economy.HasTakenAction(ActionDisengage) || economy.HasTakenBonusAction(ActionDisengage) {
        return false
    }

    // Other checks...
    return enemy.ActionEconomy.CanUseReaction()
}
```

**Pros:**
- Simple, direct query
- No extra state to manage

**Cons:**
- Movement logic needs to know about ActionEconomy
- Tight coupling between systems

#### Approach B: Turn-Scoped Condition

Feature activation applies a condition that lasts until end of turn:

```go
func (s *StepOfTheWind) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    // ... Ki and bonus action consumption ...

    if input.ActionType == ActionDisengage {
        // Apply "Disengaged" condition that cancels AoO
        condition := conditions.NewDisengaged(owner.GetID())
        condition.Apply(ctx, input.Bus)  // Subscribes to AoO events, cancels them
    }

    // ... publish event ...
}
```

The condition subscribes to movement/AoO events and cancels opportunity attacks:

```go
func (d *DisengagedCondition) onOpportunityAttackCheck(ctx context.Context, event OpportunityAttackCheckEvent) {
    if event.TargetID == d.CharacterID {
        event.Cancel("target disengaged this turn")
    }
}
```

**Pros:**
- Movement logic stays simple - just fires events
- Conditions handle the rules (existing pattern)
- Extensible - other effects can also cancel AoO

**Cons:**
- More moving parts
- Condition needs cleanup at end of turn

#### Which to choose?

Both are valid. The condition approach fits better with the event-driven architecture. The query approach is simpler but creates coupling.

**Open question for design phase.**

---

## Use Case 3: Dash Speed Bonus

### Context

Monk uses Step of the Wind (Dash). Their movement should be doubled.

### Alternative Approaches

#### Approach A: Query ActionEconomy History

```go
func (s *MovementService) GetMaxMovement(ctx context.Context, character *Character) int {
    base := character.Speed

    economy := gamectx.GetActionEconomy(ctx, character.ID)
    if economy.HasTakenAction(ActionDash) || economy.HasTakenBonusAction(ActionDash) {
        base *= 2
    }

    return base
}
```

#### Approach B: Turn-Scoped Condition

Feature applies a "Dashing" condition that modifies speed:

```go
func (d *DashingCondition) onSpeedCalculation(ctx context.Context, event SpeedCalculationEvent) {
    if event.CharacterID == d.CharacterID {
        event.Multiplier *= 2
    }
}
```

**Note:** This may be overkill for Dash since it's a simple calculation. Approach A might be better here.

---

## Use Case 4: Patient Defense (Dodge)

### Context

Monk uses Patient Defense. Costs 1 Ki, bonus action, grants Dodge effect (attackers have disadvantage).

### Difference from Step of the Wind

- **No choice needed**: Patient Defense always grants Dodge
- **Effect needs tracking**: Attackers targeting this character have disadvantage

### Flow

#### 1. UI Requests Activation (No Choice)

```protobuf
message ActivateFeatureRequest {
  string encounter_id = 1;
  string character_id = 2;
  string feature_id = 3;
  // action_type not set - no choice needed
}
```

#### 2. Feature Executes

```go
func (p *PatientDefense) Activate(ctx context.Context, owner core.Entity, input FeatureInput) error {
    // Consume Ki
    ki := accessor.GetResource(resources.Ki)
    if err := ki.Use(1); err != nil {
        return err
    }

    // Consume bonus action - ActionType is always Dodge
    record := ActionRecord{
        Source:     refs.Features.PatientDefense(),
        ActionType: ActionDodge,
    }
    if err := input.ActionEconomy.UseBonusActionFor(record); err != nil {
        return err
    }

    // Publish generic event
    FeatureActivatedTopic.On(input.Bus).Publish(ctx, FeatureActivatedEvent{
        CharacterID: owner.GetID(),
        FeatureRef:  refs.Features.PatientDefense(),
        ActionType:  ActionDodge,
    })

    return nil
}
```

#### 3. Attack Roll Against Dodging Character

Either approach works:

**Approach A: Query history**
```go
func (a *AttackResolver) ResolveAttack(ctx context.Context, attacker, target *Character) *AttackResult {
    economy := gamectx.GetActionEconomy(ctx, target.ID)

    if economy.HasTakenBonusAction(ActionDodge) || economy.HasTakenAction(ActionDodge) {
        attack.Disadvantage = true
    }
    // ...
}
```

**Approach B: Condition**
```go
// DodgingCondition subscribes to attack events
func (d *DodgingCondition) onAttackRoll(ctx context.Context, event AttackRollEvent) {
    if event.TargetID == d.CharacterID {
        event.AttackerDisadvantage = true
    }
}
```

---

## Summary: What the Use Cases Reveal

### Confirmed Decisions

1. **Features declare their choices via interface** - `GetActivationChoices() []ActivationChoice`
2. **Feature validates its own input** - same as resource validation
3. **One FeatureActivatedTopic** - not per-feature topics
4. **ActionType is an enum** - in protos and toolkit, not string

### New Gaps Identified

1. **Proto changes needed:**
   - `ActionType` enum in protos
   - `ActivationChoice`/`ActivationOption` messages
   - `ActivateFeatureRequest.action_type` field
   - `TurnState` with action history

2. **Toolkit changes needed:**
   - `FeatureActivatedTopic` replaces per-feature topics
   - `GetActivationChoices()` on feature interface
   - ActionEconomy history (from brainstorm)

3. **Open design choice:**
   - Query ActionEconomy vs Turn-scoped Conditions for effects like Disengage/Dodge
   - May need both patterns for different use cases

### Affected Projects

This idea spans:
- **rpg-api-protos** - ActionType enum, ActivationChoice messages, TurnState updates
- **rpg-toolkit** - ActionEconomy history, feature interface, events
- **rpg-api** - Game server orchestration, context setup
