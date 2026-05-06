package events

import "github.com/KirkDiggler/rpg-toolkit/encounter/types"

// EncounterEvent is the sealed sum type of events the broker carries.
//
// External packages cannot implement this interface — the marker method
// isEncounterEvent() is unexported, and only types declared in this
// package can satisfy it. Consumers type-switch on the concrete type.
type EncounterEvent interface {
	isEncounterEvent()
	EncounterID() types.EncounterID
	Sequence() uint64
	Audience() types.AudienceSet
}

// audienceFromMap derives the audience slice from a PerPlayer map's keys.
// Used by each concrete event's Audience() method.
func audienceFromMap[V any](m map[types.PlayerID]V) types.AudienceSet {
	out := make(types.AudienceSet, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
