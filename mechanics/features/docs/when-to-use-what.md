# When to Use What - Feature Creation Patterns

## The Two Paths

### 1. NewFeature() - Character Creation/Level Up

```go
// Used when:
// - Creating a new character
// - Character gains a level and gets new feature
// - Testing
// - Character editor/builder tools

func CreateNewBarbarian(name string) *Character {
    char := &Character{Name: name, Class: "barbarian", Level: 1}
    
    // At character creation, use NewRage
    rage := barbarian.NewRage(1)
    char.AddFeature(rage)
    
    // Save to database
    char.Save() // Calls rage.ToJSON()
    
    return char
}

// When barbarian reaches level 3
func LevelUp(char *Character) {
    if char.Class == "barbarian" && char.Level == 3 {
        // Add new feature
        frenzy := barbarian.NewFrenzy(3)
        char.AddFeature(frenzy)
        char.Save()
    }
}
```

### 2. LoadFromJSON() - Game Server Runtime

```go
// Game server NEVER calls NewRage()
// It only loads what was previously saved

func LoadFromGameContext(ctx GameContext, characterID string) (*Character, error) {
    var data CharacterData
    ctx.Database.Get(characterID, &data)
    
    char := &Character{}
    
    // Load features from saved JSON
    for _, featJSON := range data.Features {
        // Peek at ref
        var peek struct {
            Ref string `json:"ref"`
        }
        json.Unmarshal(featJSON, &peek)
        
        switch peek.Ref {
        case "dnd5e:feature:rage":
            // Load from saved state - NOT NewRage()
            rage := barbarian.LoadRageFromJSON(featJSON)
            char.features = append(char.features, rage)
            rage.Apply(ctx.EventBus)
        }
    }
    
    return char, nil
}
```

## Use Cases for Each

### NewFeature() is used by:

1. **Character Creation Service**
   ```go
   // POST /api/characters
   func CreateCharacter(req CreateCharacterRequest) {
       char := NewCharacter(req.Name, req.Class)
       if req.Class == "barbarian" {
           char.AddFeature(barbarian.NewRage(1))
       }
       database.Save(char)
   }
   ```

2. **Level Up Service**
   ```go
   // POST /api/characters/:id/levelup
   func LevelUpCharacter(charID string) {
       // Add new features based on class/level
   }
   ```

3. **Tests**
   ```go
   func TestRageDamageReduction(t *testing.T) {
       rage := barbarian.NewRage(5)
       rage.Apply(testBus)
       // Test damage reduction
   }
   ```

4. **Character Builder/Editor**
   ```go
   // UI for building characters
   func AddFeatureToCharacter(char *Character, featureType string) {
       switch featureType {
       case "rage":
           char.AddFeature(barbarian.NewRage(char.Level))
       }
   }
   ```

### LoadFromJSON() is used by:

1. **Game Server**
   ```go
   // When player connects to game
   func OnPlayerConnect(playerID string) {
       char := LoadFromGameContext(ctx, playerID)
       // Features are loaded from saved state
   }
   ```

2. **Migration/Import Tools**
   ```go
   // Importing character from another system
   func ImportCharacter(jsonData []byte) {
       // Parse and load features
   }
   ```

## The Key Insight

**The game server is stateless** - it doesn't know what a "Rage" is conceptually, it just knows:
1. Load JSON from database
2. Switch on ref to call right loader
3. Apply to event bus
4. Features handle themselves through events

The game server never creates new features - it only loads existing ones!

## Complete Lifecycle

```
Character Creation:
1. Character Builder → NewRage(1) → rage instance
2. rage.ToJSON() → {"ref": "dnd5e:feature:rage", "uses": 2, ...}
3. Save to database

Game Session:
1. Database → {"ref": "dnd5e:feature:rage", "uses": 1, ...}
2. LoadRageFromJSON() → rage instance (with uses: 1)
3. rage.Apply(eventBus) → subscribes to events
4. Play happens → rage modifies state
5. rage.ToJSON() → back to database

Level Up (separate service):
1. Check character level/class
2. NewFrenzy(3) → add to character
3. Save to database

Next Game Session:
1. Loads both rage AND frenzy from JSON
```

The separation is clean:
- **Creation functions** (NewRage) = For when features are first granted
- **Load functions** (LoadFromJSON) = For runtime/game server
- **Game server** = Doesn't need to know about creation, just loading