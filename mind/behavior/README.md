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
4. **What the creature has seen, heard and suffered.** An entry's `When` is a
   LIST of conditions read against the caller's own `Facts`, and every one must
   hold (an empty list is a standing order). An entry whose conditions do not
   all hold is ABSENT from the roll, not weighted zero.

### The conditions

Each `When` in the list names exactly one of these; two in one `When` is
refused, because the list is already the `and`.

| condition | holds when | reads |
|---|---|---|
| `enemy: reach \| seen \| remembered \| lost \| none` | the nearest opposed thing is in that band (the bands are exclusive); `lost` is a memory the creature went back to and found empty | `Facts.EnemyInReach/Seen/Remembered/Lost` |
| `enemy: walk \| dash \| far` | what this turn's movement buys against the foe `enemy` names: a walk puts me in reach, only a dash does, or not even a dash. Exclusive among themselves and with `reach`; they sit beside `seen` and `remembered`. The host computes them from both creatures' speed and reach, and no number is written here (rpg-project#671, R18) | `Facts.EnemyWalk/Dash/Far` |
| `threat: none \| walk \| dash` | who ELSE could reach the cell this row's move lands on (where the creature stands, for a row that does not move) and swing next turn: nobody, a walking foe, or only a dashing one. The row's own target does not count. Judged per row: the host answers `Facts.Threat(row)`, asked only for rows that name a threat; nil or an empty answer holds nothing, `none` included; the row's verdict records the answered word as `RowVerdict.Threat` (rpg-project#671, R18) | `Facts.Threat` |
| `<deed>: { within: N, on/as }` — `attacked`, `intimidated`, `persuaded`, `fled` | that deed was done to me (or my side, or by me) within N units | `Facts.Deeds/AllyDeeds/OwnDeeds` |
| `heard: { kind, within: N }` | a sound of that kind (or one a `/…/` pattern matches) was heard within N units | `Facts.Sounds` |
| `fact: <id>` | the creature holds that world fact (or one a `/…/` pattern matches) | `Facts.Known` |
| `not: { <one condition> }` | the condition inside does not hold | whatever it wraps |

```yaml
- { when: [{ fact: guards-turned }, { not: { enemy: seen } }], toward: { at: [20,12] } }
- { when: [{ enemy: dash }, { threat: none }], dash: enemy }   # the careful dasher
```

`Answer.Validate` and `When.Validate` refuse the shape faults this module's
types cannot carry, each as a sentence for the form:

- a `When` that does not name exactly one condition;
- a `threat` that is not `none`, `walk` or `dash`;
- a `heard` with no kind or a span below 1;
- a deed's `within` or scope on a condition that is not a deed;
- a selector naming no target or more than one;
- a `path` off `toward` and `dash`;
- `sound` on `attack` (a sound cannot be struck);
- `sound` on a row whose list names no `heard` condition (R10);
- a `search` distance below 1.

Which words are legal under which key, which enemy and deed words a build
reads, and a deed span below 1 stay the dialect's door.

### Names and patterns

A `heard` kind and a `fact` id are a `Name`: a literal, or a pattern written
between slashes (`/clang|bell/`) that matches the WHOLE name. A bare string is
a literal and marshals as the plain string it always was. Patterns match
authored names only, never an engine payload (rpg-project#684); whether a
pattern matches anything the file authored is the dialect's to refuse.

### Groups: a row whose answer is a table

A row with `Rows` is a GROUP (rpg-project#670). It carries no word of its own.
At its level it is one candidate: eligible when its own `When` all hold and at
least one row inside it is a candidate, loaded at its `Weight` × the
temperament percent of the word its `As` names (100 when empty). When the die
lands on it, a second die rolls over its own candidates, and so on down, to
`MaxGroupDepth`. The output's `Path` names the row picked at every level
(`Entry` is `Path[0]`), and a group's `RowVerdict` holds its rows' verdicts
and the inner die. A bundle name is the dialect's; by the time a table gets
here it is only the tree, with `From` and `Params` kept for the record.

### Selectors

`attack`, `toward`, `dash` and `away` take a `Selector`: exactly one of a
word, a cell (`at`), or a route (`path`, `toward` and `dash` only). The words
are `enemy`, `attacker`, `actor` and `sound`.

- **`attacker`** is who last attacked the creature, in the reading the
  row's deed condition names (`Facts.Attacker`). The row is a candidate only
  while that member is in `Facts.FoesInReach` (to strike) or `SeenFoes` or
  `RememberedFoes` (to walk); otherwise it is absent, and the author's
  sibling `attack: enemy` row is the fallback through the roll.
- **`sound`** is where the freshest held sound was made. It is legal on
  `toward`, `dash` and `away` only, and only on a row that names a `heard` condition
  (R10), the way `actor` needs a deed. Resolving it is the caller's.
- **`search: { distance: N }`** walks on from a lost trail for N cells. It
  is eligible only while `Facts.EnemyLost` holds and the creature can move,
  and it carries the `toward` temperament factor. The trail, its heading and
  when the search is over are the caller's.
- **`dash: <selector>`** runs at the target, spending the turn on the run
  (rpg-project#671, R17). It takes `toward`'s selectors and carries the
  `toward` temperament factor. A dash at a foe already in reach is absent,
  not a pass: `dash: enemy` while `Facts.EnemyInReach` holds, `dash:
  attacker` while the attacker is in `Facts.FoesInReach`. Every other
  threshold is the row's own `when`, so the dasher reads `when: { enemy:
  dash }` and `dash: enemy`. What a run costs is the caller's.
- **A route** is a patrol and holds no state: which waypoint is next is the
  caller's to derive from where the creature stands each time the row fires.

**Affordability is eligibility.** `Facts.CanAttack` and `Facts.CanMove` say
what the creature can still pay for, and an entry it cannot pay for is absent
for the same reason an unmet condition is. A `dash` also needs
`Facts.CanDash`: the run spends the creature's action, so the host clears it
once the swing is spent on a turn, and a run it cannot pay for is absent
rather than picked and refused (rpg-project#671). A pick that cannot act is noise on
the log. `hold` is never gated — it is the word that lets a creature with
nothing left still have HAD its turn — and the zero value is "cannot", so a
caller that forgets a budget gets a creature that holds rather than one that
quietly swings.

### What it deliberately does not know

One trigger is named here, `KeyTime` — a creature having time is the one thing
every game has. Every other key is the caller's to name, seal and refuse. The
words on an entry (`Fact`, `Flee`, `Hold`, `Attack`, `Toward`, `Dash`, `Away`, `Search`) are
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
  written. A witness holds the freshest deed of EACH KIND by each actor, so a
  flight it was the target of and a shout it merely watched the same figure
  make are two memories rather than one overwriting the other.
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

## The sounds channel

`sound.Land` is the deeds channel's sibling for noise. A `Sound` is a kind, a
place and a time — and **never a maker** (rpg-project#569, R4): a sound has no
figures in it, so every hearer holds the same report and nothing is blanked.
A guard who hears a clang knows something metal happened over there, not who
swung; finding out who is what walking over and looking is for. Who is close
enough to hear, and how loud each kind is, are the caller's physics.
`sound.Held` reads a creature's holdings back into `Facts.Sounds`; a hearer
holds the latest landed sound of each kind, which is the freshest when the
caller lands sounds in clock order.

## The arrow points one way

This module depends on `mind/perception`; perception never learns anything
about it (R1). `stage.Land` and `sound.Land` write through the store's own
`Report` door and run no perception pass of their own.

Design contract: `docs/ideas/mind/behavior/design.md`. R7 (the ladder) and R11
(the Space) are retired with the code they bound. R2 (a payload is the
caller's to read, except the deeds and sounds channels this module wrote) and
R9 (a deed lands in each witness's own terms) are what remains.
