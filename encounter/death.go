package encounter

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// ErrEncounterEnded is returned by combat verbs (TakeAction, EndTurn,
// NPCAct) when called against an encounter whose mode is ModeEnded.
// Maps to gRPC FailedPrecondition on the rpg-api side.
var ErrEncounterEnded = errors.New("encounter has ended")

// EntityRemovedReasonDestroyed is the Reason value on EntityRemovedEvent
// when the entity was removed because its HP reached zero. Future waves
// add other reasons ("fled", "transformed", etc.).
const EntityRemovedReasonDestroyed = "destroyed"

// EncounterEndedReasonAllHostilesDefeated is the Reason value on
// EncounterEndedEvent when the last hostile died. Wave 2.10 ships only
// this end condition; future waves add others ("fled", "negotiated",
// "time_out", etc.).
const EncounterEndedReasonAllHostilesDefeated = "all_hostiles_defeated"

// EncounterEndedReasonAbandoned is the Reason value on EncounterEndedEvent
// when the encounter is administratively ended via End rather than reaching
// a natural victory/TPK conclusion — rpg-toolkit#797 (rpg-api#663). The slot
// was reserved by #783's reason-taxonomy comment on this same constant
// block. Typical caller: rpg-api's abandon path, invoked by a lobby host (or
// the liveness/resume check) on an encounter with no other way to end —
// e.g. a FREE_ROAM encounter stuck by #483's movement deadlock, which never
// reaches checkEncounterEnd's victory/TPK predicates because those only
// evaluate from combat kill paths.
const EncounterEndedReasonAbandoned = "abandoned"

// EncounterEndedReasonTPK is the Reason value on EncounterEndedEvent when
// every seated player has died within the encounter (a Total Party Kill) —
// rpg-toolkit#772/#782. Symmetric with EncounterEndedReasonAllHostilesDefeated
// on the monster side: the same terminal-transition machinery
// (checkEncounterEnd) fires either reason, clearing Initiative/ActiveIdx/
// Round and sweeping combat-scoped conditions identically regardless of
// which side lost.
//
// "Dead" here means CONFIRMED death (3 failed death saves, or the
// non-hydrated instant-death fallback — see PlayerData.Dead), not merely
// unconscious (HP<=0, still death-saving, could nat-20 revive) — a
// merely-unconscious last player does NOT trigger this, so the death-save
// mechanic still gets its chance to matter even when only one player is
// left standing.
const EncounterEndedReasonTPK = "tpk"

// killEntity is the toolkit-side state-mutation helper for "monster died."
// Callers (post-damage paths in TakeAction and NPCAct) invoke it after a
// monster's HP is clamped to zero. The helper:
//
//  1. Builds the per-viewer projection for EntityDiedEvent (visibility
//     derived from LoS to the dying monster's last position OR the killer's
//     position) and publishes it.
//  2. Removes the monster from data.Monsters.
//  3. Splices the monster out of data.Initiative; if the removed slot was
//     before ActiveIdx, decrements ActiveIdx so the active actor pointer
//     still references the same entity after the shift.
//  4. Publishes EntityRemovedEvent (broadcast — every player must drop
//     the entity from local state, even out-of-LoS viewers).
//  5. Calls checkEncounterEnd which may transition to ModeEnded and
//     publish EncounterEndedEvent.
//
// killEntity is monster-only. Player death is partial in Wave 2.10:
// EntityDiedEvent fires for the dying player (so the web can surface it),
// but the player is NOT removed from initiative and EntityRemovedEvent
// is NOT published — the dying-state machinery is Wave 2.11+. The post-
// damage path in NPCAct calls publishPlayerDied directly instead of
// killEntity for that reason.
//
// Returns an error if monsterID is not present in data.Monsters (caller
// bug — the path that triggered the kill should have validated the
// target). KillerID may be empty for environmental / future indirect kills.
func (e *Encounter) killEntity(monsterID, killerID core.EntityID) error {
	mon, ok := e.data.Monsters[monsterID]
	if !ok {
		return fmt.Errorf("killEntity: monster %q not in encounter", monsterID)
	}

	// Snapshot dying-monster position before deletion — needed for the
	// per-viewer LoS projection on EntityDiedEvent.
	dyingPos := mon.Position
	killerPos, killerHasPos := e.positionFor(killerID)

	diedPerPlayer := make(map[core.PlayerID]events.EntityDiedSlice)
	// witnesses is who actually SAW the monster die (as opposed to seeing
	// only the killer) — the set whose geometry memory of dyingPos needs an
	// immediate witnessed-removal refresh below. Seeing only the killer
	// does not mean seeing the monster's hex.
	var witnesses []core.PlayerID
	for viewerID, viewer := range e.data.Players {
		seesDying := perception.CanSeeAt(viewer.View, dyingPos, e.room)
		seesKiller := killerHasPos && perception.CanSeeAt(viewer.View, killerPos, e.room)
		if !seesDying && !seesKiller {
			continue
		}
		diedPerPlayer[viewerID] = events.EntityDiedSlice{Visible: true}
		if seesDying {
			witnesses = append(witnesses, viewerID)
		}
	}
	if err := e.broker.Publish(events.NewEntityDiedEvent(
		e.data.ID, e.nextSeq(), monsterID, killerID, diedPerPlayer,
	)); err != nil {
		return fmt.Errorf("publish entity died: %w", err)
	}

	// Mutate state: drop from monsters and splice out of initiative.
	delete(e.data.Monsters, monsterID)
	e.spliceFromInitiative(monsterID)

	// rpg-toolkit#851: a witnessed removal must update memory IMMEDIATELY,
	// not wait for the witness's own next unrelated move/reveal to notice.
	// Refresh runs AFTER the delete above so placementsAt's scan of
	// e.data.Monsters naturally no longer finds monsterID — the resulting
	// observation is simply a VISIBLE record that no longer lists it
	// (HexObservation's doc: "nothing is ever deleted" — a witnessed
	// removal is exactly this, never a tombstone). Viewers who saw only the
	// killer, not the dying monster's own hex, are NOT in witnesses and get
	// no refresh here — they never had authorized knowledge of that hex's
	// contents to correct.
	for _, viewerID := range witnesses {
		e.refreshObservations(e.data.Players[viewerID].View, core.NewHexSet(dyingPos), "")
	}

	// Broadcast removal — every player must drop the entity from local
	// state, even those out of LoS at death time. Build PerPlayer over
	// the full player set with Visible: true.
	if err := e.broker.Publish(events.NewEntityRemovedEvent(
		e.data.ID, e.nextSeq(), monsterID, EntityRemovedReasonDestroyed,
		e.allViewersEntityRemoved(),
	)); err != nil {
		return fmt.Errorf("publish entity removed: %w", err)
	}

	// Check terminal-state predicate; may publish EncounterEndedEvent.
	if err := e.checkEncounterEnd(); err != nil {
		return err
	}
	// rpg-toolkit#794: if the whole dungeon didn't just clear (some
	// monsters remain elsewhere — checkEncounterEnd above was a no-op),
	// check whether THIS pocket (the current Initiative) is now clear of
	// monsters. If so, non-terminally exit back to FREE_ROAM instead of
	// soft-locking on a rotation of player-only turns with no monster left
	// to fight.
	return e.checkPocketCleared()
}

// publishPlayerDied fires an EntityDiedEvent for a player whose HP reached
// zero, with NPC-as-killer visibility projection, marks the seat Dead, and
// re-evaluates the encounter-end predicate (rpg-toolkit#772/#782). It does
// NOT publish EntityRemovedEvent and does NOT remove the player from
// Initiative — "seats stay on death" is a deliberate, still-standing Wave
// 2.10 call: a dead player's corpse keeps a nominal turn slot (auto-skipped
// via seedActorTurn's HP<=0 branch) rather than disturbing initiative
// order; killEntity's monster-only splice logic is not reused here.
//
// Visibility: a viewer is in PerPlayer iff they have LoS to the dying
// player OR the killing NPC. The dying player is ALWAYS included
// unconditionally (rpg-toolkit#741 hardening) — a player learning of their
// own death must never depend on a LoS/perception computation over their
// own position; that dependency is real (CanSeeAt reads live position +
// sight range) but conceptually unnecessary, and any future change to LoS
// rules (multi-room, forced movement, senses) should not be able to hide a
// player's own death from them.
//
// This is the single chokepoint both death paths funnel through — the
// CharacterDied bridge (3 failed death saves, subscribeCharacterDiedBridge)
// and applyUnconsciousOnZeroHP's non-hydrated instant-death fallback — so
// marking Dead + checking checkEncounterEnd here covers every current and
// future death path without any call site needing to remember to do it.
func (e *Encounter) publishPlayerDied(playerEntityID, killerID core.EntityID) error {
	playerData := e.findPlayerByEntityID(playerEntityID)
	if playerData == nil || playerData.View == nil {
		return fmt.Errorf("publishPlayerDied: player entity %q not found", playerEntityID)
	}
	dyingPos := playerData.View.Position
	killerPos, killerHasPos := e.positionFor(killerID)

	diedPerPlayer := make(map[core.PlayerID]events.EntityDiedSlice)
	diedPerPlayer[playerData.ID] = events.EntityDiedSlice{Visible: true}
	for viewerID, viewer := range e.data.Players {
		if viewerID == playerData.ID {
			continue
		}
		seesDying := perception.CanSeeAt(viewer.View, dyingPos, e.room)
		seesKiller := killerHasPos && perception.CanSeeAt(viewer.View, killerPos, e.room)
		if !seesDying && !seesKiller {
			continue
		}
		diedPerPlayer[viewerID] = events.EntityDiedSlice{Visible: true}
	}
	if err := e.broker.Publish(events.NewEntityDiedEvent(
		e.data.ID, e.nextSeq(), playerEntityID, killerID, diedPerPlayer,
	)); err != nil {
		return fmt.Errorf("publish entity died (player): %w", err)
	}

	// #772/#782: this player is now CONFIRMED dead. Mark the seat and
	// re-evaluate the encounter-end predicate — a no-op unless every other
	// seated player is also Dead, in which case this transitions the
	// encounter to ModeEnded with Reason "tpk".
	playerData.Dead = true
	if err := e.checkEncounterEnd(); err != nil {
		return fmt.Errorf("check encounter end after player death: %w", err)
	}
	return nil
}

// applyUnconsciousOnZeroHP replaces the old "player hits 0 HP -> immediately
// dead" behavior (#733). Instead of calling publishPlayerDied directly, a
// player whose HP just transitioned >0 -> 0 has the Unconscious condition
// applied, which engages the death-save mechanism (auto-roll at their own
// next turn start, auto-fail on further damage, wake on healing — all
// already implemented in conditions.UnconsciousCondition and wired live by
// rpg-toolkit#729's turn-start revival). EntityDiedEvent for the player now
// only fires via the CharacterDied bridge (subscribeCharacterDiedBridge)
// after 3 failed death saves.
//
// Falls back to the pre-#733 publishPlayerDied behavior when the target has
// no hydrated *character.Character (a flat stat-snapshot seat) — there is no
// rulebook object to carry death-save state onto, so this is a deliberate,
// narrow scope call rather than building unconscious-tracking for
// non-hydrated seats.
//
// Monster HP-zero handling (killEntity) is completely untouched by this —
// this helper is player-only, called from the three sites that used to call
// publishPlayerDied for the player branch of their isMonster check.
func (e *Encounter) applyUnconsciousOnZeroHP(ctx context.Context, target *PlayerData, sourceID core.EntityID) error {
	char := e.heldCharacter(target.EntityID)
	if char == nil {
		return e.publishPlayerDied(target.EntityID, sourceID)
	}

	// rpg-toolkit#781/#784: combat-inflicted damage (NPCAct, Move-triggered
	// opportunity attacks) mutates PlayerData.HP directly and never calls
	// char.ApplyDamage — neither the legacy nor phased attack-resolution
	// path in rulebooks/dnd5e/combat touches a hydrated defender's HP; that
	// remains entirely the encounter package's job. So by the time this
	// function runs, char.GetHitPoints() can still read whatever stale,
	// pre-knockdown value it held (target.HP is already authoritatively 0 —
	// that IS the trigger for this function being called; char's own copy
	// never got the memo). seedActorTurn's downed-actor gate
	// (turn_economy.go) reads char.GetHitPoints(), not PlayerData.HP, so an
	// unsynced character re-seeds a full economy on every turn AFTER the
	// knockdown, not just the same one — the #781 mid-turn fix below only
	// ever closed the current-turn window.
	//
	// Correct it here, once, at the single existing chokepoint every
	// player's >0->0 transition already funnels through: apply "however
	// much char.hitPoints currently (staleness included) shows" as a
	// bookkeeping-only damage instance, bringing it to exactly 0. No need
	// to know the real attack's damage number — the target is already
	// authoritatively down; this just makes the held character agree.
	// char.ApplyDamage (character.go) is a pure c.hitPoints -= total
	// mutation with no event publish and no resistance/vulnerability
	// lookup (DamageInstance.Amount's own doc: "after modifiers, before
	// resistance" — resistance lives upstream in the damage chain's
	// StageFinal, never consulted here), so this can't be softened by a
	// raging barbarian's physical resistance or double-fire anything the
	// Unconscious condition application below owns.
	if hp := char.GetHitPoints(); hp > 0 {
		char.ApplyDamage(ctx, &combat.ApplyDamageInput{
			Instances: []combat.DamageInstance{{Amount: hp, Type: "untyped"}},
		})
	}

	uc := conditions.NewUnconsciousCondition(string(target.EntityID), e.roller)
	evt := dnd5eEvents.ConditionAppliedEvent{
		Target:    char,
		Type:      dnd5eEvents.ConditionUnconscious,
		Source:    dnd5eEvents.ConditionSourceDamage,
		Condition: uc,
	}
	// Publishing on ConditionAppliedTopic is what actually applies the
	// condition: character.Character.onConditionApplied is permanently
	// subscribed to this topic and calls evt.Condition.Apply(ctx, c.bus)
	// itself (the Dodge/Rage pattern — see combatabilities/dodge.go).
	if err := dnd5eEvents.ConditionAppliedTopic.On(e.bus).Publish(ctx, evt); err != nil {
		return fmt.Errorf("publish unconscious condition: %w", err)
	}

	// Bridge the same event to the broker-facing ConditionAppliedEvent so
	// live clients see the status, reusing applyActivatedConditions (which
	// already resolves target/audience from cond.Target — the R1 fix from
	// #716) rather than writing new broker-bridging code. sourceID as the
	// actorID arg gives correct "who did this" narration; audience
	// resolution is unaffected by it since cond.Target is always set here.
	if err := e.applyActivatedConditions(nil, sourceID, []dnd5eEvents.ConditionAppliedEvent{evt}); err != nil {
		return fmt.Errorf("bridge unconscious condition: %w", err)
	}

	// #781: if this player is the CURRENTLY ACTIVE actor, they went
	// unconscious mid-turn — e.g. an opportunity attack triggered by their
	// own Move (npc.go's applyCapturedDamage / applyMoveDamage paths). Their
	// action economy for this turn was already seeded in full at turn start
	// (seedActorTurn), and nothing else re-checks it before their next
	// TakeAction call. Without this, a player could trigger an OA, drop to 0
	// HP, and still land an attack of their own with the remainder of an
	// already-seeded-but-unspent economy — "landed a killing blow while
	// unconscious." This closes the CURRENT-turn half of that bug; the
	// EVERY-SUBSEQUENT-turn half (a combat-downed player re-seeding a full
	// economy again and again) was a second, independent bug — the
	// char.GetHitPoints() staleness the HP-sync above this block fixes
	// (#784). Both together are what "full economy one round, zero
	// another" actually described.
	//
	// Reuses char.EndTurn — the exact zeroing call seedActorTurn's own
	// downed-at-turn-start branch (turn_economy.go) already uses — rather
	// than inventing new zeroing logic. A player who is NOT the active
	// actor has no reachable economy to close off: TakeAction gates on
	// ActiveActor() == player.EntityID, so stale economy from a turn that
	// already ended is already unusable; ActiveActor() itself safely
	// returns "" outside ModeTurnBased, so no extra mode guard is needed
	// here.
	if e.ActiveActor() == target.EntityID {
		if _, err := char.EndTurn(ctx, &character.EndTurnInput{}); err != nil {
			return fmt.Errorf("zero turn economy for newly-unconscious actor %q: %w", target.EntityID, err)
		}
	}
	return nil
}

// subscribeCharacterDiedBridge installs a PERMANENT subscription to the
// rulebook's CharacterDiedTopic on e.bus, bridging final death-save-death (3
// failed death saves) to the broker-facing EntityDiedEvent. Unlike the
// transient capture-buffers used elsewhere in this package (subscribeAttacks,
// subscribeConditions, installTriggerBuffer), CharacterDiedEvent can fire
// from multiple, unrelated call sites over the encounter's lifetime — a
// turn-start auto-roll (UnconsciousCondition.onTurnStart) or getting hit
// again while already down (UnconsciousCondition.onDamageReceived) — not
// just once per verb call. So this is installed once, immediately after e.bus
// is constructed, and lives for the encounter's lifetime; both New and
// LoadFromData call this right after building a fresh bus.
//
// Handler: resolves the dying character to a seated player; if found,
// publishes EntityDiedEvent via publishPlayerDied with an empty killer id —
// there is no specific final blow at 3-failed-saves death.
// publishPlayerDied/positionFor already handle an empty id gracefully (it
// simply contributes nothing to the per-viewer LoS projection beyond the
// dying player's own position). Does NOT call killEntity, remove the player
// from initiative, or trigger checkEncounterEnd — Wave 2.10's "player is NOT
// removed from initiative, dying-state/TPK is future work" call still stands;
// this only wires the missing broker event. A no-op (not an error) if the
// dead character doesn't resolve to a seated player — e.g. a future monster
// with an Unconscious condition would simply not match here.
func (e *Encounter) subscribeCharacterDiedBridge(ctx context.Context) error {
	_, err := dnd5eEvents.CharacterDiedTopic.On(e.bus).Subscribe(ctx,
		func(_ context.Context, event dnd5eEvents.CharacterDiedEvent) error {
			p := e.findPlayerByEntityID(core.EntityID(event.CharacterID))
			if p == nil {
				return nil
			}
			return e.publishPlayerDied(p.EntityID, "")
		})
	if err != nil {
		return fmt.Errorf("subscribe character died bridge: %w", err)
	}
	return nil
}

// subscribeCharacterStabilizedBridge installs a PERMANENT subscription to
// the rulebook's CharacterStabilizedTopic on e.bus, bridging "3 successful
// death saves" to the broker-facing EntityStabilizedEvent (rpg-toolkit#741 —
// previously this topic had zero production subscribers anywhere in
// encounter/, so stabilization was completely wire-invisible). Same
// lifetime shape as subscribeCharacterDiedBridge: CharacterStabilizedEvent
// can fire from a turn-start auto-roll at any point in the encounter's
// life, not just once per verb call, so this is installed once alongside
// the other permanent bridges in both New and LoadFromData.
//
// A no-op (not an error) if the stabilized character doesn't resolve to a
// seated player, mirroring subscribeCharacterDiedBridge's same fallback.
func (e *Encounter) subscribeCharacterStabilizedBridge(ctx context.Context) error {
	_, err := dnd5eEvents.CharacterStabilizedTopic.On(e.bus).Subscribe(ctx,
		func(_ context.Context, event dnd5eEvents.CharacterStabilizedEvent) error {
			return e.bridgeCharacterStabilized(core.EntityID(event.CharacterID))
		})
	if err != nil {
		return fmt.Errorf("subscribe character stabilized bridge: %w", err)
	}
	return nil
}

// bridgeCharacterStabilized publishes an EntityStabilizedEvent for the
// stabilized entity. Visibility policy mirrors publishPlayerDied's
// EntityDied projection: the stabilized player is always included
// unconditionally, and any other viewer with LoS to their position is added
// alongside — same "own state is never LoS-gated" reasoning documented on
// publishPlayerDied. No-op if the entity doesn't resolve to a seated player.
func (e *Encounter) bridgeCharacterStabilized(entityID core.EntityID) error {
	p := e.findPlayerByEntityID(entityID)
	if p == nil {
		return nil
	}
	perPlayer := e.deathSaveAudience(p)
	if err := e.broker.Publish(events.NewEntityStabilizedEvent(
		e.data.ID, e.nextSeq(), entityID, perPlayer,
	)); err != nil {
		return fmt.Errorf("publish entity stabilized: %w", err)
	}
	return nil
}

// subscribeDeathSaveRolledBridge installs a PERMANENT subscription to the
// rulebook's DeathSaveRolledTopic on e.bus, bridging EVERY death save roll
// (or automatic damage-while-unconscious failure) to the broker-facing
// DeathSaveRolledEvent (rpg-toolkit#741 — previously zero production
// subscribers, so individual rolls never reached the wire even though the
// terminal Died/Stabilized outcomes now do). Same permanent-lifetime shape
// as the other two death-save bridges: rolls happen at turn-start or on
// damage, not confined to one verb call.
//
// A no-op (not an error) if the rolling character doesn't resolve to a
// seated player.
func (e *Encounter) subscribeDeathSaveRolledBridge(ctx context.Context) error {
	_, err := dnd5eEvents.DeathSaveRolledTopic.On(e.bus).Subscribe(ctx,
		func(_ context.Context, event dnd5eEvents.DeathSaveRolledEvent) error {
			return e.bridgeDeathSaveRolled(event)
		})
	if err != nil {
		return fmt.Errorf("subscribe death save rolled bridge: %w", err)
	}
	return nil
}

// bridgeDeathSaveRolled publishes a DeathSaveRolledEvent carrying the roll
// detail (roll value, running successes/failures, crit flags, and the
// nat-20-revival fields) for the rolling entity. Visibility policy mirrors
// bridgeCharacterStabilized. No-op if the entity doesn't resolve to a
// seated player.
func (e *Encounter) bridgeDeathSaveRolled(event dnd5eEvents.DeathSaveRolledEvent) error {
	entityID := core.EntityID(event.CharacterID)
	p := e.findPlayerByEntityID(entityID)
	if p == nil {
		return nil
	}
	perPlayer := e.deathSaveRolledAudience(p)
	if err := e.broker.Publish(events.NewDeathSaveRolledEvent(&events.NewDeathSaveRolledEventInput{
		EncID:                 e.data.ID,
		Seq:                   e.nextSeq(),
		EntityID:              entityID,
		Roll:                  event.Roll,
		Successes:             event.Successes,
		Failures:              event.Failures,
		IsCriticalFail:        event.IsCriticalFail,
		IsCriticalSuccess:     event.IsCriticalSuccess,
		Stabilized:            event.Stabilized,
		Dead:                  event.Dead,
		RegainedConsciousness: event.RegainedConsciousness,
		HPRestored:            event.HPRestored,
		PerPlayer:             perPlayer,
	})); err != nil {
		return fmt.Errorf("publish death save rolled: %w", err)
	}
	return nil
}

// subscribeHealingReceivedBridge installs a PERMANENT subscription to the
// rulebook's HealingReceivedTopic on e.bus, syncing PlayerData.HP from
// whatever healing just landed on a held character. Same permanent-lifetime
// shape as the other bridges in this file: healing (a nat-20 death save's
// "regain 1 HP," and any future healing spell/feature) can fire from any
// future turn or action, not just once per verb call.
//
// rpg-toolkit#772/#781/#784: nothing synced PlayerData.HP from the held
// character's own HP in the healing direction before this — combat
// damage (applyAndPublishNPCOutcome / applyCapturedDamage) mutates
// PlayerData.HP directly, but character.onHealingReceived only ever
// touched c.hitPoints. A player revived by a nat-20 death save correctly
// regained consciousness and 1 HP server-side (UnconsciousCondition's own
// state, and the Unconscious condition itself, were both correctly
// updated/removed), but PlayerData.HP — and anything reading it, a
// reconnecting client's snapshot included — stayed stuck at 0 forever.
// Found writing the per-RPC-reload regression test for this wave: the
// test's own "has alice been revived" check read PlayerData.HP and could
// not tell a real revival from a mechanism that was still stuck.
//
// Deliberately does NOT read char.GetHitPoints() and take the live value
// wholesale (that was tried and reverted — see git history on this
// function's addition): char.GetHitPoints() is stale-LOW for un-synced
// combat damage (rpg-toolkit#784's still-open general per-hit-sync gap),
// so blindly trusting it here would silently overwrite a correct,
// freshly-damaged PlayerData.HP with a stale, higher, pre-damage value
// every time ToData() ran. Applying the event's own Amount as a targeted
// delta — mirroring exactly what character.onHealingReceived does to
// c.hitPoints, on the same signal, clamped to the same MaxHP — cannot
// make that mistake: it only ever reacts to a genuine healing event, never
// to reading a possibly-stale snapshot.
func (e *Encounter) subscribeHealingReceivedBridge(ctx context.Context) error {
	_, err := dnd5eEvents.HealingReceivedTopic.On(e.bus).Subscribe(ctx,
		func(_ context.Context, event dnd5eEvents.HealingReceivedEvent) error {
			p := e.findPlayerByEntityID(core.EntityID(event.TargetID))
			if p == nil {
				return nil
			}
			p.HP += event.Amount
			if p.MaxHP > 0 && p.HP > p.MaxHP {
				p.HP = p.MaxHP
			}
			return nil
		})
	if err != nil {
		return fmt.Errorf("subscribe healing received bridge: %w", err)
	}
	return nil
}

// deathSaveAudience builds the per-viewer projection shared by the
// Stabilized and (via deathSaveRolledAudience) DeathSaveRolled bridges: the
// rolling/stabilizing player themselves is always included unconditionally
// (same "own state is never LoS-gated" reasoning as publishPlayerDied),
// plus any other viewer with LoS to their current position.
func (e *Encounter) deathSaveAudience(p *PlayerData) map[core.PlayerID]events.EntityStabilizedSlice {
	perPlayer := make(map[core.PlayerID]events.EntityStabilizedSlice)
	perPlayer[p.ID] = events.EntityStabilizedSlice{Visible: true}
	if p.View == nil {
		return perPlayer
	}
	pos := p.View.Position
	for viewerID, viewer := range e.data.Players {
		if viewerID == p.ID {
			continue
		}
		if perception.CanSeeAt(viewer.View, pos, e.room) {
			perPlayer[viewerID] = events.EntityStabilizedSlice{Visible: true}
		}
	}
	return perPlayer
}

// deathSaveRolledAudience is deathSaveAudience's DeathSaveRolledSlice
// counterpart — identical projection, different slice type (the two events
// are not structurally related beyond sharing this visibility policy).
func (e *Encounter) deathSaveRolledAudience(p *PlayerData) map[core.PlayerID]events.DeathSaveRolledSlice {
	perPlayer := make(map[core.PlayerID]events.DeathSaveRolledSlice)
	perPlayer[p.ID] = events.DeathSaveRolledSlice{Visible: true}
	if p.View == nil {
		return perPlayer
	}
	pos := p.View.Position
	for viewerID, viewer := range e.data.Players {
		if viewerID == p.ID {
			continue
		}
		if perception.CanSeeAt(viewer.View, pos, e.room) {
			perPlayer[viewerID] = events.DeathSaveRolledSlice{Visible: true}
		}
	}
	return perPlayer
}

// checkEncounterEnd evaluates the encounter-end predicate and, if true,
// transitions the encounter to ModeEnded and publishes EncounterEndedEvent.
// Returns nil whether or not the predicate fired this call — both callers
// (killEntity, publishPlayerDied) only care whether the check itself
// errored, never whether it actually ended the encounter, so there is no
// "ended" bool to report.
//
// Two predicates, checked in order:
//  1. len(data.Monsters) == 0 (all hostiles defeated) — Wave 2.10, victory.
//  2. allPlayersDead() (every seated player confirmed dead) — Wave 2.11/
//     #772/#782, TPK/defeat.
//
// Encapsulated here so future waves swap or extend the predicate (boss-only
// kill, fled, negotiated peace, time-out) without touching the kill/death
// paths that call this. Victory is checked first: the two predicates are
// not expected to both go true in the same call (they fire from disjoint
// call sites — killEntity for monsters, publishPlayerDied for players — and
// this function returns early once Mode is already ModeEnded), but victory
// winning any theoretical tie is an arbitrary, harmless default.
//
// On transition: clears Initiative + ActiveIdx + Round so the verb-gate
// in EndTurn / TakeAction (which checks len(Initiative)) consistently
// rejects post-end calls with ErrEncounterEnded once the mode check
// above also rejects them. The mode check is the primary gate; clearing
// the turn state keeps the persisted snapshot tidy for clients reading
// it post-end. This part of the transition is reason-agnostic — it runs
// identically for victory and defeat, which is what gives TPK the same
// #752 (condition sweep) and #767 (ExitCombat/economy clear) composition
// victory already had, with no special-casing.
func (e *Encounter) checkEncounterEnd() error {
	if e.data.Mode == core.ModeEnded {
		return nil
	}
	var reason string
	switch {
	case len(e.data.Monsters) == 0:
		reason = EncounterEndedReasonAllHostilesDefeated
	case e.allPlayersDead():
		reason = EncounterEndedReasonTPK
	default:
		return nil
	}
	return e.endWithReason(reason)
}

// End administratively ends the encounter with the given reason, driving
// the SAME terminal transition checkEncounterEnd uses for a natural
// victory/TPK conclusion (Mode -> ModeEnded, Initiative/ActiveIdx/Round
// cleared, combat-scoped conditions swept, ActionEconomy cleared via
// ExitCombat, EncounterEndedEvent published) rather than a parallel path.
// rpg-toolkit#797 (rpg-api#663).
//
// Unlike checkEncounterEnd, End does not evaluate the victory/TPK
// predicates — it forces the transition unconditionally, regardless of the
// encounter's current Mode. This is what makes it usable on a FREE_ROAM
// encounter (checkEncounterEnd's predicates only ever get evaluated from
// combat kill paths — killEntity, publishPlayerDied — so a stuck FREE_ROAM
// encounter can never reach them) as well as a TURN_BASED one.
//
// Returns ErrEncounterEnded if the encounter has already ended — abandoning
// an already-terminal encounter is a caller bug (the caller should have
// seen ModeEnded and not tried again), surfaced rather than silently
// swallowed. This mirrors the same sentinel every other post-end verb
// (TakeAction, EndTurn, NPCAct, SetMode) already returns, so callers can
// treat "encounter already over" identically everywhere.
//
// The caller supplies reason; rpg-api's abandon path passes
// EncounterEndedReasonAbandoned. Any string is accepted with no validation
// — EncounterEndedEvent.Reason is a bare passthrough string all the way to
// the wire (see #783's "no proto or rpg-api change needed" note), so End
// imposes no new constraint on that contract.
func (e *Encounter) End(reason string) error {
	if e.data.Mode == core.ModeEnded {
		return ErrEncounterEnded
	}
	return e.endWithReason(reason)
}

// endWithReason performs the terminal-state transition shared by every
// encounter-end path (natural, via checkEncounterEnd, and administrative,
// via End): flips Mode -> ModeEnded, clears Initiative/ActiveIdx/Round,
// sweeps combat-scoped conditions + ActionEconomy, and publishes
// EncounterEndedEvent with the given reason. Callers are responsible for
// the ModeEnded re-entrancy guard — this function assumes the caller
// already confirmed e.data.Mode != core.ModeEnded, so it is not itself
// idempotent.
func (e *Encounter) endWithReason(reason string) error {
	e.data.Mode = core.ModeEnded
	e.data.Initiative = nil
	e.data.ActiveIdx = 0
	e.data.Round = 0

	// Sweep combat-scoped conditions (rage, etc.) BEFORE publishing the
	// terminal event, so any StatusRemoved a condition emits lands ahead of
	// EncounterEndedEvent in sequence — clients see the drop, then the
	// terminal transition, rather than a stale status surviving past "over."
	// rpg-toolkit#752: without this, a raging character survives to
	// ToData() with the condition still in Conditions, and it silently
	// re-hydrates into the character's NEXT encounter.
	//
	// Best-effort: a sweep failure must NOT skip the terminal publish below.
	// Mode is already flipped to ModeEnded above — if we returned here
	// without publishing, the encounter would be stuck ended with no client
	// ever told so (every subsequent verb fails ErrEncounterEnded with no
	// recovery path), which is strictly worse than a condition failing to
	// sweep. Capture the error and surface it after the publish instead
	// (Copilot review, PR #753).
	sweepErr := e.endCombatForPlayers()

	if err := e.broker.Publish(events.NewEncounterEndedEvent(
		e.data.ID, e.nextSeq(),
		reason,
		e.allViewersEncounterEnded(),
	)); err != nil {
		return fmt.Errorf("publish encounter ended: %w", err)
	}
	if sweepErr != nil {
		return fmt.Errorf("end combat for players: %w", sweepErr)
	}
	return nil
}

// allPlayersDead reports whether every seated player carries Dead=true —
// the TPK predicate (rpg-toolkit#772/#782). False when there are no
// players at all (an empty encounter is not a TPK); in practice this is
// only ever called from checkEncounterEnd, itself only ever called
// (for the defeat branch) from publishPlayerDied right after it just
// confirmed one player's death, guaranteeing len(data.Players) > 0.
func (e *Encounter) allPlayersDead() bool {
	if len(e.data.Players) == 0 {
		return false
	}
	for _, p := range e.data.Players {
		if !p.Dead {
			return false
		}
	}
	return true
}

// endCombatForPlayers publishes CombatEnd for every hydrated player
// character, giving combat-scoped conditions (e.g. raging) the same
// self-terminating opportunity LongRest/ShortRest already give them via
// RestEvent (see character.Character.EndCombat, conditions.RagingCondition
// .onCombatEnd). Conditions with no reason to end at combat's close (a
// curse, say) simply never subscribed to CombatEndTopic, so they are
// untouched — this sweeps by opt-in, not by an encounter-side allowlist of
// condition types.
//
// Also calls character.Character.ExitCombat (rpg-toolkit#767), clearing
// ActionEconomy back to nil for every held player. ExitCombat's own doc
// says "Call this when the encounter ends" but nothing ever did —
// checkEncounterEnd (this function's caller) is the toolkit's SOLE
// transition point to ModeEnded (SetMode explicitly rejects ModeEnded — see
// its doc), so it is the correct and complete place for it: ActionEconomy
// is a flat, encounter-unscoped field on the persisted character
// field on the persisted character (rulebooks/dnd5e/character/data.go),
// and ToData()/LoadFromData round-trip whatever is in it verbatim with no
// encounter-identity check. Without this, a character who ever finished a
// fight carried their depleted economy (e.g. movement_remaining: 0) into
// every SUBSEQUENT encounter forever — found live via Redis inspection in
// The Dungeon wave 1's closing playtest (rpg-api PR #645, commit 759eca9),
// which added a StartEncounter-side defensive clear as a backstop, NOT the
// fix. That backstop stays in place after this change: checkEncounterEnd's
// predicate is len(data.Monsters)==0, so it never fires on a TPK (all
// players dead, monsters alive) — an abandoned/TPK'd encounter's
// participants still need the StartEncounter backstop to catch stale
// economy on their next encounter, until a TPK-end predicate exists
// (tracked separately).
//
// Flat stat-snapshot seats (no hydrated *character.Character — heldCharacter
// returns nil) have nothing to sweep and are skipped.
//
// ctx: not threaded through checkEncounterEnd's callers today — killEntity
// is invoked from both ctx-having and ctx-less call sites. context.Background()
// per the existing precedent (applyUnconsciousOnZeroHP, combat_phased.go).
func (e *Encounter) endCombatForPlayers() error {
	ctx := context.Background()
	for _, pd := range e.data.Players {
		char := e.heldCharacter(pd.EntityID)
		if char == nil {
			continue
		}
		if err := char.EndCombat(ctx); err != nil {
			return fmt.Errorf("end combat for %q: %w", pd.EntityID, err)
		}
		if _, err := char.ExitCombat(ctx, &character.ExitCombatInput{}); err != nil {
			return fmt.Errorf("exit combat for %q: %w", pd.EntityID, err)
		}
	}
	return nil
}

// spliceFromInitiative removes id from Initiative (if present) and adjusts
// ActiveIdx to preserve the "currently-active actor" pointer:
//
//   - Removed index <  ActiveIdx: everyone after it shifted left, so
//     ActiveIdx must decrement.
//   - Removed index == ActiveIdx: the active actor itself was removed.
//     ActiveIdx stays in place so EndTurn naturally moves to the next
//     actor on the next call. (Wave 2.10 doesn't expect this in the
//     monster-killed-by-player path because the active actor is the
//     attacking player, not the target. NPC-killed-NPC is out of scope.)
//   - Removed index >  ActiveIdx: no shift affecting the pointer.
//
// After removal, if Initiative is shorter than ActiveIdx (e.g. tail
// removal at the wrap edge), ActiveIdx clamps to 0 to avoid a stale OOB
// pointer; the next EndTurn will publish a TurnStarted for whichever
// actor occupies index 0.
func (e *Encounter) spliceFromInitiative(id core.EntityID) {
	idx := -1
	for i, eid := range e.data.Initiative {
		if eid == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	e.data.Initiative = append(e.data.Initiative[:idx], e.data.Initiative[idx+1:]...)
	if idx < e.data.ActiveIdx {
		e.data.ActiveIdx--
	}
	if e.data.ActiveIdx >= len(e.data.Initiative) {
		// Wrap to the start; if Initiative is now empty the next combat
		// verb will fail the len(Initiative) > 0 gate first.
		e.data.ActiveIdx = 0
	}
}

// positionFor returns the last-known hex of the entity (player or monster)
// matching id. The boolean is false when id is unknown — the caller treats
// "no killer position" as "killer-side LoS contributes nothing to the
// per-viewer projection."
func (e *Encounter) positionFor(id core.EntityID) (core.Hex, bool) {
	if id == "" {
		return core.Hex{}, false
	}
	if p := e.findPlayerByEntityID(id); p != nil && p.View != nil {
		return p.View.Position, true
	}
	if m, ok := e.data.Monsters[id]; ok {
		return m.Position, true
	}
	return core.Hex{}, false
}

// allViewersEntityRemoved builds a per-player slice marking every player
// as a viewer of the removal — entity-removal is broadcast.
func (e *Encounter) allViewersEntityRemoved() map[core.PlayerID]events.EntityRemovedSlice {
	out := make(map[core.PlayerID]events.EntityRemovedSlice, len(e.data.Players))
	for id := range e.data.Players {
		out[id] = events.EntityRemovedSlice{Visible: true}
	}
	return out
}

// allViewersEncounterEnded builds a per-player slice marking every player
// as a viewer of the terminal transition — encounter-end is broadcast.
func (e *Encounter) allViewersEncounterEnded() map[core.PlayerID]events.EncounterEndedSlice {
	out := make(map[core.PlayerID]events.EncounterEndedSlice, len(e.data.Players))
	for id := range e.data.Players {
		out[id] = events.EncounterEndedSlice{Visible: true}
	}
	return out
}
