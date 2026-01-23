# Class-Level Grants Architecture

*Date: 2025-11-30*
*Type: Design*

## The Problem

While implementing Monk Level 1 features, we noticed a pattern emerging in `draft.go`:

```go
// Scattered if-statements for class-specific logic
if d.class == classes.Barbarian {
    // create rage feature
}
if d.class == classes.Monk {
    // create martial arts condition
    // create unarmored defense condition
}
// ... more classes = more if-statements
```

This doesn't scale. Each new class adds more hardcoded logic to `draft.go`.

## Existing Infrastructure

We already have good patterns:

1. **AutomaticGrants** (classes, races, backgrounds) - handles proficiencies, hit dice, saving throws
2. **SubclassModifications** - has `GrantedProficiencies`, `GrantedSpells`, `GrantedCantrips`
3. **Ref system** - `core.Ref` allows routing to module/type/value for loading
4. **Condition/Feature loaders** - `LoadJSON()` functions that hydrate from JSON blobs

The gap: class definitions don't include conditions/features they grant.

## The Principle: Classes Handle Themselves

A class definition should be self-contained:
- What proficiencies it grants
- What conditions it applies at each level
- What features it grants at each level
- What choices the player must make

External modules could define new classes via refs. The draft system shouldn't know "Monk gets Martial Arts" - it should ask "what does this class grant at level 1?"

## Proposed Extension

Add level-based grants to `AutomaticGrants`:

```go
type AutomaticGrants struct {
    // ... existing fields (HitDice, SavingThrows, etc.) ...

    LevelGrants []LevelGrant
}

type LevelGrant struct {
    Level      int
    Conditions []ConditionGrant
    Features   []FeatureGrant
}

type ConditionGrant struct {
    Ref    core.Ref        `json:"ref"`
    Config json.RawMessage `json:"config"`  // Passed to condition's loadJSON
}

type FeatureGrant struct {
    Ref    core.Ref        `json:"ref"`
    Config json.RawMessage `json:"config"`
}
```

## Config Format Options Considered

| Format | Pros | Cons |
|--------|------|------|
| `map[string]any` | Easy to write in Go | Type assertions, runtime panics |
| `json.RawMessage` | Matches existing `loadJSON()` pattern | Harder to construct in Go |
| Typed config per condition | Fully type-safe | Explosion of types |
| Interface-based | Each condition owns its config | More complex |

**Decision:** `json.RawMessage` aligns with existing patterns. Conditions already have `loadJSON()` methods - we pass the config blob directly.

## Example: Monk as Self-Contained

```go
case Monk:
    return &AutomaticGrants{
        HitDice:      8,
        SavingThrows: []abilities.Ability{abilities.STR, abilities.DEX},
        WeaponProficiencies: []proficiencies.Weapon{
            proficiencies.WeaponSimple,
            proficiencies.WeaponShortsword,
        },
        LevelGrants: []LevelGrant{{
            Level: 1,
            Conditions: []ConditionGrant{
                {
                    Ref: core.Ref{Module: "dnd5e", Type: "conditions", Value: "unarmored_defense"},
                    Config: json.RawMessage(`{"variant": "monk"}`),
                },
                {
                    Ref: core.Ref{Module: "dnd5e", Type: "conditions", Value: "martial_arts"},
                    Config: json.RawMessage(`{}`),
                },
            },
        }},
    }
```

## Simplified `compileConditions`

```go
func (d *Draft) compileConditions(characterID string) ([]ConditionBehavior, error) {
    grants := classes.GetAutomaticGrants(d.class)

    for _, lg := range grants.LevelGrants {
        if lg.Level <= 1 { // Character creation is level 1
            for _, cg := range lg.Conditions {
                cond, err := conditions.CreateFromGrant(cg, characterID)
                if err != nil {
                    return nil, err
                }
                conditionList = append(conditionList, cond)
            }
        }
    }

    // Fighting styles are choices, not grants - handle separately
    if style := d.GetFightingStyleSelection(); style != nil {
        // ...
    }

    return conditionList, nil
}
```

## Benefits

1. **Scalability** - Adding classes doesn't touch draft.go
2. **Self-contained** - All Monk data lives with Monk
3. **Extensible** - External modules can define classes via refs
4. **Level-up ready** - Just iterate grants where `Level <= newLevel`
5. **Consistent** - Matches existing `GrantedSpells` pattern from subclass modifications

## Relationship to Protos

This pattern is proto-friendly for the future. The grant structures could be proto messages, and `json.RawMessage` maps cleanly to `google.protobuf.Struct` or `bytes`.

## Next Steps

1. Add `LevelGrant`, `ConditionGrant`, `FeatureGrant` types to `classes/grants.go`
2. Add factory function `conditions.CreateFromGrant(grant, characterID)`
3. Update Monk and Barbarian to use new pattern
4. Simplify `compileConditions` and `compileFeatures` in draft.go
5. Consider doing same for races (racial traits as conditions/features)

## Open Questions

- Should fighting style choices also become grants? (They're choices, not automatic)
- Should we have a `ChoiceGrant` for things like "choose 2 skills from this list"?
- How does this interact with multiclassing? (Each class contributes its grants)
