# ADR-0044: Move on the Turn Clock — the Active Member Walks and Spends Movement

**Date:** 2026-08-22
**Status:** Proposed (implementing the recommended option; Kirk to rule)

## Context

A fight forms the moment two sides see each other, and from then on
`encounter.Step` refuses **every** member caught in the bubble outright:

> A member caught in a bubble acts through the fight's own turn structure
> — there is no in-fight movement verb yet, and until there is, a fight
> member cannot move at all rather than moving outside initiative.

`Turn`, `EndTurn` and the `Pass` driver (#1162, ADR-0043) made the clock
itself playable — a fighter can end her turn and a skeleton's turn passes
automatically — but nobody can actually take a step while it runs. Kirk,
2026-08-22: *"it feels like we are rushing to the attack but I think
better would be operating in the new clock. take turn, move maybe, end
turn monster takes turn."* This closes that gap.

## What already exists (verified, not assumed)

- **Movement is already a keyed capacity.** `combat.CapacityMovement` is
  a full member of the closed `CapacityType` enum
  (`combat/capacity.go:37`), and `character.Character` already
  implements `CapacityLeft`/`SpendCapacity`/`BankCapacity` for it
  (`character/ledger.go:115-147`), reading and writing
  `ActionEconomyData.MovementRemaining`, seeded from `GetSpeed()` at
  `StartTurn`/`RefreshForTurn`. `combat.Pay`/`CanPay` — the identical gate
  `priceSwing` charges attacks through — already checks and spends this
  currency for free: nothing in `combat` or `character` needs to change.
- **Nothing prices a path today.** The economy-gate ruling's "fork (d)"
  is v1's honest answer — *nothing charges movement, because there is no
  in-fight movement verb to charge* — recorded as settled-not-decided in
  the movement design addendum
  ([#1035 comment][addendum]): *"If Kirk later wants in-fight movement
  earlier than reactions, it is a new slice (E8) and not a re-litigation
  of E1."* This is that slice.
- **"Not your turn" already has an unnamed refusal.** `EndTurn`'s own doc
  says so plainly: *"the bubble's own rejection when it is not this
  member's turn (`clock.ErrNotActive`, with no state change)"* —
  `play/clock`'s own sentinel, surfacing through `encounter.EndTurn`
  unwrapped (`fmt.Errorf("end turn %q: %w", in.Member, err)` preserves it
  verbatim) and through `session`'s own `translate()`, which has no arm
  for it. No test asserts a specific sentinel today
  (`TestEndingSomebodyElsesTurnIsRefused` checks only `s.Require().Error`).
  A caller who wants to match on it has nothing but `errors.Is(err,
  clock.ErrNotActive)` — reaching into a leaf module two layers down, the
  exact S2 leak `translate`/`translateResolution` exist to prevent
  everywhere else.
- **A bubble has no spatial extent.** It is pure membership and order
  (`*clock.Turn`) — positions live on the canvas, entirely separately.
  Nothing "the fight's reach" exists to bound a walk against; the only
  constraints on where an active member can step are the ordinary ones
  `Step` already applies (floor, walls, doors).
- **A second fight cannot form while one is in progress.** Policy is one
  bubble per encounter, enforced at `form`'s own door
  (`ErrInBubble` if `len(e.bubbles) > 0`). A member already in a bubble
  therefore cannot walk into a second one — the "does a walk on the turn
  clock ever trigger a NEW fight" question the brief raised is
  unreachable by construction, not a case this ADR has to design for.

[addendum]: https://github.com/KirkDiggler/rpg-toolkit/issues/1035#issuecomment-5326344695

## Decision

### 1. Who may move: the active member, and only the active member

`encounter.Step`'s bubble gate changes from an unconditional refusal to:
refused unless the stepper is their bubble's own Active member. A member
whose turn it is not gets a **named** refusal — `encounter.ErrNotActive`,
translating `clock.ErrNotActive` at the boundary the same way every other
leaf sentinel is translated at a module seam (S2) — and `session` grows
the matching `ErrNotYourTurn`, wired into `translate()`. **`EndTurn`'s
existing, unnamed refusal is fixed to use the same sentinel in the same
commit** rather than left as a second, undocumented spelling of the
identical fact — the brief's own question ("should it become a named
sentinel now that two verbs share it") answers itself once two verbs
share it. `TestEndingSomebodyElsesTurnIsRefused` gains the `ErrorIs`
assertion it was always missing.

### 2. What it costs: a profile compiled per call, not per sheet

Movement's cost is **not** a static, class-derived compile like
`character.CostOfStrike` — it depends on the *path the caller asked for*,
which nothing about the actor determines. `session.Move`, when the walker
is on the turn clock, computes `feet := 5 * len(path)` — the RAW 5e
default, deliberately not the diagonal or difficult-terrain variants
(neither is modeled anywhere in this stack; the addendum's own "Edges,
named" section already reserves that seam for later without building it)
— and builds `&combat.SpendProfile{Capacity: {CapacityMovement: feet}}`
inline. No `character.CostOfMove` compiler is needed: unlike an attack,
where a level-5 fighter's Attack action banks a class-table fact
(`CostOfAttack`/`CostOfStrike` exist because the RULE varies by class),
a step's cost is `len(path) * 5`, a constant every caller can compute —
compiling it centrally would add a function with nothing rulebook-shaped
in it.

**Charged through the identical gate `priceSwing` uses, never a second
one.** `readyForTurn` (already shared, unmodified) lights a cold sheet or
refreshes a stale one so `MovementRemaining` reflects the CURRENT turn's
speed before anything is priced — same ignition problem, same fix,
reused verbatim. `combat.CanPay` checks the whole path's cost against the
whole budget; a path that cannot be paid for whole is refused whole,
**before any step is taken** — matching R5 and the identical "a caller
who mis-computed a route wants none of it" law `validateWalk` already
states for adjacency. Only once `combat.CanPay` says yes does
`combat.Pay` spend it and `runWalk` begin.

**The shortfall text says feet, deliberately more than the gate's own
generic wording.** `combat.check()`'s native message —
`"movement: 15 needed, 10 left"` — is unit-less by construction (the
same string family serves `"action: 1 needed, 0 left"`, where a bare
number is the natural reading). Movement is the one currency this seam
prices in a real-world unit a player actually says out loud. `session`
composes `"movement: %d ft needed, %d ft left"` itself, from the SAME two
numbers the gate already computed — the requested `feet` and
`sheet.CapacityLeft(CapacityMovement)`, read straight off the ledger the
gate just checked — never a re-derivation of *whether* it's affordable
(that verdict is `combat.CanPay`'s alone), only a friendlier spelling of
numbers already in hand. This mirrors, not violates, the "never a second
copy of the arithmetic" law: the AFFORDABILITY judgment has exactly one
source; the TEXT is formatted once, at the one call site that needs
units.

**This retires fork (d).** `docs/ideas/session-sdk/economy-gate.md`'s
status line is updated to record it: v1 now prices movement on the turn
clock; free roam remains, and stays, unpriced.

### 3. Free roam: untouched

`Step`'s relaxed gate only changes behavior for a member `bubbleFor`
finds — a free-roaming member has none, so nothing about their `Move`
call changes: no active check, no price, no budget. This is the same
"the two are the same value today" caution `runWalk`'s own comment
already keeps about coupling a fact that is true today to a fact that is
guaranteed.

### 4. `Afford` grows `VerbMove`, and `Declaration.Remaining`

A `VerbMove` declaration reports whether the active member can move at
all right now (false only when `MovementRemaining == 0` — the "1 needed"
of any distance already exceeds a zero budget) — but a bare
`Affordable` bool is a worse answer for Move than it is for Attack. A
client drawing a path preview needs a number to bound it against, not
just a yes: **`Declaration.Remaining *int`** (feet), present (even at
`*0`) for `VerbMove`, `nil` for `VerbAttack` — the same false-vs-absent
law the rest of this seam already keeps (types.go), applied to a field
that exists for exactly one verb rather than to a bool that exists for
all of them. `Slot` for `VerbMove` is always `SlotNone`: 5e movement is
not a per-turn slot cost (Dash spends a slot to grant more of it; a bare
step spends none), so nothing about it lights the action/bonus/reaction
shapes `Slot` exists to draw.

### 5. Reach (#1010): explicitly not this slice

Once positions move under a clock, "is the target within 5 feet" becomes
a fact `session.Attack` could check — but wiring it is the attack
wave's work, not this one's, exactly as the brief scopes it. Noted here
so the connection is not rediscovered cold.

## Options considered for the "who may move" gate

1. **Leave `Step`'s refusal blanket, add a parallel in-fight-move verb.**
   Rejected: a second verb duplicates `runWalk`'s whole shape (path
   validation, doorway crossing, discovery merging, ending detection) for
   a difference that is exactly one gate condition. Two verbs also means
   two places a future rule (reach, forced movement) has to be taught.
2. **Relax `Step`'s gate to "active member only" — chosen.** One verb,
   one gate, the difference between free roam and a fight member's turn
   is a single additional check where the composition already holds the
   answer (`bubbleFor` + `bubble.Active()`).
3. **Gate at `session`, leave `Step` blanket-refusing.** Rejected:
   `session` cannot decide this — "is it this member's clock turn" is a
   composition fact (`ClockOf`/`EndTurn` already live there), and
   duplicating it at the session layer would be a second copy of a
   membership check the composition already owns, the identical
   objection ADR-0043 raised against putting the pass-driver rule one
   layer in the wrong direction.

## Consequences

### Positive
- The turn clock is finally playable end to end: take a turn, move
  (maybe), end the turn, watch the monster's pass go by.
- `EndTurn`'s "not your turn" refusal gets a real, matchable sentinel for
  the first time — a latent S2 gap this ADR closes as a side effect of
  needing the same fact twice.
- No `combat`, `character`, or `resolution` changes: `CapacityMovement`
  was already fully wired, and movement's cost needs no resolution
  machine (no reaction moments, no multi-participant custody) — a
  session-layer price-then-step, the same shape `priceSwing` already
  proved for attacks.

### Negative
- `encounter.Step`'s contract changes for anyone who was relying on the
  blanket "a bubble member cannot move" refusal as a signal rather than a
  gap — reachable only by reading the exact sentinel, so a caller
  matching on `ErrInBubble` today is unaffected; one matching on
  "any error" for that case now needs to know it can also mean
  `ErrNotActive`.
- Two Go modules (`rulebooks/dnd5e/encounter` then
  `rulebooks/dnd5e/session`) in sequence, same as ADR-0043's chain,
  though shorter — no `resolution` PR this time.

### Neutral
- 5 ft/cell, uniform, no diagonal or difficult-terrain variant. Matches
  the addendum's own reservation ("per-unit cost stays out of the
  profile") — the day either arrives, only the `feet := ...` computation
  changes; the profile shape and the gate do not.

## What rpg-api and web need to know

- **New sentinel → status code.** `session.ErrNotYourTurn` needs the same
  status-code mapping `ErrCannotAfford` already has (a rules refusal, not
  a caller defect) — `FAILED_PRECONDITION` or equivalent, not
  `INVALID_ARGUMENT`.
- **`Declaration.Remaining` → a new optional proto field**, `int32`,
  matching the `Shortfall`/`Slot` fields' own optional/wrapper treatment
  already on `Declaration` — present for a Move declaration, absent for
  an Attack one.
- **No new verb, no proto surface change to `MoveEntity`/`Move` itself**
  — the existing RPC gains new refusal cases (`ErrNotYourTurn`,
  `ErrCannotAfford` naming movement) rather than a new shape.
