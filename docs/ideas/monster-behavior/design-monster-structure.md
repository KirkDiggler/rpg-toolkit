# Monster Structure Design

Following the character pattern: Data struct for persistence, LoadFromData to wire to bus.

---

## The Pattern

```
Game Server (rpg-api)
    │
    ├─ Creates EventBus for encounter
    │
    ├─ Loads Character from storage
    │   └─ character.LoadFromData(ctx, charData, bus)  ← wires to bus
    │
    └─ Loads Monster from storage
        └─ monster.LoadFromData(ctx, monsterData, bus)  ← same pattern
```

**Key principle:** Monster is self-contained. The Data struct has everything needed to take a turn. Load it, wire it to the bus, and it participates in combat like a character.

---

## Monster Data Struct (Persistence)

What gets saved to storage. Pure JSON, no logic.

```go
// Data represents the serializable form of a monster
type Data struct {
    // Identity
    ID       string `json:"id"`
    Name     string `json:"name"`

    // What kind of monster (for behavior lookup, display, etc.)
    MonsterType string `json:"monster_type"`  // "goblin", "orc", "dragon"

    // Core stats
    HitPoints    int                  `json:"hit_points"`
    MaxHitPoints int                  `json:"max_hit_points"`
    ArmorClass   int                  `json:"armor_class"`
    AbilityScores shared.AbilityScores `json:"ability_scores"`

    // Movement
    Speed SpeedData `json:"speed"`

    // Senses (for perception/targeting)
    Senses SensesData `json:"senses"`

    // Actions this monster can take
    Actions []ActionData `json:"actions"`

    // Features (special abilities like Nimble Escape)
    Features []json.RawMessage `json:"features,omitempty"`

    // Conditions (runtime state: poisoned, hidden, etc.)
    Conditions []json.RawMessage `json:"conditions,omitempty"`

    // Inventory (potions, items)
    Inventory []InventoryItemData `json:"inventory,omitempty"`

    // Proficiencies (for skill checks like Stealth)
    Proficiencies []ProficiencyData `json:"proficiencies,omitempty"`
}

type SpeedData struct {
    Walk   int `json:"walk"`
    Fly    int `json:"fly,omitempty"`
    Swim   int `json:"swim,omitempty"`
    Climb  int `json:"climb,omitempty"`
    Burrow int `json:"burrow,omitempty"`
}

type SensesData struct {
    Darkvision       int `json:"darkvision,omitempty"`        // feet
    Blindsight       int `json:"blindsight,omitempty"`
    Tremorsense      int `json:"tremorsense,omitempty"`
    Truesight        int `json:"truesight,omitempty"`
    PassivePerception int `json:"passive_perception"`
}
```

---

## Monster Actions (Using core.Action[T] Pattern)

Monster actions use the same `core.Action[T]` pattern as features:

```go
// core.Action[T] interface (already exists)
type Action[T any] interface {
    Entity  // GetID(), GetType()
    CanActivate(ctx context.Context, owner Entity, input T) error
    Activate(ctx context.Context, owner Entity, input T) error
}
```

### MonsterActionInput

Everything a monster action needs to execute:

```go
type MonsterActionInput struct {
    // Event bus for publishing attacks, damage, conditions
    Bus events.EventBus

    // What the monster perceives (targets, distances, cover)
    Perception PerceptionData

    // Current conditions on the monster (for checking "am I raging?")
    Conditions []dnd5eEvents.ConditionBehavior

    // Action economy (for actions that grant extra actions)
    ActionEconomy *combat.ActionEconomy

    // Target selection (if action needs a target)
    Target core.Entity

    // Dice roller (for attacks, damage)
    Roller dice.Roller
}
```

### MonsterAction Interface

Extends core.Action with behavior scoring:

```go
// MonsterAction is a core.Action with scoring for AI behavior
type MonsterAction interface {
    core.Action[MonsterActionInput]

    // Cost returns the action economy cost
    Cost() ActionCost  // action, bonus_action, reaction

    // Score returns how desirable this action is in current situation
    Score(monster *Monster, perception PerceptionData) int

    // ActionType for categorization
    ActionType() ActionType  // melee_attack, ranged_attack, heal, etc.
}
```

### Action Data (Persistence)

What gets saved - just a ref to load the action, plus config:

```go
type ActionData struct {
    Ref    core.Ref        `json:"ref"`    // "dnd5e:monster-actions:scimitar"
    Config json.RawMessage `json:"config"` // Action-specific config
}
```

### Example: Scimitar Action

```go
type ScimitarAction struct {
    id          string
    attackBonus int
    damageDice  string
    damageType  damage.Type
    scoring     ScoringConfig
}

// Implement core.Entity
func (s *ScimitarAction) GetID() string            { return s.id }
func (s *ScimitarAction) GetType() core.EntityType { return "monster-action" }

// Implement core.Action[MonsterActionInput]
func (s *ScimitarAction) CanActivate(ctx context.Context, owner core.Entity, input MonsterActionInput) error {
    // Need a target
    if input.Target == nil {
        return rpgerr.New(rpgerr.CodeInvalidArgument, "no target")
    }

    // Target must be adjacent (5ft reach) - use GameCtx.Room for distance
    targetPos := input.GameCtx.Room.GetPosition(input.Target.GetID())
    myPos := input.GameCtx.Room.GetPosition(owner.GetID())
    dist := spatial.Distance(myPos, targetPos)
    if dist > 5 {
        return rpgerr.New(rpgerr.CodeOutOfRange, "target not in melee range")
    }

    return nil
}

func (s *ScimitarAction) Activate(ctx context.Context, owner core.Entity, input MonsterActionInput) error {
    if err := s.CanActivate(ctx, owner, input); err != nil {
        return err
    }

    monster := owner.(*Monster)

    // Use existing attack resolution
    // NOTE: Raging bonus is NOT added here - the RagingCondition subscribes
    // to DamageChain and adds its bonus via chain.Add(StageFeatures, ...)
    // This is the same pattern as characters.
    result, err := combat.ResolveAttack(ctx, &combat.AttackInput{
        Attacker:    monster,
        Defender:    input.Target,
        AttackBonus: s.attackBonus,
        DamageDice:  s.damageDice,
        DamageType:  s.damageType,
        EventBus:    input.Bus,
        Roller:      input.Roller,
    })
    if err != nil {
        return err
    }

    // Consume action from economy
    input.ActionEconomy.UseAction()

    return nil
}

// Implement MonsterAction
func (s *ScimitarAction) Cost() ActionCost     { return CostAction }
func (s *ScimitarAction) ActionType() ActionType { return TypeMeleeAttack }

func (s *ScimitarAction) Score(monster *Monster, perception *PerceptionData) int {
    score := s.scoring.BaseScore  // 50

    // Bonus if target adjacent
    if perception.HasAdjacentEnemy() {
        score += 20
    }

    return score
}
```

**Important:** Conditions like Raging modify damage via event chain subscription, not by the action checking for conditions. This is the same pattern used for characters.

### Example: Nimble Escape (Disengage)

```go
type NimbleEscapeDisengage struct {
    id      string
    scoring ScoringConfig
}

func (n *NimbleEscapeDisengage) CanActivate(ctx context.Context, owner core.Entity, input MonsterActionInput) error {
    // Always available as long as we have bonus action
    return nil
}

func (n *NimbleEscapeDisengage) Activate(ctx context.Context, owner core.Entity, input MonsterActionInput) error {
    // Apply "disengaged" condition - prevents opportunity attacks
    disengaged := conditions.NewDisengaged(owner.GetID())

    topic := dnd5eEvents.ConditionAppliedTopic.On(input.Bus)
    return topic.Publish(ctx, dnd5eEvents.ConditionAppliedEvent{
        Target:    owner,
        Type:      "disengaged",
        Condition: disengaged,
    })
}

func (n *NimbleEscapeDisengage) Cost() ActionCost { return CostBonusAction }
func (n *NimbleEscapeDisengage) ActionType() ActionType { return TypeMovement }

func (n *NimbleEscapeDisengage) Score(monster *Monster, perception PerceptionData) int {
    score := n.scoring.BaseScore  // 30

    // Much more valuable when hurt
    if monster.HPPercent() < 50 {
        score += 40
    }

    // More valuable when surrounded
    adjacent := perception.AdjacentEnemyCount()
    score += adjacent * 20

    return score
}
```

### Loading Actions

```go
func LoadAction(data ActionData) (MonsterAction, error) {
    switch data.Ref.Value {
    case "scimitar":
        var config ScimitarConfig
        json.Unmarshal(data.Config, &config)
        return NewScimitarAction(config), nil

    case "nimble-escape-disengage":
        var config NimbleEscapeConfig
        json.Unmarshal(data.Config, &config)
        return NewNimbleEscapeDisengage(config), nil

    // ... more actions
    }
    return nil, rpgerr.New(rpgerr.CodeNotFound, "unknown action: "+data.Ref.Value)
}
```

---

## Runtime Monster Struct

The in-memory monster with bus reference and typed data.

```go
type Monster struct {
    // Identity
    id          string
    name        string
    monsterType string

    // Stats
    hitPoints    int
    maxHitPoints int
    armorClass   int
    abilityScores shared.AbilityScores
    speed        Speed
    senses       Senses

    // Actions (typed, ready to use)
    actions []Action

    // Features (wired to bus)
    features []features.Feature

    // Conditions (wired to bus)
    conditions []dnd5eEvents.ConditionBehavior

    // Inventory
    inventory []InventoryItem

    // Proficiencies
    proficiencies map[string]int  // skill -> bonus

    // Event bus wiring
    bus             events.EventBus
    subscriptionIDs []string
}
```

---

## LoadFromData (Wire It Up)

```go
// LoadFromData creates a Monster from persistent data and wires it to the bus
func LoadFromData(ctx context.Context, d *Data, bus events.EventBus) (*Monster, error) {
    if bus == nil {
        return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "event bus is required")
    }

    // 1. CREATE THE MONSTER with data
    m := &Monster{
        id:            d.ID,
        name:          d.Name,
        monsterType:   d.MonsterType,
        hitPoints:     d.HitPoints,
        maxHitPoints:  d.MaxHitPoints,
        armorClass:    d.ArmorClass,
        abilityScores: d.AbilityScores,
        speed:         loadSpeed(d.Speed),
        senses:        loadSenses(d.Senses),
        bus:           bus,
        subscriptionIDs: make([]string, 0),
    }

    // 2. LOAD ACTIONS (convert from data to runtime)
    m.actions = make([]Action, 0, len(d.Actions))
    for _, actionData := range d.Actions {
        action := loadAction(actionData)
        m.actions = append(m.actions, action)
    }

    // 3. LOAD FEATURES from JSON and wire to bus
    m.features = make([]features.Feature, 0, len(d.Features))
    for _, rawFeature := range d.Features {
        feature, err := features.LoadJSON(rawFeature)
        if err != nil {
            continue
        }
        // Wire feature to bus if it subscribes to events
        if subscriber, ok := feature.(events.Subscriber); ok {
            subscriber.Subscribe(ctx, bus)
        }
        m.features = append(m.features, feature)
    }

    // 4. LOAD CONDITIONS and wire to bus
    m.conditions = make([]dnd5eEvents.ConditionBehavior, 0, len(d.Conditions))
    for _, rawCondition := range d.Conditions {
        condition, err := conditions.LoadJSON(rawCondition)
        if err != nil {
            continue
        }
        if err := condition.Apply(ctx, bus); err != nil {
            _ = condition.Remove(ctx, bus)
            continue
        }
        m.conditions = append(m.conditions, condition)
    }

    // 5. LOAD PROFICIENCIES
    m.proficiencies = make(map[string]int)
    for _, prof := range d.Proficiencies {
        m.proficiencies[prof.Skill] = prof.Bonus
    }

    // 6. LOAD INVENTORY
    m.inventory = loadInventory(d.Inventory)

    // 7. WIRE MONSTER TO BUS (subscribe to combat events)
    if err := m.subscribeToEvents(ctx); err != nil {
        return nil, rpgerr.Wrapf(err, "failed to subscribe to events")
    }

    return m, nil
}
```

---

## Monster Subscribing to Events

```go
// subscribeToEvents subscribes the monster to gameplay events
func (m *Monster) subscribeToEvents(ctx context.Context) error {
    // Subscribe to damage received
    damageTopic := dnd5eEvents.DamageReceivedTopic.On(m.bus)
    subID, err := damageTopic.Subscribe(ctx, m.onDamageReceived)
    if err != nil {
        return err
    }
    m.subscriptionIDs = append(m.subscriptionIDs, subID)

    // Subscribe to healing received
    healingTopic := dnd5eEvents.HealingReceivedTopic.On(m.bus)
    subID, err = healingTopic.Subscribe(ctx, m.onHealingReceived)
    if err != nil {
        return err
    }
    m.subscriptionIDs = append(m.subscriptionIDs, subID)

    // Subscribe to condition applied/removed
    conditionApplied := dnd5eEvents.ConditionAppliedTopic.On(m.bus)
    subID, err = conditionApplied.Subscribe(ctx, m.onConditionApplied)
    if err != nil {
        return err
    }
    m.subscriptionIDs = append(m.subscriptionIDs, subID)

    return nil
}
```

---

## Initiative and Turn Order

Monsters are added to initiative the same way as characters.

### Adding Monsters to Turn Order

```go
// In encounter orchestrator - creating dungeon/starting combat
func (o *Orchestrator) CreateDungeon(ctx context.Context, input *CreateDungeonInput) (*CreateDungeonOutput, error) {
    entities := make(map[core.Entity]int)

    // Add characters
    for _, charID := range input.CharacterIDs {
        char, _ := o.charRepo.Get(ctx, charrepo.GetInput{ID: charID})
        dexMod := char.AbilityScores.Modifier(abilities.DEX)
        participant := initiative.NewParticipant(charID, "character")
        entities[participant] = dexMod
    }

    // Add monsters - same pattern
    for _, monsterData := range input.Monsters {
        monster, _ := monster.LoadFromData(ctx, monsterData, bus)
        dexMod := monster.AbilityScores().Modifier(abilities.DEX)
        participant := initiative.NewParticipant(monster.GetID(), "monster")
        entities[participant] = dexMod
    }

    // Roll initiative for everyone
    rolls := initiative.RollForOrder(entities, dice.NewRoller())
    initiativeOrder := extractOrder(rolls)
    tracker := initiative.New(initiativeOrder)

    // Store tracker data
    trackerData := tracker.ToData()
    // ... persist ...
}
```

### Turn Flow

```go
// EndTurn advances to next entity
func (o *Orchestrator) EndTurn(ctx context.Context, input *EndTurnInput) (*EndTurnOutput, error) {
    // Load initiative tracker
    tracker := initiative.FromData(encounterData.InitiativeData)

    // Advance to next entity
    nextEntity := tracker.Next()

    // Reset action economy for new turn
    movementRemaining := defaultMovementSpeed  // 30 feet

    // Persist updated state
    o.encRepo.Update(ctx, &encounterrepo.UpdateInput{
        InitiativeData:    tracker.ToData(),
        MovementRemaining: &movementRemaining,
    })

    return &EndTurnOutput{
        NextEntityID: nextEntity.GetID(),
        EntityType:   nextEntity.GetType(),  // "character" or "monster"
        Round:        tracker.Round(),
    }
}
```

### Monster Turn Detection

When `EndTurn` returns a monster, the orchestrator calls `TakeTurn`:

```go
// After EndTurn, check if it's a monster's turn
output, _ := o.EndTurn(ctx, input)

if output.EntityType == "monster" {
    // Load the monster
    monsterData := o.loadMonsterData(ctx, output.NextEntityID)
    monster, _ := monster.LoadFromData(ctx, monsterData, bus)

    // Create turn input
    turnInput := &monster.TurnInput{
        Bus:           bus,
        ActionEconomy: combat.NewActionEconomy(),  // Fresh 1/1/1
        GameCtx:       &GameContext{Room: room},
        Roller:        dice.NewRoller(),
    }

    // Execute monster turn
    result, _ := monster.TakeTurn(ctx, turnInput)

    // Auto-advance to next turn (monsters don't wait for player input)
    o.EndTurn(ctx, &EndTurnInput{EncounterID: encounterID})
}
```

---

## Taking a Turn (Behavior)

Game server calls `TakeTurn` when it's the monster's turn, providing everything needed:

### TurnInput (What the game server provides)

```go
type TurnInput struct {
    Bus           events.EventBus
    ActionEconomy *combat.ActionEconomy  // Fresh 1/1/1 + movement
    GameCtx       *GameContext           // Room access for spatial queries
    Roller        dice.Roller
}

type GameContext struct {
    Room spatial.Room  // Query for targets, positions, pathfinding, cover
}
```

### TurnResult (What the monster returns)

```go
type TurnResult struct {
    MonsterID string
    Actions   []ExecutedAction  // What actions were taken
    Movement  []Position        // Path moved
}

type ExecutedAction struct {
    ActionID   string
    ActionType ActionType
    TargetID   string
    Success    bool
    Details    any  // Attack result, healing amount, etc.
}
```

### The TakeTurn Loop

```go
func (m *Monster) TakeTurn(ctx context.Context, input *TurnInput) (*TurnResult, error) {
    result := &TurnResult{
        MonsterID: m.id,
        Actions:   make([]ExecutedAction, 0),
    }

    // Build perception from room data
    perception := m.buildPerception(input.GameCtx)

    // Keep selecting actions until resources exhausted
    for input.ActionEconomy.HasResources() {
        // Score all valid actions
        best := m.selectBestAction(input.ActionEconomy, perception)
        if best == nil {
            break  // No valid actions
        }

        // Select target for this action
        target := m.selectTarget(best, perception)

        // Build action input
        actionInput := &MonsterActionInput{
            Bus:           input.Bus,
            Perception:    perception,
            Conditions:    m.conditions,
            ActionEconomy: input.ActionEconomy,
            Target:        target,
            GameCtx:       input.GameCtx,
            Roller:        input.Roller,
        }

        // Execute the action
        err := best.Activate(ctx, m, actionInput)

        // Record result
        result.Actions = append(result.Actions, ExecutedAction{
            ActionID:   best.GetID(),
            ActionType: best.ActionType(),
            TargetID:   target.GetID(),
            Success:    err == nil,
        })

        // Action economy updated by the action itself
    }

    return result, nil
}
```

### Building Perception from Room

```go
func (m *Monster) buildPerception(gameCtx *GameContext) *PerceptionData {
    myPos := gameCtx.Room.GetPosition(m.id)
    entities := gameCtx.Room.GetEntities()

    perception := &PerceptionData{
        MyPosition: myPos,
        Enemies:    make([]PerceivedEntity, 0),
        Allies:     make([]PerceivedEntity, 0),
    }

    for _, entity := range entities {
        if entity.GetID() == m.id {
            continue
        }

        pos := gameCtx.Room.GetPosition(entity.GetID())
        dist := spatial.Distance(myPos, pos)

        // Skip if outside perception range
        if dist > m.senses.PerceptionRange {
            continue
        }

        perceived := PerceivedEntity{
            Entity:   entity,
            Position: pos,
            Distance: dist,
            Adjacent: dist <= 5,
        }

        if isEnemy(m, entity) {
            perception.Enemies = append(perception.Enemies, perceived)
        } else {
            perception.Allies = append(perception.Allies, perceived)
        }
    }

    // Sort enemies by distance (closest first)
    sort.Slice(perception.Enemies, func(i, j int) bool {
        return perception.Enemies[i].Distance < perception.Enemies[j].Distance
    })

    return perception
}
```

### Selecting Best Action

```go
func (m *Monster) selectBestAction(economy *combat.ActionEconomy, perception *PerceptionData) MonsterAction {
    var best MonsterAction
    bestScore := -1000

    for _, action := range m.actions {
        // Can we afford it?
        if !economy.CanAfford(action.Cost()) {
            continue
        }

        // Score it
        score := action.Score(m, perception)
        if score > bestScore {
            bestScore = score
            best = action
        }
    }

    return best
}
```

### Target Selection

```go
func (m *Monster) selectTarget(action MonsterAction, perception *PerceptionData) core.Entity {
    switch action.ActionType() {
    case TypeMeleeAttack:
        // Closest enemy for melee
        if len(perception.Enemies) > 0 {
            return perception.Enemies[0].Entity
        }
    case TypeRangedAttack:
        // Closest enemy not adjacent (avoid disadvantage)
        for _, e := range perception.Enemies {
            if !e.Adjacent {
                return e.Entity
            }
        }
        // Fall back to closest
        if len(perception.Enemies) > 0 {
            return perception.Enemies[0].Entity
        }
    case TypeHeal:
        // Self
        return m
    }
    return nil
}
```

---

## Goblin Example (Complete Data)

```json
{
  "id": "goblin-1",
  "name": "Goblin",
  "monster_type": "goblin",

  "hit_points": 7,
  "max_hit_points": 7,
  "armor_class": 15,

  "ability_scores": {
    "str": 8, "dex": 14, "con": 10, "int": 10, "wis": 8, "cha": 8
  },

  "speed": { "walk": 30 },

  "senses": {
    "darkvision": 60,
    "passive_perception": 9
  },

  "actions": [
    {
      "id": "scimitar",
      "name": "Scimitar",
      "action_cost": "action",
      "action_type": "melee_attack",
      "range": { "type": "reach", "normal": 5 },
      "attack_bonus": 4,
      "damage": [{ "dice": "1d6+2", "damage_type": "slashing" }],
      "scoring": {
        "base_score": 50,
        "modifiers": [
          { "condition": "target_adjacent", "adjustment": 20 }
        ]
      }
    },
    {
      "id": "shortbow",
      "name": "Shortbow",
      "action_cost": "action",
      "action_type": "ranged_attack",
      "range": { "type": "ranged", "normal": 80, "long": 320 },
      "attack_bonus": 4,
      "damage": [{ "dice": "1d6+2", "damage_type": "piercing" }],
      "scoring": {
        "base_score": 50,
        "modifiers": [
          { "condition": "target_in_range", "value": 80, "adjustment": 20 },
          { "condition": "target_adjacent", "adjustment": -100 }
        ]
      }
    },
    {
      "id": "nimble-escape-disengage",
      "name": "Nimble Escape (Disengage)",
      "action_cost": "bonus_action",
      "action_type": "movement",
      "range": { "type": "self" },
      "applies_condition": "disengaged",
      "scoring": {
        "base_score": 30,
        "modifiers": [
          { "condition": "hp_below", "value": 50, "adjustment": 40 },
          { "condition": "enemies_adjacent", "value": 1, "adjustment": 20 },
          { "condition": "enemies_adjacent", "value": 2, "adjustment": 40 }
        ]
      }
    },
    {
      "id": "nimble-escape-hide",
      "name": "Nimble Escape (Hide)",
      "action_cost": "bonus_action",
      "action_type": "stealth",
      "range": { "type": "self" },
      "applies_condition": "hidden",
      "scoring": {
        "base_score": 30,
        "modifiers": [
          { "condition": "cover_available", "adjustment": 30 },
          { "condition": "has_ranged_attack", "adjustment": 20 },
          { "condition": "target_adjacent", "adjustment": -100 }
        ]
      }
    }
  ],

  "features": [],

  "conditions": [],

  "proficiencies": [
    { "skill": "stealth", "bonus": 6 }
  ]
}
```

---

## Resolved Questions

1. **Perception Data** — Built from room via `buildPerception()`. Contains enemies, allies, positions, distances.

2. **Movement** — Part of action execution. Actions use `GameCtx.Room` for pathfinding and spatial queries. Action economy tracks movement remaining.

3. **Actions as core.Action[T]** — Monster actions implement `core.Action[MonsterActionInput]` with `Score()` for behavior.

4. **Senses** — Keep simple for now. Just perception range. Darkvision etc. can come later.

5. **Game Server Role** — Creates bus, wires entities, calls `monster.TakeTurn(ctx, input)` with fresh action economy and game context.

## Open Questions

1. **ToData()** — Need the reverse: serialize monster back to Data for persistence.

2. **Enemy Detection** — How does `isEnemy()` work? Team/faction system?

3. **Cover Detection** — For Hide scoring, how do we query for nearby cover?

---

## Summary

**Monster is self-contained data + actions.** The game server orchestrates:

```
Game Server (rpg-api)
    │
    ├─ Creates EventBus for encounter
    ├─ Loads all entities, wires to bus
    ├─ Tracks turn order via initiative
    │
    └─ On monster's turn:
        ├─ Create fresh ActionEconomy (1/1/1 + movement)
        ├─ Create TurnInput with bus, economy, GameCtx
        └─ Call monster.TakeTurn(ctx, input)
            │
            ├─ Build perception from room
            ├─ Loop: score actions, pick best, execute
            └─ Return TurnResult
```

**Key patterns used:**
- `core.Action[MonsterActionInput]` for actions
- Same Data/LoadFromData pattern as characters
- Same condition/feature wiring pattern
- Spatial module for room queries and pathfinding

## Next Steps (Implementation)

1. Create `monster` package in `rulebooks/dnd5e/monster/`
2. Implement `Data` struct and `LoadFromData`
3. Create simple goblin with scimitar action
4. Wire up to encounter orchestrator
5. Test: goblin finds target, moves, attacks
