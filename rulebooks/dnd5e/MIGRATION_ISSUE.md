# Migration to New Draft API - Local Tracking

## Overview
Migrate rpg-api from Builder pattern to new Draft API with typed constants and immediate validation feedback.

## Implementation Plan

## Phase 1: Toolkit Preparation ✓
- [x] Create Draft API methods (SetClass, SetRace, SetBackground, SetAbilityScores)
- [x] Add Equipment interface in shared package
- [x] Create spells package with Spell type
- [x] Add typed constants (FightingStyle, DraconicAncestry, etc.)
- [x] Add comprehensive tests including duplicate skill warnings
- [ ] Run linter and fix any issues
- [ ] Create toolkit PR

## Phase 2: API Orchestrator Implementation
- [ ] Add replace directive to rpg-api go.mod:
  ```
  replace github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e => ../rpg-toolkit/rulebooks/dnd5e
  ```

### Orchestrator Changes
- [ ] Update orchestrator service methods to use new Draft API:
  - `UpdateDraftClass` to use `draft.SetClass(input)`
  - `UpdateDraftRace` to use `draft.SetRace(input)`
  - `UpdateDraftBackground` to use `draft.SetBackground(input)`
  - `UpdateDraftAbilityScores` to use `draft.SetAbilityScores(input)`
  - `FinalizeDraft` to use `draft.ToCharacter()` (no params!)

- [ ] Remove Builder pattern:
  - Remove all `character.NewBuilder()` calls
  - Remove all `builder.Build()` calls
  - Remove passing race/class/background data around

### Comprehensive Orchestrator Tests
- [ ] Unit tests for each Set* method
- [ ] Integration test: Full character creation workflow
- [ ] Edge cases:
  - Half-Orc Soldier (duplicate skill warnings)
  - Cleric without subclass (error)
  - Elf without subrace (error)
  - Fighter with all choices (skills, fighting style, equipment)
  - Wizard with spells and cantrips
- [ ] Validation feedback tests for each step
- [ ] Test that ToCharacter() works without parameters

## Phase 3: Proto Updates (after orchestrator works)
- [ ] Update proto definitions:
  - Add ValidationResult message type
  - Update SetClass/SetRace/etc requests to include choices
  - Add fields for typed constants (fighting styles, spells, etc.)
- [ ] Generate proto code
- [ ] Create rpg-protos PR and merge

## Phase 4: Handler Implementation
- [ ] Update rpg-api dependency for new protos
- [ ] Create conversion functions:
  ```go
  func protoToSetClassInput(req *pb.SetClassRequest) *character.SetClassInput
  func validationResultToProto(result *choices.ValidationResult) *pb.ValidationResult
  ```
- [ ] Update GraphQL/gRPC handlers to use orchestrator methods
- [ ] Handler tests with proto conversions

## Phase 5: Final Integration
- [ ] Merge toolkit PR
- [ ] Remove replace directive from rpg-api
- [ ] Update rpg-api dependency: `go get -u github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e@latest`
- [ ] Final test run
- [ ] Create rpg-api PR
- [ ] Deploy rpg-api

## Out of Scope (Separate Work)
- [ ] Update rpg-dnd5e-web (React Discord activity) to use new validation feedback
- [ ] Update UI to show warnings/errors from ValidationResult
- [ ] Add UI for new typed choices (fighting styles, spells, etc.)

## Benefits After Migration
✅ Immediate validation feedback on every update
✅ No need to pass race/class/background data to ToCharacter
✅ Type safety with constants (`spells.Spell`, `choices.FightingStyle`, etc.)
✅ Cleaner API matching actual rpg-api workflow
✅ Better error messages for Discord users

## Key API Changes

### Old Way
```go
draft.SetClass(classID, subclassID)
draft.SetRace(raceID, subraceID)
// No validation feedback until later
character, err := draft.ToCharacter(raceData, classData, backgroundData)
```

### New Way
```go
result, err := draft.SetClass(&character.SetClassInput{
    ClassID: classes.Fighter,
    SubclassID: classes.Champion,
    Choices: character.ClassChoices{
        Skills: []skills.Skill{skills.Athletics, skills.Survival},
        FightingStyle: &choices.FightingStyleDefense,
        Equipment: []character.EquipmentSelection{...},
        // All typed constants!
    },
})
// Immediate validation feedback in result!

character, err := draft.ToCharacter() // No parameters needed!
```

## Testing Checklist
- [ ] Fighter with Defense fighting style
- [ ] Wizard with cantrips and spells
- [ ] Dragonborn with ancestry selection
- [ ] Half-Orc Soldier (duplicate skill warning)
- [ ] Cleric without subclass (validation error)
- [ ] Elf without subrace (validation error)
- [ ] Standard array validation
- [ ] Point-buy validation

## Notes
- Equipment implementation will need actual Equipment types implementing the interface
- **NO v2 APIs** - clean replacement of old methods
- **NO migration code** - just replace with new implementation
- Existing drafts in DB will work because DraftData structure unchanged