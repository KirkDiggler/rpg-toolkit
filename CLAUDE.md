# RPG Toolkit Development Guidelines

## Slash Commands for Common Workflows

Use these slash commands to follow structured workflows:

- `/bugfix` - Complete bug fix workflow (branch creation, failing test, fix, PR)
- `/feature` - Feature development workflow (TDD, implementation, documentation, PR)

These commands guide you through best practices and ensure nothing is missed.

## Module Development Workflow

**IMPORTANT: NO go.work FILES OR LOCAL REPLACE DIRECTIVES**

This project uses a clean module development approach:

1. **Each module is developed independently**
   - Work on one module at a time
   - Use published dependencies only
   - No replace directives pointing to local paths
   - No go.work files

2. **Dependency Management**
   - Modules reference published versions (e.g., `v0.1.0`)
   - When you need updates from another module:
     - Push the changes to that module first
     - Then `go get -u` in the dependent module
   - During development, Go creates pseudo-versions automatically (e.g., `v0.0.0-20230907052031-37f5183ecf93`)

3. **Why This Approach**
   - Keeps development honest - you work with real APIs
   - Focuses work on one module at a time
   - Avoids local development issues and CI failures
   - Clear dependency tracking
   - No confusion about which version is being used

## Testing Strategy

### Preferred Testing Approach: Testify Suite Pattern

**Use testify suite for clean test organization**:

```go
type ServiceTestSuite struct {
    suite.Suite
    service  *Service
    mockDep  *MockDependency
    testData *TestData
}

// SetupTest runs before EACH test function
func (s *ServiceTestSuite) SetupTest() {
    // Create mocks - fresh for each test
    s.mockDep = NewMockDependency(s.T())
    s.service = NewService(&ServiceConfig{
        Dependency: s.mockDep,
    })
    // Initialize common test data
    s.testData = createTestData()
}

// SetupSubTest runs before EACH s.Run()
func (s *ServiceTestSuite) SetupSubTest() {
    // Reset test data to clean state for each subtest
    s.testData = createTestData()
    // Can also reset specific mock expectations if needed
}

// Run the suite
func TestServiceSuite(t *testing.T) {
    suite.Run(t, new(ServiceTestSuite))
}
```

**Key Testing Principles**:
- Use `suite.Suite` for test organization
- Use `s.Run()` for subtests with test cases
- `SetupTest()` runs before each test function - establish mocks here
- `SetupSubTest()` runs before each `s.Run()` - reset test data here
- Keep test bodies focused on arrange/act/assert
- Use suite assertions: `s.Assert()`, `s.Require()`

### Testing Commands

When working on a module:
```bash
# Run tests
go test ./...

# Run linter
golangci-lint run ./...

# Update dependencies
go get -u ./...
go mod tidy
```

## Pre-commit Checks

The repository has comprehensive pre-commit hooks that run:
- Formatting (gofmt, goimports)
- go mod tidy
- Linting
- Tests

These run automatically on commit.

## Development Principles

- **Optimize for simplicity, not hypothetical future needs**
- **Only add what is necessary**
- **Pick ONE way to represent data and use it directly**
- **Avoid conversion layers and dual representations**
- **Delete code that creates unnecessary indirection**

## Project Philosophy

**IMPORTANT: RPG Toolkit provides infrastructure, NOT implementation**

1. **Generic Tools, Not Game Rules**
   - We provide the infrastructure for game mechanics
   - Games implement their specific rules using our tools
   - Example: We provide proficiency infrastructure, games define what "Acrobatics" means

2. **Event-Driven Architecture**
   - Modules communicate through events, not direct calls
   - Use the event bus for all inter-module communication
   - This allows maximum flexibility for game implementations

3. **Entity-Based Design**
   - All game objects implement core.Entity interface
   - Entities have ID and Type
   - This provides consistent patterns across the toolkit

## Current Project Status

### Completed Modules
1. **core** - Base interfaces and types
2. **events** - Event bus system for module communication
3. **dice** - Dice rolling infrastructure
4. **mechanics/conditions** - Status effects and conditions
5. **mechanics/proficiency** - Proficiency system
6. **mechanics/effects** - Shared infrastructure for conditions/proficiencies
7. **mechanics/resources** - Resource management (spell slots, abilities, etc.)
8. **tools/spatial** - Complete spatial positioning system with multi-room orchestration
9. **tools/spawn** - Complete entity spawn engine with Phases 1-4 implementation per ADR-0013

### Spatial Module Features (Completed)
- **Grid Systems**: Square (D&D 5e), Hex, and Gridless positioning
- **Room Management**: Entity placement, movement, and spatial queries
- **Multi-Room Orchestration**: Connection system, layout patterns, entity transitions
- **Event Integration**: Full event-driven architecture
- **Query System**: Efficient spatial queries with filtering
- **Connection Types**: Doors, stairs, passages, portals, bridges, tunnels
- **Layout Patterns**: Tower, branching, grid, and organic arrangements

### Pending Work (Issues #31-#33)
1. **Equipment System (#31)** - Items, inventory, equip/unequip mechanics
2. **Enhanced Conditions (#32)** - Advanced condition features
3. **Feature System (#33)** - Character features and traits

### Important Patterns
1. **Config Pattern**: Use config structs for constructors
2. **Composition Over Inheritance**: Use embedded structs and interfaces
3. **Error Handling**: Always check errors in tests with require.NoError(t, err)
4. **Event Naming**: Use dot notation (e.g., "resource.consumed", "condition.applied")
5. **JSON Serialization Pattern**: See below

### Feature/Condition Serialization Pattern

**IMPORTANT: Typed Data Structs for JSON**

Features and Conditions use a JSON-in/JSON-out pattern where:
- The **game server (rpg-api)** stores conditions/features as **opaque JSON blobs** - it doesn't know internal structure
- The **toolkit** is responsible for marshaling JSON into **strongly-typed structs**

**Pattern:**
```go
// Data struct for serialization - uses core.Ref for routing
type RagingData struct {
    Ref               core.Ref `json:"ref"`
    CharacterID       string   `json:"character_id"`
    DamageBonus       int      `json:"damage_bonus"`
    // ... other fields
}

// Runtime struct - no JSON tags needed
type RagingCondition struct {
    CharacterID string
    DamageBonus int
    // ... other fields + non-serialized runtime state
}

// ToJSON serializes to typed struct
func (r *RagingCondition) ToJSON() (json.RawMessage, error) {
    data := RagingData{
        Ref: core.Ref{Module: "dnd5e", Type: "conditions", Value: "raging"},
        CharacterID: r.CharacterID,
        // ...
    }
    return json.Marshal(data)
}

// loadJSON deserializes from typed struct
func (r *RagingCondition) loadJSON(data json.RawMessage) error {
    var ragingData RagingData
    if err := json.Unmarshal(data, &ragingData); err != nil {
        return err
    }
    r.CharacterID = ragingData.CharacterID
    // ...
    return nil
}
```

**Loader routes by ref:**
```go
func LoadJSON(data json.RawMessage) (ConditionBehavior, error) {
    var peek struct { Ref core.Ref `json:"ref"` }
    json.Unmarshal(data, &peek)

    switch peek.Ref.Value {
    case "raging":
        c := &RagingCondition{}
        c.loadJSON(data)
        return c, nil
    // ...
    }
}
```

**Key Benefits:**
- Type-safe serialization (not `map[string]interface{}`)
- Clear separation between runtime and serialized state
- Game server doesn't need to understand toolkit internals
- Easy to add new condition/feature types

### Recent Architectural Decisions
- **ADR-0005**: Extract shared effect infrastructure from conditions/proficiencies
- **Journey 005**: Documents the discovery of duplicate code and extraction pattern
- **Dice Modifiers**: Need fresh rolls each time (e.g., Bless adds 1d4 per attack)
- **ADR-0009**: Multi-room orchestration architecture (extend spatial module with architectural validation)
- **Journey 013**: Multi-room orchestration implementation complete with thread safety and type safety

### Multi-Room Orchestrator Usage
The spatial module now includes multi-room orchestration capabilities:

```go
// Create orchestrator
orchestrator := spatial.NewBasicRoomOrchestrator(spatial.BasicRoomOrchestratorConfig{
    ID:       "dungeon-orchestrator",
    Type:     "orchestrator",
    EventBus: eventBus,
    Layout:   spatial.LayoutTypeOrganic,
})

// Add rooms
orchestrator.AddRoom(room1)
orchestrator.AddRoom(room2)

// Create connections
door := spatial.CreateDoorConnection("door-1", "room-1", "room-2", 
    spatial.Position{X: 9, Y: 5}, spatial.Position{X: 0, Y: 5})
orchestrator.AddConnection(door)

// Move entities between rooms
orchestrator.MoveEntityBetweenRooms("hero", "room-1", "room-2", "door-1")
```

**Key Features**:
- Connection types: doors, stairs, passages, portals, bridges, tunnels
- Layout patterns: tower, branching, grid, organic
- Entity tracking across rooms
- Event-driven architecture
- Pathfinding between connected rooms

### Spawn Module Features (Completed)
- **Phase 1**: Basic spawn engine infrastructure with entity selection and pattern support
- **Phase 2**: Advanced patterns (formation, team-based, player choice, clustered spawning)
- **Phase 3**: Constraint system (spatial constraints, line of sight, area of effect, wall proximity)
- **Phase 4**: Environment integration (capacity analysis, room scaling, split recommendations)
- **Cross-Cutting**: Event system, split-aware architecture, gridless room support, configuration validation

### Development Workflow Reminders

**Git Workflow (see /home/kirk/personal/CLAUDE.md for full details):**
```bash
gcm                           # Switch to main
gl                            # Pull latest changes
gcb fix/issue-number          # Create new feature branch
# ... make changes, run tests ...
git add .
git commit -m "Description"
git push -u origin fix/issue-number
gh pr create                  # Create PR
```

**Development checklist:**
1. Always check existing patterns in similar modules
2. Read Journey and ADR docs before implementing new features
3. Never create files unless necessary - prefer editing existing ones
4. Run the full test suite before committing (`go test ./...`)
5. Run linter before committing (`golangci-lint run ./...`)
6. Use `gh pr create` for PRs with proper formatting

### Critical Module Isolation Rules
**LEARNED FROM PR #76 TROUBLESHOOTING**

1. **NEVER touch other modules when working on a specific module**
   - Other modules are READ-ONLY for reference
   - If other modules have issues, create separate PRs
   - Don't run `go mod tidy` or similar commands in other modules

2. **Be extremely careful with troubleshooting commands**
   - Always check current directory before running go commands
   - Don't run bulk operations across all modules unless absolutely necessary
   - Accidental `go mod tidy` in wrong modules can corrupt dependencies

3. **Focus on actual changes, not CI configuration**
   - When CI fails, check what files were actually changed first
   - Don't assume CI configuration issues - often it's code conflicts
   - Look for accidentally committed files (like stray modules without go.mod)

4. **Type conflicts from orphaned modules**
   - Files without go.mod get treated as part of root workspace
   - Can cause type conflicts with existing modules
   - Always ensure new modules have proper go.mod or remove them entirely

## AI Assistant Guidelines

**CRITICAL: NO ASSUMPTIONS WITHOUT VERIFICATION**

1. **Research Before Acting**
   - Never make assumptions about tool versions, compatibility, or technical specifications
   - Always research and verify facts before providing commands or instructions
   - Use web search, documentation, or other verification methods when uncertain

2. **Explicit Assumption Declaration**
   - If you must make an assumption, explicitly state: "I'm making an assumption here that..."
   - Explain what you're assuming and why
   - Suggest verification steps the user can take

3. **Version Compatibility**
   - Always check actual compatibility matrices for tools and dependencies
   - Don't assume version support without verification
   - When in doubt, recommend checking official documentation

4. **Error Recovery**
   - When corrected, acknowledge the mistake clearly
   - Update long-term memory (this file) with correct information
   - Learn from the correction to avoid similar errors

5. **Context Disambiguation**
   - **CRITICAL**: This project uses two different "context" concepts
   - **Go Context**: Standard `context.Context` for cancellation, timeouts, request-scoped values
   - **Event Context**: Custom `events.Context` for game data (damage, modifiers, entities)
   - Always specify which context you're referring to
   - Use Go context only where cancellation/timeouts are genuinely needed
   - Remove unused Go context parameters from internal functions
   - Use event context for game data flow between event handlers

## CI Protection Guidelines

**CRITICAL: ALL PUBLIC APIS MUST BE DOCUMENTED**

Follow these patterns to avoid CI failures:

### Documentation Requirements
1. **All Public Functions** must have comments explaining what they do:
   ```go
   // NewBasicTable creates a new weighted selection table with the specified configuration
   func NewBasicTable[T any](config BasicTableConfig) *BasicTable[T] {
   ```

2. **All Public Types** must have comments explaining their purpose:
   ```go
   // SelectionTable provides weighted random selection for any content type
   // Purpose: Core interface for all grabbag/loot table functionality
   type SelectionTable[T any] interface {
   ```

3. **All Public Constants** must have comments explaining their meaning:
   ```go
   // SelectionModeUnique prevents duplicate selections in multi-item rolls
   const SelectionModeUnique SelectionMode = "unique"
   ```

4. **All Public Variables** must have comments explaining their purpose:
   ```go
   // ErrEmptyTable indicates an attempt to select from a table with no items
   var ErrEmptyTable = errors.New("selection table contains no items")
   ```

### Documentation Patterns from Environments Package
Based on `/home/frank/projects/rpg-toolkit/tools/environments/`:

1. **Multi-line purpose explanations**:
   ```go
   // ConstraintType categorizes different kinds of generation constraints
   // Purpose: Allows the generator to handle different constraint types appropriately.
   // Some constraints affect placement, others affect connections, etc.
   type ConstraintType int
   ```

2. **Constructor functions**:
   ```go
   // NewBasicEnvironment creates a new environment with the specified configuration
   // Purpose: Standard constructor with config struct, proper initialization
   func NewBasicEnvironment(config BasicEnvironmentConfig) *BasicEnvironment {
   ```

3. **Interface method documentation**:
   ```go
   // GetID returns the unique identifier for this environment
   func (e *BasicEnvironment) GetID() string {
   
   // SetTheme changes the visual and atmospheric theme of the environment.
   // Purpose: Allows dynamic environment appearance changes during gameplay
   func (e *BasicEnvironment) SetTheme(theme string) error {
   ```

### Implementation Rules
1. **NO functions that only return nil** - CI will fail
2. **NO empty function bodies** - implement meaningful functionality or document why it's intentionally empty
3. **ALL public methods** must have accompanying comments
4. **Follow toolkit naming patterns** - see existing code for conventions

### Error Handling Patterns
```go
// SelectMany selects multiple items from the table with the specified count
// Returns ErrEmptyTable if the table contains no items
// Returns ErrInvalidCount if count is less than 1
func (t *BasicTable[T]) SelectMany(ctx SelectionContext, count int) ([]T, error) {
    if len(t.items) == 0 {
        return nil, ErrEmptyTable
    }
    if count < 1 {
        return nil, ErrInvalidCount
    }
    // ... implementation
}
```

### Pre-Implementation Checklist
Before writing any public API:
1. ✅ Function/type has descriptive comment
2. ✅ Comment explains purpose and behavior  
3. ✅ Error cases documented in comment
4. ✅ Function has meaningful implementation (not just `return nil`)
5. ✅ Follows existing toolkit patterns