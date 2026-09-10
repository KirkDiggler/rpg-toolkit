# resolution — where one interaction happens, on the only bus that exists

> Contract, laws (R1–R7) and guarantees: [`doc.go`](./doc.go). **This file is not a summary of that one.**
> It answers a different question: I am building something — does it belong here, and what may I ask?

Its two neighbours answer the same question for themselves:
[`../encounter/doc.go`](../encounter/doc.go) (the composition) and
[`../session/doc.go`](../session/doc.go) (the host seam). Which package plays
which part is [`../CLAUDE.md`](../CLAUDE.md).

## What this package owns

- **The bus.** One per interaction, created in [`resolve.go`](./resolve.go),
  torn down with the call ([ADR-0038](../../../docs/adr/0038-resolution-owns-the-bus.md)).
  Nothing above or below holds one. If your design needs a subscription, it
  needs an interaction, and an interaction starts here.
- **The step vocabulary and the fold.** `Gather | Request | Pose | Done` in
  [`step.go`](./step.go) — sealed, and opaque on purpose: a machine cannot
  construct a `Gather` or a `Request`, it calls a constructor here that names
  what it wants. That opacity is how R6 is a guarantee rather than a habit.
- **The machines — the rules of D&D as steps over data.** `NewSave`
  ([`save.go`](./save.go)), `NewContest` ([`contest.go`](./contest.go)),
  `NewActivation` ([`activation.go`](./activation.go)), `NewStrike` /
  `NewStrikeResumed` ([`strike.go`](./strike.go)), `NewAction`
  ([`action.go`](./action.go)), `NewMovement` ([`movement.go`](./movement.go)),
  `NewBoundary` ([`boundary.go`](./boundary.go)).
- **The door, and the door pays.** `Machine.Start` is pure preflight; the cost
  is charged after it and before the first step — [`resolve.go:452-459`](./resolve.go),
  `payAtTheDoor` at [`cost.go:148`](./cost.go). A resolution nobody can pay for
  executes no step and writes nothing.
- **Ambient truth.** `installTruth` at [`truth.go:60`](./truth.go) is the ONE
  function allowed to call a `gamectx.With*`: the room, the cast, reaction
  readiness. Six attached-behaviour entries reach it and a seventh is deliberately
  not one — see [`preflight.go`](./preflight.go).
- **The attach loop and its teardown.** Sheets are loaded purely, attached in
  sorted order, and every subscription this package granted is revoked, success
  or failure. `Output.Hooks` is the record of what attached.
- **The entries with no interaction to run**: `ProjectCharacter`
  ([`projection.go`](./projection.go)), `Participation` / `Standing`
  ([`participation.go`](./participation.go), [`standing.go`](./standing.go)),
  `MakeCheck` ([`check.go`](./check.go)), `LongRest` ([`long_rest.go`](./long_rest.go)),
  `DeathSave` ([`death_save.go`](./death_save.go)). Each goes through the same
  door; none is a mode of another.

## What it must never learn

- **Anything between calls.** There is no resolution process. The bus dies with
  the call and the sheets go back out as data. A field you want to keep "just
  until next time" belongs to the caller.
- **How to persist.** [`resolve.go`](./resolve.go) hands back
  `Output.World`, `Output.DirtyCharacters`, `Output.DirtyMonsters`. Writing them
  is `session`'s job (load-act-save). This package opens no repository.
- **The bus, inside a machine (R6).** Guaranteed BY CONSTRUCTION: the step
  constructors' internals are unexported, so a machine cannot build a step that
  touches a bus.
- **The ledger — and here the seal does not exist.** "The runner spends, machines
  yield" is a DISCIPLINE, not a compiler-checked claim: `Participants` hands a
  machine the same `*character.Character` the gate debits, and `combat.Pay`
  ([`cost.go:168`](./cost.go)) is an ordinary exported function. `doc.go` says
  so plainly and rpg-toolkit#1095 is the issue. Do not reason about the two
  as if they were the same kind of guarantee.
- **Geometry, and who is on the board.** Those are `encounter`'s. This package
  builds no room; it installs the canvas the composition compiled. `encounter`
  does not even require `rulebooks/dnd5e` in its `go.mod` — which is why hit
  points are a fact it can only be told, and why the roster question comes back
  through `Input.World` rather than being computed here.
- **Which spell it is holding.** A cast declares an `actions.CastProfile`
  ([`../combat/actions/cast.go:65`](../combat/actions/cast.go)) that names no
  spell ([ADR-0045](../../../docs/adr/0045-actions-are-data.md)). Content
  declares, the machine executes. `spells.CastDefinition`
  ([`../spells/cast.go:286`](../spells/cast.go)) reads the `castContent` table
  and hands over a filled-in form.
- **What an action cost.** `Input.Cost` is compiled above this package. A
  machine cannot tell a swing that cost an action from one that cost nothing.

## Questions it answers, questions it asks

It ANSWERS exactly one question: **what happened when this interaction ran** —
and it answers in data (R2). `Output` carries the world, the dirty sheets, the
outcome or the pose, the hooks, and the concentration checks and breaks. No
runtime object crosses either way.

It ASKS the caller for everything else, and the `Input` fields say so in their
own godoc: `Standing`, `Sight`, `TurnDriver`, `CheckResolver`, `Witness` are
**carried, never consulted** — handed to the composition so a world can be
loaded at all. `Initiative` and `Deciders` likewise. `Roller` is REQUIRED and
never defaulted (rpg-toolkit#1033): a silent default is unreproducible dice in a
result that looks fine.

**R3 is the sharp one.** Pass everyone in. Not "everyone who might matter" —
everyone. Applicability is the effect's own predicate; scope is the caller's
business. Deciding relevance out in the wiring is how a rule ends up somewhere
no rulebook can see it, and it fails in the direction nobody looks: the effect
that would have applied simply never hears the event. `defaultReadiness`
([`truth.go:137`](./truth.go)) makes the same move — it readies every
participant for every free reaction and lets each condition refuse itself.

## Predicates and producers

Two shapes of question, and they are not the same animal.

A **pointed question** — "does this participant have condition X", "is this save
a success", "is b hostile to a" — is cheap and safe. It names its subject, the
answer is about that subject, and a wrong universe cannot hide inside it.

A question that **produces a set** — "who does this spell hit", "who is in this
area" — is different. Its answer silently depends on what happened to be loaded.
Two identical calls over the same fiction return different sets if the caller
attached a different cast, and nothing in the signature says so.

> **Every collection-returning API here names the universe it ranges over.** An
> observation about the surface as it stands, not a gate on the next one — but
> worth keeping, because a producer with an unstated universe fails invisibly:
> whatever it left out looks exactly like something that was never there.

For this package, R3 already supplies the universe: **the participant list IS
the declared universe of a resolution.** That is precisely why R3 says pass
everyone in — the completeness of the cast is what makes a derived set mean
anything. So: **a machine that derives a target set may only range over what was
attached**, and it says so where it is declared.

The producers that exist today all obey it, and are the models to copy:

- `Participants.IDs()` ([`resolve.go:325`](./resolve.go)) — the cast, in attach
  order.
- `gamectx.Cast.Members()` ([`../gamectx/cast.go:75`](../gamectx/cast.go)) —
  the same cast, deterministic order, and its godoc says why (R4).
- `spells.Castable(ids)` ([`../spells/cast.go`](../spells/cast.go)) — returns a
  subset of the ids it was GIVEN, preserving their order. The universe is the
  argument.

A footprint in space — the first content that declares a shape rather than a
target list — is the case this rule was written for, and it is the case the rule
**refuses to let this package settle on its own**. A machine here may only range
over what was attached; the attached cast is not the set a shape in space
catches. Three universes nest, and they are not the same size:

- the caster's intel — what a compiled offer ranges over, which skips stale
  memories (`len(h.CurrentVia) == 0`) and drops world NPCs entirely
  (`excludeWorldNPCs`, [`../session/offers.go:677`](../session/offers.go));
- the **roster** — `encounter.Members()`, everyone actually standing there;
- the **participants** — this package's cast, built from roster members that
  HAVE A SHEET. `compileResolutionCast` skips `KindWorld` outright:
  [`../session/attack.go:891`](../session/attack.go), *"placed world NPC — no
  sheet, contributes nothing to the cast"*.

So a footprint fold over `Participants` would return a target list silently
short by exactly the members most worth seeing — a blast that reached the
shopkeeper standing beside the caster and never named them. Folding over the
room instead is worse: `readOnlyRoom.GetEntitiesInRange`
([`../encounter/canvas.go:197`](../encounter/canvas.go)) returns every entity
the composition placed, **props included** (`propEntityOf`,
`../encounter/compilefield.go:808`), and declares no universe at all.

The caught set is drawn from the roster and the resolvable set from the
participants, and only `session` sees both. Which is why the answer is being
designed rather than assumed — rpg-project#421. Do not pick an owner for it
here.

## Where does this go?

| I am adding… | Owner | Why |
|---|---|---|
| A new rule (what Rage does, what a condition does) | `../conditions`, `../combat`, `../features` — the rules packages | They know the rule; they do not know what a session is. This package drives them, it does not hold them. |
| A new die roll inside an interaction | here, in a machine | Machines roll with the roller they were handed. `saves.MakeSavingThrow` and `checks.MakeAbilityCheck` REQUIRE a bus (rpg-toolkit#1382) and this package is their only lawful supplier. |
| A new condition | `../conditions` + its loader | It is data on a sheet plus a behaviour that attaches. This package routes the blob and attaches it; it never learns the condition's name. |
| A reaction or follow-up a subscriber wants | `events.FollowUp` ([`../events/damage_taken.go:50`](../events/damage_taken.go)) | **SUBSCRIBERS DESCRIBE, MACHINES ROLL.** The subscriber appends data — settled DC, settled consequence — to the fact it heard; `runFollowUps` ([`damagetaken.go:131`](./damagetaken.go)) rolls it nested, in the same interaction. `ConcentratingCondition.onDamageTaken` ([`../conditions/concentrating.go:406`](../conditions/concentrating.go)) is the worked example. |
| A new cast shape | content first, then an arm in `newCast` | `newCast` ([`action.go:106`](./action.go)) reads the profile and splits: a profile with a `Save` gate goes to `newGatedCast` → `NewContest` ([`action.go:535`](./action.go)); one without goes to `newGatelessCast` → `NewActivation` ([`action.go:584`](./action.go)). Fit the shape or add an arm — never a spell name. |
| Something that must survive the call | `session` (and `encounter`'s record) | The bus dies with the call and this package opens no repository. What survives is what leaves in `Output`. |
| A new ambient fact a predicate needs | `installTruth` — as a design decision | One installer, on every path, inside no condition. A registry populated by the call sites that happen to need it is rpg-toolkit#1251, which fought a whole campaign at base AC and logged nothing. |

## Traps

- **A `gamectx` reader in a plain typed handler sees nothing.** Chain handlers
  are given the PUBLISHER's context, so they see the door's installs; a typed
  subscription closes over the context it was SUBSCRIBED with, which from where
  `installTruth` is called is nothing. It fails closed and logs nothing —
  rpg-toolkit#1251 said back to us. [`truth.go:51-59`](./truth.go).
- **A machine CAN reach the room at `Start`.** `installTruth(ctx, room, cast, enc)`
  runs at [`resolve.go:438`](./resolve.go), fourteen lines BEFORE
  `start(ctx, in.Machine, cast)` at [`resolve.go:452`](./resolve.go), so
  `gamectx.Room` is installed for the whole of pure preflight. "Resolution
  cannot see the map" is false and should not be repeated. The reason not to
  enumerate from it is the undeclared universe, not the absence of one.
- **A step returned as a pointer compiles.** `isStep()` has a value receiver, so
  `*Done` and `*Gather` satisfy `Step` and are refused at RUNTIME, not by the
  compiler. The vocabulary is the value forms.
- **A nil `Input.Cost` is a free action, not a missing one.** A profile that
  forgot to compile its price is indistinguishable here from a monster's innate
  cast. If the price matters, it is the caller who must not lose it.
- **`resolution.ErrBadAction` has no arm in `session`'s `translateResolution`**
  ([`../session/attack.go:572`](../session/attack.go)). `cast.go` names it by
  hand for the construction error only; an `ErrBadAction` raised DURING a run —
  `strike.go:196`, `contest.go:366/422/444` — falls through the default arm and
  reaches the host unchanged. It is also absent from `resolutionSentinels`, so
  nothing catches it.
- **A condition that rolls in its own handler is the OLD way.** It puts a rules
  decision in a subscription order and hides the roll from the record. Several
  survive, and several reach for a process-global roller when theirs is nil —
  `../features/deflect_missiles.go:186`, `../conditions/sneak_attack.go:230`.
  Bring them to `FollowUp` when you touch them; do not copy them.
- **Don't cite ADR-0007 for sealed enumerations.** Its actual subject is generic
  restoration triggers, and the link in [`doc.go`](./doc.go) points at a file
  that does not exist. Check an ADR's title before repeating a citation.
