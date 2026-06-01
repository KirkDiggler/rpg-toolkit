package core

// EncounterID uniquely identifies an encounter instance.
type EncounterID string

// PlayerID uniquely identifies a player seat in an encounter.
type PlayerID string

// EntityID uniquely identifies any entity in an encounter (player char,
// monster, prop, door, ...).
type EntityID string

// CorrelationID ties together the events produced by a single causal action.
// Every event the encounter publishes while resolving one action (the
// resolved-action event plus its effect events — damage, condition, resource)
// carries the same CorrelationID, so a downstream consumer (the toolkit-owned
// combat log per North-Star Invariant 8) can reassemble "this damage came from
// that attack" without relying on adjacent sequence numbers.
//
// Empty when an event is not part of a correlated action group (e.g. mode
// changes, turn boundaries — emitted outside any single action's resolution).
type CorrelationID string
