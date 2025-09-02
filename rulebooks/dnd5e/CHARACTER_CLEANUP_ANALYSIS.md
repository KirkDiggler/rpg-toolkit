# Character Package Cleanup Analysis

## Summary
The character package has grown to 30 files during development iterations. With our new clean Draft API using internal data, many files represent legacy patterns that are no longer needed.

## Current Architecture (NEW - Keep)
- **draft.go** - New clean Draft API with SetName/SetRace/SetClass/etc methods
- **draft_inputs.go** - Strongly typed input structs for new API  
- **draft_test.go** - Tests for new Draft API
- **character.go** - Core Character domain object
- **character_test.go** - Character tests

## Files to DELETE (Legacy/Deprecated)

### 1. Migration/Conversion Code
- **choice_converter.go** - Converts between old and new choice formats. Not needed with clean API.
- **choice_data.go** - Old ChoiceData sum type. New API creates these directly.
- **choices.go** - Old Choice interface types. New API bypasses this layer.

### 2. Builder Pattern (Replaced)
- **builder.go** - Old builder pattern. Replaced by Draft API.

### 3. Validation System (choices/ subdirectory)
**Problem**: Complex validation system that duplicates rulebook knowledge
- choices/constants.go
- choices/requirements.go  
- choices/submission_types.go
- choices/types.go
- choices/validation_constants.go
- choices/validation_types.go
- choices/validator.go
- All choices/*_test.go files

**Replacement**: Simple ValidationResult in draft.go using internal data

### 4. Legacy Equipment System  
- **equipment.go** - Old equipment system
- **equipment_test.go**
- **resources.go** - Old resource management
- **resources_test.go**

### 5. Example/Test Files
- **example_api_test.go** - Examples of old API
- **example_context_test.go** - Old context examples
- **attack_test.go** - Attack system tests (may belong elsewhere)
- **character_context_test.go** - Old context tests
- **feature_test.go** - Feature tests
- **test_helpers.go** - Helpers for deleted systems

### 6. Specialized Files (Evaluate)
- **starting_class.go** - May be legacy class loading

## Recommended File Structure (After Cleanup)

```
character/
├── character.go          # Core Character domain object
├── character_test.go     # Character tests  
├── draft.go             # New Draft API with internal data
├── draft_inputs.go      # Typed inputs for Draft API
└── draft_test.go        # Tests for new Draft API
```

## Migration Impact

### What Breaks:
- Any code using the old builder pattern
- Code depending on the choices validation system
- Code using choice_converter functions

### What Still Works:
- Character domain object (unchanged interface)
- Internal data system (races.GetData, classes.GetData)
- New Draft API

## Questions to Resolve:

1. **choices/ directory**: Delete entirely? It's a complex validation system that duplicates what the rulebook data already provides.

2. **attack_test.go**: Does this belong in character package or elsewhere?

3. **starting_class.go**: Legacy or still needed?

4. **Validation approach**: Keep simple ValidationResult in draft.go or restore some validation?

## Benefits of Cleanup:
- **30 files → 5 files** (83% reduction)
- Single source of truth (internal data)
- Eliminates complex migration/conversion layers
- Clear separation: Draft for creation, Character for gameplay
- Type safety throughout (no more `any` or string conversions)

## Risk Assessment: LOW
The new Draft API is already implemented and tested. The files marked for deletion represent older patterns that are no longer used by the new architecture.