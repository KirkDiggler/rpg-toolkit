// Package events defines the EncounterEvent taxonomy — typed concretes
// under a sealed interface (AWS v2 SDK AttributeValue pattern).
//
// Each concrete event has its own struct, fields, and per-player slice
// type. The unexported isEncounterEvent() marker makes the interface
// externally unsatisfiable, giving compile-time bounded sum semantics.
//
// Cause vs effect:
//   - Cause events describe what happened in the world (MoveEvent,
//     DoorOpenedEvent, ConditionRemovedEvent, ...).
//   - Effect events describe per-player perception change (HexRevealedEvent,
//     and future HexHiddenEvent for vision-loss).
//
// See ../../sdk-direction.md for the design.
package events
