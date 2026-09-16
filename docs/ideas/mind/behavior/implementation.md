# mind/behavior implementation

The PR on branch `feat/1718-mind-behavior` implements
[issue #1718](https://github.com/KirkDiggler/rpg-toolkit/issues/1718) as a
new module, `mind/behavior` v0.1.0, on `mind/perception` v0.2.0.

No `plan.md` was written. The issue carried the API, thirteen rules, the
use cases, and a done-when; the spike it was cut from
(`examples/behavior`, #1677) had already paid for every rule with a proof.
This pair records what changed between the spike and the module, and what
the mutants said.

## What shipped

- `doc.go`, `errors.go`, `behavior.go`, `game.go` — the nouns, the mind,
  the ladder, the composition.
- `deed/deed.go` — the deeds channel, the qualified subject, the payload.
- `stage/stage.go` — `Aim`, `Recall`, `Step`, `Land`.
- `content_test.go`, `minds_test.go`, `scene_test.go`, `usecases_test.go`
  — the content vocabulary, the three worked minds, the scene, and nine
  proofs.

## What changed from the spike

The spike ran on `examples/perception`, whose store hid identity behind
per-observer track handles and held a trail of stamped entries. `mind/perception`
holds one payload per `(observer, subject)` with two counters. Five things
moved as a result, and each is a rule now rather than a workaround.

1. **Subjects are entity ids, and a noise is its own subject.** The spike's
   `projection.Handle(channel, source)` separated a chant from the figure
   chanting by construction. Perception's R11 says the caller qualifies; the
   scene does (`hearing|knight`), and `deed.Subject` does for deeds. The
   captain's wrong merge still costs it only a claim — and now the reason
   is that the truth does not know a subject called `hearing|knight`.
2. **Content is the caller's.** The spike's `content` package was shared
   with perception. Here it lives in the tests, and behaviour reads every
   payload but its own through `Reader` (R2). The mirror of perception's
   `Reach`: physics there, vocabulary here, both owned by the caller.
3. **Belief is behaviour's.** The spike's `Judge` was perception's
   reconciler and its names landed as perception claims. Here `Judge`
   returns `Same` pairs the game unions (R3), and names persist in the game
   on the contact's bearer (R4). Perception holds testimony and nothing it
   concludes.
4. **Where rides in the payload.** The spike had a `Locus` per entry. The
   encounter already encodes position into its sight payload, so `Reading`
   carries it and `Contact.Where` is a fold over readings (R6).
5. **The deed's witnesses are the caller's.** The spike's `Witnesses` read
   the projection's senses. A deed happens at a place, and whose senses
   reach a place is exactly the physics perception refuses to own, so
   `Land` takes the list (R9).

## What the mutants said

Seven mutants, applied to the working tree and reverted, one per claim.

| Mutant | Killed by |
|--------|-----------|
| `Keep` → 0: the archer stands in the corridor and shoots point-blank | use case 3 |
| the ladder ignores fences | use case 4 |
| the captain never attaches a deed | use case 2 |
| the captain never tires | use case 5b |
| `Away` accepts a dead end (a first version left a variable unused and never compiled; the compilable one is the record) | use case 4 |
| `Where` reads only current holdings, so a ghost is unplaced | use cases 5 and 5b |
| `Aim` resolves through any present subject rather than the name's bearer | **survived** |

The survivor is a lesson the module learned by losing it. The spike's
first rule — *a name is on one track, and a swing goes there* — existed
because resolving through any track sent a swing at a chant, and the chant
bound to the knight. With qualified subjects the chant binds to nothing,
so the bearer no longer decides where a swing lands. `Aim` was simplified
to the first subject of the contact the truth knows, and the bearer stayed
as what a name persists on (R4). The case that would pay for aiming
through the bearer is a mind that believes two live figures are one
person. No use case has, and the design says so.

## What the new store cost, and what it did not

- **A tick has no inside.** The spike's `Stamp{Seq, Tick}` ordered testimony
  landed in the same turn. With one counter, three figures first seen in
  one tick are tied on `FirstObserved`, and the worked minds break the tie
  by subject. Use case 2's zombie swung at the cleric instead of the knight
  until the knight was seen a tick earlier — which is what the story said
  anyway. The trap is recorded on #1704's shelf: `Stamp` returns with
  multi-channel, where two channels on one subject in one tick have to say
  which landed last.
- **No trail.** Nothing here reads a previous payload or a previous place.
  Patience reads `Confirmed`, first-seen reads `Observed`, freshest-placed
  reads `Confirmed` and the reading. A bounded trail is on the same shelf.
- **Report was needed and only Report.** A deed through `Observe` would be
  current for one pass, and a deed is not something you currently perceive.
  Perception's v0.2.0 door lands it held and sustaining nothing, which is
  what "never current" means, and `Land` is a loop over witnesses and
  nothing else.

## What the independent review changed

Reviewed at `5fcfa3d1` by a session that did not implement the change,
with every gate and two mutants reproduced there. Every finding was
verified against the code before it was answered; each thread on the PR
records the reasoning.

- **Accepted, fixed.** `Self.Adjacent` and `Self.Fences` aliased the game's
  own slices; a consumer sorting a situation would have rewritten the
  dungeon. Cloned, and R12 now says a situation is copied out. Rung 0 fled
  a creature of unknown place when `Keep` exceeded the grain's ceiling;
  every rung now skips the unplaced, as rungs 2 and 3 already did. `Deed`'s
  doc described the per-witness payload and not `Land`'s input, and an
  empty input actor would have collided every actorless deed on one
  subject; the doc says both readings and `Land` refuses an empty actor.
  Each has a proof.
- **Accepted, recorded.** The R9 claim that a witness learns "a heal
  happened and not who did it" held only against payload-readers: the
  subject `deeds|cleric` carries the actor because attachment needs it. R9
  now says which question the payload answers and which the subject does.
  Last deed wins per actor, and a merged contact wears the first name in
  subject order — both are consequences of the chosen shapes, stated in the
  deed doc and R4, and left for the use case that pays to change them.
- **Declined.** Nothing. Every finding was correct as stated; the only
  judgment was fix versus record, and the line was whether a use case
  already paid for the mechanism.

## v0.2.0 — geometry is the caller's Space (#1723)

The arrow was written down first: a monster on a real board that shoots
while you are far and switches to melee when you close. The encounter is
cells with line of sight and a pathfinder; v0.1.0 kept rooms and doors
inside the module. The ladder did not care which, but `Self.Distance` and
the game's routing did, and they would have been rewritten per grain.

So the rooms left. `Space` is the third caller-owned seam: `Distance`,
`Toward`, `Away`. `Self.Adjacent`, `Beyond`, `Game.Connect`, `Route`, and
`Farther` are gone; `KeepOutput.Regions` became `Steps` because the unit is
the Space's. The rooms-and-doors geometry moved into the tests as the
proofs' `Space`, and every proof passes with the same assertions.

One rung changed meaning on the way. Rung 0 used to fire when there was
*anywhere* to step; it now asks the Space for a step away from that
creature and fires only if one exists. An archer with its back to the wall
stands and shoots instead of spending the turn on a flee the stage would
refuse — and that claim survived its mutant until use case 3b (a room
with no doors) was written to pay for it. Rung 3 deliberately did not
follow: fear is not a preference, so a
cornered creature still means to flee and the stage finds it nowhere. The
word "region" left the module's docs; the dungeon builder owns regions.

### What the independent review of v0.2.0 changed

Reviewed at `74d98bff`, gates and all three mutants reproduced. Three
findings, all record-keeping, all taken: issue #1723's rung-3 sentence
said the Space is asked at rung 3 and the code deliberately does not —
the issue was corrected, the code is the party that was right; a stale
"once it tags" bullet and a stale perception version survived the
`Qualify` commit and were removed; R13 now says a `Reader`'s and a
`Space`'s errors pass through as the caller's vocabulary. Declined:
nothing. The arrow was also corrected on Kirk's word in the same commit:
the mind that changes is the skeleton's, not the weapon.

## v0.3.0 — the store is the caller's (#1725, PR 1 of the adoption)

The encounter already owns a `perception.Perception` and runs its passes;
v0.2.0's `Game` built a second one. Two stores would be a lie about who
holds what, so the game stopped holding one. `Turn` and `Situation` take
`Holdings` as values; `Observe`, `Report`, and `Held` left the game;
`stage.Land` takes a `Store` — exactly perception's two methods — so the
caller hands its own store and nothing wraps it. The proofs' scene now
owns the store and runs the passes, the way a real board does, and every
prior proof passes with the same assertions. R1 says it: behaviour never
runs a pass. The adoption's own rules are in [adoption.md](adoption.md).

One rule was amended on the way, found by the encounter's first deed
proof. An observer never perceives itself, so a witness holds no sight of
itself — and `Land` named actor and target only from what the witness held
on sight, so a deed done TO the witness named nobody. The bow skeleton
could never have believed it was shot at, and the adoption's Retaliator
ranks on exactly that. R9 now says a witness knows itself: `Land` names
the witness whenever it is the actor or the target. The proof is
`TestAWitnessKnowsItWasTheTarget`, and removing the shortcut in `seen`
fails it on "knows the knight attacked IT".

## v0.3.1 — a bearer is never a deed (#1732)

Found by the independent review of the adoption's driver (#1729, finding
1), which had already worked around it: the rulebook's `memberID` matched a
seen member on the contact's plain subject and never on `Bearer`, because
`Bearer` could not be trusted to be a figure.

The defect was one rung missing from `bearer`. It took the current creature
holding, else `Holdings[0]`. Holdings sort by subject and a deeds handle is
`deeds|<id>` (R11), which sorts before most plain ids — so a contact first
folded as ghost-plus-deed, the shooter who stepped out of sight after the
shot, recorded its name on the deed. Nothing recovered from that: the
handle is never present on any truth, so every later situation found the
word under it, set `Bearer` to it, and `stage.Aim` landed the swing on
nothing for as long as that actor lived.

`bearer` now prefers a current creature holding, else the first holding
that is not a deeds handle, else the first. The last rung is the real
answer for a contact of deeds alone, not a fallback that fires by accident,
and it has its own proof. The test is the subject's shape rather than the
holding's `Channel`: `Channel` is the provenance of the latest accepted
testimony and moves with it, while what makes a handle unreachable is that
it is qualified. perception parses no qualified id back apart and says it
will not until a caller needs the entity out of one, so the prefix is built
with its own `Qualify` and the separator stays perception's.

The proofs are `TestANameIsNeverBorneByADeed` — the captain is shot at,
loses sight of zara before it ever takes a situation, and still names the
figure and not the shot — and `TestAContactOfDeedsAloneBearsItsDeed`. The
first also counts how often the mind is asked for a word, which is the only
way from outside to see which subject the name was filed under.

| Mutant | Killed by |
|--------|-----------|
| the middle rung removed, so `bearer` falls straight to `Holdings[0]` | `TestANameIsNeverBorneByADeed`, on both halves: `Bearer` was `deeds|zara` where `zara` was expected, and the second situation read the name back from under `deeds|zara` |

#1729's `memberID` may now match on `Bearer` again. Whether it does is the
driver's call and not this change's: the rulebook is a different module,
and nothing there is wrong today.

## session v0.85.0 — a driver per session (#1734)

Found by the independent review of #1731 (finding 1) and by rpg-api#980's
own workaround. `Config.TurnDriver` was read once at `NewManager` and the
one Manager served every session in the process, so a host could not give
a session its own `Minded`. rpg-api built one process-wide driver behind
a mutex and wrote the caveat down beside it: member ids are authored per
dungeon rather than minted per run, so two parties in the same tomb
shared a skeleton's assigned mind and names. Harmless while every shipped
mind names contacts off the view it is handed; not harmless the first
time a mind carries memory, which is the next slice.

**The ruling (Kirk, 2026-09-13).** Ownership first. A session's lifetime
is the host's — a Redis TTL, a run ending — and the Manager is
deliberately stateless per verb with no session-end signal, so a
per-session driver cache inside it would have no owner for eviction and
does not go there. `Config` grows `TurnDrivers`, a per-session resolver
asked once per verb for the session it is about; `TurnDriver` stays for a
stateless driver serving every session; exactly one of the two is set.
Inside the Manager the resolved driver rides the verb's scope, and
resolution happens in two places rather than eleven. Two write verbs on
ONE session already race that session's own scope; a per-session driver
inherits that boundary and does not change it.

**The shape, and why.** `TurnDrivers` is an interface —
`TurnDriverSource.DriverFor(ctx, sessionID) (TurnDriver, error)` — rather
than a func type, because every other capability on `Config` is one
(`Roller`, `PresentationIDGenerator`, the three repositories) and a host
implements them side by side. It takes the verb's context for the reason
the named payer gives: minds as authored data is a driver built from
per-session material a host may have to go and fetch, and adding the
parameter afterwards would break every host that had implemented the
interface — the exact asymmetry `Config`'s own doc warns about for
required fields.

The Manager now holds **no driver at all**, only a source. A host that
wired the stateless field gets a `staticTurnDrivers` built around it at
`NewManager`, so "which driver serves this verb" has one answer and no
branch. `openForWrite` resolves once and puts the answer on `writeScope`;
`loadWorld` resolves beside the read path's world. The eleven sites that
read `m.turnDriver` read the scope's driver, and `compelledDriver.next`
became a method reading `d.scope.driver` rather than a field copied from
it — a field would have been a second name for one value, and the only
question anybody asks there is whether it is the same brain the rest of
the verb is using.

`loadAuthored` is the one site with no session to name, and it takes a
new `refusingTurnDriver` — the fourth refusing stand-in at that same call
site. Its neighbours already said no clock advances on an authored world;
now the turn driver says it too. `StartSession` proves a world loads
*before* the session exists, and `AtlasOf` previews a world nobody has
started, so neither has a session id to resolve against and asking the
host would have it mint a driver for a session that may never exist.

It wraps `ErrInvalidWorld` rather than carrying a sentinel of its own,
where the pattern it cites — `encounter.RefusingStriker` — has
`ErrRefusingStriker`. Raised by the review and kept as written: the two
stand-ins beside it at the same call site, `refusingCheckResolver` and
`refusingWitness`, both wrap `ErrInvalidWorld`, so the package's own
family stays coherent and a host matching on one matches all three. The
choice is recorded here so it stays visible next to the pattern it
diverges from; a dedicated sentinel becomes right when a caller needs to
tell the three apart, which none does today.

Two sentinels landed rather than one. Neither-set stays
`ErrIncompleteConfig` — it is the S8 by-name refusal every required
capability gets — with the row renamed to name **both** doors, since
either satisfies it and a host that meant to wire the source is not
helped by an error naming only the other field. Both-set is
`ErrAmbiguousConfig`: a different fault with a different fix (delete a
line, not add one), and refused rather than resolved by precedence
because picking either silently would leave a host watching the driver it
did not mean to wire take every turn in the process. `ErrNoTurnDriver` is
the third: a source that reports success and hands over nothing has
broken its contract, and the nil is refused where it happens rather than
wrapped into a seam that panics mid-turn.

**Reads resolve too**, which is a deliberate cost. A read advances no
clock and never consults the driver, and every other capability on that
path is a refusing stand-in — so a refusing driver there would have been
uniform and free. The ruling put the driver next to the world instead, so
that no world this package builds ever holds a brain belonging to
somebody else, and the price is that a read fails closed when the host
cannot name a session's driver. It does not widen the host's cache: reads
touch the same bounded set of sessions writes do.

**"Once per verb" is counted, not assumed.** The review found the doc's
load-bearing promise — the source is asked once per verb and that answer
serves the whole verb — held up by nothing but the current shape of
`openForWrite`. The fake source now counts asks per session id beside the
drivers it built, and the two proofs assert exactly one ask per verb: per
`EndTurn` on the write path, per `Roster` on the read path, on the
succeeding read as well as the two refused ones. The counter is the point
rather than the map: `built` is keyed by session, so a verb that resolved
twice landed in the same entry and looked identical to one that resolved
once. It matters for the host the doc itself names — one that mints per
ask, from per-session material fetched fresh — where a second ask is a
second driver rather than a wasted map lookup.

| Mutant | Killed by |
|--------|-----------|
| the host source returns the same driver for every session id | `TestEachSessionDrivesItsOwnTurnsWithItsOwnDriver`, on "the host was asked about two sessions and built two drivers" — one entry, not two |
| a second resolution inside one write verb, at the end of `openForWrite` | the same test, on "one write verb, one ask": 9 asks where 8 were expected |
| a second resolution inside one read verb, at the top of `loadWorld` | `TestAResolverErrorFailsTheVerbAndDrivesNoTurn`, on "one read verb, one ask": 8 asks where 7 were expected |
| `openForWrite` resolves a constant session id, so every write uses one session's driver | the same test, on "so did the second's, for the member of the same name": the second session's driver never took a turn |
| `compelledDriver.next()` returns `Behavior()` instead of the scope's driver | the same test, on "the first session's driver took its own skeleton's turn": the host's driver is never reached for an uncompelled member |
| `resolveTurnDriver` falls back to `Behavior()` on a source error | `TestAResolverErrorFailsTheVerbAndDrivesNoTurn`: the host's error never reaches the host |
| the both-set refusal removed from `NewManager` | `TestExactlyOneTurnDriverIsWired/both` |

## dnd5e/behavior v0.5.0 — the Retaliator profile (#1745)

**UNMERGED as of writing.** The version in the heading is the one this PR's
merge will mint, not a tag that exists; the wave publishes inside-out,
#1746 → #1748 → #1750 → rpg-api#984. This record is written before the
merge so the reasoning is reviewable beside the code.

**What the walk staged, 2026-09-14** (full record on
[#1748](https://github.com/KirkDiggler/rpg-toolkit/pull/1748)). The coward
goblin was seen on the board stepping away from a player who walked up to
it. The berserker thug's chase was seen; its grudge held PAST A NEARER
PLAYER was not staged, and rests on
`TestTheBerserkerDoesNotCareWhatTheShooterIsHolding` and that test's killed
mutant. The retaliator skeleton was not re-walked, its only change being the
patience spelling. Kirk's call was that the tests carry the two unstaged
halves. The finding that came out of it: staging a mind's moment on a board
— a shooter placed, a weapon swapped, a nearer body standing by — is hard
enough that it wants a tool, filed as a follow-up rather than built here.

**The ruling (Kirk, #1745).** The Retaliator is really a grudge system, and
what sets it off and how long it holds are things to configure. What the
monster DOES stays apart from WHERE it is configured, so this slice tunes
the mind in code and learns which fields a mind has before any
authored-data format is chosen.

### The shape, and why

`Grudge{Patience, Excuse}` is what a mind does about being attacked, and
`Room` is how much space it wants. Both are plain fields on the
`Retaliator`, so a preset is a literal and the whole profile is readable in
one line.

- **Patience is a SPAN**, not a maximum age: the number of ticks a deed
  stays worth answering, counting from the tick it was confirmed. That
  reading is what makes `Grudge{}` honest. Read as a maximum age instead, a
  patience of zero still answers a deed of age zero — so the mind with no
  grudge would hold one for exactly the tick anybody was looking, which is
  the zero value telling a lie about itself. The retaliator's number moved
  from 2 to 3 for the same arithmetic: `age > 2` and `age < 3` cover the
  same three ticks, and #1725's walked behaviour is unchanged.
- **`ExcuseNever` is the zero Excuse.** An Excuse is a rule for letting an
  actor GO, so an author who set a grudge and named no excuse wrote down no
  way out of it. The alternative zero, `ExcuseUnarmed`, would have every
  profile assert a weapon rule its author never typed.
- **The field is `Room`, not `Keep`.** A mind answers `Keep` with a method,
  and a field of that name cannot sit beside it. `Room` says what it means —
  how much space it wants between itself and a live creature, in the Space's
  own steps — and its doc points at the judgment it answers.
- **`NewMindedInput.Patience` and `DefaultPatience` are gone.** How long a
  mind holds a grudge is the mind's. One number for every mind in a process
  could not have a berserker and a bow skeleton on the same board, which is
  the whole scene this slice exists to show. `Ranged` stays on the driver —
  what a bow IS is a catalog, whether the mind cares is the profile — and is
  handed to every preset.

**The profile feeds the ranking and never reorders it.** Rank's order is
fixed: a grudge, then the live ahead of the remembered, then the closest,
then the subject. What the profile decides is WHO has a grudge and how much
room the mind wants; rung 0 reads `Room` and this mind does not. That is the
line `scenarios.md` draws between a profile and a claim, and it is now in
the `Retaliator`'s own doc.

### The presets

Every number is a feel number. A tick is a fight ROUND (walk 2 of #1725).

| word | patience | excuse | room | what it is |
|---|---|---|---|---|
| `retaliator` | 3 | unarmed | 0 | #1725's bow skeleton, unchanged |
| `berserker` | 10 | never | 0 | longer than any fight here: within one fight it never forgets |
| `coward` | 0 (none) | — | 2 | no grudge at all; backs away from anything adjacent |

The berserker's 10 is a number rather than an infinity because "never
forgets" is a claim no walk has paid for, and a span is the shape the field
already has. The coward's 2 is the smallest room that means anything: one
step would name only a creature sharing its cell, which is nobody.

### The proofs

One scene, three words, and the body never changes — the member is called
`skeleton` throughout, because what is under test is the word on its sheet.
Alice shot it with a crossbow two ticks ago, has since drawn a longsword and
closed to three cells; bob has stood two cells off and attacked nobody; the
bow reaches both and the blade reaches neither. Three derivations change one
thing each: the crossbow back in alice's hands, bob at touching distance,
the shot landing on the tick being answered.

| | sword seen | crossbow still up | bob in its face |
|---|---|---|---|
| retaliator | bob, bow | alice, bow | bob, melee |
| berserker | alice, bow | alice, bow | alice, bow (past the nearer man) |
| coward | bob, bow | bob, bow | one step back from bob |

The middle column is the control for the grudge itself rather than the
excuse: with a shooter nothing could excuse, a mind that holds a grudge and
a mind that holds none finally choose different people.

### What the mutants said

Five mutants, applied to the working tree and reverted.

| Mutant | Killed by |
|--------|-----------|
| the retaliator's excuse swapped to `ExcuseNever` | `TestTheRetaliatorLetsAnUnarmedShooterGo/the sword is seen` (alice where bob was expected), and `TestTheGrudgeEndsWhenTheBowIsPutAway` and `TestHandsItDidNotSeeAreNotABow` from #1725 |
| the berserker's patience set to 0 | `TestTheBerserkerDoesNotCareWhatTheShooterIsHolding`, all three scenes: bob every time |
| the coward's room set to 0 | `TestTheCowardHoldsNoGrudgeAndKeepsItsRoom/somebody is in its face`: it swung at bob instead of stepping back |
| the berserker and coward presets swapped | `TestAZeroGrudgeRefusesTheShotThatJustLanded/the coward` and the berserker's own three scenes |
| `Grudge{}` holds a grudge (`fresh` reads the span as a maximum age) | `TestTheZeroGrudgeIsNoGrudge` on "not even the deed that landed this tick", `TestTheZeroGrudgeRanksNobodyFirst/the zero grudge`, and `TestAZeroGrudgeRefusesTheShotThatJustLanded/the coward` |

Each of the first four is also killed by
`TestEachWordsPresetIsWhatItsDocSays`, the table over `mindFor` — the only
test written from inside the package, because a preset is not reachable
from a driver's answer except through a scene built to tell one number from
another. It is what catches a doc that drifted from its table.

### The lesson

Every tune is a new word and a toolkit release. A berserker is not a new
type, a new judgment, or a new rung — it is `Excuse: ExcuseNever` and a
bigger number — and yet shipping it cost a constant in two modules, a preset
in a third, a pin in each consumer and four PRs. That is the cost authored
data would remove, and it is being paid deliberately a few times first: the
fields a profile has are now known (patience, excuse, room), which is
exactly what had to be true before anybody chose a format to author them in.

## mind/behavior — rung 0 asks about each creature (rpg-project#454)

Keep was asked **once per turn**, before rung 0's loop, and that one number
was compared against every contact. A mind that keeps its distance from one
figure and none from another was inexpressible — which is what a frightened
monster is.

`KeepInput` gained a `Contact`, and `Decide` asks inside rung 0's own loop
about the creature it is measuring. Nothing else moved: the rungs, their
order, and what each one skips are what they were, and a mind with one
answer for everybody behaves identically. Every `Keep` in the repo ignored
its input before this and still does; every existing proof passed unchanged.

It is its own PR because `mind/behavior` is its own Go module.

| Mutant | Killed by |
|--------|-----------|
| the single per-turn `Keep` call put back | `TestRungZeroAsksTheMindAboutEachCreature/it steps away from the one it fears` — Attack (`0x1`) where Away (`0x3`) was expected: the fearful monster swings at what it meant to run from |

## dnd5e/behavior v0.6.0 — Fear and the provocation set (rpg-project#454)

The first shenanigan's mind half. The encounter lands an `intimidate` deed
on the witnesses; this is the three words deciding what it is worth.

- **`Fear{Patience}`** on the preset beside `Grudge`, coward `3`. While a
  threat is fresh, `Keep` answers `math.MaxInt` for the one who made it and
  `Room` for everybody else. The zero `Fear{}` is never cowed, which is
  every other word.
- **`Grudge.Provokes`** replaces the hardcoded `Verb == DeedAttack` in
  `grudge`. Retaliator `{attack}`; berserker `{attack, intimidate}`; coward
  none, because it reads the threat through `Fear` instead. Empty provokes
  nobody, so the zero `Grudge` is still honest.
- **`Judge` reads no verb at all** any more. That was not tidying: a
  coward's grudge is the zero, so a verb-filtered Judge would leave the
  threat unattached — a deeds handle floating beside the fighter rather than
  on her — and `Keep` would be asked about a contact holding no memory of
  the one thing that happened to it. Whether two holdings are one figure is
  a claim about perception and is true whatever she did; what the deed is
  worth is the profile's, and now says so in one place.

### Two things the design said and the code did not

**The sight range is not on the view.** The design says a cowed coward's
`Keep` answers "my sight range". `encounter.MonsterView` carries no sight
range — `SightFeet` is a member fact the view does not project — so the
answer is `math.MaxInt`, documented as "no distance is far enough while I
can see her". The outcome is the design's and arguably better at the edge:
rung 0 only ever considers a CREATURE, and a creature is a figure some
current holding reads as one, so the moment she is a memory the question is
never asked about her again. "As far as it can see the fighter" is the
ladder's own doing, not a number anybody computed.

**The provocation set is not shared between `Judge` and `grudge`.** The
plan was to replace the verb test in both with one set. Doing that would
have broken fear, for the reason above.

### What the mutants said

Five mutants, applied to the working tree and reverted.

| Mutant | Killed by |
|--------|-----------|
| `Keep` ignores the intimidate deed | `TestACowedCowardRunsInsteadOfShooting` (Attack at alice where Move away was expected), plus `TestAMindWithNoFearIsNeverCowed` and `TestFearWearsOff/still working on the last tick` |
| the berserker's `Provokes` loses `intimidate` | `TestTheBerserkerChargesWhoeverThreatenedIt`: bob with a sword, where alice across the room was expected |
| the retaliator's `Provokes` gains `intimidate` | `TestTheRetaliatorIsUnmovedByAThreat`: alice where bob was expected, both against the literal and against the same scene with no deed in it |
| `Judge` filters deeds by verb again | `TestACowedCowardRunsInsteadOfShooting`, `TestFearWearsOff/still working on the last tick`, and `TestJudgeAttachesAThreatToTheFigureWhoMadeIt` |
| `Fear.fresh` never expires | `TestFearWearsOff/spent on the next`: still fleeing three ticks on |

Two existing tests failed the moment `Provokes` landed and were corrected
rather than bumped: `handGrudge` and the preset table both had to start
naming the verb they answer. That is the fail-closed zero doing its job —
an omission that used to be invisible is now a fixture that provokes
nobody.

## Left for a later rung

- Persistence: names and fears. The encounter integration pays for it.
- The encounter's `Reader`, `Truth`, and `Space` (its canvas and
  pathfinder), and a `Reach` that answers the deeds question ("whose senses
  reached this place") so `Land`'s caller does not compute witnesses by
  hand.
- Retiring `examples/behavior` (#1677) and the `act`/`stage` seam still on
  `examples/perception`.
- Everything under the design's non-goals, each with its use case.
