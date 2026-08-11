# Composable Attack Damage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give characters, monsters, and creatures one combat path for attacks with independent typed damage pools, beginning with Gray Ooze and Ochre Jelly Pseudopods.

**Architecture:** `damage.DamageSpec` is the canonical unrolled recipe. `attack.Definition` adds action identity, category, targeting, and fixed-versus-derived bonuses. Existing attack and damage chains remain; compatibility code converts legacy single-pool values until their callers migrate.

**Tech Stack:** Go, existing `dice.Roller`, typed event bus, staged chains, Go tests.

## Global Constraints

- Preserve legacy weapon attack behavior and public input fields during this increment.
- Keep `damage.DamageSpec` as the sole long-term unrolled damage format.
- Keep `events.DamageComponent` as the sole rolled/realized damage format.
- Add ability modifiers and other dynamic bonuses at resolution, never in a declared pool.
- Pseudopod is a natural melee attack at 1 hex; corrosion and save resolution are deferred.
- Do not stage temporary Go cache folders.

---

## File structure

- Create `rulebooks/dnd5e/damage/spec.go` and `spec_test.go`: pure reusable damage pools and validation.
- Create `rulebooks/dnd5e/attack/definition.go` and `definition_test.go`: shared attack definitions without combat/event dependencies.
- Modify `rulebooks/dnd5e/weapons/types.go`, `monster/actions/melee.go`, and `events/events.go`: additive authoring and event transport.
- Modify `rulebooks/dnd5e/combat/attack.go`, `attack_phases.go`, and `damage_profile.go`: normalize legacy values and resolve every pool.
- Modify `monster/monsters/gray_ooze.go`; create `ochre_jelly.go` and focused tests.

### Task 1: Define reusable damage pools

**Files:**
- Create: `rulebooks/dnd5e/damage/spec.go`
- Test: `rulebooks/dnd5e/damage/spec_test.go`

**Interfaces:**
- Produces `damage.Damage`, `damage.DamageSpec`, `damage.Property`, `damage.SaveSpec`, and `(*DamageSpec).Validate() error`.

- [ ] **Step 1: Write the failing validation test**

```go
func TestDamageSpecValidate(t *testing.T) {
    valid := damage.DamageSpec{Pools: []damage.Damage{{
        Dice: "1d6", Type: damage.Bludgeoning, FlatBonus: -1,
        Properties: []damage.Property{damage.PropertyCritEligible},
    }}}
    require.NoError(t, valid.Validate())
    require.Error(t, (&damage.DamageSpec{}).Validate())
    require.Error(t, (&damage.DamageSpec{Pools: []damage.Damage{{Dice: "bad", Type: damage.Acid}}}).Validate())
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./damage -run TestDamageSpecValidate -count=1`

Expected: FAIL because the new types do not exist.

- [ ] **Step 3: Implement the minimal pure model**

```go
type Damage struct {
    Dice string `json:"dice"`
    Type Type `json:"type"`
    FlatBonus int `json:"flat_bonus,omitempty"`
    Properties []Property `json:"properties,omitempty"`
    Save *SaveSpec `json:"save,omitempty"`
}
type DamageSpec struct { Pools []Damage `json:"pools"` }
type Property string
const PropertyCritEligible Property = "crit_eligible"
```

Require one or more pools, parse dice with `dice.ParseNotation`, reject `damage.None` and unknown properties, and validate a present `SaveSpec` without resolving it.

- [ ] **Step 4: Add save metadata proof**

```go
func TestDamageSpecKeepsSaveMetadata(t *testing.T) {
    spec := damage.DamageSpec{Pools: []damage.Damage{{
        Dice: "8d6", Type: damage.Fire,
        Save: &damage.SaveSpec{Ability: abilities.Dexterity, DC: 14, Effect: damage.SaveEffectHalf},
    }}}
    require.NoError(t, spec.Validate())
}
```

- [ ] **Step 5: Verify and commit**

Run: `go test ./damage -count=1`

Expected: PASS.

```bash
git add rulebooks/dnd5e/damage/spec.go rulebooks/dnd5e/damage/spec_test.go
git commit -m "feat(dnd5e): add composable damage specs"
```

### Task 2: Define shared attack identity and rules

**Files:**
- Create: `rulebooks/dnd5e/attack/definition.go`
- Test: `rulebooks/dnd5e/attack/definition_test.go`

**Interfaces:**
- Consumes `damage.DamageSpec` and optional `weapons.Weapon`.
- Produces `attack.Definition`, `attack.Category`, `attack.BonusRule`, and `(*Definition).Validate() error`.

- [ ] **Step 1: Write failing natural and equipment attack tests**

```go
func TestNaturalDefinitionAllowsNoEquipmentWeapon(t *testing.T) {
    def := attack.Definition{
        ActionID: "pseudopod", DisplayName: "Pseudopod", Category: attack.CategoryNatural,
        Bonus: attack.FixedBonus(3), Targeting: attack.MeleeReach(1),
        Damage: damage.DamageSpec{Pools: []damage.Damage{{Dice: "1d6", Type: damage.Bludgeoning}}},
    }
    require.NoError(t, def.Validate())
}
func TestEquipmentWeaponDefinitionRequiresWeapon(t *testing.T) {
    require.Error(t, (&attack.Definition{
        ActionID: "slash", Category: attack.CategoryEquipmentWeapon,
        Bonus: attack.DerivedBonus(), Targeting: attack.MeleeReach(1),
        Damage: damage.DamageSpec{Pools: []damage.Damage{{Dice: "1d8", Type: damage.Slashing}}},
    }).Validate())
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./attack -run 'Test(NaturalDefinitionAllowsNoEquipmentWeapon|EquipmentWeaponDefinitionRequiresWeapon)' -count=1`

Expected: FAIL because package `attack` does not exist.

- [ ] **Step 3: Implement Definition and validation**

```go
type Definition struct {
    ActionID string
    DisplayName string
    Category Category
    Bonus BonusRule
    Targeting Targeting
    EquipmentWeapon *weapons.Weapon
    Damage damage.DamageSpec
}
```

Require a non-empty ActionID, valid category, targeting, bonus rule, and DamageSpec. Require a non-nil weapon only for `CategoryEquipmentWeapon`. Do not infer category from a weapon pointer.

- [ ] **Step 4: Add same-ID independence proof**

```go
func TestDefinitionsWithSameActionIDRemainIndependent(t *testing.T) {
    gray := validNatural("pseudopod", 3)
    ochre := validNatural("pseudopod", 4)
    require.Equal(t, gray.ActionID, ochre.ActionID)
    require.NotEqual(t, gray.Bonus, ochre.Bonus)
}
```

- [ ] **Step 5: Verify and commit**

Run: `go test ./attack -count=1`

Expected: PASS.

```bash
git add rulebooks/dnd5e/attack
git commit -m "feat(dnd5e): add shared attack definitions"
```

### Task 3: Add authoring and event compatibility

**Files:**
- Modify: `rulebooks/dnd5e/weapons/types.go:51-63`
- Modify: `rulebooks/dnd5e/monster/actions/melee.go:19-58,124-163`
- Modify: `rulebooks/dnd5e/events/events.go:172-183,552-570`
- Test: `rulebooks/dnd5e/monster/actions/melee_test.go`

**Interfaces:**
- Adds `DamageSpec *damage.DamageSpec` beside legacy damage fields.
- Adds `Definition attack.Definition` to `events.AttackEvent` while retaining legacy event fields during migration.

- [ ] **Step 1: Write the failing authoring-precedence test**

```go
func (s *MeleeActionTestSuite) TestMeleeActionPrefersDamageSpec() {
    action := NewMeleeAction(MeleeConfig{
        Name: "Pseudopod", AttackBonus: 3, Reach: 1,
        DamageDice: "1d6", DamageType: damage.Bludgeoning,
        DamageSpec: &damage.DamageSpec{Pools: []damage.Damage{{Dice: "2d6", Type: damage.Acid}}},
    })
    event := activateAndReceiveEvent(s, action)
    s.Equal("2d6", event.Definition.Damage.Pools[0].Dice)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./monster/actions -run TestMeleeActionTestSuite/TestMeleeActionPrefersDamageSpec -count=1`

Expected: FAIL because `DamageSpec` and `AttackEvent.Definition` do not exist.

- [ ] **Step 3: Implement additive conversion**

Add `DamageSpec *damage.DamageSpec` to `weapons.Weapon` and `MeleeConfig`. When a MeleeConfig has no spec, convert its legacy dice/type to a one-pool spec. Publish a natural `attack.Definition` with a fixed configured bonus and ActionID derived from the action's local ID. Add `events.DamageSourceNaturalAttack`; do not remove `WeaponRef`, `IsMelee`, or `AttackDamageComponent` in this task.

- [ ] **Step 4: Add legacy fallback proof**

```go
func (s *MeleeActionTestSuite) TestMeleeActionConvertsLegacySinglePool() {
    action := NewMeleeAction(MeleeConfig{
        Name: "Club", AttackBonus: 2, Reach: 1,
        DamageDice: "1d4", DamageType: damage.Bludgeoning,
    })
    event := activateAndReceiveEvent(s, action)
    s.Equal("1d4", event.Definition.Damage.Pools[0].Dice)
}
```

- [ ] **Step 5: Verify and commit**

Run: `go test ./weapons ./events ./monster/actions -count=1`

Expected: PASS.

```bash
git add rulebooks/dnd5e/weapons/types.go rulebooks/dnd5e/events/events.go rulebooks/dnd5e/monster/actions
git commit -m "feat(dnd5e): publish shared attack definitions"
```

### Task 4: Normalize and resolve every pool in combat

**Files:**
- Modify: `rulebooks/dnd5e/combat/attack.go:66-183`
- Modify: `rulebooks/dnd5e/combat/attack_phases.go:47-154,210-531`
- Modify: `rulebooks/dnd5e/combat/damage_profile.go`
- Test: `rulebooks/dnd5e/combat/attack_test.go`, `attack_phases_test.go`, `breakdown_test.go`, `damage_profile_test.go`

**Interfaces:**
- Adds `Attack *attack.Definition` to attack inputs and the authoritative definition to AttackContext.
- Converts every `damage.Damage` into realized `events.DamageComponent` values for the existing damage chain.

- [ ] **Step 1: Write a failing fixed-natural-attack test**

```go
func TestResolveAttackNaturalAttackRollsEveryPool(t *testing.T) {
    result, err := combat.ResolveAttack(ctx, &combat.AttackInput{
        AttackerID: "gray", TargetID: "hero", Attack: grayPseudopod,
        EventBus: bus, Roller: fixedRoller,
    })
    require.NoError(t, err)
    require.True(t, result.Hit)
    require.Len(t, result.Breakdown.Components, 2)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./combat -run TestResolveAttackNaturalAttackRollsEveryPool -count=1`

Expected: FAIL because `AttackInput.Attack` does not exist and a weapon is required.

- [ ] **Step 3: Normalize hit-phase input**

Allow exactly one valid shared Attack or legacy Weapon. Convert legacy weapons to equipment-weapon definitions with one crit-eligible pool. Store the definition in AttackContext. Use a fixed bonus for a fixed rule; preserve ability calculation, proficiency, off-hand validation, and weapon melee/ranged logic for a derived equipment rule.

- [ ] **Step 4: Realize every damage pool**

Replace the single `ac.Weapon.Damage` roll with a helper that parses and rolls each declared pool. Double dice only for CritEligible pools, add FlatBonus once, and select the realized source from attack category. Append the existing separate ability component only for derived equipment attacks. Pass all components to the existing `ResolveDamage` chain.

- [ ] **Step 5: Write regression tests**

```go
func TestResolveAttackLegacyWeaponKeepsWeaponAndAbilityComponents(t *testing.T) {
    result := resolveLongsword(t)
    require.Len(t, result.Breakdown.Components, 2)
    require.Equal(t, dnd5eEvents.DamageSourceWeapon, result.Breakdown.Components[0].Source)
    require.Equal(t, dnd5eEvents.DamageSourceAbility, result.Breakdown.Components[1].Source)
}
func TestResolveAttackCriticalDoublesOnlyEligibleDice(t *testing.T) {
    result := resolveSelectiveCritical(t)
    require.Equal(t, []int{6, 6}, result.Breakdown.Components[0].FinalDiceRolls)
    require.Equal(t, []int{4}, result.Breakdown.Components[1].FinalDiceRolls)
}
func TestResolveAttackAcidResistanceLeavesBludgeoningUntouched(t *testing.T) {
    result := resolveResistedPseudopod(t)
    require.Equal(t, 5, amountFor(result.Breakdown, damage.Bludgeoning))
    require.Equal(t, 4, amountFor(result.Breakdown, damage.Acid))
}
func TestResolveAttackNaturalAttackAddsNoAbilityComponent(t *testing.T) {
    result := resolvePseudopod(t)
    require.NotContains(t, componentSources(result.Breakdown), dnd5eEvents.DamageSourceAbility)
}
```

- [ ] **Step 6: Verify and commit**

Run: `go test ./combat -count=1`

Expected: PASS.

```bash
git add rulebooks/dnd5e/combat
git commit -m "feat(dnd5e): resolve composable attack damage"
```

### Task 5: Add and prove both ooze Pseudopods

**Files:**
- Modify: `rulebooks/dnd5e/monster/monsters/gray_ooze.go`
- Modify: `rulebooks/dnd5e/monster/monsters/gray_ooze_test.go`
- Create: `rulebooks/dnd5e/monster/monsters/ochre_jelly.go`
- Create: `rulebooks/dnd5e/monster/monsters/ochre_jelly_test.go`
- Modify: `docs/ideas/future-cleanup.md`

**Interfaces:**
- Consumes the shared attack and damage types from Tasks 1-4.
- Produces independent natural attacks that share ActionID `pseudopod`.

- [ ] **Step 1: Write failing monster-definition tests**

```go
func TestGrayOozePseudopod(t *testing.T) {
    def := grayOozePseudopod(t)
    require.Equal(t, 3, def.Bonus.Fixed)
    require.Equal(t, -1, def.Damage.Pools[0].FlatBonus)
    require.Equal(t, damage.Acid, def.Damage.Pools[1].Type)
}
func TestOchreJellyPseudopod(t *testing.T) {
    def := ochreJellyPseudopod(t)
    require.Equal(t, 4, def.Bonus.Fixed)
    require.Equal(t, -2, def.Damage.Pools[0].FlatBonus)
    require.Equal(t, damage.Acid, def.Damage.Pools[1].Type)
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./monster/monsters -run 'Test(GrayOozePseudopod|OchreJellyPseudopod)' -count=1`

Expected: FAIL because Pseudopod definitions and the Ochre Jelly factory do not exist.

- [ ] **Step 3: Implement the two definitions**

Give both actions CategoryNatural, reach 1, ActionID `pseudopod`, and CritEligible on every listed pool. Give Gray Ooze fixed +3 with 1d6 bludgeoning FlatBonus -1 plus 2d6 acid. Give Ochre Jelly fixed +4 with 2d6 bludgeoning FlatBonus -2 plus 1d6 acid. Do not add corrosion or Split.

- [ ] **Step 4: Add behavior proofs**

```go
func TestGrayOozeCriticalPseudopod(t *testing.T) {
    result := resolveGrayOozeCritical(t)
    require.Equal(t, -1, result.Breakdown.Components[0].FlatBonus)
    require.Len(t, result.Breakdown.Components[0].FinalDiceRolls, 2)
    require.Len(t, result.Breakdown.Components[1].FinalDiceRolls, 4)
}
func TestOozesShareActionIDWithoutSharingRules(t *testing.T) {
    gray, ochre := grayOozePseudopod(t), ochreJellyPseudopod(t)
    require.Equal(t, gray.ActionID, ochre.ActionID)
    require.NotEqual(t, gray.Bonus.Fixed, ochre.Bonus.Fixed)
    require.NotEqual(t, gray.Damage.Pools, ochre.Damage.Pools)
}
```

- [ ] **Step 5: Record cleanup, verify, and commit**

Record removal of `combat.DamageProfileComponent` and `events.AttackDamageComponent` after all toolkit and rpg-api consumers use `damage.DamageSpec`.

Run: `go test ./monster/monsters -count=1 && go test ./...`

Expected: PASS.

```bash
git add rulebooks/dnd5e/monster/monsters docs/ideas/future-cleanup.md
git commit -m "feat(dnd5e): add ooze pseudopod attack definitions"
```

## Final verification

- [ ] Run `go test ./...` from `rulebooks/dnd5e` with temporary caches outside the repository.
- [ ] Run `git diff --check`.
- [ ] Confirm `git status --short` shows cache folders untracked and unstaged.
