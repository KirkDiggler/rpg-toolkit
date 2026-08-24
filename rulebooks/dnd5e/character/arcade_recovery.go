package character

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// RestoreForLaunch applies arcade recovery to a character's persisted
// data before it is seated at a game LAUNCH — an arcade run start, not a
// real-world rest. (Named RestoreForNewEncounter until rpg-toolkit#1225
// renamed it: "encounter" stopped describing the call site once the only
// legitimate caller became the host's launch path — the old name invited
// exactly the mid-run misuse the contract below forbids.) Two independent restorations, both ungated:
//
//  1. HP / death-save / Unconscious (rpg-toolkit#785, ungated by #1225):
//     death and damage are run-scoped outcomes, not persistent character
//     state. EVERY seated character is restored to full HP with its
//     death-save state cleared and its Unconscious condition removed —
//     whether it arrived dead (3 failed death saves or an unresolved TPK
//     snapshot) or merely wounded from an earlier run. The original
//     HitPoints <= 0 gate ("no free heal on every new fight") guarded the
//     OLD stack's per-encounter reseat through encounter.AddPlayer; on the
//     session stack the only caller is the host's launch path and fights
//     afterwards form by sighting, so a full heal here is a run start, not
//     a mid-run freebie (Kirk's ruling, rpg-project#253 walk 2026-08-24).
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
//     touched — see restoreResourcePools' doc for why (tracked as
//     rpg-toolkit#800 and #799 respectively, both out of this scope).
//
// Contract — read before calling this from anywhere new: it fires only at
// launch seating, never on rehydration. Callers own distinguishing the
// two. A per-RPC reload of an EXISTING seat must NEVER call this; only the
// host's launch path (today, rpg-api's StartEncounter before it Joins each
// member) should — mid-run, the full heal this now performs would be a
// free heal, which is exactly what launch-only scoping prevents. Calling
// it on an already-healthy, already-full-resource character is a harmless
// no-op.
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
// least one resource pool — so the caller knows whether the record needs
// persisting before it is seated. A character already at full HP with full
// pools returns false.
func RestoreForLaunch(d *Data) bool {
	if d == nil {
		return false
	}
	restored := false

	if d.HitPoints < d.MaxHitPoints {
		d.HitPoints = d.MaxHitPoints
		restored = true
	}
	if d.DeathSaveState != nil {
		d.DeathSaveState = nil
		restored = true
	}
	// Stripped unconditionally: an Unconscious blob with HP already at max is
	// exactly the incoherent state this function exists to remove.
	if stripped := stripConditionByRef(d.Conditions, refs.Conditions.Unconscious()); len(stripped) != len(d.Conditions) {
		d.Conditions = stripped
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
//
// Deliberate divergence from RAW rest rules, worth calling out explicitly
// (arcade semantics, not a bug): hit dice's own RestEvent-triggered
// recovery (dnd5eResources.NewHitDiceResource's RecoveryFunc,
// combat/recoverable_resource.go) restores only HALF of maximum
// (minimum 1) on a long rest, per PHB p.186 — but a fresh arcade seating
// restores hit dice to their FULL maximum here, same as every other pool.
// The point of arcade recovery is a clean run start, not a simulated rest;
// applying the half-on-long-rest rule would be re-litigating a real-world
// rest mechanic this function isn't modeling.
//
// Does NOT touch Data.ClassResources (dead map, never written outside
// Finalize — cleanup tracked as rpg-toolkit#800) or Data.SpellSlots
// (orphaned: no live consumption, no reset even on LongRest — tracked as
// rpg-toolkit#799). Both are out of this function's scope; restoring
// fields nothing spends from would be dead code guarding mechanics that
// don't exist yet.
//
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
