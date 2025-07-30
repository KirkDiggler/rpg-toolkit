# Normalize Choice Selections

## Problem
The `Selection any` field in ChoiceData requires type assertions throughout the codebase, making it fragile and complex. Current types include:
- Single strings (fighting style, name)
- String arrays (equipment, spells, cantrips)
- Typed arrays ([]constants.Skill, []constants.Language)
- Complex structs (RaceChoice, ClassChoice, AbilityScores)

## Solution: Sum Type Pattern
Use a sum type pattern similar to protobuf oneofs, where each Category has its own typed field:

```go
type ChoiceData struct {
    Category shared.ChoiceCategory `json:"category"`
    Source   shared.ChoiceSource   `json:"source"`
    ChoiceID string                `json:"choice_id"`
    
    // Selection fields - only one populated based on Category
    NameSelection          *string                `json:"name,omitempty"`
    SkillSelection         []constants.Skill      `json:"skills,omitempty"`
    LanguageSelection      []constants.Language   `json:"languages,omitempty"`
    AbilityScoreSelection  *shared.AbilityScores  `json:"ability_scores,omitempty"`
    FightingStyleSelection *string                `json:"fighting_style,omitempty"`
    EquipmentSelection     []string               `json:"equipment,omitempty"`
    RaceSelection          *RaceChoice            `json:"race,omitempty"`
    ClassSelection         *ClassChoice           `json:"class,omitempty"`
    BackgroundSelection    *constants.Background  `json:"background,omitempty"`
    SpellSelection         []string               `json:"spells,omitempty"`
    CantripSelection       []string               `json:"cantrips,omitempty"`
}
```

## Benefits
1. **No runtime type assertions** - compile-time type safety
2. **Clear which field to use** - Category determines the field
3. **Clean JSON serialization** - omitempty keeps it compact
4. **Natural proto mapping** - maps directly to protobuf oneofs
5. **Self-documenting** - field names indicate the type

## Migration Plan
1. Update ChoiceData struct to sum type pattern
2. Update all creation points to use specific fields
3. Update all reading points to check specific fields
4. Update tests
5. Run pre-commit and create PR