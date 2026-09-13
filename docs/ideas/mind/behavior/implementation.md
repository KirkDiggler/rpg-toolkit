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

## Left for a later rung

- Persistence: names and fears alongside perception's `Data`. The
  encounter integration pays for it.
- The encounter's `Reader` and `Truth`, and a `Reach` that answers the
  deeds question ("whose senses reached this place") so `Land`'s caller
  does not compute witnesses by hand.
- Retiring `examples/behavior` (#1677) and the `act`/`stage` seam still on
  `examples/perception`.
- Everything under the design's non-goals, each with its use case.
