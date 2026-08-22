# ADR-0042: Afford Answers in Declarations, Not Remaining Currencies

**Date:** 2026-08-22
**Status:** Accepted (Kirk, 2026-08-22 — "oh I like B a lot. backend tells
dumb client what it can do.")

## Context

`session/v0.17.0` made the action economy real: a second swing in a turn
that bought only one is refused with `ErrCannotAfford`, and the sentinel
carefully names the currency that ran out — "action: 1 needed, 0 left" —
because "a refusal a client cannot explain is a refusal that reads as a
bug." All of that is about the refusal. Nothing on the seam reports the
budget *before* it.

There is no read for what a member can still afford. `Turn` returns
`{Clock, Active, Round, Order}`. `Status` is `{Open, Outcome}`. `View`
reports perception. A client building a turn UI has exactly two options:
offer every action and let the server refuse it — the button that fails,
fine for a debug harness and not for a game — or re-derive the economy
client-side: know that a level-1 fighter gets one attack, that Extra
Attack lands at level 5, that a bonus action is not an action. That is a
D&D rule living in the client, the Boundary Rule violation this whole seam
exists to prevent. Filed as rpg-toolkit#1138, and the shape it names in
passing ("What a client needs to render is closer to *what can I still
declare* than *how many points do I have*") is what this ADR settles.

## Options considered

1. **Remaining currencies.** `{action: 0, bonus: 1}` — the ledger, read
   back. Cheapest to build: it is `combat.SpendProfile`'s own vocabulary
   with nothing translated. Rejected on inspection: a client reading it
   still has to know that a swing *costs* an action and that Extra Attack
   *banks* capacity to turn those numbers into "can I swing" — the rule
   would leak through the very read meant to keep it server-side. The
   D&D-in-the-client problem does not go away; it moves one read earlier.

2. **Declarations.** One entry per verb the seam prices — `{Verb, Slot,
   Affordable, Shortfall}` — answering the client's actual question
   (*can I do this*) rather than handing over the ledger and trusting the
   client to derive the answer. `Slot` rides along so a client can still
   light the action/bonus/reaction shapes it already draws, without
   needing to know *why* — it is read off the same `SpendProfile.Slots`
   the door prices, never a second table.

3. **Both.** Declarations plus the raw remaining currencies, for a client
   that wants more than can-or-cannot. Rejected for v1: nothing asking for
   it exists yet, and shipping the currencies teaches every future client
   that reading them is legitimate — the same "re-derive the rule" failure
   mode option 1 has, just available as an opt-out rather than the only
   path. Nothing forecloses adding a currency-shaped read later if a real
   caller needs one; it is not this read's job to guess that caller into
   existence.

## Kirk's ruling

> oh I like B a lot. backend tells dumb client what it can do. in our ui
> we hand shapes for action, bonus and reaction that lined up with the
> various things we could do. that was really nice

Option 2, verbatim. The `Slot` field exists because of the second half of
the quote: a UI that already draws three shapes (action / bonus /
reaction) needs to know which one a declaration would spend, without the
seam handing over the currency ledger to compute it from.

## What this read does NOT decide

**Whose turn it is.** `Attack` does not check whose turn it currently is —
nothing on this seam does yet, `EndTurn` aside (verified by reading
`attack.go`: no consult of `ClockOfOutput.Active` anywhere in the
verb) — so `Afford` does not either. Folding "not your turn" into a
`Declaration.Shortfall` would answer a question this read was never asked,
in a currency it does not price: that is `Turn`'s question, and a future
turn gate belongs beside `Turn`'s own `Active` field rather than inside a
declaration meant to describe the economy alone.

**Weapon compilation.** `Afford` prices what `priceSwing` prices — the
turn's economy — and `priceSwing` never touches `resolution.AttackProfile`
or a weapon at all; that half of `Attack`'s compilation
(`compileAttack` → `resolution.AttackFromCharacter`) is a different
refusal (`ErrBadAttack`) about a different fact. Deciding whether an
unarmed or weaponless member's `Declaration` should say so is left for the
caller that needs it — v1's only caller is a client asking about its own
authenticated member, who has a weapon or does not swing at all.

## Decision

`Manager.Afford(ctx, *AffordInput) (*AffordOutput, error)`, in the spirit
of `Where`: a caller-scoped singular read, the host binding `Member` to the
authenticated caller exactly as `Where` requires.

```go
type AffordOutput struct {
    Clock        ClockKind      // ClockWorld: the economy does not apply; Declarations is empty and that IS the answer
    Declarations []Declaration  // one per verb the seam prices; v1: Attack only
}

type Declaration struct {
    Verb       Verb    // VerbAttack in v1
    Slot       Slot    // SlotAction | SlotBonus | SlotReaction | SlotNone — read off SpendProfile.Slots
    Affordable bool    // no omitempty; false is an answer
    Shortfall  string  // empty when Affordable; otherwise the SAME text the refusal carries
}
```

### The load-bearing invariant, and how it is kept true by construction

`Afford` must never become a second copy of what `Attack` already prices,
because two copies is exactly the shape that drifts. It is not: `Afford`
calls the identical `priceSwing` a real swing compiles from — the same
function, refactored to take an `*encounter.Encounter` rather than a
`*writeScope` so a read verb can call it too — and then charges that price
through `combat.Pay`, the SAME gate function `resolution`'s door pays a
real swing through. The sheet it charges is loaded fresh for the call and
handed to nobody: never adopted into the scope, never passed to
`saveDirty`, never reaching a repository. Charging it is therefore both
safe (nothing persists) and honest (it is the real gate, not a
read-only twin of it) — a payment that succeeds sets `Affordable`; one
that fails hands back `Pay`'s own error text as `Shortfall`, verbatim.

This was checked against an alternative — adding a read-only
`combat.Explain(Ledger, *SpendProfile) error` next to `CanPay`, wrapping
the same private `check` — and calling `Pay` on the throwaway sheet was
preferred: `combat` is a *different Go module* from `session`
(`rulebooks/dnd5e` vs. `rulebooks/dnd5e/session`), pinned by version in
`session/go.mod`, and this repo's own history sequences a lower-module
change and the session-side change that consumes it as separate PRs
(e.g. `9a6d1bb` / `e1f8ba7`'s "take the X correction" commits) rather than
landing both in one. `Pay` is already exported, already the exact function
the door calls, and calling it on a copy nobody saves needs no new export
and no cross-module version bump to ship this as one PR.

Proven by test as well as by construction (`afford_test.go`):
`TestAffordableMeansAttackWillNotRefuse`,
`TestUnaffordableMeansAttackRefusesWithTheSameShortfall`, walking the
level-1 fighter's second swing and Extra Attack's second and third swings
from `economy_test.go`'s own fixtures, plus `TestFreeRoamAffordsNothing`
and `TestAffordSavesNothing` (the no-persistence half, mirroring
`TestARefusedSwingWritesNothing`).

## Consequences

### Positive
- A turn UI can grey a button honestly instead of offering one the server
  will refuse, without knowing a single 5e rule.
- The rule stays server-side. A client that wanted to bypass `Declaration`
  and compute affordability itself has no currencies to read — there is
  nothing to re-derive from.
- `priceSwing`'s signature change (`*writeScope` → `*encounter.Encounter`)
  is a pure widening: `Attack` passes `scope.enc` and is otherwise
  unchanged, and the function's only real dependency was ever the
  encounter, not the write machinery around it.

### Negative
- `Declaration` carries no field for "why can't this verb even be
  attempted" reasons outside the economy (not your turn, no weapon,
  member is downed). A client that wants those has to call the verb and
  read the refusal, same as before this ADR.
- Calling `combat.Pay` (a mutating function) on a throwaway sheet to get a
  read-only answer is a slightly unusual use of an exported API — the
  alternative (`combat.Explain`) is cleaner in isolation but was rejected
  above for the cross-module reason. Worth revisiting if `combat` ever
  needs its own reason-returning read for another caller.

### Neutral
- v1 has exactly one verb to declare, so `Declarations` is always
  length-0-or-1 today. The shape does not change when a second verb (Dash,
  Dodge) is priced; it grows another entry.

## Rule

**A read that could leak a rule by handing over the ledger instead answers
the caller's actual question.** When a client's real question is
can-or-cannot, reporting the numbers behind the answer is not a more
complete version of the same read — it is a second, unofficial API for
deriving the rule the first one was built to keep server-side.
