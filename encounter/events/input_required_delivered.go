// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// InputRequiredDeliveredEvent notifies a single reactor that an input prompt
// has been queued for them on the encounter's pending state.
//
// This is the metadata-only event for prompts that arrive on the per-viewer
// stream rather than as the response of the calling RPC. The actual prompt
// content (e.g. reaction_ref + trigger_kind for Wave 2.11d reaction prompts)
// lives on Encounter.Data.PendingReactionPrompts and is read by the wire-side
// translator when this event arrives. Keeping the event metadata-only matches
// the pat-encounter-pending-prompt pattern used for skill checks: the SDK
// event signals "there is now a pending prompt"; the canonical prompt content
// is the encounter Data state.
//
// Audience is single-viewer: only ReactorID receives this event. The
// audience-of-one routing keeps reaction prompts from leaking to other
// players' streams.
//
// PromptKind is a discriminator so future prompt types can reuse this event
// shape without adding new event variants. Wave 2.11d uses "reaction"; Wave
// 2.9-style skill checks could reuse this with kind "skill_check" (today
// they ride the response payload directly).
type InputRequiredDeliveredEvent struct {
	eventMeta
	encID      core.EncounterID
	seq        uint64
	ReactorID  core.PlayerID
	PromptKind string // "reaction" (Wave 2.11d) | future kinds
}

// PromptKindReaction identifies the reaction-prompt variant of
// InputRequiredDeliveredEvent. The wire-side translator reads
// PendingReactionPrompts[ReactorID] for the prompt content.
const PromptKindReaction = "reaction"

// NewInputRequiredDeliveredEvent constructs an InputRequiredDeliveredEvent
// targeted at a single reactor.
func NewInputRequiredDeliveredEvent(
	encID core.EncounterID,
	seq uint64,
	reactorID core.PlayerID,
	promptKind string,
) *InputRequiredDeliveredEvent {
	return &InputRequiredDeliveredEvent{
		encID:      encID,
		seq:        seq,
		ReactorID:  reactorID,
		PromptKind: promptKind,
	}
}

func (*InputRequiredDeliveredEvent) isEncounterEvent() {}

// EncounterID returns the encounter this event belongs to.
func (e *InputRequiredDeliveredEvent) EncounterID() core.EncounterID { return e.encID }

// Sequence returns the encounter-monotonic sequence number stamped at publish time.
func (e *InputRequiredDeliveredEvent) Sequence() uint64 { return e.seq }

// Audience returns the single-element set containing only the reactor.
// Other players' streams do NOT receive this event.
func (e *InputRequiredDeliveredEvent) Audience() AudienceSet {
	return AudienceSet{e.ReactorID}
}

type inputRequiredDeliveredWire struct {
	metaWire
	EncID      core.EncounterID `json:"encounter_id"`
	Seq        uint64           `json:"sequence"`
	ReactorID  core.PlayerID    `json:"reactor_id"`
	PromptKind string           `json:"prompt_kind"`
}

// MarshalJSON exposes encID and seq under stable JSON field names.
// Implements encoding/json.Marshaler.
func (e *InputRequiredDeliveredEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(inputRequiredDeliveredWire{
		metaWire:   e.toWire(),
		EncID:      e.encID,
		Seq:        e.seq,
		ReactorID:  e.ReactorID,
		PromptKind: e.PromptKind,
	})
}

// UnmarshalJSON populates the unexported fields from JSON.
// Implements encoding/json.Unmarshaler.
func (e *InputRequiredDeliveredEvent) UnmarshalJSON(b []byte) error {
	var w inputRequiredDeliveredWire
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	e.fromWire(w.metaWire)
	e.encID = w.EncID
	e.seq = w.Seq
	e.ReactorID = w.ReactorID
	e.PromptKind = w.PromptKind
	return nil
}
