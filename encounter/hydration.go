package encounter

// #689 — combatant hydration cascade.
//
// Hydration is NOT a new subsystem: it is the toolkit's existing
// ToData/LoadFromData round-trip, which composes. Encounter.LoadFromData
// cascades into each combatant's own LoadFromData (character.LoadFromData /
// monster.LoadFromData), holds the runtime entities as combat.Combatant, and
// applies their persistent conditions + default reaction conditions onto the
// encounter bus ONCE. That single cascade is the only place conditions Apply()
// to e.bus — the subscribe-exactly-once cure for the #684 double-subscribe
// class (previously the host loaded entities per attack + per turn-end, each
// re-Apply'ing conditions to the same bus).
//
// ToData mirrors the cascade: held entities whose state mutated (IsDirty) are
// re-serialized back into their owning PlayerData/MonsterData.DataJSON so the
// next load sees current state, replacing the host's scattered save-back.
//
// Boundary note (Kirk's ratified call, design plan §0): the encounter SDK is
// dnd5e-coupled by precedent (npc.go already calls monster.LoadFromData;
// activate_feature.go already calls character.LoadFromData; encounter.go
// imports dnd5eEvents). This cascade makes that coupling coherent and
// single-sourced rather than scattered across the host. "Fully agnostic
// engine" remains separately tracked.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	monsteractions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
)

// hydrateCombatants is the LoadFromData cascade. It walks every player and
// monster seat and rehydrates the ones carrying rehydratable data, holding the
// runtime entity in e.combatants and applying its conditions (persistent +
// default reactions) onto e.bus exactly once. Seats without rehydratable data
// are skipped — the resolver falls back to its stat-snapshot stand-in for them.
//
// A single per-entity hydration failure is fatal: a half-hydrated encounter
// (some combatants on the bus, others not) would resolve attacks inconsistently
// and is harder to debug than a clean load error. On failure we explicitly
// Cleanup the already-hydrated entities (unwinding their bus subscriptions)
// before returning, then LoadFromData discards e. The explicit unwind is
// defense-in-depth: the new e.bus is unreferenced on the failure path and would
// be GC'd anyway, but unwinding keeps the failure path honest and safe against a
// future change that recovers from / reuses a partially-loaded encounter.
func (e *Encounter) hydrateCombatants(ctx context.Context) error {
	for _, pd := range e.data.Players {
		if len(pd.DataJSON) == 0 {
			continue // no blob — resolver uses the stat snapshot
		}
		char, err := e.hydratePlayer(ctx, pd)
		if err != nil {
			e.cleanupHydrated(ctx)
			return fmt.Errorf("hydrate player %q: %w", pd.EntityID, err)
		}
		e.combatants[pd.EntityID] = char
	}

	for _, md := range e.data.Monsters {
		if len(md.DataJSON) == 0 {
			continue // no blob — resolver uses the stat snapshot
		}
		mon, err := e.hydrateMonster(ctx, md)
		if err != nil {
			e.cleanupHydrated(ctx)
			return fmt.Errorf("hydrate monster %q: %w", md.ID, err)
		}
		e.combatants[md.ID] = mon
	}
	return nil
}

// cleanupHydrated unwinds each held entity's own bus subscriptions (the ones
// its LoadFromData applied) and drops it from the map, best-effort, on the
// hydrateCombatants error path. This is defense-in-depth: LoadFromData discards
// the partially-hydrated encounter and its fresh, now-unreferenced e.bus is
// GC'd, so the subscriptions are already harmless — fatality comes from
// discarding e, not from this unwind. Note Cleanup only removes the entity's
// OWN conditions; the OA reaction we Apply separately onto e.bus is not unwound
// here, but that too dies with the discarded bus. Errors are ignored — the
// encounter is about to be thrown away.
func (e *Encounter) cleanupHydrated(ctx context.Context) {
	for id, c := range e.combatants {
		switch ent := c.(type) {
		case *character.Character:
			_ = ent.Cleanup(ctx)
		case *monster.Monster:
			_ = ent.Cleanup(ctx)
		}
		delete(e.combatants, id)
	}
}

// hydratePlayer rehydrates a *character.Character from PlayerData.DataJSON onto
// e.bus, then applies the player's default reaction conditions per the
// encounter-owned ReactionReadiness map. character.LoadFromData itself cascades
// the persisted conditions (conditions.LoadJSON + Apply) — see
// rulebooks/dnd5e/character/data.go.
func (e *Encounter) hydratePlayer(ctx context.Context, pd *PlayerData) (*character.Character, error) {
	var data character.Data
	if err := json.Unmarshal(pd.DataJSON, &data); err != nil {
		return nil, fmt.Errorf("unmarshal character data: %w", err)
	}
	char, err := character.LoadFromData(ctx, &data, e.bus)
	if err != nil {
		return nil, fmt.Errorf("load character: %w", err)
	}
	if err := e.applyPlayerReactionConditions(ctx, char); err != nil {
		return nil, err
	}
	return char, nil
}

// hydrateMonster rehydrates a *monster.Monster from MonsterData.DataJSON onto
// e.bus: monster.LoadFromData + LoadMonsterActions + LoadMonsterConditions
// (monster.LoadFromData deliberately does not cascade conditions — caller
// applies them, see rulebooks/dnd5e/monster/monster.go) + the monster's default
// reaction conditions. The MonsterData snapshot's authoritative HP/AC/Speed are
// synced onto the deserialized data before load.
func (e *Encounter) hydrateMonster(ctx context.Context, md *MonsterData) (*monster.Monster, error) {
	var data monster.Data
	if err := json.Unmarshal(md.DataJSON, &data); err != nil {
		return nil, fmt.Errorf("unmarshal monster data: %w", err)
	}
	syncMonsterDataFromSnapshot(&data, md)

	mon, err := monster.LoadFromData(ctx, &data, e.bus)
	if err != nil {
		return nil, fmt.Errorf("load monster: %w", err)
	}
	if err := monsteractions.LoadMonsterActions(mon, data.Actions); err != nil {
		return nil, fmt.Errorf("load monster actions: %w", err)
	}
	if err := monstertraits.LoadMonsterConditions(ctx, mon, data.Conditions, e.bus, e.roller); err != nil {
		return nil, fmt.Errorf("load monster conditions: %w", err)
	}
	if err := e.applyMonsterReactionConditions(ctx, mon); err != nil {
		return nil, err
	}
	return mon, nil
}

// applyPlayerReactionConditions applies Opportunity Attack to a hydrated
// character. #689 folds this in from rpg-api's reaction_conditions.go so the
// host stops constructing conditions.New* per attack.
//
// OA only — Shield is intentionally NOT applied. The four shipped level-1
// classes (Barbarian, Fighter, Monk, Rogue) include no Shield caster (it is a
// wizard spell), so auto-applying ShieldSpellCondition would wire a reaction we
// never implement. "Only build what we need." (The toolkit's
// ShieldSpellCondition stays available in the conditions library, just not
// auto-applied by this cascade.)
//
// Subscribe-time vs fire-time (verified against the condition impl): Apply()
// only SUBSCRIBES the condition to the bus — it is a silent no-op until the
// condition's own predicate fires. OpportunityAttackCondition
// (opportunity_attack.go:163) gates firing on gamectx.IsReactionReady, which
// reads the encounter-owned ReactionReadiness map at attack time. So readiness
// is a FIRE-time concern the condition already enforces; it is NOT a
// subscribe-time gate. OA is applied universally, matching the original host
// behavior + the condition's documented "universal for melee combatants"
// contract; the fire predicate makes it a no-op for non-melee / not-readied
// characters. OA readiness is seeded default-on for melee combatants at
// AddPlayer.
func (e *Encounter) applyPlayerReactionConditions(ctx context.Context, char *character.Character) error {
	id := char.GetID()
	oa := conditions.NewOpportunityAttackCondition(id)
	if err := oa.Apply(ctx, e.bus); err != nil {
		return fmt.Errorf("apply OA condition for %q: %w", id, err)
	}
	return nil
}

// applyMonsterReactionConditions applies the monster's default reaction
// conditions (OA only — monsters don't cast Shield). Applied universally for the
// same reason as players: Apply only subscribes; the OA fire predicate gates on
// IsReactionReady (seeded default-on for melee monsters at AddMonster). Without
// this the monster's OpportunityAttackCondition.onMovementChain subscriber never
// exists, so NPC-OA-on-player-fleeing never fires.
func (e *Encounter) applyMonsterReactionConditions(ctx context.Context, mon *monster.Monster) error {
	id := mon.GetID()
	oa := conditions.NewOpportunityAttackCondition(id)
	if err := oa.Apply(ctx, e.bus); err != nil {
		return fmt.Errorf("apply OA condition for monster %q: %w", id, err)
	}
	return nil
}

// syncCombatantsToData is the ToData mirror of the hydration cascade: for each
// held entity, re-serialize its ToData() back into the owning
// PlayerData/MonsterData.DataJSON so the next load sees current state.
//
// Why NOT gated on IsDirty(): the rulebook entities' dirty flag tracks only
// HP-shaped mutations (character.go sets dirty on ApplyDamage), NOT condition
// state changes such as SneakAttack.UsedThisTurn — which is exactly the
// per-turn state #689 must round-trip for cross-RPC once-per-turn enforcement.
// Gating on the current IsDirty() would silently drop those condition flips
// (verified by reading character.go: c.dirty is set only on HP change). So the
// write-back is unconditional for held entities. character.ToData() already
// re-serializes every held condition via condition.ToJSON(), so the current
// condition state is captured. (Design plan said "dirty-gated"; the shipped
// IsDirty() does not cover condition state, so code wins — see ADR-0030.)
// Extending IsDirty() to cover condition mutations so this can be
// efficiency-gated is the future enhancement tracked in toolkit#692.
//
// Cost: one JSON marshal per held combatant per ToData (per RPC). Modest, and
// correctness-critical. MarkClean is still called so any HP-dirty tracking by
// other consumers stays consistent.
//
// Serialization errors are NOT silently dropped. A failed write-back keeps the
// prior blob for that entity (so we never persist a half-marshaled entity), but
// the error is collected and returned so ToData can surface it via SyncErr() —
// a dropped write-back is a correctness regression (lost UsedThisTurn etc.) and
// must be observable. ToData keeps its pure *Data signature (many consumers call
// enc.ToData() inline); callers that care about persistence integrity check
// enc.SyncErr() after ToData and before Save.
func (e *Encounter) syncCombatantsToData() error {
	var errs []error
	for _, pd := range e.data.Players {
		c, ok := e.combatants[pd.EntityID].(*character.Character)
		if !ok || c == nil {
			continue
		}
		raw, err := json.Marshal(c.ToData())
		if err != nil {
			errs = append(errs, fmt.Errorf("marshal player %q: %w", pd.EntityID, err))
			continue
		}
		pd.DataJSON = raw
		c.MarkClean()
	}
	for _, md := range e.data.Monsters {
		m, ok := e.combatants[md.ID].(*monster.Monster)
		if !ok || m == nil {
			continue
		}
		raw, err := json.Marshal(m.ToData())
		if err != nil {
			errs = append(errs, fmt.Errorf("marshal monster %q: %w", md.ID, err))
			continue
		}
		md.DataJSON = raw
		m.MarkClean()
	}
	return errors.Join(errs...)
}

// combatantFor returns the held runtime entity for the given id, or nil if the
// entity was not hydrated (no DataJSON). Used by the verbs to populate the
// resolver inputs (AttackInput.Attacker/Defender, MovementStepInput.Mover) so
// the resolver uses the held entity and never re-loads.
func (e *Encounter) combatantFor(id core.EntityID) combat.Combatant {
	return e.combatants[id]
}
