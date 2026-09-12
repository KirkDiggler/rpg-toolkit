# examples/perception

A spike. It owes nothing to the shipped perception path and is not a migration
of it — it exists to find out whether a four-layer shape survives contact with
the scenarios that matter, before any of it is proposed as a module.

Depends on testify and `world/journal` — which itself imports `errors` and
`slices` and nothing else, so taking it costs almost nothing. The full `world`
composer is deliberately still out: it needs a Scenario, verbs and a Resolver
standing up before a line of perception runs, and nothing here has needed an act
loop yet.

## The shape

```
truth surface        world (journal + graph)  |  encounter (canvas + roster)
    |                       percept inputs, passed as values
    v
projection           pure function. physics + deception. per observer,
    |                per channel. no world import, no state, no clock.
    v                WRITE-ONLY toward testimony. has no query surface.
testimony            immutable append-only tracks. sees neither the world
    ^                nor the meaning of what it holds.
    |
reconcile            merge / rule-out. sees ONLY testimony, which is the
    |                only reason it can be wrong.
    v
belief               the observer's own claims. contacts are folded from
    |                claims, never stored.
    v
carry                what crosses a run's edge, in either direction: re-keyed
    |                onto a name, arriving as a ghost.
    v
act / stage          a belief becomes a deed: act sees only beliefs, stage
                     resolves them against the world.
```

The arrows are the design. Nothing reads truth to answer a question about an
observer.

## The three nouns

**Track** — one channel's opaque continuity handle. *The noise I have been
following behind that door.* It carries no identity, so "something is through
there" is the ordinary case rather than a special one. Every entry is immutable
and carries two stamps: **Observed** (first seen) and **Confirmed** (last seen to
still say the same thing). Staleness is the caller's arithmetic over Confirmed;
nothing here has an opinion about whether a memory still holds.

**Contact** — the observer's claim that two tracks are one thing. The resting
state is *unmerged*, so "never merge two things the person hasn't merged" is not
a rule anybody enforces — it is what happens when nobody acts.

**Identification** — *that contact is the goblin chief*. It attaches to a track,
not a contact, because tracks are the only stable handles; a contact is folded
fresh every time it is asked for. **Carry-out is what pays for it** — see below.

## What the tests prove

`scenarios_test.go` — the stories:

| test | proves |
|---|---|
| `TestTheDoor` | two observers, byte-identical testimony, different conclusions — the difference is which mind they have |
| `TestSilentImage` | the truth surface never contains the dragon; a cross-channel disagreement is held and unresolvable |
| `TestTheWrongMerge` | a bad inference costs only the claim; splitting restores nothing because nothing was lost |
| `TestTwoObserversContradict` | one is charmed, both are right about what they perceived, nothing reconciles them |
| `TestStaleness` | identical content extends a watermark; changed content appends; the old entry stands |
| `TestFootprints` | three minds: no claim, confidently wrong, and *ruled out* |
| `TestTheRangerCannotRuleIn` | matching sign is consistent, and consistent is not identical |

`refusals_test.go` — what the design must be **incapable** of. A demo can fake
every happy path; only these prove the boundaries. `TestTheStoreCannotReadAPayload`
is the load-bearing one: different bytes under the same declared change key are
treated as unchanged, because the store cannot look.

Three mutants were run against the invariants and each was killed by exactly the
test that claims it: comparing payload bytes instead of the declared key, having
the ranger rule *in* on a mismatched sign, and letting a mind overrule its owner.

## What writing it changed

Six things the design did not know before there was code:

1. **A presence says different things to different channels.** One payload per
   presence gave hearing sight's fidelity. `Says` is keyed by channel — and a
   channel a presence says nothing to simply cannot perceive it, so a silent
   creature costs no machinery.
2. **Reach and reported locus are different capabilities.** You can perceive
   something without being able to place it. "Known to exist, not located"
   arrives as the ordinary case rather than a state anybody declares.
3. **The store owns change detection for the fields it defines, and delegates
   only the payload.** Folding locus into the emitter's key looks tidier and is
   fail-silent: a creature that moves while looking identical reports no change,
   and every observer holds a stale position nothing flagged.
4. **A mind proposes only where its owner has made no claim.** Otherwise a
   reconciler re-judges ghosts forever and undoes a player's manual split. This
   is also what gives retracting ("I no longer know") and ruling out ("I have
   decided they differ") different meanings.
5. **Track handles must be opaque and not derived from identity.** A handle that
   can be inverted lets an observer merge two channels by string comparison, and
   the merge happens with nobody deciding.
6. **Ruling out and ruling in are not symmetric.** Woodwise can say "those are
   not goblin tracks" and cannot say "those are these goblins."
7. **Carry-out is what pays for identification**, and an unnameable contact
   simply cannot leave a run.
8. **The distillation must fail closed on a tie.** Two tracks under one name,
   equally fresh, have no answer, and the arbitrary one is worse than none.
9. **Carrying in and carrying out are one pair of verbs**, and only because
   landing a belief records its name — that is what makes a player's store the
   same shape as a run's.
10. **A reconciler has to know memory from live.** Two things you can perceive at
    once are two things; a memory and a percept on one channel may well be one.
    That difference is the whole of recognising a place you have been before.
11. **A stamp needs two coordinates**, and carry-in is what forces it: a
    world-stamped memory and a run-stamped percept land in one store and have to
    be comparable. Ticks alone get it backwards.
12. **The journal already refuses a belief**, and for its own reasons — but it is
    incurious about truth, so the discipline lives in the projection having no
    way to write.
13. **A store's load checks are a list of what it cannot do.** Writing them is
    the fastest way to find out whether the invariants were ever real, because
    each one has to name a state the writer could not have produced.
14. **Aiming at a name is the whole of "act on what you hold".** It is one field
    on one struct, and it makes targeting something you never perceived
    unexpressible rather than forbidden.
15. **Binding is a pure function of the truth surface**, so forgeries are absent
    from it by construction. Nobody had to write a rule saying that attacking an
    illusion hits nothing.
16. **Witnessing and perceiving are one decision**, so they are made once. Two
    answers to "who was there" is two chances to disagree.

## Leaving a run

A dungeon run is ephemeral and so is the party, so raw testimony belongs to the
run — an append-only log keyed by handles the projection minted inside it. What
leaves is a conclusion, not the evidence, and it goes to a **player**.

**You can only carry out what you can name.** A track handle means nothing
outside the run that minted it — that opacity is what stops observers merging
channels by string comparison — so a belief travels only once it has been
re-keyed onto a name somebody could speak about later. *"There are goblins in
the eastern tunnels"* travels. *"Something is through that door"* does not, and
`Result.Unnamed` says so out loud. This is the use case that pays for
identification; before it, naming had none.

**It arrives as a ghost**, through the discrete `Report` verb: held, never
current — something you know and are not currently perceiving. The store already
had that state, so a carried memory needs no new one, and it stays exactly as
falsifiable as it was inside. Carrying a lie out carries the lie.

**A run has two exports, and they are not the same mechanism.** Facts reach the
world journal through a verb, with an actor and an audience, because you cannot
be wrong about what happened. Beliefs reach a player through a naming and a
distillation, because you can absolutely be wrong about what you learned. Same
falsifiability line, doing real work.

| test | proves |
|---|---|
| `TestTheDistillation` | a named conclusion travels on both channels; the run-local handles do not |
| `TestTheUnnamedCannotTravel` | naming is the gate, and the refusal is reported |
| `TestCarryingDoesNotLaunderALie` | the charmed belief crosses intact while truth still holds a sword |
| `TestTheCollapseNamesItsLoser` | the freshest wins and the loser is named |
| `TestAnUnresolvableCollapseCarriesNothing` | a tie carries **nothing** rather than guessing |
| `TestACarriedBeliefOnlyRefreshesByCarryingAgain` | perceiving again does not touch what a player holds |

## Walking back in

Carrying **in** turned out to be `Out` and `Land` again with the two stores
swapped — a party entering the tunnels already believing something is the same
act as a player receiving what the party brought home. That only works because
`Land` records what the receiver *calls* each belief, which makes a player's
store shaped exactly like a run's: tracks plus names. Without it a carried
belief arrives with its name sealed inside a handle nobody can read back, and it
could never travel anywhere again.

A carried belief arrives the way every memory does — held, never current, as old
as it really is, and as wrong as it ever was.

| test | proves |
|---|---|
| `TestCarryingIn` | a belief lands in a fresh run stale and ghostly; seeing the thing mints a *separate* track and does not refresh it |
| `TestRecognisingWhatYouRemember` | Woodwise walks in and knows the place; Oblivious has the same evidence and no idea |
| `TestTwoCurrentSightingsStillDoNotMerge` | the loosening did not loosen the live rule |
| `TestAnEmptyRoomDoesNotDeleteYourMemory` | he still believes, and the only thing that changed is how old the belief is |
| `TestAFreshPerceptDoesNotCorrectAMemory` | a contradiction is held side by side, and nothing chooses |

**The empty room is the case the whole design is for.** Bram walks in
remembering a dozen goblins and the tunnels are empty. Nothing corrects him.
Absence of evidence is not a negative claim, so looking at an empty room writes
nothing about what is not in it — and a memory was never current, so the empty
percept does not even fade it. He still believes it, he can still act on it, and
whether forty stamps of silence should shake him is a judgment for somebody
else to make.

`carry` is the one place folding is legitimate — deciding what you took away is
an authored act, not a read. It still refuses a tie: two tracks under one name,
equally fresh, would have to be separated by handle order, and that would be the
package inventing a conclusion on the observer's behalf.

## What leaving a run also surfaced

**It needed a coordinate that outlives the run — and that is order, not a clock
that ticks.** A carried belief keeps its `Confirmed` stamp, so it is only honest
if the number still means something afterwards.

`play/clock` cannot supply it: its `HighWater` lives inside a clock instance the
encounter creates and stores in its own `Data`, and it marks turn order, not a
calendar. `journal.Fact.Seq` is documented as *the world's only clock* and is
monotonic across the whole journal, which outlives every run.

So a `Stamp` carries **both**: `Seq`, the world's order, which advances when
something *happens* — a whole dungeon run sits at one Seq while a great deal goes
on inside it — and `Tick`, which orders testimony within a run and means nothing
outside it. `TestTheWorldsOrderOutlivesTheRun` is the proof, and it is sharper
than it looks: the memory's tick is 2 and the new run's first tick is 1, so **on
ticks alone a season-old memory reads as the fresher of the two.** Seq is what
gets it right.

## Fitting against the real journal

| test | proves |
|---|---|
| `TestTheWorldsOrderOutlivesTheRun` | both halves of a stamp are load-bearing |
| `TestABeliefCannotBecomeAFact` | `journal.Append` refuses a fact with no actor — and noticing has no actor |
| `TestTheLedgerHoldsTheActNotTheBelief` | pip acts on a lie; the ledger records what he did and who saw it, never what he thought |

**The journal enforces the crossing rule without knowing perception exists.**
Every fact names an actor, because a fact is something somebody *did*; a track's
latest entry is something somebody *noticed*, and noticing has nobody to name. So
no shape of belief is acceptable to it.

Be exact about what that does not buy, though. `Append`'s own doc says the
journal "does not check that the entities exist, that the audience is plausible,
**or that the fact is true**." It will record a lie. The rule that a lie never
reaches the ledger therefore holds for a different reason: the projection is the
only thing that authors one, and it has no path to `Append` at all.

**One piece of friction worth naming:** `journal` speaks `EntityID` and
perception speaks `Observer`. Both strings, different types, so these tests
convert at the seam.

That is not necessarily where it stays. `journal` was written when its own
entities were all anybody had, and perception is its first outside user — so the
right reading is that a shape is showing itself, not that a conversion has found
its permanent home. Packages evolve as shapes appear.

**A carried belief is only ever refreshed by carrying again.** Perceiving the
same goblins in a later run mints new run-local handles and touches nothing a
player holds. That is correct — nobody inside a run is reading the player's
store — but it means the two stores only ever meet at a distillation.

## Surviving the process

`ToData` / `Load` on both stores, per play's leaf contract — round-trip the
state, and **reject invalid state on load**.

`TestKnowledgeSurvivesTheProcess` is the claim the whole carry story rests on,
finally made real: a player's knowledge goes out through `encoding/json`,
everything in memory is dropped, and a run months later is built from nothing but
those bytes. The memory comes back a ghost, as old as it really is, still named —
and the ranger still recognises the tunnels when he walks back in.

**The refusals are the interesting half.** Every check names a state the store
*cannot reach on its own*, so meeting one means the data came from somewhere else
— a hand edit, a partial write, a version that disagreed — and continuing would
mean trusting testimony nobody gave.

| refused | because |
|---|---|
| an observer holding nothing | never-perceived and perceived-nothing would become indistinguishable in storage |
| a track with no testimony | a track *is* its testimony; every reader of `Latest` assumes this cannot happen |
| an entry repeating the one before it | the one shape neither verb can produce — see below |
| a log that runs backwards | append-only, or the two stamps mean nothing |
| confirmed before observed | a memory cannot be re-confirmed before it was made |
| `unrelated` stored as a claim | no claim is not a claim; a stored one is a judgment nobody made |
| a pair stored out of order | keys are normalised, so a second spelling would hide a duplicate |
| a track named twice, or named nothing | an empty name is how naming is *retracted* |

The sharpest one is the repeat. Two consecutive entries saying the same thing in
the same place is exactly what `Surveil` and `Report` never write — identical
content extends a watermark rather than appending. Loading one would turn a
single belief that held for a while into two separate ones, and the difference
between *still true* and *true again* is the whole reason there are two stamps.

`TestNothingHeldIsNothingStored` keeps the common case free at rest too,
including a claim made and then retracted: it leaves nothing behind to store.

## Acting on what you hold

The last line of the original list, and the composer is what closes it. `world`
comes in for real: one verb, a resolver with no dice in it, and a `Witness` that
is the projection's own senses.

```
act        a Decider sees ONE actor's holdings and nothing else, and
  |        returns an Intent whose Target is a NAME. Imports no world.
  v
stage      Aim resolves that name against what is really there.
  |        Watcher answers who witnessed it, from the same senses.
  v
world      Act records the deed.
```

**A decider is handed one actor's own holdings and nothing else** — no world, no
roster, no truth surface, not even another observer's beliefs. A decider that
could see the world would act on the world, and every wrong belief beneath it
would stop mattering.

**`Intent.Target` is a name, never an entity id.** An actor that could name an
id would be reaching past its own senses to do it. So a target only exists once
the actor has perceived something *and* worked out what to call it —
`TestYouCannotAimAtWhatYouHaveNotNamed` has bram looking straight at the band
with no word for them and therefore nothing to aim at.

**And acting on a lie needed no rule.** `projection.Bind` is a pure function of
the truth surface, so a forged track is absent from it by construction. Pip names
the dragon, swings at it in perfect good faith, and `Aim` returns nothing. The
world records an attack on nobody, the ledger never mentions the illusion, and
pip still believes it afterwards — swinging taught him nothing.

| test | proves |
|---|---|
| `TestActingOnWhatYouHold` | belief → intent → aim → deed, with the audience from the same senses |
| `TestActingOnALie` | the swing is real, the target is not, and nothing special-cased it |
| `TestYouCannotAimAtWhatYouHaveNotNamed` | naming is the gate on acting, as it is on carrying |
| `TestBehaviourSeesOnlyBeliefs` | two behaviours, one situation, and one of them waits because it has only *heard* |
| `TestAnActorNobodyCanSeeIsAWiringFault` | an empty audience is a perfect sneak, so a missing actor fails loudly |

**The witness and the perception pass are one decision.** A `Watcher` and a pass
built from the same `projection.Input` cannot disagree about who was there, which
is why witnessing lives here rather than being answered twice.

**For behaviour authors: start at [`act/README.md`](act/README.md).** `act`
imports no world. A monster's behaviour is about what the monster thinks, and
writing one should not require knowing what a verb or a journal is.

Five worked deciders show the range — one that swings at a noise, one that will
not commit to something it has only heard, one that goes to *look at* what it
only remembers, one that counts what is actually in front of it before bolting.
Every difference between them is something the actor holds.

And `act/decider_test.go` is the how-to: every test there builds a situation by
hand, with no game, no projection, no world and no journal anywhere. A behaviour
is a pure function of holdings, so testing one needs nothing but holdings.

## What a real pass costs

Every scenario above has three tracks and two observers. `scale_test.go` runs
one the shape of a fight — eighteen bodies, two channels, everyone perceiving
everyone — because the small stories cannot show what the design does under
load.

**Two findings, from the numbers rather than from reasoning about them.**

*Append-on-change becomes append-per-pass the moment anything moves.* Fifty
passes, eighteen bodies:

| | standing still | moving |
|---|---|---|
| tracks held | 36 | 36 |
| entries | 36 | 1,800 |

That is not a defect, and the obvious fix is a trap. Updating an entry's locus
in place so only payload changes append would make it read *"from tick 1 to tick
50 I perceived a goblin at cell 5"* when the goblin was somewhere else at tick 1
— a memory revised to say something nobody testified to, which is exactly the
band-aid this design exists to delete.

A moving thing genuinely produces testimony every pass. The trail is real and
every entry in it is true. The open question was never how to stop recording it;
it is whether you keep the whole trail forever — **so the measurement is what
pays for forgetting**, which had been shelved for want of a use case.

*The hot path was paying for history it never reads.* A decider asks what an
actor believes now, and a reconciler compares one track's latest testimony
against another's. Neither wants the trail, but reaching the tip meant copying
every entry of every track. [`Store.Heads`](testimony/testimony.go) asks the
cheaper question:

| 50-pass benchmark | before | after |
|---|---|---|
| 6 bodies moving | 9.4 MB/op | 107 KB/op |
| 12 bodies moving | 10.4 MB/op | 419 KB/op |
| 18 bodies moving | 11.3 MB/op | 878 KB/op |

The shape of those numbers matters more than the size. Before, cost barely moved
with body count — it was dominated by history, so it grew with *ticks elapsed*
rather than with the work. After, it scales with the bodies actually present,
which is what a pass should cost. Stored testimony is unchanged: the trail is
still there, still immutable, still every entry. Only the question got cheaper.

## Deliberately absent

Compaction. A ghost is immortal here — which was "probably correct and
definitely unpaid-for" until the measurement above, and is now the first thing
worth building. Any dependency on `encounter`, `spatial`, or `core`.
