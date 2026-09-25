# D&D 5e Character Package

## Purpose

Character creation and management for D&D 5e using toolkit infrastructure:
the draft → finalize creation workflow, choice recording for class, race,
background and equipment, and character compilation (abilities,
proficiencies, inventory, spells, features).

We implement: D&D 5e character creation rules and validation.
We use: core toolkit infrastructure (events, proficiencies, effects, equipment).

The mechanics deep dive — the two-phase pattern, the choice model, source
clearing, compilation, worked examples, testing, and known gaps — lives in
`docs/architecture/components/rulebook-dnd5e-character.md`.

## Laws

- **Draft is mutable; Character is immutable.** Finalize compiles a draft into
  a Character; there is no in-place modification of a finalized character.
- **A choice carries three identifiers:** `Source` (where the choice came
  from), `Category` (what kind of choice), `ChoiceID` (which requirement).
  Equipment adds `OptionID` — the selection within a requirement. ChoiceID and
  OptionID are different things: "fighter-armor" is the requirement,
  "fighter-armor-a" is one option inside it.
- **Source changes clear before they record.** SetClass/SetRace/SetBackground
  remove every old choice of that source first, then record the new ones.
  Why: different classes use different ChoiceIDs, so an old class's equipment
  choices do not conflict with the new class's — without the clear they
  accumulate. (Incident record: issue #344.)
- **Equipment is multi-level: Choice → Option → Items.** A class carries
  several equipment choices (armor, weapons, pack), each with options; all
  are `SourceClass` and `ChoiceEquipment`, differentiated by ChoiceID.
- **Setters validate their inputs; Finalize validates completeness.** Progress
  must be complete before Finalize succeeds.
- **A choice is player-decided and recorded; a calculation is rule-derived
  and compiled.** Ask which one a new field is, and what its source is, before
  writing it.

## New-class acceptance includes the private sheet

Use a draft created through the public choice/finalization path for at least
one regression. Do not seed resources, known spells, or class features to make
that creation regression pass. A focused condition fixture may be added after
finalization to exercise projection, but is not evidence of a successful cast.

Before declaring the class supported:

1. Finalize, serialize, reload, and call `StatusView` on the resulting character.
   Verify the actual resource keys, display names, capacities and current values.
2. Inspect the owner-resource and feature-resource catalogs in `status_view.go`,
   plus condition display/loader registration. Add only entries the class really
   owns; preserve rejection of unknown/cross-class data with no partial output.
3. Verify projected status after spending, effects, reload and rest. Source-qualified
   effects must preserve separate source identities; unqualified effects must not
   invent a source. Returned views must remain detached from later mutations.
4. Hand off private-sheet, offer, execution and live/reloaded-result acceptance to
   API/web using the actual published provider version. Provider projection does
   not establish that host authorization/observability or browser rendering works.

## Levelling up: the record, not a re-finalized draft

A character keeps an append-only record of the levels it has taken
(`Data.Levels`), and `Character.Advance` appends one. `Data.Level` and
`Data.ProficiencyBonus` are PROJECTIONS of that record — `ToData` writes them
from it, `Load` refuses a sheet where the two disagree, and nothing in the
toolkit reads them back as truth.

The entry holds the INPUTS to a level — the class it was taken in, the hit
point method and its result, the choices it required — never the effects. What
a level granted is derived from the entry plus the current rules, so correcting
a rule corrects every character. Storing the effects instead reads better and
rots: the day a grant is fixed, every stored account of it becomes a permanent
description of something that should not have happened.

Grants and class resources are indexed by **class level**
(`Character.ClassLevel`); the proficiency bonus is derived from **character
level** (`Character.GetLevel`). They are equal today and are still written down
as different numbers, because that is what multiclassing will need and it costs
one loop now.

The re-finalize-a-draft pattern is NOT the shape: a draft carries creation
totals, so replaying one would re-grant every level-1 proficiency. See
rpg-project `ideas/characters/advancement/design.md`, and toolkit#1764.

## Spell slot state and casting direction

Character data carries spell slots only through recoverable `Resources`.
`resources.SpellSlotLevel1` is the sole mutable level-1 authority; level-1 Bard
finalization seeds two uses from the factual class progression table. Generic
resource spending persists the debit, and normal long-rest recovery restores
the pool. There is no legacy `SpellSlots` field, compatibility reader, or dual
writer. Class spell-slot tables remain source progression data, not a second
runtime pool.

## File organization

```
rulebooks/dnd5e/character/
├── draft.go               - Draft type and choice recording (MAIN FILE)
├── draft_data.go          - Draft serialization
├── character.go           - Finalized Character type
├── data.go                - Character serialization (Data struct, LoadFromData)
├── choices/
│   ├── choice_data.go     - ChoiceData structure
│   ├── choice_ids.go      - Constants for choice IDs
│   └── equipment_choice.go - Equipment choice helpers
├── shared/
│   ├── types.go           - Common types (ChoiceCategory, ChoiceSource)
│   └── ability_scores.go  - Ability score calculations
└── *_test.go              - Tests (testify suite)
```

## Integration

- **Core:** Character implements `core.Entity` (GetID, GetType)
- **Proficiencies:** `GetProficiencies()` returns a `proficiency.Set` compiled
  from race/class/background during Finalize
- **Equipment:** choices reference item IDs; inventory compilation resolves
  IDs to items while preserving declared quantities
- **Effects:** race/class/background features grant effects

## Pointers

- Mechanics, worked examples, testing, known gaps:
  `docs/architecture/components/rulebook-dnd5e-character.md`
- Test commands and the testify suite pattern: `docs/how-to/run-tests.md`
- Advancement design (multiclassing, XP, ASI/feats, slot progression):
  rpg-project `ideas/characters/advancement/design.md`
- Current health and rough edges: `docs/status.md`