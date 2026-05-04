---
name: rpg-api toolkit usage audit
description: Grep-driven inventory of which rpg-toolkit modules and symbols rpg-api actually imports, and what that implies for components/ docs
status: audit complete — feeds the components/ rewrite (PR 2 of 2)
date: 2026-05-04
audience: PR 2 author, anyone re-grounding components/ docs
---

# rpg-api toolkit usage audit

This is **PR 1 of 2** for re-grounding `docs/architecture/components/`. PR 1 audits.
PR 2 rewrites the component docs informed by these findings. The autonomous-waves
context is at https://github.com/KirkDiggler/rpg-project/blob/main/ideas/autonomous-waves/design.md.

The repo paths used throughout:

- rpg-toolkit (this repo): https://github.com/KirkDiggler/rpg-toolkit
- rpg-api: https://github.com/KirkDiggler/rpg-api — read-only for this audit, only `internal/` and `cmd/` greps

The greps run against rpg-api at the working state of `main` on 2026-05-04. Symbol
counts come from `grep -oE '<pkg>\.[A-Z][a-zA-Z0-9_]*'` over Go source under
`/home/kirk/personal/rpg-api/internal/` and `/home/kirk/personal/rpg-api/cmd/`. File
counts use `grep -rln '<import path>'` with the same scope. File:line citations
are by symbol — not by line number for line-likely-to-move callsites — per
toolkit-member lesson 002.

Six empty buckets verified by hand: `mechanics/*`, `items`, `tools/spawn`,
`tools/selectables`, `behavior`, `core/chain` — **rpg-api does not import any of
these directly**. They are reached, if at all, transitively through
`rulebooks/dnd5e`. This shapes the recommendations for PR 2.

---

## Section 1: rpg-api's actual toolkit imports

Grouped by toolkit module. Each row gives file count (number of rpg-api files
that import the path) and the top symbols rpg-api references from that module.
"Top symbols" is the symbol-occurrence histogram across the rpg-api source tree;
take it as a usage-density signal, not a usage-count of distinct callsites.

### Module: `core` — 8 files

| Symbol | Refs | One citation |
|---|---|---|
| `core.Ref` | 46 | `internal/entities/encounter_events.go:219` |
| `core.Entity` | 5 | `internal/orchestrators/encounter/perception.go:128` |
| `core.EntityType` | 2 | `internal/orchestrators/encounter/perception.go:131` |

`core.Action` does **not** appear directly in rpg-api. The Action interface is a
toolkit-internal contract; rpg-api consumes its implementations (Rage, Strike,
etc.) by ref, never by Action interface assertion.

### Module: `core/combat` — 1 file

| Symbol | Refs | One citation |
|---|---|---|
| `combat.ActionType` | 2 | `internal/handlers/dnd5e/v1alpha1/character/converters.go` (`convertActionTypeToProto`) |
| `combat.ActionStandard` | 1 | same file, switch case |
| `combat.ActionBonus`, `ActionReaction`, `ActionFree` | 1 each | same file, sibling switch cases |

Used **only** for proto enum mapping. This is exactly the boundary pattern
rpg-api should be using.

### Module: `core/resources` — 1 file

| Symbol | Refs | One citation |
|---|---|---|
| `coreResources.ResourceKey` | 1 | `internal/integration/encounter/helpers.go:381` |
| `coreResources.ResetShortRest` | 1 | `internal/integration/encounter/helpers.go:386` |

Aliased as `coreResources` to disambiguate from `rulebooks/dnd5e/resources`.
Integration-test setup only. Production code reaches resources via the dnd5e
package.

### Module: `events` — 4 files

| Symbol | Refs | One citation |
|---|---|---|
| `events.NewEventBus` | 10 | `internal/orchestrators/encounter/monster_turns.go:131` |
| `events.EventBus` | 1 | `internal/orchestrators/encounter/orchestrator.go:2405` (returned from helper) |

rpg-api creates a fresh bus per attack/round of resolution (not one global bus)
and passes it into toolkit calls. It does not subscribe handlers to the bus
itself — that's a toolkit responsibility.

### Module: `dice` — 3 production + 3 mock files

| Symbol | Refs | One citation |
|---|---|---|
| `dice.Service`, `dice.Roller` | 4, 3 | `internal/services/dice/` (full service) |
| `dice.NewRoller` | 3 | service constructor |
| `dice.RollDiceInput`/`RollDiceOutput`, `dice.GetRollSessionInput`/`Output`, `dice.RollAbilityScoresInput`/`Output`, `dice.ClearRollSessionInput`/`Output` | 2-5 each | service interface |
| `dice.ContextAbilityScores` | 3 | session context type |
| `dice.MockRoller`, `dice.NewMockRoller` | 1, 1 | tests (`dice/mock`) |

The dice package is consumed both as a library (`Roller`) and as a service shape
(`Service` with Input/Output types). Service-style consumption is unusual for
rpg-api; most toolkit packages are libraries.

### Module: `tools/spatial` — 18 files

| Symbol | Refs | One citation |
|---|---|---|
| `spatial.CubeCoordinate` | 128 | `internal/entities/merged_grid.go:13` |
| `spatial.RoomData` | 82 | `internal/entities/encounter_events.go:124` |
| `spatial.GridTypeHex` | 52 | `internal/handlers/dnd5e/v1alpha1/encounter/handler.go:157` |
| `spatial.EntityCubePlacement` | 45 | `internal/entities/merged_grid.go:24` |
| `spatial.Position` | 28 | `internal/entities/merged_grid.go` |
| `spatial.HexOrientationPointyTop` | 19 | encounter handler |
| `spatial.EntityPlacement` | 11 | merged_grid |
| `spatial.HexOrientation`, `spatial.GridTypeSquare`, `spatial.HexOrientationFlatTop`, `spatial.GridTypeGridless`, `spatial.Room`, `spatial.OffsetCoordinateToCubeWithOrientation` | 1–8 | various |

Spatial is the second-heaviest toolkit dependency. rpg-api stores
`spatial.RoomData` directly (per `architecture: Toolkit types are canonical`)
and reasons in cube coordinates throughout.

### Module: `tools/environments` — 6 files

| Symbol | Refs | One citation |
|---|---|---|
| `environments.ConnectionEdge` | 10 | `internal/entities/dungeon.go:30` |
| `environments.RoomShape` | 5 | dungeon entity |
| `environments.GetDefaultShapes` | 1 | dungeon construction |
| `environments.ConnectionPoint` | 1 | dungeon entity |

Narrow surface — rpg-api uses environments for dungeon-graph data only. The full
environment-generation machinery (`graph_generator.go`, `wall_patterns.go`,
`environment_persistence.go`) is **not** referenced by rpg-api.

### Module: `rulebooks/dnd5e/character` — 24 files

The single most-imported toolkit package by rpg-api. Top symbols:

| Symbol | Refs | One citation |
|---|---|---|
| `character.Data` | 47 | `internal/entities/encounter_events.go:78` |
| `character.EquipmentSlots`, `character.SlotMainHand`, `character.SlotOffHand` | 15, 13, 6 | converters/orchestrator |
| `character.SetRaceInput`, `character.SetClassInput`, `character.SetBackgroundInput`, `character.SetAbilityScoresInput`, `character.SetNameInput` | 4–11 each | character orchestrator (per-step setters) |
| `character.DraftData`, `character.DraftConfig`, `character.NewDraft`, `character.LoadDraftFromData` | 8 each | draft lifecycle (`internal/orchestrators/character/orchestrator.go`) |
| `character.Progress`, `character.ProgressClass`, `character.ProgressRace`, `character.ProgressName` | 3–8 | draft progress tracking |
| `character.Character`, `character.LoadFromData` | 5, 7 | runtime character (`internal/orchestrators/character/orchestrator.go:740`) |
| `character.ActionEconomyData`, `character.GrantedActionKey` | 8, 7 | converter mapping |
| `character.ClassChoices`, `character.RaceChoices` | 6, 4 | choice-data conversion |
| `character.GetCharacterInput`, `character.DeleteCharacterInput` | 6, 5 | service-shape inputs |

This module dominates because rpg-api orchestrates character creation/state and
the toolkit owns the character domain model.

### Module: `rulebooks/dnd5e/character/choices` — 6 files

| Symbol | Refs | One citation |
|---|---|---|
| `choices.ValidationResult`, `choices.ValidationError` | 11, 2 | converters / orchestrator |
| `choices.Requirements`, `choices.ChoiceData`, `choices.ChoiceID` | 3, 3, 1 | converters |
| `choices.GetClassRequirements`, `choices.GetClassRequirementsAtLevel`, `choices.GetClassRequirementsWithSubclass`, `choices.GetRaceRequirements` | 1 each | per-step requirements |
| `choices.FighterPack`, `choices.FighterArmor`, `choices.FighterWeaponsPrimary`, `choices.FighterWeaponsSecondary`, `choices.WizardPack`, `choices.WizardWeaponsPrimary`, `choices.WizardFocus` | 1–2 each | class-specific equipment choices |
| `choices.SkillRequirement`, `choices.ToolRequirement`, `choices.FightingStyleRequirement`, `choices.EquipmentRequirement` | 1 each | requirement types |

Choices is a discrete sub-API. rpg-api treats it as a service: ask for
requirements, post a validation request, get a `ValidationResult`.

### Module: `rulebooks/dnd5e/combat` — 5 files

| Symbol | Refs | One citation |
|---|---|---|
| `combat.AttackHandMain`, `combat.AttackHand`, `combat.AttackHandOff` | 8, 4, 3 | encounter orchestrator |
| `combat.WithCombatantLookup` | 6 | `internal/orchestrators/encounter/orchestrator.go:383` |
| `combat.ResolveAttack`, `combat.AttackInput` | 5, 5 | `internal/orchestrators/encounter/orchestrator.go:388` (and `monster_turns.go:353`) |
| `combat.AttackResult`, `combat.DamageBreakdown`, `combat.NewActionEconomy`, `combat.CapacityFlurryStrike` | 1 each | result handling |

This is the **chain entry point**. rpg-api calls `combat.ResolveAttack` which
internally drives the attack chain (see Section 3, claim 4).

### Module: `rulebooks/dnd5e/refs` — 12 files

| Symbol | Refs | One citation |
|---|---|---|
| `refs.Weapons` | 96 | `internal/handlers/dnd5e/v1alpha1/character/converters_test.go` and across handlers |
| `refs.Features` | 85 | converters (e.g. `refs.Features.Rage()` in tests at line 335) |
| `refs.Conditions` | 51 | converters |
| `refs.Actions` | 45 | action mapping |
| `refs.Monsters` | 43 | encounter handler |
| `refs.Tools` | 35 | choice mapping |
| `refs.CombatAbilities` | 31 | converter |
| `refs.Abilities` | 15 | ability mapping |
| `refs.Module`, `refs.Armor` | 14, 13 | misc |

Heaviest non-character dnd5e import. The boundary rule ("client sends
references, never calculations") makes refs the literal API surface.

### Module: `rulebooks/dnd5e/monster` — 14 files

| Symbol | Refs | One citation |
|---|---|---|
| `monster.Data` | 62 | `internal/repositories/encounters/repository.go:87` (still on a deprecation path per repo comment) |
| `monster.PerceivedEntity`, `monster.PerceptionData` | 8, 4 | perception |
| `monster.LoadFromData` | 6 | `internal/orchestrators/encounter/monster_turns.go:134` |
| `monster.NewGoblin`, `monster.ScimitarConfig` | 4, 2 | encounter setup helpers |
| `monster.ActionData`, `monster.TypeMeleeAttack`, `monster.TypeRangedAttack`, `monster.TakeDamage` | 2–4 | action data |

### Module: `rulebooks/dnd5e/initiative` — 7 files

| Symbol | Refs | One citation |
|---|---|---|
| `initiative.TrackerData` | 54 | `internal/repositories/encounters/repository.go:83` |
| `initiative.EntityData` | 50 | repository |
| `initiative.NewParticipant` | 34 | encounter orchestrator |
| `initiative.Roll` | 24 | encounter orchestrator |
| `initiative.New`, `initiative.RollForOrder` | 2 each | initialization |

rpg-api persists `TrackerData` directly. The tracker is reconstituted from data
in repository load paths.

### Module: `rulebooks/dnd5e/classes` — 11 files

| Symbol | Refs | One citation |
|---|---|---|
| `classes.Fighter` | 22 | converter |
| `classes.Class` | 8 | converter |
| `classes.Wizard`, `classes.Monk`, `classes.Rogue`, `classes.Ranger`, `classes.Barbarian` | 5–7 | converter |
| `classes.Warlock`, `classes.Sorcerer`, `classes.Paladin` | 4 each | converter |

Each class is a typed constant per the typed-constants pattern.

### Module: `rulebooks/dnd5e/races` — 8 files

| Symbol | Refs | One citation |
|---|---|---|
| `races.Human`, `races.Elf`, `races.Dwarf`, `races.Race` | 6–13 | converter |
| `races.Tiefling`, `races.Halfling`, `races.HalfOrc`, `races.HalfElf`, `races.Gnome`, `races.Dragonborn` | 4 each | converter |

### Module: `rulebooks/dnd5e/abilities` — 10 files

| Symbol | Refs | One citation |
|---|---|---|
| `abilities.DEX`, `abilities.STR`, `abilities.CON`, `abilities.WIS`, `abilities.INT`, `abilities.CHA` | 28–41 each | `internal/handlers/dnd5e/v1alpha1/character/converters.go:89` (and many switch sites) |
| `abilities.Ability` | 7 | converter |

### Module: `rulebooks/dnd5e/shared` — 9 files

| Symbol | Refs | One citation |
|---|---|---|
| `shared.AbilityScores` | 31 | `internal/orchestrators/character/orchestrator_test.go:175` |
| `shared.SelectionID` | 9 | converter |
| `shared.ChoiceSkills`, `shared.ChoiceFightingStyle`, `shared.ChoiceEquipment`, `shared.ChoiceLanguages` | 3–5 | choice mapping |
| `shared.SourceClass`, `shared.Proficient`, `shared.EquipmentID`, `shared.ProficiencyLevel` | 2–4 | misc |

`shared` is the cross-cutting types package — consumed broadly.

### Module: `rulebooks/dnd5e/gamectx` — 3 files

| Symbol | Refs | One citation |
|---|---|---|
| `gamectx.WithGameContext` | 12 | encounter orchestrator |
| `gamectx.Characters` | 7 | encounter orchestrator |
| `gamectx.NewCombatantRegistry` | 5 | encounter orchestrator |
| `gamectx.EquippedWeapon`, `gamectx.SlotOffHand` | 4, 2 | combat helpers |
| `gamectx.NewBasicCharacterRegistry`, `gamectx.NewGameContext`, `gamectx.GameContextConfig` | 2 each | setup |

This is the integration shim that lets toolkit combat resolution see all the
combatants in the encounter. rpg-api populates the registry, hands it to the
toolkit via context, and the toolkit looks up combatants by ID during the chain.

### Module: `rulebooks/dnd5e/skills` — 2 files

| Symbol | Refs | One citation |
|---|---|---|
| `skills.Skill` | 10 | converter |
| `skills.Athletics`, `skills.Intimidation`, `skills.Acrobatics`, `skills.Stealth`, `skills.SleightOfHand`, `skills.Religion`, `skills.Investigation` | 3–6 each | converter |

### Module: `rulebooks/dnd5e/weapons` — 5 files

| Symbol | Refs | One citation |
|---|---|---|
| `weapons.Longsword`, `weapons.GetByID` | 10 each | converter / equipment |
| `weapons.Weapon`, `weapons.PropertyTwoHanded` | 7, 3 | weapon data |
| `weapons.UnarmedStrike`, `weapons.Scimitar`, `weapons.Mace`, `weapons.GetSimpleWeapons` | 2 each | misc |

### Module: `rulebooks/dnd5e/backgrounds` — 4 files

| Symbol | Refs | One citation |
|---|---|---|
| `backgrounds.Soldier`, `backgrounds.Acolyte`, `backgrounds.Sage`, `backgrounds.Criminal`, `backgrounds.Background`, `backgrounds.Data` | 3–8 each | converter |

### Module: `rulebooks/dnd5e/damage` — 3 files

| Symbol | Refs | One citation |
|---|---|---|
| `damage.Slashing` | 10 | converter |
| `damage.Bludgeoning`, `damage.Type`, `damage.Piercing`, `damage.Poison` | 1–6 | converter / monsters |

### Module: `rulebooks/dnd5e/spells` — 3 files

| Symbol | Refs | One citation |
|---|---|---|
| `spells.Spell` | 3 | converter |
| `spells.Sleep`, `spells.Shield`, `spells.MagicMissile`, `spells.MageHand`, `spells.Light`, `spells.Identify`, `spells.FireBolt` | 2 each | converter |

Note the breadth — rpg-api references spell typed constants but does **not**
call any spell-resolution machinery directly. Spells go through refs, and the
toolkit handles resolution.

### Module: `rulebooks/dnd5e/armor` — 3 files

| Symbol | Refs | One citation |
|---|---|---|
| `armor.Shield`, `armor.ArmorID` | 4, 1 | converter |

### Module: `rulebooks/dnd5e/monstertraits` — 2 files

| Symbol | Refs | One citation |
|---|---|---|
| `monstertraits.LoadMonsterConditions` | 5 | encounter orchestrator |

### Module: `rulebooks/dnd5e/fightingstyles` — 2 files

| Symbol | Refs | One citation |
|---|---|---|
| `fightingstyles.Defense`, `fightingstyles.TwoWeaponFighting`, `fightingstyles.Protection`, `fightingstyles.GreatWeaponFighting`, `fightingstyles.Dueling`, `fightingstyles.Archery`, `fightingstyles.FightingStyle` | 2–8 | converter |

### Module: `rulebooks/dnd5e/languages` — 2 files

| Symbol | Refs | One citation |
|---|---|---|
| `languages.Language`, `languages.Elvish`, `languages.Common`, `languages.Goblin`, `languages.Orc`, `languages.Sylvan`, `languages.Primordial`, `languages.Undercommon` | 2–6 | converter |

### Module: `rulebooks/dnd5e/ammunition` — 1 file

| Symbol | Refs | One citation |
|---|---|---|
| `ammunition.Bolts20`, `ammunition.Arrows20` | 2 each | converter |

### Module: `rulebooks/dnd5e/packs` — 1 file

| Symbol | Refs | One citation |
|---|---|---|
| `packs.PackID` | 1 | converter |

### Module: `rulebooks/dnd5e/tools` — 1 file

| Symbol | Refs | One citation |
|---|---|---|
| `tools.ToolID`, `tools.WoodcarverTools`, `tools.WeaverTools`, `tools.Viol`, `tools.VehiclesWater`, `tools.VehiclesLand`, `tools.TinkerTools`, `tools.ThreeDragonAnte` | 1 each | converter |

### Module: `rulebooks/dnd5e/proficiencies` — 1 file

| Symbol | Refs | One citation |
|---|---|---|
| `proficiencies.Tool`, `proficiencies.Weapon*` (many) | 1 each | converter |

### Module: `rulebooks/dnd5e/saves` — 1 file

| Symbol | Refs | One citation |
|---|---|---|
| `saves.DeathSaveState` | 1 | converter |

### Module: `rulebooks/dnd5e/equipment` — 1 file

| Symbol | Refs | One citation |
|---|---|---|
| `equipment.Equipment` | 1 | converter |

### Module: `rulebooks/dnd5e/features` — 1 file

| Symbol | Refs | One citation |
|---|---|---|
| `features.CreateFromRefInput`, `features.CreateFromRef` | 1 each | factory call |

This is the only direct touch of the features package. Most feature use is
indirect via refs.

### Module: `rulebooks/dnd5e/actions` — 1 file

| Symbol | Refs | One citation |
|---|---|---|
| `actions.EquippedWeaponInfo`, `actions.AttackHand`, `actions.TwoWeaponGranterInput`, `actions.MartialArtsGranterInput`, `actions.CheckAndGrantOffHandStrike`, `actions.CheckAndGrantMartialArtsBonusStrike` | 1–3 | hand-tracking helper |

### Module: `rulebooks/dnd5e/events` — 1 file

| Symbol | Refs | One citation |
|---|---|---|
| `dnd5eEvents.TurnEndTopic` | 1 | `internal/orchestrators/encounter/orchestrator.go:256` (`turnEndTopic := dnd5eEvents.TurnEndTopic.On(bus)`) |
| `dnd5eEvents.TurnEndEvent` | 1 | `internal/orchestrators/encounter/orchestrator.go:257` (typed event payload) |
| `dnd5eEvents.DamageComponent` | 1 | `internal/orchestrators/encounter/orchestrator.go:616` (`convertToolkitComponent`) |

The package's actual surface is dnd5e-specific event payloads and topic
helpers, plus the `ConditionBehavior` and `ActionBehavior` interfaces. It does
**not** define or re-export `NewEventBus` — that constructor lives in the
top-level `events` package (`events/bus.go:41`). Imports under the
`dnd5eEvents` alias should not be confused with bus construction.

### Module: `rulebooks/dnd5e/resources` — 1 file

| Symbol | Refs | One citation |
|---|---|---|
| (used as constant import, e.g., `resources.RageCharges` indirectly) | rare | character module reference |

The integration helper at `internal/integration/encounter/helpers.go:381` is the
direct hit (under alias `toolkitchar`).

### Module: `rulebooks/dnd5e/monster/actions`, `monster/monsters` — 0 files each

Both subpackages are sibling packages of `rulebooks/dnd5e/monster`, not
imported by rpg-api. They are also not transitively reachable through
`monster` (the parent package can't import `monster/actions` due to an import
cycle, and the built-in monster factories like `NewGoblin` live directly in
the `monster` package — `monster.go:221` — not in `monster/monsters`).

---

### Modules NOT imported by rpg-api at all

Grep returned **zero** for these toolkit modules under `internal/` and `cmd/`:

- `mechanics/conditions`, `mechanics/effects`, `mechanics/features`, `mechanics/proficiency`, `mechanics/resources`, `mechanics/spells`
- `items` (and `items/validation`)
- `tools/spawn`
- `tools/selectables`
- `behavior`
- `core/chain`

These are toolkit-internal infrastructure consumed **only** through
`rulebooks/dnd5e`. From rpg-api's view they are implementation detail.

The honest read: the components/ docs currently dedicate a top-level page each
to `mechanics/` (six sub-modules), `items`, and `tools/spawn`. None of these are
in the rpg-api import graph. That doesn't mean the docs are wrong — they
describe the toolkit's surface. But it does mean none of them describe what
rpg-api consumes. PR 2 should decide whether the components/ docs document the
toolkit's *internal architecture* (in which case mechanics/ stays) or the
toolkit's *consumer-facing surface* (in which case mechanics/ collapses into
rulebook-dnd5e).

---

## Section 2: components/ docs vs actual usage

| Component doc | Documents | Used by rpg-api | Cruft candidate | Missing surface |
|---|---|---|---|---|
| `core.md` | core module | **YES** — `core.Ref`, `core.Entity`, `core.EntityType` (8 files) | Not cruft. Doc currently covers `Action`, `topic.go`. rpg-api does not import `core.Action` directly, but the doc is the right place to explain that Action is the toolkit-internal contract Features implement. (Conditions implement the separate `dnd5eEvents.ConditionBehavior` interface — not Action — see Section 3 Claim 1.) | `core/combat` and `core/resources` are not separately documented; both are imported by rpg-api. Decide whether to fold into `core.md` or split. |
| `events.md` | events module — `EventBus`, `BusEffect`, `TypedTopic`, `ChainedTopic` | **YES** — `events.NewEventBus`, `events.EventBus` (4 files). | Not cruft. `BusEffect` is a toolkit-internal pattern — fine to cover, but worth flagging it's not visible to rpg-api. | The "chain pattern" deserves its own first-class explanation: the chain is `core/chain`, the chained topic is in `events`, and the *worked example* lives in `rulebooks/dnd5e/combat/attack.go`. Currently scattered. |
| `mechanics.md` | conditions, effects, features, proficiency, resources, spells | **NO** — zero direct imports from rpg-api | **Cruft candidate at the consumer level.** Toolkit-internal. PR 2 should either (a) keep as a toolkit-internal architecture doc and label it as such, or (b) collapse the user-facing parts into `rulebook-dnd5e.md` and delete `mechanics.md`. | n/a — mechanics is not part of the rpg-api surface. |
| `items.md` | item interface module | **NO** — zero direct imports from rpg-api | **Cruft candidate.** Items used by rpg-api come through `rulebooks/dnd5e/weapons` etc. | n/a |
| `tools-spatial.md` | hex/square/gridless rooms, multi-room orchestration | **YES** — second-largest dependency (18 files) | Not cruft. Coverage is broadly correct. | The doc lists `Position`, `CubeCoordinate` but the rpg-api hot path is `RoomData`, `EntityCubePlacement`, grid-type and hex-orientation constants — that nuance should surface. |
| `tools-environments.md` | environment graph, persistence, generation | **PARTIAL** — `ConnectionEdge`, `RoomShape`, `ConnectionPoint` only (6 files) | Not cruft, but most of the doc covers things rpg-api doesn't touch (graph generators, wall patterns, persistence). | Worth a short "what rpg-api consumes" callout to set expectations. |
| `tools-spawn.md` | 4-phase spawn engine | **NO** — zero direct imports from rpg-api | **Cruft candidate at the consumer level.** Spawn is currently consumed only by rulebooks (via `dungeon`) or directly by tests. | n/a |
| `rulebook-dnd5e.md` | full dnd5e rulebook | **YES — heavily.** 24 sub-packages imported. This is the main consumer-facing surface. | Not cruft. Doc covers most of the right submodules but is grade-summary level — needs more depth on `refs/`, `gamectx/`, `combat/` (the chain entry), `monster/`, `initiative/`, `character/choices/`. | `refs/` (12 files in rpg-api, 96+ ref histogram) deserves a dedicated section. `gamectx/` is the integration shim — the doc should explain it's how the toolkit looks up combatants during chain resolution. `character/choices/` is its own service-shaped surface and should be called out. |

### Component-doc-shaped gaps

These rpg-api hits don't cleanly map to any existing components doc:

- `core/combat` (1 file in rpg-api: ActionType enum mapping). Almost certainly belongs as a paragraph inside `core.md`, not a new doc.
- `core/resources` (1 file: integration helper). Same — paragraph inside `core.md`.
- `dice` (3 prod files + 3 mock). Currently has no components doc. **Missing.** Worth adding, especially because rpg-api consumes it as a service (dice service rolls + persists session state) — the boundary contract is visible to rpg-api.
- `rulebooks/dnd5e/refs/` (12 files, dominant import). No dedicated section in `rulebook-dnd5e.md`. **Missing.**
- `rulebooks/dnd5e/gamectx/` (3 files, all combat-resolution paths). No dedicated section. **Missing.**

---

## Section 3: architectural truths — confirmed or corrected

### Claim 1: "Conditions and Features are Actions under the hood. Simple typed interface."

**Verdict: PARTIALLY TRUE — needs correction.**

The unified Action interface exists:

- `core/action.go` defines `Action[T any]` with `CanActivate(ctx, owner, input)` and `Activate(ctx, owner, input)`, embedding `Entity` for `GetID/GetType`.
- Features implement it. Example: `Rage` at `rulebooks/dnd5e/features/rage.go:84` and `:106` — comments explicitly state "implements core.Action[FeatureInput]".

**But Conditions do not implement core.Action.** They implement a separate interface
in `rulebooks/dnd5e/events/events.go:85`:

```go
type ConditionBehavior interface {
    IsApplied() bool
    Apply(ctx context.Context, bus events.EventBus) error
    Remove(ctx context.Context, bus events.EventBus) error
    ToJSON() (json.RawMessage, error)
}
```

`RagingCondition` at `rulebooks/dnd5e/conditions/raging.go:46-56` asserts
`var _ dnd5eEvents.ConditionBehavior = (*RagingCondition)(nil)` and implements
Apply/Remove — never CanActivate/Activate.

The actual unifying mental model in code is:

- **Action** = "thing a player or DM activates" (Rage feature, Strike, Dodge as combat ability)
- **ConditionBehavior** = "thing that subscribes to the event bus and modifies chains while applied" (RagingCondition, Defense fighting style, Unconscious)

A Feature can both implement Action **and** apply a Condition as part of its
Activate flow. Rage is the canonical case: `Rage.Activate` (an Action)
constructs and applies a `RagingCondition` (a ConditionBehavior). They are
**related but distinct interfaces** — not "the same thing under the hood".

The CLAUDE.md at `rulebooks/dnd5e/CLAUDE.md` further documents this in the
"Refs Pattern" section: features have FeatureInput, conditions have JSON
loaders. Different shapes.

**Recommendation for PR 2 docs:** explain Action as one half of the activation
surface, ConditionBehavior as the other half (the passive/listener half), and
make clear that features use both — they Activate as Actions and may produce
Conditions that Apply as listeners.

### Claim 2: "The toolkit has business logic, not data orchestration."

**Verdict: CONFIRMED.**

Worked example: `combat.ResolveAttack` at
`rulebooks/dnd5e/combat/attack.go:159` is a multi-stage attack resolver. It:

1. Builds an attack chain (rolls, modifiers, advantage)
2. Calls `dnd5eEvents.AttackChain.On(input.EventBus).PublishWithChain(ctx, attackEvent, attackChain)` — events package, line 226 of attack.go
3. Reads the modified result, computes hit/crit, calls back into the damage chain
4. Publishes `DamageReceivedEvent` so condition subscribers (like RagingCondition's resistance handler) modify damage

rpg-api's role at `internal/orchestrators/encounter/orchestrator.go:388` is:
load both characters from data, set up a fresh bus, populate the combatant
registry via `combat.WithCombatantLookup`, call `combat.ResolveAttack`, then
persist the resulting state. rpg-api does **not** decide hit/miss, AC, damage
type, or condition application. That logic lives in the toolkit.

The boundary holds: rpg-api orchestrates (load → call → save), toolkit
calculates.

### Claim 3: "Domain models like character.Character have a ToData() returning a saveable data struct."

**Verdict: CONFIRMED — pattern is consistent and widespread.**

Confirmed pairs in the toolkit:

- `character.Character.ToData() *Data` at `rulebooks/dnd5e/character/character.go:917`
- `character.LoadFromData(ctx, *Data, events.EventBus) (*Character, error)` at `rulebooks/dnd5e/character/data.go:119`

The same pattern repeats across the rulebook:

- `monster.Monster.ToData()` / `monster.LoadFromData(ctx, *Data, bus)` at `rulebooks/dnd5e/monster/monster.go:673` / `:304`
- `Tracker.ToData()` / `initiative.LoadFromData(TrackerData)` at `rulebooks/dnd5e/initiative/data.go:24` / `:41`
- `Class.ToData()` / `class.LoadFromData(Data)` at `rulebooks/dnd5e/class/types.go:215` / `:123`
- `Race.ToData()` / `race.LoadFromData(Data)` at `rulebooks/dnd5e/race/types.go:163` / `:92`
- `Draft.ToData() *DraftData` at `rulebooks/dnd5e/character/draft_data.go:39`
- `Dungeon.ToData() *DungeonData` at `rulebooks/dnd5e/dungeon/dungeon.go:235`
- Monster-action-data variants at `monster/scimitar_action.go:124`, `monster/actions/melee.go:137`, `monster/actions/bite.go:148`, `monster/actions/ranged.go:139`, `monster/actions/multiattack.go:141`

Canonical example for the docs: **Character**. It's the largest and most
heavily round-tripped — rpg-api's character orchestrator at
`internal/orchestrators/character/orchestrator.go:740` calls
`character.LoadFromData(ctx, createOutput.Character.Data, finalBus)` and
`internal/handlers/dnd5e/v1alpha1/character/converters.go:1047` calls
`char.ToData()` for proto conversion. Both ends of the pattern visible from
inside rpg-api.

Note: Conditions follow a parallel-but-different pattern. They use
`ToJSON()` / per-condition `LoadJSON` (see the repo-root `CLAUDE.md`
"Feature/Condition Serialization Pattern" section — that pattern is documented
at the workspace level, not inside `rulebooks/dnd5e/CLAUDE.md`) because the
rulebook has a routing-by-ref loader. This nuance — ToData for entity-typed
structs, ToJSON for ref-routed effects — is not surfaced in current docs and
should be in PR 2.

### Claim 4: "Event bus + chains is the load-bearing combat architecture."

**Verdict: CONFIRMED.**

Pieces:

- The chain primitive: `core/chain/types.go:17` — `Chain[T]` interface with
  `Add(stage, id, modifier)`, `Remove(id)`, `Execute(ctx, T)`. Generic, stage-ordered, ID-keyed.
- The bus: `events/bus.go` — `EventBus` interface; `events.NewEventBus()` constructor.
- The combat stage definitions: `rulebooks/dnd5e/combat/stages.go:19-31` — five
  named stages: `StageBase`, `StageFeatures`, `StageConditions`, `StageEquipment`, `StageFinal`.
  These are the concrete realization of the "base → features → conditions →
  equipment → final" chain in the project CLAUDE.md vocabulary.
- The chained topic — events package's `ChainedTopic` (covered by current
  `events.md`) is what lets a publish call drive a chain pass through subscribers.

Worked end-to-end attack flow:

1. `internal/orchestrators/encounter/orchestrator.go:382` (rpg-api): set up
   combatant registry, call `combat.WithCombatantLookup(ctx, registry)`.
2. `internal/orchestrators/encounter/orchestrator.go:388` (rpg-api): call
   `combat.ResolveAttack(ctx, &combat.AttackInput{...})`.
3. `rulebooks/dnd5e/combat/attack.go:159` (toolkit): `ResolveAttack` constructs
   an attack chain.
4. `rulebooks/dnd5e/combat/attack.go:226` (toolkit): `attacks :=
   dnd5eEvents.AttackChain.On(input.EventBus); modifiedAttackChain, err :=
   attacks.PublishWithChain(ctx, attackEvent, attackChain)` — chain runs through
   each subscriber's handler in stage order.
5. Subscribers along the chain include condition handlers (e.g.
   `RagingCondition.onDamageReceived` at `rulebooks/dnd5e/conditions/raging.go:56`)
   that were registered when conditions were Apply'd to the bus.
6. After the chain returns, `ResolveAttack` resolves hit/damage and publishes
   `DamageReceivedEvent` (`rulebooks/dnd5e/combat/attack.go:404`).
7. Condition Apply handlers also subscribe to the damage topic — the rage
   resistance halving and the rage-was-hit-this-turn tracking in `raging.go`
   both trigger here.

This is the load-bearing architecture: typed bus, stage-ordered chain, named
stages defined per-domain (combat has its own `stages.go`). PR 2's components
docs should anchor the chain description here — concrete file, concrete
attack — rather than describing it abstractly.

---

## Section 4: recommendations for PR 2

Terse — PR 2 will turn these into a rewrite plan.

**Survive (with edits):**

- `core.md` — keep. Add a paragraph each on `core/combat` (action-economy enum) and `core/resources` (resource accessor interface). Keep `Action[T]` coverage but reframe: Action is the activation half, ConditionBehavior (in rulebooks/dnd5e/events) is the passive/listener half.
- `events.md` — keep. Add a "chain pattern" section that names `core/chain`, references `events.ChainedTopic`, and links the worked example in `rulebooks/dnd5e/combat`.
- `tools-spatial.md` — keep. Reorder around the rpg-api hot path: `RoomData`, `EntityCubePlacement`, grid types and hex orientations.
- `tools-environments.md` — keep, scope it. Add a "what rpg-api consumes" callout: ConnectionEdge, RoomShape, ConnectionPoint. The graph-generator and persistence sections are fine but should be marked "toolkit-internal".
- `rulebook-dnd5e.md` — keep, expand significantly.

**Merge / restructure:**

- Promote `refs/` to a dedicated section inside `rulebook-dnd5e.md`. It is the single largest non-character import and the literal API surface of the boundary rule.
- Promote `gamectx/` to a dedicated section inside `rulebook-dnd5e.md`. It's the integration shim that lets toolkit chain resolution see all combatants.
- Promote `character/choices/` to a dedicated section inside `rulebook-dnd5e.md`. It is its own service-shaped surface.
- Conditions and Features — keep as separate sections in `rulebook-dnd5e.md` (they're separate interfaces) but add a new top-level "Activation surface" section that explains how Action and ConditionBehavior fit together: features Activate, may Apply conditions, conditions subscribe and modify chains.

**Create:**

- `dice.md` — a new components page. rpg-api consumes dice as a service (Roller, Service, Input/Output types, MockRoller). Currently undocumented.

**Delete or relabel:**

- `mechanics.md` — rpg-api does not import any `mechanics/*` directly. PR 2 decides: (a) delete and fold any genuinely user-facing parts into `rulebook-dnd5e.md`, or (b) keep but re-label "toolkit-internal — not part of the rpg-api surface" so future readers don't expect to see it in the rpg-api import graph.
- `items.md` — same call. rpg-api uses `weapons`/`armor` from rulebooks/dnd5e, never `items` directly. Either delete or label as toolkit-internal.
- `tools-spawn.md` — same call. Not in the rpg-api import graph. Either delete or label as toolkit-internal.

The deletion calls are architectural-judgement and explicitly belong to Kirk per
the autonomous-waves design (Phase 1: agent surfaces, human decides). PR 2
should *propose* the call for each, with the cite count, and let Kirk make the
final ruling.

---

## Verification commands

A reviewer can run these to confirm the audit's claims. All paths absolute.

Top-level import graph:

```
grep -rh "github.com/KirkDiggler/rpg-toolkit" /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | grep -oE 'github.com/KirkDiggler/rpg-toolkit/[a-zA-Z0-9_/\-]+' | sort -u
```

Empty buckets (each command should return 0):

```
grep -rln '"github.com/KirkDiggler/rpg-toolkit/mechanics' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l
grep -rln '"github.com/KirkDiggler/rpg-toolkit/items' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l
grep -rln '"github.com/KirkDiggler/rpg-toolkit/tools/spawn' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l
grep -rln '"github.com/KirkDiggler/rpg-toolkit/tools/selectables' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l
grep -rln '"github.com/KirkDiggler/rpg-toolkit/behavior' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l
grep -rln '"github.com/KirkDiggler/rpg-toolkit/core/chain' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l
```

File-count-per-module spot checks (a few representative ones):

```
grep -rln '"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l   # 24
grep -rln '"github.com/KirkDiggler/rpg-toolkit/tools/spatial"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l            # 18
grep -rln '"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l   # 14
grep -rln '"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l      # 12
grep -rln '"github.com/KirkDiggler/rpg-toolkit/core"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l                     # 8
```

Action interface vs ConditionBehavior interface (Section 3 claim 1):

```
# Action interface and Rage's implementation
grep -n 'type Action\[T any\] interface' /home/kirk/personal/rpg-toolkit/core/action.go
grep -n 'core\.Action\[FeatureInput\]\|CanActivate\|func .* Activate' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/features/rage.go

# ConditionBehavior interface and Raging's implementation
grep -n 'type ConditionBehavior interface' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/events/events.go
grep -n 'ConditionBehavior\|func .* Apply\|func .* Remove' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/conditions/raging.go
```

ToData/LoadFromData (Section 3 claim 3):

```
grep -rn 'func .* ToData()\|func LoadFromData' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/ --include="*.go" | head -20
```

Chain stages and attack flow (Section 3 claim 4):

```
grep -n 'chain\.Stage' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/combat/stages.go
grep -n 'PublishWithChain\|attackChain' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/combat/attack.go | head
grep -n 'combat.ResolveAttack' /home/kirk/personal/rpg-api/internal/orchestrators/encounter/orchestrator.go
grep -n 'combat.WithCombatantLookup' /home/kirk/personal/rpg-api/internal/orchestrators/encounter/orchestrator.go
```

---

## Things I could not verify

- Whether `monster.Data`'s deprecation comment ("DEPRECATED: migrating to Entities") at `internal/repositories/encounters/repository.go:87` actually reflects in-flight work or a stalled migration. I read the comment but didn't trace the migration's status. PR 2 author or Kirk: this might be a separate cruft signal.
- Whether `rulebooks/dnd5e/events.NewEventBus` is a real re-export or whether the import I caught is a false positive. The 5 `events.NewEventBus` references in the histogram for `dnd5e/events` came from a single-file scan; at most one of those files actually imports `dnd5e/events`. PR 2 should re-grep with the alias resolved.
- I did not run any Go build or test in either repo. Imports are static-grepped only. If a file is build-tagged out, my count includes it.
- The rpg-api `dice.Service` import shape suggests dice is "service-style" inside rpg-api, but I did not trace whether dice's *internal* implementation owns persistence (Redis) or if rpg-api wraps it. PR 2 should clarify that boundary if dice gets its own component doc.

## Things I discovered off-script

- **`gamectx` is a real and important package** that no current components doc mentions. It owns the combatant registry and the context-key plumbing that lets toolkit chain resolution look up combatants by ID. rpg-api drives it from `internal/orchestrators/encounter/orchestrator.go`. This is the most surprising omission from current docs.
- **`refs` is the single most-referenced symbol family from rpg-api** (the literal API of the boundary rule). It has no dedicated docs section. This is the second most surprising omission.
- **Conditions and Features are not the same interface.** Kirk's framing of them as both being "Actions under the hood" is the audit's most material correction. They are sibling interfaces with related but distinct contracts: Action (Activate) and ConditionBehavior (Apply). The unification is conceptual ("things that change combat outcomes") not mechanical (one Go interface).
- **rpg-api does not import `mechanics/`, `items`, `tools/spawn`, `tools/selectables`, `behavior`, or `core/chain`** at all. From the consumer's perspective these are toolkit-internal. The components docs currently treat them as first-class — a defensible choice for a toolkit-internal-architecture audience but the wrong choice for a consumer-facing audience. PR 2 picks the audience.
