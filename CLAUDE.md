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

## Testing Commands

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

### Development Workflow Reminders
1. Always check existing patterns in similar modules
2. Read Journey and ADR docs before implementing new features
3. Never create files unless necessary - prefer editing existing ones
4. When creating PRs, use gh CLI with proper formatting
5. Run the full test suite before committing