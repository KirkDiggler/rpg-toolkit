# Implementation Issues - Clean Build

## Phase 1: Foundation

### Issue 1: Create Domain Ref Packages
**Title:** Create domain-specific ref packages for D&D 5e

**Description:**
Create ref builder functions and common constants organized by domain.

**Tasks:**
- [ ] Create `races/refs.go` with `Ref()` builder and common race constants
- [ ] Create `classes/refs.go` with `Ref()` builder and common class constants  
- [ ] Create `skills/refs.go` with `Ref()` builder and all skill constants
- [ ] Create `choices/refs.go` with `Ref()` builder and choice type constants
- [ ] Create `features/refs.go` with `Ref()` builder
- [ ] Create `conditions/refs.go` with `Ref()` builder
- [ ] Create `system/refs.go` with system constants (Creation, LevelUp, etc.)
- [ ] Add tests for each package

**Acceptance Criteria:**
- Can create refs using domain packages: `races.Ref("dwarf")`
- Common refs available as constants: `races.Human`, `skills.Athletics`
- All refs properly validated with core.Ref

---

### Issue 2: Define Character Data Structures
**Title:** Create self-contained character data structures

**Description:**
Define the core data structures for characters using refs.

**Tasks:**
- [ ] Create simple `ChoiceData` struct with Ref, Source, Selected fields
- [ ] Create self-contained `CharacterData` struct
- [ ] Define `CompileInput` for character compilation
- [ ] Use refs for skills, languages, proficiencies
- [ ] Document the data model

**Acceptance Criteria:**
- Data structures use `*core.Ref` throughout
- CharacterData is self-contained (no external deps)
- Can be serialized/deserialized as JSON

---

## Phase 2: Core Implementation

### Issue 3: Implement Character Compiler
**Title:** Create character compiler to transform choices into data

**Description:**
Implement the compiler that takes choices and API data and produces self-contained character data.

**Tasks:**
- [ ] Create Compiler interface
- [ ] Implement compilation logic:
  - [ ] Apply racial attributes (speed, size)
  - [ ] Calculate HP from class + CON
  - [ ] Merge skills from all sources
  - [ ] Compile languages and proficiencies
  - [ ] Extract features from class/level
- [ ] Add validation
- [ ] Create comprehensive tests

**Acceptance Criteria:**
- Compiler produces self-contained character data
- All choices are properly applied
- Validation catches invalid combinations

---

### Issue 4: Implement Character Loading
**Title:** Create LoadCharacterFromData function

**Description:**
Implement loading a playable character from self-contained data.

**Tasks:**
- [ ] Create `LoadCharacterFromData(data)` function
- [ ] Build Character struct from data
- [ ] Initialize all game mechanics
- [ ] Connect to event bus for conditions/features
- [ ] Add tests

**Acceptance Criteria:**
- Character loads from data alone (no external deps)
- All game mechanics work (Attack, TakeDamage, etc.)
- Features and conditions properly initialized

---

### Issue 5: Implement Character Methods
**Title:** Implement core character gameplay methods

**Description:**
Implement the Character interface methods for actual gameplay.

**Tasks:**
- [ ] Implement combat methods (Attack, TakeDamage)
- [ ] Implement feature activation
- [ ] Implement condition checking
- [ ] Implement ToData() for persistence
- [ ] Add comprehensive tests

**Acceptance Criteria:**
- All character methods work correctly
- Proper event publishing for conditions
- Round-trip persistence works

---

## Phase 3: Features and Conditions

### Issue 6: Update Features to Use Refs
**Title:** Update feature system to use refs consistently

**Description:**
Ensure features use refs and work with the new character system.

**Tasks:**
- [ ] Update feature loading to use refs
- [ ] Ensure features store refs in their data
- [ ] Update rage, second wind, etc.
- [ ] Test feature activation with new system

**Acceptance Criteria:**
- Features use refs throughout
- Features activate properly
- Conditions applied via events

---

### Issue 7: Update Conditions to Use Refs
**Title:** Ensure conditions use refs consistently

**Description:**
Update condition system to use refs properly.

**Tasks:**
- [ ] Update condition loader to use refs
- [ ] Ensure conditions store character refs
- [ ] Test rage condition with new system
- [ ] Verify event subscriptions work

**Acceptance Criteria:**
- Conditions use refs for identification
- Conditions properly subscribe to events
- Conditions auto-terminate correctly

---

## Phase 4: Integration

### Issue 8: Create Choice Validator
**Title:** Implement choice validation helper

**Description:**
Create validator to help game servers validate player choices.

**Tasks:**
- [ ] Create Validator interface
- [ ] Implement ValidateChoice method
- [ ] Implement GetAvailableOptions method
- [ ] Add validation rules for each choice type
- [ ] Create tests

**Acceptance Criteria:**
- Can validate if a choice is allowed
- Can get available options for a choice type
- Handles prerequisites and restrictions

---

### Issue 9: Integration Tests and Examples
**Title:** Create comprehensive integration tests and examples

**Description:**
Test the complete flow and provide examples for game server integration.

**Tasks:**
- [ ] Create end-to-end character creation test
- [ ] Create gameplay integration test
- [ ] Create example game server wrapper
- [ ] Document common patterns
- [ ] Add performance benchmarks

**Acceptance Criteria:**
- Full character lifecycle tested
- Clear examples for game server integration
- Performance metrics documented

---

## Priority Order

1. **Foundation** (Issues 1-2) - Get the structure right
2. **Core** (Issues 3-5) - Make it work
3. **Features** (Issues 6-7) - Integrate existing systems
4. **Polish** (Issues 8-9) - Helpers and examples

## Notes

- No migration needed - building fresh
- No backward compatibility concerns
- Focus on clean, simple design
- Game server owns the flow