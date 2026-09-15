# mind/behavior — what a mind is meant to do

This module is the monster's head. It takes everything one actor believes
is around it, asks that actor's mind four questions, and answers with one
intent: attack this, walk toward that, back away, or pass.

It is deliberately small. It holds no map, no dice, no rules and no state
about the world. The board, the rulebook and the clock all belong to
whoever calls it. What is written here is only the decision.

How the D&D 5e rulebook configures these minds — which monster gets which
word, and what each word is tuned to — is the sibling page:
[rulebooks/dnd5e/behavior](../../rulebooks/dnd5e/behavior/README.md).

## A mind is four judgments and no state

One monster differs from another only in how it answers these four. Nothing
else about a mind exists.

| Judgment | The question it answers |
|---|---|
| `Judge` | Which of the things I am holding are one and the same thing? |
| `Name` | What do I call this thing, now that I have bundled it? |
| `Rank` | Which of these would I rather deal with first? |
| `Keep` | How close do I let a live creature get before I back off? |

Two consequences worth knowing. A mind cannot aim at what it has not
named: an intent targets a `Name`, never an id, so a contact the mind gives
no word to cannot be attacked, approached or fled. And a mind has no
memory of its own — everything it knows arrives on the turn, so the same
inputs always produce the same answer.

## The ladder is fixed

Once the mind has ranked the contacts and said how much room it wants, the
decision is made by a fixed ladder in `Decide`. The mind is consulted at
`Rank` and `Keep` and nowhere else. **A mind never reorders these rungs**,
and it cannot add one.

0. A live named creature is nearer than `Keep` allows, and the board finds
   a step away → **back away** from it.
1. A live named creature is within the actor's reach → **attack** it.
2. The first ranked named contact that is placed, is not here already, and
   is not fenced → **walk toward** it. Alive or remembered; a walk toward a
   ghost goes to where the ghost was last seen.
3. A fenced live creature is placed → **back away** from it.
4. Nothing to act on → **pass**.

A fence is the frightened condition: a subject the actor may not willingly
move toward. It forbids approach and nothing else, so a frightened archer
with the source in reach still shoots at rung 1.

Three rules hold the ladder together:

- **Live beats remembered.** A ghost is never attacked and never fled, no
  matter how the mind ranked it. You cannot hit a memory and it cannot hit
  you. Rung 2 is the one place the mind's ranking chooses between a live
  target ahead and a ghost behind, and the ladder does not second-guess it.
- **Unplaced is skipped.** Known to be there but not known where is neither
  near anything nor in reach of anything.
- **The two flights differ on purpose.** Keeping range is a preference, so
  an archer with nowhere to step stands and shoots. Fear is not, so a
  cornered creature still means to flee, finds nowhere to go, and stays
  where it is. That is what cornered looks like.

## One turn, worked

The scene is the bow skeleton, the arrow this module was built for. The
skeleton stands in the tomb. Alice is four cells away and shot it with a
crossbow on this same tick. Bob is one cell away with a longsword and has
attacked nobody.

**In** come three holdings, handed over as values by whoever owns the
perception store:

- `alice`, on the sight channel, current, payload saying where she stands
  and what is in her hands;
- `bob`, the same;
- `deeds|alice`, on the deeds channel, carrying "alice attacked skeleton".
  A deed is never current — it is always already in the past.

**Read.** The caller's `Reader` turns each sight payload into a `Reading`:
a place, and whether it is a live creature. The deeds payload is the one
thing this module decodes itself, because it is the one it wrote.

**Judged.** The skeleton's mind claims that `deeds|alice` and `alice` are
one thing. Nothing else made that connection — the store filed them apart
on purpose, so that a dumber monster could hold the same deed and never
work out who did it.

**Folded.** Two contacts: alice with her deed bundled in, and bob.

**Named.** Neither has a word yet, so the mind is asked, and it answers
with the member id. The name is recorded on the contact's bearer, so the
next turn finds the word already there.

**Ranked.** Alice first: her deed is against this skeleton, it is still
fresh, and she is still visibly holding something that can shoot back. Bob
second, because he is merely close.

**Kept.** The skeleton wants no room — it stands and fights.

**The ladder.** Rung 0 needs a creature nearer than zero, so nothing
qualifies. Rung 1 finds alice: live, named, placed, and four cells inside
the eighty-foot bow.

**Out** comes one intent: attack, target `alice`. The caller resolves that
name against what is really there and spends the turn.

Change one thing — alice puts the crossbow away and draws a sword — and
the ranking changes, so the same ladder swings at bob instead. The rungs
never moved.

## What a mind is not allowed to know

- **Perception is a tool it holds, not a thing it drives.** The store
  belongs to whoever runs the passes. What an actor holds arrives on the
  turn as values. Behaviour writes back through exactly one door, to land a
  deed, and nothing about an intent ever crosses back. Perception never
  learns that intents exist.
- **Geometry is the caller's.** A place is an opaque string. Whether it
  means a room, a cell or a hex is the caller's business. The caller's
  `Space` answers three questions and only three: how far apart two places
  are, where one step toward lands, and where one step away lands. The
  proofs in this module use rooms joined by doors; a real board uses cells
  and a pathfinder; the ladder cannot tell the difference.
- **No live state, ever.** The mind reads a situation that has already been
  copied out. A walk is resolved against belief, never against truth, so a
  monster searching the wrong room is behaving correctly and never leaks a
  position. Only a swing asks the world whether the thing is actually
  there.
- **No vocabulary for content.** This module does not know what a bow is,
  what hit points are, or what a spell does. A payload it did not write is
  read by the caller's `Reader`, or decoded by the mind itself if the mind
  cares.

## Where the rest is written down

- [`docs/ideas/mind/behavior/design.md`](../../docs/ideas/mind/behavior/design.md)
  — the rules, R1 to R13, and the use case each one was bought with.
- [`docs/ideas/mind/behavior/scenarios.md`](../../docs/ideas/mind/behavior/scenarios.md)
  — the arrows: the scenes a player should see, and which piece owns each
  part of them.
- [`docs/ideas/mind/behavior/adoption.md`](../../docs/ideas/mind/behavior/adoption.md)
  — how the D&D 5e encounter pays for all this.
