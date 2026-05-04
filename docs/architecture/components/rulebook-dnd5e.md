---
name: rulebooks/dnd5e module
description: D&D 5e rules implementation — the consumer-facing surface rpg-api imports across 31 sub-packages (character/ alone in 24 files)
updated: 2026-05-04
confidence: high — verified by directory listing, grep over public symbols, and rpg-api import-graph audit 049
---

# rulebooks/dnd5e module

**Path:** `rulebooks/dnd5e/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e`
**Grade:** B+

The top of the dependency tree. The most actively worked module. Implements
all D&D 5e game rules: character creation and leveling, combat resolution,
initiative, features (Rage, Second Wind, Martial Arts, etc.), conditions
(Raging, Dodging, Unconscious, etc.), spells, monsters, and dungeon layouts.

## What rpg-api consumes

Per a fresh grep on 2026-05-04, rpg-api imports **31 sub-packages** of
`rulebooks/dnd5e` (the `character/` package alone is imported by 24 rpg-api
files, which is where the audit's intro narrative's "24 sub-packages"
phrasing comes from — that count is files-importing-`character`, not
total sub-packages). The audit Section 1 marks
`monster/actions` and `monster/monsters` as 0 files each; that's wrong —
both are imported (`monster/actions` from
`internal/orchestrators/encounter/monster_turns.go`; `monster/monsters` from
`internal/components/dungeon/monster_factory.go` and
`internal/orchestrators/encounter/orchestrator_test.go`). Audit needs a
follow-up correction; this doc's table reflects the fresh count.

This is the dominant consumer-facing surface. The top imports by file count:

| Sub-package | Files in rpg-api | Notes |
|---|---|---|
| `character/` | 24 | Character/Draft lifecycle, ToData/LoadFromData |
| `monster/` | 14 | Monster data, NewGoblin, perception |
| `refs/` | 12 | The boundary key — `refs.Weapons.Longsword()`, `refs.Features.Rage()`, etc. |
| `classes/` | 11 | Typed class constants |
| `abilities/` | 10 | DEX/STR/CON/WIS/INT/CHA constants |
| `shared/` | 9 | Cross-cutting types (`AbilityScores`, `SelectionID`) |
| `races/` | 8 | Typed race constants |
| `initiative/` | 7 | Tracker, Roll, Participant |
| `character/choices/` | 6 | Service-shaped choice/validation surface |
| `combat/` | 5 | `ResolveAttack`, `WithCombatantLookup`, action-economy types |
| `weapons/` | 5 | Weapon data |
| `backgrounds/` | 4 | Typed background constants |
| `damage/` | 3 | Damage type constants |
| `spells/` | 3 | Spell typed constants (resolution is internal) |
| `armor/` | 3 | Armor data |
| `gamectx/` | 3 | Combatant registry — the integration shim for chain resolution |
| `skills/` | 2 | Skill constants and `Skill` type |
| `monstertraits/` | 2 | `LoadMonsterConditions` |
| `fightingstyles/` | 2 | Fighting style constants |
| `languages/` | 2 | Language constants |
| `ammunition/`, `packs/`, `tools/`, `proficiencies/`, `saves/`, `equipment/`, `features/`, `actions/`, `events/`, `resources/` | 1 each | narrow imports — see audit Section 1 |

Most rpg-api callsites send **refs in** and receive **rich breakdowns out**.
The toolkit owns the rules; rpg-api orchestrates load → call → save.

## Sub-package map (toolkit-side)

| Sub-package | Purpose | Test coverage |
|---|---|---|
| `character/` | Character struct, ToData/LoadFromData, finalization | High — full suite with fixture-driven tests |
| `character/choices/` | Choice system (class/race at creation) | Medium — testdata from external API |
| `combat/` | AC chain, attack resolution, damage, healing, action economy | High — integration and unit tests |
| `combatabilities/` | Attack, Dash, Disengage, Dodge, Hide, Move | Medium — `move.go` minimally tested |
| `actions/` | Strike, OffHandStrike, CheckAndGrant | High — used in integration tests |
| `features/` | Feature loader for dnd5e features | High — Rage, SecondWind, MartialArts, etc. |
| `conditions/` | Condition loader + all named conditions | High — loader test, individual condition tests |
| `initiative/` | Initiative roll + tracker | High |
| `saves/` | Saving throw resolution | Medium |
| `skills/` | Skill check resolution | Medium |
| `monster/` | Monster stat block + turn execution | High — used in integration tests |
| `monster/actions/` | Bite, melee, ranged, multiattack — sibling of `monster/` | High |
| `monster/monsters/` | Bandit, Brown Bear, Ghoul — sibling of `monster/` | High |
| `monstertraits/` | Special monster abilities | Medium |
| `resources/` | Resource loading (ki, rage uses, spell slots) | Medium |
| `spells/` | Spell list, slot management | Medium |
| `equipment/` | Equipment slots + item interface | Medium |
| `weapons/` | Weapon definitions and proficiencies | High — used in combat tests |
| `dungeon/` | Procedural dungeon: room types, wall perimeters, door spawning | Medium (336 test lines) |
| `gamectx/` | D&D-specific game-context plumbing — combatant registry, characters, room | Low |
| `refs/` | Typed ref constructors (`refs.Features.Rage()`) | Medium |
| `shared/` | Shared type aliases (EquipmentID, etc.) | — |
| `events/` | dnd5e event payloads, topics, `ConditionBehavior`, `ActionBehavior` | High |
| `integration/` | Full encounter integration tests (Barbarian/Fighter/Monk/Rogue) | High |

## go.mod status

Clean. All published versions. No replace directives.

Key dependencies: `core`, `dice`, `events`, `mechanics/resources`, `rpgerr`,
`tools/environments`, `tools/spatial`.

## refs/ — the boundary key

The single biggest non-character import (12 files in rpg-api, ~400 ref usages
across the histogram). The boundary rule of the project — "client sends
references, never calculations" — makes `refs/` the literal API surface
between rpg-api and the toolkit.

The `refs/` package exposes namespaced constructors:

```go
refs.Features.Rage()           // *core.Ref{Module: "dnd5e", Type: "features", ID: "rage"}
refs.Conditions.Raging()       // *core.Ref{Module: "dnd5e", Type: "conditions", ID: "raging"}
refs.CombatAbilities.Attack()  // *core.Ref{Module: "dnd5e", Type: "combat_abilities", ID: "attack"}
refs.Actions.Strike()          // *core.Ref{Module: "dnd5e", Type: "actions", ID: "strike"}
```

Methods return **singleton pointers**, enabling identity comparison
(`ref == refs.Features.Rage()` is true if both came from the same
constructor). Every namespace (`Features`, `Conditions`, `Actions`,
`CombatAbilities`, `Weapons`, `Armor`, `Spells`, `Monsters`, `Classes`,
`Races`, `Backgrounds`, `Skills`, `Abilities`, `Tools`, `DamageTypes`,
`Languages`, `MonsterActions`, `MonsterTraits`) is a struct singleton with
methods returning unexported `*core.Ref` variables.

The histogram from rpg-api's source (audit Section 1):

| Namespace | Refs in rpg-api |
|---|---|
| `refs.Weapons` | 96 |
| `refs.Features` | 85 |
| `refs.Conditions` | 51 |
| `refs.Actions` | 45 |
| `refs.Monsters` | 43 |
| `refs.Tools` | 35 |
| `refs.CombatAbilities` | 31 |
| `refs.Abilities` | 15 |
| `refs.Module`, `refs.Armor` | 14, 13 |

How the boundary rule looks in practice: rpg-api passes `refs.Features.Rage()`
to a toolkit factory or activation call; the toolkit looks up the
implementation by ref, owns the rule, and returns a result. rpg-api never
implements "what Rage does."

The full `refs/` surface and how it composes with `core.Ref` /
`core.SourcedRef` is documented separately in `refs.md`.

## gamectx/ — the integration shim for chain resolution

`gamectx/` is the most surprising omission from the previous version of this
doc (per audit "things discovered off-script"). It owns the combatant registry
and the context-key plumbing that lets toolkit chain resolution look up
combatants by ID during an attack.

Key types (verified by `grep` over `gamectx/*.go`):

| Symbol | File | Role |
|---|---|---|
| `GameContext`, `GameContextConfig`, `NewGameContext` | `gamectx.go` | aggregate game-context value carried in `context.Context` |
| `WithGameContext`, `Characters`, `RequireCharacters` | `require.go` | context-key plumbing for the character registry |
| `CharacterRegistry`, `BasicCharacterRegistry`, `NewBasicCharacterRegistry` | `gamectx.go`, `characters.go` | per-character registry (weapons, ability scores) |
| `CombatantRegistry`, `NewCombatantRegistry`, `WithCombatants`, `GetCombatant` | `combatant.go` | the registry the attack chain consults during resolution |
| `EquippedWeapon`, `CharacterWeapons`, `SlotMainHand`, `SlotOffHand` | `characters.go` | weapon-slot plumbing |
| `WithRoom`, `Room`, `RequireRoom` | `room.go` | spatial context |
| `CombatState`, `WithCombatState` | `combat.go` | per-encounter combat state |

rpg-api drives this from `internal/orchestrators/encounter/orchestrator.go`:

```go
ctx = gamectx.WithGameContext(ctx, gameCtx)
registry := gamectx.NewCombatantRegistry()
// ... populate registry ...
ctx = combat.WithCombatantLookup(ctx, registry)
result, err := combat.ResolveAttack(ctx, &combat.AttackInput{ /* ... */ })
```

Inside `combat.ResolveAttack` (and inside subscriber handlers along the attack
chain), code calls `combat.GetCombatantFromContext(ctx, id)` — which reads
back through the context keys gamectx sets up. Without this shim the chain
has no way to find "what is the attacker's STR mod" or "is the target
prone."

## combat/ — the chain entry point

`combat/` is where the attack chain entry point lives. The worked example for
the chain pattern (see `events.md`) is `combat.ResolveAttack` in
`rulebooks/dnd5e/combat/attack.go` — search for the `func ResolveAttack`
symbol.

Top symbols rpg-api consumes:

| Symbol | Role |
|---|---|
| `combat.AttackInput`, `combat.ResolveAttack` | the chain entry point |
| `combat.WithCombatantLookup` | wires the gamectx registry into context |
| `combat.AttackHandMain`, `combat.AttackHandOff`, `combat.AttackHand` | which hand is attacking |
| `combat.AttackResult`, `combat.DamageBreakdown` | return shape with rich modifier provenance |
| `combat.NewActionEconomy`, `combat.CapacityFlurryStrike` | action-economy support |

The chain stages (`StageBase`, `StageFeatures`, `StageConditions`,
`StageEquipment`, `StageFinal`) are defined in
`rulebooks/dnd5e/combat/stages.go`. See `events.md` for how `StagedChain[T]`
and `ChainedTopic[T]` cooperate to drive the chain.

## character/choices/ — service-shaped surface

`character/choices/` is its own service-shaped surface inside the rulebook
(6 files in rpg-api). rpg-api treats it like a service: ask for requirements,
post a validation request, get a `ValidationResult`. Hot path:

| Symbol | Role |
|---|---|
| `choices.ValidationResult`, `choices.ValidationError` | return shape (11, 2 references) |
| `choices.GetClassRequirements`, `choices.GetClassRequirementsAtLevel`, `choices.GetClassRequirementsWithSubclass`, `choices.GetRaceRequirements` | per-step requirement queries |
| `choices.Requirements`, `choices.ChoiceData`, `choices.ChoiceID` | request shape |
| Class-specific selectors (`choices.FighterPack`, `choices.WizardWeaponsPrimary`, etc.) | typed equipment-choice constants |
| `choices.SkillRequirement`, `choices.ToolRequirement`, `choices.FightingStyleRequirement`, `choices.EquipmentRequirement` | requirement-type taxonomy |

Subclass modifications live in `subclass_modifications.go`. The `submissions.go`
file is the input shape rpg-api converts proto requests into. Implementation
notes are in `character/choices/CHOICES_SYSTEM.md`.

## monster/ — note on sibling sub-packages

`monster/` is imported by 14 rpg-api files. **Important nuance**:
`monster/actions` and `monster/monsters` are **sibling sub-packages** of
`monster/`, not subdirectories in the import sense — they cannot be reached
through `monster/` because of an import cycle. rpg-api imports them directly
where it needs them: `monster/actions` from
`internal/orchestrators/encounter/monster_turns.go` (1 file) and
`monster/monsters` from `internal/components/dungeon/monster_factory.go` and
`internal/orchestrators/encounter/orchestrator_test.go` (2 files). The audit
at `docs/journey/049-rpg-api-toolkit-usage-audit.md` Section 1 incorrectly
records both as 0 files; this needs a follow-up correction to the audit.

The built-in monster factory functions like `NewGoblin` live in
`rulebooks/dnd5e/monster/monster.go` (search for `func NewGoblin`), **not** in
`monster/monsters/`. `monster/monsters/` contains `Bandit`, `BrownBear`, and
`Ghoul` — newer additions. `monster/actions/` contains `Bite`, `Melee`,
`Ranged`, `Multiattack` — the action types monsters compose into their
turns.

Top symbols rpg-api consumes from `monster/`:

| Symbol | Role |
|---|---|
| `monster.Data` | persisted monster shape (62 references; deprecation comment in rpg-api repository at `internal/repositories/encounters/repository.go` flags migration in flight) |
| `monster.LoadFromData` | reconstitute a Monster from Data + bus |
| `monster.PerceivedEntity`, `monster.PerceptionData` | perception output |
| `monster.NewGoblin`, `monster.ScimitarConfig` | encounter-setup helpers |
| `monster.ActionData`, `monster.TypeMeleeAttack`, `monster.TypeRangedAttack`, `monster.TakeDamage` | action plumbing |

## initiative/ — round/turn tracker

7 files in rpg-api. The tracker is reconstituted from data in repository load
paths:

| Symbol | Role |
|---|---|
| `initiative.TrackerData` | the persisted shape (54 references) |
| `initiative.EntityData` | per-entity entry in the tracker |
| `initiative.NewParticipant` | participant constructor |
| `initiative.Roll` | typed roll result |
| `initiative.New` | tracker constructor |
| `initiative.RollForOrder` | roll all initiatives at once |
| `initiative.LoadFromData` | reconstitute from `TrackerData` |

Persistence pattern matches the broader `ToData`/`LoadFromData` convention
(see `docs/architecture/data-model.md`).

## character/ — the dominant import

24 files in rpg-api. This module owns the character domain model:

| Symbol | Role |
|---|---|
| `character.Data` | the persisted character shape (47 references) |
| `character.Character`, `character.LoadFromData` | runtime character |
| `character.DraftData`, `character.DraftConfig`, `character.NewDraft`, `character.LoadDraftFromData` | draft lifecycle |
| `character.SetRaceInput`, `character.SetClassInput`, `character.SetBackgroundInput`, `character.SetAbilityScoresInput`, `character.SetNameInput` | per-step setters |
| `character.Progress`, `character.ProgressClass`, `character.ProgressRace`, `character.ProgressName` | draft progress |
| `character.EquipmentSlots`, `character.SlotMainHand`, `character.SlotOffHand` | equipment slots |
| `character.ActionEconomyData`, `character.GrantedActionKey` | action-economy |
| `character.ClassChoices`, `character.RaceChoices` | choice-data conversion |
| `character.GetCharacterInput`, `character.DeleteCharacterInput` | service-shape inputs |

The canonical `ToData` / `LoadFromData` round-trip lives here:

```go
// Create and finalize
char := character.New(config)
char.Finalize()

// Serialize (what rpg-api stores)
data := char.ToData()

// Reconstitute (what rpg-api does before any rule call)
newChar, err := character.LoadFromData(ctx, data, bus)
```

`LoadFromData` reconnects the event bus — conditions and features resubscribe
their handlers on reconstruct. This is the critical step that makes the
toolkit stateless from rpg-api's perspective.

## Activation surface — features Activate, conditions Apply

Per audit Section 3 Claim 1, the activation surface has **two halves**:

- **Features** implement `core.Action[T]` (Activate / CanActivate). When a player triggers Rage, rpg-api calls into the feature's Activate flow.
- **Conditions** implement `dnd5eEvents.ConditionBehavior` (Apply / Remove / IsApplied / ToJSON). They subscribe to the bus when applied and modify chains as events flow through.

A feature can both implement Action **and** apply a Condition as part of its
Activate flow. Rage is the canonical case: `Rage.Activate` (an Action)
constructs and applies a `RagingCondition` (a ConditionBehavior). The feature
is "do something now"; the condition is "be present on the bus while
applied."

This nuance is also covered in `core.md` (the interface definitions) and
`events.md` (how the condition's subscriptions interact with the chain
during attack resolution). The split is real and architectural: it lets the
toolkit's combat resolution stay ignorant of which specific features or
conditions are active — the chain just publishes, and whichever subscribers
were Apply'd respond.

## Integration tests

`integration/` contains full encounter simulations. Each file creates a
character, enters combat, and exercises the full rule pipeline:
- `barbarian_encounter_test.go` — Rage activation, reckless attack, extra attack
- `fighter_encounter_test.go` — Second Wind, Action Surge, Fighting Style
- `monk_encounter_test.go` — Martial Arts, Flurry of Blows, ki expenditure, Unarmored Defense
- `rogue_encounter_test.go` — Sneak Attack, Cunning Action, DEX-based combat

These are the most valuable tests in the toolkit. They exercise the full
chain: character creation → finalization → combat entry → feature
activation → attack resolution → resource consumption.

## Known gaps

### Untested grant logic (issue #615)

`backgrounds/grants.go` (172 lines) implements `GetGrants(bg Background)
*Grant` — a switch on 13 background types returning skill proficiencies, tool
proficiencies, and language grants. **No test file.** The switch is
straightforward data-mapping but is non-trivial: wrong skill assignments
break character creation in rpg-api.

`races/grants.go` (109 lines) implements `GetGrants(race Race) *Grant` —
racial language grants, skill proficiencies (e.g., Half-Orc's Intimidation
from Menacing), weapon/armor proficiencies (not yet populated). **No test
file.**

Both are called during character creation finalization. A bug here silently
produces a character with wrong proficiencies.

### dungeon/ location (planned move)

`rulebooks/dnd5e/dungeon/` implements procedural dungeon generation: room
shape selection, hex-perimeter wall calculation, door spawning, theme-based
layout. It uses `tools/environments` and `tools/spatial` (correct direction —
lower layers), but living inside the rulebook means rpg-api must import the
full dnd5e module to use dungeon logic. The planned move is to
`tools/dungeon/` or a standalone module. No issue filed yet.

### character/choices testdata provenance

`character/choices/testdata/api/classes/` and `testdata/api/races/` contain
JSON fixtures from an external API. No note documents when they were fetched,
from which URL, or how to refresh them. If the upstream API changes its
schema, tests silently test stale data.

### combatabilities/move.go

`move.go` (movement action) is tested minimally — no test for stopping
reasons, multi-leg paths, or movement exhaustion mid-turn. This matters for
the multi-room dungeon when movement spans a door.

### monster.Data deprecation comment

The audit flagged a deprecation comment on `monster.Data` in rpg-api's
repository file (`internal/repositories/encounters/repository.go`) — "DEPRECATED:
migrating to Entities." Whether that migration is in flight or stalled wasn't
verified. Track separately when the migration's status is clarified.

## Verification

```sh
# Sub-package import surface
grep -rln '"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l   # 24
grep -rln '"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l    # 14
grep -rln '"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l       # 12
grep -rln '"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/gamectx"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l    # 3

# Combat chain entry point
grep -n 'combat.ResolveAttack\|combat.WithCombatantLookup' /home/kirk/personal/rpg-api/internal/orchestrators/encounter/orchestrator.go | head

# NewGoblin location (NOT in monster/monsters)
grep -n 'func NewGoblin' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/monster/monster.go

# refs surface
grep -nE '^var [A-Z]' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/refs/*.go | grep -v _test
```
