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
- **R9** — `Holding.CurrentVia` is intel's own per-channel list, retyped
  and sorted as intel sorts it, and `CurrentOn(channel)` is how it is asked.
  `Observed` and `Confirmed` pass through unchanged. intel's derived
  `Status` is deliberately **not** carried over: it answers "is anything at
  all delivering this", which is almost never the question a caller means.

  v0.1.0 shipped a `Current bool` here instead, and it was wrong in a way
  worth recording rather than quietly fixing. With one channel in existence
  the bool read as "currently delivered" and every consumer meant *sight*;
  seven call sites across `encounter`, `session` and `resolution` were
  correct only because a subject id from another channel would fail some
  unrelated roster lookup. Correct by accident is not correct. Worse,
  `session/convert.go` had to reconstruct a per-channel answer it no longer
  had, pairing the bool with `Holding.Channel` — and since `Report` moves
  `Channel` without sustaining anything, that pair can say "current via
  deeds" about a holding only sight is delivering. Removing the bool rather
  than keeping it beside `CurrentVia` is the point: a caller now has to name
  the channel it means, and cannot fall back to the fudge.

  This is also where the "callers never see intel" charter admits its one
  exception: `Load`'s errors wrap `intel.ErrInvalidData`, and `Data.Intel`
  is `intel.Data` verbatim — persistence is intel's shape, stated rather
  than hidden.
- **R10** — One payload per `(observer, subject)`. The store has exactly one
  slot, so a subject landed on a second channel **overwrites** the first
  channel's payload and the loss is silent. `CurrentVia` can hold two
  channels at once; the payload cannot. This is deliberately not refused:
  the multi-channel shape is still open, and a refusal would wall off a
  design before the use cases have finished arguing for one. The eventual
  fix is intel keying by `(channel, subject)`, at which point R11 stops
  being a discipline and becomes the store's own shape.
- **R11** — `Presence.ID` is the unit of identity this package will not look
  past. A caller that emits one figure under the same id on two channels has
  **decided those are one thing**, and no observer gets to be wrong about
  it. Merging is the observer's judgment — the whole reason a mind has one —
  so a caller perceiving a figure on a second channel qualifies the id by
  channel (`hearing|goblin`), and the store then cannot merge them. R10 is
  what happens when this goes unheeded. Stated as an instruction rather than
  a warning: qualification is the thing to do, not merely the hazard to
  avoid.

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
| `Report` | `ReportInput` | `(*ReportOutput, error)` | Discrete testimony to exactly one observer (wraps `intel.Report`). Lands **held and sustaining nothing**: a reported subject is current on no channel, including the one that reported it. Retires nothing either — a report makes no claim about what the observer was *not* told, so it cannot fade a holding the way a complete percept does. Validates in `Observe`'s order: `ErrNoChannel`, `ErrNoSubject`, `ErrNoObserver`. Repeated subjects are **not** rejected (see below). |

The two verbs differ in what they *claim*. A `Pass` is a complete statement
about one channel at one moment — everything delivered, and by omission
everything no longer delivered, which is what lets a holding fade. A
`Report` is one observer being told something; it asserts only what it
carries.

That difference is also why `Report` does not reject a repeated subject when
`Observe` does. `Observe`'s rejection (R6) exists because sorting a `Pass`
makes intel's last-wins dedupe depend on an unstable sort. `Report` does not
sort, so last-wins is already deterministic, and rejecting would be
strictness with nothing behind it.

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
| `ErrNoChannel` | empty `Pass.Channel` or `ReportInput.Channel` | `Observe`, `Report` |
| `ErrNoSubject` | empty `Presence.ID`, or an empty subject passed to `On` | `Observe`, `Report`, `On` |
| `ErrNoObserver` | empty observer ID, in a `Pass`, a `ReportInput`, or passed directly | `Observe`, `Report`, `Held`, `On` |
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
- `TestReportLandsHeldAndSustainsNothing` proves a reported subject is
  current on nothing, including the channel that reported it.
- `TestReportedSubjectIsNotFadedByALaterPass` proves a `Pass` on another
  channel cannot retire reported testimony — the defect the deeds-through-
  `Observe` workaround produced.
- `TestReportMovesProvenanceWithoutSustainingItsChannel` proves R9's whole
  point: after a report about a currently-sighted subject, `Channel` and
  `CurrentVia` disagree, and only `CurrentVia` is true. Fails if `Holding`
  reports currency as anything a caller can read without naming a channel.
- `TestTwoChannelsSustainOneSubject` and
  `TestLosingOneChannelOfTwoIsNotAFade` prove `CurrentVia` carries two live
  channels and that losing one is not a fade — neither expressible under
  v0.1.0's bool.
- `TestReportOverwritesAnUnqualifiedSubject` and
  `TestQualifiedIDsKeepBothChannelsIntact` prove R10's cost and R11's
  remedy as a matched pair.
- `TestReportValidationOrderAndNothingWritten` proves `Report`'s ordering
  with every violation present at once, and that a rejected report writes
  nothing.
- `gofmt`, `go vet`, and `golangci-lint` (CI's pinned version) all clean;
  `go test -race` clean.
