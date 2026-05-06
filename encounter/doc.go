// Package encounter implements the encounter SDK — the orchestrator-facing
// facade for running an encounter (combat, free-roam, social) end-to-end.
//
// An Encounter is a transient object. Game servers Load it from persisted
// state, mutate via verb methods (Move, OpenDoor, ...), serialize back via
// ToData, and save. Player-facing events flow through a process-scoped
// Broker that publishes per-player projected events through a pluggable
// Transport (InMemoryTransport, RedisTransport, ...).
//
// Internal layout:
//
//	encounter/core        — IDs (EncounterID, PlayerID, EntityID) + spatial primitives (Hex, HexSet)
//	encounter/events      — sealed EncounterEvent interface + concrete events + AudienceSet
//	encounter/perception  — View + projection functions
//	encounter (top-level) — Encounter aggregate, Broker, Transport
//
// See rpg-project/ideas/encounter/v1alpha2/sdk-direction.md for the design.
package encounter
