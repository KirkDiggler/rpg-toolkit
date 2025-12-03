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

## Alternative Pattern: Singleton Refs with Identity

During brainstorming, we explored a more advanced pattern that wasn't needed here but is worth documenting for future use.

### The Limitation of Current Approach

Pointers don't fully solve validity:
```go
var ref *core.Ref = nil           // Obviously missing - good!
var ref *core.Ref = &core.Ref{}   // Pointer exists but empty strings - still invalid!
```

### Singleton Pattern for Identity

The idea: refs are created once at package init, and every call returns the SAME pointer.

```go
// Package-level singletons - created once
var (
    _rage       = &core.Ref{Module: Module, Type: TypeFeatures, ID: "rage"}
    _secondWind = &core.Ref{Module: Module, Type: TypeFeatures, ID: "second_wind"}
)

func (featuresNS) Rage() *core.Ref       { return _rage }
func (featuresNS) SecondWind() *core.Ref { return _secondWind }
```

**What this enables:**
```go
// Pointer equality = identity check
if someRef == refs.Features.Rage() {
    // This IS the rage ref, not just something that looks like it
}

// Use as map keys by pointer (not string)
handlers := map[*core.Ref]Handler{
    refs.Features.Rage(): rageHandler,
    refs.Features.SecondWind(): secondWindHandler,
}
```

### Even Further: Registered Refs

For maximum control, refs could be registered through a central registry:

```go
// In core package
type Ref struct {
    module Module
    typ    Type
    id     ID
}

var registry = map[string]*Ref{}

// Only way to create refs - guarantees uniqueness
func Register(module Module, typ Type, id ID) *Ref {
    key := fmt.Sprintf("%s:%s:%s", module, typ, id)
    if existing, ok := registry[key]; ok {
        return existing  // Same ref every time
    }
    ref := &Ref{module, typ, id}
    registry[key] = ref
    return ref
}

// In refs package - registered at init
var _rage = core.Register("dnd5e", "features", "rage")

func (featuresNS) Rage() *core.Ref { return _rage }
```

**Benefits:**
- **Identity**: Pointer equality works
- **Canonicalization**: Only one instance per ref exists
- **Validation**: Can't create arbitrary refs without going through Register
- **Discoverability**: Registry could be queried for all known refs

### Simplest Alternative: Ref as String Type

If we wanted maximum simplicity:

```go
type Ref string

const (
    RageFeature     Ref = "dnd5e:features:rage"
    RagingCondition Ref = "dnd5e:conditions:raging"
)

func (r Ref) Module() Module { return Module(strings.Split(string(r), ":")[0]) }
func (r Ref) Type() Type     { return Type(strings.Split(string(r), ":")[1]) }
func (r Ref) ID() ID         { return ID(strings.Split(string(r), ":")[2]) }
```

**Benefits:**
- Constants are true constants (compile-time)
- Identity via `==` works naturally
- Zero allocation
- Can be map keys directly

**Drawbacks:**
- Parsing on every accessor call (could cache)
- Less structured than a proper struct
- No IDE autocomplete for `refs.Features.<tab>`

### Why We Didn't Use These (Yet)

For our current needs:
1. We compare by `.String()` or `.ID` which works fine
2. We don't use refs as map keys by pointer
3. The namespace pattern (`refs.Features.Rage()`) gives us IDE discoverability

But these patterns are valuable to know for:
- Error types (like `rpgerr` where `Is` checks matter)
- Enum-like constants that need identity
- High-performance scenarios where allocation matters
- Plugin systems where refs need central registration

## Lessons Learned

1. **Brainstorming is valuable** - We went full circle from "use pointers" to "maybe values" back to "use pointers" but with better reasoning.

2. **Context matters** - For `core.Ref` specifically, values would work fine because zero values are invalid. But consistency with the rest of the codebase won.

3. **Document the reasoning** - Future us will wonder "why pointers?" This doc explains it.

## Related

- PR #379 - Created refs package
- PR #381 - Removed duplicate ID constants
- Issue #380 - Tracked the cleanup work
