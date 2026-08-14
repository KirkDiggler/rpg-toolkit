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

## The direction, assembled

- **`combat` — rules as values.** Stages, chains, phases. `core/chain` is
  already bus-free; the package sheds its required-bus construction as part of
  wave 4 (the *ponder* is how much of its ~45 observe sites are genuinely
  notification, which the courier takes over).
- **Effects — self-contained, per Kirk.** Write one object; it declares its
  contributions. Enumerable because it is persisted on the entity it affects.
- **Resolution — a phase machine whose boundaries are data.** Each phase
  enumerates the participants' effects against freshly loaded state. Suspend =
  persist the folded-so-far event; resume = reload, re-enumerate, continue.
  Shield works *because* of the re-enumerate, not despite it.
- **Encounter — geometry, story, clocks. Still bus-free.** What a modifier
  needs from the world (Pack Tactics' adjacency, prone's 5-foot split) rides
  `gamectx` (ADR-0025 — verified: `room.go`, `combatant.go`,
  `reaction_readiness.go` already exist) into the chain functions.
- **The bus — keeps observation**, exactly as 052 said. Facts for optional
  listeners; the trap listening to movement; the broker bridging to clients.

## Open before an ADR

1. **The contribute interface's shape** — 052's open question, still open:
   per-chain-type methods, a single typed dispatch, or stage-declared. This is
   the seam decision; genuine options and trade-offs before choosing
   (ADR-0037's process note).
2. **The probe that gates suspension points:** verify each phase boundary's
   chain output is fully self-describing data. Any phase whose intermediate
   state holds a closure or live object cannot be a suspension point, and the
   boundaries move. First thing #965 should do.
3. **Monster and trap enumeration paths** — the table's empty cells. The
   monster three-call assembly wants collapsing behind one seam *anyway*; this
   gives it the reason.
4. **Whether `ReactionTriggerEvent` survives** — it exists to compensate for
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
