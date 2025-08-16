# Journey 027: Actions as Internal Implementation Pattern (Clean Version)

## Date: 2025-08-16

## The Core Insight

Actions are an **internal implementation pattern** within the rulebook, not part of the public API. This gives us type safety and consistency internally while keeping the public interface clean and intuitive.

## The Architecture Layers

### 1. Game Server (rpg-api)
- Knows it's implementing D&D 5e
- Uses repository pattern for storage
- Imports rulebook directly
- Uses rulebook's typed constants

### 2. Rulebook Public API
- Provides data contracts (CharacterData)
- Defines typed constants (FeatureID, ResourceKey)
- Exposes clean interfaces (Character, Feature, Spell)

### 3. Rulebook Internal
- Uses Action[T] pattern for consistency
- Never exposes actions to game server
- Handles all game-specific logic

### 4. Toolkit Infrastructure
- Provides base types (EventType, ResourceKey)
- Event bus, resources, effects
- Knows nothing about specific games

## The Correct Implementation

### Game Server (rpg-api)

```go
package orchestrators

import (
    "context"
    "fmt"
    
    "github.com/KirkDiggler/rpg-api/internal/repositories/character"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

type CharacterOrchestrator struct {
    repo     character.Repository
    rulebook *dnd5e.Rulebook
}

// LoadCharacterInput follows the pattern
type LoadCharacterInput struct {
    CharacterID string
    PlayerID    string
}

type LoadCharacterOutput struct {
    Character *dnd5e.Character
}

func (o *CharacterOrchestrator) LoadCharacter(ctx context.Context, input *LoadCharacterInput) (*LoadCharacterOutput, error) {
    if input.CharacterID == "" {
        return nil, fmt.Errorf("character ID required")
    }
    
    // Get from repository
    getOutput, err := o.repo.Get(ctx, &character.GetInput{
        ID: input.CharacterID,
    })
    if err != nil {
        return nil, fmt.Errorf("get character: %w", err)
    }
    
    // Convert to rulebook character data (already typed!)
    charData := dnd5e.CharacterData{
        ID:        getOutput.Character.ID,
        Name:      getOutput.Character.Name,
        Level:     getOutput.Character.Level,
        Resources: getOutput.Character.Resources, // map[dnd5e.ResourceKey]int
        Features:  getOutput.Character.Features,  // []dnd5e.FeatureID
    }
    
    // Load through rulebook
    char, err := o.rulebook.LoadCharacter(ctx, charData)
    if err != nil {
        return nil, fmt.Errorf("load character: %w", err)
    }
    
    return &LoadCharacterOutput{
        Character: char,
    }, nil
}

// ActivateFeatureInput for player actions
type ActivateFeatureInput struct {
    CharacterID string
    FeatureName string  // User input like "rage"
}

type ActivateFeatureOutput struct {
    Success bool
    Message string
}

func (o *CharacterOrchestrator) ActivateFeature(ctx context.Context, input *ActivateFeatureInput) (*ActivateFeatureOutput, error) {
    if input.CharacterID == "" {
        return nil, fmt.Errorf("character ID required")
    }
    
    if input.FeatureName == "" {
        return nil, fmt.Errorf("feature name required")
    }
    
    // Load character
    loadOutput, err := o.LoadCharacter(ctx, &LoadCharacterInput{
        CharacterID: input.CharacterID,
    })
    if err != nil {
        return nil, fmt.Errorf("load character: %w", err)
    }
    
    char := loadOutput.Character
    
    // Find feature by user input
    feature := char.GetFeature(input.FeatureName)
    if feature == nil {
        return nil, fmt.Errorf("feature not found: %s", input.FeatureName)
    }
    
    // Check if can activate
    if err := feature.CanActivate(ctx); err != nil {
        return &ActivateFeatureOutput{
            Success: false,
            Message: fmt.Sprintf("Cannot activate %s: %v", feature.Name(), err),
        }, nil
    }
    
    // Activate it
    if err := feature.Activate(ctx); err != nil {
        return nil, fmt.Errorf("activate feature: %w", err)
    }
    
    return &ActivateFeatureOutput{
        Success: true,
        Message: fmt.Sprintf("%s activated!", feature.Name()),
    }, nil
}
```

### Rulebook Public API

```go
// rulebooks/dnd5e/types.go
package dnd5e

import (
    "context"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// Typed constants game server uses
type FeatureID string
type SpellID string
type ResourceKey resources.ResourceKey

const (
    // Features
    Rage        FeatureID = "rage"
    SecondWind  FeatureID = "second_wind"
    ActionSurge FeatureID = "action_surge"
    
    // Resources
    RageUses       ResourceKey = "rage_uses"
    SecondWindUses ResourceKey = "second_wind_uses"
    
    // Spells
    Fireball     SpellID = "fireball"
    CureWounds   SpellID = "cure_wounds"
)

// Data contract for game server
type CharacterData struct {
    ID       string
    Name     string
    Level    int
    
    // Already typed!
    Resources map[ResourceKey]int
    Features  []FeatureID
    Spells    []SpellID
}

// Clean public interfaces
type Character interface {
    // Identity
    ID() string
    Name() string
    
    // Features - by string for user input
    GetFeature(name string) Feature
    GetFeatureByID(id FeatureID) Feature
    ListFeatures() []Feature
    
    // Resources
    GetResource(key ResourceKey) int
    
    // Event bus access
    Activate(ctx context.Context, bus events.EventBus) error
}

type Feature interface {
    ID() FeatureID
    Name() string
    Description() string
    
    // State
    IsActive() bool
    UsesRemaining() int
    
    // Activation
    CanActivate(ctx context.Context) error
    Activate(ctx context.Context) error
    Deactivate(ctx context.Context) error
}

type Spell interface {
    ID() SpellID
    Name() string
    Level() int
    
    // Casting
    CanCast(ctx context.Context, input CastInput) error
    Cast(ctx context.Context, input CastInput) (*CastResult, error)
}

type CastInput struct {
    SlotLevel int
    Targets   []string  // Entity IDs
}

type CastResult struct {
    Success bool
    Damage  int
}
```

### Rulebook Internal Implementation

```go
// rulebooks/dnd5e/features/rage/rage.go
package rage

import (
    "context"
    "fmt"
    
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/actions"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

// Public feature type
type RageFeature struct {
    id       dnd5e.FeatureID
    owner    dnd5e.Character
    bus      events.EventBus
    
    // Internal - not exposed
    action   actions.Action[actions.EmptyInput]
    isActive bool
}

// Load creates rage from JSON data
func Load(owner dnd5e.Character, data json.RawMessage, bus events.EventBus) (*RageFeature, error) {
    rage := &RageFeature{
        id:    dnd5e.Rage,
        owner: owner,
        bus:   bus,
    }
    
    // Create internal action
    rage.action = &rageAction{
        owner: owner,
        bus:   bus,
    }
    
    return rage, nil
}

// Public API methods

func (f *RageFeature) ID() dnd5e.FeatureID {
    return f.id
}

func (f *RageFeature) Name() string {
    return "Rage"
}

func (f *RageFeature) IsActive() bool {
    return f.isActive
}

func (f *RageFeature) UsesRemaining() int {
    return f.owner.GetResource(dnd5e.RageUses)
}

func (f *RageFeature) CanActivate(ctx context.Context) error {
    if f.isActive {
        return fmt.Errorf("already raging")
    }
    
    if f.UsesRemaining() <= 0 {
        return fmt.Errorf("no rage uses remaining")
    }
    
    return nil
}

func (f *RageFeature) Activate(ctx context.Context) error {
    if err := f.CanActivate(ctx); err != nil {
        return err
    }
    
    // Use internal action
    if err := f.action.Execute(ctx, f.owner, actions.EmptyInput{}, f.bus); err != nil {
        return fmt.Errorf("execute rage: %w", err)
    }
    
    f.isActive = true
    return nil
}

// Internal action implementation

type rageAction struct {
    owner dnd5e.Character
    bus   events.EventBus
}

// Implements actions.Action[actions.EmptyInput]
func (a *rageAction) Execute(ctx context.Context, actor events.Entity, input actions.EmptyInput, bus events.EventBus) error {
    // Consume resource
    current := a.owner.GetResource(dnd5e.RageUses)
    a.owner.SetResource(dnd5e.RageUses, current-1)
    
    // Create rage effects
    damageBonus := &rageDamageBonus{
        owner: a.owner,
        value: calculateRageDamage(a.owner.Level()),
    }
    
    resistance := &rageResistance{
        owner: a.owner,
    }
    
    // Apply effects to bus
    if err := damageBonus.Apply(ctx, bus); err != nil {
        return fmt.Errorf("apply damage bonus: %w", err)
    }
    
    if err := resistance.Apply(ctx, bus); err != nil {
        return fmt.Errorf("apply resistance: %w", err)
    }
    
    // Publish activation event
    return bus.Publish(ctx, dnd5e.FeatureActivated, &FeatureActivatedEvent{
        Feature: dnd5e.Rage,
        Actor:   a.owner,
    })
}

// Rage effects

type rageDamageBonus struct {
    owner dnd5e.Character
    value int
}

func (e *rageDamageBonus) Apply(ctx context.Context, bus events.EventBus) error {
    return bus.Subscribe(dnd5e.DamageRoll, func(ctx context.Context, event events.Event) error {
        // Only our attacks
        if event.Source() != e.owner {
            return nil
        }
        
        // Only melee
        attackType, ok := event.Context().GetString("attack_type")
        if !ok || attackType != "melee" {
            return nil
        }
        
        // Add rage bonus
        event.Context().AddModifier(events.NewModifier(
            "rage",
            events.ModifierDamageBonus,
            e.value,
        ))
        
        return nil
    })
}
```

## Key Patterns

### Early Return on Errors

```go
// ✅ GOOD - Early return
feature := char.GetFeature(name)
if feature == nil {
    return nil, fmt.Errorf("feature not found: %s", name)
}

// Continue with feature...
return feature.Activate(ctx)
```

### Input/Output Types Everywhere

```go
// ✅ GOOD - Always use Input/Output
func (s *Service) CreateCharacter(ctx context.Context, input *CreateCharacterInput) (*CreateCharacterOutput, error)

// ❌ BAD - Never raw parameters
func (s *Service) CreateCharacter(ctx context.Context, name string, level int) (*Character, error)
```

### Repository Pattern

```go
// ✅ GOOD - Repository with typed inputs
getOutput, err := repo.Get(ctx, &character.GetInput{
    ID: characterID,
})
if err != nil {
    return nil, fmt.Errorf("get character: %w", err)
}

// ❌ BAD - Direct database access
var char Character
err := db.Get(&char, "SELECT * FROM characters WHERE id = ?", id)
```

## Benefits of This Architecture

1. **Clean Separation** - Each layer has clear responsibilities
2. **Type Safety** - Constants prevent typos, generics prevent assertions
3. **Testability** - Mock at interface boundaries
4. **Flexibility** - Internal implementation can change
5. **Intuitive API** - Game server code reads naturally

## What We Learned

1. **Actions are internal** - Implementation detail, not API
2. **Errors communicate** - Use them to provide context
3. **Repository pattern works** - Clean separation of storage
4. **Constants everywhere** - Type safety from top to bottom
5. **Input/Output always** - Even for single parameters

## Next Steps

1. Implement the Action[T] interface in toolkit
2. Add missing base types to toolkit (ResourceKey, etc.)
3. Create working Rage feature with internal action
4. Validate the public API feels right
5. Test that the patterns scale to complex features

The architecture is solid. We just needed to understand the proper layering and where each pattern belongs.