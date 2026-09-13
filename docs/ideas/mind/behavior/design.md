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

## Scope

A monster's mind: what it holds, what it makes of it, what it means to do,
and where that lands. Behaviour is a customer of `mind/perception`
v0.2.0 — the three asks it took there are
[#1704](https://github.com/KirkDiggler/rpg-toolkit/issues/1704) — and it
never learns anything about the world except through perception and the
values a caller hands it.

**Charter.** A mind speaks in names. It is a few judgments and no state of
its own: which holdings are one thing, what to call it, which to deal with
first, how close to let it get. The ladder is fixed and not the mind's to
change. You cannot aim at what you have not named.

**Non-goals:** persistence (names and fears would need a `Data`; the
encounter integration pays for it); authoring minds as designer data;
dice; cell-grain geometry (this module is region grain: *here*, *next
door*, *beyond*); commitment (no mind dithers yet — every worked mind ranks
by first noticed); absence as testimony (a ghost's place is never
downgraded by arriving and finding nothing; the ladder skips what is *here*
and that covers every proof); how a mind learns who frightened it (a deed
like any other, unpaid). Each arrives with the use case that pays for it.

## Rules

- **R1** — Perception is a tool. `Game.Observe` and `Game.Report` are
  perception's own doors passed through; the game holds one `Perception`
  and writes to it nowhere else. Nothing about an intent crosses back.
- **R2** — Payloads are read by the caller's `Reader`, the mirror of
  perception's `Reach`: behaviour holds no vocabulary for what a sighting
  says. It decodes exactly one channel, `deed.Channel`, which it wrote. A
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
- **R5** — You cannot aim at what you have not named. `Intent.Target` is a
  `Name`, never a subject. A contact the mind gives no word to cannot be
  attacked, walked toward, or fled.
- **R6** — `Contact.Where` is the freshest placed reading across its
  holdings, live or ghost. One rule, two callers — the ladder and `Recall`.
  A current holding's latest confirmation is its placement, so the memory
  rule already answers for a live contact.
- **R7** — The ladder is fixed, and the mind is asked `Rank` and `Keep` and
  nothing else:
  0. a live named creature is nearer than `Keep`, and there is somewhere to
     step → `Away`
  1. a live named creature is within `Sheet.Reach` → `Attack`
  2. the first ranked named contact, live or ghost, that is placed, not
     here, and not fenced → `Toward`
  3. a fenced live creature is placed and there is somewhere to step →
     `Away`
  4. nothing to act on → `Pass`

  Live beats remembered: a ghost is never attacked and never fled. Rung 2 is
  where the mind's ranking decides between a live target ahead and a ghost
  behind, and the ladder does not second-guess it.
- **R8** — A fence forbids approach and nothing else. The frightened
  condition arrives on the sheet as a subject, the ladder refuses `Toward`
  a contact holding it and flees instead, and `Attack` is untouched: a
  frightened archer with the source in reach still shoots. The mind is
  never offered a fence as a choice.
- **R9** — A deed lands on its own channel through perception's `Report`,
  under a subject qualified by channel (`deed.Subject(actor)`), one per
  figure the witness saw act. It is said in each witness's terms: actor and
  target are named only if the witness currently holds them on sight, so a
  witness who could not see the healer learns a heal happened and not who
  did it. A deed is never current. Only a mind's `Judge` attaches it to a
  figure. Nothing is written to anybody's sight holding.
- **R10** — A walk resolves against belief; a swing resolves against truth.
  `Recall` reads the situation only, so a monster searching the wrong room
  is correct behaviour and never leaks a position. `Aim` asks `Truth` which
  of a named contact's subjects is present — a chant, a deed, an illusion
  are not, so a swing at them lands on nothing and is not an error.
- **R11** — Routing is the game's, because static topology is construction
  truth: a monster may know the way through its own dungeon and may not
  know who is standing in it. `Toward` takes the first door on the way;
  `Away` takes the door that puts the most dungeon between them and refuses
  a dead end. Fleeing into a corner is not fleeing; an `Away` the stage
  refuses is an actor that stays put, which is what cornered means.
- **R12** — `Self` is handed as values: `Place` for where the actor stands,
  `Connect` for its dungeon's doors, `Sheet` for what it is armed with,
  `Frighten` for what it may not approach. Nothing in behaviour reads the
  world.
- **R13** — Errors wrap exactly one sentinel and callers dispatch with
  `errors.Is`: `ErrNoReader` from `New`; `ErrNoMind` and `ErrNoSelf` from
  `Situation` and `Turn`. An actor nobody placed fails loudly rather than
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
- `Sheet` — `{Reach int}`. Zero is melee, one is a bow.
- `Self` — `{Sheet, Where string, Adjacent []string, Fences []core.EntityID}`
  with `Fenced` and `Distance` (0 here, 1 next door, `Beyond` = 2).
- `Situation` — `{Actor core.EntityID, Contacts []Contact, Self Self, At uint64}`.
- `Verb` — sealed: `Pass`, `Attack`, `Toward`, `Away`.
- `Intent` — `{Verb Verb, Target Name}`.
- `Pair` — `{A, B core.EntityID}`, a claim that two subjects are one thing.
- `Mind` — `interface { Judge; Name; Rank; Keep }`, each `(in *XxxInput) (*XxxOutput, error)`:
  `JudgeInput{Holdings, At}` → `JudgeOutput{Same []Pair}`;
  `NameInput{Contact}` → `NameOutput{Name, Named}`;
  `RankInput{Situation}` → `RankOutput{Ranked []Contact}`;
  `KeepInput{Situation}` → `KeepOutput{Regions int}`.
- `Game` — the composition. Zero value not usable; construct via `New`.

`package deed`

- `Channel` — `perception.Channel("deeds")`.
- `Subject(actor)` — `"deeds|" + actor`, the qualified subject (R9).
- `Deed` — `{Verb string, Actor, Target core.EntityID, Where string}`,
  with `Encode` and `Decode` (`ErrNotADeed` for anything else).

`package stage`

- `Truth` — `interface { Present(subject core.EntityID) bool }`. The one
  question a swing may ask of the world.

## Verbs

| Verb | Input | Output | Semantics |
|------|-------|--------|-----------|
| `Decide` | `DecideInput{Situation, Mind}` | `(*DecideOutput{Intent}, error)` | R7. |
| `Game.Turn` | `TurnInput{Actor, At}` | `(*TurnOutput{Intent, Situation}, error)` | `Situation` then `Decide`. Both come back because the stage resolves the one against the other. |
| `Game.Situation` | `SituationInput{Actor, At}` | `(*Situation, error)` | R2–R4, R12, R13. Reads every holding, asks `Judge`, folds, names. |
| `Game.Observe` | `perception.Pass` | `error` | R1. Perception's door. |
| `Game.Report` | `perception.ReportInput` | `error` | R1. Perception's door. |
| `Game.Connect` / `Sheet` / `Mind` / `Place` / `Frighten` | one `Input` each | — | R12. Construction and sheet facts, handed as values. |
| `Game.Route` | `RouteInput{From, To}` | `(*RouteOutput{Next, Found}, error)` | R11. First step along the doors. |
| `Game.Farther` | `FartherInput{From, AwayFrom}` | `(*FartherOutput{Next, Found}, error)` | R11. Refuses a dead end. |
| `stage.Aim` | `AimInput{Truth, Situation, Intent}` | `(*AimOutput{Source}, error)` | R10. `""` is a swing at nothing, not an error. |
| `stage.Recall` | `RecallInput{Situation, Name}` | `(*RecallOutput{Where, Placed}, error)` | R10. Belief only. |
| `stage.Step` | `StepInput{Game, Situation, Intent}` | `(*StepOutput{To, Moved}, error)` | R11. One region, this turn. |
| `stage.Land` | `LandInput{Game, Deed, Witnesses, At}` | `error` | R9. Who witnessed is the caller's: a deed happens at a place, and whose senses reached it is the physics perception also leaves to the caller. |

`Game.Held(observer)` is a fixed-arity read of perception's holdings, for
the stage to say a deed in a witness's terms.

## Errors

| Sentinel | Meaning | Returned by |
|----------|---------|-------------|
| `ErrNoReader` | `New` without a `Reader` | `New` |
| `ErrNoMind` | the actor was never given a mind | `Situation`, `Turn` |
| `ErrNoSelf` | the actor was never placed | `Situation`, `Turn` |

Perception's own errors pass through `Observe`, `Report`, and `Held`
unwrapped; they are perception's vocabulary and a caller already imports it
to build a `Pass`.

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
| 4 | The intimidated goblin — will not approach, would flee, is cornered, and shoots when the knight comes within bowshot anyway | R8, R11 |
| 5 | The ghost worth walking to — both walk to where they last saw him; the captain goes back to its post, the zombie stands there for as long as anyone ticks | R6, R7 rung 2 |
| 5b | The ghost not worth walking to — patience is the captain's, and the zombie has none | R7 |
| — | Wiring faults fail loudly | R13 |

The worked minds — a zombie, a captain, an archer — and the content
vocabulary they speak live in the module's tests. They read a payload
behaviour does not own, and they are the types an author copies.

## Acceptance criteria

- `go test ./...` passes from `mind/behavior`; `-race` clean; `gofmt`,
  `go vet`, and `golangci-lint` at CI's pinned version clean.
- Each use case kills the mutant that overclaims it: `Keep` → 0 (use case
  3); the ladder ignoring fences (4); the captain never attaching a deed
  (2); the captain never tiring (5b); `Away` accepting a dead end (4);
  `Where` reading only current holdings (5, 5b).
- Every sentinel is `errors.Is`-tested from a call that returns it.
