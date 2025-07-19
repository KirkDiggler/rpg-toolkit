# RPG Toolkit Development Guidelines

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

### Development Workflow Reminders
1. Always check existing patterns in similar modules
2. Read Journey and ADR docs before implementing new features
3. Never create files unless necessary - prefer editing existing ones
4. When creating PRs, use gh CLI with proper formatting
5. Run the full test suite before committing

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