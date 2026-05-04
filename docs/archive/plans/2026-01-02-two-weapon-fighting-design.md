# Two-Weapon Fighting Design

**Issue:** #508
**Date:** 2026-01-02
**Status:** Implemented

## Overview

Enable off-hand bonus action attacks for two-weapon fighting. The toolkit validates weapon requirements and manages action economy - the API just passes `AttackHand: Off`.

## Implementation Summary

### New Types in `combat/attack.go`

```go
// AttackHand indicates which hand is making the attack
type AttackHand string

const (
    AttackHandMain AttackHand = "main"  // Default - standard action attack
    AttackHandOff  AttackHand = "off"   // Bonus action off-hand attack
)

// EquippedWeaponInfo provides weapon information for validation
type EquippedWeaponInfo struct {
    WeaponID weapons.WeaponID
}

// TwoWeaponContext provides character weapon and action economy information
// Satisfied by gamectx adapter - avoids circular import between combat and gamectx
type TwoWeaponContext interface {
    GetMainHandWeapon(characterID string) *EquippedWeaponInfo
    GetOffHandWeapon(characterID string) *EquippedWeaponInfo
    GetActionEconomy(characterID string) *ActionEconomy
}
```

### AttackInput Changes

```go
type AttackInput struct {
    // ... existing fields (Attacker, Defender, Weapon, etc.)
    AttackHand AttackHand  // Which hand is attacking (default: main)
}
```

### Weapon Lookup via WeaponID

Added `WeaponID` field to `gamectx.EquippedWeapon`:

```go
type EquippedWeapon struct {
    ID        string            // Instance ID ("shortsword-1")
    WeaponID  weapons.WeaponID  // Weapon definition ("shortsword") for property lookup
    // ...
}
```

Validation uses `weapons.GetByID(weaponID)` to look up properties from the rulebook.

### ResolveAttack Flow (when AttackHand == Off)

1. Get `TwoWeaponContext` from context via `GetTwoWeaponContext(ctx)`
2. Validate main-hand weapon exists and is light via `weapons.GetByID()`
3. Validate off-hand weapon exists and is light via `weapons.GetByID()`
4. Check and consume bonus action via `ActionEconomy.UseBonusAction()`
5. Set `IsOffHandAttack = true` on `ResolveDamageInput`
6. Flag passes through to `DamageChainEvent`
7. TWF condition adds ability modifier when flag is true

### Error Cases

| Condition | Error |
|-----------|-------|
| No TwoWeaponContext in context | "two-weapon context not available for off-hand attack validation" |
| No main-hand weapon equipped | "no weapon in main hand" |
| No off-hand weapon equipped | "no weapon in off hand" |
| Main-hand weapon not light | "main hand weapon must be light for two-weapon fighting" |
| Off-hand weapon not light | "off hand weapon must be light for two-weapon fighting" |
| No bonus action available | "bonus action" (CodeResourceExhausted) |

## Files Modified

1. **`gamectx/characters.go`**
   - Added `WeaponID weapons.WeaponID` field to `EquippedWeapon`
   - Deprecated `IsTwoHanded`, `IsMelee` (use weapon lookup instead)

2. **`combat/attack.go`**
   - Added `AttackHand` type and constants
   - Added `TwoWeaponContext` interface
   - Added `WithTwoWeaponContext()` / `GetTwoWeaponContext()`
   - Added `AttackHand` field to `AttackInput`
   - Added `validateOffHandAttack()` function
   - Updated `ResolveAttack` to call validation for off-hand attacks

3. **`combat/damage.go`**
   - Added `IsOffHandAttack` and `AbilityModifier` to `ResolveDamageInput`
   - Pass fields through to `DamageChainEvent`

4. **`combat/two_weapon_fighting_test.go`** (new)
   - Tests for all validation scenarios
   - Tests for bonus action consumption

## Architecture Decision: TwoWeaponContext Interface

We avoided circular imports between `combat` and `gamectx` by defining `TwoWeaponContext` interface in `combat`. The API creates an adapter that implements this interface using `gamectx` data, then passes it via context.

This pattern:
- Keeps combat package focused on rules
- Avoids gamectx importing combat (which would create a cycle)
- Allows testing with mocks without needing gamectx

## Future Considerations

**Dual Wielder Feat** (out of scope):
- Removes light weapon requirement
- Add check in `validateOffHandAttack`: "has Dual Wielder feat? skip light validation"

**API Integration** (Issue #359):
- API creates adapter implementing `TwoWeaponContext`
- Passes via `combat.WithTwoWeaponContext(ctx, adapter)`
- Maps proto `AttackHand` to toolkit's `AttackHand`
