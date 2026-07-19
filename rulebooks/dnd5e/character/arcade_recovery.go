package character

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// RestoreForNewEncounter applies arcade recovery (rpg-toolkit#785) to a
// character's persisted data before it is seated in a BRAND NEW encounter:
// death is an encounter-scoped outcome, not a persistent character state.
// A character carrying 0 HP or less from a prior encounter — whether a
// confirmed 3-failed-death-save death or an unresolved TPK snapshot — is
// restored to full HP with its death-save state cleared and its Unconscious
// condition removed, as if walking into a fresh fight healthy. A character
// already above 0 HP is untouched: this is recovery from death/
// incapacitation, not a free heal on every new fight (that would quietly
// obsolete resting, which stays a separate, real system for later).
//
// Contract — read before calling this from anywhere new: it fires only at
// first seating, never on rehydration. Callers own distinguishing the two.
// encounter.LoadFromData — the per-RPC reload of an EXISTING seat — must
// NEVER call this; only a new-seat path (encounter.AddPlayer) should.
// Calling it on an already-restored or already-healthy character is a
// harmless no-op.
//
// Scope (Kirk's decision on #785): HP and death-save state only. Ability
// uses, class resources, spell slots, and hit dice are deliberately left
// untouched — "not hardcore" currently means recovery from death, not a
// free short/long rest. Revisit explicitly if that scope changes; don't
// fold it in silently here.
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
// Returns true iff it actually restored something, so a caller (today,
// encounter.AddPlayer) knows whether to also resync any HP snapshot it
// keeps alongside d — see that call site's own doc for why the two must
// not be allowed to diverge.
func RestoreForNewEncounter(d *Data) bool {
	if d == nil || d.HitPoints > 0 {
		return false
	}
	d.HitPoints = d.MaxHitPoints
	d.DeathSaveState = nil
	d.Conditions = stripConditionByRef(d.Conditions, refs.Conditions.Unconscious())
	return true
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
