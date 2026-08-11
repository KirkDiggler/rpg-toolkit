# RPG Toolkit

RPG Toolkit is a collection of independently versioned Go modules for building
RPG rules engines and game hosts. It contains reusable foundations and tools, a
D&D 5e rulebook, and a currently D&D-5e-coupled encounter SDK. The toolkit owns
game rules; a host owns storage, transport, and request orchestration.

## Start here

Choose the shortest route for the work you are doing:

- **Add D&D 5e content:** read the [D&D 5e rulebook guide](rulebooks/dnd5e/README.md),
  then follow [Add a D&D 5e monster](docs/how-to/add-a-dnd5e-monster.md).
  A simple, supported monster is the recommended first contribution.
- **Use a module from Go:** see [Install and use](#install-and-use), then read the
  README or package documentation nearest that module.
- **Change a mechanic:** read [How to add a mechanic](docs/how-to/add-a-mechanic.md)
  and the relevant code and tests.
- **Understand the architecture:** start with the
  [architecture overview](docs/architecture/overview.md), then read only the
  linked [ADRs](docs/adr/README.md) and [journey notes](docs/journey/README.md)
  relevant to the change.
- **Contribute to the repository:** see [Documentation and contributor
  navigation](docs/README.md) and [How to run tests](docs/how-to/run-tests.md).

Current behavior is defined by code and tests. ADRs record decisions; journey
notes record rationale and history. Plans and ideas may describe APIs that have
not shipped.

## Architecture and module map

The repository has no root `go.mod`. It currently contains 22 Go module roots,
each with its own dependency/version boundary and module-prefixed Git tags.
Each module has its own test command, although some packages/modules currently
contain no test files.
Dependency direction is generally **Core → Mechanics / Play primitives → Tools
→ Rulebooks**; higher layers may import lower ones, not the reverse.

| Layer | Current modules | Responsibility |
|---|---|---|
| Core | `core`, `dice`, `events`, `game`, `items`, `rpgerr` | IDs and refs, actions, dice, event chains, shared game context, base item contracts, errors |
| Mechanics | `mechanics/conditions`, `effects`, `features`, `proficiency`, `resources`, `spells` | Rule-agnostic mechanic building blocks |
| Play primitives | `play/clock`, `intel`, `interrupt`, `record` | Small reusable time, knowledge, interruption, and record contracts; these currently depend only on Core |
| Tools | `tools/environments`, `selectables`, `spatial`, `spawn` | Environment graphs, weighted selection, positioning, and placement |
| Rulebooks | `rulebooks/dnd5e` | D&D 5e content and rules, including characters, combat, monsters, conditions, and refs |
| Current composition | `encounter` | Encounter aggregate and host-facing composition; it imports `rulebooks/dnd5e` today and is not rulebook-agnostic |

The top-level `behavior/` and `spawn/` directories contain package-design stubs,
not additional Go modules or usable implementations. Use the module map in the
[architecture overview](docs/architecture/overview.md) for current seams and
clearly labelled migration plans.

## Install and use

Install the module you need, not the repository root. For example:

```bash
go get github.com/KirkDiggler/rpg-toolkit/core@latest
go get github.com/KirkDiggler/rpg-toolkit/events@latest
go get github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e@latest
```

Then import the exact package:

```go
import (
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
    "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

constructor, ok := monsters.ByRef(refs.Monsters.Bandit().String())
if !ok {
    // handle an unsupported content ref
}
bandit := constructor("bandit-1")
data := bandit.ToData() // serializable rulebook-owned data; your host stores it
```

`@latest` asks Go for the latest published version of that module. Releases are
tagged by module path in this repository (for example,
`rulebooks/dnd5e/v0.72.0`), while Go source imports remain the module paths shown
above. Do not add a local `replace` directive to committed code.

For development, clone the repository and run commands from the module being
changed:

```bash
git clone https://github.com/KirkDiggler/rpg-toolkit.git
cd rpg-toolkit/rulebooks/dnd5e
go test -race ./...
golangci-lint run ./...
```

See [How to run tests](docs/how-to/run-tests.md) for repository-wide commands.

## Rulebook contribution model

A rulebook contribution is not “enter a stat block and assume every sentence
works.” It composes content from capabilities the rulebook already implements.
When a creature clause needs new rules behavior, stop and scope that mechanic
separately rather than silently dropping the clause or describing proposed
behavior as shipped.

The monster guide makes that contract concrete:

1. establish an allowed source and attribution;
2. compare every clause with supported action, trait, targeting, and persistence
   behavior;
3. add the ref, factory, registry entry, and tests;
4. prove construction and the applicable `ToData` / load / `ToData` round trip.

**Continue:** [Add a D&D 5e monster →](docs/how-to/add-a-dnd5e-monster.md)

## Documentation

- [Documentation index](docs/README.md)
- [Current status](docs/status.md) and [quality scorecard](docs/quality.md)
- [Architecture overview](docs/architecture/overview.md) and
  [data model](docs/architecture/data-model.md)
- [Architecture Decision Records](docs/adr/README.md)
- [Journey notes](docs/journey/README.md)
- [Historical plans](docs/plans/) and [design ideas](docs/ideas/) — context only;
  verify status banners and current code before following them

## License

Code is licensed under GNU GPL v3.0; see [LICENSE](LICENSE). Third-party or
adapted game content may also require source-specific attribution. The monster
contribution guide defines the minimum provenance gate for new content.
