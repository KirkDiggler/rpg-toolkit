# Character Package - Detailed File Analysis

## Overview
Analyzing all 30 files in the character package to determine what's needed for our new Draft API architecture vs. what's legacy code that can be removed.

---

## CORE FILES (KEEP - Essential)

### character.go
**Purpose**: Core Character domain object for gameplay  
**Status**: ✅ KEEP - Essential  
**Why**: Main entity representing a playable character. Has methods like Attack(), GetAbilityScore(), etc.  
**Dependencies**: Used by everything, depends on shared types  
**Usage**: Referenced throughout codebase  

### draft.go  
**Purpose**: New Draft API with SetName/SetRace/SetClass methods using internal data  
**Status**: ✅ KEEP - New architecture foundation  
**Why**: Our new clean API that replaces external data dependencies  
**Dependencies**: Uses races.GetData(), classes.GetData() internal lookups  
**Usage**: Primary API for character creation  

### draft_inputs.go
**Purpose**: Strongly typed input structs for Draft API  
**Status**: ✅ KEEP - Essential for new API  
**Why**: Type safety for SetRaceInput, SetClassInput, etc.  
**Dependencies**: None - just type definitions  
**Usage**: Used by all Draft.Set* methods  

---

## TEST FILES (EVALUATE CASE BY CASE)

### character_test.go
**Purpose**: Tests for Character domain object  
**Status**: ✅ KEEP - Tests core functionality  
**Analysis**: Tests Character methods like Attack(), ability score calculations  
**Recommendation**: Keep, may need updates for new API  

### draft_test.go  
**Purpose**: Tests for new Draft API  
**Status**: ✅ KEEP - Tests new architecture  
**Analysis**: Tests our SetName/SetRace/SetClass methods with internal data  
**Recommendation**: Essential for validating new approach  

### attack_test.go
**Purpose**: Tests character attack mechanics  
**Status**: ❓ EVALUATE - May belong elsewhere  
**Analysis**: Tests weapon attacks, damage calculation  
**Question**: Should this be in a combat package instead of character?  

### character_context_test.go  
**Purpose**: Tests character context functionality  
**Status**: ❓ EVALUATE - Check what context this refers to  
**Analysis**: Need to examine what "context" means here  

### equipment_test.go
**Purpose**: Tests equipment system  
**Status**: ❓ DEPENDS on equipment.go decision  

### feature_test.go
**Purpose**: Tests character features  
**Status**: ❓ EVALUATE - What features?  

### resources_test.go  
**Purpose**: Tests character resources (spell slots, class abilities)  
**Status**: ❓ DEPENDS on resources.go decision  

### example_api_test.go
**Purpose**: Examples of API usage  
**Status**: 🔍 EXAMINE - Which API? Old or new?  
**Analysis**: If it shows old builder pattern, delete. If it shows new Draft API, keep.  

### example_context_test.go
**Purpose**: Context examples  
**Status**: ❓ EVALUATE - Need to see what context  

### test_helpers.go
**Purpose**: Helper functions for tests  
**Status**: ❓ EVALUATE - Which tests do these help?  

---

## CHOICES VALIDATION PACKAGE (KEEP - Active System)

### choices/constants.go  
**Purpose**: Constants for choice validation system  
**Status**: ✅ KEEP - Used by validation  
**Analysis**: Defines FieldSkills, SourceClass, etc. Used 64 times in draft.go  

### choices/requirements.go
**Purpose**: Defines what choices are required for each class/race  
**Status**: ✅ KEEP - Core validation logic  
**Analysis**: Knows "Fighter needs 2 skills from this list", etc.  

### choices/submission_types.go  
**Purpose**: Types for submitting choices for validation  
**Status**: ✅ KEEP - Used by Draft.ValidateChoices()  

### choices/types.go
**Purpose**: Core types for choice system  
**Status**: ✅ KEEP - Foundation types  

### choices/validation_constants.go
**Purpose**: Error codes and validation constants  
**Status**: ✅ KEEP - Used for proper error reporting  

### choices/validation_types.go  
**Purpose**: ValidationResult and related types  
**Status**: ✅ KEEP - Return type of ValidateChoices()  

### choices/validator.go
**Purpose**: Main validation engine  
**Status**: ✅ KEEP - Core validation logic  
**Analysis**: This is where the complex D&D rule validation happens  

### choices/example_validation_test.go
**Purpose**: Example of how validation works  
**Status**: ✅ KEEP - Documents validation system  

### choices/nature_domain_test.go  
**Purpose**: Tests specific subclass validation  
**Status**: ✅ KEEP - Tests domain-specific rules  

### choices/requirements_test.go
**Purpose**: Tests requirement definitions  
**Status**: ✅ KEEP - Validates requirement logic  

### choices/validator_knowledge_test.go
**Purpose**: Tests validator's knowledge of game rules  
**Status**: ✅ KEEP - Critical for rule accuracy  

### choices/validator_test.go  
**Purpose**: Core validator tests  
**Status**: ✅ KEEP - Essential validation tests  

---

## LEGACY/CONVERSION FILES (DELETE)

### builder.go
**Purpose**: Essential validation methods and types for character creation  
**Status**: ✅ KEEP - Contains methods used by new Draft API  
**Analysis**: Contains MissingSubclass(), MissingRequiredSubrace(), ClassChoice/RaceChoice validation, and Proficiencies struct. The new Draft API actively uses these methods.  
**Why Keep**: Draft API calls rc.MissingRequiredSubrace() and cc.MissingSubclass() for validation logic. Despite confusing name, this is not legacy builder pattern but essential domain logic.  

### choice_converter.go  
**Purpose**: Converts between old and new choice formats  
**Status**: ❌ DELETE - Migration code no longer needed  
**Analysis**: Has ToolProficiencySelection errors, converts old ChoiceData  
**Why Delete**: New Draft API creates ChoiceData directly, no conversion needed  

### choice_data.go
**Purpose**: Old ChoiceData sum type with all possible choice fields  
**Status**: ❓ CONFLICTED - Used by new API but problematic  
**Analysis**: New Draft API creates these, but has type issues (ToolProficiencySelection vs ToolSelection)  
**Decision Needed**: Fix the type issues or redesign?  

### choices.go  
**Purpose**: Old Choice interface types (NameChoice, SkillChoice, etc.)  
**Status**: ❓ CONFLICTED - May be used by conversion  
**Analysis**: If choice_converter.go goes, this probably can too  

---

## SPECIALIZED FUNCTIONALITY (EVALUATE)

### equipment.go
**Purpose**: Equipment management system  
**Status**: ❓ EVALUATE - Is this core functionality?  
**Analysis**: Character equipment is important, but is this the right implementation?  
**Question**: Does this integrate with new Draft API or is it separate?  

### resources.go  
**Purpose**: Resource management (spell slots, class features)  
**Status**: ❓ EVALUATE - Core functionality?  
**Analysis**: Characters need resource tracking, but does this fit new architecture?  

### starting_class.go
**Purpose**: API for listing available starting classes with their requirements  
**Status**: ✅ KEEP - Useful API functionality  
**Analysis**: Provides ListStartingClasses() that shows what choices are required for each class. Works with choices validation system.  
**Why Keep**: Useful for UI to show "Cleric requires subclass at level 1" etc.  

---

## UPDATED ANALYSIS

### equipment.go  
**Purpose**: Equipment pack definitions (Burglar's Pack, etc.)  
**Status**: ✅ KEEP - Core game data  
**Analysis**: Standard D&D equipment packs as static data. Not legacy, just data.  

### example_api_test.go
**Purpose**: Examples showing NEW Draft API usage  
**Status**: ✅ KEEP - Shows new SetRace/SetClass API  
**Analysis**: Documents how to use our new typed constants API properly  

---

## PRELIMINARY RECOMMENDATIONS

### Definite KEEP (20 files):
**Core API:**
- character.go, draft.go, draft_inputs.go, builder.go
- character_test.go, draft_test.go
- starting_class.go, equipment.go
- example_api_test.go

**Choices validation system (10 files):**
- All files in choices/ package

**Data/helpers:**
- choice_data.go (after fixing ToolSelection issue)

### Definite DELETE (1 file):
- choice_converter.go (migration code with compilation errors)

### Still Need Investigation (9 files):
- choices.go - Still needed by anything?
- resources.go - Core functionality or separate concern?
- attack_test.go, character_context_test.go, equipment_test.go, feature_test.go, resources_test.go, example_context_test.go, test_helpers.go - Which systems do they test?

### Expected Final Count: ~28-29 files
Much more conservative cleanup than initially thought. Most files are actually essential infrastructure, not legacy code.
The main issue was misunderstanding file purposes due to misleading names (builder.go) and not examining contents thoroughly.

---

## NEXT STEPS

1. Examine the "Need Investigation" files in detail
2. Fix type issues in choice_data.go (ToolSelection vs ToolProficiencySelection)  
3. Determine if equipment.go and resources.go fit new architecture
4. Clean up test files that test deleted functionality
5. Update any remaining external data dependencies to use internal data

## RISK ASSESSMENT: MEDIUM
The choices package is more complex and essential than initially thought. Need careful analysis of interconnections before deleting anything else.