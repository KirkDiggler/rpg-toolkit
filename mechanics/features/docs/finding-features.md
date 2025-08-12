# Finding Features - Matching Player Input

## The Ref is the Unique ID

```go
// Each feature has a unique ref
"dnd5e:feature:rage"
"dnd5e:feature:second_wind"
"dnd5e:spell:fireball"
```

## Character Has Helper Methods

```go
type Character struct {
    features map[string]Feature  // Keyed by ref string for fast lookup
}

// GetFeature by ref - O(1) lookup
func (c *Character) GetFeature(ref string) (Feature, error) {
    if feature, exists := c.features[ref]; exists {
        return feature, nil
    }
    return nil, ErrFeatureNotFound
}

// GetFeatureByRef - type safe version
func (c *Character) GetFeatureByRef(ref *core.Ref) (Feature, error) {
    return c.GetFeature(ref.String())
}
```

## Player Activation Flow

```go
// 1. UI shows available actions
func (c *Character) GetAvailableActions() []ActionInfo {
    var actions []ActionInfo
    
    for ref, feature := range c.features {
        if feature.CanActivate() {
            actions = append(actions, ActionInfo{
                Ref:         ref,  // This is what we'll get back!
                Name:        feature.Name(),
                Description: feature.Description(),
                Remaining:   feature.GetRemainingUses(),
            })
        }
    }
    
    return actions
}

// Returns:
[
    {
        "ref": "dnd5e:feature:rage",
        "name": "Rage",
        "description": "Enter battle fury...",
        "remaining": "2/3"
    }
]

// 2. Player clicks button, sends ref back
POST /api/activate
{
    "ref": "dnd5e:feature:rage"
}

// 3. Server activates by ref
func HandleActivate(req ActivateRequest) error {
    feature, err := character.GetFeature(req.Ref)
    if err != nil {
        return err
    }
    
    return feature.Activate()
}
```

## Alternative: Also Support Fuzzy Matching

```go
// For voice commands, chat bots, etc.
func (c *Character) FindFeature(input string) (Feature, error) {
    input = strings.ToLower(input)
    
    // Try exact ref match first
    if feature, exists := c.features[input]; exists {
        return feature, nil
    }
    
    // Try matching by name
    for _, feature := range c.features {
        if strings.ToLower(feature.Name()) == input {
            return feature, nil
        }
    }
    
    // Try partial match
    var matches []Feature
    for _, feature := range c.features {
        name := strings.ToLower(feature.Name())
        if strings.Contains(name, input) {
            matches = append(matches, feature)
        }
    }
    
    if len(matches) == 1 {
        return matches[0], nil
    }
    
    if len(matches) > 1 {
        return nil, &AmbiguousError{
            Input:   input,
            Matches: matches,
        }
    }
    
    return nil, ErrFeatureNotFound
}

// Usage:
// "rage" → finds Rage
// "second" → finds Second Wind
// "wind" → finds Second Wind
// "ra" → might be ambiguous if you have Rage and Ray of Frost
```

## For Discord/Chat Commands

```go
// Player types: !use rage
func HandleChatCommand(player *Character, command string) {
    parts := strings.Split(command, " ")
    if parts[0] != "!use" {
        return
    }
    
    featureName := strings.Join(parts[1:], " ")
    
    // Find feature by name or ref
    feature, err := player.FindFeature(featureName)
    if err != nil {
        if ambig, ok := err.(*AmbiguousError); ok {
            reply := "Which one? "
            for _, f := range ambig.Matches {
                reply += f.Name() + ", "
            }
            sendReply(reply)
            return
        }
        sendReply("Feature not found: " + featureName)
        return
    }
    
    // Try to activate
    if err := feature.Activate(); err != nil {
        sendReply(err.Error())
        return
    }
    
    sendReply(fmt.Sprintf("%s activated!", feature.Name()))
}
```

## The Clean Separation

```go
// Features are stored by ref for fast lookup
type Character struct {
    features map[string]Feature  // map["dnd5e:feature:rage"] = RageFeature
}

// Loading builds the map
func LoadFromGameContext(ctx GameContext, charID string) (*Character, error) {
    char := &Character{
        features: make(map[string]Feature),
    }
    
    for _, featJSON := range data.Features {
        feature := LoadFeatureFromJSON(featJSON)
        char.features[feature.Ref().String()] = feature
        feature.Apply(ctx.EventBus)
    }
    
    return char, nil
}

// Activation is simple
func (c *Character) ActivateByRef(ref string) error {
    feature, exists := c.features[ref]
    if !exists {
        return ErrFeatureNotFound
    }
    return feature.Activate()
}
```

## The Key Point

**The ref IS the unique identifier!**

- UI sends ref with activation request
- Server looks up feature by ref (O(1) with map)
- Feature activates itself
- No string parsing or guessing needed

For chat/voice, you add fuzzy matching on top, but the core system uses refs!