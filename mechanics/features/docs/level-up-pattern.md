# Level Up Pattern - The Real Flow

## The Problem You Identified

We don't want the game server calling NewRage()! The game server should only load from data.

## The Real Solution: Level Up is a Separate Service

### Option 1: Level Up Service Modifies Database

```go
// Level Up Service (separate from game server)
func LevelUpCharacter(charID string, newLevel int) {
    // This service knows about game rules
    charData := database.Get(charID)
    
    if charData.Class == "barbarian" && newLevel == 1 {
        // Create the feature data directly
        rageData := RageData{
            Ref:           "dnd5e:feature:rage",
            Level:         1,
            UsesRemaining: 2,
            MaxUses:       2,
            IsActive:      false,
        }
        
        // Add to character's features in database
        charData.Features = append(charData.Features, json.Marshal(rageData))
        database.Save(charData)
    }
}

// Game server just loads what's there
func LoadFromGameContext(ctx GameContext, charID string) (*Character, error) {
    data := database.Get(charID)
    // Just load features that exist - no logic about levels!
    for _, featJSON := range data.Features {
        feature := LoadFeatureFromJSON(featJSON)
        char.AddFeature(feature)
    }
}
```

### Option 2: Features Check Their Own Availability

```go
// Each feature knows when it should exist
func (r *RageFeature) ShouldExist(class string, level int) bool {
    return class == "barbarian" && level >= 1
}

// But this still requires something to CREATE it initially...
```

### Option 3: Pre-Generated Feature Tables (Best?)

```go
// Database has a table of "what features at what level"
// This gets populated during game setup, not runtime

CREATE TABLE class_features (
    class VARCHAR,
    level INT,
    feature_data JSON
);

// Populated with:
INSERT INTO class_features VALUES 
    ('barbarian', 1, '{"ref": "dnd5e:feature:rage", "uses": 2, ...}'),
    ('barbarian', 3, '{"ref": "dnd5e:feature:frenzy", ...}'),
    ('fighter', 1, '{"ref": "dnd5e:feature:second_wind", ...}');

// When character levels up (separate service)
func GrantLevelUpFeatures(charID string, newLevel int) {
    char := database.GetCharacter(charID)
    
    // Get features for this class/level from template
    templates := database.Query(
        "SELECT feature_data FROM class_features WHERE class = ? AND level = ?",
        char.Class, newLevel
    )
    
    for _, template := range templates {
        // Copy template to character
        char.Features = append(char.Features, template)
    }
    
    database.Save(char)
}
```

## The Key Insight

**The game server should NEVER make decisions about what features to grant!**

Instead:
1. **Character Creation Service** - Knows the rules, creates initial features
2. **Level Up Service** - Knows the rules, adds new features to database
3. **Game Server** - Just loads and runs what's in the database

## The Clean Separation

```go
// rulebooks/dnd5e/progression/barbarian.go
// This is DATA, not runtime code!

var BarbarianProgression = map[int][]FeatureData{
    1: {
        RageData{
            Ref:           "dnd5e:feature:rage",
            UsesRemaining: 2,
            MaxUses:       2,
        },
        UnarmoredDefenseData{
            Ref: "dnd5e:feature:unarmored_defense",
        },
    },
    2: {
        RecklessAttackData{
            Ref: "dnd5e:feature:reckless_attack",
        },
    },
    3: {
        // Subclass feature added here
    },
}

// Level up service uses this data
func GrantBarbarianFeatures(char *Character, level int) {
    if features, exists := BarbarianProgression[level]; exists {
        for _, featData := range features {
            char.Features = append(char.Features, json.Marshal(featData))
        }
    }
}
```

## So When IS NewRage() Used?

Honestly... maybe never in production! It might just be for:
- Tests (create a rage to test with)
- Development tools
- Data generation scripts

The production flow might be:
1. **Data setup**: Create feature templates in database
2. **Character creation**: Copy templates to character
3. **Level up**: Copy more templates to character  
4. **Game server**: Just load from database

NewRage() might just be a convenience for testing!