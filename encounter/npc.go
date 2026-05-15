package encounter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	dnd5events "github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	monsteractions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// NPCAct executes a single turn for the named NPC. It rehydrates the
// monster from MonsterData.DataJSON onto a per-call dnd5e bus, builds
// monster.TurnInput from encounter state, subscribes a temporary listener
// to capture the dnd5e events the action publishes, and re-publishes them
// as encounter-scoped per-viewer events.
//
// For each captured dnd5e.AttackEvent, the encounter SDK resolves hit/damage
// by delegating to the wired CombatResolver — the same resolver used by the
// player-attack path (TakeAction). NPCAct returns ErrNoCombatResolver if no
// resolver has been wired via WithCombatResolver.
//
// Position is updated from TurnResult.Movement; a MoveEvent is emitted
// per-viewer for the NPC's path.
//
// Does NOT auto-cycle the turn — orchestrator calls EndTurn(npcID) next.
func (e *Encounter) NPCAct(ctx context.Context, npcID encountercore.EntityID) error {
	if e.data.Mode == encountercore.ModeEnded {
		return ErrEncounterEnded
	}
	if e.data.Mode != encountercore.ModeTurnBased {
		return ErrNotTurnBased
	}
	if active := e.ActiveActor(); active != npcID {
		return fmt.Errorf("%w: active=%q got=%q", ErrNotYourTurn, active, npcID)
	}
	mon, ok := e.data.Monsters[npcID]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownTarget, npcID)
	}
	if e.combatResolver == nil {
		return ErrNoCombatResolver
	}
	if len(mon.DataJSON) == 0 {
		// No rehydratable monster — fall back to a minimal scripted attack
		// against the closest reachable player, using the monster's stored
		// AttackBonus / DamageDice / DamageType. This keeps the verb
		// non-blocking when the orchestrator only seeded the snapshot fields.
		return e.npcActScripted(ctx, mon)
	}

	// Wave 2.11d: use the encounter-scoped bus instead of a fresh per-call
	// bus so post-roll subscribers (Shield spell condition Apply'd by rpg-api
	// at player rehydration) see NPC attacks against players. Without this,
	// NPC vs player attacks would never fire player reaction prompts.
	bus := e.bus
	captured, unsubAttack, err := subscribeAttacks(ctx, bus)
	if err != nil {
		return fmt.Errorf("subscribe dnd5e attack: %w", err)
	}
	defer func() { _ = unsubAttack() }()

	capturedDmg, unsubDmg, err := subscribeDamage(ctx, bus)
	if err != nil {
		return fmt.Errorf("subscribe dnd5e damage: %w", err)
	}
	defer func() { _ = unsubDmg() }()

	capturedCond, unsubCond, err := subscribeConditions(ctx, bus)
	if err != nil {
		return fmt.Errorf("subscribe dnd5e condition: %w", err)
	}
	defer func() { _ = unsubCond() }()

	var data monster.Data
	if err := json.Unmarshal(mon.DataJSON, &data); err != nil {
		return fmt.Errorf("unmarshal monster data: %w", err)
	}
	// MonsterData is the encounter SDK's authoritative state; the
	// serialized monster.Data may be stale (e.g. HP after damage from a
	// prior turn lives only on MonsterData). Sync the volatile fields
	// from MonsterData onto data before LoadFromData so the loaded
	// *Monster sees current HP / AC / Speed and so its targeting / AI
	// scoring use the authoritative numbers.
	syncMonsterDataFromSnapshot(&data, mon)
	loaded, err := monster.LoadFromData(ctx, &data, bus)
	if err != nil {
		return fmt.Errorf("load monster: %w", err)
	}
	if err := monsteractions.LoadMonsterActions(loaded, data.Actions); err != nil {
		return fmt.Errorf("load monster actions: %w", err)
	}

	perception := e.buildPerception(mon)
	speed := mon.Speed
	if speed <= 0 {
		speed = 6
	}
	economy := combat.NewActionEconomy()
	economy.MovementRemaining = speed

	input := &monster.TurnInput{
		Bus:           bus,
		ActionEconomy: economy,
		Perception:    perception,
		Roller:        e.roller,
		Speed:         speed,
	}

	result, err := loaded.TakeTurn(ctx, input)
	if err != nil {
		return fmt.Errorf("monster.TakeTurn: %w", err)
	}

	if err := e.applyNPCMovement(mon, result.Movement); err != nil {
		return err
	}
	if err := e.applyCapturedAttacks(mon, *captured); err != nil {
		if IsNPCPausedForReaction(err) {
			// NPC turn paused for player reaction — return the sentinel so
			// the orchestrator sees it explicitly and can break the dispatch
			// loop. Other captured-event processing (damage, conditions) is
			// skipped because the attack chain itself is suspended.
			return err
		}
		return err
	}
	if err := e.applyCapturedDamage(mon, *capturedDmg); err != nil {
		return err
	}
	if err := e.applyCapturedConditions(mon, *capturedCond); err != nil {
		return err
	}
	return nil
}

// syncMonsterDataFromSnapshot copies authoritative volatile state from
// the encounter's MonsterData snapshot onto a deserialized monster.Data
// before it is loaded into a live *Monster. The encounter SDK is the
// single source of truth for HP/AC/Speed; the JSON blob is the
// serialization seam for action data only and may be stale across turns
// (e.g. when TakeAction reduced HP last round).
//
// MonsterData does not track full SpeedData (just walking speed in
// hexes), so we only override Walk; other movement modes survive from
// the JSON snapshot.
func syncMonsterDataFromSnapshot(data *monster.Data, snap *MonsterData) {
	if data == nil || snap == nil {
		return
	}
	if snap.HP > 0 || snap.MaxHP > 0 {
		// HP can legitimately be 0 (dead/dying) but we only override when
		// the snapshot has set MaxHP — otherwise MonsterData is empty and
		// we leave the JSON values alone.
		if snap.MaxHP > 0 {
			data.HitPoints = snap.HP
			data.MaxHitPoints = snap.MaxHP
		}
	}
	if snap.AC > 0 {
		data.ArmorClass = snap.AC
	}
	if snap.Speed > 0 {
		data.Speed.Walk = snap.Speed
	}
}

// applyNPCMovement walks the NPC to the final hex from the captured
// movement path and emits a MoveEvent for any viewer who saw any segment.
func (e *Encounter) applyNPCMovement(mon *MonsterData, movement []spatial.CubeCoordinate) error {
	if len(movement) == 0 {
		return nil
	}
	final := movement[len(movement)-1]
	mon.Position = encountercore.Hex{Q: final.X, R: final.Y, S: final.Z}
	path := make([]encountercore.Hex, 0, len(movement))
	for _, hop := range movement {
		path = append(path, encountercore.Hex{Q: hop.X, R: hop.Y, S: hop.Z})
	}
	movePerPlayer := make(map[encountercore.PlayerID]events.MovePlayerSlice)
	for viewerID, viewer := range e.data.Players {
		seen := make([]encountercore.Hex, 0, len(path))
		for _, h := range path {
			if e.viewerCanSee(viewer, h) {
				seen = append(seen, h)
			}
		}
		if len(seen) > 0 {
			movePerPlayer[viewerID] = events.MovePlayerSlice{SeenSegments: seen}
		}
	}
	if len(movePerPlayer) == 0 {
		return nil
	}
	if err := e.broker.Publish(events.NewMoveEvent(
		e.data.ID, e.nextSeq(), mon.ID, path, movePerPlayer,
	)); err != nil {
		return fmt.Errorf("publish npc move: %w", err)
	}
	return nil
}

// applyCapturedAttacks resolves each captured dnd5e AttackEvent (cause-only)
// through the wired CombatResolver, mutates the target player's HP,
// and emits AttackResolved + DamageDealt encounter events.
//
// Wave 2.10: when an NPC's attack drops a player to HP=0, also publishes
// EntityDiedEvent for the player (with the NPC as killer). The player is
// NOT removed from initiative and EntityRemovedEvent is NOT published —
// player dying-state is Wave 2.11+ territory. The encounter does NOT
// auto-end on player deaths (TPK is also Wave 2.11+).
//
// Death is published only on the HP transition (hpBefore > 0 && hpAfter == 0),
// not whenever HP happens to be 0 — so multi-attack NPCs and re-hits on a
// downed player do not duplicate EntityDiedEvent.
func (e *Encounter) applyCapturedAttacks(mon *MonsterData, attacks []dnd5eEvents.AttackEvent) error {
	for _, atk := range attacks {
		targetID := encountercore.EntityID(atk.TargetID)
		targetPlayer := e.findPlayerByEntityID(targetID)
		if targetPlayer == nil {
			continue
		}
		dmgType := mon.DamageType
		if dmgType == "" {
			dmgType = damageTypeUntyped
		}
		hpBefore := targetPlayer.HP

		input := AttackInput{
			AttackerID:          mon.ID,
			TargetID:            targetID,
			ActionRef:           core.Ref{Module: "dnd5e", Type: "action", ID: actionIDAttack},
			AttackerAttackBonus: mon.AttackBonus,
			AttackerDamageDice:  mon.DamageDice,
			AttackerDamageType:  mon.DamageType,
			TargetAC:            targetPlayer.AC,
			EventBus:            e.bus,
		}

		// Wave 2.11d: route NPC attacks through the phased path when the
		// resolver supports it so player Shield prompts can fire on hits
		// against players. Falls back to single-phase ResolveAttack when the
		// resolver is legacy-only.
		outcome, paused, err := e.npcResolveAttackPhased(input, targetPlayer)
		if err != nil {
			return fmt.Errorf("combat resolver: %w", err)
		}
		if paused {
			// Player reactor was prompted. The encounter has the pending
			// reaction state + prompt event already published. Stop processing
			// further captured attacks for this NPC turn — the orchestrator
			// will resume the NPC dispatch after the reactor responds.
			return errNPCPausedForReaction
		}
		if outcome == nil {
			return fmt.Errorf("combat resolver: nil outcome with nil error")
		}
		if outcome.Hit {
			targetPlayer.HP -= outcome.Damage
			if targetPlayer.HP < 0 {
				targetPlayer.HP = 0
			}
		}
		if outcome.DamageType != "" {
			dmgType = outcome.DamageType
		}
		if err := e.publishAttackOutcome(
			mon.ID, targetID, outcome,
			targetPlayer.HP, targetPlayer.MaxHP, dmgType,
			mon.Position, targetPlayer.View.Position,
		); err != nil {
			return err
		}
		// Wave 2.10 partial player-death: fire EntityDiedEvent only on the
		// HP transition (avoids duplicates from multi-attack NPCs or hits
		// on an already-downed player). No EntityRemoved / no initiative
		// splice / no encounter-end.
		if outcome.Hit && hpBefore > 0 && targetPlayer.HP == 0 {
			if err := e.publishPlayerDied(targetID, mon.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// npcResolveAttackPhased runs an NPC attack via the resolver's phased path
// when available, installing a buffered subscriber on ReactionTriggerTopic
// to catch player Shield prompts. Returns:
//   - (outcome, false, nil): attack resolved normally (no reactions, or only
//     NPC-reactor triggers that were auto-resolved inline).
//   - (nil, true, nil): a player reactor was prompted; the AttackContext is
//     persisted on Encounter Data and the InputRequiredDeliveredEvent has
//     been published. The NPC dispatch loop must pause until the player
//     responds via SubmitCheck.
//   - (_, _, err): resolver failure.
//
// Falls back to the legacy single-phase ResolveAttack when the resolver
// does not implement PhasedCombatResolver.
func (e *Encounter) npcResolveAttackPhased(input AttackInput, _ *PlayerData) (*AttackOutcome, bool, error) {
	phased, ok := e.combatResolver.(PhasedCombatResolver)
	if !ok {
		// Legacy path — single-phase. Reactions cannot fire because the
		// resolver doesn't expose phase-1 state. Return the outcome as-is.
		out, err := e.combatResolver.ResolveAttack(input)
		return out, false, err
	}

	triggers, drainCleanup, err := e.installTriggerBuffer()
	if err != nil {
		return nil, false, fmt.Errorf("install trigger buffer: %w", err)
	}
	defer drainCleanup()

	attackCtx, resolverTriggers, err := phased.ResolveAttackHit(input)
	if err != nil {
		return nil, false, err
	}

	allTriggers := append([]ReactionTrigger{}, resolverTriggers...)
	allTriggers = append(allTriggers, *triggers...)
	npcTriggers, playerTriggers := e.partitionTriggers(allTriggers)

	if len(playerTriggers) == 0 {
		// No player prompts — resolve NPC modifiers (if any) and run phase 2.
		modifiers := e.npcModifiers(npcTriggers)
		outcome, err := phased.ApplyAttackOutcome(attackCtx, modifiers)
		if err != nil {
			return nil, false, err
		}
		return outcome, false, nil
	}

	// Player triggers present — persist the pending state + emit prompt
	// events, then signal the caller to pause. The orchestrator's NPC
	// dispatch loop sees PendingReactionPrompts on the encounter and stops
	// advancing.
	if err := e.persistNPCPendingReactions(attackCtx, playerTriggers); err != nil {
		return nil, false, fmt.Errorf("persist NPC pending reactions: %w", err)
	}
	return nil, true, nil
}

// persistNPCPendingReactions records each player trigger from an NPC attack
// as a PendingReactionPrompt and emits an InputRequiredDeliveredEvent.
//
// AttackContextJSON is left empty here because the encounter SDK does not
// know the resolver's rulebook-AttackContext shape. The orchestrator's
// CompleteTakeAction path on resume rebuilds the context from the prompt's
// metadata + the wired resolver. This works because rpg-api's Dnd5eCombatResolver
// rebuilds the prep on the resume path (cache miss when running in a fresh
// SubmitCheck RPC).
//
// FUTURE: when the SDK gains a way to ask the resolver "marshal this
// AttackContext to bytes", we can persist the actual context here for fidelity.
// For 2.11d, NPC-attacks-trigger-player-Shield works via the encounter-bus
// PostAttackRollChain: the orchestrator's CompleteTakeAction calls
// ApplyAttackOutcome with a fresh prep that re-runs phase 1 + finds the
// trigger again, then runs phase 2 with the modifier. For 2.11d this shape
// works because the orchestrator stores enough metadata on the prompt.
func (e *Encounter) persistNPCPendingReactions(
	attackCtx *PhasedAttackContext,
	playerTriggers []ReactionTrigger,
) error {
	for _, trig := range playerTriggers {
		reactorPlayerID, ok := e.playerByEntityIDLookup(trig.ReactorID)
		if !ok {
			continue
		}
		// Without a host-side AttackContext serializer (cross-rulebook concern),
		// the prompt persists metadata only. The host's SubmitCheck handler is
		// responsible for marshaling AttackContext via its rulebook adapter
		// before this call lands; this branch is the SDK fallback for hosts
		// that haven't wired the marshaling step.
		e.SetPendingReactionPrompt(reactorPlayerID, &PendingReactionPrompt{
			ReactorEntityID: trig.ReactorID,
			ConditionRef:    trig.ConditionRef,
			TriggerKind:     trig.TriggerKind,
			SourceEntity:    trig.SourceEntity,
			// AttackContextJSON intentionally empty — the host marshals the
			// PhasedAttackContext (which includes the rulebook-side context)
			// via its own type before re-saving the encounter.
		})
		if err := e.PublishInputRequiredDelivered(reactorPlayerID, events.PromptKindReaction); err != nil {
			return err
		}
		// Cache the in-flight phasedCtx so the host can pull it via a
		// follow-up call (added below for the resume path).
		e.cachePhasedAttackContext(reactorPlayerID, attackCtx)
	}
	return nil
}

// playerByEntityIDLookup looks up a player ID by entity ID. Returns false
// if the entity isn't a player (caller should skip).
func (e *Encounter) playerByEntityIDLookup(id encountercore.EntityID) (encountercore.PlayerID, bool) {
	for pid, pd := range e.data.Players {
		if pd.EntityID == id {
			return pid, true
		}
	}
	return "", false
}

// errNPCPausedForReaction signals to the NPC dispatch loop that a player
// reaction prompt was issued and the NPC's turn must pause. Internal
// sentinel; not exported.
var errNPCPausedForReaction = fmt.Errorf("npc turn paused for player reaction")

// IsNPCPausedForReaction reports whether the given error indicates an NPC
// turn paused for a player reaction prompt. Used by the orchestrator's
// EndTurn loop to distinguish a benign pause from a real failure.
func IsNPCPausedForReaction(err error) bool {
	return err == errNPCPausedForReaction
}

// cachePhasedAttackContext stores the in-flight PhasedAttackContext for a
// reactor so the host can read it via PendingPhasedAttackContext on the
// resume path. This complements the persisted PendingReactionPrompt: the
// prompt is durable across RPCs (snapshot persists), the AttackContext is
// in-process for the same RPC (the cache is per-Encounter instance).
//
// For the cross-RPC resume case, the host marshals the AttackContext into
// the PendingReactionPrompt.AttackContextJSON before persisting; on the
// next RPC the host unmarshals it back. This cache only matters when the
// host doesn't marshal (fast-path single-RPC NPC attacks that pause).
func (e *Encounter) cachePhasedAttackContext(
	reactorPlayerID encountercore.PlayerID,
	attackCtx *PhasedAttackContext,
) {
	if e.pendingPhasedAttacks == nil {
		e.pendingPhasedAttacks = make(map[encountercore.PlayerID]*PhasedAttackContext)
	}
	e.pendingPhasedAttacks[reactorPlayerID] = attackCtx
}

// PendingPhasedAttackContext returns the in-process AttackContext for the
// reactor (set by an NPC attack that paused for a player reaction). Nil if
// none. The host calls this on the resume path to feed CompleteTakeAction.
func (e *Encounter) PendingPhasedAttackContext(reactorPlayerID encountercore.PlayerID) *PhasedAttackContext {
	return e.pendingPhasedAttacks[reactorPlayerID]
}

// applyCapturedDamage translates each dnd5e DamageReceivedEvent into an
// encounter DamageDealtEvent. Today no shipped action publishes one
// through this bus, but the wiring is in place so downstream changes
// flow automatically.
//
// Target resolution: tries player → monster. If neither matches, the
// damage event is skipped (no DamageDealtEvent published) — emitting
// with hp_after=0 / zero-hex position would leak garbage values to
// clients.
//
// Wave 2.10: if captured damage drops a monster's HP from >0 to 0, fires
// the kill chain (EntityDied + EntityRemoved + checkEncounterEnd). If it
// drops a player's HP from >0 to 0, fires only EntityDiedEvent (partial
// player-death per Wave 2.10 architectural call). Re-applying damage to
// an already-zero target does NOT re-fire death events — death is gated
// on the >0 → 0 transition.
func (e *Encounter) applyCapturedDamage(mon *MonsterData, damages []dnd5eEvents.DamageReceivedEvent) error {
	for _, dmg := range damages {
		targetID := encountercore.EntityID(dmg.TargetID)
		sourceID := encountercore.EntityID(dmg.SourceID)

		hpBefore, hpAfter, maxHP, targetPos, isMonster, ok := e.applyDamageToTarget(targetID, dmg.Amount)
		if !ok {
			// Unknown target — skip publish rather than emit a stub event.
			continue
		}
		damageType := string(dmg.DamageType)
		if damageType == "" {
			damageType = damageTypeUntyped
		}
		damagePerPlayer := make(map[encountercore.PlayerID]events.DamageDealtSlice)
		for viewerID, viewer := range e.data.Players {
			if !e.viewerCanSee(viewer, mon.Position) && !e.viewerCanSee(viewer, targetPos) {
				continue
			}
			damagePerPlayer[viewerID] = events.DamageDealtSlice{Visible: true}
		}
		if err := e.broker.Publish(events.NewDamageDealtEvent(
			e.data.ID, e.nextSeq(),
			targetID, sourceID,
			dmg.Amount, damageType,
			hpAfter, maxHP,
			damagePerPlayer,
		)); err != nil {
			return fmt.Errorf("publish damage dealt: %w", err)
		}
		if hpBefore > 0 && hpAfter == 0 {
			if isMonster {
				if err := e.killEntity(targetID, sourceID); err != nil {
					return err
				}
			} else {
				if err := e.publishPlayerDied(targetID, sourceID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// applyDamageToTarget mutates HP on a player or monster matching id and
// returns its pre-damage HP, post-damage HP, MaxHP, position, and whether
// the target was a monster (vs a player). hpBefore lets callers detect
// the >0 → 0 transition so death events fire exactly once per kill, not
// per "HP happens to be 0 right now" check. Returns ok=false if the id
// matches neither a player nor a monster. HP is clamped at 0.
func (e *Encounter) applyDamageToTarget(
	id encountercore.EntityID, amount int,
) (hpBefore, hpAfter, maxHP int, pos encountercore.Hex, isMonster bool, ok bool) {
	if p := e.findPlayerByEntityID(id); p != nil {
		hpBefore = p.HP
		p.HP -= amount
		if p.HP < 0 {
			p.HP = 0
		}
		return hpBefore, p.HP, p.MaxHP, p.View.Position, false, true
	}
	if m, exists := e.data.Monsters[id]; exists {
		hpBefore = m.HP
		m.HP -= amount
		if m.HP < 0 {
			m.HP = 0
		}
		return hpBefore, m.HP, m.MaxHP, m.Position, true, true
	}
	return 0, 0, 0, encountercore.Hex{}, false, false
}

// applyCapturedConditions translates each dnd5e ConditionAppliedEvent into
// an encounter ConditionAppliedEvent. As with damage, no shipped action
// publishes one through this bus today.
//
// Per-viewer projection mirrors publishAttackOutcome: a viewer is in
// PerPlayer iff they have LoS to the source (mon) or the target. Viewers
// out of LoS are omitted from PerPlayer entirely so the broker does not
// deliver to them — matching the Move / OpenDoor audience-routing
// pattern.
func (e *Encounter) applyCapturedConditions(mon *MonsterData, conds []dnd5eEvents.ConditionAppliedEvent) error {
	for _, cond := range conds {
		targetID := encountercore.EntityID("")
		var targetPos encountercore.Hex
		var haveTargetPos bool
		if cond.Target != nil {
			targetID = encountercore.EntityID(cond.Target.GetID())
			if p := e.findPlayerByEntityID(targetID); p != nil && p.View != nil {
				targetPos = p.View.Position
				haveTargetPos = true
			} else if m, ok := e.data.Monsters[targetID]; ok {
				targetPos = m.Position
				haveTargetPos = true
			}
		}
		condRef := string(cond.Type)
		condPerPlayer := make(map[encountercore.PlayerID]events.ConditionAppliedSlice)
		for viewerID, viewer := range e.data.Players {
			seesSource := e.viewerCanSee(viewer, mon.Position)
			seesTarget := haveTargetPos && e.viewerCanSee(viewer, targetPos)
			if !seesSource && !seesTarget {
				continue
			}
			condPerPlayer[viewerID] = events.ConditionAppliedSlice{Visible: true}
		}
		if err := e.broker.Publish(events.NewConditionAppliedEvent(
			e.data.ID, e.nextSeq(),
			targetID, mon.ID, condRef, 0, condPerPlayer,
		)); err != nil {
			return fmt.Errorf("publish condition applied: %w", err)
		}
	}
	return nil
}

// npcActScripted runs a minimal attack against the closest player when
// the monster has no DataJSON to rehydrate. Used for tests and
// orchestrator-seeded fixtures that don't carry a serialized monster.
//
// Delegates to the wired CombatResolver (same as applyCapturedAttacks and
// TakeAction). The resolver guard at NPCAct entry ensures combatResolver
// is non-nil before this path is reached.
//
// Wave 2.10: same partial player-death semantics as applyCapturedAttacks
// — fires EntityDiedEvent on a player kill (only on HP transition, not
// whenever HP is 0) but does not remove the player or end the encounter.
func (e *Encounter) npcActScripted(_ context.Context, mon *MonsterData) error {
	target := e.closestPlayer(mon.Position)
	if target == nil {
		return nil
	}
	dmgType := mon.DamageType
	if dmgType == "" {
		dmgType = damageTypeUntyped
	}
	hpBefore := target.HP
	outcome, err := e.combatResolver.ResolveAttack(AttackInput{
		AttackerID:          mon.ID,
		TargetID:            target.EntityID,
		ActionRef:           core.Ref{Module: "dnd5e", Type: "action", ID: "attack"},
		AttackerAttackBonus: mon.AttackBonus,
		AttackerDamageDice:  mon.DamageDice,
		AttackerDamageType:  mon.DamageType,
		TargetAC:            target.AC,
		EventBus:            e.bus,
	})
	if err != nil {
		return fmt.Errorf("combat resolver: %w", err)
	}
	if outcome == nil {
		return fmt.Errorf("combat resolver: nil outcome with nil error")
	}
	if outcome.Hit {
		target.HP -= outcome.Damage
		if target.HP < 0 {
			target.HP = 0
		}
	}
	if outcome.DamageType != "" {
		dmgType = outcome.DamageType
	}
	if err := e.publishAttackOutcome(
		mon.ID, target.EntityID, outcome,
		target.HP, target.MaxHP, dmgType,
		mon.Position, target.View.Position,
	); err != nil {
		return err
	}
	if outcome.Hit && hpBefore > 0 && target.HP == 0 {
		if err := e.publishPlayerDied(target.EntityID, mon.ID); err != nil {
			return err
		}
	}
	return nil
}

// buildPerception assembles the PerceptionData a monster needs to choose
// and target an action. Wave 2.8 treats every player as an enemy and
// computes distances directly from hex coordinates — no walls or cover.
func (e *Encounter) buildPerception(mon *MonsterData) *monster.PerceptionData {
	pos := spatial.CubeCoordinate{X: mon.Position.Q, Y: mon.Position.R, Z: mon.Position.S}
	pd := &monster.PerceptionData{
		MyPosition: pos,
	}
	for _, p := range e.data.Players {
		dist := hexDistance(mon.Position, p.View.Position)
		pd.Enemies = append(pd.Enemies, monster.PerceivedEntity{
			Entity:   &playerEntity{id: string(p.EntityID), name: string(p.ID)},
			Position: spatial.CubeCoordinate{X: p.View.Position.Q, Y: p.View.Position.R, Z: p.View.Position.S},
			Distance: dist,
			Adjacent: dist == 1,
			HP:       p.HP,
			AC:       p.AC,
		})
	}
	// Sort enemies by distance, closest first — required by the targeting
	// strategies in monster.PerceptionData.
	sortEnemiesByDistance(pd.Enemies)
	return pd
}

// playerEntity adapts a player into a core.Entity for the dnd5e
// perception layer. Only ID is read by the targeting code paths shipped
// in Wave 2.8.
type playerEntity struct {
	id   string
	name string
}

func (p *playerEntity) GetID() string            { return p.id }
func (p *playerEntity) GetType() core.EntityType { return "character" }
func (p *playerEntity) GetName() string          { return p.name }

// closestPlayer returns the player nearest the given hex, or nil if the
// encounter has no players.
func (e *Encounter) closestPlayer(from encountercore.Hex) *PlayerData {
	var best *PlayerData
	bestDist := -1
	for _, p := range e.data.Players {
		d := hexDistance(from, p.View.Position)
		if best == nil || d < bestDist {
			best = p
			bestDist = d
		}
	}
	return best
}

// findPlayerByEntityID returns the player whose EntityID matches id.
func (e *Encounter) findPlayerByEntityID(id encountercore.EntityID) *PlayerData {
	for _, p := range e.data.Players {
		if p.EntityID == id {
			return p
		}
	}
	return nil
}

// viewerCanSee delegates to the perception stub LoS but tolerates a nil view.
func (e *Encounter) viewerCanSee(p *PlayerData, h encountercore.Hex) bool {
	if p == nil || p.View == nil {
		return false
	}
	return hexDistance(p.View.Position, h) <= p.View.SightRange
}

// hexDistance is the cube-coordinate hex distance between two hexes.
func hexDistance(a, b encountercore.Hex) int {
	dq := absInt(a.Q - b.Q)
	dr := absInt(a.R - b.R)
	ds := absInt(a.S - b.S)
	if dq > dr {
		if dq > ds {
			return dq
		}
		return ds
	}
	if dr > ds {
		return dr
	}
	return ds
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// sortEnemiesByDistance sorts perceived enemies in ascending distance
// order, ties broken by entity id for determinism.
func sortEnemiesByDistance(es []monster.PerceivedEntity) {
	for i := 1; i < len(es); i++ {
		for j := i; j > 0; j-- {
			if es[j].Distance < es[j-1].Distance ||
				(es[j].Distance == es[j-1].Distance && es[j].Entity.GetID() < es[j-1].Entity.GetID()) {
				es[j], es[j-1] = es[j-1], es[j]
				continue
			}
			break
		}
	}
}

// subscribeAttacks attaches a listener for the dnd5e AttackTopic on bus
// and returns (capturedSlicePtr, unsubscribe, err). The slice grows for
// the lifetime of the bus subscription.
func subscribeAttacks(ctx context.Context, bus dnd5events.EventBus) (*[]dnd5eEvents.AttackEvent, func() error, error) {
	captured := &[]dnd5eEvents.AttackEvent{}
	topic := dnd5eEvents.AttackTopic.On(bus)
	subID, err := topic.Subscribe(ctx, func(_ context.Context, evt dnd5eEvents.AttackEvent) error {
		*captured = append(*captured, evt)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	unsub := func() error { return topic.Unsubscribe(ctx, subID) }
	return captured, unsub, nil
}

// subscribeDamage attaches a listener for the dnd5e DamageReceivedTopic.
func subscribeDamage(
	ctx context.Context, bus dnd5events.EventBus,
) (*[]dnd5eEvents.DamageReceivedEvent, func() error, error) {
	captured := &[]dnd5eEvents.DamageReceivedEvent{}
	topic := dnd5eEvents.DamageReceivedTopic.On(bus)
	subID, err := topic.Subscribe(ctx, func(_ context.Context, evt dnd5eEvents.DamageReceivedEvent) error {
		*captured = append(*captured, evt)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	unsub := func() error { return topic.Unsubscribe(ctx, subID) }
	return captured, unsub, nil
}

// subscribeConditions attaches a listener for the dnd5e ConditionAppliedTopic.
func subscribeConditions(
	ctx context.Context, bus dnd5events.EventBus,
) (*[]dnd5eEvents.ConditionAppliedEvent, func() error, error) {
	captured := &[]dnd5eEvents.ConditionAppliedEvent{}
	topic := dnd5eEvents.ConditionAppliedTopic.On(bus)
	subID, err := topic.Subscribe(ctx, func(_ context.Context, evt dnd5eEvents.ConditionAppliedEvent) error {
		*captured = append(*captured, evt)
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	unsub := func() error { return topic.Unsubscribe(ctx, subID) }
	return captured, unsub, nil
}
