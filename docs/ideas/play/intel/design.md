# play/intel — Design (the WHAT)

**Status:** PROPOSED
**Module:** `github.com/KirkDiggler/rpg-toolkit/play/intel` (package `intel`)
**Why:** `brainstorm.md`. **How:** `plan.md` (after this is approved).

## Scope

Per-observer intel: channel-sourced, possibly false, possibly stale
holdings about opaque subjects. Two testimony verbs (`Surveil` sustained,
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
  convention (`ToData() IntelData`, package-level
  `LoadIntel(data IntelData) (*Intel, error)`).
- **R4** — Deltas are returned in the verb's Output, never published,
  never delivered.
- **R5** — Verbs are atomic: on a non-nil error, no state changed.
- **R6** — One current holding per (observer, subject). Latest accepted
  testimony wins unconditionally; the module MUST NOT prefer channels,
  verbs, or payloads when overwriting.
- **R7** — No randomness.
- **R8** — Dynamic state round-trips via `ToData`/`LoadIntel` (plain
  JSON-serializable structs, no behavior). Idle snapshot deep-equals the
  zero `IntelData` and marshals to `{}`.
- **R9** — `LoadIntel` MUST reject unreachable states with
  `ErrInvalidData`: empty observer keys, empty subject keys, duplicate
  entries in a holding's `CurrentVia`, nil holdings. Channels are an open
  vocabulary and are NOT validated.
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
- `Report` — one piece of testimony: `{Subject Subject, Payload []byte}`.
  Payload is opaque; intel never interprets it. Empty payloads are legal
  ("something is there"). Compositions control the dominant size term of
  persisted intel through payload discipline — keep payloads small
  (refs, compact snapshots).
- `Holding` — the read-side value: `{Subject, Payload, Channel, At,
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
| `Surveil` | `{Observer core.EntityID, Channel Channel, Percept []Report, At uint64}` | `{FirstContact []Report, Refreshed []Subject, Faded []Subject}` | Sustained collection. `Percept` is the COMPLETE current percept for this observer+channel — the contract that makes fading derivable. For each report: subject unknown to observer → holding created, in `FirstContact`; known → payload/provenance overwritten (R6), in `Refreshed`; every subject previously current *via this channel* but absent from `Percept` → this channel removed from its `CurrentVia`, and if that empties it, the subject is in `Faded` (still held — the ghost goblin). Channels other than the surveilling one are untouched. An empty `Percept` is legal (seeing nothing: fades everything this channel sustained). MUST error: `ErrNilInput`, `ErrNoSubject` (any report with an empty subject), `ErrNoObserver` (empty observer). Duplicate subjects within one `Percept`: last entry wins, earlier ones ignored (a percept is a set; the slice is transport). |
| `Report` | `{Observer core.EntityID, Channel Channel, Reports []Report, At uint64}` | `{FirstContact []Report, Updated []Subject}` | Discrete testimony (the crash of pots, the informant's tip, the charm's plant). Lands directly as HELD — `CurrentVia` is not touched for existing holdings and is empty for new ones; there is nothing to fade. Unknown subjects → `FirstContact`; known → overwritten (R6), in `Updated`. Same errors as `Surveil`. |

Falsehood is orthogonal to the verb (design invariant): `Surveil`
carries sustained falsehoods (illusions, disguises), `Report` carries
discrete ones. No verb verifies; none can.

## Queries

| Query | Input | Returns | Semantics |
|-------|-------|---------|-----------|
| `HeldBy` | `{Observer core.EntityID}` | `([]Holding, error)` | Everything the observer holds, statuses derived, stable order (sorted by Subject). An observer with no holdings answers an empty slice, nil error. Copy-out: returned holdings (including payload bytes) MUST NOT alias internal state. |
| `On` | `{Observer core.EntityID, Subject Subject}` | `(Holding, error)` | The observer's holding on one subject. `ErrNotHeld` when they hold nothing — never a guessable zero value. Copy-out as above. |

The decider contract (normative for compositions, stated here because it
is the module's purpose): an NPC decider consults `HeldBy(itself)` and
nothing else. Monsters act on their intel, not the world.

## Errors

All errors wrap one sentinel; `errors.Is` dispatch; messages user-facing.

| Sentinel | Meaning | Returned by |
|----------|---------|-------------|
| `ErrNilInput` | nil `*XxxInput` | every Input-taking function |
| `ErrNoObserver` | empty observer ID | `Surveil`, `Report`, `HeldBy`, `On` |
| `ErrNoSubject` | a report with an empty subject, or an empty query subject | `Surveil`, `Report`, `On` |
| `ErrNotHeld` | the observer holds nothing on that subject | `On` |
| `ErrInvalidData` | any R9 rejection | `LoadIntel` |

## Persistence

`IntelData` = `map[core.EntityID]map[Subject]HoldingData` with
`HoldingData{Payload []byte, Channel Channel, At uint64, CurrentVia
[]Channel}` (JSON tags: `payload`, `channel`, `at`, `current_via`, all
`omitempty`; outer field `holdings`). Family conventions: `ToData`
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
  (two channels sustain one subject; one fades, status stays Current);
  R6 last-wins regardless of channel/verb.
- **AC3 (round-trips)** — `ToData`/`LoadIntel` at every distinct state
  (idle; current-and-held mix; multi-channel `CurrentVia`; post-fade);
  behavior-identical after reload; every R9 rejection has a test.
- **AC4 (compat gate)** — the module added to the existing
  `compat.yml` gorelease job (path filter + job, pinned version).
- **AC5 (suite conventions)** — black-box `package intel_test`, testify
  suites for per-type tests, plain functions for the AC1 integration
  spine (the documented family exception).
- **AC6 (error vocabulary)** — every sentinel `errors.Is`-tested from a
  function that returns it.
