//nolint:revive // package name "types" is purposeful — see doc.go
package types

// EncounterID uniquely identifies an encounter instance.
type EncounterID string

// PlayerID uniquely identifies a player seat in an encounter.
type PlayerID string

// EntityID uniquely identifies any entity in an encounter (player char,
// monster, prop, door, ...).
type EntityID string

// AudienceSet is the set of player IDs that can perceive an event.
// Slice (not map) is sufficient — the broker only iterates.
type AudienceSet []PlayerID
