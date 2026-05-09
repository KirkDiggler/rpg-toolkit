package encounter

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/events"
	"github.com/KirkDiggler/rpg-toolkit/encounter/perception"
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
// player OR the killing NPC. The dying player themselves are always
// considered to perceive their own death (their position is, by
// definition, in their own view).
func (e *Encounter) publishPlayerDied(playerEntityID, killerID core.EntityID) error {
	playerData := e.findPlayerByEntityID(playerEntityID)
	if playerData == nil || playerData.View == nil {
		return fmt.Errorf("publishPlayerDied: player entity %q not found", playerEntityID)
	}
	dyingPos := playerData.View.Position
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
		e.data.ID, e.nextSeq(), playerEntityID, killerID, diedPerPlayer,
	)); err != nil {
		return fmt.Errorf("publish entity died (player): %w", err)
	}
	return nil
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
