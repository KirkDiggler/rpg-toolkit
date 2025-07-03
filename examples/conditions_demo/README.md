# Conditions Demo

This example demonstrates the RPG Toolkit's conditions system, including:

- **Basic Conditions**: Status effects that modify game mechanics
- **Concentration**: Spells/abilities that require focus to maintain
- **Relationship Management**: How conditions relate to their sources
- **Aura Effects**: Range-based conditions that affect nearby entities

## Running the Demo

```bash
go run main.go
```

## Key Concepts Demonstrated

### 1. Concentration Mechanics
- Casters can only concentrate on one spell at a time
- Starting a new concentration spell breaks the previous one
- Losing concentration (from damage, etc.) removes all related conditions

### 2. Condition Relationships
The demo shows several relationship types:
- **Concentration**: One active effect per caster
- **Aura**: Effects based on proximity to source
- **Channeled**: Would require continuous actions (not shown)
- **Maintained**: Would cost resources per turn (not shown)

### 3. Event Integration
Conditions integrate with the event system to:
- Modify attack rolls (Bless adds d4)
- Prevent actions (Hold Person paralyzes)
- React to game events

## Architecture Notes

- Conditions are Entities (can be persisted/queried)
- Conditions track their own event subscriptions
- RelationshipManager handles complex interactions
- Clean separation between mechanics and rulebook implementations