package encounter

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
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
// "tpk", "time_out", etc.).
const EncounterEndedReasonAllHostilesDefeated = "all_hostiles_defeated"

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
	for viewerID, viewer := range e.data.Players {
		seesDying := perception.CanSeeAt(viewer.View, dyingPos)
		seesKiller := killerHasPos && perception.CanSeeAt(viewer.View, killerPos)
		if !seesDying && !seesKiller {
			continue
		}
		diedPerPlayer[viewerID] = events.EntityDiedSlice{Visible: true}
	}
	if err := e.broker.Publish(events.NewEntityDiedEvent(
		e.data.ID, e.nextSeq(), monsterID, killerID, diedPerPlayer,
	)); err != nil {
		return fmt.Errorf("publish entity died: %w", err)
	}

	// Mutate state: drop from monsters and splice out of initiative.
	delete(e.data.Monsters, monsterID)
	e.spliceFromInitiative(monsterID)

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
	if _, err := e.checkEncounterEnd(); err != nil {
		return err
	}
	return nil
}

// publishPlayerDied fires an EntityDiedEvent for a player whose HP reached
// zero, with NPC-as-killer visibility projection. It does NOT mutate any
// state and does NOT publish EntityRemovedEvent — player dying-state is
// Wave 2.11+ territory. Wave 2.10 limits the player-death surface to a
// single narrative event so the web can surface "alice was downed by
// goblin" without committing to a dying-state model that hasn't shipped.
//
// Visibility: a viewer is in PerPlayer iff they have LoS to the dying
// player OR the killing NPC. The dying player themselves is ALWAYS included
// unconditionally (rpg-toolkit#741 hardening) — a player learning of their
// own death must never depend on a LoS/perception computation over their
// own position; that dependency is real (CanSeeAt reads live position +
// sight range) but conceptually unnecessary, and any future change to LoS
// rules (multi-room, forced movement, senses) should not be able to hide a
// player's own death from them.
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
		seesDying := perception.CanSeeAt(viewer.View, dyingPos)
		seesKiller := killerHasPos && perception.CanSeeAt(viewer.View, killerPos)
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
		if perception.CanSeeAt(viewer.View, pos) {
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
		if perception.CanSeeAt(viewer.View, pos) {
			perPlayer[viewerID] = events.DeathSaveRolledSlice{Visible: true}
		}
	}
	return perPlayer
}

// checkEncounterEnd evaluates the encounter-end predicate and, if true,
// transitions the encounter to ModeEnded and publishes EncounterEndedEvent.
// Returns (ended, err) — ended is true when the predicate fired this call.
//
// Wave 2.10 predicate: len(data.Monsters) == 0 (all hostiles defeated).
// Encapsulated here so future waves swap the predicate (boss-only kill,
// fled, negotiated peace, time-out) without touching the kill path.
//
// On transition: clears Initiative + ActiveIdx + Round so the verb-gate
// in EndTurn / TakeAction (which checks len(Initiative)) consistently
// rejects post-end calls with ErrEncounterEnded once the mode check
// above also rejects them. The mode check is the primary gate; clearing
// the turn state keeps the persisted snapshot tidy for clients reading
// it post-end.
func (e *Encounter) checkEncounterEnd() (bool, error) {
	if e.data.Mode == core.ModeEnded {
		return false, nil
	}
	if len(e.data.Monsters) > 0 {
		return false, nil
	}
	e.data.Mode = core.ModeEnded
	e.data.Initiative = nil
	e.data.ActiveIdx = 0
	e.data.Round = 0

	if err := e.broker.Publish(events.NewEncounterEndedEvent(
		e.data.ID, e.nextSeq(),
		EncounterEndedReasonAllHostilesDefeated,
		e.allViewersEncounterEnded(),
	)); err != nil {
		return true, fmt.Errorf("publish encounter ended: %w", err)
	}
	return true, nil
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
