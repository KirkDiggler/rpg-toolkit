# play/intel — Design (the WHAT)

**Status:** PROPOSED
**Module:** `github.com/KirkDiggler/rpg-toolkit/play/intel` (package `intel`)
**Why:** `brainstorm.md`. **How:** `plan.md` (after this is approved).

## Scope

Per-observer intel: channel-sourced, possibly false, possibly stale
holdings about opaque subjects, each on a topic. A holding is a record of
three parts — holder (observer), subject, topic: the holder and subject at
the outside edges, the topic carrying the relationship between them
(amended 2026-11-11, rpg-project#928: the operator's ruling that an intel
record carries a topic). Two testimony verbs (`Surveil` sustained,
`Report` discrete), read queries, persistence. The module never sees the
world and MUST NOT be able to verify anything.

**Non-goals (v1):** geometry and channel physics (the stage's and the
rulebooks' business); dice and perception checks (rulebook rolls, the
composition gates testimony); truth access or verification of any kind;
reconciliation of contradictory intel (the reader's judgment);
delivery/streaming (host wire); retained event logs (`play/record`'s
axis); forgetting (a future additive `Forget` verb — gameplay, not
hygiene: modify-memory spells are negative testimony); subject identity
resolution (identity is testimony, chosen by the caller).

## Rules

The family laws from `play/clock`'s design apply verbatim and are
restated here as binding:

- **R1** — Depends only on `core` and the standard library. MUST NOT
  import `events`, `spatial`, any rulebook, or any `play/` sibling.
- **R2** — No `context.Context` anywhere.
- **R3** — Signature law, all three clauses (error last; single
  `*XxxInput` for any function taking parameters; mutating verbs return
  `*XxxOutput`, zero-arg reads return bare value + error). Nil-Input
  rule as revised 2026-08-09: every Input-taking function guards nil
  first and returns `ErrNilInput`. Persistence pair exempt per family
  convention (`ToData() Data`, package-level
  `LoadIntel(data Data) (*Intel, error)` — the family persistence
  naming law: every persistable type T pairs the method `ToData() TData`
  with the package-level constructor `Load<T>(data TData) (*T, error)`.
  The loader carries the type name because packages may host multiple
  persistable types (clock does: LoadTurn/LoadTick); a literal LoadData
  cannot disambiguate, so Load<T> is the one scheme uniform across the
  family. The scheme serves a spirit older than its letter — the host
  keeps rich runtime models and can always obtain a savable copy that
  loads back to exactly here; the letter has evolved before and may
  again. Namesake-collision clause, decided 2026-08-09 during execution:
  intel is the family's first module whose persistable type IS the
  package's namesake, and `TData`'s letter would produce
  `intel.IntelData` — a stutter the qualified name already resolves
  (revive agrees). When T is the package namesake, `TData` collapses to
  `Data`: the caller reads `intel.Data` alongside `clock.TurnData` and
  `record.LogData`, and disambiguation stays where it always lived, in
  the loader's name — a second persistable type here would still be
  `Load<Other>(OtherData)`).
- **R4** — Deltas are returned in the verb's Output, never published,
  never delivered.
- **R5** — Verbs are atomic: on a non-nil error, no state changed.
- **R6** — One current holding per (observer, subject, topic). Latest
  accepted testimony on that topic wins unconditionally; testimony on one
  topic never reads or writes a holding on another; the module MUST NOT prefer channels,
  verbs, or payloads when overwriting.
- **R7** — No randomness.
- **R8** — Dynamic state round-trips via `ToData`/`LoadIntel` (plain
  JSON-serializable structs, no behavior). Idle snapshot deep-equals the
  zero `Data` and marshals to `{}`.
- **R9** — `LoadIntel` MUST reject unreachable states with
  `ErrInvalidData`: empty observer keys, empty subject keys, duplicate
  entries in a holding's `CurrentVia`, empty channel identifiers (in a
  holding's `Channel` or any `CurrentVia` entry — unreachable since
  `ErrNoChannel`), nil or empty inner subject maps, nil or empty topic
  maps under a subject
  (holdings are never removed in v1, so an observer or subject entry with
  zero holdings is unreachable — the reachable-states-only rationale). The
  empty topic key is legal: it is the default topic. Channel *names* are an open
  vocabulary and are never validated beyond non-emptiness.
- **R10** — Not safe for concurrent use (family convention).

## Types

- `Intel` — the container: all observers' holdings for one encounter.
  Construct via `NewIntel()` (`(*Intel, error)` per R3(a)); the zero
  value is not usable (internal maps).
- `Channel` — open string vocabulary. `Sight` is predeclared; physical
  channels get physics from the stage, supernatural ones from rulebooks;
  intel treats all identically and never validates names.
- `Subject` — string newtype. Opaque, caller-chosen, at the observer's
  fidelity: a place key, an entity ID, a believed identity. Choosing
  subjects is part of the testimony and MUST NOT leak truth the observer
  lacks (design guidance to compositions, unenforceable here by
  construction).
- `Topic` — string newtype, the record's third part: what the holding is
  about the subject. Open, opaque, caller-chosen vocabulary like `Channel`
  and `Subject`; never interpreted. The empty topic is the default and is
  legal everywhere — a caller that never names a topic gets exactly the
  one-holding-per-subject behaviour of the module before topics. Example:
  a ledger clerk holds, about account `acct-7`, where its papers are filed
  (default topic) and that it was signed off (topic `audited`); refiling
  the papers never touches the sign-off. Not taken (rpg-project#925): the
  topic folded into the subject's name (the record no longer says whom it
  is about); one holding carrying a list (two sources overwrite each other,
  and current/held would apply to the bundle).
- `Report` — one piece of testimony: `{Subject Subject, Payload []byte}`.
  Payload is opaque; intel never interprets it. Empty payloads are legal
  ("something is there"); nil and empty are the same legal empty payload,
  and `LoadIntel` accepts payload-less holdings (unlike record, whose
  `ErrNoPayload` makes them unreachable there). Compositions control the dominant size term of
  persisted intel through payload discipline — keep payloads small
  (refs, compact snapshots).
- `Holding` — the read-side value: `{Subject, Topic, Payload, Channel, At,
  CurrentVia []Channel, Status}`. `Channel`/`At` are provenance of the
  latest accepted testimony. `Status` is DERIVED, never stored:
  `Current` if `CurrentVia` is non-empty, `Held` otherwise (the old
  VISIBLE/REMEMBERED wire contract, renamed).
- `At` stamps are opaque caller-supplied ordering tokens (`uint64`);
  intel records them as provenance and MUST NOT order or validate by
  them.

## Verbs

| Verb | Input | Output | Semantics |
|------|-------|--------|-----------|
| `Surveil` | `{Observer core.EntityID, Channel Channel, Topic Topic, Percept []Report, At uint64}` | `{FirstContact []Report, Refreshed []Subject, Faded []Subject}` | Sustained collection. `Percept` is the COMPLETE current percept for this observer+channel+topic — the contract that makes fading derivable. The topic scopes it: everything below reads and writes only holdings on `Topic`, so a percept about one topic can never fade a holding on another (the empty `Topic` is the default topic). Every subject in the output is a holding on that one topic. For each report: subject unknown to observer → holding created with `CurrentVia = [Channel]`, in `FirstContact`; known → payload/provenance overwritten (R6) and `Channel` added to its `CurrentVia` (set semantics), in `Refreshed`; every subject previously current *via this channel* but absent from `Percept` → this channel removed from its `CurrentVia`, and if that empties it, the subject is in `Faded` (still held — the ghost goblin). Channels other than the surveilling one are untouched. Losing one of several sustaining channels shrinks `CurrentVia` silently — deliberately unreported; `Faded` fires only when the set empties. An empty `Percept` is legal (seeing nothing: fades everything this channel sustained). MUST error: `ErrNilInput`, `ErrNoObserver` (empty observer), `ErrNoChannel` (empty channel — the vocabulary is open but the identifier is required), `ErrNoSubject` (any report with an empty subject) — validation in that listed order, first failure wins (the record-side convention, applied family-wide), all before any mutation (R5). Duplicate subjects within one `Percept`: dedupe first, last entry wins, the survivor occupying its last occurrence's position — each subject classified once (the slice is transport for a set). Delta ordering: `FirstContact`/`Refreshed` in post-dedupe percept order; `Faded` sorted by Subject (deterministic transcripts — the AC1/record contract). |
| `Report` | `{Observer core.EntityID, Channel Channel, Topic Topic, Reports []Report, At uint64}` | `{FirstContact []Report, Updated []Subject}` | Discrete testimony, on the input's one `Topic` (every report in the call is about it, as every report is on the one `Channel`; a caller with several topics makes several calls). (the crash of pots, the informant's tip, the charm's plant). Lands directly as HELD — `CurrentVia` is not touched for existing holdings and is empty for new ones; there is nothing to fade. Same dedupe-first rule as `Surveil` (each subject classified once): unknown → `FirstContact`; known → overwritten (R6), in `Updated` — a `Report` overwriting a currently-surveilled holding changes payload and provenance only; `CurrentVia` and thus `Status` are unchanged. Delta ordering: post-dedupe input order. Same errors as `Surveil`. |

Falsehood is orthogonal to the verb (design invariant): `Surveil`
carries sustained falsehoods (illusions, disguises), `Report` carries
discrete ones. No verb verifies; none can.

## Queries

| Query | Input | Returns | Semantics |
|-------|-------|---------|-----------|
| `HeldBy` | `{Observer core.EntityID}` | `([]Holding, error)` | Everything the observer holds across every topic, statuses derived, stable order (sorted by Subject, then Topic). An observer with no holdings answers an empty slice, nil error. Copy-out: returned holdings (including payload bytes) MUST NOT alias internal state. |
| `About` | `{Observer core.EntityID, Subject Subject}` | `([]Holding, error)` | Everything the observer holds about one subject, one holding per topic, sorted by Topic. Nothing held answers an empty slice, nil error. Errors `ErrNilInput`, `ErrNoObserver`, `ErrNoSubject` in that order. Copy-out as above. |
| `OnTopic` | `{Observer core.EntityID, Topic Topic}` | `([]Holding, error)` | Everything the observer holds on one topic, one holding per subject, sorted by Subject. The empty topic is a legal question (the default topic). Nothing held answers an empty slice, nil error. Errors `ErrNilInput`, `ErrNoObserver`. Copy-out as above. |
| `On` | `{Observer core.EntityID, Subject Subject, Topic Topic}` | `(Holding, error)` | The observer's holding on one subject and topic (empty = default topic). `ErrNotHeld` when they hold nothing there — never a guessable zero value. Copy-out as above. |

The decider contract (normative for compositions, stated here because it
is the module's purpose): an NPC decider consults `HeldBy(itself)` and
nothing else. Monsters act on their intel, not the world.

## Errors

All errors wrap one sentinel; `errors.Is` dispatch; messages user-facing.

| Sentinel | Meaning | Returned by |
|----------|---------|-------------|
| `ErrNilInput` | nil `*XxxInput` | every Input-taking function |
| `ErrNoObserver` | empty observer ID | `Surveil`, `Report`, `HeldBy`, `About`, `OnTopic`, `On` |
| `ErrNoChannel` | empty channel identifier (names are open vocabulary, but required) | `Surveil`, `Report` |
| `ErrNoSubject` | a report with an empty subject, or an empty query subject | `Surveil`, `Report`, `About`, `On` |
| `ErrNotHeld` | the observer holds nothing on that subject | `On` |
| `ErrInvalidData` | any R9 rejection | `LoadIntel` |

## Persistence

`Data` = `struct{ Holdings map[core.EntityID]map[Subject]map[Topic]HoldingData }`
(observer → subject → topic, the record's three parts in order; the
default topic is the JSON key `""`. One representation for every topic —
the default is not stored apart. A record saved before topics existed,
with the holding directly under the subject, fails to decode rather than
loading misread: pre-release, no migration is carried)
(named per the namesake-collision clause in R3 — `intel.Data`, not
`intel.IntelData`)
(field tag `holdings,omitempty` — a struct wrapper, not a bare map, so the
zero value marshals to `{}` per R8, matching TurnData/TickData) with
`HoldingData{Payload []byte, Channel Channel, At uint64, CurrentVia
[]Channel}` (JSON tags: `payload`, `channel`, `at`, `current_via`, all
`omitempty`). Family conventions: `ToData`
deep-copies (snapshot immune to later verbs; `LoadIntel` copies in —
caller's maps never aliased); idle deep-equals zero and marshals `{}`;
wire shape pinned by a golden-JSON test. Size note (lifecycle, like
`DriverProgress`): intel never forgets on its own — HELD is forever
until overwritten; retention pressure is governed by composition payload
discipline and, later, the `Forget` verb.

## Acceptance criteria

- **AC1 (the door scene)** — one integration test, the brainstorm's
  narrative as transcript: `Report` hearing testimony on subject
  `behind-door-3` ("crashing"); a `Surveil` sight percept elsewhere
  establishes a goblin holding; the goblin leaves the percept → `Faded`
  (ghost at last-seen); the door opens and a sight `Surveil` includes
  `behind-door-3` with the truth ("pots, floor") → the sound-holding is
  overwritten and the connection worked because the composition aimed
  the subject; one `Report` on a supernatural channel plants a false
  holding and `On` returns it faithfully (deception native). Full
  delta-transcript and end-state holdings asserted.
- **AC2 (invariants)** — R5 atomicity from populated states; copy-out
  immunity both directions (mutate returned holdings/payloads and
  caller's input slices; internal state unchanged); per-channel currency
  (two channels sustain one subject; one channel's sustain lapses, the
  subject is NOT in `Faded` and status stays Current);
  R6 last-wins regardless of channel/verb.
- **AC3 (round-trips)** — `ToData`/`LoadIntel` at every distinct state
  (idle; current-and-held mix; multi-channel `CurrentVia`; post-fade);
  behavior-identical after reload; every R9 rejection has a test.
- **AC4 (compat gate)** — the module added to the existing
  `compat.yml` gorelease job (path filter + job, pinned version).
  Workflow-level path triggering means intel PRs also run the clock job
  (a no-op pass) — accepted; per-job filtering is optional plan polish.
- **AC5 (suite conventions)** — black-box `package intel_test`, testify
  suites for per-type tests, plain functions for the AC1 integration
  spine (the documented family exception).
- **AC6 (error vocabulary)** — every sentinel `errors.Is`-tested from a
  function that returns it.
