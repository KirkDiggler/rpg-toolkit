# ADR-0008: Tools Directory Structure for Infrastructure Modules

Date: 2025-07-16

## Status

Accepted

## Context

During design of the spatial module (Issue #53), we identified that spatial positioning, grids, and rooms are foundational infrastructure that other game mechanics depend on, rather than being game mechanics themselves.

Currently, the project has two organizational patterns:
1. **Top-level modules** for foundational infrastructure: `core/`, `dice/`, `events/`
2. **Mechanics modules** for game mechanics: `mechanics/conditions/`, `mechanics/spells/`, etc.

The spatial module doesn't fit cleanly into either category:
- It's not foundational like `core/` or `events/` (those are used by everything)
- It's not a game mechanic like `conditions/` or `spells/` (those implement specific gameplay rules)
- It's infrastructure that enables game mechanics to work

We anticipate other similar infrastructure modules in the future:
- Time management (initiative, turn order, duration tracking)
- Inventory systems (item management, containers)
- Vision systems (line of sight, lighting)
- Movement systems (pathfinding, terrain)
- Targeting systems (range validation, area effects)

## Decision

Create a new `tools/` directory to organize infrastructure modules that:
1. Provide specialized infrastructure for game mechanics
2. Are not fundamental enough to be top-level modules
3. Are used by multiple game mechanics
4. Are infrastructure, not game mechanics themselves

Structure:
```
tools/
├── spatial/     # Positioning, grids, rooms, line-of-sight
├── time/        # Turn order, initiative, duration tracking (future)
├── inventory/   # Item management, containers (future)
├── vision/      # Line of sight, lighting (future)
├── movement/    # Pathfinding, movement (future)
└── targeting/   # Target selection, area effects (future)
```

This creates a clear three-tier architecture:
1. **Foundation** (`core/`, `dice/`, `events/`) - Used by everything
2. **Tools** (`tools/spatial/`, `tools/time/`, etc.) - Specialized infrastructure
3. **Mechanics** (`mechanics/conditions/`, `mechanics/spells/`, etc.) - Game mechanics

## Consequences

### Positive
- Clear architectural distinction between infrastructure tools and game mechanics
- Room for growth as we add more infrastructure modules
- Logical grouping of related infrastructure capabilities
- Consistent import patterns: `github.com/KirkDiggler/rpg-toolkit/tools/spatial`
- Keeps the root directory clean and organized
- Makes it obvious what type of functionality each module provides

### Negative
- Adds one more directory level to the project structure
- Slightly longer import paths for tools modules
- Developers need to understand the three-tier architecture

### Neutral
- Establishes a pattern for future infrastructure modules
- May require updating documentation to explain the new structure
- Could influence how we organize other aspects of the toolkit

## Example

```go
// Foundation modules (used by everything)
import "github.com/KirkDiggler/rpg-toolkit/core"
import "github.com/KirkDiggler/rpg-toolkit/events"

// Tools modules (specialized infrastructure)
import "github.com/KirkDiggler/rpg-toolkit/tools/spatial"
import "github.com/KirkDiggler/rpg-toolkit/tools/time"

// Mechanics modules (game mechanics)
import "github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
import "github.com/KirkDiggler/rpg-toolkit/mechanics/spells"
```

## Implementation Notes

1. Create `tools/spatial/` directory structure
2. Update import paths in spatial module design
3. Document the three-tier architecture in project README
4. Consider how this affects dependency management between modules
5. Update Issue #53 to reflect the new location

## Related
- Issue #53: Implement Spatial Module for RPG Toolkit
- Journey 012: Spatial Module Design
- Future infrastructure modules (time, inventory, vision, movement, targeting)