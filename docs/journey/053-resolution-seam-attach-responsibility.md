# Journey 053: The Resolution Seam Arrives — Attach Responsibility Decides the Bus Question

**Date:** 2026-08-14
**Participants:** Kirk, Claude (session SDK lane)
**Status:** direction, deliberately not yet a decision. An ADR follows when we
are happy with it (Kirk's call, this conversation).
**Predecessor:** [052](052-event-bus-evolution-broadcast-chains-enumeration.md),
which ended by deferring: *"the stance gets its first consumer when the
resolution seam is designed."* This is that conversation — wave 4 of the
session SDK (toolkit #959, step 4.4 / #965).

## The occasion

Wave 4 builds combat into the new stack, and the new stack has a property the
old one never had: **the composition is entirely bus-free.** Zero `events.`
references in `rulebooks/dnd5e/encounter` outside tests. Every layer below the
session returns values — the `play/*` leaves by contract, the composition by
construction. Meanwhile `combat` is the inverse: `NewTurnManager` *requires* an
`EventBus` and errors without one, and publishes `TurnStartEvent` /
`TurnEndEvent` itself.

Something has to give, and the wave charter (plan.md, clause 2) marked it
*ponder, not deflect*. This doc records the pondering.

## Kirk's requirement, stated first because it constrains everything

> *"What I like most about the bus is we get to define the effect and it is
> applied and manages itself. Want to make a new effect? Write the function and
> attach it to a player, monster, anything that gets applied to the bus."*

That is the authorship model, and it is non-negotiable: one self-contained
effect object, owning its own predicate, its own chain stages, its own
persistence. Nobody else holds a registry of what dodging does. 052's census
confirmed this is not aspiration — Rage really is one object owning four chain
hooks and five lifecycle observers, and all ~26 modify sites repo-wide go
through disciplined `c.Add(stage, name, closure)` registrations.

Any resolution design that centralizes effect knowledge — a gatherer that
enumerates *kinds* of effects, a switch over conditions — fails this
requirement and also recreates the fail-open hand-maintained list
(rpg-project#218) at the worst possible seam.

## Kirk's caution, which turned out to be the deciding argument

> *"Be careful — everything that can attach would be responsible. Monsters need
> to attach too. Traps also would be attached."*

Under bus-subscription discovery, an effect contributes only if something
remembered to attach it. That responsibility is distributed across every load
path, and the inventory (verified in code, 2026-08-14) is already uneven:

| Entity | Attach path today | Shape |
|---|---|---|
| Character | `character.LoadFromData(ctx, data, bus)` | **one call, complete** — conditions re-attach as part of loading; a bus is required, so forgetting is impossible |
| Monster | `monster.LoadFromData` + `actions.LoadMonsterActions` + `monstertraits.LoadMonsterConditions` | **three calls** — `monster/README.md` warns that a partial load looks complete; a test calling only the first has not proven a round trip |
| Monster, at the session seam | `Spawn` stores `monster.Data` in `SessionData.NPCs` | **attaches nothing** — zero references; W3 stored the instance, no verb has yet put an NPC's traits on any bus |
| Trap | — | **no owner yet** — 052's census has the trap as the canonical *observation* listener, but nothing defines who attaches a trap's effects when a room loads |

The failure mode is the quiet kind: a monster whose traits never attached does
not error — it just fights without Pack Tactics, and encounter difficulty is
tuned against a stat block that lies. Every new entity kind (trap, hazard,
aura, environmental effect) adds another load path that must remember.

## What the conversation moved through

**Noodle 1 (Kirk): maybe combat is the composable data; maybe resolution has
the bus.** Checking `core/chain` confirmed the first half is already true —
`Chain[T]` is `Add / Remove / Execute`, zero bus knowledge, a pure fold. The
bus's only modification-era job is discovery ("who wants to contribute?"),
exactly as 052's census found.

**Noodle 2 (Claude): gather per phase over the call bus.** Load-act-save
already rebuilds subscriptions from persisted data on *every call* — that is
how conditions work today — so a suspended resolution can resume in a fresh
process by reloading participants and re-gathering. Phase boundaries persist as
data (the folded chain event is already pure data — `AttackModifierSource`
records, not closures). **Shield is the proof the re-gather is a feature:** a
reaction that adds +5 AC *after the attack roll is known, including against the
triggering attack*, is inexpressible under gather-once and free under
re-gather-at-the-resume-phase. The suspension freeze (every change verb
rejected while a window is open) is what makes the re-gather deterministic.

**Then 052's stance, re-read, amended noodle 2.** 052 had already concluded:
keep the bus for **observation**, hand **discovery** to enumeration — *"the
durable modifier registry already exists: it is the character's condition
list."* Each effect exposes its chain contributions through an interface; the
pipeline enumerates the participants' persisted effects instead of
broadcasting and hoping the right closures are listening.

Kirk's caution is what decides between broadcast-gather and
enumeration-gather, and it decides for enumeration: **it moves attach
responsibility from N load paths to one enumerate step.** Under enumeration, a
monster's traits contribute because they are *in its data*, not because a load
path remembered a third call. The table above stops being a risk register and
becomes a list of data shapes.

And Kirk's authorship model survives intact, because enumeration reads the
same self-contained objects: the effect still owns its predicate, stages, and
persistence — attached to a player, a monster, anything. Only *how it is
found* changes: read from the entity, not overheard on a wire.

**Noodle 3 (Kirk): everything is data, and resolution is the one place a bus
exists.** The reconciliation, and where this doc lands:

> *"What if the character was just data, monsters just data — both have an
> interface that says they should subscribe. We pass all the data objects into
> a resolution package which loads the data, takes the action, and is
> responsible for loading everything on the bus. … We need the bus, but that
> should be in a single spot and just load the data. This isn't just combat
> that needs the bus — all actions must go through the bus, all interactions."*

This keeps 052's discipline and the bus mechanism both. The subscribe
interface is the enumerable contract: **the data declares that it has
attachments; resolution alone honors that.** Discovery becomes enumeration one
level up — *who is in scope* — with bus self-registration below — *what each
contributes*. The attach inventory collapses from N load paths that must each
remember into one loop in one package: for each participant, attach. A trap
contributes because it was passed in.

Custody falls out cleanly: the **session** keeps the repositories and passes
data in, saves data out — it never holds a bus again (`loadCharacter` +
`newCallBus` migrate out of it). **Resolution** owns the bus for exactly one
call. The **composition** stays bus-free. And **combat becomes the rules
vocabulary**: stage definitions and their order, the phase shapes of an attack
(ADR-0027), damage pools and crit rules (ADR-0026/0036), action economy — the
chain definitions and pure functions resolution executes. Combat says what a
strike *is*; resolution makes one *happen*; the courier tells everyone it
*did*.

One duplication resolves itself before being built: `TurnManager` publishes
`TurnStartEvent`/`TurnEndEvent` today, but `play/clock` already returns
`TurnStarted`/`TurnEnded` as milestone values. Under this split those
publishes do not move — they disappear. The clock was always the source of
truth for turn lifecycle; combat was duplicating it onto a bus. Evidence the
split carves at a real joint.

And "all interactions" reframes #965 from combat-specific to
**interaction-generic**: a walk through a trapped corridor is an interaction;
alertness modifying perception is an interaction. Resolution is the seam for
all of them; combat is merely the first rules content it executes.

## The direction, assembled

- **Everything at the seams is data.** The session fetches participant data
  from its repositories, passes it in, saves what comes back. Characters,
  monsters, traps — data objects carrying an interface that says they have
  attachments.
- **Resolution — the ONE place a bus exists, for one call at a time.** It
  receives the participants, loads each onto its bus (the single attach site —
  Kirk's caution answered structurally), executes the interaction through the
  chains, and returns outcomes plus updated data. It is
  **interaction-generic**: combat is its first content, not its definition.
- **A phase machine whose boundaries are data.** Each phase runs against
  freshly loaded participants. Suspend = persist the folded-so-far event;
  resume = pass the data in again, re-attach, continue. Shield works *because*
  of the re-attach, not despite it; the suspension freeze makes it
  deterministic.
- **`combat` — the rules vocabulary.** Stages and their order, attack phases
  (ADR-0027), damage pools and crit rules (ADR-0026/0036), action economy.
  Chain definitions and pure functions. It sheds its required-bus construction
  and its turn-lifecycle publishing — the latter was duplicating `play/clock`'s
  milestones.
- **Effects — self-contained, per Kirk, unchanged.** Write one object; it
  subscribes itself when resolution attaches its owner. The authorship model
  is the fixed point every option was measured against.
- **Encounter — geometry, story, clocks. Still bus-free.** What a modifier
  needs from the world (Pack Tactics' adjacency, prone's 5-foot split) rides
  `gamectx` (ADR-0025 — verified: `room.go`, `combatant.go`,
  `reaction_readiness.go` already exist) into the chain functions. The
  composition *raises* interactions; resolution resolves them.
- **The bus — keeps observation**, exactly as 052 said, inside resolution's
  walls. Facts for optional listeners; the broker bridging to clients.

## The edges the decision must survive

Named here because "do we have our edges?" (Kirk) is the right gate for an
ADR, and because the subscribe-interface options genuinely differ on some of
these. Hardest first:

- **The aura.** A paladin's aura modifies *other creatures'* saves, so
  "participants = attacker + target" is wrong: enumeration scope must include
  proximate entities whose effects project onto others. The strongest argument
  for the composition *raising* interactions — it is the only layer that knows
  who is near.
- **Concentration.** Damage to the caster ends an effect on someone else — an
  interaction's writes can ripple beyond its participants.
- **Shield.** Covered above: re-attach at resume; the freeze makes it
  deterministic.
- **The trap, in free roam and in combat.** Same seam, different clock — one
  code path. Detection is already composition-side machinery:
  `TriggerReachedPosition` is evaluated per step against the destination cell
  (verified — the ending-evaluation loop in Move). A trap is the same
  detection with a different consequence: raise an interaction instead of
  closing the encounter. The trap never sits subscribed anywhere waiting —
  the difference from the old stack, where 052's census had the trap as the
  canonical long-lived movement listener.
- **A monster's action on the tick.** Pump's decider intents will raise
  interactions too (the monster steps on the trap; the monster attacks, which
  is #964's mutual-awareness trigger). The seam must be caller-agnostic:
  session verb or Pump, same resolution.
- **Unconscious mid-walk.** #845 already files that free roam has no HP gate;
  the interaction seam is where that gate naturally lands.

**And the simulation-server question, answered while it was asked (Kirk,
2026-08-14):** free-roam trap triggering does NOT need a resident world
process. Detection is world data consulted per step; resolution is a one-call
bus. What a sim server would buy — the world acting while nobody plays — is
against the composition's own charter ("player activity pumps the clock, the
world thinks on the tick"), and everything else it provides, load-act-save
provides more honestly: attachment rebuilt from data every call (~5µs a
character, measured), suspension surviving process death, no
resubscribe-after-crash problem — which is the attach caution in its worst
form. If wall-clock simulation is ever wanted, that is a *driver* choice —
a host calling Pump on a schedule — not a toolkit architecture change.

**The per-click cost, measured rather than argued (2026-08-14):** the concern
"free roam means load-act-save on every click" is real and quantified. One
full cycle — JSON-decode a 2.7KB blob, `LoadEncounter` on a tomb-shaped world
(3 rooms, walls, doors, 6 members), walk a 4-cell path with per-step trigger
evaluation, `ToData`, re-encode — costs **~187µs / 67KB / 982 allocs**
(benchmarked in-package). With Redis round trips, ~1ms per click server-side,
under a 30–100ms client RTT: two orders of magnitude below perception. One
core sustains ~5k clicks/sec; a six-player party clicking every second uses
~0.1% of it. Two structural facts keep the rate low: **a click is a path, not
a cell** (`Move` walks the whole path in one verb), and the blob stays small
because retention bounds the story log. The escape hatch if scale ever changes
is already written into the contract — `EncounterRepository`'s doc: *"an
encounter held in memory on a live server and checkpointed periodically is
invisible from here."* A host-side RAM repository with write-through IS the
simulation server, without the toolkit ever knowing. rpg-api already ships
load-per-RPC today (Chapter 1's "one private load(id) per orchestrator
method"), so this pattern is in prod, not hypothetical.

## Open before an ADR

1. **The subscribe interface's shape** — the seam decision. What does a data
   object expose so resolution can attach it: per-chain-type methods, a single
   typed dispatch, or stage-declared contributions (052's original three),
   now posed as "what does *anything attachable* implement" rather than "what
   does a condition implement". Genuine options and trade-offs before choosing
   (ADR-0037's process note). **The route to the ADR:** one options document,
   each candidate tested against the edge list above — the aura especially,
   since scoping is where the shapes actually differ — then Kirk picks, and
   ADR-0038 records the winner with the rejects.
2. **Where an interaction begins.** A plain geometry move loads nobody today,
   and probably still should not. Lean: the composition walks and *raises* an
   interaction when one occurs (a trap cell, first contact); resolution
   resolves it with exactly the participants involved — the interrupt
   architecture doing what it was built for. Undecided, and it bounds
   resolution's cost per verb.
3. **Heterogeneous loading is ADR-0037 again.** Resolution receives mixed
   data — characters, monsters, traps — and routing each to its loader is
   what a ref is for: *a ref names the package that can load the data*. Same
   seam `Spawn`'s `instantiate` already routes on. The monster three-call
   assembly wants collapsing behind one loader *anyway*; this gives it the
   reason.
4. **The probe that gates suspension points:** verify each phase boundary's
   chain output is fully self-describing data. Any phase whose intermediate
   state holds a closure or live object cannot be a suspension point, and the
   boundaries move. First thing #965 should do.
5. **Whether `ReactionTriggerEvent` survives** — it exists to compensate for
   straight-line resolution; a real suspension replaces it (052's question,
   now concrete in wave 5's scope).

## Pointers

- [052](052-event-bus-evolution-broadcast-chains-enumeration.md) — the census
  and the stance this ratifies-with-amendment.
- `docs/ideas/session-sdk/plan.md` — wave 4 charter (clauses 2 and 3 are the
  ones this serves).
- Toolkit #965 (resolution), #964 (trigger — mutual awareness, Kirk
  2026-08-14, revisable), #959 (wave tracker).
- `rulebooks/dnd5e/conditions/dodging.go` — the effect shape everything above
  must keep working.
- `rulebooks/dnd5e/session/entities.go` — `newCallBus`, `loadCharacter`, and
  the no-session-process comments that make load-act-save's guarantees
  explicit.
