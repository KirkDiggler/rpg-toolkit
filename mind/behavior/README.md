# mind/behavior — the deeds channel

This module is what a creature comes to HOLD about what was done, and
nothing else. Two packages:

| Package | What it owns |
|---|---|
| [`deed`](./deed) | What a deed is — verb, actor, target, place — and how it is encoded on the deeds channel |
| [`stage`](./stage) | `Land`: telling every witness what they saw, in their own terms |

It holds no map, no dice, no rules, and no opinion about what anybody does
next. The board, the rulebook and the clock all belong to whoever calls it.

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
