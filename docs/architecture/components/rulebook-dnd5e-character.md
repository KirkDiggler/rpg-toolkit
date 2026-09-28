---
name: rulebooks/dnd5e/character module
description: Character creation mechanics — two-phase draft/finalize, the three-identifier choice model, source clearing, equipment structure, compilation, validation, worked examples, and known gaps
updated: 2026-09-25
confidence: high — moved verbatim from rulebooks/dnd5e/character/CLAUDE.md (the injected file now carries only charter, laws, and pointers); code references re-verified against draft.go
---

# rulebooks/dnd5e/character — creation mechanics

Loaded on demand: this is the deep dive for sessions changing character
creation mechanics. The laws and acceptance gates that bind all character work
live in `rulebooks/dnd5e/character/CLAUDE.md` (the injected file).

## The two-phase creation pattern

```go
// Phase 1: Build draft with choices
draft := character.NewDraft(...)
draft.SetName(...)
draft.SetRace(...)
draft.SetClass(...)
draft.SetBackground(...)

// Phase 2: Finalize when complete
character, err := draft.Finalize()
```

Why this shape: incremental building; validation only when complete (Finalize);
"save and continue later" workflows; clear separation — Draft is mutable,
Character is immutable.

## The choice model — three identifiers

```go
type ChoiceData struct {
    Category shared.ChoiceCategory  // WHAT: ChoiceSkills, ChoiceEquipment, etc.
    Source   shared.ChoiceSource    // WHERE: SourceClass, SourceRace, SourceBackground
    ChoiceID ChoiceID               // WHICH: "fighter-armor", "barbarian-skills"

    // One selection field populated based on Category
    SkillSelection         []skills.Skill
    EquipmentSelection     []shared.SelectionID
    // ... etc
}
```

The three-level hierarchy:

1. **Source** — where did this choice come from? (class, race, background, player)
2. **Category** — what type of choice is this? (skills, equipment, languages)
3. **ChoiceID** — which specific choice requirement? (fighter-armor vs barbarian-weapons)

Why it matters: changing class clears ALL SourceClass choices; multiple
equipment choices per source are allowed (armor + weapons + pack); the same
category from different sources coexists (class skills + background skills).

## Source-level clearing and deduplication

Two levels, both in `draft.go`:

**Level 1: `clearChoicesBySource`** — removes ALL choices of a source.
Called at the start of SetClass/SetRace/SetBackground before recording new
choices.

**Level 2: `recordChoice`** — within a source, equipment removes only
matching ChoiceIDs (so multiple equipment choices coexist); other categories
replace on Category AND Source match.

```go
// The pattern in every setter:
d.clearChoicesBySource(shared.SourceClass)   // Step 1: clear old choices
d.recordChoice(skillChoice)                  // Step 2: record fresh
d.recordChoice(equipmentChoice1)
d.recordChoice(equipmentChoice2)
```

## Equipment choice structure

Two levels: a **Choice** ("fighter-armor") lists **Options**
("fighter-armor-a": chain-mail …, "fighter-armor-b": leather …). The player
picks one option and receives all items in it. Recording:

```go
d.recordChoice(choices.ChoiceData{
    Category:           shared.ChoiceEquipment,
    Source:             shared.SourceClass,
    ChoiceID:           "fighter-armor",           // The choice
    OptionID:           "fighter-armor-a",          // The selected option
    EquipmentSelection: [...items...],             // Items from that option
})
```

A class has multiple equipment choices (armor, weapons, pack), all
`SourceClass` and `ChoiceEquipment`, differentiated by ChoiceID.

## Compilation

`Finalize()` orchestrates compilation (`draft.go`): validate completeness →
compile base stats (race modifiers) → proficiencies (race, class, background)
→ inventory (fixed grants plus equipment choices, preserving declared
quantities) → cantrips/spells → features → build the Character entity.

Each compilation method (`compileAbilityScores`, `compileProficiencies`,
`compileInventory`, `compileSpells`, `compileFeatures`) is stateless — a pure
function of draft data.

## Validation

Two levels: setters validate their inputs (class exists, skill count matches);
Finalize validates completeness (progress complete, all required choices made).
Cross-validation (chosen skills actually in the class skill list) is a known
gap — see below.

## Worked examples

### Creating a character from scratch

```go
draft := character.NewDraft(character.NewDraftInput{PlayerID: "player-123"})
draft.SetName("Conan")
draft.SetAbilityScores(character.SetAbilityScoresInput{
    Scores: shared.AbilityScores{STR: 15, DEX: 14, ...},
})
draft.SetRace(character.SetRaceInput{
    Race: races.Human, Subrace: races.SubraceNone,
    Choices: choices.RaceChoices{
        Languages: []languages.Language{languages.Common, languages.Orcish},
    },
})
draft.SetClass(character.SetClassInput{
    Class: classes.Barbarian, Subclass: classes.SubclassNone,
    Choices: choices.ClassChoices{
        Skills:    []skills.Skill{skills.Athletics, skills.Intimidation},
        Equipment: []choices.EquipmentChoiceSelection{
            {ChoiceID: "barbarian-weapons-primary", OptionID: "barbarian-weapon-a"},
            {ChoiceID: "barbarian-weapons-secondary", OptionID: "barbarian-secondary-a"},
            {ChoiceID: "barbarian-pack", OptionID: "barbarian-pack-explorer"},
        },
    },
})
draft.SetBackground(character.SetBackgroundInput{
    Background: backgrounds.Folk,
    Choices:    choices.BackgroundChoices{
        Skills: []skills.Skill{skills.AnimalHandling, skills.Survival},
    },
})
draft.SetProgress(character.ProgressComplete)
character, err := draft.Finalize()
```

### Changing class

```go
draft.SetClass(character.SetClassInput{Class: classes.Barbarian, Choices: ...})
// draft.choices now contains ONLY Barbarian choices:
// clearChoicesBySource(SourceClass) removed every Fighter choice first.
```

## Testing

Use the testify suite pattern (shape in `docs/how-to/run-tests.md`). What to
cover:

- **Draft operations:** setters record choices correctly; deduplication within
  category/source; source-level clearing on changes (the #344 regression)
- **Compilation:** racial ability bonuses, merged proficiencies, inventory
  with preserved quantities, spells from class choices
- **Validation:** invalid class rejected, wrong skill count rejected,
  incomplete finalization rejected

Test data patterns: use the ChoiceID constants from
`character/choices/choice_ids.go`; test class-specific finalizations
separately (`barbarian_finalize_test.go`, `fighter_finalize_test.go`).

## Questions before adding features

1. Is this character creation or character management? (Management of a
   finalized character is not supported.)
2. Is this a choice or a calculation?
3. What is the source of this data — which Source clears it on changes?
4. Does it need validation, and at which level: setter input, Finalize
   completeness, or cross-validation?

## Known gaps

- **Cross-validation missing** — chosen skills are not verified against the
  class's actual skill list; equipment OptionIDs are not verified against the
  Choice's options. Both are validation work, not design gaps.
- **Character management not supported** — a new draft from a finalized
  character (modify-then-refinalize) is a named future capability, not built.
- **Not yet built:** multiclassing, XP, ability score improvements and feats,
  spell slot progression above the resource pool, levelling down. Each has a
  seam named in rpg-project `ideas/characters/advancement/design.md`.
- **Equipment item resolution** waits on the items module (#31): choices
  reference item IDs today.