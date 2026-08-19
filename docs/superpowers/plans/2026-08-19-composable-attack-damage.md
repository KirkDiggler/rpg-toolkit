# Composable Attack Damage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Compile weapons and monster actions into ordered typed damage pools, then resolve them through one strike, one damage fold, and one damage application with SRD 5.1 critical-hit behavior.

**Architecture:** `rulebooks/dnd5e` owns declared and resolved damage shapes, persisted action migration, weapon declarations, and damage-chain feature semantics. `rulebooks/dnd5e/resolution` compiles character or monster content into `AttackProfile`, validates the profile before randomness, rolls every pool, folds once, applies once, and returns typed evidence. The top-level `encounter` module projects canonical action damage into its legacy readiness snapshot; `session` continues recording aggregate damage.

**Tech Stack:** Go 1.25, multi-module Go workspace, testify suites, typed staged event chains, JSON persistence, GitHub Actions.

## Global Constraints

- Follow TDD for every behavior: observe the focused test fail before implementation, then pass before committing.
- Preserve one attack roll, one `Gather(damage)`, one `combat.FinalDamage` call, and one `ApplyDamage` call per strike.
- Dice notation in `damage.Damage.Dice` is pure `NdM`; intrinsic arithmetic belongs in `FlatBonus` and character ability arithmetic belongs in `AttackProfile.AbilityModifier`.
- Every damage pool is critical-eligible unless it explicitly carries `damage.DoesNotCrit`; flat bonuses never double.
- Sneak Attack dice double on a critical hit; Brutal Critical's granted die rolls once and has component `IsCritical == false`.
- Great Weapon Fighting rerolls only the marked primary weapon component; Martial Arts replaces that component; Rage and Sneak Attack inherit its type; Dueling and Two-Weapon Fighting consume the new primary metadata.
- New persisted action writers emit only `damage`; readers accept canonical data first and fall back to a complete valid legacy `damage_dice`/`damage_type` pair.
- Canonical data never falls back to stale legacy fields when canonical validation fails.
- Typed evidence is guaranteed through `StrikeOutcome`; encounter/session records remain aggregate-only.
- Do not add production Lifedrinker, ranged strike semantics, save-gated damage changes, multiattack orchestration, or a new event lifecycle.
- Do not commit `go.work`, local `replace` directives, or unpublished pseudo-versions.

---

## Checkpoint 1: Shared damage declarations and weapon content

### Task 1: Add canonical declared and resolved damage types

**Files:**
- Modify: `rulebooks/dnd5e/damage/damage.go`
- Create: `rulebooks/dnd5e/damage/damage_test.go`

**Interfaces:**
- Produces: `damage.Property`, `damage.AddsAttackAbilityModifier`, `damage.DoesNotCrit`, `damage.Damage`, `damage.Instance`, `Damage.HasProperty(Property) bool`, and `damage.Validate([]Damage) error`.
- Consumes: `dice.ParseNotation` for syntax validation and `damage.All` for recognized types.

- [ ] **Step 1: Write failing property and validation tests**

```go
func (s *DamageTestSuite) TestHasProperty() {
	d := damage.Damage{Properties: []damage.Property{damage.DoesNotCrit}}
	s.True(d.HasProperty(damage.DoesNotCrit))
	s.False(d.HasProperty(damage.AddsAttackAbilityModifier))
}

func (s *DamageTestSuite) TestValidationRejectsFusedModifierAndDuplicateAbilityMarkers() {
	s.Error(damage.Validate([]damage.Damage{{Dice: "1d8+2", Type: damage.Slashing}}))
	s.Error(damage.Validate([]damage.Damage{
		{Dice: "1d8", Type: damage.Slashing, Properties: []damage.Property{damage.AddsAttackAbilityModifier}},
		{Dice: "1d6", Type: damage.Fire, Properties: []damage.Property{damage.AddsAttackAbilityModifier}},
	}))
}
```

- [ ] **Step 2: Run the focused test and confirm RED**

Run: `cd rulebooks/dnd5e && go test ./damage -run 'TestDamageTestSuite' -count=1`

Expected: FAIL because `damage.Damage`, properties, and `Validate` do not exist.

- [ ] **Step 3: Implement the canonical types and validator**

```go
type Property string

const (
	AddsAttackAbilityModifier Property = "adds-attack-ability-modifier"
	DoesNotCrit                 Property = "does-not-crit"
)

type Damage struct {
	Dice       string     `json:"dice"`
	Type       Type       `json:"type"`
	FlatBonus  int        `json:"flat_bonus,omitempty"`
	Properties []Property `json:"properties,omitempty"`
}

type Instance struct {
	Amount int  `json:"amount"`
	Type   Type `json:"type"`
}

func (d Damage) HasProperty(want Property) bool {
	for _, got := range d.Properties {
		if got == want { return true }
	}
	return false
}
```

Implement `Validate` to reject an empty slice, empty/malformed/fused dice, `None` or unknown types, unknown properties, and more than one ability marker. Wrap the pool index and notation into each returned error.

- [ ] **Step 4: Run the damage package tests and confirm GREEN**

Run: `cd rulebooks/dnd5e && go test ./damage -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add rulebooks/dnd5e/damage/damage.go rulebooks/dnd5e/damage/damage_test.go
git commit -m "feat(dnd5e): add canonical damage pools"
```

### Task 2: Migrate weapon declarations and versatile selection

**Files:**
- Modify: `rulebooks/dnd5e/weapons/types.go`
- Modify: `rulebooks/dnd5e/weapons/weapons.go`
- Modify: `rulebooks/dnd5e/weapons/versatile.go`
- Modify: `rulebooks/dnd5e/weapons/weapons_test.go`
- Modify: `rulebooks/dnd5e/weapons/versatile_test.go`
- Modify: `rulebooks/dnd5e/character/equipment_display.go`
- Test: `rulebooks/dnd5e/character/equipment_display_test.go`

**Interfaces:**
- Consumes: `damage.Damage` and `damage.AddsAttackAbilityModifier` from Task 1.
- Produces: `Weapon.Damage []damage.Damage`, `Weapon.PrimaryDamage() (damage.Damage, bool)`, and `Weapon.DamageForGrip(twoHanded bool) ([]damage.Damage, error)`.

- [ ] **Step 1: Write failing weapon and versatile tests**

```go
func (s *VersatileTestSuite) TestTwoHandsReplaceOnlyMarkedPrimaryPool() {
	w := weapons.Weapon{Properties: []weapons.WeaponProperty{weapons.PropertyVersatile}, Damage: []damage.Damage{
		{Dice: "1d8", Type: damage.Slashing, Properties: []damage.Property{damage.AddsAttackAbilityModifier}},
		{Dice: "1d6", Type: damage.Fire},
	}}
	got, err := w.DamageForGrip(true)
	s.Require().NoError(err)
	s.Equal("1d10", got[0].Dice)
	s.Equal("1d6", got[1].Dice)
	s.Equal("1d8", w.Damage[0].Dice, "compiler helpers must not mutate catalog content")
}
```

- [ ] **Step 2: Run focused tests and confirm RED**

Run: `cd rulebooks/dnd5e && go test ./weapons ./character -run 'Versatile|Weapon|EquipmentDisplay' -count=1`

Expected: FAIL because `Weapon.Damage` is singular and `DamageForGrip` is missing.

- [ ] **Step 3: Replace singular declarations throughout the catalog**

Use this exact shape for ordinary weapons:

```go
Damage: []damage.Damage{{
	Dice: "1d8", Type: damage.Slashing,
	Properties: []damage.Property{damage.AddsAttackAbilityModifier},
}},
```

Implement `PrimaryDamage` by locating the single marked pool. Implement `DamageForGrip` by copying the slice and, only for a versatile two-handed grip, replacing the marked pool's dice with `VersatileTwoHandedDamage`. Return an error if a versatile weapon lacks exactly one marked pool. Update equipment descriptions to join every pool as `dice type damage` while preserving property display.

- [ ] **Step 4: Run all weapon and character tests and confirm GREEN**

Run: `cd rulebooks/dnd5e && go test ./weapons ./character -count=1`

Expected: PASS, including one- and two-handed longsword/spear cases.

- [ ] **Step 5: Commit**

```bash
git add rulebooks/dnd5e/weapons rulebooks/dnd5e/character
git commit -m "feat(dnd5e): migrate weapons to damage pools"
```

---

## Checkpoint 2: Persisted monster-action migration

### Task 3: Canonicalize melee, bite, and ranged action damage

**Files:**
- Create: `rulebooks/dnd5e/monster/actions/damage_config.go`
- Create: `rulebooks/dnd5e/monster/actions/damage_config_test.go`
- Modify: `rulebooks/dnd5e/monster/actions/melee.go`
- Modify: `rulebooks/dnd5e/monster/actions/melee_test.go`
- Modify: `rulebooks/dnd5e/monster/actions/bite.go`
- Modify: `rulebooks/dnd5e/monster/actions/bite_test.go`
- Modify: `rulebooks/dnd5e/monster/actions/ranged.go`
- Modify: `rulebooks/dnd5e/monster/actions/ranged_test.go`
- Modify: `rulebooks/dnd5e/monster/actions/loader.go`
- Modify: monster fixtures under `rulebooks/dnd5e/monster/monsters/*.go`

**Interfaces:**
- Consumes: `damage.Validate` and canonical `damage.Damage`.
- Produces: action config `Damage []damage.Damage`, deprecated input-only `DamageDice`/`DamageType`, and `canonicalDamage(new, legacyDice, legacyType) ([]damage.Damage, error)`.

- [ ] **Step 1: Write table-driven RED tests for migration precedence**

```go
func (s *DamageConfigTestSuite) TestCanonicalDamagePrecedence() {
	newPools := []damage.Damage{{Dice: "1d8", Type: damage.Acid}}
	got, err := canonicalDamage(newPools, "not-dice", damage.None)
	s.Require().NoError(err)
	s.Equal(newPools, got)
}

func (s *DamageConfigTestSuite) TestLegacyModifierIsSplit() {
	got, err := canonicalDamage(nil, "2d4-1", damage.Piercing)
	s.Require().NoError(err)
	s.Equal([]damage.Damage{{Dice: "2d4", Type: damage.Piercing, FlatBonus: -1}}, got)
}
```

Add cases for `+2`, zero, partial legacy input, malformed notation, unknown type, empty declarations, and invalid canonical data beside valid legacy data.

- [ ] **Step 2: Run focused action tests and confirm RED**

Run: `cd rulebooks/dnd5e && go test ./monster/actions -run 'DamageConfig|Melee|Bite|Ranged' -count=1`

Expected: FAIL because canonical action damage is absent.

- [ ] **Step 3: Implement one shared migration helper and update action structs**

```go
type damageConfig struct {
	Damage     []damage.Damage `json:"damage,omitempty"`
	DamageDice string          `json:"damage_dice,omitempty"`
	DamageType damage.Type     `json:"damage_type,omitempty"`
}
```

Embed or repeat these JSON fields in `MeleeConfig`, `BiteConfig`, and `RangedConfig`. Constructors/loaders canonicalize immediately and retain only the canonical slice internally. `ToData` writes `damage` only. Keep Bite's `SaveGate`/`KnockdownDC` new-first behavior unchanged. Keep ranged compilation disabled; this task changes persistence only.

- [ ] **Step 4: Convert in-repository monster fixtures and verify canonical-only output**

Replace each `DamageDice`/`DamageType` initializer with:

```go
Damage: []damage.Damage{{Dice: "2d4", Type: damage.Piercing, FlatBonus: 2}},
```

Run: `cd rulebooks/dnd5e && go test ./monster/... -count=1`

Expected: PASS; JSON assertions show `damage` and no `damage_dice`/`damage_type` on new writes.

- [ ] **Step 5: Commit**

```bash
git add rulebooks/dnd5e/monster
git commit -m "feat(dnd5e): canonicalize monster action damage"
```

### Task 4: Convert the legacy scimitar without double-counting

**Files:**
- Modify: `rulebooks/dnd5e/monster/scimitar_action.go`
- Modify: `rulebooks/dnd5e/monster/scimitar_action_test.go`
- Modify: `rulebooks/dnd5e/monster/actions/loader.go`
- Modify: `rulebooks/dnd5e/monster/actions/integration_test.go`

**Interfaces:**
- Produces: `ScimitarConfig.Damage []damage.Damage`; legacy reads convert fused `DamageDice` to slashing and do not add `DamageBonus` again.

- [ ] **Step 1: Write a persisted-fixture regression test**

```go
func (s *ScimitarActionTestSuite) TestLegacyFusedBonusIsNotCountedTwice() {
	raw := json.RawMessage(`{"id":"scimitar","attack_bonus":4,"damage_dice":"1d6+2","damage_bonus":2}`)
	action, err := actions.LoadAction(monster.ActionData{Ref: *refs.MonsterActions.Scimitar(), Config: raw})
	s.Require().NoError(err)
	var written map[string]any
	s.Require().NoError(json.Unmarshal(action.ToData().Config, &written))
	s.Equal(float64(2), written["damage"].([]any)[0].(map[string]any)["flat_bonus"])
	s.NotContains(written, "damage_dice")
}
```

- [ ] **Step 2: Run and confirm RED**

Run: `cd rulebooks/dnd5e && go test ./monster ./monster/actions -run 'Scimitar.*Legacy|LegacyFused' -count=1`

Expected: FAIL because scimitar still writes legacy fields.

- [ ] **Step 3: Implement the dedicated scimitar converter**

Parse `DamageDice` into pure dice plus its modifier, force `damage.Slashing`, ignore the separate historical `DamageBonus` for arithmetic, store canonical damage internally, and write only `damage`.

- [ ] **Step 4: Run monster tests and confirm GREEN**

Run: `cd rulebooks/dnd5e && go test ./monster/... -count=1`

Expected: PASS with exactly one `+2` in the fixture.

- [ ] **Step 5: Commit**

```bash
git add rulebooks/dnd5e/monster/scimitar_action.go rulebooks/dnd5e/monster/scimitar_action_test.go rulebooks/dnd5e/monster/actions
git commit -m "fix(dnd5e): migrate legacy scimitar damage once"
```

---

## Checkpoint 3: Damage-chain carrier and feature semantics

### Task 5: Add typed primary metadata and compatibility mirrors

**Files:**
- Modify: `rulebooks/dnd5e/events/events.go`
- Modify: `rulebooks/dnd5e/events/events_test.go`
- Modify: `rulebooks/dnd5e/combat/damage.go`
- Modify: `rulebooks/dnd5e/combat/damage_test.go`

**Interfaces:**
- Produces: `DamageChainEvent.WeaponDamageDice`, `DamageChainEvent.WeaponDamageType`, and constructor/helper `NewDamageChainEvent` that derives deprecated `WeaponDamage` and `DamageType` mirrors.
- Preserves: `Components` as authoritative typed damage and `combat.FinalDamage` as the only multiplier arithmetic.

- [ ] **Step 1: Write RED tests for derived mirrors**

```go
func (s *EventsTestSuite) TestPrimaryMetadataDerivesLegacyMirrors() {
	evt := dnd5eEvents.NewDamageChainEvent(dnd5eEvents.DamageChainInput{
		WeaponDamageDice: "1d8", WeaponDamageType: damage.Slashing,
	})
	s.Equal("1d8", evt.WeaponDamage)
	s.Equal(damage.Slashing, evt.DamageType)
}
```

- [ ] **Step 2: Run and confirm RED**

Run: `cd rulebooks/dnd5e && go test ./events ./combat -run 'PrimaryMetadata|DamageChain' -count=1`

Expected: FAIL because the named primary fields and constructor are absent.

- [ ] **Step 3: Add the new fields and route every in-repository event construction through the constructor**

```go
type DamageChainEvent struct {
	Components       []DamageComponent
	WeaponDamageDice string
	WeaponDamageType damage.Type
	// Deprecated derived mirrors; never mutate in subscribers.
	WeaponDamage string
	DamageType   damage.Type
	// existing fields remain
}
```

Update `ResolveDamageInput` with the new names. Keep old input fields only where the legacy combat API requires source compatibility, and translate them once at construction.

- [ ] **Step 4: Run event and combat tests and confirm GREEN**

Run: `cd rulebooks/dnd5e && go test ./events ./combat -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add rulebooks/dnd5e/events rulebooks/dnd5e/combat
git commit -m "feat(dnd5e): add typed damage-chain primary metadata"
```

### Task 6: Migrate damage subscribers and critical feature dice

**Files:**
- Modify/Test: `rulebooks/dnd5e/conditions/fighting_style_great_weapon_fighting.go` and `_test.go`
- Modify/Test: `rulebooks/dnd5e/conditions/brutal_critical.go` and `_test.go`
- Modify/Test: `rulebooks/dnd5e/conditions/martial_arts.go` and `_test.go`
- Modify/Test: `rulebooks/dnd5e/conditions/raging.go` and `_test.go`
- Modify/Test: `rulebooks/dnd5e/conditions/sneak_attack.go` and `_test.go`
- Modify/Test: `rulebooks/dnd5e/conditions/fighting_style_dueling.go` and `_test.go`
- Modify/Test: `rulebooks/dnd5e/conditions/fighting_style_two_weapon_fighting.go` and `_test.go`

**Interfaces:**
- Consumes: authoritative typed components plus `WeaponDamageDice`/`WeaponDamageType` from Task 5.
- Produces: critical-aware Sneak Attack feature components, single-roll Brutal Critical components, and no authoritative reads from legacy mirrors.

- [ ] **Step 1: Write RED tests for each subscriber contract**

```go
func (s *SneakAttackTestSuite) TestCriticalRollsSneakDiceTwice() {
	evt := s.damageEvent(func(e *events.DamageChainEvent) {
		e.IsCritical = true
		e.WeaponDamageType = damage.Piercing
	})
	got := s.execute(evt, scripted(4, 5))
	component := featureComponent(got, refs.Features.SneakAttack())
	s.Equal([]int{4, 5}, component.FinalDiceRolls)
	s.True(component.IsCritical)
}

func (s *BrutalCriticalTestSuite) TestGrantedDieIsNotItselfDoubled() {
	component := s.executeCritical(scripted(11))
	s.Equal([]int{11}, component.FinalDiceRolls)
	s.False(component.IsCritical)
}
```

Add focused cases proving GWF rerolls only the marked primary weapon component, Martial Arts replaces that component, Rage/Sneak Attack inherit the primary type, Dueling/TWF use new metadata, and Rage resistance evaluates component types.

- [ ] **Step 2: Run condition tests and confirm RED**

Run: `cd rulebooks/dnd5e && go test ./conditions -run 'SneakAttack|BrutalCritical|GreatWeapon|MartialArts|Raging|Dueling|TwoWeapon' -count=1`

Expected: FAIL on new metadata and critical feature semantics.

- [ ] **Step 3: Migrate subscribers**

For Sneak Attack, roll `1 + boolToInt(event.IsCritical)` times and append both roll sets to one `DamageSourceFeature` component. Set component `IsCritical` only when doubled. For Brutal Critical, roll one die parsed from `WeaponDamageDice` and keep `IsCritical` false. Replace all reads of legacy mirrors with new primary metadata or component-local type/dice.

- [ ] **Step 4: Run the complete D&D module suite and confirm GREEN**

Run: `cd rulebooks/dnd5e && go test ./... -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add rulebooks/dnd5e/conditions
git commit -m "feat(dnd5e): make damage features pool-aware"
```

### Task 7: Prove the Lifedrinker extension seam

**Files:**
- Create: `rulebooks/dnd5e/combat/composable_damage_test.go`

**Interfaces:**
- Consumes: the damage-chain constructor and typed components.
- Produces: test-only synthetic StageFeatures subscriber; no production Lifedrinker type.

- [ ] **Step 1: Write a synthetic chain integration test**

```go
func (s *ComposableDamageTestSuite) TestFlatNecroticFeatureDoesNotDoubleOnCritical() {
	// Subscriber appends FlatBonus max(1, CHA modifier), Necrotic,
	// DamageSourceFeature, IsCritical false.
	got := s.foldCriticalPactLongsword(strengthMod(3), charismaMod(5))
	s.Equal(5, componentByType(got, damage.Necrotic).FlatBonus)
	s.False(componentByType(got, damage.Necrotic).IsCritical)
}
```

Also assert slashing vulnerability and necrotic resistance affect only their own components.

- [ ] **Step 2: Run and confirm RED**

Run: `cd rulebooks/dnd5e && go test ./combat -run Lifedrinker -count=1`

Expected: FAIL until the synthetic subscriber and assertions are completed.

- [ ] **Step 3: Complete only the test subscriber and fixture**

Use `DamageSourceFeature`, `damage.Necrotic`, `FlatBonus: max(1, charismaModifier)`, and `IsCritical: false`; do not add exported production code.

- [ ] **Step 4: Run and confirm GREEN**

Run: `cd rulebooks/dnd5e && go test ./combat -run 'Lifedrinker|ComposableDamage' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add rulebooks/dnd5e/combat/composable_damage_test.go
git commit -m "test(dnd5e): prove typed feature damage seam"
```

---

## Checkpoint 4: AttackProfile compilation and Strike resolution

### Task 8: Compile and validate canonical attack profiles

**Files:**
- Modify: `rulebooks/dnd5e/resolution/strike.go`
- Modify: `rulebooks/dnd5e/resolution/strictness_test.go`
- Modify: `rulebooks/dnd5e/resolution/character_attack.go`
- Modify: `rulebooks/dnd5e/resolution/character_attack_test.go`
- Create: `rulebooks/dnd5e/resolution/monster_attack_test.go`

**Interfaces:**
- Consumes: `[]damage.Damage`, weapon `DamageForGrip`, and canonical monster action config.
- Produces: `AttackProfile.Damage []damage.Damage`, `AbilityModifier int`, exact cross-field validation, `AttackFromCharacter`, and `AttackFromMonsterAction`.

- [ ] **Step 1: Write RED profile-invariant tests**

```go
func (s *StrictnessTestSuite) TestAbilityRequiresExactlyOneMarkedPool() {
	p := AttackProfile{Ref: refs.Weapons.Longsword(), AbilityUsed: abilities.STR, AbilityModifier: 3,
		Damage: []damage.Damage{{Dice: "1d8", Type: damage.Slashing}}}
	s.Error(p.validate())
}

func (s *StrictnessTestSuite) TestMonsterProfileRejectsAbilityMarker() {
	p := AttackProfile{Ref: refs.MonsterActions.Bite(), Damage: []damage.Damage{{
		Dice: "2d4", Type: damage.Piercing,
		Properties: []damage.Property{damage.AddsAttackAbilityModifier},
	}}}
	s.Error(p.validate())
}
```

Add compiler cases for ordinary/finesse/versatile character attacks, positive/zero/negative intrinsic monster bonuses, canonical-first precedence, bite gate preservation, scimitar, and ranged refusal.

- [ ] **Step 2: Run resolution compiler tests and confirm RED**

Run: `cd rulebooks/dnd5e/resolution && go test ./... -run 'Strictness|AttackFromCharacter|AttackFromMonster' -count=1`

Expected: FAIL because the profile is singular and fused.

- [ ] **Step 3: Implement the profile shape and compilers**

```go
type AttackProfile struct {
	Ref             *core.Ref
	AttackBonus     int
	Damage          []damage.Damage
	AbilityUsed     abilities.Ability
	AbilityModifier int
	Gate            *saves.SaveGate
	Imposes         Consequence
}
```

Character compilation copies `DamageForGrip`, sets ability fields, and never fuses the modifier into dice. Monster compilation copies canonical pools, leaves ability fields empty/zero, preserves gate/consequence, and refuses ranged actions. `validate` calls `damage.Validate` and enforces: non-empty ability means exactly one marker; empty ability means zero modifier and no marker.

- [ ] **Step 4: Run compiler tests and confirm GREEN**

Run: `cd rulebooks/dnd5e/resolution && go test ./... -run 'Strictness|CharacterAttack|MonsterAttack' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add rulebooks/dnd5e/resolution/strike.go rulebooks/dnd5e/resolution/strictness_test.go rulebooks/dnd5e/resolution/character_attack.go rulebooks/dnd5e/resolution/character_attack_test.go rulebooks/dnd5e/resolution/monster_attack_test.go
git commit -m "feat(resolution): compile canonical attack profiles"
```

### Task 9: Roll multiple pools once through Strike and preserve typed outcomes

**Files:**
- Modify: `rulebooks/dnd5e/resolution/strike.go`
- Modify: `rulebooks/dnd5e/resolution/strike_test.go`
- Modify: `rulebooks/dnd5e/resolution/damage_custody_test.go`
- Modify: `rulebooks/dnd5e/resolution/testrollers_test.go`

**Interfaces:**
- Produces: `StrikeOutcome.DamageInstances []damage.Instance` and `StrikeOutcome.DamageComponents []dnd5eEvents.DamageComponent`.
- Consumes: one `AttackProfile.Damage` slice, `combat.FinalDamage`, and one `ApplyDamage` call.

- [ ] **Step 1: Write headline RED tests**

```go
func (s *StrikeTestSuite) TestTwoPoolsUseOneFoldAndOneApplication() {
	profile := oozeProfile(
		damage.Damage{Dice: "1d8", Type: damage.Bludgeoning, FlatBonus: 2},
		damage.Damage{Dice: "1d6", Type: damage.Acid},
	)
	out := s.resolveStrike(profile, scripted(15, 4, 5))
	s.Equal(11, out.Damage)
	s.Len(out.DamageInstances, 2)
	s.Len(out.DamageComponents, 2)
	s.Equal(1, s.damageGatherCount())
	s.Equal(1, s.applyDamageCount())
}

func (s *StrikeTestSuite) TestCriticalDoublesEveryEligiblePoolButNoFlatBonus() {
	profile := oozeProfile(
		damage.Damage{Dice: "1d8", Type: damage.Bludgeoning, FlatBonus: 2},
		damage.Damage{Dice: "1d6", Type: damage.Acid},
	)
	out := s.resolveStrike(profile, scripted(20, 4, 5, 6, 3))
	s.Equal(20, out.Damage)
	s.True(out.DamageComponents[0].IsCritical)
	s.True(out.DamageComponents[1].IsCritical)
}
```

Add cases for `DoesNotCrit`, ability modifier attached only to the marked pool, mixed vulnerability/immunity, and no randomness consumed on invalid profiles.

- [ ] **Step 2: Run Strike tests and confirm RED**

Run: `cd rulebooks/dnd5e/resolution && go test ./... -run 'TwoPools|CriticalDoubles|DoesNotCrit|TypedOutcome|InvalidProfile' -count=1`

Expected: FAIL because `Strike` rolls one fused pool and returns aggregate damage only.

- [ ] **Step 3: Replace singular rolling with component construction**

For each pool: parse pure dice, roll once, roll again only for eligible critical dice, append a weapon component carrying notation, rolls, `FlatBonus`, type, properties, source ref, and actual `IsCritical`. Append one ability-source flat component to the marked pool's type. Construct one damage event with effective advantage:

```go
effectiveAdvantage := len(folded.AdvantageSources) > 0 && len(folded.DisadvantageSources) == 0
```

After the one fold, call `combat.FinalDamage` once, copy results into `[]damage.Instance`, convert once to `[]combat.DamageInstance`, call `ApplyDamage` once, and store aggregate plus typed instances/components on the outcome.

- [ ] **Step 4: Run the full resolution suite and confirm GREEN**

Run: `cd rulebooks/dnd5e/resolution && go test ./... -count=1`

Expected: PASS, including canceled advantage not granting Sneak Attack.

- [ ] **Step 5: Commit**

```bash
git add rulebooks/dnd5e/resolution
git commit -m "feat(resolution): fold composable strike damage once"
```

---

## Checkpoint 5: Encounter projection and session boundary

### Task 10: Project canonical action damage into encounter readiness snapshots

**Files:**
- Modify: `encounter/seed_monsters.go`
- Modify: `encounter/seed_monsters_test.go`
- Modify: `encounter/monster_fixture_test.go`

**Interfaces:**
- Consumes: persisted canonical `damage` arrays and the complete legacy pair.
- Produces: deterministic singular `(attackBonus, damageDice, damageType)` projection for OA readiness only.

- [ ] **Step 1: Write RED projection tests**

```go
func (s *SeedMonstersTestSuite) TestCanonicalDamageSeedsOAReadiness() {
	action := actionData(`{"attack_bonus":4,"damage":[{"dice":"1d6","type":"acid"},{"dice":"1d8","type":"slashing","flat_bonus":-1,"properties":["adds-attack-ability-modifier"]}]}`)
	bonus, notation, kind := primaryAttackSnapshot(monsterWith(action))
	s.Equal(4, bonus)
	s.Equal("1d8-1", notation)
	s.Equal("slashing", kind)
}
```

Add cases for `+2`, zero, no marker (first pool), canonical-invalid beside valid legacy (skip, no fallback), partial legacy (skip), and first eligible action wins.

- [ ] **Step 2: Run and confirm RED**

Run: `cd encounter && go test ./... -run 'CanonicalDamage|PrimaryAttackSnapshot|OAReadiness' -count=1`

Expected: FAIL because the private decoder reads only `damage_dice`.

- [ ] **Step 3: Implement new-first private decoding and projection**

Extend `attackSnapshot` with `Damage []damage.Damage`. Validate canonical pools. Choose the marked pool or first pool; format `Dice`, `Dice+N`, or `Dice-N`. With no canonical pools, accept only a complete valid legacy pair. Invalid data makes that action ineligible. Do not change real resolution or add multi-pool fields to `MonsterInput`.

- [ ] **Step 4: Run the encounter suite and confirm GREEN**

Run: `cd encounter && go test ./... -count=1`

Expected: PASS and canonical-only monsters seed opportunity-attack readiness.

- [ ] **Step 5: Commit**

```bash
git add encounter/seed_monsters.go encounter/seed_monsters_test.go encounter/monster_fixture_test.go
git commit -m "feat(encounter): project canonical action damage"
```

### Task 11: Keep session recording aggregate-only while consuming typed StrikeOutcome

**Files:**
- Modify: `rulebooks/dnd5e/session/attack.go`
- Modify: `rulebooks/dnd5e/session/attack_test.go`
- Modify: `rulebooks/dnd5e/session/attack_internal_test.go`

**Interfaces:**
- Consumes: expanded `resolution.StrikeOutcome`.
- Preserves: `encounter.RecordInput.Values[encounter.ValueAmount] = struck.Damage`; no typed encounter-history schema.

- [ ] **Step 1: Add a RED boundary test**

```go
func (s *AttackInternalTestSuite) TestRecordUsesAggregateFromTypedStrikeOutcome() {
	struck := resolution.StrikeOutcome{
		Hit: true, Damage: 9,
		DamageInstances: []damage.Instance{{Amount: 5, Type: damage.Slashing}, {Amount: 4, Type: damage.Fire}},
	}
	record := recordFor(s.attackInput(), struck)
	s.Equal(9, record.Values[encounter.ValueAmount])
}
```

- [ ] **Step 2: Run and confirm RED against the new resolution dependency**

Run: `cd rulebooks/dnd5e/session && go test ./... -run 'AggregateFromTyped|RecordFor' -count=1`

Expected: FAIL until the module consumes the expanded outcome cleanly.

- [ ] **Step 3: Keep the adapter deliberately aggregate-only**

Do not add typed fields to encounter outcomes. Update imports/types required by the new resolution version and leave the write contract as:

```go
if struck.Hit {
	kind = encounter.OutcomeStruck
	values[encounter.ValueAmount] = struck.Damage
}
```

- [ ] **Step 4: Run the session suite and confirm GREEN**

Run: `cd rulebooks/dnd5e/session && go test ./... -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add rulebooks/dnd5e/session/attack.go rulebooks/dnd5e/session/attack_test.go rulebooks/dnd5e/session/attack_internal_test.go
git commit -m "test(session): pin aggregate strike recording"
```

---

## Checkpoint 6: Cross-module integration, compatibility, and release

### Task 12: Verify the complete repository and publish in dependency order

**Files:**
- Modify only when version bumps are ready: `rulebooks/dnd5e/resolution/go.mod`, `encounter/go.mod`, `rulebooks/dnd5e/session/go.mod`
- Verify: every `go.mod`, `go.sum`, and repository documentation touched above

**Interfaces:**
- Produces: releasable module commits with no local workspace artifacts.

- [ ] **Step 1: Create an uncommitted local workspace for integration verification**

```bash
go work init ./rulebooks/dnd5e ./rulebooks/dnd5e/resolution ./encounter ./rulebooks/dnd5e/session
```

Confirm `git status --short` lists only `go.work`/`go.work.sum` beyond intended commits.

- [ ] **Step 2: Run focused cross-module regressions**

```bash
(cd rulebooks/dnd5e && go test ./damage ./weapons ./monster/... ./events ./combat ./conditions -count=1)
(cd rulebooks/dnd5e/resolution && go test ./... -count=1)
(cd encounter && go test ./... -run 'PrimaryAttackSnapshot|OAReadiness|SeedMonster' -count=1)
(cd rulebooks/dnd5e/session && go test ./... -run 'Attack|Record' -count=1)
```

Expected: all PASS.

- [ ] **Step 3: Run repository-wide verification**

```bash
make fmt-all
make lint-all
make test-all
./scripts/check-decisions.sh
git diff --check
```

Expected: every command exits 0; all ADRs are summarized; no formatting errors.

- [ ] **Step 4: Prove no compatibility artifacts can be committed**

```bash
rm go.work go.work.sum
! find . -name go.work -o -name go.work.sum | grep .
! rg '^replace .*=> \.\.' --glob 'go.mod'
git status --short
```

Expected: no `go.work`, `go.work.sum`, or local sibling `replace`; only intended source/test/doc changes remain.

- [ ] **Step 5: Request final code review before release commits**

Review against the approved spec for: one fold/application, critical eligibility, Sneak Attack/Brutal Critical distinction, canonical-first persistence, encounter OA readiness, typed outcome boundary, versatile weapons, and canceled advantage.

- [ ] **Step 6: Publish and bump modules in exact order**

```text
1. Publish rulebooks/dnd5e.
2. Update resolution and top-level encounter to that released version; test and publish each.
3. Update session to the released dnd5e and resolution versions; test and publish session.
4. Publish rulebooks/dnd5e/encounter only if its code or go.mod actually changed.
```

Use the repository's `make release-module MODULE=<path> VERSION=<version>` flow and never invent version numbers without checking existing tags.

- [ ] **Step 7: Run post-bump verification and commit dependency updates**

```bash
make test-all
make lint-all
git diff --check
git add rulebooks/dnd5e/resolution/go.mod rulebooks/dnd5e/resolution/go.sum encounter/go.mod encounter/go.sum rulebooks/dnd5e/session/go.mod rulebooks/dnd5e/session/go.sum
git commit -m "chore: consume composable damage module releases"
```

Expected: all commands pass and the final commit contains published module versions only.
