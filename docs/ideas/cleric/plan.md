# Cleric level-one contribution plan

## Current direction: expand level-one play, preparation deferred

The user confirmed the implemented Cleric slice, including the private-sheet
fix, has been deployed and tested end to end. This is user-reported acceptance;
it does not imply every domain, advertised cantrip, or first-level spell works.
PR #1721 merged as `9ec6aae8` and its published root tag is `v0.165.1`.

Continue filling out what a level-one Cleric can do, including cantrips and
first-level spells. Keep preparation deferred: use the existing supported-spell
acquisition policy without adding a preparation picker, rest-time selection,
or automatic backfills. Do not introduce generic infrastructure speculatively;
let the next concrete spell or domain feature expose the missing capability.

The current supported first-level acquisition pool is Bane, Bless, Command,
Cure Wounds and Healing Word. Spare the Dying and Guidance are executable
cantrips too (see their own sections below) — Guidance only on `Unlock`,
Search's find checks deliberately excluded, recorded in Guidance's own
section. Acquisition lists and descriptive spell data are not proof of
executable behavior on their own: Light, Resistance, Toll the Dead, Word of
Radiance and Thaumaturgy remain offered as cantrip choices with no execution
support yet, and each needs checking separately from its catalog entry.

Next-slice inspection candidates include those remaining cantrips and spells
such as Shield of Faith, Guiding Bolt and Inflict Wounds. This is an inspection
queue, not an approved implementation order or a claim that their contracts
already fit. For each candidate, verify the edition-specific rules and current
main, then record existing reusable behavior, the actual missing provider work,
and whether any input/result cannot cross the existing API/proto contract.
Prefer a bounded playable contribution; discuss new gameplay/default decisions
with the user before implementing them. Keep domain acquisition/grants as
explicit slices rather than silently expanding a spell PR.

Each slice must cover native acquisition, private projection, offers/execution,
authoritative spending/results, persistence and relevant rest/concentration
cleanup. Publish one module PR at a time, then consume the real release. API
adoption may require only a pin; proto changes require an identified wire-shape
gap. Track toolkit, API and browser evidence independently using the checklist
below. Preparation is not a prerequisite for continuing this work.

## Guidance: post-roll check-offer plan

The user chose Guidance as the next inspection candidate from the current
cantrip list and picked the RAW-accurate shape over a cheaper approximation:
the recipient decides whether to spend the die *after* seeing their own roll,
matching 2014 Basic Rules text supplied by the user: "You touch one willing
creature. Once before the spell ends, the target can roll a d4 and add the
number rolled to one ability check of its choice. It can roll the die before
or after making the ability check. The spell then ends." One action, touch,
V/S only, concentration up to one minute, no ritual tag. Taking the die ends
the spell immediately, not just on the duration running out.

### What already exists and what does not

Bardic Inspiration (`features/bardic_inspiration.go`, `conditions/inspired.go`)
is the die-holder template, not the hard part: a condition subscribes to an
offer chain, appends an `Offer`, and self-removes when `OfferTakenTopic` names
it. `GuidanceCondition` mirrors this almost directly, with two differences —
it is concentration-bound rather than "until combat ends or rest" (so it needs
no combat-end/rest subscription of its own; the concentration teardown Bless
and Bane already use removes it), and RAW's "one ability check of its choice"
means it offers on whichever check the recipient later makes, not a
skill-filtered one.

The actual new work is the roll side. `events/offer.go`'s
`PostRollOfferEvent`/`PostRollOfferChain` is the only place in the toolkit
that implements "ask after the roll, before the outcome, resume from bytes
later" — folded today only on attack rolls. Its own doc names this gap
directly: "Folded on an ATTACK roll only today. Saving throws and ability
checks are the same shape and are deliberately not folded yet — they arrive
with the slice that asks for them." This is that slice, for checks only —
saving throws are a separate future slice, since `saveMachine` is only ever
driven as a `Request` sub-machine from inside a casting machine
(`contest.go`), and `Request` explicitly refuses a sub-machine that poses
("a requester cannot be suspended"). That wall is out of scope here.

`resolution/strike.go` + `strike_pose.go` is the only working example of the
full roll → offer → `Pose` → `Frozen` → `Resume` shape, and is the template to
copy for checks — not because checks currently resemble Strike, but because
nothing else in the codebase does this yet. Reuse `OfferAnswer`/`OfferSpend`/
`OfferKeep` from `strike_pose.go` rather than redefining them.

`checks.MakeAbilityCheck` (`checks/checks.go`) has no `RollCalculation` today,
unlike `saves.MakeSavingThrow` — but checked how Strike actually builds
`StrikeOutcome.Calculation` (`resolution/strike.go:385`) and it is built
entirely inside `resolution`, from the roll/modifier data the lower-level
attack code already returns, not inside the attack rules package itself. The
check equivalent follows the same layering: `resolution` (Slice 2) builds its
own sourced `RollCalculation` locally from `checks.AbilityCheckResult`'s
existing `Roll`/`Total`/`BonusSources`, so the offered die can append as one
more sourced component. `checks.AbilityCheckResult` itself does not change.

`resolution.MakeCheck` is already the right shape to build on: no `World`, no
`Initiative`, no combat dependency — its own doc already names this as the
anticipated first consumer of a check's write-back ("the day a condition
spends itself on a check (guidance is the canonical one), the write-back is
already in the caller's hands instead of silently lost down here").

**Ruled out**: routing checks through `resolution.Resolve`/`Machine`. Checked
its `Input` — it requires `World`, `Initiative` (required), `Standing`
(required) and a full `Participants` list, the heavyweight full-encounter
apparatus `MakeCheck` was deliberately built without ("there is no world
here, and that is v1's honest answer"). Not needed anyway: every check already
crosses to resolution through exactly one session-level function.

**The one true seam**: `session.Manager.resolveStagedCheck`
(`session/conceal.go`), whose own doc says so directly — "THE ONE PLACE A
CHECK CROSSES TO RESOLUTION at this seam... Search (through checkSeam) and
Unlock's lock checks both land here." Confirmed by grep across the whole
rulebook: `.ResolveCheck(` has exactly one real caller anywhere
(`encounter/search.go`), and nothing inside Attack/Cast/Activate/Move
resolution currently invokes a check at all — `CheckResolver` is wired into
every `resolution.Input` but unexercised today. So fixing
`resolveStagedCheck` once covers every live consumer; `OpenDoor` has no check
at all and is unaffected.

Neither `Search` nor `Unlock` touch action economy or turn state — confirmed
by grep, no hits. They already work identically in or out of combat, and so
does `resolution.MakeCheck`. There is no separate combat-mode/exploration-mode
implementation to build — one Pose-capable `resolveStagedCheck` serves both.
Combat state matters only for *casting* Guidance: `session.Cast` currently
refuses world-clock casts at all, a pre-existing, separately tracked
limitation, so Guidance can only be cast during a turn today. The held die
should still be usable on a later check outside combat, since no
combat-end subscription was found anywhere in the concentration mechanism —
to be confirmed with a test, not assumed.

### Slices

1. **Root, not yet castable.** `PostCheckRollOfferEvent`/
   `PostCheckRollOfferChain` in `events/offer.go`, sibling to the attack one
   (`CheckerID` in place of `AttackerID`). `GuidanceCondition` in
   `conditions/`, mirroring `InspiredCondition` per above. `checks` package is
   untouched — its sourced calculation is built in `resolution`, not here (see
   above). **Not added to `castContent`** — stays selectable-but-inert exactly
   where it sits today, matching how Spare the Dying's provider PR shipped
   before its content was enabled.
2. **Resolution + session: the real suspend/resume work.** Build a sourced
   `RollCalculation` for the check locally in `resolution`, the way
   `strike.go` does for `StrikeOutcome`, then extend `resolveStagedCheck` to
   fold the new offer chain after `MakeCheck` returns; on a validated single
   offer for the checker, freeze enough state (checker, applied approach, DC,
   roll, total, calculation, offer, plus whatever `Search`/`Unlock` need to
   finish) and report posed instead of a finished result; unchanged
   otherwise. Session gains a new interrupt window kind
   (sibling to `windowKindReaction`) and a resume path that thaws, applies
   spend/keep (rolling the die and appending it to the calculation on spend,
   as `strikeMachine.spendOffer` does), then runs the one genuinely per-verb
   piece — finish whichever verb was in flight (Search: reveal findings;
   Unlock: apply the lock verdict). Regression guard: existing Search/Unlock
   suites must pass unchanged for the no-offer path.
3. **Root: enable it.** Add Guidance to `castContent`. Acceptance: a
   finalized Cleric casts Guidance on an ally or self during combat, the
   recipient later makes a Search or Unlock check (in or out of combat), gets
   posed, spends or keeps, reload preserves an open window mid-decision, and
   concentration end / the one-minute duration / taking the die each
   correctly end the condition.

Deferred, explicitly out of scope here: propagating a check-triggered `Pose`
out of Attack/Cast/Activate/Move resolution (nothing calls a check from
inside those today, so there is nothing live to fix); world-clock casting of
Guidance itself (existing `session.Cast` limitation, separate slice); saving
throws gaining the same offer treatment (blocked on `Request`, separate
slice, Resistance's future work). Protos/API are expected to be adoption-only
— the ask/answer shape (`VERB_REACT`/`ReactionRef`) and the outcome shape
(`RollComponent`/`RollCalculation`) both already cross generically — but that
is to be confirmed once inside that code, not asserted here.

Sources: [2014 Guidance](https://www.dndbeyond.com/spells/2149-guidance),
user-supplied rules text above.

### Slice 1 delivered: chain plumbing and the condition, not yet castable

`PostCheckRollOfferEvent`/`PostCheckRollOfferChain` added to `events/offer.go`,
sibling to the attack pair. `GuidedCondition` added to `conditions/`
(`refs.Conditions.Guided()`, spell source ref `refs.Spells.Guidance()`,
already present from the existing cantrip-choice catalog entry). It mirrors
`InspiredCondition`'s offer/take mechanism on the new check chain, filtered
on `CheckerID` with no skill filter, and mirrors `BlessedCondition`'s
long-rest cleanup subscription rather than Inspired's combat-end one —
Guidance is concentration, same duration category as Bless, not Bardic
Inspiration's ten-minutes-or-combat-end. Registered in the condition loader
and in both cross-package contract tests (`ref_contract_test.go`,
`long_rest_registry_test.go`) that keep the loader registry honest against
every loadable condition.

`checks.AbilityCheckResult` was deliberately left untouched — checked how
`StrikeOutcome.Calculation` is actually built (`resolution/strike.go:385`)
and it happens entirely inside `resolution`, not the lower-level attack
rules package; the check equivalent follows the same layering and belongs to
Slice 2.

Not wired into anything yet: nothing folds `PostCheckRollOfferChain` at a
real roll site, so the condition cannot currently be exercised end to end.
Guidance is not in `castContent`. Full root module suite, vet, and lint pass
clean; `go build`/`go vet ./...` and `golangci-lint run` reported 0 issues on
the touched packages.

### Search: deferred, with the user's authorization, and why

Slice 3 wired Guidance's interactive offer onto `Unlock` only. Extending the
same offer to Search's find checks was investigated and explicitly declined
by the user ("seems like a lot of baggage") after the actual shape of the
problem was read out of the code, not assumed. Recorded here so a later
session does not have to re-derive it.

**Search cannot honestly pose the same way Unlock does.** `encounter.Search`
calls `CheckResolver.ResolveCheck` inside a loop, once per concealed door
touching the swept region:

```go
for _, doorID := range e.world.concealedDoors {
    ...
    verdict, err := e.checkResolver.ResolveCheck(&ResolveCheckInput{...})
    ...
}
```

One `Search` call can make zero, one, or many checks. RAW's "one ability
check of its choice" has no defined answer to "which of the N checks a
single Search call happens to make" — that is not a plumbing gap, it is an
undecided rule.

**Worse, posing anything here breaks Search's own secrecy law.** The
package doc is explicit: "An empty region and a failed check return the
same bytes... even the persisted blob is identical" — a failed searcher
must not be able to tell a room was empty from a room that hid something
they missed. Stopping to ask "spend your die?" only when a check actually
runs would itself leak that a concealed door exists in the region, before
the searcher has found anything — exactly the fact this verb exists to
protect.

**The user's resolution**, once this was laid out: apply the die
automatically, not as an offer, to every check one `Search` call makes,
since the player experiences "search the room" as one action regardless of
how many concealed doors the engine happens to be checking against, and
they never learn that count either way — so uniform automatic application
leaks nothing a plain `Search` call does not already conceal.

**That still is not a small addition, which is why it stays deferred.**
The tracking half is free — an applied bonus already rides the existing
sourced `RollComponent`/`RollCalculation` shape no matter how it is
delivered, the same way Raging's advantage or a proficiency bonus already
does. The mechanism half is not: `GuidedCondition`'s existing subscription
(`PostCheckRollOfferChain`) is Unlock's interactive path, and it cannot
also subscribe to the ordinary pre-roll `AbilityCheckChain` bonus fold to
get automatic behavior for Search — that chain fires on *every* check,
Unlock's included, so the same condition auto-applying there would silently
bypass the interactive pose Slice 3 just built. A condition has no way to
know which verb triggered the roll it is being asked to join.

So automatic application for Search has to be Search's own explicit code
path, not a second subscription on the existing condition: check whether
the searcher holds Guidance before the sweep starts, apply the bonus to
every check the sweep makes (rolled once and reused, or independently per
check — undecided, and immaterial to the tracking, which is uniform
either way), and consume the condition exactly once when the sweep ends
rather than after the first internal check. That last part is the reason
this cannot live inside `resolveStagedCheck` as written: it is invoked once
per door, with no memory of "this is check 2 of 3 in the same Search call."
It also still needs `encounter.Search`'s own calling contract to change —
the same `encounter` + `session` PR pair Slice 3 already named — now to
carry an extra caller-supplied bonus rather than to carry a pose.

Deferred, not ruled impossible: a future session can pick this up from the
mechanism description above without re-deriving the secrecy problem.

### Slice 4 delivered: Guidance is castable

Root-only, no consumer pins: enabling content never crosses a dependency
edge session/resolution don't already own. Root PR #1754 published
`rulebooks/dnd5e v0.170.0`, resolution PR #1755 published
`rulebooks/dnd5e/resolution v0.49.0`, session PR #1756 published
`rulebooks/dnd5e/session v0.88.0` — all three merged before this slice, in
that order, each repinned to the prior's real tag before its own merge.

Guidance added to `castContent`: a level-0 action cantrip, touch range,
`CastTargetTouch` (self included, Cure Wounds's own precedent for why
`CastTargetSelf` cannot stand for this), concentration `TurnEnds: 10,
SkipFirstTurnEnd: true` — "up to one minute" in this rulebook's own
turn-end vocabulary, the same shape Bless already uses. Delivers
`GuidedCondition` through the existing no-save condition-delivery path via
`CastEffect{Ref: *refs.Conditions.Guided(), CounterpartKey: "source_id"}`.

`GuidedCondition` needed a construction path for that delivery it did not
have: `resolution/contest.go`'s `prepareCondition` special-cases Blessed
and Baned (both need a parsed `*core.Ref` for their own strict
canonical-source validation) and falls back to `conditions.CreateFromRef`
— the root module's own generic factory — for everything else. Guidance's
constructor carries the same strict validation Blessed's does, so rather
than add a third resolution-side special case (a resolution PR this slice
does not need), `conditions/factory.go` gained a `createGuided` that
parses the caller-supplied source-ref string itself before calling
`NewGuidedCondition` — root-only, mirroring how `createTrueStrike` and
`createCommanded` already parse their own config, just with the extra
parse step Blessed's shape requires.

Two existing tests asserted the PRE-Guidance world and needed updating to
the new true state, not fixes to a defect: `spells.Castable`'s Cleric
subset test now includes Guidance alongside Sacred Flame and Spare the
Dying, and a Cleric-finalize reload test that iterated every executable
known cantrip and asserted every one carries a save DC — true before this
slice, false now that Guidance is executable and carries none. Both
updated to assert the actual (correct) new behavior rather than loosened
to stop failing.

Acceptance covers: `CastDefinition` shape (target/range/concentration/
effect/counterpart-key, clone independence, JSON round-trip), the factory
building `GuidedCondition` from its counterpart key and refusing a wrong
source spell, and the full existing root/resolution/session suites passing
unchanged elsewhere. Full root module build/vet/test/lint clean.

Guidance is now castable end to end on `Unlock`: a finalized Cleric casts
it on an ally or self during combat (world-clock casting remains a
separate, pre-existing `session.Cast` limitation), the recipient's next
`Unlock` attempt (in or out of combat) can pose, and spend/keep/reload all
behave as Slice 3 already proved. Search stays deferred per the section
above. Protos/API/web adoption is out of toolkit scope and untouched here.

## Resistance: saving-throw post-roll offer plan

Resistance is Guidance's saving-throw sibling — 2014 PHB: touch one willing
creature; once before the spell ends, the target can roll a d4 and add it to
one saving throw of its choice, before or after making the save; the spell
then ends. Same shape as Guidance in every way that matters (touch,
one-minute concentration, the die joins the roll before the outcome is read)
except the roll kind: a saving throw, never a check, never damage, never an
attack. The user authorized this work after the actual architecture was
read out of the code, not assumed — recorded here so a later session does
not have to re-derive it.

### Why this is harder than Guidance, precisely

Checks had one problem (no suspend capability at all) and one lucky
accident (Unlock doesn't go through `resolution.Resolve`, so it was cheap
to make poseable standalone). Saves have a different, harder problem:
**every saving throw in this codebase is invoked as a `Request` sub-machine,
and `Request` refuses a sub-machine that poses** — checked in `step.go`:
`drive()` (the function `Request` alone calls) errors outright with "a
requester cannot be suspended" the moment its driven machine returns a
`Pose`. This is not a check gap, it is a deliberate refusal already coded.

**But there is exactly one place this needs fixing**, confirmed by grep:
`requestSave` (`contest.go`) is the *only* caller of `NewSave` anywhere in
the module, and it is what every save-gated spell's cast machine
(`contestMachine`, built by `newGatedCast`) goes through. Bane, Command,
Sacred Flame's DEX save — all of them, one choke point. Exactly the
`resolveStagedCheck` shape: fix it once, every existing and future
save-gated spell gets it for free, no per-spell integration.

**The complication is nesting depth, not breadth.** A single-target
save-gated cast has ONE `Request` in the way (`requestSave`, inside
`contestMachine`). A multi-target cast (Bane, up to 3 creatures) has TWO:
`castMachine.resolveTarget` also wraps each target's `contestMachine` in
its own `Request`, recursively building `m.outcome.Targets` as it goes
(`action.go`). So a save's pose has to climb two `Request` layers for
Bane, one for Sacred Flame — the fix has to work at both depths, but
`castMachine`'s own recursive shape already carries exactly the state
freezing it would need (which target index, everything resolved before
it) — there is no separate bookkeeping structure to invent.

**Session needs no module-boundary split this time.** Checks split across
Search (blocked, `encounter.CheckResolver` can't carry a pose) and Unlock
(clean, direct). Every current save happens through casting a spell, and
every spell — Bane, Command, Sacred Flame, and Resistance's own future
targets — already goes through the one `session.Manager.Cast` entry.
Fixing Cast's pose handling once covers every save-gated spell that
exists today, the same way fixing `requestSave` does in resolution.

### The actual fix

`Request` gains a second, optional callback beside `next` — something
like `onPose(ctx, Pose) (Step, error)` — invoked when the driven machine
poses instead of finishing. Left unset, behavior is EXACTLY what it is
today: `drive()` refuses a posed sub-machine, unchanged, so every existing
`Request` user (including ones that never touch saves) is provably
unaffected. `requestSave` supplies one that turns the save's `Pose` into
`contestMachine`'s own, freezing the contest's own inputs (ability, DC,
source) alongside the save's frozen bytes. `castMachine.resolveTarget`
supplies one that does the same one layer up: freeze `index`, the
`CastOutcome` accumulated so far, and the inner (contest's) pose.

Note what does NOT need to change: `driveStep`'s own `Gather`-to-`Pose`
path already works today (that is how Strike poses, see `strike_pose.go`),
and `resolveOn`/`Output.Posed` already handle a `Pose` arriving from
`driveStep` regardless of how deep inside the machine it originated. The
only place that currently forecloses this is `Request`'s own `drive()`
call and its blanket refusal.

### Slices as delivered — three PRs, not four

The draft plan below assumed Guidance's exact four-PR shape (root →
resolution → session → root). Once inside the code, two things collapsed
that to three:

- `step.go`, `contest.go`, and `action.go` all sit under the one
  `resolution` go.mod, so the one-nearest-go.mod-module-per-PR rule never
  forced the `Request` capability, the single-target wiring, and the
  multi-target propagation apart — they were always one release unit
  regardless of how many files they touch. Splitting them would have been
  an incremental-safety preference, not a boundary the rules require, and
  it would have cost an extra PR for no isolation Guidance itself didn't
  need.
- The root "not yet castable" slice and the root "enable it" slice are
  the same module too, and since nothing about #1778 had merged (or could
  have merged, given the pseudo-pin approach below) before #1779/#1780
  were built, there was no ordering reason to keep them in separate PRs
  either. They landed as two commits on the same branch instead.

**The four-PR draft, superseded by what actually shipped:**

1. ~~Root, not yet castable~~ — folded into PR 1 below.
2. ~~Resolution, single-target~~ / ~~multi-target~~ — one PR either way; see PR 2.
3. Session — shipped as planned; see PR 3.
4. ~~Root: enable it~~ — folded into PR 1 as a second commit.

**What shipped:**

1. **[#1778](https://github.com/KirkDiggler/rpg-toolkit/pull/1778) — root.**
   `PostSaveRollOfferEvent`/`PostSaveRollOfferChain` in `events/offer.go`
   (sibling to the check one), `ResistanceCondition` mirroring
   `GuidedCondition` — offers on the SAVER's next saving throw, no
   ability filter (RAW: "one saving throw of its choice") — and, once
   #1779/#1780 existed to run it, a second commit on the same branch
   adding Resistance to `castContent` and the character-package
   acceptance trace the CLAUDE.md checklist asks for.
2. **[#1779](https://github.com/KirkDiggler/rpg-toolkit/pull/1779) —
   resolution, one PR.** The opt-in `onPose` callback on
   `Request`/`driveStep` in `step.go` (unset stays today's
   error-on-pose, proven byte-identical by the full existing suite);
   `save.go` folds `PostSaveRollOfferChain` right after the d20 and poses
   when the saver holds an offer; `contest.go`/`contest_pose.go` freeze
   the contest's own consequences; `action.go`/`cast_pose.go` propagate a
   pose one `Request` layer further out for a multi-target cast, and
   `NewCastResumed` hands the resumed machine to the EXISTING `Resolve`
   entry unchanged — `Output.Posed` already existed generically from
   Strike's own pose, so `resolve.go` needed no changes at all. Proven
   against Bane (shipped, save-gated, multi-target) for both the
   single- and two-`Request`-layer cases, and against a real
   `ResistanceCondition` actually holding the offer.
3. **[#1780](https://github.com/KirkDiggler/rpg-toolkit/pull/1780) —
   session.** `Cast`'s tail is extracted into `finishCast`, called by
   both the fresh and resumed paths so they cannot write two different
   beats for the same cast. `poseCastWindow` mirrors `poseUnlockWindow` —
   and building its own session-level test caught a real bug: the
   caster's payment (a spell slot, an action) is charged in memory before
   the door yields its first step, but nothing was persisting that charge
   on the posed path until `poseCastWindow` was given its own
   adopt-and-save. `answerCastOffer` resumes through
   `resolution.NewCastResumed` with no `Cost` (already paid), and detects
   a SECOND `Output.Posed` after resuming — Bane can ask several targets
   in turn — reaching for `poseCastWindow` again rather than a second
   copy of it. New `windowKindCastOffer` payload/declaration/dispatch,
   `windowKindCheckOffer`'s own shape. Proven against Bane, including the
   two-target re-pose case.

**Pin discipline for this slice.** The user explicitly authorized a
one-off exception to the standing one-PR-at-a-time release rule for this
whole slice: each PR pins the PREVIOUS one's pushed commit as a
pseudo-version (`go get module@sha`) rather than waiting for it to merge
and tag, specifically "in case we need to go back" — so a finding at the
session layer could still change the resolution design before anything
was tagged. All three PRs were built and their full suites proven green
this way before any of them merged. The pins still get swapped to real
tags in publish order (#1778 → #1779 → #1780) before each is marked
merge-ready, per the standing rule.

### Expected cross-repo scoping (unconfirmed until there)

Following the pattern the Guidance handoff established and verified: the
ask/answer wire shape (`VERB_REACT`, `ReactionRef`) and the outcome shape
(`RollComponent`/`RollCalculation`) are both already generic in protos and
already adopted by rpg-api for the attack case, per the rpg-api#985
investigation. A save-offer window should ride the same generic shapes
with no new proto messages, same as Unlock's did — but that is an
expectation carried over from a different feature, not a fact checked
against this one yet, and should be verified once rpg-api actually adopts
this slice.

### Deferred, explicitly out of scope

Nothing about `AbilityCheckChain`'s existing pre-roll bonus mechanism
(Raging, etc.) changes. Death saving throws are saving throws mechanically
but are rolled through a separate `DeathSave` verb, not through a cast at
all — whether Resistance's die can be banked for one is a real rules
question (RAW allows it: a death save is "one saving throw of its
choice") not addressed by this plan, and DeathSave's own resumption shape
has not been inspected. World-clock casting remains the same pre-existing
`session.Cast` limitation Guidance already lives with. Search-style
secrecy concerns do not apply here — nothing about a saving throw is
supposed to be hidden from the creature making it.

Sources: [2014 Resistance](https://www.dndbeyond.com/spells/2153-resistance),
user-confirmed scope: saving throws only, the roll itself rather than any
resulting damage.

## Spare the Dying wiring inspection

### Executable content after session adoption

Session #1741 merged as `89e3ac92`, publishing `session v0.86.0` with stabilization
offers, persistence and typed live/Story results. The root-content slice now
registers Spare the Dying as a one-action cantrip with touch reach, exactly one
recipient and only `CastProfile.Stabilize`. It declares no slot cost, roll, save,
healing, condition or concentration. Existing cantrip choices remain unchanged.

Content tests cover the exact profile/price, clone and JSON round-trip, supported
cantrip filtering, and compatibility with the bonus-action spell restriction.
A normally finalized Cleric chooses it through the draft path, reloads draft and
character JSON, compiles the definition, retains both slots and reads private
status. This is acquisition/content evidence, not a public session cast.

Root #1742 merged as `9c99b7e8`, publishing `v0.168.0`. The final session-only
slice adopts that tag and verifies public `Afford` → `Cast` using saved output
from a normally finalized Cleric, including stable turn behavior and private
status after reload. Fixture provenance is recorded alongside the session test.
Hosts adopting the enabled root
content need at least session `v0.86.0`, resolution `v0.48.0`, and encounter
`v0.82.0`; the upcoming session release will pin the complete tested combination.
Protos/API/web result adoption remains pending. Preparation, monster saving and
timed natural recovery remain deferred.

### Session adoption after encounter recording

Encounter #1740 merged as `d534145b`, publishing `encounter v0.82.0`.
The session slice adopts that tag, resolution `v0.48.0`, and root `v0.167.0`.
Stabilization profiles request candidates from `resolution.StabilizationTargets`
over the same known-sheet universe used for touch healing. This preserves touch
without requiring current sight, while eligibility comes from the stabilization
provider rather than healing or attack rules.

Both activation and cast result translation carry the authoritative stabilization
detail into encounter recording. Live delivery and Story decoding expose
`ActivationResultBody.Stabilized` with target/source identity, before/after
`LifeState`, unchanged HP and `DeathSaveProgress`. Strict decoding rejects missing,
null, duplicate or extraneous stabilization fields; other result kinds cannot
carry stabilization detail. Existing host turn-driver configuration is preserved.

The session regression uses a generic compiled profile because the published root
catalog still disables Spare the Dying. It covers offer compilation, target
selection, resolution, sheet saves, record/commit, live events and JSON Story
reload. It is not yet a public `Afford` → `Cast` or native Cleric acceptance test.
After this session release, enable executable root content in its own PR, then
adopt that root release in session with public-verb acceptance. That test must use
a normally finalized Cleric, check private status and stable turn behavior, and
confirm damage/healing transitions remain intact.

Protos needs a stabilization result arm matching `StabilizedBody`; API maps that
arm for live events and Story and adopts the final toolkit releases. Existing
private-sheet life/death-save fields are sufficient. Protos/API/web evidence is
pending; no cross-repo changes are included here. Preparation, monster saving and
timed natural recovery remain deferred.

### Resolution delivery after the provider release

Resolution #1739 merged as `a0d4a5e5`, publishing `resolution v0.48.0`.
The encounter-only recording slice adds `ResultStabilized` to the shared cast /
ability result family. Its required neutral detail carries before/after state,
unchanged HP and resulting death-save progress; no healing or rolled death save
is synthesized. Both recording verbs reject malformed inputs before appending
and preserve the detail through JSON reload and audience-scoped Story reads.
Encounter needs no rulebook dependency or pin change for this primitive carrier.

Next, after encounter merges and publishes its tag, update session against that
tag and resolution `v0.48.0`. Session must wire stabilization offers and preflight,
map `ImposedStabilized` into recording and live results, and test save/reload.
Main also includes session `v0.85.0`'s supplied-driver changes; preserve those
contracts when adopting. Executable content and proto/API adoption are still
pending, with monsters and timed recovery deferred.

Root PR #1738 merged and published `rulebooks/dnd5e v0.167.0`. The next
resolution-only slice adopts that real tag and executes `CastProfile.Stabilize`
through the existing gateless cast. Character eligibility and touch reach run
before payment; the normal Gather calls `Character.Stabilize` after payment.
`EffectStabilized` becomes `ImposedStabilized` in `CastOutcome.Targets`, carrying
the provider's detached before/after life state, unchanged HP and reset progress.
`StabilizationTargets` supplies provider-owned eligibility for explicitly named
candidates so session can use the same rule in offers.

After this resolution release, add the neutral encounter recording result in
its own PR, then adopt the published providers in session for offers, execution,
recording and reload. Enable executable cantrip content only after those consumers
support it. Coordinate the proto result and API adoption separately; existing
private-sheet fields already carry stable state. No temporary pins or concurrent
dependent PRs. The inspection below records the pre-provider baseline.

### Accepted scope and first provider slice

The user authorized implementation. Use existing character death-save/life-state
support only; saving downed monsters and natural recovery after 1d4 hours are
deferred. A living character at zero HP remains eligible when already stable.
Casting still spends its action. Use touch reach (currently five feet with
blocking boundaries), with no added visibility prerequisite. Dead and positive-HP
characters fail eligibility before payment. No slot, roll, healing, concentration
replacement or resurrection is involved.

The first root-module PR supplies `Character.CanStabilize`, `Character.Stabilize`
and the generic `CastProfile.Stabilize` delivery declaration. Stabilization
clears both death-save counters, preserves HP, marks the recipient dirty and
returns detached before/after state and progress for result reporting. Existing
damage and healing transitions remain authoritative after stabilization.

Do not register Spare the Dying as executable in this provider PR: it is already
selectable, so enabling its profile before resolution/session understand the
delivery would advertise an unusable cast. After the root release, add delivery,
target eligibility and neutral result recording in their owning modules, one
PR/release at a time; then enable content against consumers that handle it.
Coordinate the stabilization wire result and consumer adoption explicitly.

Provider regression coverage uses a genuinely finalized Cleric, applies damage,
stabilizes, reloads JSON, reads private status, confirms repeat stabilization,
then verifies subsequent damage and healing. Profile validation/round-trip and
ineligible-recipient refusal are covered separately. These are provider tests,
not a claim of session casting or end-to-end acceptance.

Inspected toolkit `origin/main` at `7ed4c96e`. This is an investigation, not
tested spell support; implementation authorization is recorded above. The existing stabilized
life state persists, suppresses death saves, retains unconsciousness at zero HP,
auto-passes turns, and is cleared by subsequent applied damage or recovery.
There is no public authoritative character stabilization operation; the old
reset method is deliberately inert. Do not mutate a copied death-save state or
manufacture death-save successes to implement the spell.

The root cast profile currently supports damage, conditions and healing, with
no instantaneous stabilization delivery. Resolution's gateless activation and
result conversion need a corresponding delivery/result. Touch reach can be
reused, but healing eligibility cannot substitute for living-at-zero eligibility.
Session selects special candidates only for known-creature and healing profiles;
it needs provider-owned stabilization eligibility and the usual offer/preflight
agreement. Root content must declare an action cantrip, with no slot, save,
healing roll or concentration. Persistence must mark the recipient dirty.

Encounter activation results and the current proto activation-result union have
no stabilization result. DeathSaveRolled describes a real rolled death save and
HealingApplied describes actual healing; neither is an honest substitute.
Propose a reusable stabilization result for live/Story reporting, verify it with
the contract owner, then adopt published releases in the established sequence.
Existing private-sheet life/death-save fields already represent the final state.

Before implementation, resolve already-stable target behavior and the current
down-monster limitation (monsters are classified as defeated, without character
death-save state). Explicitly scope natural recovery after 1d4 hours: no matching
recovery scheduler was found. These are not reasons to silently invent a new
monster lifecycle or omit recovery behavior. Tests should cover unchanged zero HP,
cleared save progress, no concentration replacement, no slot spend, turn skipping,
damage restarting dying, later healing, reload, and authoritative live/Story facts.

Rules evidence: [2014 Spare the Dying](https://www.dndbeyond.com/sources/dnd/basic-rules-2014/spells#SparetheDying)
and [stabilization](https://www.dndbeyond.com/sources/dnd/basic-rules-2014/combat#StabilizingaCreature).

## Private-sheet acceptance correction (#1720)

The creation/persistence/casting plan omitted a required read path: projecting
the normally finalized character for the owner's private sheet. Cleric resources
were created correctly, but `StatusView`'s closed owner catalog omitted Cleric.
Condition source identities also survived mechanics and persistence but were
dropped by that private projection. This is a projection coverage gap, not
evidence of missing spellcasting mechanics.

Toolkit PR #1721 adds the Cleric Hit Dice/level-one slot catalog arm and projects
source-qualified condition identity. Its regression finalizes a Life Cleric,
checks resources, spends a slot, adds focused Bless/Bane fixtures, reloads JSON,
and verifies sources, rest cleanup/restoration and detached resource views.
Cross-class resource rejection remains strict. The condition fixtures do not
constitute cast acceptance. Local root-module tests/vet/lint and GitHub race
checks passed on `038f5070`; merge/release and user-reported end-to-end closure
are recorded in the current-direction section above.

For subsequent slices, record the actual published root
version and API adoption, then verify native unseeded creation, owner
GetCharacterData, combat offers/casts, authoritative resources/effects, and
private-sheet plus Story recovery after reconnect. Verify observable condition
sources at the host boundary. API tests and browser acceptance must each report
their own evidence; passing one does not close the other. Keep preparation,
automatic domain grants and Knowledge Domain's extra acquisition choices separate.

The reusable prevention checklist is in `character/CLAUDE.md` under this rulebook;
cross-project handoff evidence requirements are in the repository `CLAUDE.md`.

## Bless acquisition completion

Session #1712 merged as `ab7f7a80`, releasing `session v0.82.0`.
The root-module acquisition slice adds Bless to Cleric's supported first-level
pool through the existing selection/finalization path. The temporary
select-all requirement now contains five spells: Bane, Bless, Command,
Cure Wounds and Healing Word. Preparation and automatic domain grants remain
deferred. Existing finalized sheets are not backfilled during load; unfinished
drafts must include the expanded supported selection before finalizing.

No consumer pins change in this slice. API/SDK adoption still needs an explicit
stale-target policy (recommended `refuse`), and API/web adoption must expose
the expanded choice list and cast-miss events before claiming a complete
player-facing Cleric journey.

## Active session integration

Session adopts released resolution `v0.46.0`, encounter `v0.79.0`, and root
`v0.164.0`. The user selected optional host configuration: unset
`Config.StaleTargetPolicy` leaves known-creature offers visible but disabled
with an explicit reason; casting that offer returns `ErrIncompleteConfig`.
Other spells remain usable. Invalid nonempty settings fail Manager construction.
API/SDK setup should explicitly default to `StaleTargetRefuse`, with
`StaleTargetAttempt` available as an override. No external repository changes
are included here.

Known-cast selectors include the policy, so changing it invalidates an old
offer instead of silently changing whether it spends. Existing cast selectors
retain their format and behavior. `MissedTargets` appears in CastOutput and
`EventCastMissed`/`CastMissedBody` preserve the encounter beat for stream and
story readers. Bless acquisition remains the next root-module slice.

## Bless resolution and recording handoff

Resolution #1710 merged as `c893f5f5`, releasing `resolution v0.46.0`.
Main also includes session #1690 (`v0.81.0`), which projects area footprints
on declarations; preserve that behavior when integrating Bless.

Session inspection found an additional provider prerequisite: encounter's
`CastTargetResult` only records saves/effects and cannot represent the new miss.
Ship an encounter-only additive `Missed` field and `cast_missed` story beat,
then adopt its released tag alongside resolution `v0.46.0` in session. A miss
carries actor, named target and spell identity, without coordinates or an
invented save/effect. Contradictory miss-plus-save/effect data is rejected before
any transaction beat is appended. Existing false/omitted miss values retain
the existing recording behavior.

Root provider #1708 is merged and released as `rulebooks/dnd5e v0.164.0`.
Resolution adopts that tag with encounter `v0.78.0`. Known-creature casts use
the observer's canonical location testimony, including remembered locations,
for range and clear-path checks. Self needs no self holding; conscious, dying,
and stabilized recipients are eligible. Dead/defeated recipients are not.

The user chose two explicit host-configured stale-target policies: `refuse`
rejects the whole cast before spending; `attempt` pays once and reports a miss
only for recipients no longer at their remembered locations. No policy is
silently selected, and no hidden live position replaces the aimed location.
Mixed casts retain caller target order. An all-miss concentration cast follows
the existing all-save behavior: it ends the previous hold and starts an empty
hold with the declared duration.

Session must carry the same policy through offers and execution and preserve
the new per-target miss result when recording after that provider releases;
Bless acquisition follows the session integration. External repository pins,
preparation, and upcasting remain outside this slice.

Design baseline: toolkit main `fd4cdade`. Session handoff updated 2026-09-12
against main `ead003f0` after #1693, #1694, and #1695 merged.
Implementation is authorized by the user in this conversation after their maintainer discussion.
The external proposal still says approval requested; this records the user's authorization,
not an invented GitHub approval or board status.

Proposal: [toolkit #1643](https://github.com/KirkDiggler/rpg-toolkit/issues/1643).
Journey: [rpg-project #406](https://github.com/KirkDiggler/rpg-project/issues/406).
Companions: [design.md](design.md), [implementation.md](implementation.md),
[source-index.json](source-index.json).

## North star

### Release handoff correction (2026-09-12)

The user requires one PR at a time with actual released dependency tags.
Rulebook PR #1693 goes first, followed by encounter #1694, resolution #1695,
and session #1696. Leave the already opened downstream PRs in place; do not
advance or repin them together. After each provider merges, verify its CI-issued
module tag before updating and validating the next consumer. Existing temporary
pins in downstream PRs are outstanding release work, not an approved pattern
to repeat. The user controls merging. No other-repository dependency bumps are
authorized. Verify live PR and release state when resuming; this sequence is
not evidence that any PR has merged.

Provider releases verified: #1693 is `rulebooks/dnd5e/v0.159.0`; #1694 is
`rulebooks/dnd5e/encounter/v0.76.0`; #1695 is
`rulebooks/dnd5e/resolution/v0.44.0`. Session #1696 consumes all three real tags.

The user decided that missing creature type must not block selection or healing.
Only a known excluded type receives a paid no-effect result. Include the user
before settling eligibility, missing-data defaults, compatibility, or new scope.
Broad creature classification remains outside this slice. Session #1696 removes
the now-unneeded `HealingTargetsInput.Excludes` argument. Its acceptance tests
verify selection, healing, payment, saved HP and exact story replay after JSON
reload for monsters with missing or unknown refs and no explicit type. The full
session suite and vet pass with released dependencies and no workspace override.
The remaining handoff is review/merge of #1696 and its CI-issued session tag;
API/protos/web adoption remains outside this slice.

### Intended behavior

A level-one 2014 Cleric with explicit access to Cure Wounds can, during initiative,
choose themself or another creature they can touch, spend one action and one
first-level slot, and restore HP through the existing Cast path. The story explains
what was rolled, which modifiers contributed, and how much healing actually landed.
Reload preserves that story, the healed creature and the spent slot.

Example: Wisdom 16 and a d8 result of 5 request 8 HP. Disciple of Life adds a
separately attributed 3 at first level, requesting 11. If the target lacks only
4 HP, the result reports 11 requested and 4 restored. No saving throw or new
concentration is involved, and existing concentration is preserved.

This is a bounded combat healing contribution, not complete Cleric or Cure Wounds
support. Preparation remains deferred; explicit spell-access fixtures are intentional.

## What is already done

- Cleric creation/proficiencies and persistence: merged #1585.
- Sacred Flame content and session acceptance: merged #1592 and #1594.
  Cover fidelity, higher-level scaling and the shared cast-damage correctness
  work remain outside those proofs; do not absorb them into this healing slice.
- Bane supplies leveled-spell offers, first-level resource-backed slot payment,
  ordered target results, contributed dice and source-qualified concentration.
- Blade Ward supplies self-only casting. It does not supply choosing yourself
  among other creature targets.
- Area spells, Dissonant Whispers and Command now add area/cell targets, movement,
  half damage, options and driven turns. Preserve these existing contracts.
- Animated Armor supplies a real construct fixture; Skeleton supplies an undead fixture.

At this baseline session pins D&D `v0.158.0`, encounter `v0.75.0`, resolution
`v0.43.0`, spatial `v0.13.0` and intel `v0.2.0`. Those are evidence of the starting
graph, not versions to enforce forever. No other-repository pins are in scope.

## Design boundaries

### Healing is an immediate consequence, not a stored condition

The existing no-save cast path only delivers conditions. Generalize it to deliver
healing as a distinct consequence. Rename the condition-only result conversion
when its responsibility broadens; never create a transient healing condition just
to change HP. Preserve existing condition delivery and replacement behavior.

Separate initiating/paying, calculating, and applying healing:

- The cast/feature/item/rest entry owns its action and resource cost.
- Shared healing rules produce a sourced calculation from declared dice and
  applicable contributions. A spell delivery invokes this; reusable arithmetic
  must not be buried in the Cast verb or its delivery loop.
- Character/monster handlers remain the owners of HP mutation, clamping and
  applied facts. Character healing above zero already resets death-save progress.
- Persistent features/conditions can affect healing where their rules apply.
  Disciple of Life owns its spell-healing predicate and contribution; it is not
  a Cure Wounds-specific switch in session or a universal healing bonus.

Introduce only the small shared healing operation and context this customer needs.
Do not add a new independently versioned mechanics module, generic effect language,
or speculative potion/rest framework. Other healing sources can reuse the operation
later without inheriting spell rules. Medicine-related actions are not automatically HP healing.

### Content and targeting

Add a healing declaration to `combat/actions.CastProfile`, with validation and deep
cloning, and compile Cure Wounds through the existing spell table. Extend the exact
caster inputs beyond save DC to support the casting modifier and sourced healing
contributions. Reuse the current action-plus-first-level-resource price.

Add a content-controlled way to select one creature including the caster.
`CastTargetSelf` is a no-picker self-only mode; it cannot stand for this choice.
Cure Wounds is not restricted to allies. Do not disturb area, cell or option inputs.

Encounter owns positional facts. Resolution/rulebook owns what those facts mean for
touch and recipient eligibility; session projects those answers. Existing sightings,
attack-target participation and range alone do not establish the complete healing
contract. In particular, do not add a sight requirement merely because attack offers
use sightings. Inspect existing encounter/spatial obstruction capabilities first.
`CellAt` answers movement permission, which is not itself permission to touch.

Resolve and test touch, dead-target and no-effect rules against the reviewed 2014
sources before implementing those decisions. Undead/construct immunity to this spell
must not be confused with an invalid request. Full HP does not imply a malformed target.
Do not fabricate HP support for world-kind NPCs that have no healable sheet.

## Implementation route

These describe implementation responsibilities; the release handoff above governs
the one-PR-at-a-time publication sequence.

1. **Define the acceptance scene and narrow contracts.** Start with a Cleric,
   another injured character, an undead and a construct. Record the expected
   request, payment and sourced result. Resolve the targeting/no-effect questions
   above and the Life feature's load/attachment path. Keep ordinary-caster controls.
2. **Root rulebook support.** Add the healing declaration, shared healing calculation
   and Life contribution, plus Cure Wounds content. Use typed source context so the
   feature can distinguish leveled-spell healing from unrelated healing. Feature
   availability must survive load without duplicate bonuses; it must not require
   implementing the separate automatic domain-spell grant system.
3. **Resolution delivery.** Extend `newGatelessCast`, `preparedCast`, `startCast`
   and `deliverCast`. Preflight declaration and recipient before payment/RNG;
   execute healing inside the existing Gather after the door pays. Publish
   `HealingReceivedEvent` with matching amount/calculation and spell provenance.
   Reuse `EffectHealingApplied` capture. Generalize `deliveredConditions` to
   translate condition and healing results, adding an `ImposedEffect` healing arm
   that retains requested/applied HP, before/after HP and calculation. Preserve
   Command option binding, condition replacement, movement and other result arms.
4. **Encounter and session integration.** Reuse `RecordCast` and
   `ResultHealingApplied`; add record acceptance rather than a new record format.
   Extend encounter production code only where touch or recovery proves a missing
   capability. Update session offers and `imposedResult` to carry the provider's
   target policy and healing facts. Keep `Manager.Cast`, `Targets`, selectors,
   payment, dirty-sheet saving and the existing healing presentation body.
5. **Prove the combined path.** Run the scene through actual module consumers,
   including repository JSON round trips and manager recreation. Fix demonstrated
   gaps before presenting the wave for review. Record observed results and remaining
   limits in `implementation.md`; do not turn this checklist green from code reading.

## Acceptance checklist

Passed in the combined local walk; full root tests also pass on independent pins.
Pushed consumer pins and CI are checked during publication.

- [x] Ordinary caster: correct d8 and casting modifier, with sourced calculation.
- [x] Life Cleric: separate first-level Disciple contribution, applied exactly once;
  survives load and does not apply to an unrelated nonspell healing control.
- [x] Self and another reachable creature are selectable; no accidental ally-only rule.
- [x] Touch boundary and obstruction agree between offers and execution, including
  the resolved visibility rule. No healing through an impassable barrier by distance alone.
- [x] Malformed, duplicate, unknown, stale and unreachable selections refuse before
  RNG/payment; current spell access and target state are revalidated.
- [x] Exactly one action and one first-level slot are spent. Either exhausted currency
  refuses without mutation. Existing cantrips remain action-only.
- [x] Requested versus applied healing is correct at and near maximum HP; full-HP and
  zero-effect casts have explicit, source-backed payment and result assertions.
- [x] Dying/stabilized character healed above zero resets death-save progress and can
  participate correctly afterward and after reload. Dead targets are not revived.
- [x] Skeleton and Animated Armor receive no healing; no-effect handling and cost
  are explicit and distinct from invalid-request refusal.
- [x] Negative healing cannot lower HP. Coordinate the domain guard with #1467;
  keep calculation totals and requested amounts consistent.
- [x] Cure Wounds causes no save or concentration owner and preserves existing concentration.
- [x] JSON round trips and manager recreation retain exact healing story, HP, Life
  availability and spent resources without duplicate application or reroll.
- [x] Existing long rest restores the slot resource; no new slot ledger is created.
- [x] Roller, publication and repository failures have explicit assertions describing
  actual state at failure; no unsupported cross-repository atomicity/refund promise.
- [x] Sacred Flame, Bane, Blade Ward, Second Wind and the newer cast paths retain their
  behavior. Full suites in changed modules and the combined consumer path pass.

## Failure semantics and scope control

[#1467](https://github.com/KirkDiggler/rpg-toolkit/issues/1467) tracks negative healing.
[#1466](https://github.com/KirkDiggler/rpg-toolkit/issues/1466) tracks Second Wind
resource behavior on infrastructure failure. Both remained open at reassessment.
Coordinate shared fixes without silently widening this issue into a resource rewrite.

Known invalid inputs and inability to pay must refuse without spending or rolling.
That is different from an unforeseen failure after payment. Session currently saves
sheets before recording/committing the encounter; do not promise transaction-wide
rollback across those writes. Test and document the actual failure behavior.

Deferred: preparation management, automatic domain-spell grants (#299), upcasting,
level progression, components/focus enforcement, out-of-combat/world-clock casting,
API/protos/web adoption, other-repository pins, Bless content and broad damage fixes.
`session.Cast` still refuses world-clock casts at this baseline. Between-fights healing
is a later explicit slice, not a hidden assumption in combat acceptance.

## Development, review and publication

Advance one owning-module PR at a time using actual released provider tags.
Do not introduce temporary dependency pins or committed local overrides. Verify
the current PR before handoff; the user merges it, CI issues the module tag, and
only then update and verify the next consumer. Keep pending consumer acceptance
distinct from evidence already established for the current module.

Run formatting, tidy and meaningful tests in owning modules. On Windows, keep
unrelated line-ending changes out of the diff. Report local tooling limits honestly;
CI supplies additional validation. Check current adjacent PRs before changing shared
cast/targeting surfaces, especially ongoing perception and declaration presentation work.

Keep this plan and the implementation evidence current as findings change the shape.
User authorization is already given; ordinary implementation choices do not require
asking for the same approval again. Material scope changes should be surfaced clearly.

## After this slice

### Healing Word inspection (2026-09-12)

The user authorized continuing with Healing Word, Bless, remaining cantrips,
and domain support as separate slices, one PR at a time. Healing Word starts
from main `f39a7b26`, after acquisition #1698 released as rulebook v0.160.0.

Existing healing arithmetic, ability/Life contributions, exclusions, paid
delivery, HP persistence, and story results are reusable. Missing creature
types remain eligible under the settled user policy. The current profile
validator permits healing only on touch casts; resolution and session also
use touch-specific target handling. Healing Word needs a visible creature
within 60 feet, including eligible self-targeting, and a bonus-action cost.
Its first-level healing is 1d4 plus casting modifier. Preparation, automatic
domain grants, upcasting and world-clock casting remain separate.

The 2014 bonus-action spell restriction is not currently enforced: session's
cast documentation explicitly relies on spending the standard action to stop
a second spell. The user explicitly chose to implement the shared same-turn
restriction with Healing Word. Enforcement must handle both cast orders,
one-action cantrips, same-turn reaction spells, persistence, and turn boundaries;
it must not be replaced with a blanket one-spell-per-turn restriction.

Sources: [Healing Word](https://www.dndbeyond.com/spells/2140-healing-word),
[2014 casting rules](https://www.dndbeyond.com/sources/dnd/basic-rules-2014/spellcasting).
Acceptance should cover visibility/range, self/ally/other known recipients,
dying recovery, missing-type healing, known excluded types, ordinary/Life
calculations, bonus action plus slot payment, replay, and the agreed same-turn
policy. Stage provider changes first; adopt each real release before advancing
the next module. Do not enable acquisition before the delivery path is usable.

The first provider PR supplies the shared rule and character payment operation:
`combat.SpellTurnState.AfterCast` checks the declared spell level and casting
time in both orders; `Character.CanPaySpell` projects legality and affordability;
`Character.PaySpell` pays through the existing gate and records history only
on success. History persists with the character economy but has its own explicit
turn identity, preserved across economy refresh. It is cleared on combat exit.
Ordinary `combat.Pay` continues to own prices only.

This provider does not enable Healing Word or wire the live casting door yet.
After it releases, the remaining work is spell content/classification, ranged
healing target validation and delivery, then session offers and execution. The
composition must supply a turn identity that distinguishes active creatures,
rounds, and encounters; the payer's existing refresh number is insufficient.
Consumer tests must prove that identity using actual clock transitions, as well
as offer/execution agreement, saved continuation, and failed-payment behavior.

### Healing Word session integration (after #1703)

#1703 released resolution `v0.45.0`. The session slice updates that pin and
rulebook `v0.162.0` together with the calling code; encounter stays `v0.76.0`.
Offers ask `Character.CanPaySpell`, and Cast supplies the same explicit turn
identity to resolution payment. The token includes session, encounter, round,
and active member. Existing combat-exit cleanup clears spell history before
a new fight can reuse round one. The seam reports the provider's restriction
as an unavailable offer; it does not recreate the spellcasting rule.

Healing offers route to touch or ranged healing eligibility in resolution.
Ranged healing includes self and dying/stabilized recipients while requiring
current sight and unblocked range for other creatures. Missing creature type
continues to allow healing; known exclusions remain paid no-effect.

Validation covers both spell orders, action-cantrip combinations, actual round
and active-member changes, combat exit/re-entry, JSON reload, saved cast-push
resume without repayment, healing calculation/story projection, and stale
sight/wall/range/target/slot refusal before payment. Existing Bard and Cure
Wounds tests remain part of the full session suite.

After this session release, the next root-only slice can expose Healing Word
in acquisition. This PR does not change preparation, acquisition, or external
repository pins.

### Healing Word acquisition (after #1705)

#1705 merged as `9fbfba45` and released session `v0.79.0`. The root-module
acquisition slice adds Healing Word to the existing Cleric and Bard options.
Cleric continues to select all supported first-level spells while preparation
is deferred (now four). Bard keeps the existing four-known-spell limit and
chooses four of five supported options. No automatic grant is added to loading
an existing sheet, and existing Bard selections remain valid. An unfinished
Cleric draft must refresh its spell selection to include the new supported entry.

Validate accepted selections through finalization and JSON reload, preserve
spent slots on load, and retain rejection of duplicates, unsupported entries,
and incorrect counts. Slot initialization, casting mechanics, preparation,
domain grants, and adjacent repository pins do not change in this slice.

### Bless: provider, resolution, session, acquisition

Healing Word acquisition #1707 merged as `7d79fad7`, releasing rulebook
`v0.163.0`. Bless is the next authorized spell. Its reference is the
[2014 Basic Rules Bless entry](https://www.dndbeyond.com/spells/2016-bless):
one action, 30 feet, up to three creatures, concentration up to one minute,
and an additive d4 on attack rolls and saving throws.

The user explicitly chose known targets within range and a clear path,
including the caster and dying/stabilized recipients. Current sight is not
required. This must be a declared targeting contract, not spell-name dispatch
or reuse of attack eligibility. Dead/defeated targets are not eligible; Bless
adds no undead/construct exclusion. Keep the existing one-to-three selected
target convention and ten-subsequent-turn-end concentration timing.

1. **Root provider:** add `BlessedCondition`, canonical ref/loader/display and
   cleanup registration, plus the level-one Bless profile and a declared
   `CastTargetKnownCreature` contract. Reuse contributed dice, source-qualified
   addresses, non-stacking groups, and the existing concentration owner.
   No acquisition or consumer pin changes in this PR.
2. **Resolution, after the root release:** pin the actual tag; bind Blessed
   in the existing prepared condition delivery; support known-creature
   eligibility and range/clear-path validation. Prove selected self, multiple
   recipients, dying/death saves, attacks and saves with sourced calculations,
   Bless plus Bane, overlapping casters, reload, and all concentration endings.
   Validate before payment or RNG and preserve the spell-turn gate.
3. **Session, after resolution release:** update the real pin together with
   offer/execution plumbing. Project known targets including self rather than
   current-sight attack candidates, revalidate on execution, and prove the
   resulting casts and bonuses through persisted story and actual turn flow.
   Check whether this consumer requires any additional projection fields.
4. **Root acquisition, after session release:** add Bless to the existing
   supported Cleric selection without silently implementing domain preparation.

Continue one Go module per PR, merge/tag before the next consumer pin, and no
temporary versions or adjacent repository work. Preparation, domain automatic
grants, upcasting, and world-clock casting remain deferred.

After #1709 and #1706, the next Bless consumer work starts from resolution
`v0.45.1`, encounter `v0.78.0`, and session `v0.80.0`. Encounter reads return
`perception.Holding` with `Current`; persisted channel data remains under
`EncounterData.Perception.Intel`. Bless's known-target policy must not filter
out a remembered target merely because `Current` is false. The root provider
has no encounter/perception dependency and needs no pin or mechanic changes
for this refactor.

### Healing Word content provider (after #1700)

#1700 merged as `412f3dc7` and released rulebook `v0.161.0`. The next provider
adds Healing Word's level-one content: one bonus action and one first-level
slot, one creature at 60 feet, 1d4 plus existing sourced healing modifiers,
and the settled undead/construct exclusions. The profile accepts single-target
ranged healing; resolution must still implement sight/range and healing-life-state
validation, including self and dying recipients, before the session enables it.

Every executable spell now declares `combat.SpellCasting` alongside its content.
Do not derive classification from payment: free spells still obey casting rules.
The older `SpellData` table is incomplete for working Bard cantrips, so it is
not a prerequisite for compiling cast content. An absent profile classification
remains explicitly unclassified; it must never be interpreted as a free action
cantrip by the upcoming consumer. Existing profile validation accepts older
unclassified declarations; the live enforcement contract is still pending.

Cleric/Bard acquisition lists remain unchanged. After this provider releases,
resolution must adopt its real tag and wire classification plus explicit turn
identity into spell payment and ranged healing. Session follows with offers,
execution, and real clock transition tests. No adjacent repository pins change.

### Healing Word resolution integration (after #1701)

#1701 merged as `45d5ab49`, releasing rulebook `v0.162.0`. Resolution adopts
that actual tag, keeping encounter at its existing `v0.76.0` pin.

The user approved rejecting incomplete costed casting inputs, with the explicit
requirement that session's calling code and dependency pin be updated together.
The runner reads classification from the cloned cast profile and requires
`Cost.SpellTurn` plus a matching caster/payer. It uses Character.PaySpell after
target preflight. A free combat spell supplies a Cost with nil Profile; the
existing nil-Cost API remains explicitly ungated mechanical resolution.

Ranged healing uses the same prepared healing delivery and a dedicated target
projection. It permits healable dying recipients, self, and unknown creature
types; exclusions remain paid no-effect outcomes. Other recipients require
current encounter sight holdings as well as range and an unblocked ray. The
encounter's pinned View implementation was audited: it only reads held intel,
without consulting capabilities or refreshing perception.

Before session adopts the resolution release, update every costed cast to
supply the explicit turn identity and project the same spell-payment rule in
offers. Prove identity across actual turn transitions and encounter lifetimes,
including combat exit/re-entry, and preserve already-paid history on resume.
Update the session pin and plumbing in one PR; a pin-only upgrade would refuse
existing casts. Then enable acquisition and verify the complete player flow.

### Supported spell acquisition (2026-09-12)

The user authorized reusing Bard's creation/known-spell pipeline while full
preparation is designed separately. Offer all currently supported first-level
Cleric spells: Bane, Command, Cure Wounds. `ClericSpells1` requires each once;
the shared validator must reject duplicates (#1662). Existing cantrip choices
remain unchanged. This is temporary spell access, not a Wizard spellbook,
prepared-spell limit, or automatic domain grant.

Finalization seeds the existing first-level slot resource from Cleric's class
table (two slots), sharing Bard's initialization and long-rest recovery.
Creation tests cover draft/character persistence, executable definitions,
slot spending/reload/rest, invalid picks, and class changes. Existing finalized
sheets are not backfilled during load. Unfinished drafts must supply the new
spell choice before finalizing; API/UI adoption must expose that requirement.
No consumer pins or preparation/rest selection flows change in this rulebook PR.

Next candidate is Bless using Bane's contributed dice and source-qualified ownership,
including selected-self targeting and overlapping-caster acceptance. Preparation/domain
grants and out-of-combat casting remain separately scheduled work. The eventual API/web
integration must be observed through the real game before declaring the Cleric journey
complete. Historical creation/Sacred Flame evidence remains in `implementation.md`.
