# D&D 5e rulebook

`rulebooks/dnd5e` is one Go module:

```text
github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e
```

It implements D&D 5e rules and content. It does not store game state or know
about a particular database, API, proto, or UI. A host supplies IDs, loads
rulebook-owned data, calls rulebook behavior, and stores the resulting data.

## Recommended first contribution

Add a simple monster whose entire stat block can be composed from behavior that
already exists. The [human + agent monster guide](../../docs/how-to/add-a-dnd5e-monster.md)
contains the provenance gate, supported-capability audit, exact files, tests,
and round-trip checklist.

Do not copy closed Monster Manual text or statistics. SRD 5.1 content may be
used under its CC BY 4.0 terms with attribution; original content is also
welcome. The guide has the full source contract.

## The rulebook mental model

Follow one fact through these layers:

```text
ref/key → definition or factory → runtime rule behavior → Data/JSON → host storage
                                      ↑
                               event bus / rule chains
```

1. **Refs are boundary keys.** `refs.Monsters.Bandit()` identifies content as
   `dnd5e:monsters:bandit`. Hosts pass keys; they do not reproduce the rule.
2. **Definitions compose existing rules.** A monster factory chooses identity,
   stats, supported actions, supported traits, speed, and targeting. Character,
   class, weapon, spell, and other packages follow their own composition
   patterns.
3. **Runtime types own behavior.** Packages such as `combat`, `character`,
   `conditions`, `features`, `monster`, and `monstertraits` implement rules and
   subscribe to or publish on the event bus.
4. **Data is the persistence boundary.** Stateful runtime objects expose
   `ToData` (or `ToJSON` for polymorphic conditions/features); rulebook loaders
   reconstruct behavior and subscriptions. The host persists the data but does
   not interpret it.
5. **Composition is above rule resolution.** The top-level `encounter` module
   currently composes D&D 5e monster decisions, combat resolution, spatial
   state, and encounter events. It is D&D-5e-coupled today.

Loading has two halves, and they are separately callable: `character.Load` and
`monstertraits.LoadMonster` turn data into a sheet with no event bus involved,
and `character.Attach` / `monstertraits.AttachMonster` put that sheet on a bus
— its own keeper first, then each persisted effect through a bus scoped to that
effect's ref. `Load(d).ToData()` is the data it was given.

The older single-step loaders remain for their existing callers.
`character.LoadFromData` is the two halves in one call, still forgiving about a
blob it cannot read where `Load` refuses and names it. Monster actions are inert definitions loaded directly by `monster.Load`.
`monstertraits.LoadMonster` plus `AttachMonster` composes the same sheet with
persisted trait behavior; there is no action behavior loader.

## Current package map

The module contains many packages. Start with the nearest package rather than
trying to learn all of them.

| Area | Current packages | What they own |
|---|---|---|
| Boundary vocabulary | `refs`, `abilities`, `damage`, `skills`, `languages`, `weapons`, `armor` | Typed identifiers and rulebook vocabulary |
| Characters | `character`, `character/choices`, `class`, `classes`, `race`, `races`, `backgrounds`, `packs`, `equipment` | Character authoring, grants, equipment, runtime state, and persistence |
| Rules | `combat`, `combat/actions`, `combatabilities`, `checks`, `saves`, `initiative`, `features`, `conditions`, `fightingstyles`, `resources`, `spells` | D&D-specific data contracts, resolution rules, and behavior |
| Monsters | `monster`, `monster/monsters`, `monstertraits` | Runtime/data model, direct definitions, built-in factories/registry, and trait behaviors |
| Integration/composition helpers | `events`, `gamectx`, `dungeon`, `integration` | D&D event vocabulary, runtime lookup context, dungeon content, and end-to-end rule tests |

The root `dnd5e` package is a small facade containing aliases for selected race,
class, and shared character-creation types. It is **not** a facade for the whole
rulebook. Import the owning subpackage directly.

There is no `rulebooks/dnd5e/monsters` package today. Built-in content currently
lives at `rulebooks/dnd5e/monster/monsters` inside this same module. A possible
cleanup is recorded as a proposed follow-up in the
[nearest monster README](monster/README.md); current instructions do not depend
on it.

## Install

Because this repository is multi-module, install this module directly:

```bash
go get github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e@latest
```

Import the package that owns the behavior:

```go
import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

constructor, ok := monsters.ByRef(refs.Monsters.Bandit().String())
```

No root toolkit package needs to be installed, and this directory must not gain
another `go.mod` for each content family.

## Contributor routes

- [Add a D&D 5e monster](../../docs/how-to/add-a-dnd5e-monster.md)
- [Monster package guide](monster/README.md)
- [Add a mechanic](../../docs/how-to/add-a-mechanic.md)
- [Add another rulebook entry](../../docs/how-to/add-a-rulebook-entry.md)
- [Rulebook architecture component](../../docs/architecture/components/rulebook-dnd5e.md)
- [Data model and round trips](../../docs/architecture/data-model.md)
- [Run tests](../../docs/how-to/run-tests.md)

## Validate this module

From `rulebooks/dnd5e`:

```bash
go test -race ./...
golangci-lint run ./...
```

A content contribution should also run the focused commands in its nearest
how-to. The code and tests are the final behavior truth; architecture docs,
plans, and examples can lag.
