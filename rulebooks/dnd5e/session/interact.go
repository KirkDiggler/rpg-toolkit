// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// interact.go is the host seam's half of interacting with a placed world NPC
// (rpg-toolkit#1404, design.md's "Session Seam"). encounter.Interact answers
// only identity, adjacency, and visibility — it holds no NPC content and
// builds no descriptor. This file is where an NPC's placed content actually
// becomes a reader-facing answer: it confirms the reach through encounter,
// then looks the confirmed target up in SessionData.WorldNPCs (PlaceNPC's
// store) to build the WorldNPCDescriptor a caller sees.

// InteractInput names who is reaching for whom, and how far that reach
// extends. Range is forwarded to encounter.Interact untouched — the
// negative-range rejection already lives there (encounter/interact.go), and
// duplicating it here would be a second place that rule could drift from the
// first.
type InteractInput struct {
	// Session is the session to act in.
	Session string

	// Actor is the player member initiating the interaction.
	Actor string

	// Target is the KindWorld member being interacted with.
	Target string

	// Range is the maximum distance, in cells, Target may stand from Actor.
	// Zero (the default) means adjacent — one cell. Forwarded to
	// encounter.Interact untouched; see this type's own doc.
	Range int
}

// WorldNPCDescriptor is what a player sees after reaching a world NPC: the
// content session's own store holds for the confirmed target, not anything
// encounter reports (encounter carries none — design.md's 2026-09-02
// amendment).
//
// MemberID and Ref are both projected as plain strings (S2 —
// TestNoInnerTypeCrossesTheBoundary), the same convention turndriver.go
// already applies to a core.Ref for the same reason: an inner package's
// type must never cross this seam's exported surface.
type WorldNPCDescriptor struct {
	// TargetID echoes the confirmed member ID, so a caller need not trust
	// its own input back.
	TargetID string

	// Ref is the placed NPC content's reference, as core.Ref.String() —
	// never the *core.Ref itself. Empty if the stored content carried no
	// ref, which npc.Data's own validation should not normally allow, but
	// this seam does not assume that guarantee holds forever.
	Ref string

	// DisplayName is the NPC's player-facing name.
	DisplayName string

	// Capabilities are the NPC's opaque interaction-capability labels.
	// Copy-out: mutating this slice does not mutate session's stored
	// record.
	Capabilities []npc.Capability

	// CombatPolicy is the NPC's authored combat participation policy — the
	// MVP has exactly one legal value, non-combatant.
	CombatPolicy npc.CombatPolicy
}

// InteractOutput reports the descriptor reached and what interacting
// recorded.
type InteractOutput struct {
	// Descriptor is what the target IS, assembled from session's own store.
	Descriptor WorldNPCDescriptor

	// Seq is the sequence number of the recorded interaction beat.
	Seq uint64

	// Saved names what was persisted.
	Saved SaveReport

	// Delivery names what reached the event stream.
	Delivery DeliveryReport
}

// Interact confirms a player can reach a placed world NPC and returns what
// it is.
//
// Two layers, two questions: encounter.Interact answers "can Actor reach
// Target" (identity, adjacency, visibility) and holds no NPC content to
// answer anything else; this verb answers "what IS Target" by looking the
// confirmed member up in SessionData.WorldNPCs, exactly parallel to how
// Attack reads a monster's sheet out of SessionData.NPCs by member ID
// rather than asking encounter what kind of monster it placed.
//
// Returns ErrNilInput, ErrNoMemberID (empty actor or target), ErrNoSessionID,
// ErrNoSession, ErrNoEncounter, ErrClosed, ErrNoMember (actor or target
// missing from the roster, or present but the wrong kind), ErrBadPosition
// (either member has no cell — encounter.ErrBadPlacement, translated; a
// roster member with no canvas placement, an internal inconsistency rather
// than an ordinary caller mistake), ErrOutOfRange, ErrNotVisible, ErrNoSheet
// (an internal inconsistency: encounter confirmed a KindWorld target but
// session's own WorldNPCs store has nothing recorded for it — content the
// session itself never recorded, the same shape of defect ErrNoSheet
// already names for a monster), or ErrSaveFailed with a populated report.
func (m *Manager) Interact(ctx context.Context, in *InteractInput) (*InteractOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("interact: %w", ErrNilInput)
	}
	if in.Actor == "" || in.Target == "" {
		return nil, fmt.Errorf("interact: %w", ErrNoMemberID)
	}

	scope, err := m.openForWrite(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("interact: %w", err)
	}

	confirmed, err := scope.enc.Interact(&encounter.InteractInput{
		Actor:  encounter.MemberID(in.Actor),
		Target: encounter.MemberID(in.Target),
		Range:  in.Range,
	})
	if err != nil {
		return nil, fmt.Errorf("interact: %w", translate(err))
	}

	descriptor, err := worldNPCDescriptor(scope.data, string(confirmed.Target))
	if err != nil {
		return nil, fmt.Errorf("interact: %w", err)
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("interact: %w", err)
	}

	return &InteractOutput{
		Descriptor: descriptor,
		Seq:        confirmed.Seq,
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// worldNPCDescriptor builds a WorldNPCDescriptor from session's own
// WorldNPCs store — the one place a placed NPC's content lives.
//
// A confirmed target missing from this store is an internal inconsistency,
// not a caller mistake: encounter.Interact already proved targetID names a
// live KindWorld member, so a record's absence here means PlaceNPC's two
// writes (the store append and the encounter.Join) disagreed at some point.
// Failed closed with ErrNoSheet — content the session itself never
// recorded, the same defect that sentinel already names for a monster —
// rather than returning a zero-value descriptor a caller could mistake for
// a real, empty NPC.
func worldNPCDescriptor(data *SessionData, targetID string) (WorldNPCDescriptor, error) {
	for i := range data.WorldNPCs {
		if data.WorldNPCs[i].MemberID != targetID {
			continue
		}
		content := data.WorldNPCs[i].NPC
		ref := ""
		if content.Ref != nil {
			ref = content.Ref.String()
		}
		return WorldNPCDescriptor{
			TargetID:     targetID,
			Ref:          ref,
			DisplayName:  content.DisplayName,
			Capabilities: slices.Clone(content.Capabilities),
			CombatPolicy: content.CombatPolicy,
		}, nil
	}
	return WorldNPCDescriptor{}, fmt.Errorf("world npc %q: %w", targetID, ErrNoSheet)
}
