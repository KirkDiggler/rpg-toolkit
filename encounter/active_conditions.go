package encounter

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// activeConditionRefs extracts the canonical ref string (e.g.
// "dnd5e:conditions:raging") for each currently-applied condition, for the
// PlayerData.ActiveConditions / MonsterData.ActiveConditions snapshot
// projection (rpg-toolkit#754).
//
// Takes the ALREADY-serialized condition blobs — character.Data.Conditions /
// monster.Data.Conditions, as returned by the ToData() call
// syncCombatantsToData already makes to build DataJSON — rather than
// re-serializing each live condition via GetConditions()+ToJSON() a second
// time. ToData() internally loops every held condition and calls ToJSON()
// once per condition to build exactly this slice; calling GetConditions()
// here as well paid that ToJSON() cost twice per combatant per RPC (Copilot
// + gate review on PR #776). Safe to read post-load: entities reaching
// syncCombatantsToData were hydrated via the encounter's LoadFromData
// cascade (monster.LoadFromData, never monster.New), so
// monster.Monster.traitData — the pre-bus staging slice AddTraitData
// populates, only reachable from factory construction — is always empty by
// the time ToData() runs here, meaning Data.Conditions is exactly
// serialize(GetConditions()), no more and no less.
//
// Every condition Data type in this rulebook (RagingData, DisengagingData,
// UnconsciousData, the monstertraits condition types, etc.) leads with a
// `Ref *core.Ref` field — confirmed across the condition package rather
// than assumed. Peeking at that shared field on each already-serialized
// blob is therefore sufficient; no rulebook-specific type-switch is needed
// here, matching the toolkit's existing rulebook-agnostic dispatch
// elsewhere (e.g. applyCapturedConditions in npc.go never switches on
// condition kind either).
//
// A blob that fails to unmarshal, or omits "ref", is skipped rather than
// failing the whole snapshot — this is best-effort visibility, not a hard
// dependency, mirroring monster.Monster.ToData's own "skip conditions that
// can't be serialized" precedent. Returns nil (not an empty slice) when
// nothing qualifies, so the omitempty JSON tag actually omits the field.
func activeConditionRefs(conditionBlobs []json.RawMessage) []string {
	if len(conditionBlobs) == 0 {
		return nil
	}
	refs := make([]string, 0, len(conditionBlobs))
	for _, raw := range conditionBlobs {
		var wire struct {
			Ref *core.Ref `json:"ref"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil || wire.Ref == nil {
			continue
		}
		refs = append(refs, wire.Ref.String())
	}
	if len(refs) == 0 {
		return nil
	}
	return refs
}
