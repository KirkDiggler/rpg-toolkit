# Monster Dice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Support signed linear monster damage expressions and route Melee, Ranged, and Bite monster actions through one shared combat-definition path.

**Architecture:** A D&D damage parser converts a linear expression into signed pure-dice terms plus one flat bonus. `damage.Damage` becomes the canonical structured pool, and combat rolls its terms together as one typed component. Monster action adapters convert and preserve legacy text while publishing shared natural-attack definitions; Bite remains damage-only and does not apply knockdown.

**Tech Stack:** Go, existing `dice.Roller`, D&D 5e `damage`, `attack`, `combat`, `events`, monster action packages, testify, gomock.

## Global Constraints

- Support only `XdY (+ or - XdY)* (+ or - whole number)?`, with optional whitespace; X and Y are positive whole numbers.
- Every dice term has sign `+1` or `-1`; a trailing number without `d` is the one pool `FlatBonus`.
- Do not add parentheses, multiplication, division, functions, variables, arbitrary expression evaluation, saving throws, Prone, or Bite knockdown behavior.
- A pool remains one typed damage component; affinities apply after all signed terms and its flat bonus are resolved.
- Critical hits double every dice term in a crit-eligible pool, including negative terms, and never double `FlatBonus`.
- Preserve legacy `DamageDice` / `DamageComponents` text on load and serialization; persist the structured specification as authoritative.
- Melee uses configured reach; Ranged uses configured normal/long ranges; Bite uses one-hex melee reach.
- Do not use an equipment weapon or create Pseudopod-, Brown-Bear-, or monster-specific combat resolution.

---

### Task 1: Parse and validate signed expressions

**Files:**
- Create: `rulebooks/dnd5e/damage/expression.go`
- Create: `rulebooks/dnd5e/damage/expression_test.go`
- Modify: `rulebooks/dnd5e/damage/spec.go`
- Modify: `rulebooks/dnd5e/damage/spec_test.go`

**Interfaces:**

```go
type DiceTerm struct { Dice string `json:"dice"`; Sign int `json:"sign"` }
type Expression struct { Terms []DiceTerm; FlatBonus int; Notation string }
func ParseExpression(input string) (Expression, error)
func (e Expression) Validate() error
```

`damage.Damage` gains `Terms []DiceTerm`; `Terms` are authoritative when present, while `Dice string` remains compatibility/readability data.

- [ ] **Step 1: Write failing parser tests**

Assert exact parsed terms, signs, bonuses, and normalized notation for `1d8+4`, `2d6-2`, `1d6+1d4`, `1d6-1d4+2`, and `2d6 + 1d4 - 3`. Reject `0d6`, `1d0`, `1d6*2`, `(1d6)`, `1d6/2`, `1d6++2`, and a bare number.

- [ ] **Step 2: Verify RED**

Run: `go test ./damage -run TestParseExpression -count=1`

Expected: FAIL because the parser and types do not exist.

- [ ] **Step 3: Implement the parser and validation**

Use a small left-to-right tokenizer/parser, not Go expression evaluation. Require an initial dice token; then read operator plus dice token or whole number. Normalize notation without whitespace. Add `DamageSpec.Validate` support: validate `Terms` when present, otherwise parse legacy `Dice`. Canonical terms must be pure positive `XdY` groups.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./damage -run 'TestParseExpression|TestDamageSpecValidate' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add rulebooks/dnd5e/damage/expression.go rulebooks/dnd5e/damage/expression_test.go rulebooks/dnd5e/damage/spec.go rulebooks/dnd5e/damage/spec_test.go
git commit -m "feat(dnd5e): parse signed damage expressions"
```

### Task 2: Roll and display signed terms as one component

**Files:**
- Modify: `rulebooks/dnd5e/events/events.go:201-225`
- Modify: `rulebooks/dnd5e/combat/damage_profile.go:14-107`
- Modify: `rulebooks/dnd5e/combat/damage_display.go`
- Modify: `rulebooks/dnd5e/combat/damage_display_test.go`
- Modify: `rulebooks/dnd5e/combat/damage_profile_test.go`

**Interfaces:**

```go
type RolledDiceTerm struct {
    Dice string `json:"dice"`
    Sign int `json:"sign"`
    Original []int `json:"original"`
    Final []int `json:"final"`
}
```

`events.DamageComponent` gains `Terms []RolledDiceTerm`. `Total()` sums signed final rolls plus `FlatBonus`, but retains its current flattened-roll calculation when `Terms` is absent.

- [ ] **Step 1: Write failing resolution/display tests**

With deterministic rolls, verify `1d6-1d4+2 acid` rolls `5` and `2` into one acid component totaling `5`, and displays exactly `1d6 (5) - 1d4 (2) + 2 acid = 5`. Add a critical test proving both dice terms double and `+2` appears once.

- [ ] **Step 2: Verify RED**

Run: `go test ./combat -run 'TestRollAttackDamageSignedTerms|TestFormatDamageComponentSignedTerms' -count=1`

Expected: FAIL because resolved signed terms and formatter support do not exist.

- [ ] **Step 3: Implement shared signed rolling**

Update `rollAttackDamage` to parse each pool, roll every term independently, and emit exactly one component per pool. Crit-eligible critical pools roll each positive and negative term twice. Populate `Terms`, plus flattened compatibility roll fields in declared term order. Update the formatter to prefer signed terms and retain current formatter behavior as a fallback for older feature-produced components.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./combat -run 'TestRollAttackDamageSignedTerms|TestFormatDamageComponentSignedTerms|TestResolveAttackCriticalDoublesOnlyEligibleDice' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add rulebooks/dnd5e/events/events.go rulebooks/dnd5e/combat/damage_profile.go rulebooks/dnd5e/combat/damage_profile_test.go rulebooks/dnd5e/combat/damage_display.go rulebooks/dnd5e/combat/damage_display_test.go
git commit -m "feat(dnd5e): resolve signed damage terms"
```

### Task 3: Convert and preserve legacy Melee actions

**Files:**
- Modify: `rulebooks/dnd5e/monster/actions/melee.go`
- Modify: `rulebooks/dnd5e/monster/actions/melee_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/brown_bear_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/gray_ooze_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/ochre_jelly_test.go`

**Interfaces:** A helper in `monster/actions` consumes legacy config and produces a `damage.DamageSpec` with precedence: structured spec; legacy `DamageComponents`; then legacy `DamageDice` plus `DamageType`.

- [ ] **Step 1: Write failing conversion/persistence tests**

Assert Brown Bear `1d8+4` becomes one crit-eligible piercing pool with term `+1d8` and `FlatBonus: 4`. Assert legacy Pseudopod components `1d6-1` bludgeoning and `2d6` acid become two typed pools. Round-trip `ToData` through the loader and assert legacy text plus structured data survive. Assert Gray Ooze and Ochre Jelly structured pools remain unchanged.

- [ ] **Step 2: Verify RED**

Run: `go test ./monster/actions ./monster/monsters -run 'Test.*(BrownBear|Legacy.*Pseudopod|RoundTrip|GrayOoze|OchreJelly)' -count=1`

Expected: FAIL because legacy strings are copied directly and legacy mixed components are not authoritative.

- [ ] **Step 3: Implement conversion boundary**

Add one focused conversion helper and use it in `NewMeleeAction`. Parse every legacy expression, preserve types and text, and mark converted pools `PropertyCritEligible`. Keep `NewMeleeAction` source compatible: store conversion failure on the action and return it from `CanActivate`/`Activate`; make loader reject invalid persisted data before returning an action. Serialize both old and structured forms.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./monster/actions ./monster/monsters -run 'Test.*(BrownBear|Legacy.*Pseudopod|RoundTrip|GrayOoze|OchreJelly)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add rulebooks/dnd5e/monster/actions/melee.go rulebooks/dnd5e/monster/actions/melee_test.go rulebooks/dnd5e/monster/monsters/brown_bear_test.go rulebooks/dnd5e/monster/monsters/gray_ooze_test.go rulebooks/dnd5e/monster/monsters/ochre_jelly_test.go
git commit -m "feat(dnd5e): convert legacy melee damage"
```

### Task 4: Connect RangedAction

**Files:**
- Modify: `rulebooks/dnd5e/monster/actions/ranged.go`
- Modify: `rulebooks/dnd5e/monster/actions/ranged_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/bandit_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/skeleton_test.go`

**Interfaces:** RangedAction consumes the Task 3 helper and publishes `attack.Definition{Category: attack.CategoryNatural, Bonus: attack.FixedBonus(...), Targeting: attack.Ranged(...), Damage: ...}`.

- [ ] **Step 1: Write failing ranged tests**

Assert Bandit light crossbow publishes fixed `+3`, normal `80`, long `320`, and one piercing `1d8+1` pool. Assert Skeleton shortbow publishes `1d6+2`. Assert malformed legacy text prevents event publication.

- [ ] **Step 2: Verify RED**

Run: `go test ./monster/actions ./monster/monsters -run 'Test.*(Ranged|Bandit.*Crossbow|Skeleton.*Shortbow)' -count=1`

Expected: FAIL because RangedAction publishes no definition.

- [ ] **Step 3: Implement ranged publication**

Store converted structured damage and conversion error in RangedAction. Publish a natural ranged definition with fixed bonus and `attack.Ranged(normal, long)`. Preserve range checks and legacy serialization fields.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./monster/actions ./monster/monsters -run 'Test.*(Ranged|Bandit.*Crossbow|Skeleton.*Shortbow)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add rulebooks/dnd5e/monster/actions/ranged.go rulebooks/dnd5e/monster/actions/ranged_test.go rulebooks/dnd5e/monster/monsters/bandit_test.go rulebooks/dnd5e/monster/monsters/skeleton_test.go
git commit -m "feat(dnd5e): resolve ranged monster attacks"
```

### Task 5: Connect BiteAction without knockdown

**Files:**
- Modify: `rulebooks/dnd5e/monster/actions/bite.go`
- Modify: `rulebooks/dnd5e/monster/actions/bite_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/wolf_test.go`

**Interfaces:** BiteAction consumes the Task 3 helper and publishes a natural melee definition with `attack.MeleeReach(1)`.

- [ ] **Step 1: Write failing Bite tests**

Assert Wolf Bite publishes action ID/display name `bite`, fixed `+4`, one-hex reach, and a piercing `2d4+2` pool. Subscribe to condition/save topics and assert no save request or condition application occurs. Assert `KnockdownDC: 11` survives `ToData` round-trip.

- [ ] **Step 2: Verify RED**

Run: `go test ./monster/actions ./monster/monsters -run 'Test.*(Bite|Wolf)' -count=1`

Expected: FAIL because BiteAction publishes no definition.

- [ ] **Step 3: Implement damage-only Bite publication**

Store converted data/error; publish a natural melee definition using fixed bonus and `attack.MeleeReach(1)`. Preserve `KnockdownDC` serialization but add no saving-throw, Prone, or post-hit effect code.

- [ ] **Step 4: Verify GREEN**

Run: `go test ./monster/actions ./monster/monsters -run 'Test.*(Bite|Wolf)' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add rulebooks/dnd5e/monster/actions/bite.go rulebooks/dnd5e/monster/actions/bite_test.go rulebooks/dnd5e/monster/monsters/wolf_test.go
git commit -m "feat(dnd5e): resolve bite monster attacks"
```

### Task 6: Prove Monster Dice in real combat

**Files:**
- Modify: `rulebooks/dnd5e/combat/attack_test.go`
- Modify: `rulebooks/dnd5e/combat/damage_display_test.go`

**Interfaces:** Consumes all action definitions and signed pools. Produces generic combat and display regression coverage.

- [ ] **Step 1: Write end-to-end regression tests**

Use deterministic combat to cover Brown Bear `1d8+4`, Bandit crossbow `1d8+1`, Wolf Bite `2d4+2`, and `1d6-1d4+2`. Assert total, component-local flat bonus, exact display, and melee/ranged metadata. Add a signed critical assertion that doubles both dice groups but not the flat bonus.

- [ ] **Step 2: Run regression tests**

Run: `go test ./combat -run 'TestResolveAttack.*(MonsterDice|Signed|BrownBear|Bandit|Wolf)' -count=1`

Expected: PASS because Tasks 1–5 provide behavior; this is real-resolver regression coverage.

- [ ] **Step 3: Run the D&D module suite**

From `rulebooks/dnd5e`, run: `go test ./...`

Expected: PASS. If dependency download is sandbox-blocked, record the exact environmental error and do not call it a source-test failure.

- [ ] **Step 4: Inspect final state**

Run `git diff --check` and `git status --short`. Report any cache entries separately, and never stage/remove them.

- [ ] **Step 5: Commit regression coverage**

```powershell
git add rulebooks/dnd5e/combat/attack_test.go rulebooks/dnd5e/combat/damage_display_test.go
git commit -m "test(dnd5e): cover monster dice combat paths"
```
