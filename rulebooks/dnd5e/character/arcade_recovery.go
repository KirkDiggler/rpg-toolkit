package character

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// RestoreForNewEncounter applies arcade recovery to a character's persisted
// data before it is seated in a BRAND NEW encounter — an arcade run start,
// not a real-world rest. Two independent restorations, gated differently:
//
//  1. HP / death-save / Unconscious (rpg-toolkit#785): death is an
//     encounter-scoped outcome, not a persistent character state. A
//     character carrying 0 HP or less from a prior encounter — whether a
//     confirmed 3-failed-death-save death or an unresolved TPK snapshot —
//     is restored to full HP with its death-save state cleared and its
//     Unconscious condition removed, as if walking into a fresh fight
//     healthy. Gated on HitPoints <= 0: a character already alive is not
//     touched by this branch (no free heal on every new fight).
//  2. Resource pools (rpg-toolkit#795): every tracked resource in
//     d.Resources (rage charges, ki, hit dice, and anything else a future
//     feature keys into that map — see restoreResourcePools) refreshes to
//     its maximum. UNGATED — this runs regardless of HitPoints, on every
//     new seating: an ALIVE barbarian who spent rage earlier in the run
//     gets it back too, not only a barbarian who died. This is the scope
//     change #795 makes: #785's "ability uses, class resources, spell
//     slots deliberately left untouched" note no longer holds for
//     resources specifically — this is where that's revisited, as its own
//     doc promised. Data.ClassResources and Data.SpellSlots are NOT
//     touched: neither is ever decremented by any live consumption path
//     today (confirmed by grep — Character.UseResource/IsResourceAvailable
//     only ever read/write d.Resources), so there is nothing in them to
//     restore; wiring them in now would be dead code guarding against a
//     mechanic that doesn't exist yet.
//
// Contract — read before calling this from anywhere new: it fires only at
// first seating, never on rehydration. Callers own distinguishing the two.
// encounter.LoadFromData — the per-RPC reload of an EXISTING seat — must
// NEVER call this; only a new-seat path (encounter.AddPlayer) should.
// Calling it on an already-restored or already-healthy, already-full-
// resource character is a harmless no-op.
//
// Why the Unconscious condition must be stripped, not left to re-hydrate:
// conditions.UnconsciousCondition does not subscribe to CombatEndTopic, so
// it survives the encounter package's end-of-combat condition sweep
// (rpg-toolkit#753) even after a TPK (rpg-toolkit#783 added TPK-ends-the-
// encounter, but its sweep only reaches CombatEndTopic subscribers).  Left
// in d.Conditions, the next encounter's hydration cascade would re-Apply()
// it — resubscribing turn-start death-save rolls onto a character now at
// full HP, recreating the exact incoherent state this fix removes, just
// shaped differently.
//
// Returns true iff it actually restored something — HP/death-save OR at
// least one resource pool — so a caller (today, encounter.AddPlayer) knows
// whether to also resync any HP snapshot it keeps alongside d and whether
// to persist the (possibly resource-only) change. Because resource
// restoration is now ungated, this returns true for most resource-bearing
// characters on most new seatings, not only revived ones — see that call
// site's own doc for why HP/DataJSON must not be allowed to diverge.
func RestoreForNewEncounter(d *Data) bool {
	if d == nil {
		return false
	}
	restored := false

	if d.HitPoints <= 0 {
		d.HitPoints = d.MaxHitPoints
		d.DeathSaveState = nil
		d.Conditions = stripConditionByRef(d.Conditions, refs.Conditions.Unconscious())
		restored = true
	}

	if restoreResourcePools(d) {
		restored = true
	}

	return restored
}

// restoreResourcePools resets every entry in d.Resources to its own
// Maximum — generic over whatever keys are present (rage charges, ki, hit
// dice today; anything else a future feature adds to this map
// automatically restores too, no code change needed here). This is the
// "one system" version of arcade recovery for resources: no per-class
// special-casing, because d.Resources is already the single generic pools
// map every live resource-consuming feature reads and writes through
// (Character.UseResource/IsResourceAvailable, core/resources.ResourceKey).
// Returns true iff at least one pool was actually below its maximum.
func restoreResourcePools(d *Data) bool {
	changed := false
	for key, res := range d.Resources {
		if res.Current == res.Maximum {
			continue
		}
		res.Current = res.Maximum
		d.Resources[key] = res
		changed = true
	}
	return changed
}

// stripConditionByRef returns blobs with any condition matching ref
// removed, identified by peeking each blob's leading Ref field rather than
// fully deserializing via conditions.LoadJSON — this only needs identity,
// not behavior, mirroring encounter.activeConditionRefs' same peek pattern
// (every condition Data type in this rulebook leads with a `Ref *core.Ref`
// field). A blob that fails to unmarshal or omits "ref" is kept as-is:
// fail open, since this function has no way to know it's safe to drop
// something it can't identify.
func stripConditionByRef(blobs []json.RawMessage, ref *core.Ref) []json.RawMessage {
	if len(blobs) == 0 || ref == nil {
		return blobs
	}
	out := make([]json.RawMessage, 0, len(blobs))
	for _, raw := range blobs {
		var wire struct {
			Ref *core.Ref `json:"ref"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil || wire.Ref == nil {
			out = append(out, raw)
			continue
		}
		if wire.Ref.Equals(ref) {
			continue
		}
		out = append(out, raw)
	}
	return out
}
