# Player-Facing Damage Breakdown Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every resolved D&D 5e damage component a player-readable explanation that preserves its dice notation, rolls, flat bonus, damage type, and total.

**Architecture:** `events.DamageComponent` retains display-only `DiceNotation`. Damage rollers populate it from declared damage pools. A combat formatter reads resolved data without changing game math, and `DamageBreakdown` exposes the result.

**Tech Stack:** Go, testify, gomock, existing D&D 5e combat and event packages.

## Global Constraints

- `DiceNotation` is display-only metadata and never affects damage math.
- `FlatBonus` stays attached to its own damage type and displayed roll.
- Use `DamageComponent.Total()` for a component and `DamageBreakdown.TotalDamage` for the final total.
- Preserve critical-hit, reroll, affinity, character-weapon, and natural-attack behavior.

---

### Task 1: Preserve declared dice notation in resolved components

**Files:**
- Modify: `rulebooks/dnd5e/events/events.go:201-225`
- Modify: `rulebooks/dnd5e/combat/damage_profile.go:14-107`
- Test: `rulebooks/dnd5e/combat/damage_profile_test.go`

**Interfaces:**
- Consumes: `damage.Damage.Dice` and `DamageProfileComponent.Dice`.
- Produces: `events.DamageComponent.DiceNotation string`.

- [ ] **Step 1: Write the failing test**

Add a deterministic `RollDamageProfile` test with a `1d6` acid component and a `2d6` bludgeoning component. Assert the resolved components retain the strings:

```go
require.Equal(t, "1d6", components[0].DiceNotation)
require.Equal(t, "2d6", components[1].DiceNotation)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./combat -run TestRollDamageProfile -count=1`

Expected: FAIL because `DiceNotation` is missing or empty.

- [ ] **Step 3: Write minimal implementation**

Add a display-only `DiceNotation string` field to `events.DamageComponent`. Set it in both component builders:

```go
DiceNotation: damagePool.Dice,
```

```go
DiceNotation: part.Dice,
```

- [ ] **Step 4: Run focused tests to verify they pass**

Run: `go test ./combat -run 'TestRollDamageProfile|TestResolveAttackNaturalAttackRollsEveryPool' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add rulebooks/dnd5e/events/events.go rulebooks/dnd5e/combat/damage_profile.go rulebooks/dnd5e/combat/damage_profile_test.go
git commit -m "feat(dnd5e): retain damage dice notation"
```

### Task 2: Format resolved damage for players

**Files:**
- Create: `rulebooks/dnd5e/combat/damage_display.go`
- Create: `rulebooks/dnd5e/combat/damage_display_test.go`
- Modify: `rulebooks/dnd5e/combat/attack.go:136-141`

**Interfaces:**
- Consumes: `events.DamageComponent` from Task 1.
- Produces: `combat.FormatDamageComponent(component events.DamageComponent) string` and `(*combat.DamageBreakdown).Display() string`.

- [ ] **Step 1: Write the failing formatter tests**

Use resolved components directly, without mocks:

```go
acid := dnd5eEvents.DamageComponent{DiceNotation: "1d6", FinalDiceRolls: []int{4}, FlatBonus: 2, DamageType: damage.Acid}
bludgeoning := dnd5eEvents.DamageComponent{DiceNotation: "2d6", FinalDiceRolls: []int{5, 3}, FlatBonus: 3, DamageType: damage.Bludgeoning}
require.Equal(t, "1d6 (4) + 2 acid = 6", combat.FormatDamageComponent(acid))
require.Equal(t, "2d6 (5 + 3) + 3 bludgeoning = 11", combat.FormatDamageComponent(bludgeoning))
require.Equal(t, "1d6 (4) + 2 acid = 6; 2d6 (5 + 3) + 3 bludgeoning = 11. Total: 17 damage.", (&combat.DamageBreakdown{Components: []dnd5eEvents.DamageComponent{acid, bludgeoning}, TotalDamage: 17}).Display())
```

Also test a negative bonus, omitted zero bonus, and a flat-only component.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./combat -run TestFormatDamage -count=1`

Expected: FAIL because the formatter functions do not exist.

- [ ] **Step 3: Write minimal implementation**

Implement `FormatDamageComponent` in `damage_display.go`: join final rolls with `" + "`, retain `DiceNotation`, format signed bonuses, append the lower-case damage type, and use `component.Total()`. Implement `DamageBreakdown.Display()` by joining lines with `"; "` and appending `". Total: <TotalDamage> damage."`.

- [ ] **Step 4: Run formatter tests to verify they pass**

Run: `go test ./combat -run TestFormatDamage -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add rulebooks/dnd5e/combat/damage_display.go rulebooks/dnd5e/combat/damage_display_test.go rulebooks/dnd5e/combat/attack.go
git commit -m "feat(dnd5e): format player-facing damage breakdowns"
```

### Task 3: Prove the complete mixed-damage path

**Files:**
- Modify: `rulebooks/dnd5e/combat/attack_test.go`

**Interfaces:**
- Consumes: `combat.ResolveAttack`, `DamageBreakdown.Display`, and deterministic roller output.
- Produces: full combat-path regression coverage without a Pseudopod-specific display path.

- [ ] **Step 1: Write the failing end-to-end test**

Create `TestResolveAttackMixedDamageDisplay` with a deterministic natural attack: `1d6 + 2 acid` rolls `4`, then `2d6 + 3 bludgeoning` rolls `5, 3`. Assert:

```go
"1d6 (4) + 2 acid = 6; 2d6 (5 + 3) + 3 bludgeoning = 11. Total: 17 damage."
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./combat -run TestResolveAttackMixedDamageDisplay -count=1`

Expected: FAIL before Tasks 1 and 2 are complete.

- [ ] **Step 3: Keep production code unchanged**

Tasks 1 and 2 provide the behavior. Do not add a special natural-attack or Pseudopod formatter.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./combat -run TestResolveAttackMixedDamageDisplay -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add rulebooks/dnd5e/combat/attack_test.go
git commit -m "test(dnd5e): cover mixed damage display"
```

### Task 4: Verify combat paths

**Files:**
- Modify: no source changes expected.

**Interfaces:**
- Consumes: complete Tasks 1–3.
- Produces: verification evidence only.

- [ ] **Step 1: Run the complete D&D 5e suite**

Run: `go test ./...`

Expected: PASS, including critical-hit, affinity, character-weapon, and monster natural-attack tests.

- [ ] **Step 2: Inspect final changes**

Run:

```powershell
git diff 76afa60..HEAD --check
git status --short
```

Expected: no whitespace errors and no cache files staged or modified. Report the already-known `dice/pool.go` Windows line-ending metadata entry separately from source changes.
