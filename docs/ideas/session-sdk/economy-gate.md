# The economy gate: where an action's cost lives

**Date:** 2026-08-18
**Status:** open question space. Nothing here is ratified. This is the
trade-off conversation that happens *before* E1 freezes the spend vocabulary.
**Decides:** nothing. The evidence is the [#1035 census][census] and its
[supplement][supp] and [movement addendum][move]; this doc does not re-derive
any of it. What it does is lay four shapes side by side and walk each one
through the same cases.

**Verified against** `main` at `b3287ae`.

[census]: https://github.com/KirkDiggler/rpg-toolkit/issues/1035#issuecomment-5326154182
[supp]: https://github.com/KirkDiggler/rpg-toolkit/issues/1035#issuecomment-5326242250
[move]: https://github.com/KirkDiggler/rpg-toolkit/issues/1035#issuecomment-5326344695

## The question

Nothing in the new stack spends anything yet, so the cost of an action has no
home and no owner. Before the first spend exists we get to choose the shape
rather than inherit it — and the shape is not obvious, because the cost of an
action is three separable things: **where the cost is written down** (a
profile, compiled per actor, because a monk's bonus strike and a rogue's Dash
cost differently from the same nominal action), **who checks it can be paid**
(before anything happens, or at the moment the thing happens), and **who
debits the ledger** (and onto whose sheet). Kirk's framing puts it as a gate:

> There's a gate to get into the machine and that gate has the cost. maybe more
> than one cost. maybe that cost is dynamic based on the class… when an action
> is going to be taken we should know what it cost and the action economy is
> where that cost is paid from.

with two riders that shape everything below:

> A machine maybe doesn't have a cost and that's okay. it can just be taken.

> Character shaped gates is really important. the wizard spends their reaction
> for that turn only if they have it. if they used it on their turn there is
> nothing to pause for. if they do spend it it comes off their sheet.

## Four shapes

```mermaid
flowchart LR
    subgraph A["A — spend MACHINE"]
        A1["Resolve(Machine: Spend)"] --> A2["Spend debits"] -->|Request| A3["Strike"]
    end
    subgraph B["B — gate at the DOOR"]
        B1["Resolve(Cost, Machine)"] --> B2["door pays Cost"] --> B3["Strike"]
    end
    subgraph C["C — gate as a SERVICE"]
        C1["Resolve(Gate, Machine)"] --> C2["door consults Gate"] --> C3["Strike"]
        C3 -.->|"reaction moment"| C4["consult Gate again"]
    end
    subgraph D["D — ledger travels with the CAST"]
        D1["Resolve(Participants)"] --> D2["Strike holds the sheets"]
        D2 --> D3["combat.Pay(sheet, profile)"]
        D2 -.->|"reaction moment"| D4["combat.CanPay(wizard's sheet)"]
    end
```

- **(A) A spend machine wrapping the strike.** The census's F6-literal sketch:
  a machine whose steps are "debit, then `Request` the interaction below".
- **(B) A gate at `Resolve`'s door.** A compiled cost profile rides `Input`
  beside the machine, and resolution debits before calling `Machine.Start`.
- **(C) A gate as a service resolution can consult at any action moment.** The
  same profile, reached through a capability on `Input`.
- **(D) The ledger travels with the cast.** No capability and no callback: the
  gate is two pure functions in `combat` over sheets resolution *already holds*
  — `CanPay(sheet, profile)` and `Pay(sheet, profile)`.

Shape D came out of Kirk's push to keep the seam all-data:

> I would like to find a way to keep it all data… holding out hope there is an
> elegant solution.

**D is not a fourth peer to A, B and C — it is a different axis.** A/B/C answer
*where the debit is called from*; D answers *what the gate is made of*. Once the
gate is pure functions over the cast's sheets, it can be called from a wrapping
machine (A's site), from the door (B's site), or from inside a machine at a
reaction moment (C's reach) — without any of them needing a capability. That is
why it deserves its own section rather than a row in the table.

### Why D is available at all — two verified facts

1. **Resolution already holds every participant's sheet, mutably.** `castFor`
   builds the cast from **every member of the encounter**, and the rule is
   already "anyone whose effects might fire" rather than "the two swinging":

   > EVERY member, not the two swinging: scope is the caller's and applicability
   > is the effect's own predicate (ADR-0038). A bard three cells away whose
   > Bless is running has to be in the room for their subscription to fire

   (`session/attack.go:396-401`, loop at `:411-428`; law R3 at
   `resolution/doc.go:36-41`). So "extend the cast to anyone who might react" is
   **not an extension** — a wizard who is a member of the encounter is already
   in the cast, with a mutable sheet, before any reaction is considered.

2. **A machine already mutates those sheets mid-resolution, bus-free.** The
   strike's damage phase reaches into the cast and writes the target's sheet:
   `combatantFor(m.cast, m.in.TargetID)` (`resolution/strike.go:424`, helper at
   `:624-634`) then `target.ApplyDamage(...)` — commented "**Bus-free, and the
   only phase that is: applying damage is the sheet's own business and takes no
   bus**" (`:443-448`). `character.ApplyDamage` marks the sheet dirty
   (`character/character.go:767`), `Resolve` returns `dirtyCharacters(cast)`
   (`resolve.go:294`, `:413-423`), and `session.saveDirty` writes them
   (`session/attack.go:456-465`).

   **The economy ledger is the same species as hit points**, and `Pay` would be
   the second bus-free phase in exactly the category the first one established.

The structural consequence: **D needs nothing new on `Input`**, so it never
becomes "the first capability resolution consults" and never collides with
`Input.Standing`'s "Carried, never consulted". On the ADR question below, **D
is the only one of B/C/D that stays ADR-free**, and it is ADR-free for the same
reason A is — it adds no vocabulary and no seam concept.

## The cases

### Case (i) — the level-5 fighter's three swings

One action buys two attacks; the third is refused. Two `Attack` verbs in one
turn are two processes, so the bank lives on the sheet either way.

| | |
|---|---|
| **A** | Spend machine reads the sheet, debits the action if the bank is empty, banks 2, spends 1, `Request`s the strike. |
| **B** | Identical, arithmetic at the door; the strike never learns economy exists. |
| **C** | Identical to B. |
| **D** | Identical to whichever call site it is wired at — but the debit is `combat.Pay` on the cast's own sheet, so it rides out through `DirtyCharacters` with no new plumbing. |

**Discriminates: nothing.** Worth stating plainly — the case that motivated the
issue does not choose the shape.

### Case (ii) — Dash: a spend with no machine behind it

Dash spends an action and grants movement. No roll, no target, no chain.

| | |
|---|---|
| **A** | A "machine" whose entire body is a debit and a `Done`. Expressible, but it names a machine where there is no interaction — and the vocabulary's own rule is that a machine's "identity is its **yield-shape**". |
| **B** | Natural. `Cost` set, `Machine` nil or trivial. Kirk's "a machine maybe doesn't have a cost" read the other way. |
| **C** | Natural, same as B. |
| **D** | **Honestly, no better than A or B.** A Dash verb still needs *something* to run; what D changes is that the cost is not pretending to be a machine — it is a function that something calls. The awkwardness is relocated, not dissolved. |

**Discriminates: A (mildly).** Note the case is currently hypothetical at the
seam — there is no Dash verb in `session` — but it is the shape of every
non-attack action in the catalogue (Dodge, Disengage, Help, Hide).

### Case (iii) — the wizard's Shield

A fighter swings at Aldric. Mid-resolution, after the roll and before damage,
Aldric may cast Shield as a **reaction** — and only if he still has one. Already
modelled as a nine-beat test, `play/interrupt/shieldscene_test.go`.

| | |
|---|---|
| **A** | The spend machine above already finished; it debited the *fighter's* action. Reaching the wizard means the strike must `Request` a spend machine for somebody else — economy inside the strike, the thing the census said the machine must never hold. |
| **B** | **Cannot express it.** At the door the actor is the fighter. Whether a window opens depends on a fact discovered several steps into somebody else's resolution. |
| **C** | Works: the spine consults the gate before posing. Costs a consulted capability, and therefore an ADR. |
| **D** | **Works, with nothing added.** At the reaction moment the strike already holds `m.cast`, and the wizard's sheet is in it (fact 1). `combat.CanPay(wizardSheet, shieldProfile)` is a pure read of data the machine has. No reaction, no window. On the answer, `combat.Pay` debits that same sheet and it rides out through `DirtyCharacters`. |

**Discriminates: B fails; A only by breaking its own rule; C works but pays an
ADR; D works for free.** Kirk's eligibility insight is what makes the case
sharp:

> the wizard spends their reaction for that turn only if they have it. if they
> used it on their turn there is nothing to pause for.

That is not a UI nicety — it decides whether the window exists. And the ledger
cannot decide it: `interrupt.Pose` validates the audience and the option tokens
and nothing else, because the module is deliberately "custody, not execution"
and "never interprets an option, a choice, or a payload byte"
(`play/interrupt/doc.go`). **So somebody must consult the economy before `Pose`
is called** — and under D that somebody is the machine, reading its own input.

**A second problem D dissolves that the others do not:** multiattack. If a
fighter's two swings are two strikes inside one resolution, a wizard who spends
Shield on the first must not be offered it on the second. Under D the later
check reads the **same sheet instance** the earlier debit wrote, so the second
window never opens. Under B or C the debit's visibility depends on whether the
gate's writes are readable mid-resolution — which is a question D never has to
ask.

### Case (iv) — suspension: the strike pauses and the process dies

**There is no resume path today, and that has to be said before anything else
about this case.** Nothing outside `play/interrupt` imports it — the only
mention anywhere else in the repo is prose in `session/doc.go`, which records
that the adapter was retired. `Answer` returns an envelope "for the rulebook to
resume" and nothing calls it. So this case cannot be settled by reading code; it
can only be reasoned about.

What *is* verified is the failure case, which is cleanly the same for all four:
**nothing was spent**, because a failed resolution persists nothing — "a refused
resolution leaves nothing on a bus and no half-written sheet"
(`resolution/doc.go:82-85`) — and `session.Attack` returns before
`adopt`/`saveDirty` on any resolution error (`session/attack.go:213-215`).
Failure refunds are free because failure writes nothing.

| | |
|---|---|
| **A / B / C** | The actor's cost was paid before the interaction started. If a suspension persists the frozen resolution and resume continues *from the window* rather than from the top, that debit must have been persisted alongside the frozen payload, or it is lost on resume (free action) or re-applied (double charge). |
| **D** | The actor's door-cost has the identical problem — D does not rescue it. What D does fix is the **reaction** half: the wizard's `Pay` happens at the *Answer* door, after a fresh load, so a window that is never answered spent nothing, and a reloaded sheet cannot be double-charged by a debit that was never written. |

**Discriminates: partially, and the same way for all four.** The honest reading
is that the actor's pre-suspension debit is a **suspension-contract** question,
not an economy question: whatever freezes a resolution has to decide what
travels with the frozen payload, and the debit is one more thing on that list.

### Case (v) — the opportunity attack on a walk

A creature leaves a threatened square; whoever threatens it may spend a reaction
to swing. Two halves, and they are not equally ready.

**The reactor side is door-computable, with one caveat.** Resolution builds an
`interactionRoom` out of `Input.World` — "The positions are already here",
because `EncounterData.Members` carries every member's room and position
(`resolution/world.go:23-46`). So "who is adjacent to this path" is answerable
from data the door already has. **The caveat is real:** that builder returns
`nil` when the participants do not all share one room (`world.go:63-67`), and
`castFor` passes *every* encounter member — so in a multi-room dungeon with the
party spread out, no room is installed at all (`resolve.go:257-263`). Widening
the cast makes this *more* likely, which is a cost D inherits rather than
creates. Filed as a side-finding below.

**The mover side is not ready at all, and D does not change that.** There is no
in-fight movement verb and no walk machine: `encounter.Move` refuses a member in
a bubble, saying in-fight movement "arrives with the resolution work"
(`encounter/encounter.go:1469-1481`). **So D's OA story lands with the in-fight
movement machine, not before it** — the same conclusion the movement addendum
reached from the other direction.

## Where this leaves the comparison

Reading the cases rather than the intuition:

| | (i) swings | (ii) Dash | (iii) Shield | (iv) suspension | ADR? |
|---|---|---|---|---|---|
| **A** machine | fine | awkward | only by breaking its own rule | door-cost open | no |
| **B** door | fine | natural | **cannot express** | door-cost open | probably |
| **C** service | fine | natural | works | door-cost open | probably, and after `Pose` |
| **D** cast | fine | **no better than A/B** | works, nothing added | door-cost open; reaction half fixed | **no** |

**What the evidence supports:** D gives C's reach at B's price, and it is the
only shape besides A that adds no seam concept. It does *not* win case (ii), and
it does *not* solve the suspension door-cost — no shape does. Its advantage is
concentrated in case (iii) and in the multiattack visibility problem, and it
gets there by using two mechanisms that already ship rather than by adding one.

**What that does not settle:** D leaves the A-versus-B question open rather than
answering it — the debit still has to be *called* from somewhere. That may be
D's most useful property: it makes the call-site choice smaller and later,
because moving a pure function call is a refactor, while moving a capability is
a seam change.

### D's obligation, and Kirk's refinement of it — interested-by-declaration

D is only correct while **everyone who might react is already in the cast.**
Today that holds for encounter members and is not a new requirement (fact 1) —
but "pass literally everyone" is a blunt instrument, and Kirk's refinement is to
narrow it by *relevance to the declared action*:

> what cast enters the input? what action is being taken? what can counter that
> action? if it is an attack, a shield would go in because a shield can counter
> that. the key is not shoving every possible thing into the input but only the
> things that can have an effect on it.

**The tension to clear first.** `castFor` passes the whole roster *precisely
because* the session is not allowed to filter: "applicability is the effect's
own predicate (ADR-0038)… deciding they are irrelevant here would be this
package deciding a rule" (`session/attack.go:398-401`). So a narrower cast is
legal **only if the relevance answer comes from the rulebook**, never from the
seam. That is the whole design constraint, and it has a known solution shape.

**The mechanism: a reaction declares its trigger as data.** The house pattern
already exists twice, and both times it was invented to solve this exact class —
"can this be answered before anything runs?":

- **`SaveGate`** declares a contestable consequence's whole contest as data:
  abilities, DC source, success semantics, recurrence. Its own doc says "**It is
  data — content carries it, nothing here executes it**", and names the founding
  complaint: "a stat block could carry a knockdown DC and be lying about whether
  anything read it" (`saves/gate.go:160-173`, ADR-0039).
- **`AttackProfile.Imposes`** declares the consequence itself as data, after
  #1013 found the machine hardcoding prone — with #1014 adding the both-directions
  validation, because "a consequence with no contest is as meaningless as a
  contest with no consequence".

A reaction-shaped feature would declare the same way: Shield as *reacts-to an
attack against me*; an opportunity attack as *reacts-to a creature leaving my
reach*; and the same declaration is what a hook like Bless already expresses
implicitly through its chain subscription. **Cast assembly then becomes
mechanical** — something like `combat.Interested(declaredAction, roster)`,
scanning sheets for declared triggers that match the action about to be taken.
The session hands over an action and a roster and decides nothing; the rulebook
answers who is interested. R3 is not violated, it is *implemented*: applicability
is still the effect's own predicate, only now the predicate is readable before
the bus exists.

**The failure modes are asymmetric, and that asymmetry is the load-bearing
constraint.** Over-inclusion fails safe, by R3's own reasoning — "attaching a
participant who turns out to be irrelevant costs correctness nothing"
(`resolution/doc.go:36-41`): the sheet rides along, its predicates do not match,
nothing fires. Under-inclusion fails **silent**: a wizard left out of the cast
has a Shield that can never fire, no window ever opens, and *no error is ever
produced* — the resolution looks entirely healthy. Nothing distinguishes "had no
reaction" from "was never asked".

So the filter has to be **closed by construction**: a reaction may exist *only*
by declaring its trigger, so that the feature **is** its own index entry and
there is no separate registry anybody can forget to update. That is the rule
ownership already lives under — ADR-0006 keeps "a registry for definitions,
entity-centric storage for ownership. No central who-has-what table", and a
condition exists because it is on the sheet (`character/data.go:73`), not because
a table says so. *(Honest caveat: the precedent transfers on the ownership half,
not the routing half — `conditions.LoadJSON` still routes refs through a switch
(`conditions/loader.go:28+`), which is a build-time registry of a different kind.
#971 is in that neighbourhood.)*

**What composes out of this.** The window's audience becomes the intersection of
two data reads, which is Kirk's two insights joined:

> audience = **interested** (declared trigger matches the action) ∩ **affordable**
> (`CanPay` on the sheet the cast already holds)

Both halves are door-readable, both are data, and D is intact — no capability,
no callback. The same intersection is also exactly the list a client needs to
render "you may react" prompts, which is open question 4 answered by
construction rather than by a new read.

**What it relieves, and what it does not.** Narrowing the cast reduces a cost
that is real and already paid: `castFor` performs a repository read per
character on every verb (`session/attack.go:423-427`), and `standingSeam` does
its own per consult — a cost that scales with **roster size** rather than with
the interaction, which is the wrong axis. It also reduces pressure on
[#1090](https://github.com/KirkDiggler/rpg-toolkit/issues/1090), since a smaller
cast spans rooms less often — but **it does not fix #1090**, whose real answer is
the one-map position question rather than cast width. Do not let this section be
read as closing that issue.

**The honest cost: none of this exists yet.** No feature carries trigger data
today, so `Interested` has nothing to scan and the filter cannot be built. **The
whole-roster rule remains v1's honest behaviour** — over-inclusive, safe, and
paid for — and this section is its designed successor, arriving with the
reaction wave that gives it both its first declarations and its first caller.

## The cross-cutting facts any shape must respect

Established in the census and its addenda; cited, not re-argued.

1. **The profile is compiled per actor, in the rulebook.** Class-dynamic cost is
   the compiler's business, following `AttackFromCharacter`. The machine holds no
   class table. The repo has the anti-pattern three times: the monk hardcode
   (`character/action_economy.go:592-607`), the ref→resource switch (`:742-763`),
   and Dash's two constructors baking cost into object identity
   (`combatabilities/dash.go:29-41` vs `:43-55`).
2. **The ledger is on the sheet.** Three verbs in one turn are three processes.
3. **Keyed, never fielded.** `combat.ActionEconomy` grew a named field per
   feature; its persisted twin replaced them with a keyed map, and the bridge
   between the two maps only three of five keys.
4. **Per-unit movement cost stays out of the profile.**
5. **The sealed vocabulary is `Gather | Pose | Request | Done`**, and extending
   it costs an ADR (ADR-0038).

### Does any shape need an ADR?

The census said "no ADR" on the strength of the socket table pricing a new
machine as cheap. **That reasoning covers shape A only.**

B and C do not add a step kind, so ADR-0038's stated trigger is not pulled. But
they change what `resolution.Input` *means*, and there is a law in the way. R2
reads "**Everything at the seam is data.** No runtime object crosses into or out
of [Resolve]" (`resolution/doc.go:34-36`). Four capabilities already ride
`Input` — `Deciders`, `Initiative`, `Standing`, `Roller` — and the package
rationalises them as pass-through rather than exceptions: `Standing`'s own field
doc says "**Carried, never consulted.**" A cost gate under B or C would be the
first capability resolution itself *consults*, which is a real change to the
package's self-description.

**D pulls neither trigger.** Pure functions in `combat` over sheets already in
`Participants` add no step kind and no capability; the sheets are already data
and already mutated in place. So: **A and D are ADR-free; B and C want one**, and
C's would have to be written with `Pose` rather than before it.

## Open questions

1. **Where is `Pay` called from — a wrapping machine (A's site) or the door
   (B's site)?** Under D this is a smaller question than it looks, and it can be
   answered later.
2. **One ledger behind the gate, or several?** Per-turn slots, granted capacity,
   pools and slots have different cadences and, today, different homes. One debit
   surface, or several verbs to the caller?
3. **Pools and spell slots through the same system — genuinely unresolved.** Ki
   is a keyed pool with a `ResetType`; a spell slot is keyed by *level* with no
   reset field, and upcasting means a 3rd-level spell may eat a 5th-level slot.
4. **The eligibility read.** "Could Shield fire?" is the may-I-act question in
   reaction clothes, and it is two questions: the server needs it to decide
   whether to open a window, and a client may want it to grey a button.
   Interested-by-declaration answers both with one intersection — is the read
   the *same* one, or does the client's version want more (why not, not just
   whether)?
5. **What travels with a frozen resolution?** Case (iv) suggests the actor's
   debit is part of the *suspension* contract rather than the economy's. Worth
   settling when `Pose` is designed, not before.
6. **Is "a reaction exists only by declaring its trigger" enforceable?** It is
   what makes a narrowed cast safe, because under D a missing participant stops
   being a missed buff and becomes a missed refusal — and an unfireable Shield
   produces no error at all. Can the compiler or a test make an undeclared
   reaction impossible, or is it a convention somebody eventually breaks?
7. **Does the declared trigger vocabulary want to be sealed?** `DCSource` was
   closed because 5e closed it (ADR-0039). Trigger shapes — attacked-by,
   left-my-reach, ally-damaged, spell-cast-nearby — may or may not be a closed
   set, and getting that wrong in either direction is expensive.

## What each shape does to E1–E3

E0 (dirty-marking) is ruling-independent and already dispatched — and note it
becomes **more** load-bearing under D, since every debit reaches storage through
exactly that path.

| | A | B | C | D |
|---|---|---|---|---|
| **E1** cost compiler | unchanged | unchanged | unchanged | unchanged |
| **E2** the spend | a `Spend` machine | a field on `Input` + door debit | a capability on `Input` + two consult points | `combat.CanPay`/`Pay` + one call site |
| **E3** session one-liner | hands `Resolve` the machine | hands `Resolve` a cost | hands `Resolve` a gate | **unchanged if the call site is a machine** |

**E1 survives all four shapes unchanged**, which is the useful result: the
compiler and the profile are common ground, so E1 can proceed on the census's
ruling while this conversation continues.

## What would make this doc wrong

- If reactions turn out not to need an eligibility check before posing, case
  (iii) stops discriminating and B becomes as good as C or D.
- If a sheet in the cast cannot safely carry a debit across a `Request`
  boundary — if a sub-machine gets a different instance — D's central claim
  fails and it collapses into A.
- If the cast cannot be guaranteed complete for reaction purposes, D is unsafe
  in exactly the cases it was chosen for, and C's consult (which can go and ask)
  becomes the honest answer. This is the one that would hurt most, because
  under-inclusion is silent: there is no failing test to write for a window that
  never opened.
- If a reaction's relevance turns out not to be declarable ahead of the
  interaction — if "can this counter that?" genuinely needs the interaction's
  intermediate state — then interested-by-declaration cannot be computed at the
  door, and the whole-roster cast is not a v1 compromise but the permanent
  answer.
- If `Pose` arrives shaped so a machine can consult supplied capabilities
  directly, C stops being distinct and becomes A with a longer reach.

## Side-finding, filed separately

**A multi-room cast silently disables prone's positional split.** `castFor`
passes every encounter member (`session/attack.go:411-428`), and
`interactionRoom` installs no room when participants span rooms
(`resolution/world.go:63-67`, `resolve.go:257-263`). In a multi-room dungeon
with the party spread out — the reference tomb's normal state — no room is
installed for any strike, so prone's advantage-within-five-feet predicate has no
positions to read. The behaviour is documented as intentional ("a machine that
needs positions then refuses"), but the *combination* with pass-everyone-in
means the refusal is the common case rather than the rare one. Not fixed here.

— census-1035 agent, on behalf of KirkDiggler
