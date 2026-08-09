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
- **R3** — Mutating verbs MUST have the shape
  `func (x *T) Verb(in *VerbInput) (*VerbOutput, error)`. Read-only queries
  MUST be plain methods.
- **R4** — Every verb MUST return all `Milestone`s it caused, in causal
  order, in its Output. The module MUST NOT publish, call back, or otherwise
  deliver milestones.
- **R5** — Verbs MUST be atomic: on a non-nil error, clock state is
  unchanged.
- **R6** — An entity belongs to at most one clock at a time. `Transfer` MUST
  leave both clocks unchanged when it fails.
- **R7** — The module MUST NOT contain randomness. All orderings arrive from
  the caller (`SetOrderInput.Order`, `InsertInput.Pos`, `MergeInput.Order`).
- **R8** — Dynamic state MUST round-trip via `ToData`/`LoadFromData`
  (plain JSON-serializable structs, no behavior). Configuration MUST be
  re-supplied at construction and MUST NOT serialize.
- **R9** — `LoadFromData` MUST reject invalid state with an error: duplicate
  members, `ActiveIdx` out of range, negative budgets.
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

## Turn — the initiative bubble

State: an ordered member list, an active index, a round counter.
Data shape: `TurnData{Order []core.EntityID, ActiveIdx int, Round int}`.
A zero/empty `Turn` is valid and idle.

Queries: `Active() core.EntityID` (zero when empty), `Round() int`,
`Order() []core.EntityID`, `Contains(id) bool`.

| Verb | Input | Output | Semantics |
|------|-------|--------|-----------|
| `SetOrder` | `{Order []EntityID}` | `{Milestones}` | Replaces the order (rulebook rolled initiative). Round becomes 1, first member active. Milestones: `RoundStarted{1}`, `TurnStarted{first}`. MUST error on duplicate IDs or empty order. |
| `End` | `{Actor EntityID}` | `{Milestones, Next EntityID, RoundWrapped bool}` | MUST error unless `Actor == Active()`. Advances. On wrap: increments Round; milestones `TurnEnded{actor}, RoundStarted{n}, TurnStarted{next}`; otherwise `TurnEnded, TurnStarted`. |
| `Insert` | `{ID EntityID, Pos int}` | `{Milestones}` | Fall-in / reinforcement at caller-chosen position. MUST error if ID present or `Pos` outside `[0, len]`. Inserting at or before the active position MUST keep the currently active entity active. Milestone: `MemberJoined`. |
| `Remove` | `{ID EntityID}` | `{Milestones}` | Death/flee. MUST keep the active entity correct: removing a non-active member adjusts the index; removing the active member makes the next member active (milestones `MemberLeft, TurnStarted{next}`); removing the last member leaves the clock empty (`MemberLeft` only). |
| `Merge` | `{Other *Turn, Order []EntityID}` | `{Milestones}` | Two bubbles collide. `Order` MUST be a permutation of the union of both member sets (error otherwise). The receiving clock's active entity MUST remain active; its Round is retained. `Other` is drained to empty. Milestone: `Merged`. |
| `Dissolve` | `{}` | `{Members []EntityID, Milestones}` | Fight over. Empties the clock, returns the members for the composition to re-home. Milestone: `Dissolved`. |

## Tick — the world clock

Advances only because players act (monster-AI brainstorm: the roguelike
clock). v1 accrual rule is **high-water max-displacement**, built in (config
appears when a second rule exists): the clock tracks each driver's cumulative
reported displacement and a high-water mark; when a report raises the
high-water mark, the delta is granted to every member's budget. Four players
each moving 3 grants 3, not 12. Units are the caller's; the clock never
interprets them.

State/Data shape: `TickData{Budgets map[EntityID]int, DriverProgress
map[EntityID]int, HighWater int}`.

Queries: `Budget(id) int`, `Members() []core.EntityID`, `Contains(id) bool`.

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

| Func | Input | Output | Semantics |
|------|-------|--------|-----------|
| `Transfer` | `{From Leaver, To Joiner, ID EntityID, Pos int}` | `{Milestones}` | Leave-then-join, R6 atomicity: on any failure both clocks are unchanged and an error returns. Milestones: the concatenation of the leave's and the join's, in that order. |

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
- **AC3 (round-trip)** — `ToData`/`LoadFromData` at every distinct state
  (idle, mid-round, post-merge, post-dissolve; tick with partial budgets)
  reproduces behavior-identical clocks; R9 rejections each have a test.
- **AC4 (compat gate)** — `gorelease`/`apidiff` wired into CI from the first
  tag.
- **AC5 (suite conventions)** — black-box `package clock_test`, testify
  suite, no mocks (the module has nothing to mock).
