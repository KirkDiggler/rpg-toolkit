# Actions Are Data Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to execute this live cross-PR plan task-by-task. Keep each checkbox current on the open idea PR and stop at every provider-tag checkpoint.

**Goal:** Replace executable producer-specific action objects with one shared, serializable action-definition contract consumed by the active D&D 5e resolution/session stack.

**Architecture:** `rulebooks/dnd5e/combat/actions` is a data-only package in the existing D&D 5e module. Monster factories author `actions.Definition` values directly; `character.AssembleAttack` derives the same value from a sheet and equipment; resolution dispatches the populated profile arm to Strike and never switches on content refs. The migration is a hard cut with no loader, adapter, second JSON shape, or forwarding action object.

**Tech Stack:** Go 1.24.1, standard `encoding/json`, testify suites, independently versioned Go modules in one repository, module-isolated GitHub PRs, squash merges, and CI-generated stable tags.

**Spec:** `docs/ideas/actions-are-data/design.md`

**Decision record:** `docs/adr/0045-actions-are-data.md`

**Delivery correction (2026-08-24):** Tasks 1–8 record implementation and RED/GREEN evidence completed on quarantined draft PR #1232. Their original single-branch commit/push commands are historical and must not be replayed. Task 9 is the canonical delivery procedure: reconstruct the reviewed content on fresh module-isolated branches, squash providers, and replace temporary consumer pins with CI-generated stable tags before each consumer merges.

## Global Constraints

- Prerequisite issue #1215 must be merged before cutting the implementation branch.
- PR #1214 is the approved idea review surface. Keep it open throughout implementation; merge it only after all module implementation PRs land and `implementation.md` records what actually shipped.
- Every changed Go module gets a fresh branch and PR from current `origin/main`. A replacement PR may copy the reconciled content of draft PR #1232, but it must not inherit #1232's cross-module commit graph.
- PR #1232 is an invalid delivery artifact, not a merge candidate. Keep it draft until every changed file is assigned to exactly one replacement PR, then close it.
- Keep `design.md` and `plan.md` live on PR #1214. If implementation invalidates either document, stop, update the idea branch, obtain Kirk's approval for the changed boundary, and only then continue.
- Do not create `implementation.md` before implementation completes; it records observed results and nuances rather than predictions.
- Do not add a `go.mod` under `rulebooks/dnd5e/combat/actions`; it is part of `rulebooks/dnd5e`.
- `combat/actions` must not directly import monster, character, resolution, or `github.com/KirkDiggler/rpg-toolkit/events`.
- Implement only the `Attack` profile arm. Do not scaffold save-area, healing, or sequence profiles before their machines exist.
- Multiattack definitions are removed from current monster factories; no sequence compatibility object survives.
- Preserve ADR-0041: ordered typed damage pools, at most one ability-marked pool, one fold, one application.
- Preserve ADR-0039: `SaveGate` stays data and every contested condition resolves through the save machine.
- Validate definition, condition declarations, actor/target, and delivery range before cost, dice, or participant mutation.
- Use feet in authored profiles and convert to cells only at the comparison boundary.
- Delete old executable actions and stale tests; do not retain deprecated aliases or legacy JSON translation.
- All replacement PRs use the repository's normal squash merge. Never require a merge-commit exception.
- Open module PRs together for concurrent review. A consumer may temporarily pin a pushed provider-branch pseudo-version while both PRs are open, but it must replace that pin with the stable tag generated after the provider's squash merge before the consumer merges.
- Deliver in dependency order: root D&D and encounter first, then resolution, then session. Wait for the main-branch auto-tag workflow after each provider merge and verify the exact generated tag rather than predicting it.
- Repository-wide documentation changes belong to PR #1214 after all code PRs land. Module-local README and architecture files may travel with their owning module.
- Every task follows red-green-refactor: observe the focused test fail for the intended reason before implementation.

---

### Task 1: Add the Shared Action Definition Contract

**Completed:** `fa03101` (`feat(actions)!: define inert shared attack profiles (#1198)`)

**Files:**
- Create: `rulebooks/dnd5e/combat/actions/doc.go`
- Create: `rulebooks/dnd5e/combat/actions/definition.go`
- Create: `rulebooks/dnd5e/combat/actions/attack.go`
- Create: `rulebooks/dnd5e/combat/actions/definition_test.go`
- Create: `rulebooks/dnd5e/combat/actions/attack_test.go`
- Create: `rulebooks/dnd5e/combat/actions/architecture_test.go`
- Modify: `rulebooks/dnd5e/combat/spend_profile.go`

**Interfaces:**
- Produces: `actions.Definition`, `actions.AttackProfile`, `actions.AttackDelivery`, `actions.ConditionApplication`, `Definition.Validate()`, `Definition.Clone()`, `actions.CloneSpendProfile()`, `AttackDelivery.IsMelee()`, `AttackDelivery.MaxRangeFeet()`, and `AttackDelivery.NormalRangeFeet()`.
- Consumes: existing `combat.SpendProfile`, `damage.Damage`, `saves.SaveGate`, `abilities.Ability`, and `core.Ref`.

- [x] **Step 1: Write failing contract and round-trip tests**

Create tests covering the exact public shapes and invariants:

```go
func TestDefinitionRoundTrip(t *testing.T) {
    original := actions.Definition{
        Ref:  *refs.Weapons.Longsword(),
        Name: "Longsword",
        Cost: &combat.SpendProfile{
            Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 1},
        },
        Attack: &actions.AttackProfile{
            Category: actions.AttackCategoryWeapon,
            Delivery: actions.AttackDelivery{
                Melee: &actions.MeleeDelivery{ReachFeet: 5},
            },
            AttackBonus: 5,
            Ability: &actions.AbilityContribution{
                Ability: abilities.STR,
                Modifier: 3,
            },
            Weapon: &actions.WeaponContext{Ref: refs.Weapons.Longsword()},
            Damage: []damage.Damage{{
                Dice:       "1d8",
                Type:       damage.Slashing,
                Properties: []damage.Property{damage.AddsAttackAbilityModifier},
            }},
            OnHit: []actions.ConditionApplication{{
                Ref:  *refs.Conditions.Prone(),
                Save: saves.NewSaveGate(abilities.STR, 11),
            }},
        },
    }

    raw, err := json.Marshal(original)
    require.NoError(t, err)

    var decoded actions.Definition
    require.NoError(t, json.Unmarshal(raw, &decoded))
    require.NoError(t, decoded.Validate())
    assert.Equal(t, original, decoded)
}
```

Define the valid baseline used by the table:

```go
func validAttackProfile() actions.AttackProfile {
    return actions.AttackProfile{
        Category:    actions.AttackCategoryWeapon,
        Delivery:    actions.AttackDelivery{Melee: &actions.MeleeDelivery{ReachFeet: 5}},
        AttackBonus: 5,
        Ability:     &actions.AbilityContribution{Ability: abilities.STR, Modifier: 3},
        Weapon:      &actions.WeaponContext{Ref: refs.Weapons.Longsword()},
        Damage: []damage.Damage{{
            Dice:       "1d8",
            Type:       damage.Slashing,
            Properties: []damage.Property{damage.AddsAttackAbilityModifier},
        }},
    }
}
```

Add table tests that reject:

```go
func TestAttackProfileValidation(t *testing.T) {
    cases := []struct {
        name    string
        mutate  func(*actions.AttackProfile)
        message string
    }{
        {"no delivery", func(p *actions.AttackProfile) { p.Delivery = actions.AttackDelivery{} }, "exactly one delivery"},
        {"two deliveries", func(p *actions.AttackProfile) {
            p.Delivery.Ranged = &actions.RangedDelivery{NormalFeet: 20}
        }, "exactly one delivery"},
        {"invalid melee reach", func(p *actions.AttackProfile) { p.Delivery.Melee.ReachFeet = 0 }, "positive reach"},
        {"invalid ranged bracket", func(p *actions.AttackProfile) {
            p.Delivery = actions.AttackDelivery{Ranged: &actions.RangedDelivery{NormalFeet: 80, LongFeet: 40}}
        }, "long range"},
        {"ability without marked pool", func(p *actions.AttackProfile) {
            p.Damage[0].Properties = nil
        }, "exactly one ability-marked damage pool"},
        {"empty outcome", func(p *actions.AttackProfile) { p.Damage = nil; p.OnHit = nil }, "damage or an on-hit condition"},
        {"condition save-for-half", func(p *actions.AttackProfile) {
            p.OnHit = []actions.ConditionApplication{{
                Ref: *refs.Conditions.Prone(),
                Save: &saves.SaveGate{Abilities: []abilities.Ability{abilities.STR}, DC: saves.DCStatic(11), OnSuccess: saves.Half},
            }}
        }, "condition save must negate"},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            profile := validAttackProfile()
            tc.mutate(&profile)
            err := profile.Validate()
            require.Error(t, err)
            assert.Contains(t, err.Error(), tc.message)
        })
    }
}
```

Add a no-damage net-shaped case that passes because `OnHit` is non-empty. Add clone tests that mutate nested maps, damage properties, save abilities, parameters, and refs in the clone and prove the original is unchanged.

- [x] **Step 2: Run the focused tests and verify RED**

Run:

```bash
cd rulebooks/dnd5e
go test ./combat/actions
```

Expected: FAIL because `combat/actions` does not exist.

- [x] **Step 3: Implement the shared types and strict validation**

Use these exact signatures:

```go
package actions

type Definition struct {
    Ref    core.Ref             `json:"ref"`
    Name   string               `json:"name"`
    Cost   *combat.SpendProfile `json:"cost,omitempty"`
    Attack *AttackProfile       `json:"attack,omitempty"`
}

func (d Definition) Validate() error
func (d Definition) Clone() Definition
func CloneSpendProfile(in *combat.SpendProfile) *combat.SpendProfile

type AttackCategory string

const (
    AttackCategoryWeapon AttackCategory = "weapon"
    AttackCategorySpell  AttackCategory = "spell"
)

type AttackProfile struct {
    Category    AttackCategory          `json:"category"`
    Delivery    AttackDelivery          `json:"delivery"`
    AttackBonus int                     `json:"attack_bonus"`
    Ability     *AbilityContribution    `json:"ability,omitempty"`
    Weapon      *WeaponContext          `json:"weapon,omitempty"`
    Damage      []damage.Damage         `json:"damage,omitempty"`
    OnHit       []ConditionApplication  `json:"on_hit,omitempty"`
}

func (p AttackProfile) Validate() error
func (p AttackProfile) Clone() AttackProfile

type AbilityContribution struct {
    Ability  abilities.Ability `json:"ability"`
    Modifier int               `json:"modifier"`
}

type WeaponContext struct {
    Ref              *core.Ref `json:"ref,omitempty"`
    TwoHanded        bool      `json:"two_handed"`
    OffHandWeaponRef *core.Ref `json:"off_hand_weapon_ref,omitempty"`
}

type AttackDelivery struct {
    Melee  *MeleeDelivery  `json:"melee,omitempty"`
    Ranged *RangedDelivery `json:"ranged,omitempty"`
}

type MeleeDelivery struct {
    ReachFeet int `json:"reach_feet"`
}

type RangedDelivery struct {
    NormalFeet int `json:"normal_feet"`
    LongFeet   int `json:"long_feet,omitempty"`
}

func (d AttackDelivery) Validate() error
func (d AttackDelivery) IsMelee() bool
func (d AttackDelivery) MaxRangeFeet() int
func (d AttackDelivery) NormalRangeFeet() int

type ConditionApplication struct {
    Ref        core.Ref        `json:"ref"`
    Parameters json.RawMessage `json:"parameters,omitempty"`
    Save       *saves.SaveGate `json:"save,omitempty"`
}

func (a ConditionApplication) Validate() error
func (a ConditionApplication) Clone() ConditionApplication
```

Validation rules are the ADR rules, not defaults:

- `Definition.Ref` must have module, type, and ID; `Name` must be non-empty; exactly `Attack` is populated in this first implementation; when `Cost` is non-nil, `Cost.Validate()` must pass.
- Weapon/spell are the only categories. Spell attacks reject non-nil `Weapon`.
- Exactly one delivery arm is non-nil. Melee reach is positive. Ranged normal range is positive; long range is zero or at least normal range.
- If `Ability` is present, it names a real ability and exactly one pool has `damage.AddsAttackAbilityModifier`; if absent, no pool has that marker.
- Call `damage.Validate` only when damage is non-empty. Require damage or `OnHit`.
- Every condition ref must be `dnd5e:conditions:*`; a non-nil save must validate and must use `saves.Negated`.

Add explicit JSON tags to all `combat.SpendProfile` maps so the definition round trip writes `slots`, `capacity`, `grants`, `pools`, and `requires` rather than Go field names.

- [x] **Step 4: Add the import-boundary test**

Parse non-test Go files in `combat/actions` with `go/parser`. Fail if an import equals or begins with:

```go
var forbidden = []string{
    "github.com/KirkDiggler/rpg-toolkit/events",
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character",
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster",
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution",
}
```

The test must inspect production files only; do not exempt an offending import with a comment or lint directive.

- [x] **Step 5: Run focused and root-module verification**

Run:

```bash
cd rulebooks/dnd5e
gofmt -w combat/actions combat/spend_profile.go
go test ./combat/actions ./combat
golangci-lint run ./combat/actions ./combat/...
git diff --check
```

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add rulebooks/dnd5e/combat/actions rulebooks/dnd5e/combat/spend_profile.go
git commit -m "feat(actions)!: define inert shared attack profiles (#1198)"
```

---

### Task 2: Move Character Attack Assembly Beside the Character

**Completed:** `e6ba2a3` (`feat(character)!: assemble shared attack definitions (#1198)`)

**Files:**
- Create: `rulebooks/dnd5e/character/attack_definition.go`
- Create: `rulebooks/dnd5e/character/attack_definition_test.go`
- Modify: `rulebooks/dnd5e/character/spend_profile.go`
- Modify: `rulebooks/dnd5e/character/spend_profile_test.go`
- Test reference: `rulebooks/dnd5e/resolution/character_attack_test.go`

**Interfaces:**
- Consumes: `actions.Definition` from Task 1, character equipment slots, weapon catalog data, and caller-supplied `*combat.SpendProfile`.
- Produces: `character.AssembleAttack(*Character, *AssembleAttackInput) (actions.Definition, error)` and `character.CostOfSwing(*Character) (*combat.SpendProfile, error)`.

- [x] **Step 1: Write failing character-assembly tests**

Move `CharacterAttackTestSuite.heroSheet`, `martialHero`, and `load` from `resolution/character_attack_test.go` into `character/attack_definition_test.go`; change the suite package references from `character.X` to local character symbols. Port the existing compiler cases into that suite and add ranged coverage:

```go
type AssembleAttackInput struct {
    Slot      InventorySlot
    TwoHanded bool
    Cost      *combat.SpendProfile
}

func (s *CharacterAttackTestSuite) TestAssembleAttack_LongbowIsRanged() {
    data := s.heroSheet(
        []proficiencies.Weapon{proficiencies.WeaponMartial},
        map[InventorySlot]string{SlotMainHand: string(weapons.Longbow)},
    )
    ch := s.load(data)
    cost := &combat.SpendProfile{Capacity: map[combat.CapacityType]int{combat.CapacityAttack: 1}}

    def, err := AssembleAttack(ch, &AssembleAttackInput{Slot: SlotMainHand, Cost: cost})
    require.NoError(t, err)
    require.NotNil(t, def.Attack)
    assert.Equal(t, actions.AttackCategoryWeapon, def.Attack.Category)
    assert.Equal(t, &actions.RangedDelivery{NormalFeet: 150, LongFeet: 600}, def.Attack.Delivery.Ranged)
    assert.Equal(t, abilities.DEX, def.Attack.Ability.Ability)
    assert.Equal(t, refs.Weapons.Longbow(), def.Attack.Weapon.Ref)
    assert.Equal(t, cost, def.Cost)
}
```

Retain tests for empty-hand unarmed substitution, corrupt equipment refusal, finesse STR/DEX choice, proficiency, versatile grip, Reach property, multiple damage pools, `TwoHanded`, and `OffHandWeaponRef`. Add a clone assertion proving the returned definition does not alias the supplied cost maps or weapon catalog damage slices.

Add `CostOfSwing` tests for the first and already-banked swings, and move the `asOnePayment` cases from `session/economy_internal_test.go`: the first swing nets the Attack grant against one Strike capacity, while an already-banked attack costs only one capacity.

- [x] **Step 2: Run the focused tests and verify RED**

Run:

```bash
cd rulebooks/dnd5e
go test ./character -run 'TestAssembleAttack|TestCostOfSwing'
```

Expected: FAIL because `AssembleAttack` and `CostOfSwing` do not exist in `character`.

- [x] **Step 3: Move and generalize the assembly implementation**

Move static weapon/sheet logic from `resolution/character_attack.go` into `character/attack_definition.go`. Return:

```go
return combatActions.Definition{
    Ref:  *weaponRef,
    Name: weapon.Name,
    Cost: combatActions.CloneSpendProfile(in.Cost),
    Attack: &combatActions.AttackProfile{
        Category:    combatActions.AttackCategoryWeapon,
        Delivery:    deliveryForWeapon(weapon),
        AttackBonus: attackBonus,
        Ability: &combatActions.AbilityContribution{
            Ability:  ability,
            Modifier: modifier,
        },
        Weapon: &combatActions.WeaponContext{
            Ref:              weaponRef,
            TwoHanded:        in.TwoHanded || weapon.HasProperty(weapons.PropertyTwoHanded),
            OffHandWeaponRef: otherHandWeaponRef(c, in.Slot),
        },
        Damage: copyDamagePools(pools),
    },
}, nil
```

`deliveryForWeapon` returns melee reach 5/10 for melee weapons and catalog `Range.Normal`/`Range.Long` for ranged weapons. A thrown melee weapon remains melee in this slice because no throw choice exists in the input; do not infer a throw merely because `Weapon.Range` is non-nil.

Move `costOfSwing`, `asOnePayment`, `sum`, and `larger` from session into `character.CostOfSwing`. Keep free-roam policy in session by passing nil `Cost`; character only composes a requested price.

- [x] **Step 4: Run focused and package tests**

Run:

```bash
cd rulebooks/dnd5e
gofmt -w character/attack_definition.go character/attack_definition_test.go character/spend_profile.go
go test -race ./character
golangci-lint run ./character/...
```

Expected: PASS.

- [x] **Step 5: Commit**

```bash
git add rulebooks/dnd5e/character/attack_definition.go \
        rulebooks/dnd5e/character/attack_definition_test.go \
        rulebooks/dnd5e/character/spend_profile.go \
        rulebooks/dnd5e/character/spend_profile_test.go
git commit -m "feat(character)!: assemble shared attack definitions (#1198)"
```

---

### Task 3: Remove Executable Character Action Objects

**Completed:** `8605911` (`refactor(actions)!: delete executable character actions (#1198)`)

**Files:**
- Delete: `rulebooks/dnd5e/actions/`
- Delete: `rulebooks/dnd5e/character/actions_test.go`
- Modify: `rulebooks/dnd5e/character/character.go`
- Modify: `rulebooks/dnd5e/character/draft.go`
- Modify: `rulebooks/dnd5e/character/sheet_keeper.go`
- Modify: `rulebooks/dnd5e/character/action_economy.go`
- Modify: `rulebooks/dnd5e/character/action_economy_types.go`
- Modify: `rulebooks/dnd5e/character/action_economy_test.go`
- Modify: `rulebooks/dnd5e/character/economy_dirty_test.go`
- Modify: `rulebooks/dnd5e/character/combat_abilities_test.go`
- Modify: `rulebooks/dnd5e/character/integration_test.go`
- Modify: `rulebooks/dnd5e/character/sheet_keeper_test.go`
- Modify: `rulebooks/dnd5e/combat/action_economy.go`
- Modify: `rulebooks/dnd5e/combat/capacity.go`
- Modify: `rulebooks/dnd5e/combat/turn_manager.go`
- Modify: `rulebooks/dnd5e/combat/turn_manager_queries.go`
- Modify: `rulebooks/dnd5e/combatabilities/ability.go`
- Modify: `rulebooks/dnd5e/combatabilities/attack.go`
- Modify: `rulebooks/dnd5e/combatabilities/input.go`
- Modify: `rulebooks/dnd5e/features/flurry_of_blows.go`
- Modify: `rulebooks/dnd5e/features/flurry_of_blows_test.go`
- Modify: `rulebooks/dnd5e/events/events.go`
- Modify: `rulebooks/dnd5e/integration/monk_encounter_test.go`

**Interfaces:**
- Consumes: character's existing persisted `ActionEconomyData` and `combat.Ledger` implementation.
- Produces: no action-object API. Turn lifecycle, ledger payment, and capacity grants remain data operations.

- [x] **Step 1: Write the replacement Flurry capacity test**

Replace action-grant assertions with persisted economy assertions:

```go
func (s *FlurryOfBlowsTestSuite) TestActivateSpendsKiAndBanksTwoFlurryStrikes() {
    before := s.character.GetResource(resources.Ki).Current()

    err := s.feature.Activate(s.ctx, s.character, features.FeatureInput{})
    s.Require().NoError(err)
    s.Equal(before-1, s.character.GetResource(resources.Ki).Current())
    s.Equal(2, s.character.CapacityLeft(combat.CapacityFlurryStrike))
}
```

Add a refusal test proving zero Ki leaves `CapacityFlurryStrike` unchanged. Do not subscribe to or assert `ActionGrantedTopic`.

- [x] **Step 2: Run the focused test and verify RED**

Run:

```bash
cd rulebooks/dnd5e
go test ./features -run FlurryOfBlows
```

Expected: FAIL because Flurry still creates and publishes executable `FlurryStrike` objects.

- [x] **Step 3: Replace Flurry action objects with capacity state**

Replace the test's `mockMonkCharacter.actions` slice and four `ActionHolder` methods with a `map[combat.CapacityType]int`, plus these exact methods:

```go
func (m *mockMonkCharacter) BankCapacity(key combat.CapacityType, n int) {
    if m.capacity == nil {
        m.capacity = make(map[combat.CapacityType]int)
    }
    m.capacity[key] += n
}

func (m *mockMonkCharacter) CapacityLeft(key combat.CapacityType) int {
    return m.capacity[key]
}
```

In `FlurryOfBlows.Activate`, require the owner to satisfy resource access and the one capability this feature needs:

```go
type flurryOwner interface {
    coreResources.ResourceAccessor
    BankCapacity(combat.CapacityType, int)
}
```

After `CanActivate`, spend one Ki through `UseResource`, then call:

```go
owner.BankCapacity(combat.CapacityFlurryStrike, 2)
```

Because `BankCapacity` cannot fail and marks the sheet, no bus or rollback action object is needed. Keep bonus-action consumption in the existing feature-activation caller; this method owns Ki and granted capacity only.

- [x] **Step 4: Remove the superseded runtime action surface**

Delete the entire `rulebooks/dnd5e/actions` directory. Then remove:

- `Character.actions`, the `actions.ActionHolder` assertion, `AddAction`, `RemoveAction`, `GetActions`, and `GetAction`.
- Action initialization from `Draft.initializeStandardActions` and its call.
- `ActionGrantedTopic`/`ActionRemovedTopic` subscriptions and handlers from `SheetKeeper`/`Character`.
- Temporary-action cleanup from character turn/end cleanup.
- `CombatAbilityInput.ActionHolder`.
- `ActionGrantedEvent`, `ActionRemovedEvent`, `StrikeExecutedEvent`, Flurry/OffHand requested/activated events, and their topics when `rg` confirms the deleted action package was their only production publisher.

Remove the no-longer-consumed menu/execution API from `action_economy_types.go` and `action_economy.go`:

```text
AvailableAction
ExecuteActionInput / ExecuteActionOutput
Character.ExecuteAction
buildAvailableActions
executeStrike / executeOffHandStrike / executeFlurryStrike / executeUnarmedStrike / executeMove
checkPostStrikeGrants
```

Also remove `Actions []AvailableAction` fields from turn/activation outputs. Remove `combat.TurnManager`'s dead `ActionInfo`/`GetAvailableActions` query seam rather than forcing Character to return an empty compatibility list. Preserve `ActionEconomyData`, `StartTurn`, `RefreshForTurn`, `EndTurn`, `ExitCombat`, `GrantCapacity`, `combat.Ledger`, and cost compilation.

Delete action-object lifecycle tests. In `action_economy_test.go`, retain persistence and turn-lifecycle cases through `TestEndTurn_ResetsButStaysInCombat`; remove the old menu/ActivateAbility/ExecuteAction cases. In `economy_dirty_test.go`, retain direct ledger/turn/resource dirty tests and remove cases whose only entry is the deleted action execution API.

- [x] **Step 5: Prove no executable action object remains**

Run:

```bash
cd rulebooks/dnd5e
if rg -n 'actions\.Action|ActionHolder|ActionGrantedTopic|ActionRemovedTopic|StrikeExecutedTopic|FlurryStrikeRequestedTopic|OffHandStrikeRequestedTopic' \
  --glob '*.go' --glob '!**/*_test.go'; then
  echo 'superseded executable action surface remains' >&2
  exit 1
fi
go test -race ./character ./features ./combatabilities ./events
golangci-lint run ./character/... ./features/... ./combatabilities/... ./events/...
```

Expected: grep finds nothing and all tests pass.

- [x] **Step 6: Commit**

```bash
git add -A rulebooks/dnd5e/actions \
           rulebooks/dnd5e/character \
           rulebooks/dnd5e/features/flurry_of_blows.go \
           rulebooks/dnd5e/features/flurry_of_blows_test.go \
           rulebooks/dnd5e/combatabilities/input.go \
           rulebooks/dnd5e/events/events.go
git commit -m "refactor(actions)!: delete executable character actions (#1198)"
```

---

### Task 4: Make Monsters Persist Shared Definitions Directly

**Completed:** `d3d6e30` (`refactor(monster)!: persist shared action definitions (#1198)`)

**Files:**
- Delete: `rulebooks/dnd5e/monster/actions/`
- Delete: `rulebooks/dnd5e/monster/action.go`
- Delete: `rulebooks/dnd5e/monster/behavior_test.go`
- Delete: `rulebooks/dnd5e/monster/testutil_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/actions.go`
- Modify: `rulebooks/dnd5e/refs/monster_actions.go`
- Create: `rulebooks/dnd5e/refs/monster_actions_test.go`
- Modify: `rulebooks/dnd5e/monster/data.go`
- Modify: `rulebooks/dnd5e/monster/load.go`
- Modify: `rulebooks/dnd5e/monster/monster.go`
- Modify: `rulebooks/dnd5e/monstertraits/loader.go`
- Modify: `rulebooks/dnd5e/monster/monsters/{bandit,brown_bear,ghoul,giant_rat,goblin,skeleton,skeleton_captain,thug,wolf,zombie}.go`
- Modify: `rulebooks/dnd5e/monster/monsters/bandit_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/brown_bear_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/ghoul_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/giant_rat_test.go`
- Create: `rulebooks/dnd5e/monster/monsters/goblin_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/skeleton_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/skeleton_captain_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/thug_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/wolf_gate_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/wolf_test.go`
- Modify: `rulebooks/dnd5e/monster/monsters/zombie_test.go`
- Modify: `rulebooks/dnd5e/monster/load_test.go`
- Modify: `rulebooks/dnd5e/monster/monster_test.go`
- Modify: `rulebooks/dnd5e/monster/targeting_test.go`
- Modify: `rulebooks/dnd5e/monster/attach_rollback_test.go`
- Modify: `rulebooks/dnd5e/monstertraits/loader_test.go`
- Modify: `rulebooks/dnd5e/monstertraits/load_test.go`
- Modify: `rulebooks/dnd5e/monstertraits/attach_rollback_test.go`

**Interfaces:**
- Consumes: `combat/actions.Definition` and content-specific monster action refs.
- Produces: `monster.Data.Actions []actions.Definition`, `Monster.Actions() []actions.Definition`, and direct factory-authored definitions.

- [x] **Step 1: Write failing direct-definition and identity tests**

For the wolf, assert the factory data directly contains the complete profile:

```go
func (s *WolfTestSuite) TestBiteIsACompleteSharedDefinition() {
    data := NewWolf("wolf-1").ToData()
    s.Require().Len(data.Actions, 1)

    bite := data.Actions[0]
    s.Equal(refs.MonsterActions.WolfBite(), &bite.Ref)
    s.Equal("Bite", bite.Name)
    s.Require().NotNil(bite.Attack)
    s.Equal(actions.AttackCategoryWeapon, bite.Attack.Category)
    s.Equal(&actions.MeleeDelivery{ReachFeet: 5}, bite.Attack.Delivery.Melee)
    s.Equal(4, bite.Attack.AttackBonus)
    s.Equal([]damage.Damage{{Dice: "2d4", Type: damage.Piercing, FlatBonus: 2}}, bite.Attack.Damage)
    s.Require().Len(bite.Attack.OnHit, 1)
    s.Equal(refs.Conditions.Prone(), &bite.Attack.OnHit[0].Ref)
    s.Equal(saves.NewSaveGate(abilities.STR, 11), bite.Attack.OnHit[0].Save)
}
```

Add a skeleton test proving shortsword and shortbow have different content refs and correct melee/ranged delivery. Add a ghoul test proving bite and claw have distinct refs even though both are melee profiles.

Add a load/ToData round-trip test that mutates the slice returned by `Monster.Actions()` and proves the monster's stored definitions are unchanged.

- [x] **Step 2: Run focused tests and verify RED**

Run:

```bash
cd rulebooks/dnd5e
go test ./monster/monsters ./monster ./monstertraits
```

Expected: FAIL because monster data still contains `monster.ActionData` and factories create runtime actions.

- [x] **Step 3: Replace implementation refs with content refs**

Keep the `MonsterActions` namespace but replace generic implementation refs with these exact IDs/methods:

```text
bandit-scimitar             BanditScimitar
bandit-light-crossbow       BanditLightCrossbow
brown-bear-bite             BrownBearBite
brown-bear-claw             BrownBearClaw
ghoul-bite                  GhoulBite
ghoul-claw                  GhoulClaw
giant-rat-bite              GiantRatBite
goblin-scimitar             GoblinScimitar
skeleton-captain-longsword  SkeletonCaptainLongsword
skeleton-shortsword         SkeletonShortsword
skeleton-shortbow           SkeletonShortbow
thug-mace                   ThugMace
wolf-bite                   WolfBite
zombie-slam                 ZombieSlam
```

Delete generic `Melee`, `Ranged`, `Bite`, `Multiattack`, `Shortbow`, and unused Nimble Escape action refs. Test every method for its full identity (for example, `dnd5e:monster_actions:bandit-scimitar`) and prove all fourteen refs are unique.

- [x] **Step 4: Change monster storage to shared definitions**

Change the existing `Data.Actions` field to:

```go
Actions []combatActions.Definition `json:"actions"`
```

Store `[]combatActions.Definition` on `Monster`. `Load` clones and validates every definition before constructing the runtime sheet. `Actions()` returns deep clones. `AddAction` accepts a definition, validates it, clones it, and returns an error.

Delete `MonsterActionInput`, `MonsterAction`, `TakeTurn`, `selectBestAction`, `selectTarget`, and runtime action activation/scoring methods. They have no active consumer after #1215. Remove `LoadMonsterActions` from `monstertraits.LoadMonster`; actions are already loaded data and require no behavior reconstitution.

Repurpose `monster/monsters/actions.go` as the factory validation helper:

```go
func mustAddAction(m *monster.Monster, def combatActions.Definition) {
    if err := m.AddAction(def); err != nil {
        panic(fmt.Sprintf("invalid monster action %s: %v", def.Ref, err))
    }
}
```

Make `Monster.AddCondition` mark the monster dirty when called for a live application, and add `Monster.AddLoadedCondition` (or a private equivalent) for hydration that does not dirty. Extend `SheetKeeper` to subscribe to `ConditionAppliedTopic`, apply matching conditions to its bus, append them, and mark the sheet dirty. This makes a declarative on-hit condition work for either participant kind.

- [x] **Step 5: Rewrite factories as data literals**

Each factory passes a complete `combatActions.Definition` literal to `mustAddAction` with:

- `Category: AttackCategoryWeapon`.
- Melee or ranged delivery in feet.
- Ordered existing damage pools unchanged.
- Nil cost for monster turn definitions; the turn driver structurally grants one attack and pays above Strike.
- Nil ability/weapon evidence for precomputed stat-block numbers.
- Wolf bite's prone `ConditionApplication` with STR DC 11.

Remove current multiattack runtime definitions from brown bear, ghoul, skeleton captain, and thug. Keep their component attacks. Do not create `SequenceProfile` before a sequence machine exists.

Preserve existing authored numbers during this migration. Do not silently fix the goblin's existing `Reach: 1` unit defect in #1198; file or link a separate rules issue if it is still open.

- [x] **Step 6: Delete the runtime monster-action package and prove the hard cut**

Run:

```bash
cd rulebooks/dnd5e
if rg -n '\bMonsterAction(?:Input)?\b|monster/actions|MeleeConfig|RangedConfig|BiteConfig|MultiattackConfig|LoadMonsterActions' \
  --glob '*.go' --glob '!**/*_test.go' \
  --glob '!resolution/**' --glob '!session/**' --glob '!encounter/**'; then
  echo 'runtime monster action surface remains' >&2
  exit 1
fi
go test -race ./refs ./monster ./monster/monsters ./monstertraits
golangci-lint run ./refs/... ./monster/... ./monstertraits/...
```

Expected: grep finds nothing and all tests pass.

- [x] **Step 7: Run the complete root D&D provider gate**

Run:

```bash
cd rulebooks/dnd5e
go fmt ./...
go mod tidy
git diff --check
go test -race ./...
golangci-lint run ./...
```

Expected: PASS with no `go.mod`/`go.sum` drift after tidy.

- [x] **Step 8: Commit and push the root provider**

```bash
git add -A rulebooks/dnd5e
git commit -m "refactor(monster)!: persist shared action definitions (#1198)"
git push -u origin feat/1198-actions-are-data
```

Record the pushed provider commit for upper modules:

```bash
DND5E_PROVIDER_SHA=$(git rev-parse HEAD)
printf '%s\n' "$DND5E_PROVIDER_SHA" > /tmp/1198-dnd5e-provider-sha
```

---

### Task 5: Make the Active Encounter Projection Delivery-Neutral

**Completed:** `f591578` (`refactor(encounter)!: carry action range independent of delivery (#1198)`)

**Files:**
- Modify: `rulebooks/dnd5e/encounter/field.go`
- Modify: `rulebooks/dnd5e/encounter/data.go`
- Modify: `rulebooks/dnd5e/encounter/clocks.go`
- Modify: `rulebooks/dnd5e/encounter/turndriver.go`
- Modify: `rulebooks/dnd5e/encounter/clocks_internal_test.go`
- Modify: `rulebooks/dnd5e/encounter/clocks_test.go`
- Modify: `rulebooks/dnd5e/encounter/data_test.go`
- Modify: `rulebooks/dnd5e/encounter/encounter_test.go`
- Modify: `rulebooks/dnd5e/encounter/monsterturn_test.go`
- Modify: `rulebooks/dnd5e/encounter/testturndriver_internal_test.go`
- Modify: `rulebooks/dnd5e/encounter/testturndriver_test.go`

**Interfaces:**
- Consumes: opaque action identity/name plus a maximum range in feet supplied by session.
- Produces: `encounter.ActionView.RangeFeet` and persisted `ActionViewData.RangeFeet`; the composition still does not import the D&D rulebook.

- [x] **Step 1: Rename the contract in tests first**

Update focused fixtures to build:

```go
encounter.ActionView{
    Ref:       actionRef,
    Name:      "Shortbow",
    RangeFeet: 320,
    Kind:      "ranged",
}
```

Retain tests proving `SeenMember.InReach[actionRef]` is false beyond 320 feet and true at/below 320 feet, and that pathfinding stops at the nearest cell within the longest action range.

- [x] **Step 2: Run focused tests and verify RED**

Run:

```bash
cd rulebooks/dnd5e/encounter
go test ./... -run 'ActionView|InReach|MonsterView|Path'
```

Expected: FAIL because `RangeFeet` does not exist.

- [x] **Step 3: Rename `ReachFeet` to `RangeFeet` without aliases**

Update runtime/persisted structs, validation, copies, and comments. `RangeFeet` means the maximum legal distance of the action's delivery: melee reach or ranged long/max range. Keep `Kind` opaque. Do not add both fields or a deprecated JSON alias.

- [x] **Step 4: Verify and commit the encounter provider**

Run:

```bash
cd rulebooks/dnd5e/encounter
gofmt -w .
go mod tidy
go test -race ./...
golangci-lint run ./...
git diff --check
cd ../../..
git add rulebooks/dnd5e/encounter
git commit -m "refactor(encounter)!: carry action range independent of delivery (#1198)"
git push
ENCOUNTER_PROVIDER_SHA=$(git rev-parse HEAD)
printf '%s\n' "$ENCOUNTER_PROVIDER_SHA" > /tmp/1198-encounter-provider-sha
```

Expected: PASS.

---

### Task 6: Make Resolution Interpret Shared Definitions

**Completed:** `258dbed` (`feat(resolution)!: interpret shared action profiles (#1198)`)

**Files:**
- Create: `rulebooks/dnd5e/resolution/action.go`
- Create: `rulebooks/dnd5e/resolution/action_test.go`
- Create: `rulebooks/dnd5e/resolution/delivery.go`
- Create: `rulebooks/dnd5e/resolution/delivery_test.go`
- Delete: `rulebooks/dnd5e/resolution/character_attack.go`
- Delete: `rulebooks/dnd5e/resolution/character_attack_test.go`
- Modify: `rulebooks/dnd5e/resolution/strike.go`
- Modify: `rulebooks/dnd5e/resolution/contest.go`
- Modify: `rulebooks/dnd5e/resolution/resolve.go`
- Modify: `rulebooks/dnd5e/resolution/step.go`
- Modify: `rulebooks/dnd5e/resolution/errors.go`
- Modify: `rulebooks/dnd5e/resolution/cost_test.go`
- Modify: `rulebooks/dnd5e/resolution/damage_custody_test.go`
- Modify: `rulebooks/dnd5e/resolution/declared_consequence_test.go`
- Modify: `rulebooks/dnd5e/resolution/effective_ac_test.go`
- Modify: `rulebooks/dnd5e/resolution/monster_attack_test.go`
- Modify: `rulebooks/dnd5e/resolution/resolve_test.go`
- Modify: `rulebooks/dnd5e/resolution/strictness_test.go`
- Modify: `rulebooks/dnd5e/resolution/strike_test.go`
- Modify: `rulebooks/dnd5e/resolution/world_test.go`
- Modify: `rulebooks/dnd5e/resolution/testrollers_test.go`
- Modify: `rulebooks/dnd5e/resolution/go.mod`
- Modify: `rulebooks/dnd5e/resolution/go.sum`

**Interfaces:**
- Consumes: the pushed root D&D provider from Task 4. Resolution uses the existing `CellsFromFeet` contract and does not consume `ActionView.RangeFeet`; pinning the Task 5 encounter provider belongs to session and would force an unrelated rooms-to-regions fixture migration here.
- Produces: `resolution.NewAction(*ActionInput) (Machine, error)`, Strike over `actions.Definition`, range enforcement, ordered declarative conditions, and start-before-pay execution.

- [x] **Step 1: Pin the pushed root provider and commit only published references**

Run:

```bash
DND5E_PROVIDER_SHA=$(cat /tmp/1198-dnd5e-provider-sha)
cd rulebooks/dnd5e/resolution
GOPROXY=direct go get github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e@${DND5E_PROVIDER_SHA}
# Keep the existing encounter dependency: CellsFromFeet already exists there.
```

Expected: `go.mod` resolves the root provider to the pushed commit and has no `replace` directive. Do not run `go mod tidy` until Step 9 removes resolution's old `monster/actions` import; after the provider hard cut, tidy correctly refuses that deleted package while the old source still imports it.

- [x] **Step 2: Write failing generic-dispatch and unknown-ref tests**

Add the test as a method on the existing `StrikeTestSuite`, so it reuses that suite's concrete world/cast helpers. Define the fixture in `action_test.go`:

```go
func validMeleeDefinition() combatActions.Definition {
    return combatActions.Definition{
        Ref:  core.Ref{Module: refs.Module, Type: refs.TypeMonsterActions, ID: "test-claw"},
        Name: "Test Claw",
        Attack: &combatActions.AttackProfile{
            Category:    combatActions.AttackCategoryWeapon,
            Delivery:    combatActions.AttackDelivery{Melee: &combatActions.MeleeDelivery{ReachFeet: 5}},
            AttackBonus: 4,
            Damage:      []damage.Damage{{Dice: "1d6", Type: damage.Slashing, FlatBonus: 2}},
        },
    }
}

func (s *StrikeTestSuite) TestUnknownContentRefStillDispatchesByProfile() {
    def := validMeleeDefinition()
    def.Ref.ID = "test-unknown-claw"
    def.Name = "Unknown Claw"

    machine, err := NewAction(&ActionInput{
        Definition: def,
        AttackerID: wolfID,
        TargetID:   heroID,
        Roller:     &scriptedRoller{single: 20, pair: []int{20, 3}},
    })
    s.Require().NoError(err)

    out, err := s.resolve(s.world(spatial.Position{X: 8, Y: 5}), s.hero(), machine)
    s.Require().NoError(err)
    s.True(out.Outcome.(StrikeOutcome).Hit)
}
```

Add table tests that nil input, invalid definition, and a definition with no Attack arm fail with `ErrBadAction` before Resolve. Update existing rider assertions to read the ordered `StrikeOutcome.Conditions` field defined in Step 8 instead of the removed singular `Contest` field.

- [x] **Step 3: Write failing range and delivery tests**

Cover:

- Melee target exactly at reach succeeds; one cell beyond fails.
- Ranged target inside normal range rolls normally.
- Ranged target beyond normal but at long range receives one attributable disadvantage source.
- Ranged target beyond long range fails with `ErrOutOfRange` before dice.
- `AttackChainEvent.IsMelee` and `DamageChainEvent.IsMelee` mirror delivery.
- A no-damage on-hit attack skips the damage fold and still applies its condition.

Use a counting roller and a counting ledger to assert the out-of-range case rolls zero dice and spends zero capacity.

- [x] **Step 4: Write the start-before-pay regression test**

Create a machine whose `Start` returns a named error and a payer with one attack capacity. Resolve with a real cost and assert:

```go
_, err := Resolve(ctx, input)
s.Require().ErrorIs(err, errPreflight)
s.Equal(1, payer.CapacityLeft(combat.CapacityAttack), "invalid machine start pays nothing")
```

Also assert `Start` is called exactly once on the success path.

- [x] **Step 5: Implement structural dispatch**

Add:

```go
type ActionInput struct {
    Definition combatActions.Definition
    AttackerID string
    TargetID   string
    Roller     dice.Roller
}

func NewAction(in *ActionInput) (Machine, error) {
    if in == nil {
        return nil, ErrNilInput
    }
    if err := in.Definition.Validate(); err != nil {
        return nil, fmt.Errorf("%w: %w", ErrBadAction, err)
    }
    if in.Definition.Attack != nil {
        return NewStrike(&StrikeInput{
            AttackerID: in.AttackerID,
            TargetID:   in.TargetID,
            Definition: in.Definition.Clone(),
            Roller:     in.Roller,
        }), nil
    }
    return nil, fmt.Errorf("%w: definition %q has no supported profile", ErrBadAction, in.Definition.Ref)
}
```

`StrikeInput` holds `Definition combatActions.Definition`, not a resolution-owned profile. Delete `resolution.AttackProfile`, both producer compilers, JSON config decoding, and monster action/ref imports.

- [x] **Step 6: Move runtime validation before payment**

Split the driver without changing nested Request behavior:

```go
func start(ctx context.Context, machine Machine, cast *Participants) (Step, error) {
    return machine.Start(ctx, cast)
}

func driveStep(ctx context.Context, bus events.EventBus, step Step, cast *Participants) (Outcome, error)
```

`drive` remains the helper that starts requested sub-machines and calls `driveStep`. In `resolveOn`, after attach:

```go
first, startErr := start(ctx, in.Machine, cast)
if startErr != nil {
    return nil, errors.Join(startErr, surf.teardown(ctx))
}
if payErr := payAtTheDoor(ctx, in.Cost, cast); payErr != nil {
    return nil, errors.Join(payErr, surf.teardown(ctx))
}
outcome, runErr := driveStep(ctx, surf, first, cast)
```

Document `Machine.Start` as pure preflight: it may validate/read attached sheets and produce its first step, but it may not roll, spend, publish, or mutate.

- [x] **Step 7: Implement delivery interpretation**

In Strike Start, read the installed room, locate actor/target, calculate grid distance, and compare with `Definition.Attack.Delivery` using `encounter.CellsFromFeet`.

- Melee: refuse beyond `ReachFeet`.
- Ranged: refuse beyond `MaxRangeFeet`; seed a disadvantage source when distance exceeds `NormalFeet` and a long bracket exists.
- Set attack/damage event `IsMelee` from `Delivery.IsMelee()`.
- Use `Definition.Attack.Weapon.Ref` for weapon evidence when present; otherwise use `&Definition.Ref` as source attribution.
- Carry optional ability/weapon evidence exactly as declared.

Do not keep session's reach rule as Strike's substitute; session may still use range data to answer Afford.

- [x] **Step 8: Replace executable consequences with prepared condition declarations**

Replace `ContestInput.Consequence` with the shared declaration:

```go
type ContestInput struct {
    Gate        *saves.SaveGate
    SaverID     string
    Application combatActions.ConditionApplication
    Cause       dnd5eEvents.SaveCause
    DamageTaken int
    Roller      dice.Roller
}
```

A direct Contest prebuilds `Application` during `Start`. Strike must do that work in its own `Start` so the top-level preflight happens before payment. Build each condition through:

```go
conditions.CreateFromRef(&conditions.CreateFromRefInput{
    Ref:         app.Ref.String(),
    Config:      app.Parameters,
    CharacterID: m.in.TargetID,
    SourceRef:   m.in.Definition.Ref.String(),
})
```

Store this private prepared form on the machine:

```go
type preparedCondition struct {
    declaration combatActions.ConditionApplication
    behavior    dnd5eEvents.ConditionBehavior
}
```

Replace singular `StrikeOutcome.Contest` with ordered condition outcomes:

```go
type ConditionOutcome struct {
    Ref     core.Ref        `json:"ref"`
    Contest *ContestOutcome `json:"contest,omitempty"`
    Applied bool            `json:"applied"`
}
```

Add `Conditions []ConditionOutcome` to `StrikeOutcome` and remove the old singular field rather than retaining an alias. Automatic application records `{Ref, Contest:nil, Applied:true}`; a made save records `Applied:false`; a failed save records `Applied:true`. Preserve declaration order.

After a hit and optional damage:

- Nil save: yield a Gather that publishes `ConditionAppliedEvent` immediately.
- Non-nil save: Request a private prepared Contest with the declared gate; on failure publish the prepared condition.
- Process applications in slice order.
- Set event type from `dnd5eEvents.ConditionType(app.Ref.ID)` and source to `ConditionSourceDamage` until a dedicated attack-effect source is designed.

Delete the exported `Consequence` interface and `ImposeCondition` constructor. Keep condition publication as private resolution step machinery; no executable consequence crosses the shared-data boundary.

- [x] **Step 9: Run mutation-focused and full resolution verification**

Run:

```bash
cd rulebooks/dnd5e/resolution
gofmt -w .
go mod tidy
go test -race ./...
go test -race ./... -run 'UnknownContentRef|OutOfRange|LongRange|StartBeforePay|AutomaticCondition|SaveGatedCondition'
golangci-lint run ./...
git diff --check
```

Expected: PASS. Confirm the new test fails if a content-ref switch is reintroduced by temporarily changing `NewAction` to accept only a known ref, then restore the implementation and rerun GREEN.

- [x] **Step 10: Commit and push the resolution provider**

```bash
cd ../../..
git add -A rulebooks/dnd5e/resolution
git commit -m "feat(resolution)!: interpret shared action profiles (#1198)"
git push
RESOLUTION_PROVIDER_SHA=$(git rev-parse HEAD)
printf '%s\n' "$RESOLUTION_PROVIDER_SHA" > /tmp/1198-resolution-provider-sha
```

---

### Task 7: Migrate Session to Shared Definitions

**Completed:** `cbce247` (`feat(session)!: consume shared action definitions (#1198)`)

**Files:**
- Modify: `rulebooks/dnd5e/session/attack.go`
- Create: `rulebooks/dnd5e/session/action_views_internal_test.go`
- Modify: `rulebooks/dnd5e/session/economy.go`
- Modify: `rulebooks/dnd5e/session/afford.go`
- Modify: `rulebooks/dnd5e/session/reach.go`
- Modify: `rulebooks/dnd5e/session/striker.go`
- Modify: `rulebooks/dnd5e/session/write.go`
- Modify: `rulebooks/dnd5e/session/behavior.go`
- Modify: `rulebooks/dnd5e/session/turndriver.go`
- Modify: `rulebooks/dnd5e/session/errors.go`
- Modify: `rulebooks/dnd5e/session/afford_test.go`
- Modify: `rulebooks/dnd5e/session/attackevents_test.go`
- Modify: `rulebooks/dnd5e/session/attack_internal_test.go`
- Modify: `rulebooks/dnd5e/session/attack_test.go`
- Modify: `rulebooks/dnd5e/session/economy_internal_test.go`
- Modify: `rulebooks/dnd5e/session/economy_test.go`
- Modify: `rulebooks/dnd5e/session/monster_turn_test.go`
- Modify: `rulebooks/dnd5e/session/write_test.go`
- Modify: `rulebooks/dnd5e/session/go.mod`
- Modify: `rulebooks/dnd5e/session/go.sum`

**Interfaces:**
- Consumes: root D&D, active encounter, and resolution provider commits from Tasks 4–6.
- Produces: player and monster attacks routed through `resolution.NewAction`, honest melee/ranged ActionViews, and no producer compiler in session/resolution.

- [x] **Step 1: Pin all pushed providers**

Run:

```bash
DND5E_PROVIDER_SHA=$(cat /tmp/1198-dnd5e-provider-sha)
ENCOUNTER_PROVIDER_SHA=$(cat /tmp/1198-encounter-provider-sha)
RESOLUTION_PROVIDER_SHA=$(cat /tmp/1198-resolution-provider-sha)
cd rulebooks/dnd5e/session
GOPROXY=direct go get github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e@${DND5E_PROVIDER_SHA}
GOPROXY=direct go get github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter@${ENCOUNTER_PROVIDER_SHA}
GOPROXY=direct go get github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution@${RESOLUTION_PROVIDER_SHA}
```

Expected: no `replace` directives and all versions resolve from pushed commits. Run `go mod tidy` in Step 6 after the source no longer references removed provider symbols.

- [x] **Step 2: Write failing active-path tests**

Add these named tests using the existing fake repositories and deterministic sequence roller in the suite:

```go
func (s *AttackTestSuite) TestCharacterLongbowAttacksInsideNormalRange()
func (s *AttackTestSuite) TestCharacterLongbowAttacksAtLongRangeWithDisadvantage()
func (s *AttackTestSuite) TestOutOfRangeAttackPaysAndRollsNothing()
func (s *MonsterTurnTestSuite) TestSkeletonShortbowUsesItsOwnDefinition()
func (s *WriteTestSuite) TestMonsterActionViewsCarryMaximumRange()
```

The first test places an equipped-longbow fighter 80 feet from the target, supplies d20 values 4 then 17, and asserts roll 4 was used because no disadvantage requested a pair. The second places them 200 feet apart, supplies pair 17/4, and asserts roll 4 was selected plus the folded result carries one long-range disadvantage source. Session's out-of-range test asserts `ErrOutOfReach` and zero roller calls; resolution's `TestStartFailurePaysNothing` separately proves preflight refusal leaves capacity unchanged, since the free-roam ranged fixture has no persisted fight economy to charge. The monster path selects `refs.MonsterActions.SkeletonShortbow()` from the direct definitions, and the view test asserts shortsword range 5 and shortbow range 320. Update existing melee tests to use `combatActions.Definition` and `ActionView.RangeFeet`.

- [x] **Step 3: Replace player compilation with character assembly**

Refactor the old `compileAttack` path to:

1. Load the character once.
2. Determine free-roam or turn price.
3. Call `character.AssembleAttack` with the selected slot/grip and `price.cost.Profile` (nil in free roam).
4. Pass `definition.Cost` into `resolution.Cost.Profile`; do not compile a second price.
5. Route with `resolution.NewAction`.

Move/delete session's `costOfSwing`, `asOnePayment`, `sum`, and `larger`; call `character.CostOfSwing` from `priceSwing`.

Remove the pre-resolution `refuseOutOfReach` gate from `Manager.Attack`. Strike preflight now owns the refusal before the resolution door pays. Keep a pure range comparison for Afford's read model, using `definition.Attack.Delivery.MaxRangeFeet()`.

Change `recordFor`, `attackRefFor`, and related helpers to accept `combatActions.Definition`; identity/name come from the envelope, damage type from the profile.

- [x] **Step 4: Replace monster compilation with direct definitions**

In `strikerSeam`, find `combatActions.Definition` by exact `Definition.Ref`, clone it, and call `resolution.NewAction`. There is no JSON decode or action loader.

`memberActionsFromMonster` projects every definition whose `Attack` arm is non-nil:

```go
encounter.ActionView{
    Ref:       def.Ref,
    Name:      def.Name,
    RangeFeet: def.Attack.Delivery.MaxRangeFeet(),
    Kind:      deliveryKind(def.Attack.Delivery),
}
```

`memberActionsFromCharacter` uses `character.AssembleAttack` and the same projection. Rename session/behavior mirror fields from `ReachFeet` to `RangeFeet` with no alias.

- [x] **Step 5: Map resolution range failures and verify no duplicate rule remains**

Translate `resolution.ErrOutOfRange` to existing session `ErrOutOfReach`. Delete `refuseOutOfReach`; retain only the read-model distance helper used by Afford.

Run the negative census:

```bash
cd rulebooks/dnd5e/session
if rg -n 'AttackFromCharacter|AttackFromMonsterAction|resolution\.AttackProfile|monster\.ActionData|ReachFeet|refuseOutOfReach' \
  --glob '*.go' --glob '!**/*_test.go'; then
  echo 'old compiler or reach gate remains' >&2
  exit 1
fi
```

Expected: no matches.

- [x] **Step 6: Run session verification**

Run:

```bash
cd rulebooks/dnd5e/session
gofmt -w .
go mod tidy
go test -race ./...
go test -race ./... -run 'Longbow|LongRange|OutOfRange|MonsterTurn|ActionView'
golangci-lint run ./...
git diff --check
```

Expected: PASS.

- [x] **Step 7: Commit and push the active consumer**

```bash
cd ../../..
git add -A rulebooks/dnd5e/session
git commit -m "feat(session)!: consume shared action definitions (#1198)"
git push
```

---

### Task 8: Reconcile Current Documentation and Prove the Hard Cut

**Completed:** `bd3d856` (current docs) and `9b29787` (resolution guidance)

**Files:**
- Modify: `README.md`
- Modify: `rulebooks/dnd5e/README.md`
- Modify: `rulebooks/dnd5e/monster/README.md`
- Modify: `rulebooks/dnd5e/resolution/README.md`
- Modify: `rulebooks/dnd5e/resolution/ARCHITECTURE.md`
- Modify: `docs/architecture/overview.md`
- Modify: `docs/architecture/components/rulebook-dnd5e.md`
- Modify: `docs/architecture/how-it-plays.md`
- Modify: `docs/how-to/add-a-dnd5e-monster.md`
- Modify: `docs/status.md`
- Modify: `docs/adr/0041-composable-attack-damage.md`
- Modify: `docs/ideas/session-sdk/attack-profile-seam.md`

**Interfaces:**
- Consumes: the final shipped paths from Tasks 1–7.
- Produces: current contributor guidance with historical documents clearly marked, plus full repository evidence.

- [x] **Step 1: Update current documentation to the final paths**

Document:

- `combat/actions` as the shared inert data package.
- Monster factories author `actions.Definition` directly; no `monster/actions` loader.
- Character attack assembly lives at `character.AssembleAttack`.
- Resolution dispatches profile arm through `NewAction` and Strike owns delivery/range interpretation.
- Conditions are named by `ConditionApplication` and built by their registry.
- Multiattack is unsupported until a sequence profile/machine exists; component attacks remain available.

In ADR-0041, add a note that ADR-0045 relocates profile ownership while preserving ordered damage semantics. At the top of `docs/ideas/session-sdk/attack-profile-seam.md`, mark the compiler placement as superseded by ADR-0045; preserve the document as history rather than rewriting its narrative.

- [x] **Step 2: Run the repository-wide negative census**

Run from repository root:

```bash
if rg -n \
  'AttackFromMonsterAction|AttackFromCharacter|resolution\.AttackProfile|\bMonsterAction(?:Input)?\b|LoadMonsterActions|MeleeConfig|RangedConfig|BiteConfig|MultiattackConfig|ActionGrantedTopic|ActionRemovedTopic' \
  --glob '*.go' --glob '!docs/**'; then
  echo 'ADR-0045 hard-cut symbol remains' >&2
  exit 1
fi

if find rulebooks/dnd5e/actions rulebooks/dnd5e/monster/actions -type f 2>/dev/null | grep -q .; then
  echo 'deleted executable action package still exists' >&2
  exit 1
fi

if find rulebooks/dnd5e/combat/actions -name go.mod -print | grep -q .; then
  echo 'combat/actions must not be a Go module' >&2
  exit 1
fi
```

Expected: all three checks exit zero without findings.

- [x] **Step 3: Run formatting, module, test, lint, and ADR checks**

Run:

```bash
make fmt-all
make mod-tidy
git diff --check
make test-all
make lint-all
./scripts/check-decisions.sh
```

Expected: all commands pass. Inspect `git diff` after `make mod-tidy`; no local `replace` or accidental module-wide dependency update is allowed.

Observed: `make fmt-all`, `make mod-tidy`, `make test-all`, and the ADR check passed. Each changed module (`rulebooks/dnd5e`, active encounter, resolution, session) passed its full race tests and lint. `make lint-all` reaches an untouched pre-existing `rpgerr` failure: two `goconst` findings for the string `poison`; the same failure reproduces on `origin/main`, so #1198 does not modify that independently versioned module. Repository-wide `goimports` spill from `make fmt-all` was reverted rather than committed as unrelated formatting.

- [x] **Step 4: Commit documentation and verification cleanup**

```bash
git add README.md rulebooks/dnd5e/README.md \
        rulebooks/dnd5e/monster/README.md \
        rulebooks/dnd5e/resolution/README.md \
        rulebooks/dnd5e/resolution/ARCHITECTURE.md \
        docs/architecture docs/how-to/add-a-dnd5e-monster.md \
        docs/status.md docs/adr docs/ideas/session-sdk/attack-profile-seam.md
git commit -m "docs(actions): make shared definitions the only current path (#1198)"
git push
```

---

### Task 9: Replace the Cross-Module Draft with Module-Isolated Squash PRs

**Status:** draft PR [#1232](https://github.com/KirkDiggler/rpg-toolkit/pull/1232) proved the integrated code and exposed an invalid delivery strategy. It is explicitly marked **DO NOT MERGE**. Its commits are reconciliation evidence only; no provider SHA from that branch may survive as a committed final dependency.

**Module ownership:**
- Root D&D PR: files whose nearest `go.mod` is `rulebooks/dnd5e/go.mod`, excluding every nested module.
- Encounter PR: files whose nearest `go.mod` is `rulebooks/dnd5e/encounter/go.mod`.
- Resolution PR: files whose nearest `go.mod` is `rulebooks/dnd5e/resolution/go.mod`.
- Session PR: files whose nearest `go.mod` is `rulebooks/dnd5e/session/go.mod`.
- PR #1214: repository-wide files under `docs/`; it receives final current-state documentation and the retrospective after code lands.

**Interfaces:**
- Consumes: the reviewed content and verification evidence from draft #1232.
- Produces: four module-isolated, squash-mergeable implementation PRs whose consumer `go.mod` files reference stable provider tags at merge time.

- [x] **Step 1: Quarantine the invalid integration PR**

Convert #1232 to draft and add a visible `DO NOT MERGE` comment explaining the module-isolation and squash conflict. Correct any issue comment that calls it merge-ready. Do not close it yet; retain it until the replacement-file census passes.

- [ ] **Step 2: Create fresh module-isolated branches and reconcile every file**

Fetch `origin/main`. Create four fresh worktrees and named branches from that exact commit; do not branch from #1232 or cherry-pick its cross-module commits. Apply the final reviewed file content by module ownership:

```text
feat/1198-actions-dnd5e
feat/1198-actions-encounter
feat/1198-actions-resolution
feat/1198-actions-session
```

Generate the machine-readable census before creating branches:

```bash
REPO=/home/kirk/game-dev/rpg-toolkit
SOURCE_REF=origin/feat/1198-actions-are-data
MANIFEST=/tmp/1198-split-manifest.tsv
cd "$REPO"
git fetch origin --prune --tags
: > "$MANIFEST"
while IFS=$'\t' read -r status path extra; do
  test -z "$extra" || { echo "unexpected rename/copy row: $status $path $extra" >&2; exit 1; }
  case "$path" in
    docs/*) owner=idea-docs ;;
    rulebooks/dnd5e/encounter/*) owner=encounter ;;
    rulebooks/dnd5e/resolution/*) owner=resolution ;;
    rulebooks/dnd5e/session/*) owner=session ;;
    rulebooks/dnd5e/*) owner=dnd5e ;;
    *) echo "unassigned draft path: $path" >&2; exit 1 ;;
  esac
  printf '%s\t%s\t%s\n' "$owner" "$status" "$path" >> "$MANIFEST"
done < <(git diff --name-status origin/main..."$SOURCE_REF")
awk -F'\t' '{count[$1]++} END {for (owner in count) print owner, count[owner]}' "$MANIFEST" | sort
```

The reviewed draft currently assigns exactly 103 root D&D files, 7 encounter files, 24 resolution files, 17 session files, and 7 repository-doc files. Stop if those counts change without a new explicit reconciliation.

Create binary patches from the same source/base comparison:

```bash
git diff --binary origin/main..."$SOURCE_REF" -- rulebooks/dnd5e \
  ':(exclude)rulebooks/dnd5e/behavior' \
  ':(exclude)rulebooks/dnd5e/encounter' \
  ':(exclude)rulebooks/dnd5e/resolution' \
  ':(exclude)rulebooks/dnd5e/session' > /tmp/1198-dnd5e.patch
git diff --binary origin/main..."$SOURCE_REF" -- rulebooks/dnd5e/encounter > /tmp/1198-encounter.patch
git diff --binary origin/main..."$SOURCE_REF" -- rulebooks/dnd5e/resolution > /tmp/1198-resolution.patch
git diff --binary origin/main..."$SOURCE_REF" -- rulebooks/dnd5e/session > /tmp/1198-session.patch
git diff --binary origin/main..."$SOURCE_REF" -- docs > /tmp/1198-idea-docs.patch
```

Use the git-worktree safety procedure to create:

```text
/home/kirk/game-dev/rpg-toolkit/.worktrees/1198-actions-dnd5e
/home/kirk/game-dev/rpg-toolkit/.worktrees/1198-actions-encounter
/home/kirk/game-dev/rpg-toolkit/.worktrees/1198-actions-resolution
/home/kirk/game-dev/rpg-toolkit/.worktrees/1198-actions-session
```

Apply only the matching patch in each worktree with `git apply --index`. Keep `/tmp/1198-idea-docs.patch` unapplied until Task 10 on PR #1214.

The root D&D and encounter branches are independent. Commit and push both first. The resolution branch then temporarily resolves the new root D&D branch commit with `GOPROXY=direct go get github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e@"${DND5E_PR_SHA}"`. After resolution is committed and pushed, session temporarily resolves the new root D&D, encounter, and resolution branch commits the same way. Temporary pins are review/CI scaffolding only and must not reach `main`.

- [ ] **Step 3: Verify each isolated branch and open all four PRs**

Apply, verify, commit, and push root D&D first:

```bash
DND5E_WT=/home/kirk/game-dev/rpg-toolkit/.worktrees/1198-actions-dnd5e
git -C "$DND5E_WT" apply --index /tmp/1198-dnd5e.patch
cd "$DND5E_WT/rulebooks/dnd5e"
go mod tidy
go test -race ./...
golangci-lint run ./...
cd "$DND5E_WT"
git diff --check
test -z "$(git grep -n '^replace ' -- rulebooks/dnd5e/go.mod || true)"
git commit -m "feat(actions)!: make D&D actions inert shared data (#1198)"
git push -u origin feat/1198-actions-dnd5e
git rev-parse HEAD > /tmp/1198-dnd5e-pr-sha
```

Apply, verify, commit, and push encounter independently:

```bash
ENCOUNTER_WT=/home/kirk/game-dev/rpg-toolkit/.worktrees/1198-actions-encounter
git -C "$ENCOUNTER_WT" apply --index /tmp/1198-encounter.patch
cd "$ENCOUNTER_WT/rulebooks/dnd5e/encounter"
go mod tidy
go test -race ./...
golangci-lint run ./...
cd "$ENCOUNTER_WT"
git diff --check
test -z "$(git grep -n '^replace ' -- rulebooks/dnd5e/encounter/go.mod || true)"
git commit -m "refactor(encounter)!: project action range independent of delivery (#1198)"
git push -u origin feat/1198-actions-encounter
git rev-parse HEAD > /tmp/1198-encounter-pr-sha
```

Apply resolution, replace the quarantined draft pin with the new root PR commit, then verify and push:

```bash
RESOLUTION_WT=/home/kirk/game-dev/rpg-toolkit/.worktrees/1198-actions-resolution
DND5E_PR_SHA=$(cat /tmp/1198-dnd5e-pr-sha)
git -C "$RESOLUTION_WT" apply --index /tmp/1198-resolution.patch
cd "$RESOLUTION_WT/rulebooks/dnd5e/resolution"
GOPROXY=direct go get github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e@"$DND5E_PR_SHA"
go mod tidy
go test -race ./...
golangci-lint run ./...
cd "$RESOLUTION_WT"
git add rulebooks/dnd5e/resolution/go.mod rulebooks/dnd5e/resolution/go.sum
git diff --check
test -z "$(git grep -n '^replace ' -- rulebooks/dnd5e/resolution/go.mod || true)"
git commit -m "feat(resolution)!: interpret inert action definitions (#1198)"
git push -u origin feat/1198-actions-resolution
git rev-parse HEAD > /tmp/1198-resolution-pr-sha
```

Apply session, replace every quarantined draft pin with the corresponding replacement PR commit, then verify and push:

```bash
SESSION_WT=/home/kirk/game-dev/rpg-toolkit/.worktrees/1198-actions-session
DND5E_PR_SHA=$(cat /tmp/1198-dnd5e-pr-sha)
ENCOUNTER_PR_SHA=$(cat /tmp/1198-encounter-pr-sha)
RESOLUTION_PR_SHA=$(cat /tmp/1198-resolution-pr-sha)
git -C "$SESSION_WT" apply --index /tmp/1198-session.patch
cd "$SESSION_WT/rulebooks/dnd5e/session"
GOPROXY=direct go get github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e@"$DND5E_PR_SHA"
GOPROXY=direct go get github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter@"$ENCOUNTER_PR_SHA"
GOPROXY=direct go get github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution@"$RESOLUTION_PR_SHA"
go mod tidy
go test -race ./...
golangci-lint run ./...
cd "$SESSION_WT"
git add rulebooks/dnd5e/session/go.mod rulebooks/dnd5e/session/go.sum
git diff --check
test -z "$(git grep -n '^replace ' -- rulebooks/dnd5e/session/go.mod || true)"
git commit -m "feat(session)!: consume inert action definitions (#1198)"
git push -u origin feat/1198-actions-session
```

Before opening PRs, compare each branch's changed-file list with its owner rows in `/tmp/1198-split-manifest.tsv`; every `diff` must be empty:

```bash
check_owner() {
  owner=$1
  worktree=$2
  awk -F'\t' -v owner="$owner" '$1 == owner {print $3}' /tmp/1198-split-manifest.tsv | sort > "/tmp/1198-${owner}-expected.txt"
  git -C "$worktree" diff --name-only origin/main...HEAD | sort > "/tmp/1198-${owner}-actual.txt"
  diff -u "/tmp/1198-${owner}-expected.txt" "/tmp/1198-${owner}-actual.txt"
}
check_owner dnd5e /home/kirk/game-dev/rpg-toolkit/.worktrees/1198-actions-dnd5e
check_owner encounter /home/kirk/game-dev/rpg-toolkit/.worktrees/1198-actions-encounter
check_owner resolution /home/kirk/game-dev/rpg-toolkit/.worktrees/1198-actions-resolution
check_owner session /home/kirk/game-dev/rpg-toolkit/.worktrees/1198-actions-session
```

Open all four PRs to `main` so review can proceed concurrently. Each body must link #1198, ADR-0045, PR #1214, and replacement draft #1232; identify its sole module; disclose any temporary pseudo-version; and state that it uses the normal squash workflow. Only the final session PR closes #1198.

Use breaking conventional titles so the v0 auto-tag workflow performs a minor bump after squash:

```text
feat(actions)!: make D&D actions inert shared data (#1198)
refactor(encounter)!: project action range independent of delivery (#1198)
feat(resolution)!: interpret inert action definitions (#1198)
feat(session)!: consume inert action definitions (#1198)
```

Do not predict final version numbers; intervening main changes may alter them.

- [ ] **Step 4: Review all replacement PRs before the merge wave**

Request Copilot and human review on every replacement. Resolve findings inline and rerun the owning module's full race tests and lint after every code change. Keep all four PRs open until their independent review surfaces are clear.

- [ ] **Step 5: Squash the independent root D&D and encounter providers and capture tags**

Immediately before each merge, verify applicable GitHub checks. Squash-merge root D&D and encounter using their breaking PR titles. After each merge, wait for `Auto Tag Modules (Safe)` on `main`, fetch tags, inspect the workflow summary, and record the exact stable tag and squash commit.

Expected: nested-module exclusions cause the root D&D merge to tag only root D&D and the encounter merge to tag only encounter. If tagging fails or tags an unexpected module, stop before updating consumers and repair the release state.

- [ ] **Step 6: Replace resolution's temporary pin, then squash and tag resolution**

Update the resolution branch from current `origin/main`. Set `DND5E_TAG` to the exact root tag recorded in Step 5, derive `DND5E_VERSION=${DND5E_TAG#rulebooks/dnd5e/}`, then replace the pseudo-version:

```bash
cd rulebooks/dnd5e/resolution
: "${DND5E_TAG:?set from the Step 5 workflow output}"
DND5E_VERSION=${DND5E_TAG#rulebooks/dnd5e/}
GOPROXY=direct go get github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e@"${DND5E_VERSION}"
go mod tidy
go test -race ./...
golangci-lint run ./...
```

Confirm no replacement-provider pseudo-version remains in `go.mod`, push, and wait for CI/review. Squash-merge resolution, wait for its auto-generated stable tag, and record the exact tag and squash commit.

- [ ] **Step 7: Replace session's temporary pins, then squash and tag session**

Update the session branch from current `origin/main`. Replace all three temporary provider pins with the generated root D&D, encounter, and resolution stable tags. Run tidy, the full session race suite, lint, and focused active-path tests. Confirm `go.mod` contains no #1198 provider pseudo-version or `replace` directive, then push and wait for CI/review.

Squash-merge session only after every provider tag resolves through `GOPROXY=direct`. Wait for the session tag and record its exact value and squash commit.

- [ ] **Step 8: Reconcile the landed result and close #1232**

On current `origin/main`, run the hard-cut census, `make test-all`, changed-module race/lint gates, and `git diff` comparisons against #1232's intended source result. Differences must be explained by stable-tag pins, current-main integration, review fixes, or repository-wide docs intentionally held for #1214.

When every draft #1232 file is accounted for and no behavior was lost, close #1232 with links to the four replacement PRs and their released tags. Do not merge or retarget it.

---

### Task 10: Record What Shipped and Close the Idea

**Files (on the still-open PR #1214 idea branch):**
- Modify: `docs/ideas/actions-are-data/design.md`
- Modify: `docs/ideas/actions-are-data/plan.md`
- Create: `docs/ideas/actions-are-data/implementation.md`

**Interfaces:**
- Consumes: the four merged #1198 module PRs, their squash commits and stable module tags, verification output, and implementation discoveries.
- Produces: a durable record separating approved intent, executable plan, and observed implementation reality.

- [ ] **Step 1: Bring the open idea branch forward after implementation merges**

In the idea worktree, fetch and merge current `origin/main` into `docs/1198-actions-are-data`. Do not rewrite the implementation merge graph:

```bash
git fetch origin
git merge --no-edit origin/main
```

Expected: the idea branch now contains every shipped module squash commit and retains the docs commits from PR #1214.

- [ ] **Step 2: Write the post-implementation record from evidence**

Create `implementation.md` only now. Record:

- all module implementation PR URLs and squash commits;
- released provider and consumer module tags;
- final package/type/function paths;
- exact verification commands and outcomes;
- deviations from the original plan and why they were necessary;
- subtleties discovered while implementing, including module pinning, lifecycle, validation order, and condition behavior;
- intentionally deferred profile types and follow-up issues;
- confirmation that no compatibility representation survived.

Do not restate predictions as findings. Every claim must cite merged code, command output, a commit, or a follow-up issue.

- [ ] **Step 3: Close the live design and plan state**

Change the design status from `Approved — implementation open` to `Implemented`. Check every completed plan box. If a step was superseded, mark it explicitly with the shipped alternative and link the implementation record; do not falsely mark an unperformed command as run.

- [ ] **Step 4: Verify and merge the idea PR**

Run:

```bash
git diff --check
./scripts/check-decisions.sh
git status --short --branch
git add docs/ideas/actions-are-data docs/adr/0045-actions-are-data.md docs/adr/DECISIONS.md
git commit -m "docs(actions): record the actions-as-data implementation (#1198)"
git push
gh pr checks 1214 --repo KirkDiggler/rpg-toolkit --watch
```

Request final review of the design-to-implementation record. Squash-merge PR #1214 only after its threads and checks are clear.
