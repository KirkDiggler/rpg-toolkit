# mind/behavior — Design (the WHAT)

**Status:** IMPLEMENTED
**Module:** `github.com/KirkDiggler/rpg-toolkit/mind/behavior` (packages `behavior`, `behavior/deed`, `behavior/stage`)
**Why:** [rpg-toolkit#1718](https://github.com/KirkDiggler/rpg-toolkit/issues/1718).
**How:** implemented directly against the issue's API and rules, the way
[mind/perception](../perception/design.md) was — no separate `plan.md`.
**Where it came from:** `examples/behavior` ([#1677](https://github.com/KirkDiggler/rpg-toolkit/pull/1677)),
the spike that proved the five use cases on the perception example. Its
README holds the nineteen lessons that shaped these rules.
**What happened:** [implementation.md](implementation.md).
**v0.2.0:** [rpg-toolkit#1723](https://github.com/KirkDiggler/rpg-toolkit/issues/1723) — geometry became the caller's `Space`.
**v0.3.0:** [rpg-toolkit#1725](https://github.com/KirkDiggler/rpg-toolkit/issues/1725) — the store became the caller's; see [adoption.md](adoption.md).

## The arrow

A monster on a real board that changes its mind because of what it
perceives. The first scene a player will see: a bow skeleton shooting at
the closest player turns on the one who shot at it, and when that player
puts the bow away and closes to melee, goes back to whoever is closest.
The mind is the skeleton's: it ranks by a deed it witnessed, the way the
captain ranks the healer first, with a shot in place of a heal. Every
slice is judged by whether it brings that scene closer.

## Scope

A monster's mind: what it holds, what it makes of it, what it means to do,
and where that lands. Behaviour is a customer of `mind/perception`
v0.3.0 — the three asks it took there are
[#1704](https://github.com/KirkDiggler/rpg-toolkit/issues/1704) — and it
never learns anything about the world except through perception and the
values a caller hands it.

**Charter.** A mind speaks in names. It is a few judgments and no state of
its own: which holdings are one thing, what to call it, which to deal with
first, how close to let it get. The ladder is fixed and not the mind's to
change. You cannot aim at what you have not named.

**Non-goals:** persistence (names and fears would need a `Data`; the
encounter integration pays for it); authoring minds as designer data;
dice; a map of its own (geometry is the caller's `Space`, R11; the proofs
use rooms joined by doors); commitment (no mind dithers yet — every worked mind ranks
by first noticed); absence as testimony (a ghost's place is never
downgraded by arriving and finding nothing; the ladder skips what is *here*
and that covers every proof); how a mind learns who frightened it (a deed
like any other, unpaid). Each arrives with the use case that pays for it.

## Rules

- **R1** — Perception is a tool. The store belongs to whoever runs the
  passes; behaviour never runs one. What an actor holds arrives on a turn
  as values, and behaviour writes to the store only through its own door,
  to land a deed (`stage.Store`). Nothing about an intent crosses back.
- **R2** — Payloads are read by the caller's `Reader`, the mirror of
  perception's `Reach`: behaviour holds no vocabulary for what a sighting
  says. A `Where` is a place in whatever unit the caller's `Space` measures;
  behaviour compares places and never interprets them. It decodes exactly one channel, `deed.Channel`, which it wrote. A
  reading the reader cannot make is `Reading{}` — unplaced, not a creature —
  and not an error. A mind may still decode the payload itself.
- **R3** — A contact is folded fresh every situation from the mind's `Same`
  claims over everything the actor holds, unioned: A~B and B~C is one
  contact of three. A mind that claims nothing holds one contact per
  subject. Contacts are ordered by their smallest subject, holdings within
  one by subject, so two identical situations fold identically. A claim
  naming a subject the actor does not hold is ignored.
- **R4** — A name persists. The mind is asked only for contacts the actor
  has no word for. A name is recorded on the contact's **bearer** — the
  current creature holding if there is one, else the first — and the next
  situation that bundles that subject finds the word already there. A
  contact is folded fresh; the bearer is what a name has to hold on to.
  When a mind later bundles two subjects that each carry a name, the
  contact wears the name of the first in subject order and the mind is not
  asked again; the other name stays recorded and unreachable until the
  bundle comes apart. Deterministic, unexercised, and open to a use case
  that merges two named contacts.
- **R5** — You cannot aim at what you have not named. `Intent.Target` is a
  `Name`, never a subject. A contact the mind gives no word to cannot be
  attacked, walked toward, or fled.
- **R6** — `Contact.Where` is the freshest placed reading across its
  holdings, live or ghost. One rule, two callers — the ladder and `Recall`.
  A current holding's latest confirmation is its placement, so the memory
  rule already answers for a live contact.
- **R7** — The ladder is fixed, and the mind is asked `Rank` and `Keep` and
  nothing else:
  0. a live named creature is nearer than `Keep`, and the `Space` finds a
     step away → `Away`
  1. a live named creature is within `Sheet.Reach` → `Attack`
  2. the first ranked named contact, live or ghost, that is placed, not
     here, and not fenced → `Toward`
  3. a fenced live creature is placed → `Away`
  4. nothing to act on → `Pass`

  Every rung skips a contact with no place, and rungs 0 and 1 skip one the
  `Space` cannot measure: not known where is not nearer than anything and
  not within reach of anything. The two flights differ on purpose: keeping
  range is a preference, so an archer with nowhere to step stands and
  shoots; fear is not, so a cornered creature still means to flee, the
  stage finds it nowhere, and it stays. Live beats remembered: a ghost is never attacked and never fled. Rung 2 is
  where the mind's ranking decides between a live target ahead and a ghost
  behind, and the ladder does not second-guess it.
- **R8** — A fence forbids approach and nothing else. The frightened
  condition arrives on the sheet as a subject, the ladder refuses `Toward`
  a contact holding it and flees instead, and `Attack` is untouched: a
  frightened archer with the source in reach still shoots. The mind is
  never offered a fence as a choice.
- **R9** — A deed lands on its own channel through perception's `Report`,
  under a subject qualified by channel (`deed.Subject(actor)`), one per
  figure the witness saw act, so a witness holds an actor's latest deed
  and nothing before it. The payload is said in each witness's terms: actor
  and target are named only if the witness currently holds them on sight —
  or is them. An observer never perceives itself, so a witness holds no
  sight of itself, and a deed done to it would otherwise name nobody; you
  know when you have been shot at.
  The subject is not — it carries the actor's identity because attaching a
  deed is the claim that `deeds|X` and `X` are one thing, and a mind cannot
  make that claim without the `X`; what the payload vouches for and what the
  store files under are different questions. A deed is never current. Only
  a mind's `Judge` attaches it to a figure. Nothing is written to anybody's
  sight holding. A deed with no actor is refused (`stage.ErrNoActor`).
- **R10** — A walk resolves against belief; a swing resolves against truth.
  `Recall` reads the situation only, so a monster searching the wrong room
  is correct behaviour and never leaks a position. `Aim` asks `Truth` which
  of a named contact's subjects is present — a chant, a deed, an illusion
  are not, so a swing at them lands on nothing and is not an error.
- **R11** — Geometry is the caller's `Space`, the third caller-owned seam
  after perception's `Reach` and this module's `Reader`. It answers three
  questions and behaviour asks nothing else of a map: how far apart two
  places are (`Distance`, unknown when there is no way), one step toward a
  place (`Toward`), one step away from it (`Away`). `Away` refuses a dead
  end — fleeing into a corner is not fleeing, and the `Space` is what knows
  where the corners are. An `Away` the `Space` refuses at the stage is an
  actor that stays put, which is what cornered means. A monster may know
  the way through its own dungeon; how the dungeon is measured is the
  dungeon's business.
- **R12** — `Self` is handed as values: `Place` for where the actor stands,
  `Sheet` for what it is armed with, `Frighten` for what it may not
  approach. Nothing in behaviour reads the
  world, and nothing a caller does to a `Situation` reaches the game: what
  a situation reports is copied out, never aliased.
- **R13** — Errors wrap exactly one sentinel and callers dispatch with
  `errors.Is`: `ErrNoReader` and `ErrNoSpace` from `New`; `ErrNoMind` and
  `ErrNoSelf` from `Situation` and `Turn`. An actor nobody placed fails loudly rather than
  passing forever, which would look like a very cautious monster rather
  than a wiring fault.

## Types

`package behavior`

- `Reader` — `interface { Read(h perception.Holding) (*Reading, error) }` (R2).
- `Reading` — `{Where string, Creature bool}`. What behaviour needs from a
  payload and nothing more.
- `Holding` — `perception.Holding` plus its `Reading`.
- `Name` — `string`. The only thing an intent may name a target by (R5).
- `Contact` — `{Holdings []Holding, Name Name, Named bool, Bearer core.EntityID}`
  with `Holds`, `Current`, `Creature`, `Where` (R6), `LastConfirmed`,
  `FirstObserved`.
- `Sheet` — `{Reach int}`, in the `Space`'s unit. Zero is the same place.
- `Self` — `{Sheet, Where string, Fences []core.EntityID}` with `Fenced`.
- `Space` — `interface { Distance; Toward; Away }`, each
  `(in *XxxInput) (*XxxOutput, error)`: `DistanceInput{From, To}` →
  `DistanceOutput{Steps, Known}`; `TowardInput{From, To}` →
  `TowardOutput{Next, Found}`; `AwayInput{From, AwayFrom}` →
  `AwayOutput{Next, Found}` (R11).
- `Situation` — `{Actor core.EntityID, Contacts []Contact, Self Self, At uint64}`.
- `Verb` — sealed: `Pass`, `Attack`, `Toward`, `Away`.
- `Intent` — `{Verb Verb, Target Name}`.
- `Pair` — `{A, B core.EntityID}`, a claim that two subjects are one thing.
- `Mind` — `interface { Judge; Name; Rank; Keep }`, each `(in *XxxInput) (*XxxOutput, error)`:
  `JudgeInput{Holdings, At}` → `JudgeOutput{Same []Pair}`;
  `NameInput{Contact}` → `NameOutput{Name, Named}`;
  `RankInput{Situation}` → `RankOutput{Ranked []Contact}`;
  `KeepInput{Situation}` → `KeepOutput{Steps int}`.
- `Game` — the composition: reader, space, minds, sheets, places, fears,
  names. It holds no perception. Zero value not usable; construct via `New`.

`package deed`

- `Channel` — `perception.Channel("deeds")`.
- `Subject(actor)` — `"deeds|" + actor`, the qualified subject (R9).
- `Deed` — `{Verb string, Actor, Target core.EntityID, Where string}`,
  with `Encode` and `Decode` (`ErrNotADeed` for anything else).

`package stage`

- `Truth` — `interface { Present(subject core.EntityID) bool }`. The one
  question a swing may ask of the world.
- `Store` — `interface { Held; Report }`, exactly `perception.Perception`'s
  two methods, so a caller hands its own store and nothing wraps it (R1).

## Verbs

| Verb | Input | Output | Semantics |
|------|-------|--------|-----------|
| `Decide` | `DecideInput{Situation, Mind, Space}` | `(*DecideOutput{Intent}, error)` | R7. |
| `Game.Turn` | `TurnInput{Actor, At, Holdings}` | `(*TurnOutput{Intent, Situation}, error)` | `Situation` then `Decide`. Both come back because the stage resolves the one against the other. |
| `Game.Situation` | `SituationInput{Actor, At, Holdings}` | `(*Situation, error)` | R1–R4, R12, R13. Reads every holding handed in, asks `Judge`, folds, names. |
| `Game.Sheet` / `Mind` / `Place` / `Frighten` | one `Input` each | — | R12. Sheet facts, handed as values. |
| `stage.Aim` | `AimInput{Truth, Situation, Intent}` | `(*AimOutput{Source}, error)` | R10. `""` is a swing at nothing, not an error. |
| `stage.Recall` | `RecallInput{Situation, Name}` | `(*RecallOutput{Where, Placed}, error)` | R10. Belief only. |
| `stage.Step` | `StepInput{Space, Situation, Intent}` | `(*StepOutput{To, Moved}, error)` | R11. One step, this turn, in the caller's space. |
| `stage.Land` | `LandInput{Store, Deed, Witnesses, At}` | `error` | R1, R9. Who witnessed is the caller's: a deed happens at a place, and whose senses reached it is the physics perception also leaves to the caller. |

## Errors

| Sentinel | Meaning | Returned by |
|----------|---------|-------------|
| `ErrNoReader` | `New` without a `Reader` | `New` |
| `ErrNoSpace` | `New` without a `Space` | `New` |
| `ErrNoMind` | the actor was never given a mind | `Situation`, `Turn` |
| `ErrNoSelf` | the actor was never placed | `Situation`, `Turn` |
| `stage.ErrNoActor` | a deed with no actor | `stage.Land` |

A `Store`'s errors pass through `stage.Land` unwrapped; they are
perception's vocabulary and the caller owns the store. A `Reader`'s and a `Space`'s errors pass through the
same way, from `Situation`, `Decide`, `Turn`, and `stage.Step`: they are
the caller's vocabulary, and a pathfinder that fails is the pathfinder's
to explain, not this module's.

## Use cases

Each is one proof in `usecases_test.go`, in the vocabulary of
`scene_test.go`: the game master places figures and lets everyone look, then
asks an actor what it means to do and where that really lands.

| # | Use case | Rule it pays for |
|---|----------|------------------|
| 1 | One situation, two targets — a zombie and a captain, byte-identical testimony, different people attacked | R3, R4 |
| 1b | The captain can be wrong — the armoured one is the bard; the chant is bundled into the robed figure anyway | R3, R10 |
| 2 | A heal in sight retargets the captain — the deed lands, the captain's Judge attaches it, its Rank puts the healer first; the zombie holds the same deed and never attaches it | R9 |
| 2b | The same heal out of sight changes nothing | R9 |
| 3 | The archer keeps its range — one number its mind returns, and a bow on its sheet | R7 rung 0, R11 |
| 3b | The archer with its back to the wall — nowhere to step, so it shoots; keeping range is a preference, not a fear | R7 rung 0 |
| 4 | The intimidated goblin — will not approach, would flee, is cornered, and shoots when the knight comes within bowshot anyway | R8, R11 |
| 5 | The ghost worth walking to — both walk to where they last saw him; the captain goes back to its post, the zombie stands there for as long as anyone ticks | R6, R7 rung 2 |
| 5b | The ghost not worth walking to — patience is the captain's, and the zombie has none | R7 |
| — | Wiring faults fail loudly | R13 |
| — | A creature of unknown place is never fled | R7 |
| — | A situation does not alias the game | R12 |
| — | A deed with no actor is refused | R9 |

The worked minds — a zombie, a captain, an archer — and the content
vocabulary they speak live in the module's tests. They read a payload
behaviour does not own, and they are the types an author copies.

## Acceptance criteria

- `go test ./...` passes from `mind/behavior`; `-race` clean; `gofmt`,
  `go vet`, and `golangci-lint` at CI's pinned version clean.
- v0.2.0: rung 0 ignoring `Keep` is killed by use case 3; rung 0 fleeing
  without asking the `Space` for a step is killed by use case 3b; `Away`
  accepting a dead end is killed by use case 4.
- Each use case kills the mutant that overclaims it: `Keep` → 0 (use case
  3); the ladder ignoring fences (4); the captain never attaching a deed
  (2); the captain never tiring (5b); `Away` accepting a dead end (4);
  `Where` reading only current holdings (5, 5b).
- Every sentinel is `errors.Is`-tested from a call that returns it.
