# Journey 025: Refs Are Package Paths, Not IDs

## Date: 2025-08-12

## The Discovery

While implementing the conditions simplification, we had a debate about how to extract refs:
1. Parse from JSON: `{"ref": {"module": "dnd5e", ...}}`
2. Parse from string: `"dnd5e:condition:poisoned"`

Initially, I thought JSON extraction was "better" because it handled complex data. But Kirk pointed out that the "limitations" of string parsing were actually **strengths**.

## The Revelation

Refs aren't just identifiers. They are, in order of importance:

1. **Routing information** - WHERE to send this data for processing
2. **Package paths** - WHAT code handles this data  
3. **Identifiers** - yes, but that's almost incidental

## The Analogy That Clicked

It's like HTTP:
- URL path = the ref (`/api/conditions/poisoned`)
- Request body = the data (JSON payload)

You wouldn't embed the URL inside the request body! The ref string is our "URL" for routing.

## The String Parsing Superpower

By using `core.ParseString()`, we delegate ref evolution to the core module:

```go
// This loader code NEVER needs to change:
func Load(data json.RawMessage) (ConditionData, error) {
    // Peek at the ref string
    var peek struct {
        Ref string `json:"ref"`
    }
    json.Unmarshal(data, &peek)
    
    // Let core figure out what this means
    ref, err := core.ParseString(peek.Ref)
    
    // We just route, we don't interpret
    return &conditionData{
        ref:  ref,
        data: data,
    }, nil
}
```

Now refs can evolve without touching conditions code:
- `"dnd5e:condition:poisoned"` ✓
- `"homebrew:condition:affliction:v2"` ✓
- `"homebrew:condition:affliction:v2:experimental"` ✓
- `"homebrew:condition:affliction:v2:experimental:kirk-variant"` ✓

The conditions module doesn't care about the structure!

## The Package Path Mental Model

Refs are literally package paths for game content:

```go
// In Go:
import "github.com/KirkDiggler/rpg-toolkit/mechanics/conditions"

// In our system:
ref := "dnd5e:condition:poisoned"  // Import poisoned from dnd5e package
```

This mental model changes everything:
- Versioning becomes natural: `dnd5e:spell:fireball:v2`
- Variants are obvious: `homebrew:feature:rage:barbarian-variant`
- Namespacing is built-in: `kirks-campaign:item:sword-of-awesome`

## The Implementation Pattern

```go
// Step 1: Peek at the ref string (not the full ref object)
type peekData struct {
    Ref string `json:"ref"`
}

// Step 2: Parse it with core.ParseString
ref, err := core.ParseString(peek.Ref)

// Step 3: Return ref + full JSON for routing
return &conditionData{
    ref:  ref,  // For routing
    data: data, // Full payload
}
```

## Why This Matters

1. **Evolution without breaking changes** - Refs can grow more sophisticated without touching feature/condition code
2. **Clear separation** - Routing (ref) vs Data (JSON) are properly separated
3. **Delegation of complexity** - Only core needs to understand ref structure
4. **Natural versioning** - Package path mental model makes versions obvious

## The Anti-Pattern We Avoided

```go
// DON'T DO THIS - embedding ref in JSON structure
var data struct {
    Ref struct {
        Module string `json:"module"`
        Type   string `json:"type"`
        Value  string `json:"value"`
    } `json:"ref"`
}
```

This would have coupled us to a specific ref structure forever.

## The Pattern We Embraced

```go
// DO THIS - ref as simple string, parsed by core
var peek struct {
    Ref string `json:"ref"`  // Just "dnd5e:condition:poisoned"
}
ref, _ := core.ParseString(peek.Ref)  // Core handles evolution
```

## Success Criteria

When refs evolve to support:
- Versioning
- Variants
- Namespacing  
- Experimental flags
- Whatever we haven't thought of yet

Our feature/condition loaders won't need a single line changed.

## The Lesson

Sometimes the "limitation" IS the feature. By keeping refs as strings and delegating parsing to core, we've created a system that can evolve without cascading changes.

The best architecture decisions often feel like we're doing LESS, not more.