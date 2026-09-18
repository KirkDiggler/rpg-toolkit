# mind/behavior — the creature's table, and the deeds channel

mind/perception says what a creature **knows**. This module says what it
**does** with that: it rolls an authored weighted table.

| Here | What it owns |
|---|---|
| the root package | `Table`, `Layer`, `Pick`, `Deal` — the policy primitive and its evaluator |
| [`deed`](./deed) | What a deed is — verb, actor, target, place — and how it is encoded on the deeds channel |
| [`stage`](./stage) | `Land`: telling every witness what they saw, in their own terms |

It holds no map, no clock, no rules, and no opinion about what anybody does
next. The board, the rulebook and the clock all belong to whoever calls it.

## The table

```go
out, err := behavior.Pick(ctx, &behavior.PickInput{
    Key:    behavior.KeyTime,
    Table:  creature.Table,   // already layered by whoever owns the layers
    Temper: creature.Temper,  // a word and what it multiplies, or nothing
    Facts:  facts,            // what the creature holds, projected by the caller
    Die:    dice,             // Roll(ctx, sides) — the smallest thing this needs
})
```

`out` carries every eligible entry with its authored weight, its temperament's
factor and their product, the die, the face, and the entry that fired. A table
nobody can replay is a table nobody can trust.

### Four things load the die

1. **The layers.** `Layer(base, over)`: for each key the nearer layer wins
   WHOLESALE. No merging of entry lists, so an author never has to reason about
   what was added to what.
2. **The weights**, as written.
3. **The temperament.** A percent multiplier per table word, and nothing else:
   it adds no entries, holds no memory, carries no trigger. `Deal` picks one
   out of a faction's spread; the vocabulary is the caller's, and any word that
   arrives with a profile is multiplied by it.
4. **What the creature has seen and suffered.** An entry's `When` reads the
   caller's own `Facts` — how close the nearest opposed thing has got, or a
   deed done to this creature within N units. An entry whose condition is false
   is ABSENT from the roll, not weighted zero.

**Affordability is eligibility.** `Facts.CanAttack` and `Facts.CanMove` say
what the creature can still pay for, and an entry it cannot pay for is absent
for the same reason an unmet condition is. A pick that cannot act is noise on
the log. `hold` is never gated — it is the word that lets a creature with
nothing left still have HAD its turn — and the zero value is "cannot", so a
caller that forgets a budget gets a creature that holds rather than one that
quietly swings.

### What it deliberately does not know

One trigger is named here, `KeyTime` — a creature having time is the one thing
every game has. Every other key is the caller's to name, seal and refuse. The
words on an entry (`Fact`, `Flee`, `Hold`, `Attack`, `Toward`, `Away`) are
DATA: this module knows which one fired and has no idea what attacking is or
where `toward` walks to. A `Selector` is carried and resolved by the caller.

That is the composability claim. The table came out of a D&D composition; what
was D&D about it stayed there.

## What used to be here

This module was the monster's head. It took everything an actor believed was
around it, asked that actor's mind four questions — which holdings are one
thing, what to call it, which to deal with first, how close to let it get —
and answered with one intent, through a fixed five-rung ladder.

All of that is deleted (rpg-project#465, `ideas/creature-table/design.md` §7).
A creature decides by rolling on an **authored weighted table** now, and four
things load that die: the rulebook's default table for its kind, the author's
orders, its own temperament, and what it has seen and suffered.

The ladder was not wrong. It was a black box with three words on the lid, and
a streamer could not open it. A table is a tool an author can hold, and the
tools are the product.

## Why the deeds channel stayed

The mind got two things right, and the table reads both rather than
reimplementing them:

- **The outcome of anything is testimony a creature holds, never a flag.**
  A creature that was threatened holds a deed saying so, stamped when it
  happened and never restamped. A table's `when: { attacked: { within: 3 } }`
  is a question about that testimony, and this is where the testimony is
  written.
- **Who a creature believes is where comes from perception, per observer.**
  `Land` writes a deed into each witness's own store, naming the figures in it
  only to witnesses who could actually see them — a witness who could not see
  the healer learns that a heal happened and not who did it.

The rest went with the ladder that read it: `Contact`, `Reading`, `Reader`,
`Name`, `Self`, `Situation`, `Space`, `Verb`, `Intent` and the `Game`. Nothing
imported them but the ladder and the presets built on it, and a projection of
holdings belongs beside whoever is deciding from them — the composition builds
its own, per pick, against a board it actually has. Two projections of one
truth would be two truths to keep in step.

`rulebooks/dnd5e/behavior`, the sibling that configured these minds, is
deleted outright in the same wave.

## The arrow points one way

This module depends on `mind/perception`; perception never learns anything
about it (R1). `Land` writes through the store's own `Report` door and runs no
perception pass of its own.

Design contract: `docs/ideas/mind/behavior/design.md`. R7 (the ladder) and R11
(the Space) are retired with the code they bound. R2 (a payload is the
caller's to read, except the deeds channel this module wrote) and R9 (a deed
lands in each witness's own terms) are what remains.
