# Package Documentation Guide

Every package in the RPG Toolkit must have a clear `doc.go` file that defines its purpose, scope, and boundaries. This ensures the toolkit remains focused and prevents scope creep.

## Required Documentation Structure

Each package's `doc.go` must include:

1. **Purpose**: What problem does this package solve?
2. **Scope**: What is included in this package?
3. **Non-Goals**: What explicitly does NOT belong here?
4. **Integration Points**: How does it connect with other packages?
5. **Usage Examples**: Basic code examples

## Template

```go
// Package [name] provides [infrastructure/tools] for [specific purpose].
//
// Purpose:
// This package [solves X problem] by providing [Y infrastructure]. It is designed
// to be [game-agnostic/system-neutral] and provides only the [tools/interfaces]
// needed for [specific functionality].
//
// Scope:
// - [Included functionality 1]
// - [Included functionality 2]
// - [Included functionality 3]
//
// Non-Goals:
// - [Explicitly excluded 1]: This belongs in [where it belongs]
// - [Explicitly excluded 2]: This is game-specific logic
// - [Explicitly excluded 3]: This would couple the toolkit to [specific system]
//
// Integration:
// This package integrates with:
// - [package1]: For [purpose]
// - [package2]: For [purpose]
//
// Example:
//
//	// Basic usage example
//	[code example]
//
package [name]
```

## Package-Specific Documentation

### core/doc.go
```go
// Package core provides fundamental interfaces and types that define entities
// in the RPG toolkit ecosystem.
//
// Purpose:
// This package establishes the base contracts that all game entities must fulfill,
// providing identity and type information without imposing any game-specific
// attributes or behaviors.
//
// Scope:
// - Entity interface: Basic identity contract (ID, Type)
// - Error types: Common errors used across packages
// - No game logic, stats, or behaviors
//
// Non-Goals:
// - Game statistics (HP, AC, etc): These belong in game implementations
// - Entity behaviors: Use the behavior package for AI/actions
// - Persistence: Storage concerns belong in repository implementations
// - Game rules: All game-specific logic belongs in rulebooks
//
// Integration:
// This package is imported by all other toolkit packages as it defines
// the fundamental Entity contract. It has no dependencies on other
// toolkit packages, maintaining its position at the base of the hierarchy.
//
// Example:
//
//	type Monster struct {
//	    id   string
//	    kind string
//	}
//
//	func (m *Monster) ID() string   { return m.id }
//	func (m *Monster) Type() string { return m.kind }
//
package core
```

### events/doc.go
```go
// Package events provides a game-agnostic event bus for loose coupling between
// toolkit components and game systems.
//
// Purpose:
// This package enables components to communicate without direct dependencies,
// supporting observable and extensible game systems through event-driven architecture.
//
// Scope:
// - Event bus implementation with pub/sub
// - Event interface and base types
// - Typed event support with generics
// - Event filtering and routing
// - No game-specific event types
//
// Non-Goals:
// - Game event definitions: Define these in your game implementation
// - Event persistence: Use external storage if needed
// - Network transport: This is for in-process events only
// - Event ordering guarantees: Events are delivered best-effort
//
// Integration:
// - All packages can publish events without knowing subscribers
// - Game implementations subscribe to relevant toolkit events
// - Enables debugging and observation of system behavior
//
// Example:
//
//	bus := events.NewBus()
//
//	// Subscribe to movement events
//	bus.Subscribe("entity.moved", func(e events.Event) {
//	    fmt.Printf("Entity %s moved\n", e.Data["entityID"])
//	})
//
//	// Publish event
//	bus.Publish(events.New("entity.moved", map[string]any{
//	    "entityID": "goblin-1",
//	    "from": Position{10, 10},
//	    "to": Position{15, 10},
//	}))
//
package events
```

### dice/doc.go
```go
// Package dice provides cryptographically secure random number generation
// for RPG mechanics.
//
// Purpose:
// This package offers deterministic and non-deterministic dice rolling
// capabilities with modifier support, ensuring fair and unpredictable
// game outcomes when needed.
//
// Scope:
// - Dice notation parsing (e.g., "3d6+2")
// - Cryptographically secure random generation
// - Modifier system (bonuses, penalties)
// - Roll history and statistics
// - Deterministic rolling for tests
//
// Non-Goals:
// - Game-specific roll rules: Advantage/disadvantage belong in games
// - Roll result interpretation: Critical hits are game-specific
// - Probability calculations: Use external statistics packages
// - Dice UI/visualization: This is pure logic
//
// Integration:
// - Used by combat systems for attack/damage rolls
// - Used by skill systems for checks
// - Can be replaced with deterministic roller for testing
//
// Example:
//
//	roller := dice.NewCryptoRoller()
//
//	// Roll 3d6+2
//	result, err := roller.Roll("3d6+2")
//	fmt.Printf("Rolled %d (details: %v)\n", result.Total, result.Rolls)
//
//	// Roll with advantage (game implements this)
//	roll1 := roller.Roll("1d20")
//	roll2 := roller.Roll("1d20")
//	best := max(roll1.Total, roll2.Total)
//
package dice
```

### spatial/doc.go
```go
// Package spatial provides 2D positioning and movement infrastructure for
// entity placement and spatial queries.
//
// Purpose:
// This package handles all spatial mathematics, collision detection, and
// movement validation without imposing any game-specific movement rules
// or combat mechanics.
//
// Scope:
// - 2D coordinate system with configurable units
// - Grid support (square, hex, gridless)
// - Room-based spatial organization
// - Collision detection and spatial queries
// - Path validation (not pathfinding algorithms)
// - Multi-room orchestration and connections
//
// Non-Goals:
// - Movement rules: Speed, difficult terrain are game-specific
// - Line of sight rules: Cover mechanics belong in games
// - Pathfinding algorithms: Use behavior package for AI movement
// - Combat ranges: Weapon ranges are game-specific
// - 3D positioning: This is explicitly 2D only
//
// Integration:
// - behavior package uses this for movement validation
// - spawn package uses this for entity placement
// - Games query positions for range/area effects
//
// Example:
//
//	room := spatial.NewRoom(spatial.RoomConfig{
//	    Width:  40,
//	    Height: 30,
//	    Grid:   spatial.GridTypeSquare5ft,
//	})
//
//	// Place entity
//	err := room.Place("goblin-1", spatial.Position{X: 10, Y: 15})
//
//	// Query area
//	nearby := room.EntitiesWithin(spatial.Position{X: 10, Y: 15}, 10.0)
//
package spatial
```

### behavior/doc.go
```go
// Package behavior provides infrastructure for entity decision-making and
// action execution without implementing any specific behaviors.
//
// Purpose:
// This package establishes contracts for AI behavior systems, supporting
// multiple paradigms (state machines, behavior trees, utility AI) while
// remaining agnostic to specific game rules or creature behaviors.
//
// Scope:
// - Behavior interfaces and context
// - State machine infrastructure
// - Behavior tree node types
// - Perception system interfaces
// - Action types and constants
// - Memory management for behaviors
// - Decision event publishing
//
// Non-Goals:
// - Specific creature behaviors: Implement in game rulebooks
// - Combat tactics: Game-specific AI belongs in games
// - Pathfinding algorithms: May add A* infrastructure later
// - Behavior implementations: Only infrastructure here
// - Game state access: Behaviors receive filtered context
//
// Integration:
// - Uses spatial for position queries
// - Publishes events for decision observability
// - Games implement concrete behaviors
// - Rulebooks wire behaviors to entities
//
// Example:
//
//	// Game implements concrete behavior
//	type AggressiveBehavior struct{}
//
//	func (b *AggressiveBehavior) Execute(ctx BehaviorContext) (Action, error) {
//	    perception := ctx.GetPerception()
//	    nearest := findNearest(perception.VisibleEntities)
//	    
//	    return Action{
//	        Type: ActionTypeAttack,
//	        Target: nearest.ID(),
//	    }, nil
//	}
//
package behavior
```

### spawn/doc.go
```go
// Package spawn provides entity placement and population infrastructure
// for rooms and areas.
//
// Purpose:
// This package handles the placement of pre-created entities within spatial
// constraints, integrating with the selectables system for random selection
// while remaining agnostic to what is being spawned.
//
// Scope:
// - Entity placement with constraints
// - Spawn point management
// - Integration with selectables for random choice
// - Density calculations and space management
// - Placement validation and collision avoidance
//
// Non-Goals:
// - Entity creation: Entities must be pre-created
// - Spawn tables: Use selectables package directly
// - Creature stats: This only handles placement
// - Loot generation: Create items before spawning
// - Encounter balancing: CR/difficulty is game-specific
//
// Integration:
// - Uses spatial for placement validation
// - Uses selectables for random entity selection
// - Games provide entity pools and constraints
// - Works with room orchestrator for multi-room spawning
//
// Example:
//
//	engine := spawn.NewEngine(spawn.Config{})
//
//	result, err := engine.PopulateRoom(room, spawn.Request{
//	    EntityPools: map[string][]core.Entity{
//	        "monsters": {goblin1, goblin2, orc1},
//	        "treasure": {goldPile, potion},
//	    },
//	    Density: spawn.DensityModerate,
//	    Constraints: []spawn.Constraint{
//	        spawn.AvoidCenter{Radius: 10},
//	        spawn.NearFeature{Feature: "pillar"},
//	    },
//	})
//
package spawn
```

### selectables/doc.go
```go
// Package selectables provides weighted random selection infrastructure
// for loot tables, random encounters, and decision-making.
//
// Purpose:
// This package enables probability-based selection from weighted options,
// supporting everything from treasure generation to AI decision weighting
// without knowledge of what is being selected.
//
// Scope:
// - Weighted table creation and management
// - Probability-based selection algorithms
// - Table composition and nesting
// - Deterministic selection for testing
// - Statistical validation of weights
//
// Non-Goals:
// - Item definitions: Tables select IDs, not create items
// - Drop rate rules: Game-specific logic belongs elsewhere
// - Economy balancing: Value/rarity is game-specific
// - Specific loot tables: Games define their own tables
//
// Integration:
// - spawn package uses for entity selection
// - behavior package uses for action weighting
// - Games define specific selection tables
//
// Example:
//
//	table := selectables.NewTable[string]()
//	table.Add("goblin", 60)      // 60% chance
//	table.Add("orc", 30)         // 30% chance  
//	table.Add("troll", 10)       // 10% chance
//
//	selected := table.Select()   // Returns one of the options
//
package selectables
```

### environments/doc.go
```go
// Package environments provides procedural generation of rooms and areas
// using spatial primitives and environmental features.
//
// Purpose:
// This package generates physical spaces with rooms, corridors, and features
// while remaining agnostic to game-specific concepts like encounter design
// or narrative purpose.
//
// Scope:
// - Room shape generation (rectangle, L-shape, etc.)
// - Corridor and connection generation
// - Environmental feature placement
// - Graph-based layout algorithms
// - Room sizing based on capacity needs
//
// Non-Goals:
// - Encounter design: What spawns is game-specific
// - Trap mechanics: Game rules handle trap effects
// - Secret doors: Detection mechanics are game-specific
// - Narrative generation: Story beats belong in games
// - Dungeon ecology: Logical creature placement is game-specific
//
// Integration:
// - Builds on spatial for room creation
// - Provides rooms ready for spawn population
// - Games interpret generated spaces
//
// Example:
//
//	gen := environments.NewGenerator(environments.Config{
//	    Algorithm: environments.AlgorithmGraph,
//	})
//
//	dungeon := gen.Generate(environments.Parameters{
//	    RoomCount: 10,
//	    Connectivity: 0.7,
//	    RoomShapes: []string{"rectangle", "L", "cross"},
//	})
//
//	for _, room := range dungeon.Rooms {
//	    // Populate with spawn system
//	}
//
package environments
```

## Enforcement Guidelines

1. **Package Review Checklist**:
   - Does the package have a clear, single purpose?
   - Are the non-goals explicitly stated?
   - Is it free of game-specific logic?
   - Are integration points documented?

2. **Red Flags**:
   - Package imports game-specific constants
   - Package has knowledge of specific games/rulesets
   - Package makes assumptions about game rules
   - Package combines multiple unrelated concerns

3. **Extraction Process**:
   When extracting from game implementations:
   - Identify the infrastructure vs. game logic boundary
   - Extract only the game-agnostic parts
   - Leave game-specific behavior in the rulebook
   - Document what was intentionally left out

## Evolution Strategy

As patterns emerge in game implementations:

1. **Observe patterns** across multiple game systems
2. **Extract commonalities** that are truly game-agnostic
3. **Generalize carefully** to avoid coupling
4. **Document thoroughly** why something was added
5. **Maintain boundaries** even under pressure to add features

The toolkit grows through careful extraction, not eager addition.