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
(available by composition, imposed on nobody); rich query languages
(OR, ranges, full-text, indices) — the host projects from `All` into
whatever engine it likes; record's filter is the common-case
question-surface, not a database.

## Rules

Family laws, restated as binding:

- **R1** — Depends only on `core` and the standard library. MUST NOT
  import `events`, `spatial`, any rulebook, or any `play/` sibling.
- **R2** — No `context.Context` anywhere.
- **R3** — Signature law, all three clauses, with the revised nil-Input
  rule (`ErrNilInput` guards first). Persistence pair exempt per family
  convention (`ToData() LogData`, package-level
  `LoadLog(data LogData) (*Log, error)` — the family persistence
  naming law: every persistable type T pairs the method `ToData() TData`
  with the package-level constructor `Load<T>(data TData) (*T, error)`.
  The loader carries the type name because packages may host multiple
  persistable types (clock does: LoadTurn/LoadTick); a literal LoadData
  cannot disambiguate, so Load<T> is the one scheme uniform across the
  family. The scheme serves a spirit older than its letter — the host
  keeps rich runtime models and can always obtain a savable copy that
  loads back to exactly here; the letter has evolved before and may
  again).
- **R4** — Results are returned, never published or delivered.
- **R5** — Verbs are atomic: on a non-nil error, no state changed.
- **R6** — `Seq` is assigned by `Append`, starts at 1, and is strictly
  monotonic and gapless across the log's lifetime; `TrimBefore` never
  reuses or renumbers. Entries are immutable once appended.
- **R7** — No randomness.
- **R8** — `ToData`/`LoadLog`; plain JSON structs; idle snapshot
  deep-equals the zero `LogData` and marshals to `{}`. The fresh-log
  encoding convention that makes this hold (record's fresh runtime state
  is NOT all-zero — `NextSeq` is 1): stored `NextSeq: 0` encodes a fresh
  log (`ToData` on a never-appended log writes the zero `LogData`;
  `LoadLog(LogData{})` yields a fresh log whose `NextSeq()` answers 1).
  A trimmed-empty log stores its real `NextSeq` (always >= 2). Stored
  `NextSeq: 1` is therefore never written and MUST be rejected (R9).
- **R9** — `LoadLog` MUST reject unreachable states with
  `ErrInvalidData`: non-contiguous or zero `Seq` values (Append assigns
  gaplessly and `TrimBefore` removes only a strict prefix, so every
  reachable non-empty log has consecutive retained Seqs); for non-empty
  logs, `NextSeq` != last `Seq` + 1 (exactly — no verb can produce
  slack); for empty logs, stored `NextSeq: 1` (fresh encodes as 0 per
  R8; trimmed-empty is always >= 2); an audience containing empty IDs
  or duplicates; tag maps containing empty keys; nil payloads (empty
  non-nil is legal). Affirmative
  notes, not rejections: an empty entries slice is legal with stored
  `NextSeq` 0 (fresh) or >= 2 (trimmed-empty — reachable and MUST
  load); a trimmed log's first retained `Seq` may be any value >= 1.
- **R10** — Not safe for concurrent use.

## Types

- `Log` — the container. `NewLog()` (`(*Log, error)` per R3(a)); zero
  value not usable.
- `Entry` — the read-side value: `{Seq uint64, At uint64, Correlation
  string, Audience []core.EntityID, Payload []byte}`. `Correlation` is
  an opaque caller token grouping cause and effects; empty is legal
  (uncorrelated beat). `Audience` is materialized and duplicate-free (enforced at `Append` via `ErrBadAudience`, never silently deduped);
  EMPTY MEANS NO VIEWER (GM/debug beat) — "everyone" is an explicit
  roster. `Payload` is opaque; composition-encoded story beats (leaf
  deltas, outcomes); record never interprets. `Tags map[string]string` is
  the caller-chosen question-surface: queryable metadata flattened out of
  the payload by the composition (typical keys: `kind` —
  "intel.first_contact", "clock.turn_started" — `observer`, `subject`,
  `actor`; the vocabulary is the composition's, same naming-is-testimony
  doctrine as intel's subjects). Record never interprets keys or values;
  keys MUST be non-empty (`ErrBadTag`); empty values are legal (flags);
  nil/empty tags are legal (an unaskable beat). Differently-shaped
  sources coexist in one log because the question-surface is uniform even
  when payloads are not.

## Verbs

| Verb | Input | Output | Semantics |
|------|-------|--------|-----------|
| `Append` | `{At uint64, Correlation string, Audience []core.EntityID, Tags map[string]string, Payload []byte}` | `{Seq uint64}` | Appends one immutable entry, assigning the next `Seq`. Audience and tags are defensively copied, with empty-non-nil normalized to nil on store (the family nil-container convention — Go-level snapshot compares stay deterministic); audience MUST be duplicate-free with no empty IDs (`ErrBadAudience`); tag keys MUST be non-empty (`ErrBadTag`); payload MUST be non-nil (`ErrNoPayload`; empty non-nil is legal — presence with no content). Validation order is the Input's documented field order, first failure wins (nil guard, then At/Correlation-free fields in order: Audience, Tags, Payload) — exact sentinels for multi-defect inputs. The same first-failure-wins rule (nil guard, Viewer, Tags) governs the queries. Errors: `ErrNilInput`, `ErrBadAudience`, `ErrBadTag`, `ErrNoPayload`. |
| `TrimBefore` | `{Seq uint64}` | `{Removed int}` | Drops all entries with `Seq < in.Seq`. Retention is the composition's policy made visible (brainstorm §4). Trimming at or below the oldest retained `Seq` is a no-op (`Removed: 0`), not an error; `in.Seq == NextSeq` is legal and empties the log; `in.Seq > NextSeq` errors `ErrBadSeq` (a policy bug — you cannot forget the future). Never renumbers. |

## Queries

| Query | Input | Returns | Semantics |
|-------|-------|---------|-----------|
| `SliceFor` | `{Viewer core.EntityID, FromSeq uint64, Tags map[string]string}` | `([]Entry, error)` | Entries with `Seq >= FromSeq` whose audience contains Viewer AND which carry every given tag key with exactly the given value (AND semantics; nil/empty Tags = no filter; filter keys MUST be non-empty, `ErrBadTag`), in Seq order — the reconnect/replay call. Empty viewer errs `ErrNoViewer`. No matching entries → empty result, nil error. Copy-out: returned entries (audience and payload included) MUST NOT alias internal state. |
| `All` | `{FromSeq uint64, Tags map[string]string}` | `([]Entry, error)` | Every retained entry from `FromSeq` matching the same optional tag filter, Seq order — the GM/debug/host view. Copy-out as above. |
| `NextSeq()` | — | `(uint64, error)` | The Seq the next `Append` will assign (zero-arg read, bare value per R3(c)). Never errs today; the error slot is the law's. |

## Errors

| Sentinel | Meaning | Returned by |
|----------|---------|-------------|
| `ErrNilInput` | nil `*XxxInput` | every Input-taking function |
| `ErrBadAudience` | audience with empty IDs or duplicates | `Append` |
| `ErrNoPayload` | nil payload | `Append` |
| `ErrBadTag` | a tag (or filter) key that is empty | `Append`, `SliceFor`, `All` |
| `ErrBadSeq` | trim point beyond `NextSeq` | `TrimBefore` |
| `ErrNoViewer` | empty viewer ID | `SliceFor` |
| `ErrInvalidData` | any R9 rejection | `LoadLog` |

## Persistence

`LogData` = `struct{ NextSeq uint64; Entries []EntryData }` (tags
`next_seq,omitempty` / `entries,omitempty`; `EntryData` mirrors `Entry`
with tags `seq`/`at,omitempty`/`correlation,omitempty`/`audience,omitempty`/
`tags,omitempty`/`payload`; `encoding/json` sorts map keys, so the
golden-JSON test stays deterministic). Struct wrapper so the zero value marshals `{}` (R8). Family
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
  only retained entries. Beats carry mixed-shape tags (intel-style and
  clock-style) and the tag filter answers questions across them: all
  `kind=intel.first_contact` beats for the opener; everything tagged
  with one subject; every `kind=clock.turn_started` regardless of
  viewer (via `All`).
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
