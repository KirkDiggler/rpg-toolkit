# Complete Game Server Example: LoadFromGameContext

This shows the complete flow of loading a character with features and conditions, playing, and saving back.

## The Character Structure

```go
// game/character.go
package game

import (
    "encoding/json"
    "github.com/KirkDiggler/rpg-toolkit/core"
    "github.com/KirkDiggler/rpg-toolkit/events"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/features"
    "github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e"
)

type Character struct {
    // Core identity
    ID   string
    Name string
    
    // Game state
    features   []features.Feature
    conditions []conditions.Condition
    resources  []resources.Resource
    
    // Runtime
    eventBus events.EventBus
    room     *spatial.Room
}

// CharacterData is what gets saved/loaded from database
type CharacterData struct {
    ID         string              `json:"id"`
    Name       string              `json:"name"`
    Class      string              `json:"class"`
    Level      int                 `json:"level"`
    HP         int                 `json:"hp"`
    MaxHP      int                 `json:"max_hp"`
    
    // Everything else is JSON
    Features   []json.RawMessage   `json:"features"`
    Conditions []json.RawMessage   `json:"conditions"`
    Resources  []json.RawMessage   `json:"resources"`
    Position   json.RawMessage     `json:"position"`
}
```

## LoadFromGameContext - The Main Entry Point

```go
// LoadFromGameContext loads a character into the game world
func LoadFromGameContext(ctx GameContext, characterID string) (*Character, error) {
    // 1. Get the raw character data from database
    var data CharacterData
    if err := ctx.Database.Get(characterID, &data); err != nil {
        return nil, fmt.Errorf("failed to load character: %w", err)
    }
    
    // 2. Create the character instance
    char := &Character{
        ID:       data.ID,
        Name:     data.Name,
        eventBus: ctx.EventBus,
        room:     ctx.Room,
    }
    
    // 3. Load all features
    for _, featJSON := range data.Features {
        feature, err := loadFeature(featJSON)
        if err != nil {
            log.Printf("Failed to load feature: %v", err)
            continue // Skip unknown/broken features
        }
        
        char.features = append(char.features, feature)
        
        // Apply the feature to activate its event subscriptions
        feature.Apply(ctx.EventBus)
        
        // Mark as clean since we just loaded it
        feature.MarkClean()
    }
    
    // 4. Load all conditions
    for _, condJSON := range data.Conditions {
        condition, err := loadCondition(condJSON)
        if err != nil {
            log.Printf("Failed to load condition: %v", err)
            continue
        }
        
        char.conditions = append(char.conditions, condition)
        
        // Apply the condition
        condition.Apply(ctx.EventBus)
        condition.MarkClean()
    }
    
    // 5. Load resources (spell slots, rage uses, etc.)
    for _, resJSON := range data.Resources {
        resource, err := loadResource(resJSON)
        if err != nil {
            log.Printf("Failed to load resource: %v", err)
            continue
        }
        
        char.resources = append(char.resources, resource)
    }
    
    // 6. Place character in room
    if data.Position != nil {
        var pos spatial.Position
        json.Unmarshal(data.Position, &pos)
        ctx.Room.PlaceEntity(char.ID, pos)
    }
    
    // 7. Announce character joined
    joinEvent := events.NewGameEvent("character.joined", char, nil)
    ctx.EventBus.Publish(context.Background(), joinEvent)
    
    return char, nil
}
```

## Loading Features/Conditions - The Switch Pattern

```go
// loadFeature loads a feature from JSON
func loadFeature(data json.RawMessage) (features.Feature, error) {
    // Peek at the ref to know what type to load
    var peek struct {
        Ref string `json:"ref"`
    }
    if err := json.Unmarshal(data, &peek); err != nil {
        return nil, err
    }
    
    // Switch on the ref to load the right type
    switch peek.Ref {
    case "dnd5e:feature:rage":
        return dnd5e.LoadRageFromJSON(data)
        
    case "dnd5e:feature:second_wind":
        return dnd5e.LoadSecondWindFromJSON(data)
        
    case "dnd5e:feature:sneak_attack":
        return dnd5e.LoadSneakAttackFromJSON(data)
        
    case "dnd5e:feature:action_surge":
        return dnd5e.LoadActionSurgeFromJSON(data)
        
    default:
        return nil, fmt.Errorf("unknown feature: %s", peek.Ref)
    }
}

// loadCondition loads a condition from JSON
func loadCondition(data json.RawMessage) (conditions.Condition, error) {
    var peek struct {
        Ref string `json:"ref"`
    }
    if err := json.Unmarshal(data, &peek); err != nil {
        return nil, err
    }
    
    switch peek.Ref {
    case "dnd5e:condition:poisoned":
        return dnd5e.LoadPoisonedFromJSON(data)
        
    case "dnd5e:condition:stunned":
        return dnd5e.LoadStunnedFromJSON(data)
        
    case "dnd5e:condition:exhaustion":
        return dnd5e.LoadExhaustionFromJSON(data)
        
    case "dnd5e:condition:blessed":
        return dnd5e.LoadBlessedFromJSON(data)
        
    default:
        return nil, fmt.Errorf("unknown condition: %s", peek.Ref)
    }
}
```

## During Play - Features React to Events

```go
// During combat, features automatically work through events
func (c *Character) TakeDamage(damage int, damageType string) {
    // Fire damage event
    dmgEvent := events.NewGameEvent(events.EventBeforeTakeDamage, nil, c)
    dmgEvent.Context().Set("damage", damage)
    dmgEvent.Context().Set("damage_type", damageType)
    
    // Rage (if active) will automatically reduce physical damage
    // Uncanny Dodge (if available) might halve it
    // Resistance spells might apply
    // All through event subscriptions!
    
    c.eventBus.Publish(context.Background(), dmgEvent)
    
    // Get final damage after all modifications
    finalDamage := dmgEvent.Context().Get("damage").(int)
    c.HP -= finalDamage
}

// Player activates a feature
func (c *Character) ActivateFeature(featureRef *core.Ref) error {
    // Find the feature
    var feature features.Feature
    for _, f := range c.features {
        if f.Ref().Equals(featureRef) {
            feature = f
            break
        }
    }
    
    if feature == nil {
        return fmt.Errorf("feature not found: %s", featureRef)
    }
    
    // Fire activation event - the feature will handle it
    activateEvent := events.NewGameEvent("feature.activate", c, nil)
    activateEvent.Context().Set("feature_ref", featureRef)
    
    return c.eventBus.Publish(context.Background(), activateEvent)
}
```

## Saving Back - Only What Changed

```go
// SaveIfDirty saves only dirty features/conditions
func (c *Character) SaveIfDirty() error {
    needsSave := false
    
    // Check features
    var dirtyFeatures []json.RawMessage
    for _, feat := range c.features {
        if feat.IsDirty() {
            dirtyFeatures = append(dirtyFeatures, feat.ToJSON())
            feat.MarkClean()
            needsSave = true
        }
    }
    
    // Check conditions
    var dirtyConditions []json.RawMessage
    for _, cond := range c.conditions {
        if cond.IsDirty() {
            dirtyConditions = append(dirtyConditions, cond.ToJSON())
            cond.MarkClean()
            needsSave = true
        }
    }
    
    if !needsSave {
        return nil // Nothing to save
    }
    
    // Update only what changed
    updates := map[string]interface{}{}
    if len(dirtyFeatures) > 0 {
        updates["features"] = dirtyFeatures
    }
    if len(dirtyConditions) > 0 {
        updates["conditions"] = dirtyConditions
    }
    
    return c.savePartial(updates)
}

// SaveComplete saves everything (on logout, checkpoint, etc.)
func (c *Character) SaveComplete() error {
    data := CharacterData{
        ID:    c.ID,
        Name:  c.Name,
        HP:    c.HP,
        MaxHP: c.MaxHP,
    }
    
    // Save all features
    for _, feat := range c.features {
        data.Features = append(data.Features, feat.ToJSON())
        feat.MarkClean()
    }
    
    // Save all conditions
    for _, cond := range c.conditions {
        data.Conditions = append(data.Conditions, cond.ToJSON())
        cond.MarkClean()
    }
    
    // Save all resources
    for _, res := range c.resources {
        data.Resources = append(data.Resources, res.ToJSON())
    }
    
    // Save position
    if c.room != nil {
        pos := c.room.GetPosition(c.ID)
        data.Position, _ = json.Marshal(pos)
    }
    
    return c.database.Save(c.ID, data)
}
```

## Turn Management

```go
// EndTurn is called when character's turn ends
func (c *Character) EndTurn() error {
    // Fire turn end event - features/conditions will update
    endEvent := events.NewGameEvent("turn.end", c, nil)
    
    // Track if character attacked or took damage (for rage maintenance)
    endEvent.Context().Set("made_attack_this_turn", c.attackedThisTurn)
    endEvent.Context().Set("took_damage_this_turn", c.tookDamageThisTurn)
    
    c.eventBus.Publish(context.Background(), endEvent)
    
    // Reset turn flags
    c.attackedThisTurn = false
    c.tookDamageThisTurn = false
    
    // Save any features that changed
    return c.SaveIfDirty()
}
```

## Complete Flow Example

```go
func GameSession(ctx GameContext) {
    // 1. Player connects - load their character
    char, err := LoadFromGameContext(ctx, "player-123")
    if err != nil {
        log.Fatal(err)
    }
    
    // 2. Character enters combat
    // Their rage feature is loaded with 2 uses remaining from last session
    // They have a poisoned condition with 3 ticks left
    
    // 3. Player's turn - they rage
    char.ActivateFeature(barbarian.RageRef)
    // Rage feature marks itself dirty (uses: 2 → 1)
    
    // 4. Player attacks
    char.Attack(goblin)
    // Attack events fire, rage adds damage bonus
    
    // 5. Goblin's turn - attacks back
    char.TakeDamage(10, "slashing")
    // Rage reduces to 5 damage automatically
    
    // 6. End of player's turn
    char.EndTurn()
    // SaveIfDirty() called - rage is saved with uses=1
    // Poisoned ticks down to 2, saves
    
    // 7. Combat ends
    // Rage ends (no enemies), marks dirty, saves
    
    // 8. Player logs out
    char.SaveComplete()
    // Everything saved for next session
}
```

## The Key Benefits

1. **Features are self-contained** - They handle their own events
2. **Automatic persistence** - Dirty tracking means we only save what changed  
3. **Graceful degradation** - Unknown features are skipped, not crashed
4. **Event-driven** - No need to check "is raging?" everywhere
5. **Type safety where it matters** - Internal APIs use typed structs
6. **Flexibility where needed** - Storage uses JSON

## Potential Issues and Solutions

### Issue 1: Database Performance
**Problem**: Saving after every turn might be too frequent
**Solution**: Batch saves every N turns or time interval

### Issue 2: Unknown Features
**Problem**: Loading old save with new feature not yet implemented
**Solution**: Log and skip - game continues without that feature

### Issue 3: Feature Versioning
**Problem**: Feature data format changes between versions
**Solution**: Include version in JSON, migrate on load

### Issue 4: Circular Dependencies
**Problem**: Feature A depends on Feature B's state
**Solution**: Use events for communication, not direct references

The path forward is clear: Simple interfaces, event-driven behavior, JSON for persistence!