# examples/behavior

A spike, beside `examples/perception` and downstream of it. It exists to find
out what a monster's mind has to be, before any of it is proposed as a module
or handed to the behaviour lane as a foundation.

**Perception is a tool behaviour holds.** A monster's mind reads what it
perceives and decides; perception has no business knowing an intent exists.
That is why this is its own module importing `examples/perception`, and why the
`act` and `stage` packages that first appeared inside perception belong here.
The arrow only points one way.

We are learning. This README is the record of what we think until the code has
taught us otherwise; the design doc comes after, not before.

## The shape

```
truth surface     world (journal + graph)  |  regions, presences, forgeries
      |
      v
perception        projection -> testimony -> reconcile -> belief -> carry
      |           the read surface. behaviour never writes to it except
      |           through Report, and only from the stage.
      v
behaviour         situation -> mind -> ladder -> intent
      |           imports perception. imports no world.
      v
stage             aim / recall / witness / land deeds
      |           the composition step: the only package that knows both an
      |           intent and a verb.
      v
world.Act         records the deed, with an audience
      |
      +-----> stage lands a deed percept on every witness, and the loop closes
```

Nothing below the mind reads truth to answer a question about an actor.
Nothing above the stage knows what a verb is.

## The nouns

**Situation** — everything one actor has to go on. Its held tracks, its
contacts as folded by its *own* mind, its self, and a stamp. Built from
perception's read surface. Nothing about anyone else that did not arrive
through a channel.

**Self** — the part of a situation that is not perception: own locus, the
regions one step away (static topology, construction truth), what the sheet
says it is armed with, fences, whether the reaction is spent, and the current
commitment. The knowledge-only contract is about *others*. Your own sheet and
your own dungeon's doors are yours to read; who stands behind them is not.

**Mind** — four judgments and no state of its own:

| judgment | question | zombie | captain | archer |
|---|---|---|---|---|
| Judge | are these tracks one thing? | never | a chant is the one robed figure; a deed is who I saw do it | never |
| Name | what do I call this contact? | "thing N", reflexive | by how it looks | reflexive |
| Rank | which named contact first? | whatever I noticed first | the healer, then the chanter, then first noticed | whatever I noticed first |
| Keep | how close do I let a live creature get? | 0 | 0 | 1 |

Judge is the reconciler perception already has. A mind *is* a reconciler plus
three more questions, so a behaviour author writes one type. The archer is a
zombie with a bow on its sheet and `Keep` returning 1 — kiting is one number.

**Ladder** — fixed, and not the mind's to change:

0. a **live** named creature is nearer than the mind keeps, and there is
   somewhere to step → `Away{Name}`
1. a **live** named creature is within reach → `Attack{Name}`
2. a ranked named contact, live **or ghost**, is placed, not here, and not
   fenced → `Toward{Name}`, honouring commitment
3. a fenced live creature is placed and there is somewhere to step →
   `Away{Name}`
4. nothing to pursue → `Pass`

Live beats remembered: a ghost is never attacked and never fled. Fences are
read from self; the ladder respects them and never offers them to the mind as
a choice. A fence forbids approach and nothing else: frightened is not
disarmed.

**Intent** — sealed: `Attack{Name}`, `Toward{Name}`, `Away{Name}`, `Pass`. The
target is always a name the actor gave. **You cannot aim at what you have not
named**, which is why naming is a judgment of the mind rather than a step
somebody performs on the monster's behalf.

**Commitment** — a claim about oneself in the belief store: *pursuing that
name*. Retracted when the target resolves (perceived again, or its position
becomes unknown). A compelled or frightened turn retracts it the way any claim
is retracted. Without this a monster between two ghosts of similar distance
dithers forever.

## Two grains in the stage

**Aim** binds a name to the truth surface, for a swing. An illusion binds to
nothing and the swing hits air, and no rule had to say so.

**Recall** binds a name to the actor's *own remembered locus*, for a walk. A
walk toward a ghost never consults truth, so a monster searching the wrong room
is correct behaviour and never leaks a position.

One name, two resolutions. Which one applies is decided by the intent, never by
the mind.

**Deeds close the loop.** After `world.Act`, the stage takes the fact's audience
and Reports a deed percept to each witness on the `deeds` channel, referencing
actor and target by *that witness's own tracks* of them. A deed is always held,
never current: it is in the past the moment it exists. It is a track like any
other, so a zombie's Judge never merges it with the sight track and the zombie
cannot know who healed whom. The captain's Judge does. Same bytes, different
minds. This is the rule the whole example rests on, and the easy road — stamping
the deed onto the actor's sight track so every observer knows automatically —
was refused because it forecloses the dumb monster and the illusion in one move.

## What it proves, in order

Each fixture pays for one primitive. Nothing is built before the fixture that
needs it.

| # | fixture | pays for | built? |
|---|---|---|---|
| 1 | zombie and captain, one situation, two targets | contacts in the ladder; Name and Rank | **yes** |
| 2 | a heal in sight retargets the captain; the same heal out of sight does not | deeds as testimony | **yes** |
| 3 | archer fires from the next region, steps `Away` when someone enters its own | `Away`; Recall; rungs 0 and 2 | **yes** |
| 4 | intimidated goblin cannot go `Toward` the intimidator; shoots if it can, flees if it cannot | fences on self; routing | **yes** |
| 5 | zombie walks to a stale ghost forever; captain drops it after N and returns to post | Rank over ghost age; commitment | no |

## What the fixtures taught

Things the shape did not know before there was code.

1. **A name is on one track, and a swing goes there.** The first draft of
   `Aim` resolved a name through *any* track in the contact. The
   captain-can-be-wrong test sent its swing at the chant, and truth bound the
   chant to the knight. A contact is a claim that several tracks are one thing,
   and the claim can be wrong — so `Contact.Bearer` records which track the
   name is on, and the stage binds only that. The wrong merge costs the captain
   a claim; it never costs it a swing at the wrong person.
2. **Woodwise is too credulous at region grain.** The perception spike's
   reconciler merges a noise with *every* co-located creature, so with two
   figures in one room a chant bundles both into one contact. The captain has
   its own Judge: merge the chant into the one robed figure standing there,
   and on two, or none, make no claim. A tie is worse than no claim — the
   same rule carry-out already has for a distillation.
3. **A mind is a reconciler plus two questions, and that is enough.** Zombie
   and captain are each one type, no state, three methods. The zombie's whole
   stupidity is `Judge` returning nil.
4. **Naming lands as a claim and persists.** The mind is asked only for
   contacts the actor has no word for; the next situation finds the word
   already there. A behaviour author never sees `Identify`.
5. **The perception example still carries `act` and `stage` on main.** They
   are the seam this module now owns. Retiring them from perception is a
   perception PR, and it waits until this shape has settled.
6. **Perception needed one door, and only one.** `Tick` was its only write,
   so nothing above the composition could land a deed. `Game.Report` (toolkit
   #1678) passes the store's existing verb through and judges what landed —
   that is the whole change, and it is the sign the arrow points the right
   way: behaviour pulled exactly what it needed and nothing about intents
   crossed back.
7. **A deed names actor and target in the witness's own handles.** The stage
   translates a fact (ledger ids) into what each witness would say — *the
   hooded one I can see healed the armoured one I can see* — and only if the
   witness currently holds that sight track. A witness who could not see the
   healer learns a heal happened and not who did it. Nothing is written to
   anybody's sight track.
8. **Attaching a deed to a figure is a claim.** The captain's Judge merges the
   deeds track with the sight track it names; the zombie holds the same deed
   and never does. A mutant that stops the captain attaching is killed by
   exactly the two assertions that claim it.
9. **One deeds track per figure, per witness.** The change key is
   verb + actor + target, so a second identical heal extends the watermark
   rather than appending. What the witness knows has not changed; only how
   recently it was confirmed.
10. **Keeping range is a distance, not a rank.** The open question was
    whether Rank needed the actor's own locus. It does not: *how close do I
    let things get* is its own judgment, `Keep`, and the ladder reads it as
    rung 0 before it considers attacking. The archer is a zombie with a bow
    and `Keep` returning 1. A mutant returning 0 stands in the corridor and
    shoots point-blank; the test kills it.
11. **What you are armed with is a sheet fact; what you want is a mind
    fact.** `Sheet.Reach` says how far the bow shoots. `Keep` says how far the
    archer would rather stand. A cornered archer may decide to keep nothing,
    and the sheet does not change.
12. **A walk resolves against belief; a swing resolves against truth.**
    `Recall` reads only the situation and `Aim` never enters a walk. The
    first draft of `Recall` had two branches — the live placement, else the
    freshest memory — and a mutant that disabled the live branch *survived*.
    It was equivalent: a current track's latest entry is its placement, so
    the memory rule already answers for a live contact. The branch was
    deleted. A ghost is not a special case of recall, only an older one.
13. **Region grain cannot tell direction, but a map can tell distance.**
    The first `Step` picked any adjacent region that was not the target's.
    Fixture 4 needed a walk of two regions and could not take the first step,
    so routing moved onto the game, where static topology already lived:
    `Toward` takes the first door on the way, `Away` takes the door that puts
    the most dungeon between them. On a tie the archer may still back into
    the room the knight came from; that is the grain, and cells are the
    shipped encounter's business.
14. **A fence is a rule, and rules are the ladder's.** The frightened
    condition arrives on the sheet in ledger terms and is translated once, at
    the composition, into the actor's own sight handle of the source. From
    there the ladder refuses `Toward` a fenced contact and flees instead;
    `Attack` is untouched. The mind was never asked. A mutant that lets the
    ladder approach a fenced contact is killed. How the *mind* comes to know
    who frightened it is a deed like any other, and no fixture has paid for
    it yet.
15. **Fleeing into a corner is not fleeing.** `Away` refuses a dead end
    rather than stepping closer. The goblin in the hall, afraid of a knight
    two rooms off, has nowhere to go and stays; when the knight comes within
    bowshot it shoots. The ladder cannot know the map, so an `Away` the
    stage refuses is an actor that stays put — which is what cornered means.

## What the spike simplifies, on purpose

- **Geometry is region grain.** In reach is *same region*; ranged is *adjacent
  region*. Cells and distance are the shipped encounter's business, and the
  ladder does not change when they arrive.
- **No dice.** Resolvers are scripted, as in the perception spike.
- **No persistence** beyond what perception already round-trips.
- **No authoring.** Minds are Go types. What a designer gets to write on a
  monster definition is a later rung, once these three minds have shown which
  knobs are real.

## What we do not know yet

- How a mind comes to know who frightened it. The fence is on the sheet;
  the deed (*that one intimidated me*) would land like a heal does, and a
  mind could rank by it. No fixture has needed it.
- Whether *return to post* is a fourth intent or `Toward` a name the monster
  gave its own post at spawn. The latter costs nothing, if a post is something
  a monster can perceive.
- Whether commitment as a self claim survives the perception store's
  refusals, which were written for claims about tracks.

## What the perception team's decider taught us

Their `act` package is the seam we wanted, and its one invariant — a target is
a *name*, never an entity id — is kept whole. What it did not have, because no
fixture had asked: positional intents, a resolution against belief rather than
truth, a self, commitment, contacts in the decision, and the deed loop. Every
one of those is a fixture above.
