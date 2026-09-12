# mind/perception — Design (the WHAT)

**Status:** IMPLEMENTED
**Module:** `github.com/KirkDiggler/rpg-toolkit/mind/perception` (package `perception`)
**Why:** [rpg-toolkit#1685](https://github.com/KirkDiggler/rpg-toolkit/issues/1685).
**How:** implemented directly against the issue's rulings in [PR #1686](https://github.com/KirkDiggler/rpg-toolkit/pull/1686),
then revised during that PR's independent review — no separate `plan.md`
(see the note in [implementation.md](implementation.md)).
**What happened:** [implementation.md](implementation.md).

## Scope

`encounter.rebuildPercepts` joined four per-pass maps by hand inside a
nested loop and called `EncodeSightTestimony` N² times for N members — once
per (observer, subject) pair, even though the payload is identical for
every observer who can see that member. Perception moves the loop and
leaves the geometry behind: a caller assembles **one payload per member,
once**, and hands over a pass.

Perception is what an observer holds: channel-sourced testimony that may be
false and may be stale. It never sees the world. It is handed what is true
as values and cannot ask a question of its own, which is what lets it hold
a lie. `play/intel` is its store; callers never see it, except in
persistence (R9's documented exception).

**Non-goals:** `Report` (discrete testimony) — `Observe` only wraps
`Surveil`; a second channel per pass — one `Pass` is one channel; forgery,
contacts, or naming of any kind; geometry, range, blocking, lighting or
cover — all of it collapses into the caller-owned `Reach` interface;
encounter adoption — a separate PR, not part of this module. Each of these
arrives with the use case that pays for it, not before.

## Rules

- **R1** — One payload per presence. It is encoded by the caller before the
  pass and says the same thing to every observer who perceives it.
  Perception never decodes it.
- **R2** — For each observer in `Pass.Observers`, build a percept from every
  presence where `Reach.Reaches(pass.Channel, observer, presence.ID)` is
  true, then land it as one complete percept via `intel.Surveil`.
- **R3** — An observer never perceives itself. `presence.ID == observer` is
  excluded before `Reach` is ever consulted.
- **R4** — An observer whose reach finds nothing still gets a complete
  empty percept, so everything it held fades. Skipping that call is how
  ghosts stay falsely current forever.
- **R5** — An observer not in `Pass.Observers` is not touched at all.
  Nothing fades for them.
- **R6** — Presences are processed in sorted `ID` order, and every `Delta`
  slice is sorted, so two identical passes produce identical output.
  Determinism depends on the sorted presences forming a **set**: a
  duplicate `Presence` ID (or a duplicate `Observer`) is a caller bug and
  is rejected before any mutation (`ErrDuplicateSubject` /
  `ErrDuplicateObserver`) rather than left to `slices.SortFunc`'s
  instability and intel's last-wins dedupe to resolve by accident of input
  order.
- **R7** — `Changed` is `intel.SurveilOutput.Changed` (play/intel v0.4.0),
  consumed directly rather than derived. The original ruling derived it by
  comparing a holding's `Observed` to `pass.At` after landing; that
  comparison is only correct while `At` strictly increases, and a caller
  reusing (or never setting) `At` made every refresh read as a change.
  Intel already computes the payload comparison at landing time to decide
  whether `Observed` moves, and v0.4.0 reports that comparison instead of
  discarding it — so this package now consumes an answer instead of
  reconstructing one from stamps, and the `At`-reuse trap is closed by
  construction, not by documentation.
- **R8** — Validation before any mutation, in this order: nil `Reach` →
  `ErrNoReach`; empty `Channel` → `ErrNoChannel`; empty `ID` on any
  `Presence` → `ErrNoSubject`; empty `ID` on any `Observer` →
  `ErrNoObserver`; a duplicate `Presence` ID → `ErrDuplicateSubject`; a
  duplicate `Observer` → `ErrDuplicateObserver`. Every empty-ID check
  completes across the whole `Pass` before either duplicate check begins,
  so which violation is reported never depends on where in either slice it
  sits.
- **R9** — `Holding.Current` maps from intel's `Status == Current`.
  `Observed` and `Confirmed` pass through unchanged. This is also where the
  "callers never see intel" charter admits its one exception: `Load`'s
  errors wrap `intel.ErrInvalidData`, and `Data.Intel` is `intel.Data`
  verbatim — persistence is intel's shape, stated rather than hidden.

## Types

- `Channel` — open string vocabulary, mirroring `intel.Channel`. `Sight` is
  predeclared.
- `Presence` — `{ID core.EntityID, Payload []byte}`. One thing perceivable
  this pass; the payload is opaque (R1).
- `Reach` — `interface { Reaches(channel Channel, observer, subject
  core.EntityID) bool }`. The whole of the physics; this package supplies
  none of it.
- `Pass` — `{At uint64, Channel Channel, Presences []Presence, Observers
  []core.EntityID, Reach Reach}`. One complete perception cycle on one
  channel.
- `Holding` — `{Subject core.EntityID, Payload []byte, Channel Channel,
  Observed uint64, Confirmed uint64, Current bool}`. What one observer
  holds about one subject (R9).
- `Delta` — `{FirstContact []Presence, Refreshed []core.EntityID, Changed
  []core.EntityID, Faded []core.EntityID, Reacquired []core.EntityID}`.
  What one pass did to one observer's knowledge, matching
  `intel.SurveilOutput` field-for-field: the lists answer different
  questions and are not a partition of the percept. `Faded` means "no
  longer current via **any** channel" (intel's actual semantics), not
  merely absent from this one pass — identical today with a single
  channel, and stated this way so the doc doesn't mislead the day a second
  channel arrives. `Changed` and `Reacquired` each refine `Refreshed`
  rather than carve subjects out of it, for the same reason intel's own
  fields do.
- `Perception` — the container. Zero value not usable; construct via `New`
  or `Load`. Backed by one `*intel.Intel`.

## Verbs

| Verb | Input | Output | Semantics |
|------|-------|--------|-----------|
| `Observe` | `Pass` | `(map[core.EntityID]*Delta, error)` | R2–R8. One `intel.Surveil` call per observer in `Pass.Observers`, on a percept built from `Pass.Presences` filtered by `Pass.Reach` and sorted by ID (R6). Returns one `Delta` per observer, keyed by observer ID. |

## Queries

| Query | Signature | Returns | Semantics |
|-------|-----------|---------|-----------|
| `Held` | `Held(observer core.EntityID)` | `([]Holding, error)` | Everything the observer holds, sorted by subject (wraps `intel.HeldBy`). `ErrNoObserver` on an empty ID. An observer with no holdings answers an empty slice, nil error. |
| `On` | `On(observer, subject core.EntityID)` | `(Holding, error)` | One observer's holding on one subject (wraps `intel.On`). `ErrNoObserver` / `ErrNoSubject` on an empty ID; `ErrNotHeld` when nothing is held — intel's own not-held error is translated to this package's sentinel rather than passed through, so a caller never has to import `play/intel` to ask the single most common question this package answers. |

Neither query takes an `*XxxInput` struct: both are fixed-arity reads with
no plausible parameter growth, unlike intel's testimony verbs, which take
variable-length report lists. Considered and declined during PR #1686
review; recorded here so it reads as a decision, not an oversight.

## Errors

All errors wrap one sentinel; `errors.Is` dispatch; messages user-facing.

| Sentinel | Meaning | Returned by |
|----------|---------|-------------|
| `ErrNoReach` | nil `Pass.Reach` | `Observe` |
| `ErrNoChannel` | empty `Pass.Channel` | `Observe` |
| `ErrNoSubject` | empty `Presence.ID`, or an empty subject passed to `On` | `Observe`, `On` |
| `ErrNoObserver` | empty observer ID, in a `Pass` or passed directly | `Observe`, `Held`, `On` |
| `ErrDuplicateSubject` | two `Presence`s in one `Pass` sharing an ID | `Observe` |
| `ErrDuplicateObserver` | the same observer named twice in one `Pass`'s `Observers` | `Observe` |
| `ErrNotHeld` | the observer holds nothing on that subject (translated from `intel.ErrNotHeld`) | `On` |

## Persistence

`Data` wraps intel's own `Data` verbatim (`{Intel intel.Data}`, no
`omitempty` — a struct field never omits) — perception adds no state of its
own, so the JSON shape is free. `Load` rejects whatever `intel.LoadIntel`
rejects, wrapped, not translated: unlike `On`'s `ErrNotHeld`, an invalid
persisted shape is not routine control flow a caller dispatches on, so
there is nothing here worth hiding intel behind.

## Acceptance criteria

- `go test ./...` passes from `mind/perception`.
- Case 2 (`TestRefreshedWithoutChangeIsNotChanged`) fails if R7 is changed
  to put every `Refreshed` subject in `Changed`.
- Case 7 (`TestObserverReachingNothingFadesEverything`) fails if R4 is
  changed to skip observers whose built percept is empty.
- `TestSameAtNeverForcesChanged` proves the `At`-reuse trap R7's revision
  closed: three passes sharing the same `At` and an identical payload never
  report `Changed`.
- `TestValidationOrderAndNothingWritten` proves the full R8 ordering,
  including that a duplicate violation never wins over an empty-ID
  violation regardless of position, and that every rejected `Pass` writes
  nothing.
- Every sentinel above is `errors.Is`-tested from a call that returns it,
  including `Load`'s and the two duplicate-ID sentinels.
- `gofmt`, `go vet`, and `golangci-lint` (CI's pinned version) all clean;
  `go test -race` clean.
