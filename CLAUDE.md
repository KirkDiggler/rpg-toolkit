# RPG Toolkit

rpg-toolkit is the rules engine for D&D 5e (and future rulebooks). It implements
game rules and returns rich breakdowns. Consumed by rpg-api (the game server) and
any other rulebook host. The toolkit never orchestrates data, never persists state,
and never imports rpg-api or rpg-api-protos.

`AGENTS.md` is a symlink to this file so every agent runtime boots from the
same instructions.

## Where things live

- `docs/architecture/overview.md` — layer rules (Core → Events → Mechanics → Tools → Rulebooks), module map, boundary with rpg-api, named violations
- `docs/architecture/data-model.md` — ToData/LoadFromData pattern, entity shapes, identifier constants, condition/feature serialization, chain/breakdown output
- `docs/architecture/components/` — one doc per major module, plus focused sub-module docs (`rulebook-dnd5e-session`, `rulebook-dnd5e-character`)
- `docs/status.md` — current health: active work, paused items, known rough edges, per-subsystem confidence
- `docs/quality.md` — A-D scorecard with rationale per module
- `docs/adr/` — architectural decisions (32 ADRs). New decisions add new ADRs; superseded ones stay with a "Superseded by ADR-NNN" note. Never archive an ADR.
- `docs/journey/` — exploration narratives (49 docs). How the engine got to where it is. Future contributors learn the engine from these. Do not archive.
- `docs/plans/` — design explorations for specific features (10 plans). Historical; some are implemented, some stale.
- `docs/ideas/` — toolkit-scoped idea records. Each active idea keeps `design.md` and `plan.md` open through implementation, then adds `implementation.md` with observed results before the idea PR merges.
- `docs/how-to/` — task guides: run-tests (commands, the testify suite pattern, pre-commit gates), add-a-mechanic, add-a-rulebook-entry, fix-go-mod-replace-directives, verified-transcripts (show a module working as designed: `./scripts/verify.sh <module>`)
- `docs/archive/` — genuine archive: pre-Dec-2025 design docs, diagrams, guides that no longer reflect current architecture. Read for historical context only.

## How live play is layered

Three kinds of package. The difference between them is a real contract, not a
naming convention — and knowing which kind you are in answers most "where does
this go?" questions on its own.

- **`play/*` — composable leaves.** One concern each, deliberately ignorant of
  everything else. Each depends only on `core`, takes no `context.Context`,
  **returns its results as values and never publishes**, and is bound by a
  numbered contract (R1–R10) in `docs/ideas/play/<name>/design.md`. They never
  interpret what they hold: the ledger does not read the payload, the intel
  store cannot tell a lie from the truth, the clock holds no rules and no
  randomness. See `play/README.md`.

- **A composition.** The **courier** between leaves, and the first layer allowed
  to have an opinion about the game. Rules and trigger detection belong here.
  Each carries its own numbered laws.

- **A host seam.** The one interface a game server implements: verbs take IDs,
  repositories are key-value, no runtime object crosses the boundary. It owns
  **no** rules.

Which packages currently play which part is the rulebook's own business — see
that rulebook's `CLAUDE.md` (for D&D 5e, `rulebooks/dnd5e/CLAUDE.md`).

The direction that matters: **each layer absorbs wiring so the layer beneath it
does not have to.** The seam spares the host the composition's complexity; the
composition spares the leaves the bus, the persistence, and each other. A
rule that appears in `play/` or in a seam is a layering bug, not a shortcut.

**Read the package's godoc before designing anything in this area** — every one
of these states its own contract in `doc.go`, names the laws that bind it, and
points at its design doc. That is the discoverable surface; this section only
tells you the shape so you know to go looking.

## How We Ship — Versions, Tags, and Freezes

This workspace moves fast pre-v1.0, and the version system is what makes that
safe. Internalize this model before reasoning about releases, freezes, or
"breaking" changes.

### Pull requests follow release-unit boundaries

These are hard rules, including when several modules change for one feature:

1. **One nearest-`go.mod` module per PR.** CI builds and tags each Go module as
   an independent release unit, so a PR must contain changes for at most one
   module as determined by the nearest `go.mod`. Repository-level docs may
   accompany the relevant module PR or ship as a docs-only PR.
2. **Nested modules are separate PRs.** A nested module has its own `go.mod` and
   therefore gets its own PR even when its changes implement the same feature
   as its parent or sibling modules.
3. **Open a draft on the first working push.** Push meaningful checkpoints,
   create the draft immediately, and update its description or comments as
   checkpoints and validation evidence accumulate.
4. **Distinguish review readiness from merge readiness.** A draft exposes work
   in progress. Mark it ready for review when its declared scope is implemented
   and applicable checks are green; required review and release prerequisites
   must be satisfied before declaring it merge-ready. Neither state authorizes
   an automatic merge. Gates must not delay the first working draft.
5. **Publish providers before consumers.** Merge the provider, wait for CI to
   mint its real module tag, then update the consumer's committed pin to that
   tag. The consumer must not merge before that pin update.

The version model behind those rules is:

1. **A tag can't break you. Only a bump you choose can.** Every module pins its
   dependencies by exact version in its own `go.mod`. A merged change — even a
   breaking one — does not exist for any consumer's build until that consumer
   runs `go get` and opts in. That is why APIs change freely here: the pin
   system, not caution, is the safety mechanism.

2. **Toolkit is main-only, permanently.** There is no `dev` branch and never
   will be: the toolkit's staging *is* its versions. (rpg-api and rpg-dnd5e-web
   have `dev` branches because *deployments* need staging.) A breaking toolkit
   merge leaves every consumer pinned exactly where it was.

3. **Merges mint tags — nobody hand-tags.** CI tags each changed module
   automatically when a PR merges to main. Never cut a tag by hand, and never
   from a side branch: a hand-cut tag can reference API that no released
   dependency has, which makes it unbuildable for everyone but you. Let the
   merge flow mint tags and "latest of everything builds together" stays true.

4. **Develop outside-in on pseudo-versions. Publish inside-out, and publish
   LAST.**

   These are two phases, and collapsing them is the mistake this point exists
   to prevent. **Merging is never a development step. Nothing merges in order
   to unblock anything.**

   *While the wave is being built:* a consumer pins its provider's **pushed
   commit**. `go get <module>@<sha>` mints a real, resolvable pseudo-version —
   that is not a hand-rolled `replace` and is not the thing banned below. The
   whole stack builds and runs from those pins, locally, and gets **walked**.
   The walk is where cross-repo findings surface, and surfacing them before
   anything is tagged is the entire reason to develop outside-in.

   *Only once it is proven:* merge the innermost module, let CI mint its tag,
   `go get` that tag in the consumer, merge the consumer, repeat outward.

   **The tell that you have collapsed the two** is the sentence *"X can't start
   until Y merges and CI mints the tag."* For a Go module in this repo that is
   almost always false, and reaching for a merge to make progress is the same
   error wearing a different sentence.

   **Protos are the one real exception**, because `gen/` is gitignored there,
   so a Go consumer cannot build from a protos branch at all. They are also
   the cheapest thing to get wrong and correct — a versioned contract and
   nothing else — which is why they merge first and why there is deliberately
   no protos manifest variable.

5. **A freeze protects files, not versions.** Tags elsewhere cannot affect
   in-flight work (point 1), so there is no reason to pause tagging repo-wide.
   What can collide is concurrent edits to the same module's files — if you
   need protection for an in-flight slice, ask on the owning issue for a
   freeze scoped to exactly the modules your branch reworks, and post there
   to release it when you merge.

## Module Development Workflow

**IMPORTANT: LOCAL OVERRIDES ARE FINE — THEY MUST NEVER REACH CI**

Local `replace` directives and `go.work` files are a normal part of developing
across modules here. Working outside-in (build the consumer against a local
sibling, discover the contract, then merge inside-out) depends on them.

The failure that shaped this rule is overrides being *committed* and breaking
CI. That is the actual failure, and that is what stays banned.

1. **Override locally, publish before you merge**
   - Use `replace` or `go.work` freely while developing across modules
   - **Never commit them.** A `replace` pointing at a local path fails CI, and
     it fails it for everyone, not just you
   - Before merging: merge the dependency so CI mints its tag, then point at
     the minted version and remove the override

2. **Dependency Management**
   - Committed `go.mod` files reference published versions (e.g., `v0.1.0`)
   - To take an update from another module: merge that module's change to
     main first (CI mints its tag), then `go get` the minted version in the
     dependent module
   - Go creates pseudo-versions automatically for un-tagged commits

3. **Why This Shape**
   - Local overrides make cross-module work possible without a release per edit
   - Requiring published versions *at merge* keeps the committed graph honest:
     what CI builds is what a consumer would get
   - Editing a sibling module on disk does **not** change what your module
     compiles against unless you have an override in place — the most common
     source of "I fixed it but nothing changed"

## Laws

**Development principles**

- **Optimize for simplicity, not hypothetical future needs**
- **Only add what is necessary**
- **Pick ONE way to represent data and use it directly**
- **Avoid conversion layers and dual representations**
- **Delete code that creates unnecessary indirection**

**Toolkit is infrastructure, not implementation**

1. **Generic tools, not game rules** — we provide the infrastructure for game
   mechanics; games implement their specific rules using our tools. We provide
   proficiency infrastructure; the game defines what "Acrobatics" means.
2. **Events observe, values decide** — typed topics (`events.DefineTypedTopic`,
   the `.On(bus)` pattern) carry game occurrences to whoever subscribed; rules
   and observers react there. Results still **return as values**: `play/*`
   leaves never publish, and spatial publication is observer-only. See "How
   live play is layered" above for which layer owns the bus.
3. **Entity-based design** — all game objects implement `core.Entity` (ID and
   Type), giving consistent patterns across the toolkit.

**Patterns**

- Config structs for constructors; embedded structs and interfaces over
  inheritance
- Event names use dot notation (e.g., "resource.consumed", "condition.applied")
- **Identifier constants: toolkit is the source of truth for game-mechanics
  identifiers, and rpg-api is a pure translator** — proto enum in, typed
  constant, toolkit validates and returns, server passes the result through
  unchanged. Toolkit validates everything; its error messages are user-facing.
  The pattern and its law live in `docs/architecture/data-model.md`.
- **Condition/feature serialization: rpg-api stores opaque JSON blobs; toolkit
  marshals them into strongly-typed data structs** — runtime structs carry no
  JSON tags; the loader routes by `ref.Value`. The full pattern lives in
  `docs/architecture/data-model.md`.
- **Every public function, type, constant, and variable carries a comment**
  naming purpose, behavior, and error cases. No empty function bodies; no
  functions that only return nil; follow existing toolkit patterns. CI
  enforces this.
- **Context discipline** — standard `context.Context` only where cancellation,
  timeouts, or request-scoped values are genuinely needed; remove unused
  context parameters. `play/*` packages take no `context.Context` at all, by
  contract. Game data flows through typed topics and returned values, not
  through a general-purpose context bag.

**Module isolation**

1. **Never touch other modules when working on a specific module.** Other
   modules are read-only for reference; their issues get separate PRs.
2. **Check the current directory before running go commands.** Bulk operations
   across all modules corrupt dependencies — a stray `go mod tidy` in the
   wrong module breaks everyone.
3. **When CI fails, check what files actually changed first.** It is usually
   code conflicts or accidentally committed files, not CI configuration.
4. **A directory without `go.mod` is treated as part of the root workspace**
   and can conflict types with existing modules. Every module has a proper
   `go.mod` or is removed entirely.

**Tests**

- Test organization uses the testify suite pattern; the shape, commands, and
  pre-commit gates live in `docs/how-to/run-tests.md`.
- Check errors in tests with `s.Require().NoError(err)` / `require.NoError(t, err)`.

## Workspace discipline

- This repository participates in `KirkDiggler/rpg-project`. Toolkit owns game
  mechanics and projections; API owns mapping, authorization, and orchestration;
  web owns interaction and rendering; protos owns wire shape and generated SDKs.
  Report adjacent work before taking it on. Inspect existing contracts before
  requesting new fields. **Never repair a missing provider projection by
  loosening validation or reconstructing rules in a consumer.**
- For this user's work, correctness and controlled sequencing take priority
  over speed: advance one PR at a time, wait for the provider to merge and CI
  to publish its actual module tag, then update and verify the next consumer
  against that release. No temporary dependency versions, no parallel dependent
  PR stacks to accelerate delivery, no rewriting published branch history.
- **Cross-project acceptance evidence:** for a new class or player-facing
  mechanic, trace a normally created, **unseeded** character through
  acquisition, finalization, persistence, private sheet reads, offers,
  execution, results, and reload/rest. Creation and casting tests alone do not
  establish that the player can read their sheet. The character-package
  checklist in `rulebooks/dnd5e/character/CLAUDE.md` covers the provider checks.
  Report evidence by boundary and exact revision: toolkit regression, API
  contract test, and native browser acceptance are separate claims; a green
  provider suite or seeded fixture does not prove native acquisition or
  private-sheet reads. Mark an unrun boundary pending. Record unmerged PRs
  separately from published releases and consumer adoption.
- Include the user before deciding gameplay eligibility, missing-data
  defaults, backward-compatibility behavior, or scope that introduces
  prerequisites in other systems — these are product/rules decisions, not
  routine coding choices. Once decided, record and implement the decision
  without repeated confirmation.

## Where current state lives

Live status is never snapshotted here — it rots. Read instead:

- `docs/status.md` — active work, paused items, rough edges (a living doc,
  updated in the same PR that invalidates a line)
- `docs/quality.md` — per-module A-D scorecard
- the GitHub issues and Project 19 board — what is actually in flight