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
// and is harder to debug than a clean load error.
func (e *Encounter) hydrateCombatants(ctx context.Context) error {
	for _, pd := range e.data.Players {
		if len(pd.DataJSON) == 0 {
			continue // no blob — resolver uses the stat snapshot
		}
		char, err := e.hydratePlayer(ctx, pd)
		if err != nil {
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
			return fmt.Errorf("hydrate monster %q: %w", md.ID, err)
		}
		e.combatants[md.ID] = mon
	}
	return nil
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

// applyPlayerReactionConditions applies the default reaction conditions (OA +
// Shield) to a hydrated character, driven by the encounter-owned
// ReactionReadiness map. #689 folds this in from rpg-api's reaction_conditions.go
// so the host stops constructing conditions.New* per attack.
//
//   - Opportunity Attack: applied when the entity has OA seeded ready (melee
//     combatants get it default-on at AddPlayer). The condition's own predicate
//     (gamectx.IsReactionReady + leaving threatened reach) gates whether it ever
//     publishes a trigger; applying it is harmless for non-melee characters.
//   - Shield: applied only for spellcasters (1st-level slot heuristic). Default
//     readiness is OFF; the player opts in via SetReactionReady.
func (e *Encounter) applyPlayerReactionConditions(ctx context.Context, char *character.Character) error {
	id := char.GetID()
	if e.reactionSeeded(core.EntityID(id), OAReactionRef) {
		oa := conditions.NewOpportunityAttackCondition(id)
		if err := oa.Apply(ctx, e.bus); err != nil {
			return fmt.Errorf("apply OA condition for %q: %w", id, err)
		}
	}
	if hasFirstLevelSpellSlot(char) {
		shield := conditions.NewShieldSpellCondition(id)
		if err := shield.Apply(ctx, e.bus); err != nil {
			return fmt.Errorf("apply Shield condition for %q: %w", id, err)
		}
	}
	return nil
}

// applyMonsterReactionConditions applies the monster's default reaction
// conditions (OA only — monsters don't cast Shield), gated by the readiness
// map. Without this the monster's OpportunityAttackCondition.onMovementChain
// subscriber never fires for NPC-OA-on-player-fleeing.
func (e *Encounter) applyMonsterReactionConditions(ctx context.Context, mon *monster.Monster) error {
	id := mon.GetID()
	if e.reactionSeeded(core.EntityID(id), OAReactionRef) {
		oa := conditions.NewOpportunityAttackCondition(id)
		if err := oa.Apply(ctx, e.bus); err != nil {
			return fmt.Errorf("apply OA condition for monster %q: %w", id, err)
		}
	}
	return nil
}

// reactionSeeded reports whether the named reaction has an entry (true or false)
// in the entity's readiness map. OA is seeded (=true) for melee combatants at
// AddPlayer/AddMonster; an entry's presence is the signal that the condition is
// relevant for this entity. Absence means "never wired" → skip the Apply.
func (e *Encounter) reactionSeeded(id core.EntityID, reactionRef string) bool {
	m, ok := e.data.ReactionReadiness[id]
	if !ok {
		return false
	}
	_, present := m[reactionRef]
	return present
}

// hasFirstLevelSpellSlot reports whether the character has at least one
// 1st-level spell slot (max > 0) — the eligibility heuristic for applying the
// Shield reaction condition. Folded in from rpg-api's reaction_conditions.go.
func hasFirstLevelSpellSlot(char *character.Character) bool {
	if char == nil {
		return false
	}
	data := char.ToData()
	if data == nil || data.SpellSlots == nil {
		return false
	}
	slot, ok := data.SpellSlots[1]
	if !ok {
		return false
	}
	return slot.Max > 0
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
// IsDirty() does not cover condition state, so code wins — see the ADR.)
//
// Cost: one JSON marshal per held combatant per ToData (per RPC). Modest, and
// correctness-critical. MarkClean is still called so any HP-dirty tracking by
// other consumers stays consistent.
//
// Serialization errors are swallowed per-entity: a failed write-back keeps the
// prior blob (status-quo before this mutation), safer than failing a verb that
// produced a valid in-memory result; the next load re-derives from the prior
// blob.
func (e *Encounter) syncCombatantsToData() {
	for _, pd := range e.data.Players {
		c, ok := e.combatants[pd.EntityID].(*character.Character)
		if !ok || c == nil {
			continue
		}
		if raw, err := json.Marshal(c.ToData()); err == nil {
			pd.DataJSON = raw
			c.MarkClean()
		}
	}
	for _, md := range e.data.Monsters {
		m, ok := e.combatants[md.ID].(*monster.Monster)
		if !ok || m == nil {
			continue
		}
		if raw, err := json.Marshal(m.ToData()); err == nil {
			md.DataJSON = raw
			m.MarkClean()
		}
	}
}

// combatantFor returns the held runtime entity for the given id, or nil if the
// entity was not hydrated (no DataJSON). Used by the verbs to populate the
// resolver inputs (AttackInput.Attacker/Defender, MovementStepInput.Mover) so
// the resolver uses the held entity and never re-loads.
func (e *Encounter) combatantFor(id core.EntityID) combat.Combatant {
	return e.combatants[id]
}
