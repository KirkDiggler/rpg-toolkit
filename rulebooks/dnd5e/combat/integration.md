# Combat System Integration Tests

This document shows how to run the integration tests and what output to expect. These tests validate the event-driven combat system works end-to-end with real components.

## Overview

The integration tests use:
- **Real components**: EventBus, Character (loaded from Data), Conditions, Combat system
- **Mocked dice**: For deterministic test results
- **Testify suite**: Clean test organization with proper setup/teardown

## Running All Tests

```bash
go test -v -run TestCombatIntegrationSuite
```

## Test Cases

### 1. Normal Attack Without Rage

**What it tests**: Basic attack mechanics without any active conditions.

**Run this test**:
```bash
go test -v -run TestCombatIntegrationSuite/TestAttackWithoutRage
```

**Expected output**:
```
=== RUN   TestCombatIntegrationSuite/TestAttackWithoutRage
=== RUN   TestCombatIntegrationSuite/TestAttackWithoutRage/Normal_attack_without_rage
    integration_test.go:318: === Barbarian Normal Attack (No Rage) Test ===
    integration_test.go:319: Attacker: Grog (Level 1 Barbarian, STR +3)
    integration_test.go:320: Defender: Goblin Scout (AC 13)
    integration_test.go:321:
    integration_test.go:324: → Grog swings greataxe at Goblin Scout (NOT raging)
    integration_test.go:345:   Attack roll: 1d20(15) + STR(3) + Prof(2) = 20
    integration_test.go:346:   vs AC 13 → HIT!
    integration_test.go:347:
    integration_test.go:352:   Damage breakdown:
    integration_test.go:353:     1d12 weapon damage: 8
    integration_test.go:354:     + STR modifier: 3
    integration_test.go:355:     (No rage bonus - not active)
    integration_test.go:356:   = Total damage: 11
    integration_test.go:357:
    integration_test.go:358: ✓ Integration test passed: No rage bonus when not active
--- PASS: TestCombatIntegrationSuite/TestAttackWithoutRage (0.00s)
```

**What this validates**:
- Basic attack roll calculation (d20 + STR + proficiency)
- Hit detection (total vs AC)
- Base damage calculation (weapon dice + STR modifier)
- No bonus damage when no conditions are active

---

### 2. Barbarian Rage Adds Damage on Hit

**What it tests**: The complete event-driven flow of rage damage bonus application.

**Run this test**:
```bash
go test -v -run TestCombatIntegrationSuite/TestBarbarianRageAddsDamageOnHit
```

**Expected output**:
```
=== RUN   TestCombatIntegrationSuite/TestBarbarianRageAddsDamageOnHit
=== RUN   TestCombatIntegrationSuite/TestBarbarianRageAddsDamageOnHit/Normal_hit_with_rage_active
    integration_test.go:152: === Barbarian Rage Attack Integration Test ===
    integration_test.go:153: Attacker: Grog (Level 1 Barbarian, STR +3)
    integration_test.go:154: Defender: Goblin Scout (AC 13)
    integration_test.go:155:
    integration_test.go:161: → Grog enters a rage!
    integration_test.go:168:   ✓ Raging condition applied (+2 damage to melee attacks)
    integration_test.go:169:
    integration_test.go:175: → Grog swings greataxe at Goblin Scout
    integration_test.go:192:   Attack roll: 1d20(15) + STR(3) + Prof(2) = 20
    integration_test.go:193:   vs AC 13 → HIT!
    integration_test.go:194:
    integration_test.go:200:   Damage breakdown:
    integration_test.go:201:     1d12 weapon damage: 8
    integration_test.go:202:     + STR modifier: 3
    integration_test.go:203:     + Rage bonus: 2
    integration_test.go:204:   = Total damage: 13
    integration_test.go:205:
    integration_test.go:206: ✓ Integration test passed: Event-driven rage bonus applied correctly
--- PASS: TestCombatIntegrationSuite/TestBarbarianRageAddsDamageOnHit (0.00s)
```

**What this validates**:
- Feature activation (Rage.Activate)
- ConditionAppliedEvent published and received
- Character receives and applies RagingCondition
- RagingCondition subscribes to DamageChain
- Rage bonus (+2) correctly added to damage
- Full event-driven architecture working end-to-end

**Event flow**:
1. `Feature.Activate()` creates RagingCondition
2. Publishes `ConditionAppliedEvent`
3. Character receives event and calls `condition.Apply()`
4. RagingCondition subscribes to `DamageChain`
5. Attack triggers DamageChain
6. RagingCondition adds +2 to damage in StageFeatures
7. Final damage = 8 (weapon) + 3 (STR) + 2 (rage) = 13

---

### 3. Rage Damage Not Applied on Miss

**What it tests**: That rage bonus only applies when attacks hit.

**Run this test**:
```bash
go test -v -run TestCombatIntegrationSuite/TestRageDamageNotAppliedOnMiss
```

**Expected output**:
```
=== RUN   TestCombatIntegrationSuite/TestRageDamageNotAppliedOnMiss
=== RUN   TestCombatIntegrationSuite/TestRageDamageNotAppliedOnMiss/Attack_misses_with_rage_active
    integration_test.go:213: === Barbarian Rage Miss Test ===
    integration_test.go:214: Attacker: Grog (Level 1 Barbarian, STR +3)
    integration_test.go:215: Defender: Goblin Scout (AC 13)
    integration_test.go:216:
    integration_test.go:219: → Grog enters a rage!
    integration_test.go:223:   ✓ Raging condition applied (+2 damage to melee attacks)
    integration_test.go:224:
    integration_test.go:229: → Grog swings greataxe at Goblin Scout
    integration_test.go:246:   Attack roll: 1d20(3) + STR(3) + Prof(2) = 8
    integration_test.go:247:   vs AC 13 → MISS!
    integration_test.go:248:
    integration_test.go:249:   No damage dealt on miss
    integration_test.go:250:
    integration_test.go:251: ✓ Integration test passed: Rage bonus correctly not applied on miss
--- PASS: TestCombatIntegrationSuite/TestRageDamageNotAppliedOnMiss (0.00s)
```

**What this validates**:
- Rage condition is active (subscribed to events)
- Attack misses (8 vs AC 13)
- No damage is dealt on a miss (even with rage active)
- Damage chain is NOT executed when attack misses

---

### 4. Critical Hit with Rage

**What it tests**: D&D 5e critical hit rules - doubles weapon dice but NOT modifiers.

**Run this test**:
```bash
go test -v -run TestCombatIntegrationSuite/TestCriticalHitWithRage
```

**Expected output**:
```
=== RUN   TestCombatIntegrationSuite/TestCriticalHitWithRage
=== RUN   TestCombatIntegrationSuite/TestCriticalHitWithRage/Critical_hit_doubles_weapon_dice,_not_rage_bonus
    integration_test.go:260: === Barbarian Critical Hit with Rage Test ===
    integration_test.go:261: Attacker: Grog (Level 1 Barbarian, STR +3)
    integration_test.go:262: Defender: Goblin Scout (AC 13)
    integration_test.go:263:
    integration_test.go:266: → Grog enters a rage!
    integration_test.go:270:   ✓ Raging condition applied (+2 damage to melee attacks)
    integration_test.go:271:
    integration_test.go:278: → Grog swings greataxe at Goblin Scout
    integration_test.go:296:   Attack roll: Natural 20 → CRITICAL HIT!
    integration_test.go:297:
    integration_test.go:303:   Damage breakdown:
    integration_test.go:304:     2d12 weapon damage (doubled): 6 + 8 = 14
    integration_test.go:305:     + STR modifier: 3
    integration_test.go:306:     + Rage bonus (NOT doubled): 2
    integration_test.go:307:   = Total damage: 19
    integration_test.go:308:
    integration_test.go:309: ✓ Integration test passed: Critical doubles dice but not modifiers
--- PASS: TestCombatIntegrationSuite/TestCriticalHitWithRage (0.00s)
```

**What this validates**:
- Natural 20 detection
- Critical hit doubles weapon damage dice (1d12 becomes 2d12)
- STR modifier is NOT doubled (still +3)
- Rage bonus is NOT doubled (still +2)
- Correct D&D 5e critical hit mechanics
- Final damage = 14 (2d12) + 3 (STR) + 2 (rage) = 19

**Key rule**: Only weapon damage DICE are doubled on crits, not modifiers or bonuses.

---

## Architecture Validated

These integration tests prove the following architectural decisions work correctly:

### Event-Driven Design
- Features publish `ConditionAppliedEvent`
- Character subscribes and receives conditions
- Conditions subscribe to combat events (DamageChain)
- Loose coupling via EventBus

### Condition Lifecycle
1. **Creation**: Feature creates condition with all necessary data
2. **Application**: `condition.Apply(ctx, bus)` subscribes to events
3. **Execution**: Condition modifies events as they flow through chains
4. **Cleanup**: `condition.Remove(ctx, bus)` unsubscribes

### Staged Chain System
- `DamageChain` flows through `dnd5e.ModifierStages`
- Rage bonus applied in `StageFeatures`
- Modifiers add to event fields, then chain executes
- Final damage = sum of all modifications

### Character Loading from Data
- Simulates database load pattern
- Features deserialized from JSON
- Character subscribes to events on load
- Ready for real persistence layer

## Related Files

- **Test file**: `combat/integration_test.go`
- **Combat attack**: `combat/attack.go`
- **Raging condition**: `conditions/raging.go`
- **Rage feature**: `features/rage.go`
- **Character**: `character/character.go`
- **Journey doc**: `docs/journey/045-circular-dependencies-events-subpackage.md`

## Next Steps

Future integration tests could validate:
- Multiple conditions stacking
- Condition removal (end of rage)
- Turn-based rage expiration
- Resistance/vulnerability mechanics
- Concentration checks
- Status effect interactions
