# Refs Package Design: Pointers vs Values

## The Problem

We had circular import dependencies and magic strings scattered throughout the dnd5e package. Feature and condition IDs like `"rage"`, `"raging"`, `"brutal_critical"` were duplicated in multiple places.

## The Solution: refs Package

Created a `refs` package as a leaf package (only imports `core`) that provides discoverable, type-safe references:

```go
refs.Features.Rage()        // *core.Ref for the Rage feature
refs.Conditions.Raging()    // *core.Ref for the Raging condition
refs.Classes.Barbarian()    // *core.Ref for the Barbarian class
```

## The Brainstorm: Pointers vs Values

We had a long discussion about whether refs should return pointers or values.

### Arguments for Values

1. **Zero values are obviously invalid** - An empty `core.Ref{}` has empty strings. Any code using it fails immediately or produces obviously wrong output like `"::"`.

2. **Refs are immutable** - We create them and never modify them. The "modified a copy" bug doesn't apply.

3. **Small struct** - 3 strings, cheap to copy, no performance reason for pointers.

4. **Cleaner syntax** - `Ref: refs.Features.Rage()` vs `Ref: *refs.Features.Rage()`

### Arguments for Pointers

1. **Explicit about presence** - `nil` means "not set", zero value is ambiguous ("is this intentional?")

2. **Consistency** - Services, repos, clients are all pointers. Having refs be pointers too makes the codebase more uniform.

3. **Avoids copy bugs** - When passing a struct by value, modifications don't stick. With pointers, this is explicit.

4. **Easier to use** - If Data struct fields are `*core.Ref`, usage is `Ref: refs.Features.Rage()` with no dereference needed.

### The Java vs Go Flip

Interesting observation: In Java, passing references was problematic because everything is a reference (except primitives) - you had to be careful about unintended sharing/mutation. In Go, the choice is explicit, and for small immutable data, values are actually safer because you CAN'T accidentally mutate shared state.

## Our Decision: Pointers

We chose **pointers** for these reasons:

1. **Consistency with the rest of the codebase** - Most things are pointers
2. **Easy to use** - With `*core.Ref` fields in Data structs, no dereferencing needed
3. **Explicit** - `nil` is unambiguous
4. **Familiarity** - The team is more comfortable with pointers

### The Pattern

```go
// refs/features.go - Returns pointer
func (featuresNS) Rage() *core.Ref {
    return &core.Ref{Module: Module, Type: TypeFeatures, ID: "rage"}
}

// Data struct - Pointer field
type RageData struct {
    Ref *core.Ref `json:"ref"`
    // ...
}

// Usage - Clean, no dereference
data := RageData{
    Ref: refs.Features.Rage(),
}
```

### Bonus: Compact JSON Serialization

`core.Ref` has a custom `MarshalJSON` that serializes pointers to compact string format:

```json
{"ref": "dnd5e:conditions:brutal_critical", "character_id": "barbarian-1"}
```

Instead of the verbose object format:

```json
{"ref": {"module": "dnd5e", "type": "conditions", "id": "brutal_critical"}, "character_id": "barbarian-1"}
```

## Lessons Learned

1. **Brainstorming is valuable** - We went full circle from "use pointers" to "maybe values" back to "use pointers" but with better reasoning.

2. **Context matters** - For `core.Ref` specifically, values would work fine because zero values are invalid. But consistency with the rest of the codebase won.

3. **Document the reasoning** - Future us will wonder "why pointers?" This doc explains it.

## Related

- PR #379 - Created refs package
- PR #381 - Removed duplicate ID constants
- Issue #380 - Tracked the cleanup work
