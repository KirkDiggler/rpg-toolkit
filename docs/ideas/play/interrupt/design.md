# play/interrupt — Design (the WHAT)

**Status:** PROPOSED
**Module:** `github.com/KirkDiggler/rpg-toolkit/play/interrupt` (package `interrupt`)
**Why:** `brainstorm.md`. **How:** `plan.md` (after this is approved).

## Scope

The ledger of open windows: suspension-as-value custody for resolutions
that must stop and wait for an outside decision. `Pose` opens a window
(audience, offered options, opaque frozen payload); `Answer` validates,
closes it, and returns the envelope for the rulebook to resume. Read
queries project the open set for hosts and deciders. The module never
interprets options, choices, or payloads — custody, not execution.

**Non-goals (v1):** timeout/expiry machinery — no deadlines, no default
answers, no `Expire` verb (shelf, per the brainstorm: skip-with-default /
kick-from-encounter / wait-timer; all additive later); `Withdraw` and
parallel posing (sequential posing in initiative order is composition
policy; the leaf is identical under either); nesting (no parent links —
additive when counterspell-class play earns it); multi-candidate
audiences with first-answer-wins (counterspell-shaped, shelved); the
resolution seam itself (suspendable re-enterable phase machines are the
**resolution axis**, in the rulebook); trigger discovery (explicit
checkpoints + enumeration live in the rulebook, per journey 052); story
retention (composition feeds `play/record` from Pose/Answer outputs);
wire/UI delivery (the host projects the queries); decider integration
(deciders answer through composition calling `Answer` — the leaf does
not know deciders exist).

## Rules

The family laws from `play/clock`'s design apply verbatim and are
restated here as binding:

- **R1** — Depends only on `core` and the standard library. MUST NOT
  import `events`, `spatial`, any rulebook, or any `play/` sibling.
- **R2** — No `context.Context` anywhere.
- **R3** — Signature law, all three clauses (error last; single
  `*XxxInput` for any function taking parameters; mutating verbs return
  `*XxxOutput`, zero-arg reads return bare value + error). Nil-Input
  rule: every Input-taking function guards nil first and returns
  `ErrNilInput`. Persistence pair per the family naming law:
  `ToData() LedgerData` with package-level
  `LoadLedger(data LedgerData) (*Ledger, error)`. No namesake collision
  (the container is `Ledger`, not `Interrupt` — brainstorm decision 8),
  so the plain `Load<T>(TData)` scheme applies unmodified.
- **R4** — Deltas are returned in the verb's Output, never published,
  never delivered.
- **R5** — Verbs are atomic: on a non-nil error, no state changed.
- **R6** *(module-specific law: custody, not execution)* — The module
  never interprets an option token, a payload byte, or a choice. One
  answer per window: `Answer` removes the window; a second answer to the
  same ID is `ErrNotOpen`. Windows leave the ledger only via `Answer`
  (v1). Any number of windows may be open at once, including several for
  one audience — posing cadence is composition policy, invisible here.
- **R7** — No randomness. Window IDs are assigned monotonically from 1.
- **R8** — Dynamic state round-trips via `ToData`/`LoadLedger` (plain
  JSON-serializable structs, no behavior). Idle snapshot (never posed)
  deep-equals the zero `LedgerData` and marshals to `{}`. A ledger whose
  windows have all been answered is NOT idle: `NextID` persists so
  window IDs are never reused within an encounter's story (record beats
  and composition-held tokens reference them).
- **R9** — `LoadLedger` MUST reject unreachable states with
  `ErrInvalidData`: `NextID == 0` with any windows present (IDs are
  assigned from 1 and `NextID` advances past every assignment — also the
  uint64-wraparound forgery guard, the record precedent); any window ID
  of 0; window IDs not strictly ascending in slice order (covers
  duplicates; slice order IS pose order); any window ID `>= NextID`;
  empty audience; nil or empty options; an empty option token; a
  duplicate option within a window. Nil payload is LEGAL — reachable by
  design (see Types), and `payload` is `omitempty`, so rejecting it
  would make `LoadLedger` refuse the module's own snapshots. `At` is
  never validated (opaque provenance).
- **R10** — Not safe for concurrent use (family convention).

## Types

- `Ledger` — the container: every open window for one encounter.
  Construct via `NewLedger()` (`(*Ledger, error)` per R3); the zero
  value is not usable.
- `WindowID` — `uint64` newtype. Assigned by the ledger, monotonic from
  1 (R7 — no randomness; determinism is what makes IDs usable as resume
  tokens). The ID is the resume token in practice: composition holds it
  to route the eventual answer back.
- `Option` — string newtype. Opaque tokens owned by the rulebook
  ("shield", "decline", "attack"); the module validates only
  non-emptiness, uniqueness within a window, and membership at answer
  time. It never knows what any option means.
- `Window` — the read-side value: `{ID WindowID, Audience
  core.EntityID, Options []Option, Payload []byte, At uint64}`.
  `Audience` is the ONE entity that may answer (multi-candidate windows
  are shelved). `Payload` is the frozen resolution — opaque bytes the
  module never decodes; nil is legal (composition may keep frozen state
  elsewhere, keyed by ID). `At` is an opaque caller-supplied provenance
  stamp (`uint64`, the intel convention); the module records it and
  MUST NOT order, compare, or validate by it.

## Verbs

| Verb | Input | Output | Semantics |
|------|-------|--------|-----------|
| `Pose` | `{Audience core.EntityID, Options []Option, Payload []byte, At uint64}` | `{Window Window}` | Opens a window: assigns the next ID, stores a deep copy (payload and options copied in), returns the stored window (copied out — the caller's slices never alias the ledger's). Options MUST be non-empty: with no timeout machinery, a window without options is unanswerable and would deadlock the encounter — `ErrNoOptions` is the liveness guard. MUST error: `ErrNilInput`, `ErrNoAudience` (empty audience), `ErrNoOptions` (nil or empty options), then per option in slice order: `ErrNoOption` (empty token), `ErrDuplicateOption` (repeated token — options are distinct choices; a duplicate is caller defect, not transport-for-a-set). Validation in that listed order, first failure wins, all before any mutation (R5 — a failed Pose does not consume an ID). |
| `Answer` | `{Window WindowID, By core.EntityID, Choice Option}` | `{Window Window, Choice Option}` | Closes a window: validates, removes it from the ledger, and returns the envelope plus the accepted choice — composition hands both to the rulebook's resume. *Ownership transfer, not copy-out (amended during execution, Task 3 review):* the returned envelope is the spliced window itself — after removal nothing internal retains it, so a defensive copy would be a pin that cannot fail, which the mutation-proof law forbids as an unfalsifiable clause. Queries copy out because internal state stays; Answer transfers because it removes. Validation order: `ErrNilInput`; `ErrNoAudience` (empty `By` — an empty audience claim is a shape defect, checked before lookup); `ErrNoOption` (empty `Choice`); `ErrNotOpen` (no open window with that ID — unknown, never posed, or already answered); `ErrNotAudience` (`By` differs from the window's audience); `ErrNotOffered` (`Choice` not among the window's options). Shape, then existence, then authorization, then membership; first failure wins; on any error the window remains open and unchanged (R5). |

There is no `Withdraw`, `Expire`, or default-answer path in v1 — see
Non-goals. Auto-answered windows (the OA-autofire case) are ordinary
`Pose` + `Answer` in immediate succession by composition; the ledger
cannot tell and must not care.

## Queries

| Query | Input | Returns | Semantics |
|-------|-------|---------|-----------|
| `PendingFor` | `{Audience core.EntityID}` | `([]Window, error)` | The open windows this entity may answer, in pose order (ascending ID). An entity with none answers an empty slice, nil error. `ErrNilInput` / `ErrNoAudience` on defective input. Copy-out: returned windows (options and payload bytes included) MUST NOT alias internal state. This query is the player's prompt: the wizard's Shield button renders from it. |
| `Open` | *(zero-arg)* | `([]Window, error)` | Every open window, pose order. Copy-out as above. This query is the table's state: "waiting on Aldric…" renders from it. |

The decider contract (normative for compositions, stated here because it
is the module's purpose): a decider is shown a window from `PendingFor`
(plus whatever view its own axis grants — its intel, never the world)
and produces a choice; composition calls `Answer`. Human and machine
deciders are indistinguishable to the ledger — that symmetry is what
makes auto-OA, ask-me OA, and monster reactions one mechanism.

## Errors

All errors wrap one sentinel; `errors.Is` dispatch; messages user-facing.

| Sentinel | Meaning | Returned by |
|----------|---------|-------------|
| `ErrNilInput` | nil `*XxxInput` | every Input-taking function |
| `ErrNoAudience` | empty audience entity ID (`Pose.Audience`, `Answer.By`, `PendingFor.Audience`) | `Pose`, `Answer`, `PendingFor` |
| `ErrNoOptions` | nil or empty options — an unanswerable window (the liveness guard) | `Pose` |
| `ErrNoOption` | an empty option token (`Pose` options, `Answer.Choice`) | `Pose`, `Answer` |
| `ErrDuplicateOption` | a repeated option token within one window | `Pose` |
| `ErrNotOpen` | no open window with that ID (unknown or already answered) | `Answer` |
| `ErrNotAudience` | the answerer is not the window's audience | `Answer` |
| `ErrNotOffered` | the choice is not among the window's options | `Answer` |
| `ErrInvalidData` | any R9 rejection | `LoadLedger` |

## Persistence

`LedgerData` = `struct{ NextID uint64; Windows []WindowData }` (JSON
tags `next_id,omitempty` and `windows,omitempty`; a struct wrapper so
the zero value marshals to `{}` per R8) with `WindowData{ID WindowID,
Audience core.EntityID, Options []Option, Payload []byte, At uint64}`
(tags: `id`, `audience`, `options`, `payload,omitempty`, `at,omitempty`;
`id`/`audience`/`options` are never empty in reachable states so they
carry no `omitempty`). `Windows` is a slice in pose order (ascending
ID) — order is meaningful and the marshal is deterministic. Family
conventions: `ToData` deep-copies (snapshot immune to later verbs;
`LoadLedger` copies in — the caller's `LedgerData` is never aliased);
idle deep-equals zero and marshals `{}`; wire shape pinned by
exact-string golden-JSON tests; **mutation-proof pins required** (each
persistence pin shown to fail under the exact breakage it guards —
aliased copy, renamed wire tag, added stowaway field — the axis-two
standard, demanded from the first persistence task, not discovered in
review). Distinct persisted shapes: fresh (`{}`), open windows
(`next_id` + `windows`), all-answered (`next_id` only — MUST load; IDs
are never reused within an encounter).

## Acceptance criteria

- **AC1 (the Shield scene)** — one integration test, the brainstorm's
  narrative as transcript, fixture bytes standing in for the frozen
  resolution (the rulebook's half is the resolution axis — not here):
  an attack "freezes" → `Pose{Audience: "aldric", Options: ["shield",
  "decline"], Payload: frozenFixture, At: 7}`; `PendingFor("aldric")`
  renders the button (exactly one window, options in offered order);
  the fighter tries to answer Aldric's window → `ErrNotAudience`, window
  untouched; Aldric tries an unoffered choice → `ErrNotOffered`; Aldric
  answers "shield" → envelope returned with payload byte-identical to
  what was posed (custody proven — retroactive re-judgment is the
  rulebook's job; the scene proves the envelope's fidelity),
  `PendingFor` now empty, `Open` empty; an auto-OA beat — `Pose` +
  immediate `Answer` by a policy decider, zero queries between (the
  decider-swap unification: the ledger cannot tell); a sequential beat —
  a second window posed only after the first resolves, IDs strictly
  ascending. Full transcript asserted at each step.
- **AC2 (invariants)** — R5 atomicity from populated states (every
  error path leaves `Open()` identical); one-answer-per-window (second
  `Answer` → `ErrNotOpen`); failed `Pose` consumes no ID (next
  successful pose gets the expected ID); copy-out immunity where
  live state is aliasable — Pose's returned window, `PendingFor`, and
  `Open` (mutate returned windows/options/payloads and the caller's
  input slices post-call; internal state unchanged); `Answer`'s envelope
  is an ownership transfer, deliberately unpinned (see the Answer row);
  audience isolation
  (`PendingFor("bob")` empty while Aldric holds windows); pose-order
  stability in both queries.
- **AC3 (round-trips)** — `ToData`/`LoadLedger` at every distinct
  state (fresh; open windows including a nil-payload window;
  all-answered with `NextID > 0`); behavior-identical after reload (an
  `Answer` after reload returns the same envelope; a `Pose` after
  reload continues the ID sequence); golden JSON exact-string pins for
  fresh, open, and all-answered shapes; every R9 rejection has a test;
  mutation-proof evidence recorded for every pin (see Persistence).
- **AC4 (compat gate)** — `play/interrupt/**` added to `compat.yml`
  paths with a `gorelease-play-interrupt` job cloned from the existing
  three (pinned gorelease version).
- **AC5 (suite conventions)** — black-box `package interrupt_test`,
  testify suites for per-type tests, plain functions for the AC1
  integration spine (the documented family exception).
- **AC6 (error vocabulary)** — every sentinel `errors.Is`-tested from a
  function that returns it.
