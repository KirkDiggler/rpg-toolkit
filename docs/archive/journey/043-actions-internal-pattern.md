# Journey 026: Actions as Internal Implementation Pattern

## Date: 2025-08-16

## The Correction

We got excited about the Action pattern and almost exposed it everywhere. But through discussion, we realized: **Actions are an internal implementation detail of the rulebook, not part of the public API.**

## The Real Architecture

### Layer 1: Game Server
- Knows it's implementing D&D 5e (or Pathfinder, etc.)
- Imports the rulebook directly
- Uses rulebook's data contracts
- Stores and retrieves typed data

### Layer 2: Rulebook (Public API)
- Provides data structures (CharacterData)
- Provides interfaces (Character, Spell, Feature)
- Defines typed constants (SpellID, FeatureID, ResourceKey)
- Controls the contracts

### Layer 3: Rulebook (Internal Implementation)
- Uses Action pattern internally for consistency
- Each feature/spell has an Action[T]
- Never exposes actions to game server
- Handles all D&D 5e specific logic

### Layer 4: Toolkit
- Provides infrastructure (EventBus, Resources, Effects)
- Defines base types (EventType, ResourceKey, etc.)
- Knows nothing about specific games

## The Key Realization

```go
// What we almost did (WRONG)
type GameServer struct {
    // Game server dealing with actions? NO!
    action := character.GetAction("rage")
    action.Execute(...)
}

// What we should do (RIGHT)
type GameServer struct {
    // Game server uses feature/spell interfaces
    feature := character.GetFeature("rage")
    feature.Activate(ctx)
    
    spell := character.GetSpell("fireball")
    spell.Cast(ctx, castInput)
}
```

## The Correct Flow

### 1. Game Server Perspective

```go
import "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"

type GameServer struct {
    rulebook *dnd5e.Rulebook
    db       Database
}

func (gs *GameServer) LoadCharacter(playerID string) (*dnd5e.Character, error) {
    // Load typed struct from database
    var data dnd5e.CharacterData
    gs.db.Get(&data, "SELECT * FROM characters WHERE id = ?", playerID)
    
    // data.Resources[dnd5e.RageUses] = 3  // Already typed!
    // data.Features = []dnd5e.FeatureID{dnd5e.Rage, dnd5e.SecondWind}
    
    // Pass to rulebook
    return gs.rulebook.LoadCharacter(ctx, data)
}

func (gs *GameServer) HandlePlayerAction(ctx context.Context, playerID, action string) error {
    char := gs.GetCharacter(playerID)
    
    // Player says "rage"
    if feature := char.GetFeature(action); feature != nil {
        return feature.Activate(ctx)
    }
    
    // Player says "cast fireball"
    if spell := char.GetSpell(action); spell != nil {
        input := dnd5e.CastInput{
            SlotLevel: 3,
            Targets: gs.selectTargets(),
        }
        return spell.Cast(ctx, input)
    }
    
    return ErrUnknownAction
}
```

### 2. Rulebook Public API

```go
// rulebooks/dnd5e/character.go
package dnd5e

// Data contract - game server saves/loads this
type CharacterData struct {
    ID       string
    Name     string
    Level    int
    
    // Typed constants!
    Resources map[ResourceKey]int
    Features  []FeatureID        // Not JSON - actual types
    Spells    []SpellID
}

// Public interfaces
type Character interface {
    GetFeature(id string) Feature  // By string for user input
    GetSpell(id string) Spell
    GetResource(key ResourceKey) int
}

type Feature interface {
    ID() FeatureID
    Name() string
    CanActivate(ctx context.Context) error
    Activate(ctx context.Context) error
}

type Spell interface {
    ID() SpellID
    Name() string
    Level() int
    Cast(ctx context.Context, input CastInput) error
}

type CastInput struct {
    SlotLevel int      // What level to cast at
    Targets   []string // Entity IDs
}
```

### 3. Rulebook Internal Implementation

```go
// rulebooks/dnd5e/features/rage/rage.go
package rage

// Public feature that game server gets
type RageFeature struct {
    id     FeatureID
    owner  *Character
    
    // INTERNAL - not exposed
    action Action[EmptyInput]
}

// Public API
func (f *RageFeature) Activate(ctx context.Context) error {
    // Internally uses action pattern
    return f.action.Execute(ctx, f.owner, EmptyInput{}, f.owner.bus)
}

// Internal implementation
type rageAction struct {
    usesKey ResourceKey
}

// Implements Action[EmptyInput] - but game server never sees this
func (a *rageAction) Execute(ctx context.Context, actor Entity, input EmptyInput, bus EventBus) error {
    // Check resources
    if actor.GetResource(a.usesKey) <= 0 {
        return ErrNoUses
    }
    
    // Apply effects
    effects := []Effect{
        &RageDamageBonus{actor: actor},
        &RageResistance{actor: actor},
    }
    
    for _, e := range effects {
        e.Apply(ctx, bus)
    }
    
    return nil
}
```

```go
// rulebooks/dnd5e/spells/fireball/fireball.go
package fireball

// Public spell
type FireballSpell struct {
    id     SpellID
    level  int
    owner  *Character
    
    // INTERNAL
    action Action[CastInput]
}

// Public API
func (s *FireballSpell) Cast(ctx context.Context, input CastInput) error {
    // Internally uses action pattern
    return s.action.Execute(ctx, s.owner, input, s.owner.bus)
}

// Internal implementation
type fireballAction struct{}

// Implements Action[CastInput] - hidden from game server
func (a *fireballAction) Execute(ctx context.Context, actor Entity, input CastInput, bus EventBus) error {
    // Check spell slot
    slotKey := GetSpellSlotKey(input.SlotLevel)
    if actor.GetResource(slotKey) <= 0 {
        return ErrNoSpellSlot
    }
    
    // Create damage event
    for _, targetID := range input.Targets {
        damage := 8 * dice.D6()  // 8d6 fire damage
        event := &DamageEvent{
            Source: actor,
            Target: targetID,
            Amount: damage,
            Type:   FireDamage,
        }
        bus.Publish(ctx, DamageRoll, event)
    }
    
    return nil
}
```

## Why This Is Better

### 1. Clean Separation
- Game server uses simple, intuitive interfaces
- Rulebook handles all complexity internally
- Action pattern provides consistency within rulebook

### 2. Type Safety Without Exposure
- Actions use generics internally for type safety
- Game server never deals with type assertions
- Public API uses simple, clear types

### 3. Flexibility
- Each feature/spell can have different Action types internally
- Can change internal implementation without affecting game server
- Action pattern is opt-in, not required

## The Loading Flow

```go
// Features still load from JSON (variable content)
func (rb *Rulebook) LoadCharacter(ctx context.Context, data CharacterData) (*Character, error) {
    char := &Character{
        ID:        data.ID,
        Resources: data.Resources,  // Already typed constants
    }
    
    // Load features from JSON
    for _, featureID := range data.Features {
        featureData := rb.getFeatureData(featureID)  // JSON from somewhere
        
        switch featureID {
        case Rage:
            feature := rage.Load(char, featureData)
            char.features = append(char.features, feature)
        case SecondWind:
            feature := secondwind.Load(char, featureData)
            char.features = append(char.features, feature)
        }
    }
    
    return char, nil
}
```

## The Complete Picture

1. **Toolkit** provides generic infrastructure (events, resources, effects)
2. **Rulebook** defines the game (D&D 5e, Pathfinder, etc.)
   - Public: Clean interfaces for game server
   - Internal: Action pattern for consistency
3. **Game Server** implements a specific rulebook
   - Uses rulebook's data contracts
   - Calls rulebook's public methods
   - Never sees internal implementation

## Key Insights

1. **Actions are internal** - They're our implementation pattern, not API
2. **Rulebook controls contracts** - Game server uses what rulebook provides
3. **Features load from JSON** - But character structure is fixed
4. **Type safety throughout** - Constants everywhere, generics internally

## What This Enables

- Clean game server code that reads naturally
- Consistent internal implementation via Action pattern
- Type safety without complexity exposure
- Easy to test (mock the public interfaces)
- Easy to extend (add new features/spells)

## Next Steps

1. Implement Action[T] pattern in toolkit
2. Create example feature (Rage) using internal action
3. Create example spell (Fireball) using internal action
4. Validate the public API is clean and intuitive
5. Ensure game server code remains simple

The pattern is right, we just needed to understand where it belongs - inside the rulebook, not exposed to the world.