# Journey 024: Data-Driven Runtime Architecture

## Date: 2025-08-12

## The Core Principle

**Everything in the toolkit must be capable via data at runtime.**

No compile-time knowledge. No type checking for specific features. No hardcoded "if barbarian then rage". Just data, interfaces, and runtime discovery.

## The Problem We're Solving

Traditional RPG implementations are full of compile-time coupling:
```go
// DON'T DO THIS
if character.Class == "barbarian" {
    character.AddRageFeature()
}

// DON'T DO THIS
switch spell.Name {
case "fireball":
    return HandleFireball(spell)
case "heal":
    return HandleHeal(spell)
}

// DON'T DO THIS
if feature, ok := f.(*RageFeature); ok {
    feature.ActivateRage()
}
```

This doesn't scale. Every new feature requires code changes. Every new spell needs a new case. The toolkit becomes brittle and inflexible.

## The Vision

The toolkit should be a **runtime engine** that:
1. Loads data (features, spells, items, monsters)
2. Provides interfaces for interaction
3. Orchestrates behavior through events
4. Never needs to know what specific thing it's dealing with

## Examples of the Pattern

### Features (Already Designed)
```go
// Loading - toolkit doesn't know what features exist
features := []Feature{}
for _, featureJSON := range characterData.Features {
    feature, _ := LoadFeatureFromJSON(featureJSON)  // Could be anything
    features = append(features, feature)
}

// Activation - toolkit doesn't know what feature it is
func (c *Character) ActivateFeature(ref string) error {
    for _, feature := range c.features {
        if feature.Ref().String() == ref {
            return feature.Activate(c)  // Feature handles itself
        }
    }
    return ErrNotFound
}
```

### Spells (Following the Pattern)
```go
// Loading - data driven
spells := []Spell{}
for _, spellJSON := range spellbookData.Spells {
    spell, _ := LoadSpellFromJSON(spellJSON)  // Any spell
    spells = append(spells, spell)
}

// Casting - interface driven
func (c *Character) CastSpell(ref string, level int, target Entity) error {
    spell := c.spellbook.Find(ref)
    if spell == nil {
        return ErrSpellNotKnown
    }
    return spell.Cast(c, level, target)  // Spell handles itself
}
```

### Items (Following the Pattern)
```go
// Items are just data with behaviors
type Item interface {
    Ref() *core.Ref
    Equip(owner Entity) error
    Unequip(owner Entity) error
    Use(owner Entity, target Entity) error
}

// Character doesn't know what items do
func (c *Character) UseItem(itemRef string, target Entity) error {
    item := c.inventory.Find(itemRef)
    if item == nil {
        return ErrItemNotFound
    }
    return item.Use(c, target)  // Item handles itself
}
```

### Monsters (Following the Pattern)
```go
// Monsters are entities with abilities
type Monster interface {
    Entity
    GetActions() []Action
    TakeTurn(context GameContext) error
}

// Combat doesn't know what monsters do
func RunCombat(monsters []Monster) {
    for _, monster := range monsters {
        monster.TakeTurn(context)  // Monster AI handles itself
    }
}
```

## The Key Abstraction Layers

### 1. Data Layer (Persistence)
- JSON/Database storage
- No behavior, just state
- Version-agnostic

### 2. Domain Layer (Game Rules)
- Features, Spells, Items, Monsters
- Implement their own behaviors
- Loaded from data

### 3. Toolkit Layer (Infrastructure)
- Event bus
- Entity management  
- Resource tracking
- Spatial systems
- **Never knows specifics**

### 4. Application Layer (Orchestration)
- Load data
- Wire up systems
- Handle player input
- Persist state

## The Benefits

1. **Extensibility** - Add new content without code changes
2. **Moddability** - Users can add custom content
3. **Testability** - Test with mock data
4. **Reusability** - Same toolkit for any RPG
5. **Maintainability** - Clear separation of concerns

## The Rules

### ✅ DO
- Define clear interfaces
- Load from data
- Use runtime discovery
- Let objects handle themselves
- Communicate through events

### ❌ DON'T
- Type check for specific implementations
- Hardcode feature/spell/item names
- Create feature-specific methods
- Couple toolkit to game rules
- Make assumptions about content

## Real-World Example: Discord Bot

```go
// Command handler - fully data driven
func HandleCommand(cmd string, args []string) error {
    player := LoadPlayer(userID)
    
    switch cmd {
    case "cast":
        // We don't know what spell
        return player.CastSpell(args[0], parseLevel(args[1]), parseTarget(args[2]))
        
    case "activate":
        // We don't know what feature
        return player.ActivateFeature(args[0])
        
    case "use":
        // We don't know what item
        return player.UseItem(args[0], parseTarget(args[1]))
    }
}

// No spell-specific commands
// No feature-specific logic
// Just data and interfaces
```

## The Test

Can you add a completely new game system (Pathfinder, Starfinder, etc.) without changing toolkit code?

If yes, we've succeeded.

## Implementation Strategy

1. **Start with interfaces** - Define behavior contracts
2. **Load from data** - JSON first, database later
3. **Use factories** - Turn data into behavior
4. **Embrace events** - Decouple through messaging
5. **Trust the pattern** - Resist the urge to special-case

## Common Pitfalls to Avoid

### The "Just This Once" Trap
"Let's just check if it's Fireball, just this once..."
No. Every exception breaks the pattern.

### The "Performance" Excuse
"Dispatch through interfaces is slower..."
Premature optimization. Games aren't CPU-bound here.

### The "Type Safety" Argument
"We lose compile-time checking..."
True, but we gain runtime flexibility. Worth it.

## Success Criteria

When implementing a new RPG system:
1. Zero changes to toolkit code
2. Only data files and rule modules
3. Everything works through existing interfaces
4. No special cases in core systems

## The Philosophy

**The toolkit is a stage, not a play.**

It provides the infrastructure for any story, but doesn't know the script. Features, spells, items, and monsters are the actors - they know their parts. The toolkit just makes sure everyone gets their turn in the spotlight.

## Connection to Other Patterns

This principle reinforces:
- **Journey 008**: Features are just data with behavior
- **Journey 023**: Spells don't verify slots, systems do
- **Event-driven architecture**: Decoupling through messages
- **Interface segregation**: Small, focused contracts

## The Bottom Line

If you're writing code that checks what something IS rather than what it CAN DO, you're breaking the pattern.

Ask "What can this do?" not "What is this?"