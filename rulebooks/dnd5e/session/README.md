# session

**Charter: the game server's single point of contact with the toolkit.**

A host implements a couple of repositories and, from then on, holds no domain
object at all — it names things and calls verbs. Everything the toolkit knows
about running a session reaches it through this package.

## What it is

A composer of composers. The encounter composition is *the world* — geometry,
placement, perception, story, endings. This is *the table*: it holds what is not
the world, wires the pieces together, and presents one verb surface.

It owns wiring and lifetime. It owns no rules. **The moment a rule lives here
rather than in the module that owns that rule, this package has been
misused** — and the fix is to push it down, not to special-case it up.

## What it is not

- **Storage.** The host owns that, behind ports.
- **Authoring.** Worlds and monster definitions come from content pipelines.
- **Transport.** gRPC, protos, and streams belong to the game server.
- **Presentation.** It returns facts rich enough to narrate; it never narrates.

## Why it exists

So the toolkit's insides can be replaced without the host changing a line. Inner
types never cross the boundary, which means a module underneath can change shape
— or be swapped out entirely — while the host only ever bumps a version.

That promise is enforced rather than intended: `boundary_test.go` parses this
package's own source and fails if any toolkit type is reachable from an exported
declaration, and `gorelease` gates every release.

## Contents

| File | Role |
|---|---|
| `session.go` | `Config`, `Manager`, and total construction |
| `ports.go` | What the host implements, and the constraints on it |
| `data.go` | What this package persists |
| `types.go` | What this package returns |
| `convert.go` | The converter layer: inner shapes → ours |
| `read.go` | `Atlas`, `Status`, `View`, `Story`, and the shared load path |
| `write.go` | `Join`, `Exit`, `End`, and the save/publish seam |
| `move.go` | `Move` (walking a path) and `Traverse` |
| `start.go` | `StartSession` |
| `events.go` | Per-recipient fan-out |
| `cmd/session-workbench` | Drives a whole session; run it |

## Reading it

Start with `doc.go` for the laws, then `example_session_test.go` — the
acceptance story, executable, with a transcript `go test` re-proves.

```
go run ./cmd/session-workbench
./scripts/verify.sh rulebooks/dnd5e/session
```

## Deliberately not here

This file carries the **charter**, which barely changes. What is *currently
implemented* lives in `doc.go` next to the code, and what is *verified* lives in
the tests. A README that tracks progress is a README that lies, and it lies
quietly — the failure that motivated writing this one.
