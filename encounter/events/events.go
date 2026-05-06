package events

import "github.com/KirkDiggler/rpg-toolkit/encounter/core"

// EncounterEvent is the sealed sum type of events the broker carries.
//
// External packages cannot implement this interface — the marker method
// isEncounterEvent() is unexported, and only types declared in this
// package can satisfy it. Consumers type-switch on the concrete type.
type EncounterEvent interface {
	isEncounterEvent()
	EncounterID() core.EncounterID
	Sequence() uint64
	Audience() AudienceSet
}

// AudienceSet is the set of player IDs that can perceive an event.
// Slice (not map) is sufficient — the broker only iterates.
//
// Lives with events because it's a routing concept tied to the event
// taxonomy, not a general-purpose primitive.
type AudienceSet []core.PlayerID

// audienceFromMap derives the audience slice from a PerPlayer map's keys.
// Used by each concrete event's Audience() method.
func audienceFromMap[V any](m map[core.PlayerID]V) AudienceSet {
	out := make(AudienceSet, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
