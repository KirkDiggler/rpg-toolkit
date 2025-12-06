# Action Economy Flow Diagrams

These diagrams show how action economy integrates with the combat system for issue #387.

## Basic Bonus Action Flow (Second Wind)

```mermaid
sequenceDiagram
    participant O as Orchestrator
    participant E as EventBus
    participant AE as ActionEconomy
    participant F as Feature (SecondWind)
    participant C as Character

    Note over O: Turn Starts
    O->>E: Publish TurnStartEvent
    E->>AE: Reset (ActionUsed=false, BonusUsed=false)
    E->>F: onTurnStart() - check triggers

    Note over O: Player wants to use Second Wind
    O->>F: CanActivate()?
    F->>F: Check resource available?
    F->>AE: CanUseBonusAction()?
    AE-->>F: true (not used yet)
    F-->>O: OK to activate

    O->>F: Activate()
    F->>F: Consume resource (uses remaining -1)
    F->>AE: RecordBonusAction("second_wind")
    AE->>AE: BonusActionUsed = true
    F->>E: Publish HealEvent (or HealChain)
    E->>C: Apply healing
    F-->>O: Success

    Note over O: Player tries another bonus action
    O->>F: CanActivate()?
    F->>AE: CanUseBonusAction()?
    AE-->>F: false (already used)
    F-->>O: Error: bonus action unavailable

    Note over O: Turn Ends
    O->>E: Publish TurnEndEvent
```

## Conditional Bonus Action Flow (Martial Arts)

Shows how a bonus action becomes available based on a trigger (making a monk weapon attack).

```mermaid
sequenceDiagram
    participant O as Orchestrator
    participant E as EventBus
    participant AE as ActionEconomy
    participant MA as MartialArts Feature
    participant C as Character

    Note over O: Turn Starts
    O->>E: Publish TurnStartEvent
    E->>AE: Reset all
    E->>MA: onTurnStart()
    MA->>MA: MadeMonkWeaponAttack = false

    Note over O: Monk attacks with quarterstaff
    O->>E: Publish AttackEvent
    E->>MA: onAttack()
    MA->>MA: Is monk weapon? Yes
    MA->>MA: MadeMonkWeaponAttack = true

    Note over O: Monk wants unarmed bonus strike
    O->>MA: CanActivate()?
    MA->>MA: MadeMonkWeaponAttack? Yes ✓
    MA->>AE: CanUseBonusAction()? Yes ✓
    MA-->>O: OK to activate

    O->>MA: Activate()
    MA->>AE: RecordBonusAction("martial_arts_strike")
    MA->>E: Publish AttackEvent (unarmed)
```

## Key Design Decisions

### ActionEconomy is State, Not Events

ActionEconomy tracks per-turn state:
- `ActionUsed bool`
- `BonusActionUsed bool`
- `ReactionUsed bool`

Events flow through it:
1. `TurnStartEvent` → resets ActionEconomy
2. Features query ActionEconomy for permission
3. Features update ActionEconomy when activated
4. `TurnEndEvent` → any cleanup

### Where ActionEconomy Lives

**Decision:** ActionEconomy lives in the `combat` package since:
- Action economy only matters during combat (turn-based)
- Free roam mode won't have action restrictions
- Combat is the container for turn-based mechanics

### Component Summary

| Component | Type | Purpose |
|-----------|------|---------|
| ActionEconomy struct | State | Track action/bonus/reaction used this turn |
| TurnStartEvent | Event | Already exists - triggers reset |
| TurnEndEvent | Event | Already exists - triggers cleanup |
| HealEvent/HealChain | Event | New - for healing flow |
| RestEvent | Event | New - triggers resource recovery |
| ResourceConsumedEvent | Event | New - for Ki/ability tracking |
