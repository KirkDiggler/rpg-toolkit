# Typed IDs and ActionEconomy Extensions

**Date**: 2026-01-09
**Status**: Approved
**Issue**: #550

## Summary

Add typed identifiers (via Refs) for Actions and extend ActionEconomy to support the two-level action economy system for rpg-api integration.

## Problem

The rpg-api needs typed identifiers to map proto enums to toolkit types without understanding game mechanics. Currently:
- CombatAbilities have Refs (Attack, Dash, Dodge, etc.)
- Actions do NOT have Refs (Move, Strike, OffHandStrike, etc.)
- ActionEconomy lacks capacity tracking for off-hand attacks and flurry strikes

## Solution

### 1. refs.Actions Namespace

Add `TypeActions` to `refs/module.go` and create `refs/actions.go`:

```go
refs.Actions.Move()          // *core.Ref{Module: "dnd5e", Type: "actions", ID: "move"}
refs.Actions.Strike()        // *core.Ref{...}
refs.Actions.OffHandStrike() // *core.Ref{...}
refs.Actions.FlurryStrike()  // *core.Ref{...}
refs.Actions.UnarmedStrike() // *core.Ref{...}
```

### 2. ActionData and LoadFromData

Actions share a common schema, so use `LoadFromData` instead of `LoadJSON`:

```go
type ActionData struct {
    Ref      *core.Ref
    ID       string
    OwnerID  string
    WeaponID string // empty for Move, populated for Strike variants
}

func LoadFromData(data ActionData) (Action, error)
```

### 3. ActionEconomy Extensions

Add capacity counters (not booleans for effects - use conditions for those):

```go
type ActionEconomy struct {
    // ... existing fields ...

    OffHandAttacksRemaining int  // Set by TwoWeaponGranter
    FlurryStrikesRemaining  int  // Set by FlurryOfBlows
}
```

### 4. MoveStopReason

Typed constants for why movement stopped:

```go
type MoveStopReason string

const (
    MoveStopReasonCompleted            MoveStopReason = "completed"
    MoveStopReasonInsufficientMovement MoveStopReason = "insufficient_movement"
    MoveStopReasonPositionOccupied     MoveStopReason = "position_occupied"
    MoveStopReasonBlockedByWall        MoveStopReason = "blocked_by_wall"
    MoveStopReasonInvalidCoordinates   MoveStopReason = "invalid_coordinates"
)
```

## Not Included

- `DodgeActive`/`DisengageActive` booleans - Use `character.HasCondition(refs.Conditions.Dodging())` instead

## Implementation Order

1. Add `TypeActions` to `refs/module.go`
2. Create `refs/actions.go` with Actions namespace
3. Create `actions/data.go` with ActionData and LoadFromData
4. Extend ActionEconomy with new capacity fields
5. Create `actions/move_stop_reason.go`
6. Update `rulebooks/dnd5e/CLAUDE.md` with Refs pattern documentation
