# play/clock — Design (the WHAT)

**Status:** PROPOSED
**Module:** `github.com/KirkDiggler/rpg-toolkit/play/clock` (package `clock`)
**Why:** `brainstorm.md`. **How:** `plan.md` (after this is approved).

## Scope

Scheduling for live play: who may act, and what advances time. v1 ships two
concrete clocks (`Turn`, `Tick`), the `Milestone` vocabulary, the `Transfer`
helper, and persistence.

**Non-goals (v1):** durations and timers; initiative rolling; end conditions
and outcomes; transition *triggers* (proximity detection); event publication;
wall-clock time; merging tick clocks; concurrency safety.

## Rules

- **R1** — The module MUST depend only on `core` and the standard library. It
  MUST NOT import `events`, `spatial`, any rulebook, or any `play/` sibling.
- **R2** — No function in this module takes `context.Context`. Cancellation
  is the composition's concern, between calls.
- **R3** — Signatures follow three mechanical clauses:
  **(a)** every exported function returns `error` as its final result — the
  error channel communicates state, not only failure (see the sentinel
  vocabulary in **Errors**);
  **(b)** every function that takes parameters takes exactly one
  `*XxxInput` struct — and mutating verbs take their Input struct even
  when it has no fields yet (signatures are permanent under AC4; struct
  fields are additive);
  **(c)** mutating verbs return `(*XxxOutput, error)` (the Output carries
  the Milestones); read-only functions return their value directly
  (`(int, error)`), gaining an Output struct only when they answer with
  more than one value.
  **Narrow exemption:** the persistence pair follows the toolkit-wide
  family convention verbatim — `ToData() TurnData` / `ToData() TickData`
  (infallible snapshot, no error) and the package-level constructors
  `LoadTurn(data TurnData) (*Turn, error)` /
  `LoadTick(data TickData) (*Tick, error)` (bare parameter). Cross-module
  uniformity outranks module-local law at this one seam: a host hydrating
  a dozen toolkit types gets one shape everywhere. `LoadTurn`/`LoadTick`
  are constructors, not verbs — R4 and R5 do not apply and they emit no
  milestones. `NewTick() (*Tick, error)` conforms to (a) with no carve-out
  (construction cannot fail today; the signature leaves room for a future
  `TickConfig` that can).
  **Nil Inputs:** passing a nil `*XxxInput` is a programmer error, not a
  communicable state — verbs do not guard it and may panic. The sentinel
  vocabulary is reserved for states of the clock, never defects in the
  caller.
- **R4** — Every verb MUST return all `Milestone`s it caused, in causal
  order, in its Output. The module MUST NOT publish, call back, or otherwise
  deliver milestones.
- **R5** — Verbs MUST be atomic: on a non-nil error, clock state is
  unchanged.
- **R6** — An entity belongs to at most one clock at a time. `Transfer` MUST
  leave both clocks unchanged when it fails.
- **R7** — The module MUST NOT contain randomness. All orderings arrive from
  the caller (`SetOrderInput.Order`, `InsertInput.Pos`, `MergeInput.Order`).
- **R8** — Dynamic state MUST round-trip via `ToData` and the
  `LoadTurn`/`LoadTick` constructors
  (plain JSON-serializable structs, no behavior). Configuration MUST be
  re-supplied at construction and MUST NOT serialize.
- **R9** — `LoadTurn`/`LoadTick` MUST reject invalid state with an error:
  duplicate members, `ActiveIdx` out of range (the canonical idle encoding
  is `Order` empty with `ActiveIdx 0` and `Round 0`; `ActiveIdx` MUST be 0
  and `Round` MUST be 0 when `Order` is empty, and `Round` MUST be >= 1
  when `Order` is non-empty — no verb can produce any other state, so any
  other state is corruption), negative budgets, negative `DriverProgress`
  values, `HighWater` less than the maximum `DriverProgress` (which
  would cause a spurious grant on the next `Advance`), and any individual
  budget greater than `HighWater` (total budget ever granted equals
  `HighWater`, so no verb can produce one — the same
  reachable-states-only rationale as the Turn-side checks).
- **R10** — Instances are not safe for concurrent use (family convention;
  hosts serialize access).

## Prerequisite

`core.EntityID` (a string newtype) added to the root `core` module.
Additive; shared identity vocabulary lives in `core` per the
dependency-direction law (journey 051).

## Milestone

| Field   | Type            | Notes                                        |
|---------|-----------------|----------------------------------------------|
| Kind    | `MilestoneKind` | one of the kinds below                       |
| Subject | `core.EntityID` | zero value when not about a specific entity  |
| Round   | `int`           | `Turn` clocks only; zero otherwise           |

Kinds (v1, closed set): `TurnStarted`, `TurnEnded`, `RoundStarted`,
`Ticked`, `MemberJoined`, `MemberLeft`, `Merged`, `Dissolved`.

Every milestone a `Turn` emits carries the clock's Round **at the moment of
emission**, in causal order — on a wrap, `TurnEnded` carries the old round;
`RoundStarted` and `TurnStarted` carry the new.

## Turn — the initiative bubble

State: an ordered member list, an active index, a round counter.
Data shape: `TurnData{Order []core.EntityID, ActiveIdx int, Round int}`.
A zero/empty `Turn` is valid and idle.

Queries: `Active() (core.EntityID, error)` and `Round() (int, error)`
(both `ErrIdle` when no order is set — never a guessable zero value),
`Order() ([]core.EntityID, error)` (an idle clock answers with an empty
slice and nil error — an empty list is an answer, like `Contains`'s
false), `Contains(in *ContainsInput) (bool, error)` (false is an answer;
no sentinel today). `Members()` on `Tick` follows the same empty-slice
convention.

| Verb | Input | Output | Semantics |
|------|-------|--------|-----------|
| `SetOrder` | `{Order []EntityID}` | `{Milestones}` | Replaces the order (rulebook rolled initiative). Round becomes 1, first member active. Milestones: `RoundStarted{1}`, `TurnStarted{first}`. MUST error on duplicate IDs or empty order. |
| `End` | `{Actor EntityID}` | `{Milestones, Next EntityID, RoundWrapped bool}` | MUST error unless `Actor == Active()`. Advances. On wrap: increments Round; milestones `TurnEnded{actor}, RoundStarted{n}, TurnStarted{next}`; otherwise `TurnEnded, TurnStarted`. |
| `Insert` | `{ID EntityID, Pos int}` | `{Milestones}` | Fall-in / reinforcement at caller-chosen position. MUST error if the clock is idle (no order set — bubbles start via `SetOrder`), if ID is present, or if `Pos` is outside `[0, len]`. Inserting at or before the active position MUST keep the currently active entity active. Milestone: `MemberJoined`. |
| `Remove` | `{ID EntityID}` | `{Milestones}` | Death/flee. MUST error if ID is absent (`ErrNotMember`). MUST keep the active entity correct: removing a non-active member adjusts the index; removing the active member makes the next member active (milestones `MemberLeft, TurnStarted{next}`); removing the last member resets the clock to the canonical idle state — Round 0, mirroring `Dissolve` — and emits `MemberLeft` only (the milestone carries the round at emission, before the reset). Without this reset, a fight ending by attrition would persist `Round > 0` with an empty order, which R9 rightly rejects as corruption. |
| `Merge` | `{Other *Turn, Order []EntityID}` | `{Milestones}` | Two bubbles collide. MUST error if the receiving clock is idle. `Other` MUST be a distinct clock — merging a clock into itself is refused (`ErrSameClock`); without the guard, self-merge validates against its own member set and then zeroes the receiver while reporting success. `Order` MUST be a permutation of the union of both member sets (error otherwise). The receiving clock's active entity MUST remain active; its Round is retained. `Other` is reset to the zero/idle state (empty order, `ActiveIdx 0`, `Round 0`). Milestone: `Merged`. |
| `Dissolve` | `{}` | `{Members []EntityID, Milestones}` | Fight over. MUST error if the clock is already empty. Empties the clock, returns the members for the composition to re-home. `Members` transfers ownership of the internal slice (the clock nils its own reference in the same call) — the one sanctioned exception to the module's copy-on-read convention. Milestone: `Dissolved`. |

## Tick — the world clock

Advances only because players act (monster-AI brainstorm: the roguelike
clock). v1 accrual rule is **high-water max-displacement**, built in (config
appears when a second rule exists): the clock tracks each driver's cumulative
reported displacement and a high-water mark; when a report raises the
high-water mark, the delta is granted to every member's budget. Four players
each moving 3 grants 3, not 12. Units are the caller's; the clock never
interprets them.

State/Data shape: `TickData{Budgets map[EntityID]int, DriverProgress
map[EntityID]int, HighWater int}`. Construct via `NewTick()`; a freshly
constructed `Tick` is valid and idle, but the zero value is not usable
(nil maps).

Idle-snapshot convention (both clock types): an idle clock's `ToData()`
is deep-equal to the zero Data value (nil, not empty-non-nil, containers)
and marshals to `{}`.

Two properties compositions must know: **`Ready` is a snapshot** (members
with budget > 0 at this instant), not a became-ready event — a member with
unspent budget reappears on every `Advance`, so schedulers must not treat
presence as "newly ready". **`DriverProgress` has no removal path** — it
grows monotonically with distinct driver IDs; entries with progress <=
`HighWater` are safely prunable by a future policy (over-grant is
structurally impossible either way: total budget ever granted equals
`HighWater`).

Queries: `Budget(in *BudgetInput) (int, error)` (`ErrNotMember` for an
absent member — never an ambiguous zero), `Members() ([]core.EntityID,
error)`, `Contains(in *ContainsInput) (bool, error)`.

| Verb | Input | Output | Semantics |
|------|-------|--------|-----------|
| `Join` | `{ID EntityID}` | `{Milestones}` | Adds a member at budget 0. MUST error if present. Milestone: `MemberJoined`. |
| `Leave` | `{ID EntityID}` | `{Milestones}` | Removes the member and its budget. MUST error if absent. Milestone: `MemberLeft`. |
| `Advance` | `{Driver EntityID, Displacement int}` | `{Milestones, Ready []EntityID}` | Records the driver's displacement (MUST error if `Displacement < 0`); grants any high-water delta to all budgets. `Ready` lists members with budget > 0 after the grant. Milestone: `Ticked{driver}` (emitted whether or not a grant occurred). Drivers are not members; a driver MAY also be a member. |
| `Spend` | `{ID EntityID, Amount int}` | `{Milestones}` | MUST error if the member is absent, `Amount <= 0`, or `Amount > Budget(id)`. |

## Transfer

The only interfaces in the module, extracted where sharing is proven. Their
methods are mutating verbs and follow R3 like every other verb:

```go
type Leaver interface {
    LeaveMember(in *LeaveMemberInput) (*LeaveMemberOutput, error)
}
type Joiner interface {
    JoinMember(in *JoinMemberInput) (*JoinMemberOutput, error)
}
```

| Shape | Fields |
|-------|--------|
| `LeaveMemberInput`  | `{ID core.EntityID}` |
| `LeaveMemberOutput` | `{Milestones []Milestone}` |
| `JoinMemberInput`   | `{ID core.EntityID, Pos int}` |
| `JoinMemberOutput`  | `{Milestones []Milestone}` |

`Turn` and `Tick` implement both (thin adapters over their own verbs; `Tick`
ignores `Pos`).

`Transfer` requires both sides to speak both seams:

```go
type Membership interface {
    Leaver
    Joiner
}
```

Both sides, not just the destination: typing either side asymmetrically
would encode the execution strategy into the API (see brainstorm.md,
"join-first with compensating leave"). Execution order is an
implementation detail; the observable contract — R6: both clocks
unchanged on failure; milestones reported leave-then-join — is what tests
assert.

| Func | Input | Output | Semantics |
|------|-------|--------|-----------|
| `Transfer` | `{From, To Membership, ID EntityID, Pos int}` | `{Milestones}` | Leave-then-join, R6 atomicity: on any failure both clocks are unchanged and an error returns. `From` and `To` MUST be distinct clocks (`ErrSameClock`) — without the guard, a self-transfer of an absent entity succeeds with phantom milestones while the entity ends on no clock, making execution order observable. The error propagates the underlying leave/join sentinel, dispatchable via `errors.Is`. Milestones: the concatenation of the leave's and the join's, in that order. |

## Errors

All errors wrap one of the package sentinels below, so callers dispatch
with `errors.Is`. Messages are user-facing (toolkit convention). For
mutating verbs a non-nil error means no state changed (R5); for read-only
functions it explains why no meaningful value exists.

| Sentinel | Meaning | Returned by |
|----------|---------|-------------|
| `ErrIdle` | clock has no order set / nothing to act on | `Active`, `Round`, `End`, `Insert`, `Merge`, `Dissolve`, `JoinMember` (Turn adapter) |
| `ErrNotActive` | the named actor is not the active entity | `End` |
| `ErrNotMember` | entity is not in this clock | `Budget`, `Spend`, `Remove`, `Leave`, `LeaveMember` |
| `ErrDuplicateMember` | entity already present | `SetOrder`, `Insert`, `Join`, `JoinMember` |
| `ErrBadPosition` | `Pos` outside `[0, len]` | `Insert`, `JoinMember` |
| `ErrBadOrder` | empty order, or `Merge.Order` not a permutation of the union | `SetOrder`, `Merge` |
| `ErrSameClock` | the operation requires two distinct clocks (`Merge.Other` is the receiver; `Transfer.From` is `Transfer.To`) | `Merge`, `Transfer` |
| `ErrBadAmount` | non-positive `Spend.Amount`, negative `Advance.Displacement` | `Spend`, `Advance` |
| `ErrInsufficientBudget` | `Spend.Amount` exceeds the member's budget | `Spend` |
| `ErrInvalidData` | any R9 rejection | `LoadTurn`, `LoadTick` |

## Acceptance criteria

- **AC1 (the DOS2 spine)** — one integration test: four members on a `Tick`;
  a `Turn` bubble forms via `SetOrder`; the distant members keep accruing
  from their own `Advance` reports; one `Transfer`s into the bubble at a
  chosen `Pos`; rounds advance with `End`; `Dissolve` returns members and
  they rejoin the `Tick`. The test asserts the full milestone transcript and
  final state.
- **AC2 (invariants)** — active index valid or clock empty after every verb
  sequence; milestone causal order; `Transfer` failure leaves both clocks
  byte-identical (`ToData` compare).
- **AC3 (round-trip)** — `ToData` → `LoadTurn`/`LoadTick` at every distinct state
  (idle, mid-round, post-merge, post-dissolve; tick with partial budgets)
  reproduces behavior-identical clocks; R9 rejections each have a test.
- **AC4 (compat gate)** — `gorelease`/`apidiff` wired into CI from the first
  tag.
- **AC5 (suite conventions)** — black-box `package clock_test`, testify
  suite, no mocks (the module has nothing to mock).
- **AC6 (error vocabulary)** — every sentinel has at least one test
  asserting `errors.Is` dispatch from the verb that returns it.
