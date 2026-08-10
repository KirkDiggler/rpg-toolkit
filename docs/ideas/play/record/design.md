# play/record — Design (the WHAT)

**Status:** PROPOSED
**Module:** `github.com/KirkDiggler/rpg-toolkit/play/record` (package `record`)
**Why:** `brainstorm.md`. **How:** `plan.md` (after this is approved).

## Scope

The retained story: an append-only, sequence-ordered, audience-projected
log of opaque entries, with per-viewer replay queries and explicit
retention. Storage and query only.

**Non-goals (v1):** delivery/streaming/subscriptions (host wire);
payload interpretation (opaque, composition-encoded); computing audiences
(composition business, typically from intel/stage); correlation
assignment (caller-supplied); ordering by `At` (Seq is the only order;
`At` is provenance); mutation or deletion of individual entries (append
+ trim only — the story is not editable); event-sourcing state rebuilds
(available by composition, imposed on nobody).

## Rules

Family laws, restated as binding:

- **R1** — Depends only on `core` and the standard library. MUST NOT
  import `events`, `spatial`, any rulebook, or any `play/` sibling.
- **R2** — No `context.Context` anywhere.
- **R3** — Signature law, all three clauses, with the revised nil-Input
  rule (`ErrNilInput` guards first). Persistence pair exempt per family
  convention (`ToData() LogData`, package-level
  `LoadLog(data LogData) (*Log, error)`).
- **R4** — Results are returned, never published or delivered.
- **R5** — Verbs are atomic: on a non-nil error, no state changed.
- **R6** — `Seq` is assigned by `Append`, starts at 1, and is strictly
  monotonic and gapless across the log's lifetime; `TrimBefore` never
  reuses or renumbers. Entries are immutable once appended.
- **R7** — No randomness.
- **R8** — `ToData`/`LoadLog`; plain JSON structs; idle snapshot
  deep-equals the zero `LogData` and marshals to `{}`.
- **R9** — `LoadLog` MUST reject unreachable states with
  `ErrInvalidData`: entries out of order, duplicate or zero `Seq`,
  `NextSeq` not exceeding the last entry's `Seq`, empty entries slice
  with `NextSeq > 1` mismatched against a never-trimmed log ONLY when
  inconsistent (a trimmed-empty log with `NextSeq > 1` is reachable and
  MUST load), an audience containing empty IDs or duplicates, nil
  payloads (empty non-nil is legal).
- **R10** — Not safe for concurrent use.

## Types

- `Log` — the container. `NewLog()` (`(*Log, error)` per R3(a)); zero
  value not usable.
- `Entry` — the read-side value: `{Seq uint64, At uint64, Correlation
  string, Audience []core.EntityID, Payload []byte}`. `Correlation` is
  an opaque caller token grouping cause and effects; empty is legal
  (uncorrelated beat). `Audience` is materialized and deduplicated;
  EMPTY MEANS NO VIEWER (GM/debug beat) — "everyone" is an explicit
  roster. `Payload` is opaque; composition-encoded story beats (leaf
  deltas, outcomes); record never interprets.

## Verbs

| Verb | Input | Output | Semantics |
|------|-------|--------|-----------|
| `Append` | `{At uint64, Correlation string, Audience []core.EntityID, Payload []byte}` | `{Seq uint64}` | Appends one immutable entry, assigning the next `Seq`. Audience is defensively copied and MUST be duplicate-free with no empty IDs (`ErrBadAudience`); payload MUST be non-nil (`ErrNoPayload`; empty non-nil is legal — presence with no content). Errors: `ErrNilInput`, `ErrBadAudience`, `ErrNoPayload`. |
| `TrimBefore` | `{Seq uint64}` | `{Removed int}` | Drops all entries with `Seq < in.Seq`. Retention is the composition's policy made visible (brainstorm §4). Trimming at or below the current head is a no-op (`Removed: 0`), not an error; trimming beyond `NextSeq` errors `ErrBadSeq` (a policy bug — you cannot forget the future). Never renumbers. |

## Queries

| Query | Input | Returns | Semantics |
|-------|-------|---------|-----------|
| `SliceFor` | `{Viewer core.EntityID, FromSeq uint64}` | `([]Entry, error)` | Entries with `Seq >= FromSeq` whose audience contains Viewer, in Seq order — the reconnect/replay call. Empty viewer errs `ErrNoViewer`. No holdings → empty result, nil error. Copy-out: returned entries (audience and payload included) MUST NOT alias internal state. |
| `All` | `{FromSeq uint64}` | `([]Entry, error)` | Every retained entry from `FromSeq`, Seq order — the GM/debug/host view. Copy-out as above. |
| `NextSeq()` | — | `(uint64, error)` | The Seq the next `Append` will assign (zero-arg read, bare value per R3(c)). Never errs today; the error slot is the law's. |

## Errors

| Sentinel | Meaning | Returned by |
|----------|---------|-------------|
| `ErrNilInput` | nil `*XxxInput` | every Input-taking function |
| `ErrBadAudience` | audience with empty IDs or duplicates | `Append` |
| `ErrNoPayload` | nil payload | `Append` |
| `ErrBadSeq` | trim point beyond `NextSeq` | `TrimBefore` |
| `ErrNoViewer` | empty viewer ID | `SliceFor` |
| `ErrInvalidData` | any R9 rejection | `LoadLog` |

## Persistence

`LogData` = `struct{ NextSeq uint64; Entries []EntryData }` (tags
`next_seq,omitempty` / `entries,omitempty`; `EntryData` mirrors `Entry`
with tags `seq`/`at,omitempty`/`correlation,omitempty`/`audience,omitempty`/
`payload`). Struct wrapper so the zero value marshals `{}` (R8). Family
conventions: deep copies both directions; wire shape pinned by a
golden-JSON test. Size note: the log grows with time by design; the
pressure valve is `TrimBefore`, owned by the composition (brainstorm §4)
— the lifecycle note every growing structure in this family carries.

## Acceptance criteria

- **AC1 (the two-viewer story)** — one integration test: a sequence of
  appends with differing audiences (a shared beat, an
  audience-of-one reveal for the door opener, a smaller one for the
  follower, a GM-only beat with empty audience); `SliceFor` each viewer
  returns exactly their story in order; `All` returns everything;
  `TrimBefore` after "acknowledgment" shortens replay without
  renumbering; a post-trim `SliceFor` from an early `FromSeq` returns
  only retained entries.
- **AC2 (invariants)** — Seq gapless/monotonic across appends and trims;
  R5 atomicity (failed appends change nothing, `NextSeq` unmoved);
  copy-out immunity both directions.
- **AC3 (round-trips)** — `ToData`/`LoadLog` at idle, populated,
  post-trim (including trimmed-empty with `NextSeq > 1`);
  behavior-identical; every R9 rejection tested.
- **AC4 (compat gate)** — added to `compat.yml` (same note as intel:
  workflow-level triggering accepted).
- **AC5 (suite conventions)** — black-box, testify suites per type,
  plain-function integration spine.
- **AC6 (error vocabulary)** — every sentinel `errors.Is`-tested.
