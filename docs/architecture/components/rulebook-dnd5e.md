---
name: rulebooks/dnd5e module
description: Full D&D 5e rules implementation — character, combat, initiative, spells, monsters, dungeon, features, conditions
updated: 2026-05-02
confidence: high — verified by directory listing, go.mod, test runs, and reading key source files
---

# rulebooks/dnd5e module

**Path:** `rulebooks/dnd5e/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e`
**Grade:** B+

The top of the dependency tree. Most actively worked module. Implements all D&D 5e game rules: character creation and leveling, combat resolution, initiative, features (Rage, Second Wind, Martial Arts, etc.), conditions (Raging, Dodging, Unconscious, etc.), spells, monsters, and dungeon layouts.

## Sub-package map

| Sub-package | Purpose | Test coverage |
|---|---|---|
| `character/` | Character struct, ToData/LoadFromData, finalization | High — full suite with fixture-driven tests |
| `character/choices/` | Choice system (class/race at creation) | Medium — testdata from external API |
| `combat/` | AC chain, attack resolution, damage, healing, action economy | High — integration and unit tests |
| `combatabilities/` | Attack, Dash, Disengage, Dodge, Hide, Move | Medium — move.go minimally tested |
| `actions/` | Strike, OffHandStrike, CheckAndGrant | High — used in integration tests |
| `features/` | Feature loader for dnd5e features | High — Rage, SecondWind, MartialArts, etc. |
| `conditions/` | Condition loader + all named conditions | High — loader test, individual condition tests |
| `initiative/` | Initiative roll + tracker | High |
| `saves/` | Saving throw resolution | Medium |
| `skills/` | Skill check resolution | Medium |
| `monster/` | Monster stat block + turn execution | High — used in integration tests |
| `monstertraits/` | Special monster abilities | Medium |
| `resources/` | Resource loading (ki, rage uses, spell slots) | Medium |
| `spells/` | Spell list, slot management | Medium |
| `equipment/` | Equipment slots + item interface | Medium |
| `weapons/` | Weapon definitions and proficiencies | High — used in combat tests |
| `dungeon/` | Procedural dungeon: room types, wall perimeters, door spawning | Medium (336 test lines) |
| `gamectx/` | D&D-specific game context helpers | Low |
| `refs/` | Typed ref constructors (`refs.Features.Rage()`) | Medium |
| `shared/` | Shared type aliases (EquipmentID, etc.) | — |
| `integration/` | Full encounter integration tests (Barbarian/Fighter/Monk/Rogue) | High |
| **Data packages (no logic)** | | |
| `abilities/` | Ability score constants | None |
| `ammunition/` | Ammunition type constants | None |
| `armor/` | Armor type constants | None |
| `damage/` | Damage type constants | None |
| `effects/` | Effect constants | None |
| `fightingstyles/` | Fighting style constants | None |
| `languages/` | Language constants | None |
| `packs/` | Equipment pack constants | None |
| `proficiencies/` | Proficiency type constants | None |
| `race/` | Race constants | None |
| **Logic packages, no tests** | | |
| `backgrounds/` | Background data + `grants.go` | None — issue #615 |
| `races/` | Race data + `grants.go` | None — issue #615 |
| `items/` | Item type constants | None |
| `classes/` | Class definitions | None |

## go.mod status
Clean. All published versions. No replace directives.

Dependencies:
- `core v0.10.0`
- `dice v0.3.2`
- `events v0.6.2`
- `mechanics/resources v0.3.1`
- `rpgerr v0.1.1`
- `tools/environments v0.4.0`
- `tools/spatial v0.4.0`

## Integration tests

`integration/` contains full encounter simulations. Each file creates a character, enters combat, and exercises the full rule pipeline:
- `barbarian_encounter_test.go` — Rage activation, reckless attack, extra attack
- `fighter_encounter_test.go` — Second Wind, Action Surge, Fighting Style
- `monk_encounter_test.go` — Martial Arts, Flurry of Blows, ki expenditure, Unarmored Defense
- `rogue_encounter_test.go` — Sneak Attack, Cunning Action, DEX-based combat

These are the most valuable tests in the toolkit. They exercise the full chain: character creation → finalization → combat entry → feature activation → attack resolution → resource consumption.

## Known gaps

### Untested grant logic (issue #615)
`backgrounds/grants.go` (172 lines) implements `GetGrants(bg Background) *Grant` — a switch on 13 background types returning skill proficiencies, tool proficiencies, and language grants. **No test file.** The switch is straightforward data-mapping but is non-trivial: wrong skill assignments break character creation in rpg-api.

`races/grants.go` (109 lines) implements `GetGrants(race Race) *Grant` — racial language grants, skill proficiencies (e.g., Half-Orc's Intimidation from Menacing), weapon/armor proficiencies (not yet populated). **No test file.**

Both are called during character creation finalization. A bug here silently produces a character with wrong proficiencies.

### dungeon/ location (planned move)
`rulebooks/dnd5e/dungeon/` implements procedural dungeon generation: room shape selection, hex-perimeter wall calculation, door spawning, theme-based layout. It uses `tools/environments` and `tools/spatial` (correct direction — lower layers), but living inside the rulebook means rpg-api must import the full dnd5e module to use dungeon logic. The planned move is to `tools/dungeon/` or a standalone module. No issue filed yet.

### character/choices testdata provenance
`character/choices/testdata/api/classes/` and `testdata/api/races/` contain JSON fixtures from an external API. No note documents when they were fetched, from which URL, or how to refresh them. If the upstream API changes its schema, tests silently test stale data.

### combatabilities/move.go
`move.go` (movement action) is tested minimally — no test for stopping reasons, multi-leg paths, or movement exhaustion mid-turn. This matters for the multi-room dungeon when movement spans a door.

## The LoadFromData round-trip

The character serialization round-trip is well-tested. The pattern:

```go
// Create and finalize
char := character.New(config)
char.Finalize()

// Serialize (what rpg-api stores)
data := char.ToData()

// Reconstitute (what rpg-api does before any rule call)
newChar, err := character.LoadFromData(ctx, data, bus)
// newChar has the same conditions, features, and resources as char
```

`LoadFromData` reconnects the event bus — conditions and features resubscribe their handlers on reconstruct. This is the critical step that makes the toolkit stateless from rpg-api's perspective.
