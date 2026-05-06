// Package core holds the encounter SDK's primitive value types — IDs and
// spatial primitives shared across the SDK's internal subpackages (events,
// perception) and the top-level encounter package. It exists to keep the
// internal package graph acyclic: events and perception import core;
// encounter imports all three.
//
// Hex and HexSet may move to rpg-toolkit/tools/spatial in a future slice
// once the encounter SDK is ready to depend on the spatial module
// directly. ID types stay here long-term.
package core
