# ADR-0043: A Monster's Turn Has a Driver — a Required, Composition-Level Capability

**Date:** 2026-08-22
**Status:** Accepted (implemented; reconciled against shipped code and DECISIONS.md, 2026-08-28)

## Context

In the reference tomb the fighter sights skeleton-1, the fight forms with
initiative order `[fighter, skeleton-1]`, the fighter calls `EndTurn` and
`Next` becomes `skeleton-1`. Nothing can end skeleton-1's turn:
`encounter.EndTurn` requires `Member` to be the bubble's own active
member, and the host (rpg-api) binds `Member` to the authenticated caller
— a human who does not own the skeleton. The clock parks on skeleton-1
forever. No swing, no move, no `EndTurn` reaches it. **The fight cannot be
played through**, which blocks the whole combat track. Observed live by
Kirk, 2026-08-22. Filed as rpg-toolkit#1162.

The same gap has a second face: if initiative rolls the skeleton FIRST,
nothing is active when the fight forms either. The fix has to cover both
moments the clock can land on an unplayed member — a turn ending, and a
fight beginning.

## What already exists, and what it rules out

`rulebooks/dnd5e/encounter` already has a monster-intelligence seam:
`Decider`, consulted by `Pump` (the free-roam world tick). Its own README
states the boundary in so many words: **"A monster in a fight is not
consulted... its decider is skipped entirely until the fight dissolves."**
`Decider`'s vocabulary (`Snapshot` → `IntentMoveTo | IntentHold`) is
scoped to *where to be*, not *what to do on a turn* — the same README:
"There is no attack intent... attacking, targeting and the action economy
are wave-4 work." So the gap this ADR closes cannot be closed by widening
`Decider`; it needs its own seam, and that seam has to compose with the
existing turn machinery (`encounter.EndTurn`, `bubble.End`) rather than
duplicate it.

`MemberKind` (`KindPlayer` / `KindMonster`) is already a composition-level
fact, checked today at `NewEncounter`/`LoadEncounter` construction time
("Players cannot carry deciders," design law C2). So the first open
question in the brief — is "has no player" composition-known or
host-declared — is settled by what already ships: **it is
composition-known.** `KindMonster` already IS "has no player" everywhere
else this seam draws that line; inventing a second, host-declared flag for
the identical fact would be two answers to one question.

## Options

**1. The SDK yields for unplayed members on its own, with no supplied
capability.** After `EndTurn`, while `Next` is `KindMonster`, `session`
(or `encounter`) auto-ends that turn, recording "does nothing" beats,
until the clock reaches a `KindPlayer` member. Simplest to write. Rejected:
it bakes a game rule — *a monster with no behavior does nothing* — into
a layer whichever package holds it. If it lands in `session`, it is
exactly the violation `session`'s own charter forbids ("holds no rules").
If it lands in `encounter` as an unconditional behavior with no seam, it
is the same defect one layer down: the Monster AI initiative
(rpg-project#201/#202) will need to REPLACE this "always pass" rule with
a real decision, and replacing hardcoded behavior means finding every
place it was assumed and ripping it out — the cost option 3 exists to
avoid.

**2. The host drives it.** rpg-api calls `EndTurn` *as* the monster,
which needs verb authorization it does not have yet (rpg-api#803: today
`EndTurn`'s `Member` is bound to the authenticated caller, a human).
`session` and `encounter` stay pure. Rejected as the primary answer: every
host that ever embeds this SDK re-implements "an unplayed member passes,"
and that re-implementation is itself a game rule, now living in N hosts
instead of one seam. It also does not explain the fight-start case
cleanly — the host cannot call `EndTurn` on a member whose turn has not
even been reached by any host-visible verb yet, since fight formation
happens *inside* whichever verb (`Move`, `Attack`, `Spawn`) triggered
contact, synchronously, before that verb returns.

**3. A capability the host supplies, never defaulted.** The composition
gains a required capability — `TurnDriver`, alongside `Standing`, `Sight`,
`Initiative` — consulted at the exact moment the clock lands on a
`KindMonster` member: after `EndTurn`'s own `bubble.End`, and after `form`
builds a fresh bubble. v1's shipped implementation is `Pass` — yield, no
side effect beyond the clock advancing and a beat being recorded. Later,
the Monster AI initiative supplies a real driver through the identical
seam; nothing in `encounter` or `session` changes shape to receive it.

**Recommendation: 3.** It is the only option that keeps `session` and
`encounter` free of a game rule about what unplayed members do *while also*
actually closing the gap — option 1 only avoids the rule if nobody
implements it, which contradicts the fight staying playable, and option 2
relocates the rule rather than removing it.

## The counter-evidence, thrown honestly

`Decider` is the closest precedent for a per-member behavior capability in
this package, and it is **optional, defaulting silently to hold**:
`Pump`'s loop reads `decider, hasDecider := e.deciders[m.ID]; if
!hasDecider { continue // no decider = hold }`. That is a real, shipped
"capability defaulted" — the opposite of the law this ADR otherwise
leans on. Two questions follow from taking it seriously rather than
explaining it away.

**Does it argue for making `TurnDriver` optional too, defaulting to
`Pass`?** No, and the reason is the blast radius of getting it wrong. A
monster with no `Decider` that silently holds forever affects exactly one
monster, in free roam, where "stands still" is an ordinary and fully
recoverable state — nothing else in the encounter depends on that monster
ever moving. A member with no `TurnDriver` that the clock lands on
**stalls the entire bubble for every member in it, permanently** — the
exact defect this ADR exists to fix, reintroduced as an unexercised
default instead of a compile-time or construction-time refusal. The two
capabilities look alike (both answer "what does an unplayed member do")
and differ on the one axis that decides whether silence is safe:
`Decider`'s absence is locally inert; `TurnDriver`'s absence is globally
blocking. `Standing`/`Sight`/`Initiative` are required for the identical
reason — their absence does not degrade one member's behavior, it makes
the encounter unable to answer a question every fight needs answered.
`TurnDriver` sits with them, not with `Decider`.

**Does `Decider`'s existing shape (per-member, supplied at `MemberInput`)
argue `TurnDriver` should be per-member too?** No — and this is a
scoping choice worth stating rather than leaving implicit. `Decider`
varies genuinely per monster (a ghoul flees, a skeleton presses); v1's
`TurnDriver` answer is the SAME for every unplayed member — always
`Pass` — so there is nothing yet to key per-member. `TurnDriver` is
therefore shaped like `Standing`/`Sight`/`Initiative`: one capability per
encounter, asked with the member's ID, exactly as `Standing.Standing`
already takes a member list rather than being supplied per-member. When
the Monster AI initiative needs per-monster variation, that is the
driver's OWN internal business — it can consult a monster's stored
behavior data keyed by its ID, same as any other decider will — and does
not require a second capability shape at this seam.

## Decision

### Where it lives, and its shape

`rulebooks/dnd5e/encounter` gains a new, required capability, alongside
`Standing`/`Sight`/`Initiative` on both `SetupInput` and
`LoadEncounterInput`:

```go
// TurnDriver decides what a member with no player does when the fight's
// clock lands on their turn.
type TurnDriver interface {
    Act(member MemberID) (TurnOutcome, error)
}

// TurnOutcome is a sealed vocabulary (unexported marker method), the same
// shape Intent already uses for Decider. v1 has exactly one case.
type TurnOutcome interface{ isTurnOutcome() }

// Pass is the only outcome a v1 driver may return: the member's turn ends
// with no other effect. A second case — an attack, a move — arrives with
// the caller that needs it (the Monster AI initiative), the same way
// Decider's own Intent grew IntentMoveTo before IntentHold existed.
type Pass struct{}
func (Pass) isTurnOutcome() {}
```

Refused at construction exactly like `Standing`/`Sight`/`Initiative`: a
nil `TurnDriver` on `SetupInput` or `LoadEncounterInput` returns a new
`ErrNoTurnDriver`, never silently substituted. **The wire does not "still
work without one"** — see the counter-evidence above for why that would
recreate the stall this ADR closes, one layer later.

### Where it is consulted: one choke point, four callers

A single unexported helper, `driveMonsterTurns`, walks the bubble forward
past every consecutive `KindMonster` member starting from its current
Active member, consulting `TurnDriver.Act` and (for v1, since `Pass` is
the only outcome) calling `bubble.End` for each one — recording the SAME
`"turn-ended"` beat `EndTurn` already produces, so the story and the
`EventTurnEnded` stream need no new vocabulary. It stops at the first
`KindPlayer` member, or returns `ErrNoPlayerInBubble` if the bubble
contains no player at all.

**`EndTurn` and `form` call it directly**, and for them `ErrNoPlayerInBubble`
really is defensive-only: a bubble only ever forms on player-monster
contact, so an all-monster order is unreachable through trigger detection
(pinned white-box, `TestFormRefusesAPlayerFreeBubble`, since nothing else
can reach it).

**Two more callers turned up during implementation, and they change the
"defensive" claim's scope.** The active slot does not only move on
`EndTurn`/`form` — it can also move when a member LEAVES a running bubble:
`Transfer(To: ClockWorld)` (a straggler stepping back out, or `noticeDown`
splicing a fallen body out from under a fight it was active in) and `Exit`
(a member leaving the encounter entirely, via `leaveAnyClock`). Both can
hand the active slot to whoever the departing member left behind — exactly
the same problem `EndTurn` has, one call late. A shared
`driveIfStillRunning(bubble)` wraps `driveMonsterTurns` for these two:
skip if the bubble is now idle (the fight is over, not stuck), and —
UNLIKE the first two callers — skip rather than error if the bubble now
holds no player at all, because that IS reachable here: `Exit` can drain a
fight's last player before draining its last member, and a bubble with a
lone surviving monster is an existing, tolerated intermediate state
(`TestADrainedBubbleIsPruned`'s "a fight of one is still a fight"). Nobody
is left to hand a driven-through turn back to, so there is nothing to do —
`bubbleHasPlayer` is the one predicate both `driveMonsterTurns` and
`driveIfStillRunning` share, read two different ways by design.

So, four call sites, one decision function:

- **`EndTurn`**, after its own `bubble.End` for the acting member: if the
  new Active member is `KindMonster`, `driveMonsterTurns` runs before
  `EndTurn` returns. `EndTurnOutput.Next` is therefore already the next
  PLAYED member — Kirk's lean, confirmed: the caller learns the truth in
  one round trip, and the intervening passes are beats in the stream
  rather than a second call the host has to make. `RoundWrapped` is true
  if ANY step in the chain wrapped the round, and `Seq` is the last beat's
  sequence — a client that wants every intermediate beat already has it
  through the ordinary baseline-and-fan-out mechanism every other verb
  uses (`Story`, the event stream), so `EndTurnOutput`'s shape does not
  need to grow a list.
- **`form`**, after appending the formation beat: if the FIRST Active
  member (`Order[0]`) is `KindMonster`, `driveMonsterTurns` runs before
  `form` returns — AFTER the `"bubble-formed"` beat, deliberately, so a
  reader replaying the story sees the fight announced before any turn
  inside it can end. This is the fight-start case the brief calls out
  explicitly — a fight where the skeleton rolls first resolves to the
  fighter being Active by the time `Formed`/`FIGHT_STARTED` reaches any
  client, with no session-level change needed: `form` is unexported and
  already runs inside whichever verb (`Move`, `Attack`, `Spawn`) triggered
  contact, so every trigger path gets the fix for free.
- **`Transfer`**, ClockWorld direction, after its own `dropBubbleIfIdle`:
  `driveIfStillRunning` covers both a straggler stepping back out mid-fight
  and `noticeDown`'s splice of a fallen body, since the latter is
  implemented AS a `Transfer` call — one fix covers both.
- **`leaveAnyClock`** (Exit's own removal path), after its
  `dropBubbleIfIdle`: the same coverage for a member leaving the
  encounter entirely while active in a fight.

**A driver error aborts the whole call, including the acting member's own
already-applied `bubble.End`.** This falls out of the load–mutate–save
shape every verb in this SDK already has, for free: nothing is persisted
until the verb's own `commit`, so a `driveMonsterTurns` failure partway
through simply means the in-memory encounter (with its half-executed
pass chain) is discarded and the STORED world is untouched — the caller
can retry, and a broken driver fails loudly at the first turn it would
have mishandled rather than corrupting a fight's state. This mirrors
`Pump`'s own rule for `Decider` errors ("aborts atomically... no clock
advance, no moves, no beats") using the mechanism this seam already has
rather than inventing a second one.

### What `session` and rpg-api must do

`session.Config` gains a required field, a session-owned interface
(never `encounter.TurnDriver` directly — S2, the same reason `Config`
takes `Roller` rather than `encounter.InitiativeRoller`):

```go
type TurnDriver interface {
    Act(member string) (TurnOutcome, error)
}
type TurnOutcome interface{ isTurnOutcome() }
type Pass struct{}
```

`NewManager` refuses a nil `Config.TurnDriver` (`ErrIncompleteConfig`,
naming the field, exactly like `Sessions`/`Encounters`/`Dice`), and wraps
it in a small adapter (`turnDriverSeam`, the same shape `initiativeSeam`
and `sightSeam` already are) satisfying `encounter.TurnDriver`, threaded
into all three `encounter.LoadEncounter` call sites
(`loadWorldWithBaseline`, `StartSession`'s validation load, `adopt`).

**rpg-api must supply `session.Config.TurnDriver`.** For v1 this is one
line: the SDK ships `session.Pass{}`, satisfying `TurnDriver` by always
returning `Pass, nil` regardless of which member is asked — rpg-api wires
`Config.TurnDriver: session.Pass{}` at `NewManager` construction, the same
place it already wires `Dice`/`Events`/the repositories. No new rpg-api
verb, no new authorization, no proto change: rpg-api#803 (verb
authorization for playing a monster) is NOT a prerequisite for this fix —
that ticket is about a host that wants a HUMAN to puppet a monster, a
different feature this ADR does not touch.

## Consequences

### Positive
- The fight is playable end to end without any rpg-api change beyond
  wiring one required, already-supplied-shape field.
- The Monster AI initiative's future work is additive: a new
  `session.TurnDriver` (or its own package) implementation, wired into the
  same `Config.TurnDriver` field, with no change to `encounter` or
  `session`'s call sites — the seam Kirk's option 3 promised.
- `EndTurnOutput` and `Formed`'s existing shapes are unchanged; the fix is
  invisible on the wire except that `Next`/`Active` now name a member with
  a player, which is the bug being fixed rather than a new field to learn.

### Negative
- Every host embedding this SDK (today: only rpg-api) has one more
  required `Config` field to wire before `NewManager` succeeds — a
  breaking change for any host that has already adopted `session` (today:
  none have finished the migration, so this lands free, the same note
  `Config.Characters`'s own doc makes about required fields).
- `driveMonsterTurns`'s loop bound (`len(Order)` iterations,
  `ErrNoPlayerInBubble` past that) is a defensive check against a state
  the rest of the module currently guarantees cannot occur. If that
  guarantee is ever weakened, this becomes the first place it is
  noticed — which is the intended failure mode, not a design smell.

### Neutral
- `TurnOutcome` is a sealed vocabulary with one case today, matching
  `Intent`'s own growth path. A second case (an attack outcome) is real
  vocabulary growth and, following this repo's established practice for
  sealed vocabularies at this seam (ADR-0038's rule for `Gather | Pose |
  Request | Done`), should probably earn its own ADR when it lands rather
  than being added quietly.

## Rule

**A capability whose absence is locally inert may default; a capability
whose absence blocks the whole composition may not.** `Decider`'s
optional, default-to-hold shape and `TurnDriver`'s required,
refuse-at-construction shape are not in tension — they are the same law
applied to two different blast radii.
