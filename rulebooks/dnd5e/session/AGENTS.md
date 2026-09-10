# session — the host seam: the one interface a game server implements, holding no rules

> Contract, charter and guarantees: [`doc.go`](./doc.go). **This file is not a summary of that one.**
> It answers a different question: I am building something — does it belong here, and what may I ask?

Three seams sit beside each other. [`encounter`](../encounter/doc.go) is the
composition — the world. [`resolution`](../resolution/doc.go) is the only place a
bus exists. This is the table. Read [`../CLAUDE.md`](../CLAUDE.md) for how the
three are cast, and [ADR-0034](../../../docs/adr/0034-where-encounter-logic-lives.md)
for why the split exists at all.

## What this package owns

**The verb surface.** Thirty-two exported `Manager` methods, each one load, act,
save, return — no setup call, no teardown, no ordering for the caller to get
wrong. `Move`, `Attack`, `Cast`, `Activate`, `DeathSave`, `EndTurn`, `React`,
`Search`, `Loot`, `Hold`, `Trade`, `Interact`, `OpenDoor`, `Unlock`, `Join`,
`Exit`, `End`, `Spawn`, `PlaceNPC`, `Dissolve`, `Unpack`, `StartSession`, and the
reads `Afford`, `Roster`, `Atlas`, `AtlasOf`, `Status`, `View`, `Story`, `Where`,
`Turn`, `Doors`. A verb is a file; the file is the unit of ownership.

**The three repositories and the capabilities beside them.**
[`repositories.go`](./repositories.go) declares `SessionRepository`,
`EncounterRepository`, `CharacterRepository` — key-value, get-by-id and put-by-id
only (S12), trading in data and never in domain objects (S3). They point OUTWARD:
this package calls, the host implements. [`session.go`](./session.go) adds the
four supplied capabilities the host wires once — `Events`, `Dice`,
`PresentationIDs`, `TurnDriver` — and refuses to construct without any of them
(S8, `NewManager`).

**The compiled declaration and its selector.** [`afford.go`](./afford.go) prices
the turn; [`offers.go`](./offers.go), [`casts.go`](./casts.go) and
[`activations.go`](./activations.go) compile the rows;
[`declaration_id.go`](./declaration_id.go) mints the opaque selector each
mutating verb demands back. See [ADR-0042](../../../docs/adr/0042-afford-answers-in-declarations-not-currencies.md).

**The executors.** A declared action becomes a resolution machine here and
nowhere else: `resolution.NewActivation` in [`activate.go`](./activate.go),
the cast machine in [`cast.go`](./cast.go), the swing in
[`attack.go`](./attack.go), the walk in [`move.go`](./move.go), the save in
[`death_save.go`](./death_save.go). Each one hands `resolution.Resolve` a world
view, adopts the world back, saves dirty sheets, then records.

**The lookups.** Ten capabilities the composition cannot implement for itself,
each proved at compile time by a `var _` line: `standingSeam`
([`standing.go`](./standing.go):47), `checkSeam` and `witnessSeam`
([`conceal.go`](./conceal.go):96, :205), `strikerSeam`
([`striker.go`](./striker.go):38), `moverSeam` ([`mover.go`](./mover.go):37),
`announcerSeam` ([`announcer.go`](./announcer.go):31), `turnDriverSeam`
([`turndriver.go`](./turndriver.go):246), `initiativeSeam`
([`initiative.go`](./initiative.go)), `sightSeam` ([`sight.go`](./sight.go)),
and `reactionAttacks` ([`mover.go`](./mover.go):486, the one `resolution`
capability).

**The projection and the persistence shape.** [`convert.go`](./convert.go) and
[`types.go`](./types.go) turn inner shapes into ours; [`stream.go`](./stream.go)
numbers each member's delivered stream densely so a beat sent to somebody else
leaves no observable hole; [`data.go`](./data.go) is what the host stores.

## What it must never learn

Each of these is a *test you can run by reading* — and three of them are also
tests you can run.

**No bus.** A fold needs a bus; every route to one begins with importing
`rpg-toolkit/events`. Nothing here imports it, production or test, and
`TestNoBusLivesInThisModule` ([`no_bus_lives_here_test.go`](./no_bus_lives_here_test.go):66)
matches by import path so an alias cannot walk one in. [ADR-0038](../../../docs/adr/0038-resolution-owns-the-bus.md)
is why: resolution owns the bus, rules packages trade in values.

**No check.** No DC, no saving throw, no damage fact.
`TestSessionConstructsNoCheck` ([`no_check_lives_here_test.go`](./no_check_lives_here_test.go):85)
pins the three packages that would have to be named for the concentration rule
to leak up here.

**No inner type on the exported surface.** S2, enforced by
`TestNoInnerTypeCrossesTheBoundary` ([`boundary_test.go`](./boundary_test.go):217)
against an allow-list split into persistence shapes and contract types.

**No hit-point threshold.** `loadActorSheet` calls `combat.IsDown`;
[`standing.go`](./standing.go) finds records and
[`participation.go`](./participation.go) maps typed provider answers. Neither
computes a threshold, and `LifeState`/`DeathSaveProgress` are projections of
facts, not arithmetic.

**No decision about when a fight begins.** This is the canonical example,
because this package used to hold it and gave it away. A walk read each step's
perception delta and a first sighting stopped the walk. That is a rule, and a
wrong one twice over — sight is not the only way a fight starts, and a walker is
not the only one who can see. The composition owns it now (rpg-toolkit#964).
`doc.go` tells the story; the point for you is the shape of the mistake: it did
not look like a rule, it looked like the walk noticing something.

**The honest exception, so you can weigh it rather than discover it.** Three
range constants do live here: `helpReachFeet = 5`
([`activations.go`](./activations.go):269), `inspirationReachFeet = 60`
([`inspiration.go`](./inspiration.go):29), `defaultSightFeet = 120`
([`sight.go`](./sight.go):25). Each is documented as a ruling with its RAW
divergence named.

**Do not read them as precedent, because they are not one exception but two
different problems** (rpg-toolkit#1630):

- `helpReachFeet` and `inspirationReachFeet` are **content facts with nowhere to
  live.** An attack declares `AttackDelivery.ReachFeet` and a cast declares
  `CastProfile.RangeFeet` ([`../combat/actions/cast.go`](../combat/actions/cast.go):66);
  an activation has no such field. These two numbers are here because the data
  model has a hole exactly where they belong, not because this package decided to
  hold game numbers. When an activation can declare its range, they leave.
- `defaultSightFeet` is not a range declaration at all. It is a **fallback for an
  absent fact** — its own comment says *"we don't have this stat block's Senses
  yet"* — which is the fail-closed question, not this one.

So a fourth constant arriving is the charter slipping, and the right response is
to ask which of those two shapes it is rather than to weigh it against these.

## Questions it answers, questions it asks

Direction matters more than the noun, and here it is settled by `go.mod`.

- [`encounter/go.mod`](../encounter/go.mod) names **neither** the rulebook root
  nor `resolution`. It cannot know what a sheet is. Law C1.
- [`resolution/go.mod`](../resolution/go.mod) names the rulebook root and
  `encounter`.
- [`go.mod`](./go.mod) names **all three**.

That asymmetry is the structural reason this section exists. A fact the
composition needs about a sheet has exactly one place it can come from, and this
is it. So the composition takes a **capability** — member IDs in, member IDs out
— and this package implements it. `doc.go`'s "one question this package answers
for the world" argues that case for `Standing` (rpg-toolkit#1079); it is worth
reading before you add the eleventh capability, because it is the template.

**WHAT THIS PACKAGE CONTRIBUTES IS THE LOOKUP, NOT THE RULE.**

The test is mechanical: **if your new code here contains a threshold or a die,
you have put a rule in the seam.** `standingSeam` finds records and hands them
to `resolution.Participation`. `checkSeam` stages a stored record and hands it to
`resolution.MakeCheck`, which loads the character, attaches conditions, picks the
approach and rolls on its own lawful bus. `initiativeSeam` hands the host's
entropy to the rulebook's `RollForOrder`. In every case: records in, answers out.

What it ASKS for, and never computes:

| Question | Who answers |
|---|---|
| Where is anything, who sees whom, what does the map look like | `encounter` ([ADR-0047](../../../docs/adr/0047-encounter-owns-location-knowledge.md)) |
| Does this land, for how much, what condition does it leave | `resolution` |
| Is this member down / dead / dying | the rulebook (`combat.IsDown`, `combat.ParticipationFor`) |
| What does this action cost, what does this spell do | content, as data ([ADR-0045](../../../docs/adr/0045-actions-are-data.md)) |
| Where are the bytes | the host, through the repositories |

## Whose question is it?

**Ask this before asking where the code could go.** It is the test the rest of
this file assumes you have already applied, and the one that is easy to skip,
because the answer to *"where could this live?"* is so often *"here, easily."*

| The question | Owner |
|---|---|
| **Where is anything** — who stands where, who can see what, what shape the floor has, who is inside a shape | [`encounter`](../encounter) |
| **What happens when things interact** — does it land, for how much, what does it leave behind, what does a fact mean | [`resolution`](../resolution) |
| **What was asked for, what is loaded, what is saved** — the host's verbs, the repositories, the integrity of what enters and leaves | **`session`** (this module) |

All three seam docs carry this same table. If they ever disagree, that is the
bug — not a nuance.

**Computability is not ownership.** That a module *can* produce an answer —
because it happens to hold the two facts the answer is made from — is not
evidence the answer is its to give. Every seam here can reach far enough to
answer a neighbour's question. That is what makes them useful to each other, and
it is exactly what makes this mistake easy and quiet.

The counter-question that works: **if a second caller needed this same answer
later, from somewhere else, where would they have to go and get it?** If the
honest answer is "somewhere other than where I am about to put it", it belongs
there instead.

### The worked example, because it nearly shipped

A spell that names a shape in space — a five-foot radius around the caster — and
the engine works out who is caught by it.

`session` holds the roster and can call `Distance`, so it *can* fold the two into
a target list in about fifteen lines. It was designed that way for two drafts,
and the argument each time was economy: fifteen lines here versus a new method
there.

That is a mechanism argument, and it survived the predicate/producer test below
— the fold names its universe perfectly well at the call site. What it never
faced was the ownership question. *"Who is standing in this shape"* is a
**placement** question. `encounter` owns placement, and already answers this
exact question for an authored footprint (`MembersIn` — a roster read, filtered
through the same `placementOf` projection every other member read uses).

The cost of getting it wrong was not fifteen lines. Fireball's lingering floor is
the same question asked *continuously*, and its home is a runtime-minted region
answered by `MembersIn`. Answer the transient case in `session` and one question
has two mechanisms in two modules forever; answer both here and the persistent
case becomes *"mint the region, then ask the question we already ask."*

**One question, one owner** — decided before, and independently of, where the
answer is convenient to compute.

### What that means for this module in particular

This is the seam where the mistake above is easiest to make, and the reason is
structural: **`go.mod` lets this module import all three.** It can reach the
roster, the rulebook and the machines, so almost any question *can* be answered
here. That reach exists so this package can **carry** answers between layers,
never so it can produce them.

Things that look like somebody else's and are yours: **what enters and leaves.**
Which sheets are loaded, which world view is handed over, what is written back,
what the host is told. Integrity at the boundary is the whole product.

Things that look like yours and are not: **anything with a shape or a rule in
it.** If you are about to compute a distance, compare against a threshold, or
decide what a fact implies, stop — ask the owner and inject the answer. The
existing lookups are the model, and they are lookups precisely because they find
records and hand them on: `standingSeam` finds sheets, `resolution.Participation`
decides what being down means.

## Predicates and producers

A **pointed question** — "is `to` within `rangeFeet` of `from`", "is this sheet
down", "what does the host's repository hold for this ID" — is cheap and safe.
It takes its subject as an argument and returns a verdict. `inRange`
([`reach.go`](./reach.go):29) is the shape.

A question that **produces a set** is a different animal. "Who does this action
hit" has an answer that silently depends on what happened to be loaded, unless
the universe it ranges over is *declared*. Two compilations of the same offer
against two different loads give two different answers, both look correct, and
neither names the difference.

**Every collection-returning API here names the universe it ranges over.** That
is an observation about the surface as it stands rather than a gate on the next
one. It is worth keeping because of how it fails when it is missing — invisibly.
A producer with an unstated universe returns a plausible list, and whatever it
left out looks exactly like something that was never there.

The existing producers already obey it, which is why it is writable as a rule
rather than a wish. `buildTargetPreflight` ([`offers.go`](./offers.go):701) takes
`enc`, `positions`, `holdings`, `member`, `maxRangeFeet` — the universe is its
parameter list, not something it fetches. `casts.go` then narrows that universe
in the open: live sightings except the caster, within `RangeFeet`, world NPCs
excluded (`excludeWorldNPCs`), then `filterAttackTargets`. Four named steps, each
one visible at the call site.

For a verb, **the loading IS the declaration**, and it must be visible in the
verb's own input/output types rather than buried in a helper. `Atlas` takes a
`Member` and answers what that member knows; `AtlasOf` takes a `World` and
answers whole, because an author is asking. `Doors` answers from `DoorsFor`
exclusively — the unscoped read is the host's internal whole truth and does not
cross this seam for a member-shaped question. `View` takes a `Member`. `Story`
takes a `Member` and a `FromSeq` in that member's own dense numbering. If your
new collection cannot name its universe in its `Input` type, the API is wrong
before the implementation is.

## Where does this go?

| You are building | Owner | Why |
|---|---|---|
| A new host verb | **here** | Verbs are this package's whole product; add a file, load-act-save-return |
| A new rule the verb needs | **`resolution`** | It is the only place a bus exists and a machine never sees one |
| Geometry, placement, perception, an ending | **`encounter`** | The composition is the world; the verb reports what it says happened |
| A new action or cast kind | **content + `resolution`** | Actions are data (ADR-0045); the profile IS the executor selector |
| The offer row for that kind | **here** | Compilation, selector and refusal vocabulary are the seam's |
| A fact the composition needs but cannot compute | **here, as a capability** | Only this module imports the rulebook root; supply the lookup, never the rule |
| A new persistence shape | **here**, in `data.go` | S3/S13: one repository per data type, hydration inside the laws |
| Something the host wants to display | **here**, in `types.go` | S2 forbids the inner type; project it, and put nothing in the projection the composition did not say |
| A number about the game | **the rulebook** | See the honest exception above. If it is an action's own reach it belongs in `combat/actions` beside `ReachFeet`/`RangeFeet` (rpg-toolkit#1630); if it is a fallback for a fact content did not state, it is a fail-closed question, not a range |

## Traps

**There are two down-gates, with different sources.** On the turn clock every
verb uses `actor.downed` from one strictly-single `loadActorSheet` read
(`attack.go`:268, `cast.go`:223, `activate.go`:191, `move.go`:219). Only the
*world* clock reaches `refuseIfDown` ([`standing.go`](./standing.go):263), which
goes through the `Standing` capability instead (`move.go`:247,
`attack.go`:254). They agree today. They are not the same code path, and a
change to one is not a change to the other.

**`TargetNone` is four rulebook values wearing one seam name.** For a cast it is
keyed off `combatActions.CastTargetSelf` ([`casts.go`](./casts.go):179); for an
ability `targetKindOfAbility` ([`activations.go`](./activations.go):251)
collapses Self, None, Position and Area into it. That is deliberate — a client is
being told "do not prompt" — but the day a cast is centred on the caster and
sprays everyone else, the same value would have to mean both "lands on you" and
"lands on everyone but you". The fix at that point is a new `TargetKind`, not a
new reading of this one.

**An unrecognised ability ref gets Help's five feet, silently.**
`allyCandidatesFor` ([`inspiration.go`](./inspiration.go):41) switches on the ref
and falls back to `helpCandidates`. The comment argues it (the narrowest rule is
the safest default) and it is the one place in this package where an unknown kind
does not fail loudly. A new ally-targeted ability that forgets its `case` is
offered adjacent allies and no error.

**A new verb string cannot mint a selector, and that is where the enum is
sealed.** `validateDeclarationVerbSlot`
([`declaration_id.go`](./declaration_id.go):196) rejects any verb or slot outside
the closed list, and `selectorVariant` refuses cross-verb material rather than
dropping it. But `verbRank` ([`offers.go`](./offers.go):909) has a silent
`default: return 7`. The selector check is what actually catches you; the sort
will not.

**Initiative is flat.** `RollInitiative` ([`initiative.go`](./initiative.go):82)
passes every member a Dexterity modifier of zero, because a member ID is all the
composition hands over at formation. Ties are broken by ID for reproducibility
(C8). Both are documented decisions, not bugs — but a test that expects a
high-Dex character to act first will fail, and it will look like the dice.

**`../CLAUDE.md` says `initiative/` is dead. It is not.**
[`initiative.go`](./initiative.go):14 imports it, and is its only non-test
importer in the repository.

**`View` and `Story` return bare slices.** Every other verb returns a
`*XOutput`. Adding a field to those two is a breaking change; adding one to the
others is not.
