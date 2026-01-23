# Design: Game Server (rpg-api) Changes

## Overview

Game server changes to orchestrate monster turns during combat. When a player ends their turn, the server executes all monster turns until reaching the next player, then returns the results.

**Key principle:** rpg-api stores data and orchestrates. rpg-toolkit handles rules.

---

## 1. Monster Storage in Encounter

Monsters are stored inline in the encounter entity. They're ephemeral - created when encounter starts, gone when encounter ends.

```go
// internal/entities/encounter.go

type Encounter struct {
    ID            string
    // ... existing fields

    // Monsters in this encounter (stored inline)
    Monsters      []*MonsterData
}

// MonsterData matches toolkit's monster.Data structure
type MonsterData struct {
    ID            string                 `json:"id"`
    Name          string                 `json:"name"`
    MonsterType   string                 `json:"monster_type"`
    HitPoints     int                    `json:"hit_points"`
    MaxHitPoints  int                    `json:"max_hit_points"`
    ArmorClass    int                    `json:"armor_class"`
    AbilityScores map[string]int         `json:"ability_scores"`
    Speed         SpeedData              `json:"speed"`
    Senses        SensesData             `json:"senses"`
    Actions       []json.RawMessage      `json:"actions"`
    Position      Position               `json:"position"`
}
```

---

## 2. EndTurn Handler Updates

Handler passes through to orchestrator, returns monster turn results:

```go
// internal/handlers/dnd5e/v1alpha1/encounter/handler.go

func (h *Handler) EndTurn(
    ctx context.Context,
    req *dnd5ev1alpha1.EndTurnRequest,
) (*dnd5ev1alpha1.EndTurnResponse, error) {
    // 1. Validate request
    if req.GetEncounterId() == "" {
        return nil, status.Error(codes.InvalidArgument, "encounter_id is required")
    }

    // 2. Create orchestrator input
    input := &encounter.EndTurnInput{
        EncounterID: req.GetEncounterId(),
        EntityID:    req.GetEntityId(),
    }

    // 3. Call orchestrator
    output, err := h.encounterService.EndTurn(ctx, input)
    if err != nil {
        return nil, errors.ToGRPCError(err)
    }

    // 4. Convert to proto response
    return &dnd5ev1alpha1.EndTurnResponse{
        CombatState:  toProtoCombatState(output.CombatState),
        MonsterTurns: toProtoMonsterTurns(output.MonsterTurns),
    }, nil
}
```

---

## 3. EndTurn Orchestrator

The orchestrator advances turns and executes monster turns automatically:

```go
// internal/orchestrators/encounter/orchestrator.go

type EndTurnInput struct {
    EncounterID string
    EntityID    string
}

type EndTurnOutput struct {
    CombatState  *entities.CombatState
    MonsterTurns []*MonsterTurnResult
}

func (o *Orchestrator) EndTurn(
    ctx context.Context,
    input *EndTurnInput,
) (*EndTurnOutput, error) {
    if input == nil {
        return nil, errors.InvalidArgument("input is required")
    }

    // 1. Load encounter
    encounter, err := o.encounterRepo.Get(ctx, input.EncounterID)
    if err != nil {
        return nil, errors.Wrapf(err, "failed to load encounter")
    }

    // 2. Verify it's the entity's turn
    if encounter.CombatState.CurrentEntityID() != input.EntityID {
        return nil, errors.FailedPrecondition("not this entity's turn")
    }

    // 3. Advance to next entity
    encounter.CombatState.AdvanceTurn()

    // 4. Execute monster turns until reaching a player
    monsterTurns := o.executeMonsterTurns(ctx, encounter)

    // 5. Save encounter
    if err := o.encounterRepo.Save(ctx, encounter); err != nil {
        return nil, errors.Wrapf(err, "failed to save encounter")
    }

    return &EndTurnOutput{
        CombatState:  encounter.CombatState,
        MonsterTurns: monsterTurns,
    }, nil
}
```

---

## 4. Monster Turn Execution Helper

Extracted helper that loops through monster turns:

```go
// internal/orchestrators/encounter/monster_turns.go

import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
)

// MonsterTurnResult captures what happened during a monster's turn
type MonsterTurnResult struct {
    MonsterID    string
    MonsterName  string
    Actions      []*ExecutedAction
    MovementPath []Position
}

type ExecutedAction struct {
    ActionID   string
    ActionType string
    TargetID   string
    Success    bool
    Details    any // AttackResult, HealResult, etc.
}

// executeMonsterTurns runs all monster turns until reaching a player
func (o *Orchestrator) executeMonsterTurns(
    ctx context.Context,
    enc *entities.Encounter,
) []*MonsterTurnResult {
    var results []*MonsterTurnResult

    for o.isMonsterTurn(enc) {
        currentID := enc.CombatState.CurrentEntityID()

        // Check if monster is dead
        monsterData := o.findMonster(enc, currentID)
        if monsterData == nil || monsterData.HitPoints <= 0 {
            // Remove from initiative and continue
            enc.CombatState.RemoveFromInitiative(currentID)
            continue
        }

        // Execute this monster's turn
        result := o.executeSingleMonsterTurn(ctx, enc, monsterData)
        results = append(results, result)

        // Check for TPK (all players down)
        if o.allPlayersDown(enc) {
            enc.CombatState.EndCombat("all_players_down")
            break
        }

        // Advance to next entity
        enc.CombatState.AdvanceTurn()
    }

    return results
}

// executeSingleMonsterTurn runs one monster's turn
func (o *Orchestrator) executeSingleMonsterTurn(
    ctx context.Context,
    enc *entities.Encounter,
    monsterData *entities.MonsterData,
) *MonsterTurnResult {
    // 1. Load monster from toolkit
    toolkitMonster, err := monster.LoadFromData(ctx, toToolkitMonsterData(monsterData), o.eventBus)
    if err != nil {
        // Log error, return empty result
        return &MonsterTurnResult{MonsterID: monsterData.ID, MonsterName: monsterData.Name}
    }
    defer toolkitMonster.Cleanup(ctx)

    // 2. Build turn input
    turnInput := &monster.TurnInput{
        Bus:           o.eventBus,
        ActionEconomy: combat.NewActionEconomy(),
        Perception:    o.buildPerception(enc, monsterData),
        Roller:        o.roller,
    }

    // 3. Execute turn
    turnResult, err := toolkitMonster.TakeTurn(ctx, turnInput)
    if err != nil {
        return &MonsterTurnResult{MonsterID: monsterData.ID, MonsterName: monsterData.Name}
    }

    // 4. Update monster position from movement
    if len(turnResult.Movement) > 0 {
        finalPos := turnResult.Movement[len(turnResult.Movement)-1]
        monsterData.Position = Position{X: finalPos.X, Y: finalPos.Y}
    }

    // 5. Convert to our result type
    return &MonsterTurnResult{
        MonsterID:    monsterData.ID,
        MonsterName:  monsterData.Name,
        Actions:      toExecutedActions(turnResult.Actions),
        MovementPath: toPositions(turnResult.Movement),
    }
}

// isMonsterTurn returns true if current entity is a monster
func (o *Orchestrator) isMonsterTurn(enc *entities.Encounter) bool {
    currentEntry := enc.CombatState.CurrentEntry()
    return currentEntry != nil && currentEntry.EntityType == "monster"
}

// findMonster finds a monster by ID in the encounter
func (o *Orchestrator) findMonster(enc *entities.Encounter, id string) *entities.MonsterData {
    for _, m := range enc.Monsters {
        if m.ID == id {
            return m
        }
    }
    return nil
}

// allPlayersDown returns true if all characters are at 0 HP
func (o *Orchestrator) allPlayersDown(enc *entities.Encounter) bool {
    // Query characters and check HP
    for _, charID := range enc.CharacterIDs {
        char, err := o.characterRepo.Get(ctx, charID)
        if err != nil {
            continue
        }
        if char.HitPoints > 0 {
            return false
        }
    }
    return true
}
```

---

## 5. Building Perception

The orchestrator builds perception data from room state:

```go
// internal/orchestrators/encounter/perception.go

func (o *Orchestrator) buildPerception(
    enc *entities.Encounter,
    monsterData *entities.MonsterData,
) *monster.PerceptionData {
    perception := &monster.PerceptionData{
        MyPosition: monster.Position{
            X: monsterData.Position.X,
            Y: monsterData.Position.Y,
        },
        Enemies: make([]monster.PerceivedEntity, 0),
        Allies:  make([]monster.PerceivedEntity, 0),
    }

    // Add characters as enemies
    for _, charID := range enc.CharacterIDs {
        char, err := o.characterRepo.Get(ctx, charID)
        if err != nil || char.HitPoints <= 0 {
            continue // Skip dead or missing characters
        }

        charPos := enc.GetEntityPosition(charID)
        dist := o.calculateDistance(monsterData.Position, charPos)

        perception.Enemies = append(perception.Enemies, monster.PerceivedEntity{
            Entity:   &entityAdapter{id: charID, entityType: "character"},
            Position: monster.Position{X: charPos.X, Y: charPos.Y},
            Distance: dist,
            Adjacent: dist <= 5,
        })
    }

    // Add other monsters as allies
    for _, otherMonster := range enc.Monsters {
        if otherMonster.ID == monsterData.ID || otherMonster.HitPoints <= 0 {
            continue
        }

        dist := o.calculateDistance(monsterData.Position, otherMonster.Position)

        perception.Allies = append(perception.Allies, monster.PerceivedEntity{
            Entity:   &entityAdapter{id: otherMonster.ID, entityType: "monster"},
            Position: monster.Position{X: otherMonster.Position.X, Y: otherMonster.Position.Y},
            Distance: dist,
            Adjacent: dist <= 5,
        })
    }

    // Sort enemies by distance (closest first)
    sort.Slice(perception.Enemies, func(i, j int) bool {
        return perception.Enemies[i].Distance < perception.Enemies[j].Distance
    })

    return perception
}

// entityAdapter implements core.Entity for ID/Type lookup
type entityAdapter struct {
    id         string
    entityType string
}

func (e *entityAdapter) GetID() string            { return e.id }
func (e *entityAdapter) GetType() core.EntityType { return core.EntityType(e.entityType) }
```

---

## 6. Converters

### Monster Turn Result Converter

```go
// internal/handlers/dnd5e/v1alpha1/encounter/converters.go

func toProtoMonsterTurns(turns []*encounter.MonsterTurnResult) []*dnd5ev1alpha1.MonsterTurnResult {
    if turns == nil {
        return nil
    }

    results := make([]*dnd5ev1alpha1.MonsterTurnResult, len(turns))
    for i, turn := range turns {
        results[i] = &dnd5ev1alpha1.MonsterTurnResult{
            MonsterId:    turn.MonsterID,
            MonsterName:  turn.MonsterName,
            Actions:      toProtoMonsterActions(turn.Actions),
            MovementPath: toProtoPositions(turn.MovementPath),
        }
    }
    return results
}

func toProtoMonsterActions(actions []*encounter.ExecutedAction) []*dnd5ev1alpha1.MonsterExecutedAction {
    if actions == nil {
        return nil
    }

    results := make([]*dnd5ev1alpha1.MonsterExecutedAction, len(actions))
    for i, action := range actions {
        results[i] = &dnd5ev1alpha1.MonsterExecutedAction{
            ActionId:   action.ActionID,
            ActionType: action.ActionType,
            TargetId:   action.TargetID,
            Success:    action.Success,
        }

        // Add details based on action type
        switch details := action.Details.(type) {
        case *combat.AttackResult:
            results[i].Details = &dnd5ev1alpha1.MonsterExecutedAction_AttackResult{
                AttackResult: toProtoAttackResult(details),
            }
        case *combat.HealResult:
            results[i].Details = &dnd5ev1alpha1.MonsterExecutedAction_HealResult{
                HealResult: toProtoHealResult(details),
            }
        }
    }
    return results
}
```

### Toolkit Data Converter

```go
// internal/orchestrators/encounter/converters.go

import (
    toolkitMonster "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

func toToolkitMonsterData(m *entities.MonsterData) *toolkitMonster.Data {
    return &toolkitMonster.Data{
        ID:            m.ID,
        Name:          m.Name,
        MonsterType:   m.MonsterType,
        HitPoints:     m.HitPoints,
        MaxHitPoints:  m.MaxHitPoints,
        ArmorClass:    m.ArmorClass,
        AbilityScores: toToolkitAbilityScores(m.AbilityScores),
        Speed:         toToolkitSpeed(m.Speed),
        Senses:        toToolkitSenses(m.Senses),
        Actions:       m.Actions,
    }
}
```

---

## 7. DungeonStart Updates

DungeonStart should also execute monster turns if monsters go first:

```go
// internal/orchestrators/encounter/dungeon.go

func (o *Orchestrator) DungeonStart(ctx context.Context, input *DungeonStartInput) (*DungeonStartOutput, error) {
    // ... existing initialization ...

    // Roll initiative for all entities
    o.rollInitiative(ctx, encounter)

    // Execute monster turns if monsters go first
    monsterTurns := o.executeMonsterTurns(ctx, encounter)

    // Save encounter
    if err := o.encounterRepo.Save(ctx, encounter); err != nil {
        return nil, errors.Wrapf(err, "failed to save encounter")
    }

    return &DungeonStartOutput{
        CombatState:  encounter.CombatState,
        MonsterTurns: monsterTurns,  // May be empty if player goes first
        // ... other fields
    }, nil
}
```

---

## File Summary

| Layer | File | Changes |
|-------|------|---------|
| Entity | entities/encounter.go | Add Monsters field |
| Handler | handlers/dnd5e/v1alpha1/encounter/handler.go | Update EndTurn response |
| Handler | handlers/dnd5e/v1alpha1/encounter/converters.go | Monster turn converters |
| Orchestrator | orchestrators/encounter/orchestrator.go | EndTurn with monster execution |
| Orchestrator | orchestrators/encounter/monster_turns.go | Monster turn execution helper |
| Orchestrator | orchestrators/encounter/perception.go | Perception building |
| Orchestrator | orchestrators/encounter/converters.go | Toolkit data converters |
| Orchestrator | orchestrators/encounter/dungeon.go | DungeonStart monster turns |

---

## Key Principles

1. **Orchestrator orchestrates** - Load data, call toolkit, save results
2. **Toolkit does behavior** - Monster decisions happen in rpg-toolkit
3. **Batch monster turns** - Execute all until reaching player, return together
4. **CombatState is truth** - Always return updated CombatState
5. **Dead monsters removed** - Clean up initiative order
6. **TPK ends combat** - Stop execution when all players down

---

## Open Questions

1. **Event bus lifecycle** - One per encounter? Created in DungeonStart, passed to monster.LoadFromData?
2. **Room/spatial integration** - Current design builds perception manually. Future: use spatial.Room?
3. **Monster proto for client** - Does client need full monster data (HP, AC) for display?
