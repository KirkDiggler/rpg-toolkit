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

**Self** — the part of a situation that is not perception: own locus, fences,
whether the reaction is spent, and the current commitment. The knowledge-only
contract is about *others*. Your own sheet is yours to read.

**Mind** — three judgments and no state of its own:

| judgment | question | zombie | captain | archer |
|---|---|---|---|---|
| Judge | are these tracks one thing? | never | cross-channel and sign rules | as captain |
| Name | what do I call this contact? | "thing N", reflexive | recognised name, else reflexive | reflexive |
| Rank | which named contact first? | nearest live, then any ghost, forever | the healer over others; drops a ghost older than N | ranged targets; prefers a region away |

Judge is the reconciler perception already has. A mind *is* a reconciler plus
two more questions, so a behaviour author writes one type.

**Ladder** — fixed, and not the mind's to change:

1. a **live** named contact is in reach and an attack is left → `Attack{Name}`
2. movement left → `Toward`/`Away` whatever Rank puts first across live
   contacts **and ghosts**, honouring commitment
3. nothing to pursue → `Pass`

Live beats remembered. Fences are read from self; the ladder respects them and
never offers them to the mind as a choice.

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
| 2 | a heal in sight retargets the captain; the same heal out of sight does not | deeds as testimony | no |
| 3 | archer fires from the next region, steps `Away` when someone enters its own | `Away`; Recall | no |
| 4 | intimidated goblin cannot go `Toward` the intimidator; shoots if it can, flees if it cannot | fences on self | no |
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

- Whether Rank wants the whole situation or only the named contacts. The
  archer's *prefer a region away* is a ranking over *my own* locus, which is
  self, not a contact.
- Whether a fence is a rule the stage derives from a condition, or a deed the
  mind reads (*that one intimidated me*). Probably both: the fence is the rule,
  the deed is how the mind knows who.
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
