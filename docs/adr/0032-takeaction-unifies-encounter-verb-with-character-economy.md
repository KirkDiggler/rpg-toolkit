# ADR-0032: TakeAction unifies the encounter verb path with the character action economy/menu

Date: 2026-06-01

## Status

Accepted

## Context

The TakeAction wave (rpg-project#54 / rpg-toolkit#697, Beat 1) had to make a
level-1 character take an **action plus a bonus action** with the economy
enforced server-side, the menu computed by the toolkit, and the full story on
the event spine. Two toolkit worlds had grown up in parallel and never met:

- The **encounter verb path** (`encounter/combat_phased.go`) only knew
  `"attack"`. A hard gate — `if ref.ID != "attack" { return ErrUnsupportedAction }`
  — rejected every other ref, and attacks resolved off the flat stat snapshot on
  `PlayerData` (`AttackBonus`, `DamageDice`), never touching a character's
  economy or menu.
- The **rich two-level action economy/menu** lived in the **character package**
  (`rulebooks/dnd5e/character`): `ActivateAbility` / `ExecuteAction` validate and
  deduct, `AvailableAbilities` / `AvailableActions` compute the menu from granted
  capacity. The encounter path never consulted it.

The wave doc named "reconcile these two worlds and decide which is canonical" as
the largest piece of the work. Separately, rpg-api was injecting
`ActionEconomyData{1,1,1,Movement:30}` itself at turn start because nothing in
the engine seeded it — a North-Star Invariant 2 violation (the host authoring
rules state).

## Decision

**The character package's existing dispatch is canonical. The encounter
delegates to it; it does not build a parallel registry.** ("Default to one
system.")

1. **Delete the attack gate; delegate generally.** The `ref.ID != "attack"` gate
   is replaced by a branch: the `attack` ref keeps its dedicated two-phase
   resolver path (it carries reaction prompts and damage resolution); every other
   ref flows to `Encounter.takeCharacterAction`, which routes to the held
   character's own engine — `ActivateAbility` for abilities/features,
   `ExecuteAction` for granted-capacity actions. No ref is enumerated in the
   encounter: the character's menu *is* the membership test. An unknown ref on a
   hydrated character returns `ErrUnsupportedAction`; a non-attack ref on a flat
   stat-snapshot seat (no hydrated character) returns `ErrNonCombatant`.

2. **Run on the held character, never re-load.** The action runs on the
   `*character.Character` the `LoadFromData` cascade already hydrated onto
   `e.bus` (ADR-0030 / #689), reached via a type-assert on the held combatant —
   not a fresh `LoadFromData`. Re-loading would re-`Apply` the character's
   conditions to the bus (the #684 double-subscribe class).

3. **The engine owns turn-start seeding.** `Encounter` calls
   `character.StartTurn` on the held character at each turn boundary (the
   `SetMode→TURN_BASED` first actor and `EndTurn`'s next actor), seeding the
   economy {1 action, 1 bonus, 1 reaction, movement = speed}. This removes the
   rpg-api `ActionEconomyData{1,1,1}` injection (Invariant 2).

4. **The menu is exposed as data.** `Encounter.ActorTurnState(actorID)` returns
   the held character's `AvailableAbilities` / `AvailableActions` + economy as
   toolkit domain types. Each menu entry carries an `EconomySlot` (for grouping)
   and a `TargetKind` (so a UI raises the right prompt) — both toolkit-authored
   enums the game server projects field-for-field (Invariant 11). `TargetKind`
   distinguishes `SELF` (Dodge — targets the actor) from `NONE` (Dash —
   deliberately untargeted, no prompt); `UNSPECIFIED` is the not-set defect value.

5. **Attack becomes a citizen of the two-level economy.** When a hydrated
   character takes the attack verb, the verb drives `ActivateAbility(attack)`
   (spend the action, grant attacks) then `ExecuteAction(strike)` (consume an
   attack, fire post-strike grants), and reports the real pre/post economy diff
   on the resolved-action event. Hit/damage resolution is unchanged (still the
   two-phase resolver). A flat stat-snapshot seat (no character) falls back to
   the honest one-action cost.

6. **Every action emits its own resolved-action event with the real ref.** The
   non-attack path publishes a first-class `ActionResolvedEvent` (the umbrella
   beat, Invariant 9). The attack path threads the actor's **real submitted ref**
   (not a placeholder constant) and the real economy consumed into its event.

### Sub-decision: the encounter composes effective-availability for beat-deferred refs (D17)

The wire `AvailableAction.available` means **effective takeability** ("should the
UI offer this now?"), not raw rules-permission — the web renders availability and
never computes it. The character menu reports rules truth: a level-1 character
*can* move, so `Move.CanUse == true`. But this build does not resolve movement yet
(wave decision D5; Beat 2). Rather than make the character lie, **the encounter
composes the beat scope it owns** on top of the honest character menu
(`applyDeferredActions` in `ActorTurnState`): any ref in a small
`deferredActionRefs` set is projected `available=false` with an honest reason
("movement lands in Beat 2"), and the verb rejects the same refs with
`ErrActionDeferred`. Menu and verb stay consistent — the server never promises
what it won't do, and the web needs no logic to hide the entry.

This is a set, not a hardcoded `if id == "move"`, so Beat 2 is a one-line removal
and the mechanism generalizes to any future beat-deferred ref. It is **not** the
attack-gate backslide: that gate was a permanent "only attack works" block; this
is a temporary, reasoned, beat-scoped marker with an honest reason, removed in
Beat 2 by dropping the entry.

### Sub-decision: Monk Martial Arts bonus strike spends the bonus action

The Monk's Martial Arts unarmed strike (PHB p.78: "you can make one unarmed
strike as a bonus action") is granted as capacity after a main-hand strike when
a bonus action remains. `executeUnarmedStrike` consumed that granted capacity but
**never decremented `BonusActionsRemaining`** — so the bonus-action slot was
silently never spent, and a Monk appeared to still have a bonus action after
using it. Because the bonus-action cost is a *rule*, the fix lives in the
character package: `executeUnarmedStrike` now spends the bonus-action slot when
it consumes the martial-arts capacity. This is what makes the bonus-action axis
of the Beat-1 goal behavior (action + bonus action, both enforced) hold.

## Consequences

- The encounter verb path is general: adding a new action ref is data entry in
  the character package, not a new gate in the encounter (Beat 2 catalog).
- The held-character delegation keeps the single-subscribe discipline (ADR-0030)
  intact — one hydration, one bus, the resolver and the verb share the held
  instance.
- rpg-api stops seeding the economy and stops authoring the menu; it projects
  `ActorTurnState` field-for-field.
- **Non-attack catalog complete:** Dodge, Dash, Disengage, **Help, and Hide** are
  all seeded on every character and flow through the unified verb. Help/Hide
  constructors landed in the dnd5e half (rpg-toolkit#702, v0.61.0); the encounter
  module bumped to v0.61.0 and a real-path test proves both flow through
  `TakeAction` with the right target-kind + standard-action enforcement.
- **Known gaps left as follow-ups under #54 (not regressions of this wave):**
  - The combat-ability *mechanical effects* for Dodge / Help / Hide are not yet
    applied: Dodge publishes `DodgeActivatedEvent` but nothing wires it to
    `DodgingCondition.Apply` (attackers' disadvantage); Help publishes
    `HelpActivatedEvent` with an empty `AllyID` (the ally is not threaded through
    `ActivateAbility`) and nothing grants the advantage; Hide publishes
    `HideActivatedEvent` but nothing resolves the Stealth check or applies Hidden.
    Beat 1 proves the *dispatch* generality + economy spend, not these full
    effects.
  - Two implementations of the Monk bonus strike coexist (the character-package
    `GrantedMartialArtsBonus` path used here, and the weapon-aware
    `actions.MartialArtsBonusStrike` granter); they are not yet collapsed.
  - The phased-attack resume path (`CompleteTakeAction`) reports the attack
    default ref/economy because `PhasedAttackContext` does not carry the
    submitted ref; reactions are a later wave.
  - Class features (Flurry of Blows, Patient Defense, Step of the Wind) return
    `TargetKindUnspecified` — they are not yet classified in `targetKindForRef`.

## Related

- ADR-0030 — encounter owns combatant hydration (the held-instance cascade this
  delegation rides on).
- ADR-0031 — the event spine (causation + game-event time + `ActionResolvedEvent`)
  this wave makes real for every action.
