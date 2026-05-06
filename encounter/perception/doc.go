// Package perception computes per-player vision projections for the
// encounter SDK. Pure functions, testable in isolation, no broker or
// transport dependencies.
//
// Slice 1 ships a Manhattan-radius stub for line-of-sight. Real LoS (with
// walls, lighting, and senses like darkvision/blindsight/blinded
// conditions) is a future slice under the same package.
//
// Lives as a subpackage of encounter for slice 1 to keep the module graph
// acyclic. Future slices may extract this to rpg-toolkit/perception/ once
// shared primitive types (Hex, etc.) move to tools/spatial.
package perception
