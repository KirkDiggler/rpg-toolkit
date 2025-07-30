# Language Choice Source Context

## Problem
The character builder's `SelectLanguages` method currently defaults to `SourceRace` for all language choices, but languages can come from multiple sources:
- Race (e.g., Human gets 1 extra language)
- Background (e.g., Sage gets 2 languages)
- Feats (e.g., Linguist feat)
- Class features (e.g., Ranger's Favored Enemy)

## Current Implementation
```go
// Language choices could come from race or background
// TODO: Builder should track which source is requesting the choice
b.draft.Choices = append(b.draft.Choices, ChoiceData{
    Category:  shared.ChoiceLanguages,
    Source:    shared.SourceRace, // Default to race, but this should be contextual
    ChoiceID:  "additional_languages",
    Selection: typedLanguages,
})
```

## Proposed Solution
The builder needs to accept a source parameter when selecting languages, or have separate methods for different sources:
- `SelectRaceLanguages(languages []string)`
- `SelectBackgroundLanguages(languages []string)`
- Or: `SelectLanguages(languages []string, source shared.ChoiceSource, choiceID string)`

This would ensure proper source attribution for language choices.