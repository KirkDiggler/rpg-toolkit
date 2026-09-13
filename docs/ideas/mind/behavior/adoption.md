# mind/behavior — adoption in the D&D 5e encounter

**Status:** IN PROGRESS
**Why:** [rpg-toolkit#1725](https://github.com/KirkDiggler/rpg-toolkit/issues/1725).
**Rulings (Kirk, 2026-09-13):** the mind's name lives on the monster
definition; a shot is landed as a deed by the encounter when an attack
resolves.

## The scene

A bow skeleton shooting at the closest player turns on the one who shot
at it, and goes back to closest when that player puts the bow away and
closes to melee. A witnessed deed re-ranking a target, on a real board.

## The shape

Five modules, five PRs, published inside-out and each pinned to the tag
before it:

1. `mind/behavior` v0.3.0 — the store is the caller's (this record's
   home; see [implementation.md](implementation.md)).
2. `rulebooks/dnd5e` — `monster.Mind`, a typed constant on the sheet; the
   skeleton names the Retaliator.
3. `rulebooks/dnd5e/encounter` — the view carries the mind's name, every
   holding as values, the clock's high-water, and a step away beside the
   path; `Record` lands an attack deed on every member whose senses reach
   the attacker's cell.
4. `rulebooks/dnd5e/behavior` — the `Minded` driver, a view-backed `Space`,
   a `Reader` over the sight payload, and the Retaliator mind.
5. `rulebooks/dnd5e/session` — the name crosses at placement; the minded
   driver is constructed where the basic one is today.

## Rules

- **A1** — The store is the encounter's. Behaviour reads holdings as
  values and writes only through `stage.Store`; it never runs a pass.
- **A2** — The driver never reaches live state. `Space` and `Truth` are
  answered from the view, which the encounter fills from its canvas before
  it asks.
- **A3** — The encounter is the stage. Behaviour's intents are declared;
  the encounter validates a swing against truth and routes a walk.
  `stage.Aim` and `stage.Step` are for boards that have no encounter.
- **A4** — A deed is landed where the fact is known and nowhere else:
  `Record`, at the clock's high-water, on every member whose senses reach
  the actor's cell. A miss is still a shot at you.
- **A5** — A mind is named on the sheet and looked up by the driver. A
  member that names none gets the basic driver's answer.
- **A6** — The driver is one per session, and the host owns the cache.
  `session.Config.TurnDrivers` is asked once per verb for the session
  that verb is about; the stateless drivers keep `TurnDriver`, and
  exactly one of the two is set. The cache lives in the host because a
  session's lifetime does: the Manager is stateless per verb and gets no
  session-end signal, so a cache inside it would have no owner to evict
  it.

## Done when

- PRs 1–5 merged in order, each pinned to the tag before it.
- The scene walked on the local stack.
- The Retaliator's claims each kill a mutant: never attaches the deed,
  never tires, ignores the deed's target.
