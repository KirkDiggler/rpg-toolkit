# ADR-0051: A Driven Turn Can Activate an Ability — the Fifth Turn Intent

**Date:** 2026-11-28
**Status:** Accepted

## Context

A driven member's turn is a sealed vocabulary of four intents: `Pass`,
`Attack`, `Move` and `Routed` (`rulebooks/dnd5e/encounter/turndriver.go`,
`TurnIntent`). ADR-0043 gave a monster's turn its driver; the fourth intent,
`Routed`, earned its own design (rpg-project `ideas/spells/command`), and the
type's comment says a fifth "should expect the same". This is that record.

Three things changed underneath the four, in rpg-project#1131:

- **A creature's turn is its sheet's.** The sheet holds its action, bonus
  action and reaction; a swing, a Disengage and a run are charged to it at the
  one gate; and the encounter reads what is left from the host
  (`SheetFacts.Turn`) instead of counting.
- **A creature can hold an ability.** A stat block says `lets`, and the creature
  built from it holds a kit: the named abilities it may use and a count for
  each (`rulebooks/dnd5e/kits`, `monster.Data.Kit`).
- **The rules run it.** A monster runs a described ability as its own actor
  through the action door, paid from its own kit.

Nothing makes a creature use what it has. A behaviour's table row says one
thing a creature does with its time (`hold`, `attack`, `toward`, `dash`,
`away`, `search`), and none of the four intents can carry "use this ability".
The operator ruled the row's word on 2026-11-28: `activate`.

Two existing shapes look close and are not it:

- **The driven arm** (`Disengager`, `Dasher`) is for what the COMPOSITION
  decides and carries out itself: it knows a walk is about to leave a hostile's
  reach, or that a run adds a creature's speed. An ability is content. The
  composition cannot know what Second Wind does, only that a row named it.
- **`Attack`** names one of the member's listed actions, with a target and a
  reach the composition judges. An ability has no reach here and, today, no
  target.

## Decision

**1. A fifth intent, `Activate`.** It names one ability by the word an author
writes in a row (`second wind`). It is NOT terminal: the member's turn goes on
and its driver is asked again, as after an `Attack`. So one turn can use an
ability and still swing.

**2. The host says what a member can use, and whether it can pay for it now.**
`SheetFacts` gains the abilities a member has: for each, the author's word, a
display name, and whether the sheet can pay its whole price this moment. The
encounter does not learn what an ability costs or does (design law C1: this
module does not know the rulebook). It learns three facts and carries them.

**3. Affordability is eligibility.** A row that says `activate` is on a
member's table only when the host lists that ability for the member AND says it
can be paid for now AND the clock is a turn. A member that lacks the ability,
has spent its count, or has spent the slot it costs is never offered the row. A
round of the world offers it to nobody, as it offers no swing: a round of the
world carries no action economy.

**4. A new seam, `Activator`, that resolves AND records.** The encounter asks
the host to have the member use the ability. The host resolves it at the one
door (the price is charged before anything lands) and tells the story itself,
as a `Striker` does for a swing: the composition cannot know what the ability
did, so it cannot tell it. The story is the two beats a player's activation
already tells: `activated`, naming the ability, and one `activation-result` for
each thing it did. An error from the seam is a malfunction and aborts the
caller's verb, as a `Striker`'s does. The seam is OPTIONAL: with none wired no
member is offered the row, and an `Activate` intent from any other driver ends
the turn with a stayed beat that says why.

**5. After the ask, the composition reads the sheet again.** It keeps no count
of abilities used. The next view is built from the host's facts, which now say
the ability cannot be paid for, so the row is off the table by the same rule
that put it on.

**6. A row's word is judged twice, by who can know.** The site format judges
the shape: `activate` is one non-empty word, alone on its row as `attack` or
`dash` is. The HOST's content check judges the word: a word that is no ability
the rules describe (a misspelling, `secnd wind`) is refused when the site is
saved and when it is launched, naming the row. The composition cannot make that
judgement, and a row that is silently never offered is a failure an author
cannot see. A word that IS an ability, on a table whose creature does not have
it, is not refused: one table is shared by several creatures, so that row is
never offered to the ones without it, and the site check says so in a sentence.

## What this does not decide

- **Targets.** The only named ability today lands on its user. A row that uses
  an ability on someone else needs a selector, as `attack` has; it is added
  with the first such ability.
- **When to use it.** A row's `when` is the place for "at 30% of its hit
  points"; reading a creature's own hit points is the next step of
  rpg-project#1131.
- **Row words as a published table.** They are still typed into the studio;
  this adds one word to that list.

## Consequences

### Positive
- An NPC's ability is content from end to end: an author writes it on a stat
  block and names it in a row; no engine code names Second Wind for a monster.
- One gate, one way: the same price is charged to an NPC's sheet as to a
  player's, and the same beats tell it.
- The composition stays ignorant of the rulebook: three facts in, one ask out.

### Negative
- A fifth case in a sealed vocabulary every driver and every switch over
  `TurnIntent` must now handle.
- A host must answer "can it pay now" for every ability of every driven member
  each time a view is built. For a creature with no kit that is nothing; for
  one with a kit it is a read of its sheet.

### Neutral
- `Disengager` and `Dasher` stay as they are. They are the composition's own
  moves, not content, and folding them into `Activator` would hide that.

## Example

```yaml
templates:
  gate-lieutenant:
    base: dnd5e:monsters:human
    name: Lieutenant
    hitDice: 2d8
    actions: [dnd5e:weapons:longsword]
    lets:
      - use: second wind
        with: { level: 1 }
        uses: { max: 1, refills: { on: [short rest, long rest] } }

behaviorDefinitions:
  lieutenant-policy:
    table:
      time:
        - { activate: second wind }
        - { when: { enemy: walk },  toward: enemy }
        - { when: { enemy: reach }, attack: enemy }
```

On his first turn the Lieutenant uses Second Wind (a bonus action and his one
use), then walks or swings as his other rows say. On every later turn the first
row is not on his table: his sheet cannot pay for it.
