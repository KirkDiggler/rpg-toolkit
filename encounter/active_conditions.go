package encounter

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/core"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// activeConditionRefs extracts the canonical ref string (e.g.
// "dnd5e:conditions:raging") for each currently-applied condition, for the
// PlayerData.ActiveConditions / MonsterData.ActiveConditions snapshot
// projection (rpg-toolkit#754).
//
// Every condition Data type in this rulebook (RagingData, DisengagingData,
// UnconsciousData, the monstertraits condition types, etc.) leads with a
// `Ref *core.Ref` field — confirmed across the condition package rather
// than assumed. Serializing each condition (ToJSON, the same method ToData
// already calls per condition) and peeking at that shared field is
// therefore sufficient; no rulebook-specific type-switch is needed here,
// matching the toolkit's existing rulebook-agnostic dispatch elsewhere
// (e.g. applyCapturedConditions in npc.go never switches on condition
// kind either).
//
// A condition that fails to serialize, or whose JSON omits "ref", is
// skipped rather than failing the whole snapshot — this is best-effort
// visibility, not a hard dependency, mirroring monster.Monster.ToData's own
// "skip conditions that can't be serialized" precedent. Returns nil (not an
// empty slice) when nothing qualifies, so the omitempty JSON tag actually
// omits the field.
func activeConditionRefs(conds []dnd5eEvents.ConditionBehavior) []string {
	if len(conds) == 0 {
		return nil
	}
	refs := make([]string, 0, len(conds))
	for _, cond := range conds {
		raw, err := cond.ToJSON()
		if err != nil {
			continue
		}
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
