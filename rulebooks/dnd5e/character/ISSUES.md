# Implementation Issues

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

### Issue 2: Simplify ChoiceData Structure
**Title:** Replace complex ChoiceData with ref-based version

**Description:**
Replace the current ChoiceData with 12+ typed fields with a simple ref-based structure.

**Tasks:**
- [ ] Create new `ChoiceDataV2` struct with just Ref, Source, Selected fields
- [ ] Add conversion functions between old and new formats
- [ ] Update character package to support both versions
- [ ] Add comprehensive tests

**Acceptance Criteria:**
- New ChoiceData uses only `*core.Ref` fields
- Can convert between old and new formats
- Backward compatibility maintained

---

### Issue 3: Create Self-Contained Character Data Structure
**Title:** Design self-contained character data structure

**Description:**
Create a character Data structure that contains everything needed to play without external dependencies.

**Tasks:**
- [ ] Define new Data structure with all compiled attributes
- [ ] Use refs for skills, languages, proficiencies
- [ ] Keep features and conditions as json.RawMessage
- [ ] Add race/class/background refs for reference only
- [ ] Document what gets compiled vs referenced

**Acceptance Criteria:**
- Data structure has no external dependencies
- All attributes needed for play are included
- Can be serialized/deserialized as JSON

---

## Phase 2: Compilation

### Issue 4: Implement Character Compiler
**Title:** Create character compiler to transform choices into data

**Description:**
Implement the compiler that takes choices and API data and produces self-contained character data.

**Tasks:**
- [ ] Define Compiler interface
- [ ] Create CompileInput structure  
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

### Issue 5: Update LoadCharacterFromData
**Title:** Remove external dependencies from LoadCharacterFromData

**Description:**
Update the character loading function to work with self-contained data only.

**Tasks:**
- [ ] Remove race/class/background parameters
- [ ] Load everything from the data structure
- [ ] Update all call sites
- [ ] Update tests

**Acceptance Criteria:**
- `LoadCharacterFromData(data)` takes only data parameter
- Character loads successfully without external deps
- All existing tests pass

---

## Phase 3: Migration

### Issue 6: Deprecate Draft/Builder System
**Title:** Mark Draft and Builder as deprecated

**Description:**
Begin migration away from the complex Draft/Builder pattern.

**Tasks:**
- [ ] Mark Draft type as deprecated
- [ ] Mark Builder type as deprecated  
- [ ] Document migration path in code
- [ ] Create examples using new approach

**Acceptance Criteria:**
- Deprecation notices in place
- Migration documentation available
- New approach documented with examples

---

### Issue 7: Create Migration Guide
**Title:** Document migration from old to new character system

**Description:**
Create comprehensive documentation for migrating from Draft/Builder to the new ref-based system.

**Tasks:**
- [ ] Document old vs new approaches
- [ ] Create migration examples
- [ ] Show game server integration patterns
- [ ] Document breaking changes

**Acceptance Criteria:**
- Clear migration path documented
- Examples for common use cases
- Game server integration patterns shown

---

## Phase 4: Cleanup

### Issue 8: Remove Duplicate ChoiceData Types
**Title:** Consolidate duplicate ChoiceData definitions

**Description:**
Multiple packages define their own ChoiceData. Consolidate to single definition.

**Tasks:**
- [ ] Identify all ChoiceData definitions
- [ ] Consolidate to character package
- [ ] Update all references
- [ ] Remove duplicates

**Acceptance Criteria:**
- Single ChoiceData type in character package
- All packages use the same type
- No duplicate definitions

---

### Issue 9: Integration Tests
**Title:** Create comprehensive integration tests

**Description:**
Test the complete flow from choices to playable character.

**Tasks:**
- [ ] Test character creation flow
- [ ] Test character loading
- [ ] Test feature activation
- [ ] Test condition application
- [ ] Test persistence round-trip

**Acceptance Criteria:**
- Full character lifecycle tested
- Edge cases covered
- Performance benchmarks included

---

## Priority Order

1. **Foundation First** (Issues 1-3) - Get the structure right
2. **Compilation** (Issues 4-5) - Make it work
3. **Migration** (Issues 6-7) - Help users transition
4. **Cleanup** (Issues 8-9) - Polish and optimize

## Estimated Effort

- Phase 1: 2-3 days
- Phase 2: 3-4 days  
- Phase 3: 1-2 days
- Phase 4: 2-3 days

**Total: ~2 weeks**