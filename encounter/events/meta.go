package events

import (
	"time"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// eventMeta is the spine metadata every encounter event carries beyond its
// encounter id and sequence: the game-event timestamp stamped at publish
// (Invariant 5) and the correlation id tying effect events to their causing
// action (Invariant 8).
//
// Concrete events embed eventMeta so the OccurredAt / CorrelationID accessors
// and the Stamp mutator are single-sourced here — adding a field to the spine
// touches one struct, not every event. Embedding also satisfies the new
// EncounterEvent interface methods for free, so no per-event boilerplate.
//
// Each event's own MarshalJSON / UnmarshalJSON wire struct routes these
// through stable JSON field names (`occurred_at`, `correlation_id`) via the
// toWire / fromWire helpers; the promoted methods stay the read/write surface
// for code. Fields are unexported and prefixed to avoid colliding with the
// promoted OccurredAt / CorrelationID method names.
type eventMeta struct {
	eventOccurredAt    time.Time
	eventCorrelationID core.CorrelationID
}

// OccurredAt returns the game-event time this event was stamped with at
// publish (Invariant 5). Promoted onto every embedding event; part of the
// EncounterEvent interface.
func (m *eventMeta) OccurredAt() time.Time { return m.eventOccurredAt }

// CorrelationID returns the correlation id grouping this event with the other
// events of the action that caused it (Invariant 8). Empty when the event is
// not part of a correlated action group. Part of the EncounterEvent interface.
func (m *eventMeta) CorrelationID() core.CorrelationID { return m.eventCorrelationID }

// Stamp sets the spine metadata. Two callers, two responsibilities:
//   - The encounter sets the correlation id before publish (via
//     publishCorrelated, passing a zero time) so an action's effect events
//     group under one id (Invariant 8).
//   - The Broker re-stamps at the literal publish moment to set game-event time
//     (Invariant 5), preserving the correlation id already set. The broker is
//     therefore the single timestamp authority; "game-event time at publish" is
//     literal because the broker is the one publish point.
//
// Part of the EncounterEvent interface.
func (m *eventMeta) Stamp(at time.Time, corr core.CorrelationID) {
	m.eventOccurredAt = at
	m.eventCorrelationID = corr
}

// metaWire is the JSON shape for the spine metadata, embedded into each
// event's own wire struct so the two fields serialize under stable names.
type metaWire struct {
	OccurredAt    time.Time          `json:"occurred_at"`
	CorrelationID core.CorrelationID `json:"correlation_id,omitempty"`
}

func (m *eventMeta) toWire() metaWire {
	return metaWire{OccurredAt: m.eventOccurredAt, CorrelationID: m.eventCorrelationID}
}

func (m *eventMeta) fromWire(w metaWire) {
	m.eventOccurredAt = w.OccurredAt
	m.eventCorrelationID = w.CorrelationID
}
