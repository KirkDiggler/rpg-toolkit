# World NPCs - Implementation

**Executes:** `plan.md`, itself executing `design.md`.
**Issues:** [rpg-toolkit#1404](https://github.com/KirkDiggler/rpg-toolkit/issues/1404) (encounter/session integration), [rpg-toolkit#1434](https://github.com/KirkDiggler/rpg-toolkit/issues/1434) (movement blocking follow-up)
**Status:** SHIPPED (2026-09-03)

## What shipped

A complete, working loop for placing and handling a non-combatant NPC:
build content → place it in a session → it survives combat around it
untouched → a player can interact with it and get back what it is → it
optionally blocks movement → it persists correctly. All seven PRs below are
merged to `main`.

### #1404 — encounter/session integration (5 PRs)

| PR | Module | What it added | Merge commit |
|---|---|---|---|
| [#1411](https://github.com/KirkDiggler/rpg-toolkit/pull/1411) | `rulebooks/dnd5e/encounter` | `KindWorld` member kind — bare facts only, no `npc` import, structurally excluded from combat/turns (no code change needed there — `sidesInContactOrder`'s switch has no `default` case) | `d3f7847f` |
| [#1412](https://github.com/KirkDiggler/rpg-toolkit/pull/1412) | `rulebooks/dnd5e/encounter` | `Interact` verb — confirms identity/adjacency/visibility, no descriptor, no NPC content | `56cb297c` |
| [#1413](https://github.com/KirkDiggler/rpg-toolkit/pull/1413) | `rulebooks/dnd5e/npcs` | `NewMerchant(config *MerchantConfig)` — `nil` returns the toolkit's own demo default, non-nil delegates entirely to the existing `NewVendor` (one validation path) | `234e1fc8` |
| [#1414](https://github.com/KirkDiggler/rpg-toolkit/pull/1414) | `rulebooks/dnd5e/session` | `PlaceNPC` + `SessionData.WorldNPCs` store (caller-supplied `*npc.Data`, not ref-resolved) + `buildTargetPreflight`'s missing kind gate fixed | `67ebb9ad` |
| [#1415](https://github.com/KirkDiggler/rpg-toolkit/pull/1415) | `rulebooks/dnd5e/session` | `Interact` + `WorldNPCDescriptor` (assembled from `WorldNPCs`, `MemberID`/`Ref` projected as `string` per S2) + the full acceptance scene | `ab5c685d` |

### #1434 — movement blocking follow-up (2 PRs)

| PR | Module | What it added | Merge commit |
|---|---|---|---|
| [#1435](https://github.com/KirkDiggler/rpg-toolkit/pull/1435) | `rulebooks/dnd5e/encounter` | `BlocksMovement bool` on `MemberInput`/`JoinInput`/`Member`/`MemberData`, wired into `memberEntity.blocksMovement` (previously hardcoded `false` for every kind) across all three placement paths (`NewEncounter`, `Join`, `LoadEncounter`) | `c957c8b1` |
| [#1436](https://github.com/KirkDiggler/rpg-toolkit/pull/1436) | `rulebooks/dnd5e/session` | `PlaceNPC` computes the bool via `npc.Data.MovementPolicy.BlocksMovement()`; new `ErrBadNPC` for a hand-built, unvalidated `npc.Data` reaching this seam with a malformed policy | `7ff78cd4` |

Final tags reached (each module also picked up unrelated concurrent work from
other branches; these are the versions this work landed at, not a claim that
every intermediate tag in between is one of these seven PRs):
`rulebooks/dnd5e/encounter/v0.48.0`, `rulebooks/dnd5e/session/v0.49.0`,
`rulebooks/dnd5e/v0.129.0` (the `npcs` package's base module). `npc/v0.1.0`
was untouched — this work is entirely a consumer of it.

## Deviations from the plan, and why

The plan you're reading now is not the plan this work started from. Three
real course-corrections happened mid-implementation, each recorded as a
dated amendment in `design.md`/`plan.md` at the time, summarized here for
anyone who doesn't want to read the full amendment trail:

1. **Encounter carries zero NPC content — not the original sketch.** The
   design originally in this doc put `Ref`/`Capabilities`/three policy
   fields directly on `encounter`'s `MemberInput`. Traced how a monster
   actually works (`session.Spawn` never lets `monster.Data` cross into
   `encounter`) and rebuilt the encounter-side model to carry only bare
   facts, matching that exactly. This is the single biggest shape change in
   the whole effort, and it's why `KindWorld`'s own PR (#1411) is small —
   almost everything about "non-combatant" turned out to be free.

2. **No NPC catalog — `npc.Data` is caller-constructed, not ref-resolved.**
   Also modeled on `Spawn` at first (`PlaceNPC` was going to take a bare ref
   and resolve it the way `monsters.ByRef` does). `docs/ideas/dnd5e-npcs/
   design.md` had already ruled this out for vendors specifically (no
   `NewBlacksmith`-style toolkit archetype) before this work started;
   `PlaceNPC` ended up taking already-built `*npc.Data` directly instead —
   closer to how `Join` takes an already-loaded character than how `Spawn`
   resolves a ref.

3. **Disposition (hostile/allied/graded) was scoped out entirely, twice.**
   First amendment proposed modeling it as `world/graph` relations
   (`HostileTo`/`AlliedWith`, proven in `examples/world/scenarios/
   banditcamp`). Second pass concluded that mechanism has no consumer yet —
   `KindWorld`'s neutrality is already free and structural, and a hostile
   NPC would need to answer "what even is a hostile NPC, given `KindWorld`
   doesn't fight by construction" before it's an engineering question at
   all. Left as forward context in `design.md`, not built.

Two smaller, non-architectural deviations:

- `#1434` (movement blocking) shipped as a **separate issue**, not folded
  into `#1404`. `#1404`'s own issue body explicitly listed movement blocking
  as out of scope; extending that issue's scope after the fact would have
  made its own written record self-contradictory. Cheap to split since the
  work was genuinely independent.
- CI's `gorelease-*` checks failed transiently twice (on #1415 and #1435),
  both times because the check ran before the async tagging workflow from
  the *previous* PR's merge had finished minting that module's next tag —
  confirmed by checking the failing PR's branch already contained the prior
  merge commit. Re-running the specific failed job after the tag existed
  resolved both; not a code defect either time.

## What's explicitly still open (not done here, not forgotten)

- **Observation.** `SightFeet` already exists as a bare `encounter` field
  and already gets refreshed for any member kind with no filter —
  `PlaceNPC` just still hardcodes it to `0`. Wiring an observer-capable NPC
  is the same shape as movement blocking and about as cheap; the only real
  gap is a content question (what sight range does "observer" mean), not an
  engineering one. No consumer has asked for it.
- **Disposition, factions, hostile/allied NPCs.** See deviation 3 above.
  Forward context lives in `design.md`'s amendments; building it needs a
  real design pass, not a small follow-up.
- **Vendor stock, quoting, buying, selling.** Entirely `#1275`'s scope. This
  work exposes `npc.CapabilityVendor` and a working placement/interaction
  lane for #1275 to build on; it implements none of the transaction itself.
- **Nothing outside this repo.** `rpg-api`/web consume none of this yet —
  the toolkit can place and handle an NPC; nothing has asked it to in a real
  running game.

## Verification evidence

Every PR above passed `go test -race ./...`, `go vet ./...`,
`golangci-lint run ./...` (0 issues), `gofmt -l .` (clean), and
`go mod tidy` (clean diff) in its own module before merge, plus an
end-to-end acceptance scene (#1415: player interacts with a placed vendor,
a fight forms against a separately-placed monster without the vendor ever
entering it, the vendor stays queryable with an identical descriptor
mid-fight) and direct proof (not inference) that blocking/non-blocking
`MovementPolicy` values genuinely change canvas co-location (#1435, #1436).
