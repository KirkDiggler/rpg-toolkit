# Features Architecture Proposal

## The Key Insight

We need to separate:
1. **Identity** (the Ref) - "What is this thing?"
2. **Implementation** (the behavior) - "What does it do?"

## Where Things Live

### 1. Ref Constants (Just Identifiers)
```go
// mechanics/features/refs/refs.go
package refs

// These are JUST identifiers, no behavior
var (
    Rage = core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "feature",
        Value:  "rage",
    })
    
    SneakAttack = core.MustNewRef(core.RefInput{
        Module: "dnd5e",
        Type:   "feature",
        Value:  "sneak_attack",
    })
)
```

### 2. Feature Factories (The Actual Implementation)
```go
// rulebooks/dnd5e/features/rage.go
package features

func NewRage() *mechanics.SimpleFeature {
    return mechanics.NewSimple(
        mechanics.WithRef(refs.Rage),  // Identity
        mechanics.FromSource(sources.ClassBarbarian),
        mechanics.AtLevel(1),
        mechanics.OnApply(func(f *mechanics.SimpleFeature, bus events.EventBus) error {
            // THIS is where the actual rage logic lives
            // Subscribe to damage events for resistance
            // Add strength bonus
            // Track rage duration
            return nil
        }),
    )
}
```

### 3. Usage Patterns

```go
// Adding a feature (using the factory)
barbarian.AddFeature(dnd5e.NewRage())

// Checking for a feature (using the ref constant)
if barbarian.HasFeature(refs.Rage) {
    // Can use rage
}

// Getting a feature (returns the actual instance with behavior)
if rage, ok := barbarian.GetFeature(refs.Rage); ok {
    rage.Apply(eventBus)
}
```

## The Problem This Solves

Before:
```go
// String everywhere, no consistency
feat := NewFeature("rage", "barbarian", func() {...})
if char.HasFeature("rage") { // Typo? "Rage"? "RAGE"?
```

After:
```go
// Ref constant for identity
feat := dnd5e.NewRage() // Factory creates the feature
if char.HasFeature(refs.Rage) { // Compile-time safe
```

## How It Reads

```go
// Clean, obvious what's happening
character.AddFeature(dnd5e.NewRage())
character.AddFeature(dnd5e.NewSecondWind())

// Later, checking what they have
if character.HasFeature(refs.Rage) {
    fmt.Println("This character can rage!")
}

// Or getting all barbarian features
for _, feat := range character.GetFeaturesFromSource(sources.ClassBarbarian) {
    fmt.Printf("Has barbarian feature: %s\n", feat.Ref())
}
```

## Questions to Validate

1. **Should refs be in the mechanics package or with implementations?**
   - Mechanics: Central location, easy imports
   - With implementations: Keeps related code together
   
2. **How do we handle custom/homebrew features?**
   ```go
   customRef := core.MustNewRef(core.RefInput{
       Module: "homebrew",
       Type:   "feature",
       Value:  "super_rage",
   })
   ```

3. **Should we have a registry that maps refs to factories?**
   ```go
   registry.Register(refs.Rage, dnd5e.NewRage)
   // Then later...
   feat := registry.Create(refs.Rage)
   ```

## Next Steps

Before implementing, we should:
1. Decide on package structure
2. Create a small proof of concept
3. See how it feels to use
4. Adjust based on what we learn