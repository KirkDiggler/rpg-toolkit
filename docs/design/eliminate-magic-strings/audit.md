# Magic Strings Audit

This document catalogs all `Source string` fields and magic string patterns that should be converted to `core.Ref`.

## Summary

| Category | Count | Priority |
|----------|-------|----------|
| Condition Source fields | 4 | High |
| Event ConditionRef field | 1 | High |
| Event WeaponRef field | 1 | Medium |
| Grant Ref fields | 3 | Medium |
| Factory Ref fields | 2 | Medium |
| Effect Source field | 1 | Low |
| HealingReceivedEvent Source | 1 | Low |

## High Priority: Condition Source Fields

These fields explicitly contain `module:type:id` patterns and should be `core.Ref`.

### 1. RagingCondition.Source
**File:** `conditions/raging.go:35`
```go
Source string // Ref string in "module:type:value" format (e.g., "dnd5e:features:rage")
```
**Usage:** Tracks what feature activated the rage (e.g., "dnd5e:features:rage")
**Action:** Change to `Source core.Ref`

### 2. RagingData.Source
**File:** `conditions/raging.go:23`
```go
Source string `json:"source"` // Ref string in "module:type:value" format
```
**Usage:** JSON serialization of RagingCondition.Source
**Action:** Change to `Source core.Ref`

### 3. UnarmoredDefenseCondition.Source
**File:** `conditions/unarmored_defense.go:44,55`
```go
Source string // Ref string in "module:type:value" format (e.g., "dnd5e:classes:barbarian")
```
**Usage:** Tracks which class granted the unarmored defense
**Action:** Change to `Source core.Ref`

### 4. UnarmoredDefenseData.Source
**File:** `conditions/unarmored_defense.go:34`
```go
Source string `json:"source"`
```
**Usage:** JSON serialization
**Action:** Change to `Source core.Ref`

### 5. ActiveCondition.Source
**File:** `conditions/types.go:45`
```go
Source string `json:"source,omitempty"`
```
**Usage:** General condition tracking
**Action:** Change to `Source core.Ref`

## High Priority: Event Fields

### 6. ConditionRemovedEvent.ConditionRef
**File:** `events/events.go:137`
```go
ConditionRef string
```
**Usage:** Identifies which condition was removed (e.g., "dnd5e:conditions:raging")
**Action:** Change to `ConditionRef core.Ref`

## Medium Priority: Grant and Factory Refs

### 7. ConditionGrant.Ref
**File:** `classes/grant.go:46`
```go
Ref string `json:"ref"`
```
**Usage:** e.g., "dnd5e:conditions:unarmored_defense"
**Action:** Change to `Ref core.Ref`

### 8. FeatureGrant.Ref
**File:** `classes/grant.go:56`
```go
Ref string `json:"ref"`
```
**Usage:** e.g., "dnd5e:features:rage"
**Action:** Change to `Ref core.Ref`

### 9. SpellGrant.Ref
**File:** `classes/grant.go:65`
```go
Ref string `json:"ref"`
```
**Usage:** e.g., "dnd5e:spells:bless"
**Action:** Change to `Ref core.Ref`

### 10. ConditionFactoryInput.Ref
**File:** `conditions/factory.go:18`
```go
Ref string
```
**Action:** Change to `Ref core.Ref`

### 11. ConditionFactoryInput.SourceRef
**File:** `conditions/factory.go:26`
```go
SourceRef string
```
**Action:** Change to `SourceRef core.Ref`

### 12. FeatureFactoryInput.Ref
**File:** `features/factory.go:16`
```go
Ref string
```
**Action:** Change to `Ref core.Ref`

## Medium Priority: Attack Event

### 13. AttackEvent.WeaponRef
**File:** `events/events.go:145`
```go
WeaponRef string // Reference to the weapon used
```
**Usage:** References the weapon (currently just weapon ID, not full ref)
**Action:** Consider changing to `WeaponRef core.Ref` for consistency

## Low Priority: Other Fields

### 14. Effect.Source
**File:** `effects/types.go:39`
```go
Source string `json:"source"` // Who/what created this effect
```
**Usage:** General effect tracking - may be entity ID or ref
**Action:** Evaluate if this should be `core.Ref`

### 15. HealingReceivedEvent.Source
**File:** `events/events.go:123`
```go
Source string // What caused this healing (e.g., "second_wind")
```
**Usage:** Feature name that caused healing
**Action:** Consider changing to `Source core.Ref`

## Magic String Literals Found

These are hardcoded ref strings that should use helper functions:

| Location | String | Should Use |
|----------|--------|------------|
| `raging.go:209` | `"dnd5e:conditions:raging"` | `dnd5e.ConditionRef(conditions.RagingID)` |
| `draft.go:849` | `"dnd5e:classes:" + d.class` | `dnd5e.ClassRef(d.class)` |
| `factory.go:139` | `"dnd5e:features:rage"` | `dnd5e.FeatureRef(features.RageID)` |
| `grant.go:108` | `"dnd5e:features:second_wind"` | `dnd5e.FeatureRef(features.SecondWindID)` |
| `grant.go:136` | `"dnd5e:features:rage"` | `dnd5e.FeatureRef(features.RageID)` |
| `grant.go:143` | `"dnd5e:conditions:unarmored_defense"` | `dnd5e.ConditionRef(conditions.UnarmoredDefenseID)` |
| `grant.go:169` | `"dnd5e:conditions:unarmored_defense"` | `dnd5e.ConditionRef(conditions.UnarmoredDefenseID)` |

## Recommended Implementation Order

1. **Phase 3a:** Convert condition Source fields to core.Ref
   - RagingCondition, RagingData
   - UnarmoredDefenseCondition, UnarmoredDefenseData
   - ActiveCondition

2. **Phase 3b:** Convert event fields
   - ConditionRemovedEvent.ConditionRef

3. **Phase 3c:** Convert grant and factory refs
   - ConditionGrant, FeatureGrant, SpellGrant
   - ConditionFactoryInput, FeatureFactoryInput

4. **Phase 3d:** Create ref helper functions in dnd5e package
   - `dnd5e.ConditionRef(id string) core.Ref`
   - `dnd5e.FeatureRef(id string) core.Ref`
   - `dnd5e.ClassRef(id string) core.Ref`
   - `dnd5e.SpellRef(id string) core.Ref`

5. **Phase 3e:** Replace magic string literals with helper functions

## Notes

- `core.Ref` has `Module`, `Type`, and `ID` fields
- JSON marshaling of `core.Ref` should produce `"module:type:id"` format for backward compatibility
- The `core.Ref.String()` method already produces this format
- Need to verify `core.Ref` JSON unmarshaling can parse `"module:type:id"` strings
