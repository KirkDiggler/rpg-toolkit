# `play/` — the composable pieces of live play

o/ Hi. If you landed here trying to work out how a walk, a fight, or "wait,
what did that goblin actually *see*?" gets modelled, this is the right floor of
the building.

`play/` holds the **leaves**: small modules that each own exactly one concern
about a game in motion and know nothing about each other. None of them know what
D&D is. None of them can tell you what happened in your game — they hold the
pieces that something above them assembles into an answer.

## The four

| Package | Owns | Deliberately does *not* |
|---|---|---|
| **`clock`** | Who may act, and what advances time. Two clocks: `Tick`, the player-driven world clock, and `Turn`, a localized initiative bubble — plus `Transfer`, which moves an entity between them atomically. | Decide *when* a bubble should form. Trigger detection belongs to the composition. It also contains no randomness — orderings arrive from the caller. |
| **`intel`** | What each observer *believes*: channel-sourced holdings that may be false, may be stale. Deciders consult their own intel and nothing else. | See the world, or verify anything. Illusions, disguises and planted lies are ordinary testimony here. |
| **`interrupt`** | Custody of open windows — a resolution that stopped and is waiting on an outside decision, frozen as an opaque value. | Interpret an option, a choice, or a single byte of a payload. Custody, not execution. |
| **`record`** | The retained story: append-only, sequence-ordered, audience-projected, tag-queryable entries. | Interpret an entry, or stream it. Payloads and tag vocabularies belong to the composition. |

## What all four promise

"Leaf" means the same thing in every one of them:

- depends only on `core` and the standard library
- takes no `context.Context`
- **returns its results as values, and never publishes to a bus**
- is atomic per verb — on a non-nil error, state is unchanged
- round-trips its state through `ToData` / `Load…`, and rejects invalid state on load
- contains no randomness
- carries a **numbered design contract, R1–R10**, in `docs/ideas/play/<name>/design.md`

That last one is worth knowing about before you argue with one of these modules:
the rules are numbered, and the code cites them by number. If a comment says
`(R6)`, there is a specific sentence in a specific design doc it is pointing at.

## Why leaves, and why they never publish

Because a game that stops has to be able to start again.

Everything above these modules is load-act-save: a request loads state, acts,
saves, and dies with the response. Nothing is held between calls. A module that
returned its results by publishing to a bus would need a live subscriber at the
moment it ran — which is exactly the thing that does not survive a process
restart. A module that returns values doesn't care: the value gets persisted, and
the answer can arrive days later on a different machine.

This is also why `interrupt` freezes a suspended resolution as a *value* rather
than a goroutine or a stack. No stack means nothing to preserve.

## Where they get assembled

A **composition** is the courier. For D&D 5e that is `rulebooks/dnd5e/encounter`:
it surveils percepts into intel, lets deciders act on their own intel, appends
the story to record, and pumps the clock. The composition is the first layer
allowed to have an opinion about the game — rules and trigger detection live
there, never down here.

Above that sits a **host seam** (`rulebooks/dnd5e/session`), which is what a game
server actually talks to.

Each layer absorbs wiring so the layer beneath it doesn't have to.

## A worked example

`clock/dos2_test.go` runs the Divinity: Original Sin 2 split-party scenario end
to end, and it is the fastest way to understand what these modules feel like in
combination:

1. Four players and a goblin are on the world clock, free-roaming.
2. Alice and Bob pick a fight. A `Turn` bubble forms around the three of them.
3. **The distant pair keeps exploring** — their own moves keep advancing the
   world clock while the fight runs. They are not paused, and they are not in
   the fight.
4. Carl wanders too close and `Transfer`s into the bubble mid-round, at a slot
   the rulebook chose.
5. The round wraps with Carl in the order.
6. The fight ends. `Dissolve` hands back the members, and everyone re-homes to
   the world clock.

The test asserts the full milestone transcript, so it doubles as the
specification for what each verb emits.
