---
name: rulebooks/dnd5e/refs package
description: The boundary key — typed namespaces of *core.Ref singletons that rpg-api uses to tell the toolkit what to do
updated: 2026-05-04
confidence: high — verified by reading refs/*.go and rpg-api's symbol histogram per audit 049
---

# refs package

**Path:** `rulebooks/dnd5e/refs/`
**Module:** `github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs` (sub-package of `rulebooks/dnd5e`)

The boundary rule of the project — "client sends references, never
calculations" — makes `refs/` the literal API surface between rpg-api and
the toolkit. This is broken out into its own component doc because of how
heavily rpg-api depends on it (the audit found 12 files in rpg-api importing
it, with ~400 ref usages across the symbol histogram).

A `*core.Ref` is the routing key for any toolkit content (feature, condition,
weapon, action, monster, etc.). The `refs/` package wraps the raw `core.Ref`
constructors into typed, IDE-discoverable namespaces.

## Why this exists as its own package

Per the package doc comment in `rulebooks/dnd5e/refs/module.go`:

> This package is a leaf package — it only imports core to ensure all other
> dnd5e packages can import it without cycles.

That's the architectural reason. The user-facing reason is IDE autocomplete:
typing `refs.Features.<tab>` discovers every D&D 5e feature; typing
`refs.Conditions.<tab>` discovers every condition. No magic strings.

## How rpg-api uses it

| Namespace | Refs in rpg-api | Example callsite |
|---|---|---|
| `refs.Weapons` | 96 | converters mapping proto weapon enums |
| `refs.Features` | 85 | `refs.Features.Rage()` in feature handlers |
| `refs.Conditions` | 51 | condition state mapping |
| `refs.Actions` | 45 | action mapping |
| `refs.Monsters` | 43 | encounter handler |
| `refs.Tools` | 35 | choice mapping |
| `refs.CombatAbilities` | 31 | combat-ability mapping |
| `refs.Abilities` | 15 | DEX/STR/CON ability score mapping |
| `refs.Module`, `refs.Armor` | 14, 13 | misc |

The full call shape: rpg-api receives a proto enum (e.g. `Feature.RAGE`),
maps it to `refs.Features.Rage()`, and passes the resulting `*core.Ref` to a
toolkit factory or activation call. The toolkit owns "what Rage does"; rpg-api
just routes by ref.

## The namespace pattern

Every namespace is a struct singleton with methods returning unexported
`*core.Ref` variables. Example from `refs/features.go`:

```go
// Feature singletons - unexported for controlled access via methods
var (
    featureRage           = &core.Ref{Module: Module, Type: TypeFeatures, ID: "rage"}
    featureBrutalCritical = &core.Ref{Module: Module, Type: TypeFeatures, ID: "brutal_critical"}
    // ... more features ...
)

// Features provides type-safe, discoverable references to D&D 5e features.
var Features = featuresNS{}

type featuresNS struct{}

func (n featuresNS) Rage() *core.Ref           { return featureRage }
func (n featuresNS) BrutalCritical() *core.Ref { return featureBrutalCritical }
// ...
```

Two architectural properties fall out of this pattern:

1. **Singleton identity.** Methods return the same pointer every call. So
   `ref == refs.Features.Rage()` is a valid identity comparison — useful for
   switch-by-ref dispatch inside the toolkit.

2. **Module-namespaced.** Every ref carries `Module: "dnd5e"`. Future modules
   (e.g., a Pathfinder rulebook) can define their own `refs/` packages with
   `Module: "pathfinder"` and the same IDs won't collide.

The Module constant and Type constants are defined in `refs/module.go`:

```go
const Module core.Module = "dnd5e"

const (
    TypeFeatures        core.Type = "features"
    TypeConditions      core.Type = "conditions"
    TypeActions         core.Type = "actions"
    TypeWeapons         core.Type = "weapons"
    // ... more types ...
)
```

## Available namespaces

Verified by `grep -nE '^var [A-Z]' refs/*.go`:

| Namespace | File | Purpose |
|---|---|---|
| `refs.Abilities` | `abilities.go` | DEX/STR/CON/WIS/INT/CHA |
| `refs.Actions` | `actions.go` | Strike, Dodge, etc. |
| `refs.Armor` | `armor.go` | Armor by ID |
| `refs.Backgrounds` | `backgrounds.go` | Soldier, Acolyte, etc. |
| `refs.Classes` | `classes.go` | Fighter, Barbarian, etc. |
| `refs.CombatAbilities` | `combat_abilities.go` | Attack, Dash, Disengage |
| `refs.Conditions` | `conditions.go` | Raging, Dodging, etc. |
| `refs.DamageTypes` | `damage_types.go` | Slashing, Bludgeoning, etc. |
| `refs.Features` | `features.go` | Rage, Second Wind, etc. |
| `refs.Languages` | `languages.go` | Common, Elvish, etc. |
| `refs.MonsterActions` | `monster_actions.go` | Bite, Multiattack, etc. |
| `refs.MonsterTraits` | `monster_traits.go` | Pack Tactics, etc. |
| `refs.Monsters` | `monsters.go` | Goblin, Brown Bear, etc. |
| `refs.Races` | `races.go` | Human, Elf, etc. |
| `refs.Skills` | `skills.go` | Athletics, Stealth, etc. |
| `refs.Spells` | `spells.go` | Sleep, Magic Missile, etc. |
| `refs.Tools` | `tools.go` | Smith's tools, etc. |
| `refs.Weapons` | `weapons.go` | Longsword, Shortbow, etc. |

## Relationship to core.Ref and core.SourcedRef

`refs/` is a constructor layer over `core.Ref` (defined in `core/ref.go`).
The `Ref` struct itself has three fields:

```go
type Ref struct {
    Module string  // "dnd5e"
    Type   string  // "features"
    ID     string  // "rage"
}
```

Stringified form: `"dnd5e:features:rage"`.

`core.SourcedRef` adds provenance:

```go
type SourcedRef struct {
    Ref    *Ref
    Source *Source  // Category + Name (class, background, race, etc.)
}
```

`SourcedRef` is what the toolkit uses to track *where* a modifier came from
(e.g., "this +2 came from Barbarian Rage") so UI breakdowns can explain a
final number. `refs/` constructors return plain `*core.Ref`; the toolkit
wraps them in `SourcedRef` when it needs provenance.

## Verification

```sh
# All refs namespaces
grep -nE '^var [A-Z]' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/refs/*.go | grep -v _test

# Module/Type constants
grep -nE '^\tType[A-Z]' /home/kirk/personal/rpg-toolkit/rulebooks/dnd5e/refs/module.go

# rpg-api's import count and top namespaces
grep -rln '"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | wc -l   # expect 12
grep -rohE 'refs\.(Weapons|Features|Conditions|Actions|Monsters)' /home/kirk/personal/rpg-api/internal/ /home/kirk/personal/rpg-api/cmd/ --include="*.go" | sort | uniq -c | sort -rn
```
