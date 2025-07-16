# Direct Replacement Guide: DND Bot → RPG Toolkit

This guide shows how to directly replace DND bot systems with rpg-toolkit, without adapters.

## Overview

Instead of creating adapters, we directly replace the bot's systems with toolkit equivalents:

1. **Event Bus** → Use `events.Bus` directly
2. **Spell Slots** → Use `resources.Pool`
3. **Conditions** → Use `conditions.Manager`
4. **Proficiencies** → Use `proficiency.Manager`

## Step 1: Replace Event Bus

### Before (DND Bot)
```go
// internal/domain/game/event_bus.go
type EventBus struct {
    listeners map[EventType][]EventListener
}

func (eb *EventBus) Subscribe(eventType EventType, listener EventListener) {
    eb.listeners[eventType] = append(eb.listeners[eventType], listener)
}

func (eb *EventBus) Publish(eventType EventType, data interface{}) {
    for _, listener := range eb.listeners[eventType] {
        listener(Event{Type: eventType, Data: data})
    }
}
```

### After (Direct Toolkit Usage)
```go
// internal/services/events.go
package services

import (
    "context"
    "github.com/KirkDiggler/rpg-toolkit/events"
)

// Just use toolkit's bus directly
func NewEventBus() *events.Bus {
    return events.NewBus()
}

// In your service initialization:
type GameService struct {
    eventBus *events.Bus  // Direct toolkit usage
}

func NewGameService() *GameService {
    return &GameService{
        eventBus: events.NewBus(),
    }
}

// Subscribe to events
func (s *GameService) Init() {
    s.eventBus.SubscribeFunc(events.EventOnAttackRoll, 100, s.handleAttackRoll)
    s.eventBus.SubscribeFunc(events.EventOnDamageRoll, 100, s.handleDamageRoll)
}

func (s *GameService) handleAttackRoll(ctx context.Context, e events.Event) error {
    // Direct toolkit event handling
    weapon, _ := e.Context().GetString("weapon")
    
    // Add modifiers using toolkit's system
    e.Context().AddModifier(events.NewModifier(
        "proficiency",
        events.ModifierAttackBonus,
        events.NewRawValue(3, "proficiency"),
        100,
    ))
    
    return nil
}
```

## Step 2: Replace Spell Slot System

### Before (DND Bot)
```go
type Character struct {
    SpellSlots map[int]int // level -> slots
}

func (c *Character) UseSpellSlot(level int) bool {
    if c.SpellSlots[level] > 0 {
        c.SpellSlots[level]--
        return true
    }
    return false
}
```

### After (Direct Toolkit Usage)
```go
import (
    "github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

type Character struct {
    // Other fields...
    spellSlots map[int]*resources.Pool // Direct toolkit pools
}

func (c *Character) InitSpellSlots() {
    c.spellSlots = make(map[int]*resources.Pool)
    
    // Create pools for each spell level
    for level := 1; level <= 9; level++ {
        slots := c.getSpellSlotsForLevel(level)
        if slots > 0 {
            pool, _ := resources.NewPool(resources.PoolConfig{
                Name:     fmt.Sprintf("spell-slots-%d", level),
                MaxValue: slots,
                MinValue: 0,
                Current:  slots,
            })
            c.spellSlots[level] = pool
        }
    }
}

func (c *Character) UseSpellSlot(level int) bool {
    pool, exists := c.spellSlots[level]
    if !exists {
        return false
    }
    
    // Direct toolkit API
    return pool.Consume(1)
}

func (c *Character) RestoreSpellSlots(restType string) {
    for level, pool := range c.spellSlots {
        if restType == "long" {
            pool.SetCurrent(pool.GetMax()) // Full restore
        } else if restType == "short" && level <= 5 {
            // Warlock pact magic example
            pool.SetCurrent(pool.GetMax())
        }
    }
}
```

## Step 3: Replace Condition System

### Before (DND Bot)
```go
type Character struct {
    Conditions []Condition
}

func (c *Character) AddCondition(name string) {
    c.Conditions = append(c.Conditions, Condition{Name: name})
}

func (c *Character) HasCondition(name string) bool {
    for _, cond := range c.Conditions {
        if cond.Name == name {
            return true
        }
    }
    return false
}
```

### After (Direct Toolkit Usage)
```go
import (
    "github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
)

type Character struct {
    // Other fields...
    conditions *conditions.Manager // Direct toolkit manager
}

func (c *Character) InitConditions(eventBus *events.Bus) {
    c.conditions = conditions.NewManager(
        conditions.WithEventBus(eventBus),
        conditions.WithEntity(c), // Character implements core.Entity
    )
}

func (c *Character) AddCondition(name string) error {
    // Direct toolkit API
    return c.conditions.Apply(conditions.Condition{
        Name:     name,
        Type:     conditions.Debuff,
        Duration: conditions.UntilRemoved,
    })
}

func (c *Character) HasCondition(name string) bool {
    // Direct toolkit API
    return c.conditions.Has(name)
}

func (c *Character) RemoveCondition(name string) {
    // Direct toolkit API
    c.conditions.Remove(name)
}
```

## Step 4: Replace Proficiency System

### Before (DND Bot)
```go
type Character struct {
    Proficiencies map[string]bool
}

func (c *Character) IsProficient(item string) bool {
    return c.Proficiencies[item]
}

func (c *Character) GetProficiencyBonus() int {
    return 2 + ((c.Level - 1) / 4)
}
```

### After (Direct Toolkit Usage)
```go
import (
    "github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
)

type Character struct {
    // Other fields...
    proficiencies *proficiency.Manager // Direct toolkit manager
}

func (c *Character) InitProficiencies(eventBus *events.Bus) {
    c.proficiencies = proficiency.NewManager(
        proficiency.WithEventBus(eventBus),
        proficiency.WithEntity(c),
    )
    
    // Add character's proficiencies
    for _, prof := range c.getProficiencyList() {
        c.proficiencies.Add(proficiency.Proficiency{
            Name: prof,
            Type: proficiency.Skill, // or Weapon, Tool, etc.
        })
    }
}

func (c *Character) IsProficient(item string) bool {
    // Direct toolkit API
    return c.proficiencies.Has(item)
}

// Proficiency bonus is still calculated the same way
func (c *Character) GetProficiencyBonus() int {
    return 2 + ((c.Level - 1) / 4)
}
```

## Step 5: Update Combat Service

### Before (DND Bot)
```go
type CombatService struct {
    eventBus *EventBus
}

func (s *CombatService) Attack(attacker, target *Character, weapon string) {
    // Manual event handling
    event := AttackEvent{
        Attacker: attacker.ID,
        Target:   target.ID,
        Weapon:   weapon,
    }
    
    // Add modifiers manually
    if attacker.IsProficient(weapon) {
        event.AttackBonus += attacker.GetProficiencyBonus()
    }
    
    s.eventBus.Publish(EventAttackRoll, event)
}
```

### After (Direct Toolkit Usage)
```go
type CombatService struct {
    eventBus *events.Bus // Direct toolkit bus
}

func (s *CombatService) Attack(ctx context.Context, attacker, target *Character, weapon string) error {
    // Create toolkit event
    event := events.NewGameEvent(events.EventOnAttackRoll, attacker, target)
    event.Context().Set("weapon", weapon)
    
    // Proficiency is handled by event subscribers
    return s.eventBus.Publish(ctx, event)
}

// Event handler adds proficiency automatically
func (s *CombatService) handleAttackRoll(ctx context.Context, e events.Event) error {
    attacker := e.Source().(*Character)
    weapon, _ := e.Context().GetString("weapon")
    
    if attacker.IsProficient(weapon) {
        bonus := attacker.GetProficiencyBonus()
        e.Context().AddModifier(events.NewModifier(
            "proficiency",
            events.ModifierAttackBonus,
            events.NewRawValue(bonus, "proficiency"),
            100,
        ))
    }
    
    return nil
}
```

## Migration Steps

1. **Replace one system at a time**
   - Start with event bus (foundation for other systems)
   - Then resources (spell slots)
   - Then conditions
   - Finally proficiencies

2. **Update imports**
   ```go
   // Remove
   import "github.com/yourusername/dnd-bot/internal/domain/game"
   
   // Add
   import (
       "github.com/KirkDiggler/rpg-toolkit/events"
       "github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
       "github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
       "github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
   )
   ```

3. **Update service initialization**
   ```go
   func NewGameService() *GameService {
       eventBus := events.NewBus()
       
       return &GameService{
           eventBus: eventBus,
           // Initialize other services with the same bus
       }
   }
   ```

4. **Update character initialization**
   ```go
   func (c *Character) Init(eventBus *events.Bus) {
       c.InitSpellSlots()
       c.InitConditions(eventBus)
       c.InitProficiencies(eventBus)
   }
   ```

## Benefits of Direct Replacement

1. **No adapter overhead** - Use toolkit APIs directly
2. **Full toolkit features** - Modifiers, event context, priorities
3. **Cleaner code** - Remove duplicate implementations
4. **Better testing** - Toolkit has comprehensive tests
5. **Automatic features** - Event cascading, modifier stacking

## Example: Complete Character Update

```go
// internal/domain/character.go
package domain

import (
    "context"
    "fmt"
    
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

type Character struct {
    ID     string
    Name   string
    Level  int
    Class  string
    
    // Direct toolkit managers
    spellSlots    map[int]*resources.Pool
    conditions    *conditions.Manager
    proficiencies *proficiency.Manager
}

// Implement core.Entity
func (c *Character) GetID() string   { return c.ID }
func (c *Character) GetType() string { return "character" }

// Initialize all toolkit systems
func (c *Character) Init(eventBus *events.Bus) error {
    // Initialize spell slots
    c.spellSlots = make(map[int]*resources.Pool)
    for level := 1; level <= 9; level++ {
        slots := c.getSpellSlotsForLevel(level)
        if slots > 0 {
            pool, err := resources.NewPool(resources.PoolConfig{
                Name:     fmt.Sprintf("spell-slots-%d", level),
                MaxValue: slots,
                Current:  slots,
            })
            if err != nil {
                return err
            }
            c.spellSlots[level] = pool
        }
    }
    
    // Initialize conditions
    c.conditions = conditions.NewManager(
        conditions.WithEventBus(eventBus),
        conditions.WithEntity(c),
    )
    
    // Initialize proficiencies
    c.proficiencies = proficiency.NewManager(
        proficiency.WithEventBus(eventBus),
        proficiency.WithEntity(c),
    )
    
    return nil
}

// Direct toolkit API usage
func (c *Character) CastSpell(ctx context.Context, spellLevel int, eventBus *events.Bus) error {
    // Check spell slot
    pool, exists := c.spellSlots[spellLevel]
    if !exists || !pool.CanConsume(1) {
        return fmt.Errorf("no spell slots available for level %d", spellLevel)
    }
    
    // Consume slot
    if !pool.Consume(1) {
        return fmt.Errorf("failed to consume spell slot")
    }
    
    // Publish event
    event := events.NewGameEvent(events.EventOnSpellCast, c, nil)
    event.Context().Set("spell_level", spellLevel)
    event.Context().Set("remaining_slots", pool.GetCurrent())
    
    return eventBus.Publish(ctx, event)
}

func (c *Character) LongRest() {
    // Restore all spell slots
    for _, pool := range c.spellSlots {
        pool.SetCurrent(pool.GetMax())
    }
    
    // Remove conditions that end on long rest
    for _, cond := range c.conditions.GetAll() {
        if cond.Duration == conditions.UntilLongRest {
            c.conditions.Remove(cond.Name)
        }
    }
}
```

This approach directly replaces the bot's systems with toolkit equivalents, no adapters needed!