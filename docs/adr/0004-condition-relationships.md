# ADR-0004: Generic Condition Relationships

## Status
Accepted

## Context
After implementing the conditions system (ADR-0003), we needed to handle relationships between conditions and their sources. The initial implementation included a specific `ConcentrationManager` for D&D-style concentration mechanics.

However, we realized that concentration is just one type of relationship among many:
- Concentration (one spell per caster)
- Auras (range-based effects)
- Channeled abilities (require continuous actions)
- Maintained effects (cost resources over time)
- Linked conditions (must be removed together)
- Dependent conditions (require another to exist)

## Decision
We will implement a generic `RelationshipManager` that handles all types of condition relationships uniformly. Concentration becomes just one relationship type (`RelationshipConcentration`) rather than a first-class concept.

## Consequences
### Positive
- More flexible system supporting diverse rulebook mechanics
- No special-casing for specific game systems
- Easier to add new relationship types
- Consistent API for all relationship types

### Negative
- Slightly more complex than a dedicated concentration manager
- Rulebooks must understand relationship types to use them correctly

### Neutral
- Relationship types are extensible through the RelationshipType constants