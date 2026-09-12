# examples/perception

A spike. It owes nothing to the shipped perception path and is not a migration
of it — it exists to find out whether a four-layer shape survives contact with
the scenarios that matter, before any of it is proposed as a module.

Depends on nothing but testify.

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
carry                the distillation: what leaves a finished run, re-keyed
                     onto a name, landing with a player as a ghost.
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

`carry` is the one place folding is legitimate — deciding what you took away is
an authored act, not a read. It still refuses a tie: two tracks under one name,
equally fresh, would have to be separated by handle order, and that would be the
package inventing a conclusion on the observer's behalf.

## What leaving a run also surfaced

**It needs a clock that outlives the run.** A carried belief keeps its
`Confirmed` stamp so staleness stays measurable afterwards — but only if that
number still means something. Re-stamping to the moment of leaving would make
nine-day-old knowledge look fresh, which is the exact staleness lie the two
stamps exist to kill. Carrying the stamp out unchanged is the honest half; the
other half is a clock above the run, and there isn't one.

**Carry-in is unbuilt.** A carried belief is only ever refreshed by carrying
again — perceiving the same goblins in a later run mints new run-local handles
and touches nothing a player holds. Entering a dungeon *already believing*
something is the inverse move and the obvious next gap.

## Deliberately absent

Persistence (`ToData`/`Load…`). Forgetting or compaction — a ghost is immortal
here, which is probably correct and definitely unpaid-for. Any dependency on
`world`, `encounter`, `spatial`, or `core` — the next move is testing the fit
against real `world`, and that is when the testify-only property gets spent.
