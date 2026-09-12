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
                     claims, never stored.
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

**Identification** — *that contact is Bob*. Deliberately not built. No use case
has paid for it.

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

## Deliberately absent

Identification. Persistence. Forgetting or compaction — a ghost is immortal here,
which is probably correct and definitely unpaid-for. Any dependency on `world`,
`encounter`, `spatial`, or `core`.
