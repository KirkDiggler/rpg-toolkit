// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/npcs"
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
	// encounter.Interact untouched; see this type's own doc. Not read for a
	// thing done to a prop, which is measured at [InteractionReach] as a
	// line is.
	Range int

	// Prop is the prop a thing is done to (rpg-project#959): the dungeon
	// file's `place[].id`, as [InteractOptionRow.Prop] carries it. EXACTLY
	// ONE of Target and Prop; with Prop, OptionID is required.
	Prop string

	// OptionID names the thing done, verbatim from
	// [InteractOptionRow.OptionID]. It must be one the prop offers THIS
	// actor now, and one to do rather than to say ([ErrNoOption]).
	OptionID string

	// Revision is the thing's [InteractOptionRow.Revision], verbatim; a
	// thing done at any other revision is [ErrStaleOption], one pending on
	// somebody's waiting roll [ErrLineReserved]. Not read for a thing that
	// touches no conversation.
	Revision uint64
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

	// Inventory is the NPC's resolved vendor stock, in display-ready form.
	// Empty (nil) for any NPC that isn't a vendor — the normal case for
	// most world NPCs, not an error. Resolved fresh from the stored
	// content on every call via npcs.VendorInventoryFromNPCData; a caller
	// mutating this slice does not mutate session's stored record.
	Inventory []npcs.StockEntryView
}

// InteractOutput reports the descriptor reached and what interacting
// recorded.
type InteractOutput struct {
	// Descriptor is what the target IS, assembled from session's own store.
	// The zero descriptor for a thing done to a prop.
	Descriptor WorldNPCDescriptor

	// Done is the thing done to a prop (rpg-project#959): the attempt in the
	// open, [SpeakOutput]'s shape, beside the world NPC's descriptor. Nil
	// for a world NPC.
	Done *DoneOutput

	// Saved names what was persisted.
	Saved SaveReport

	// Delivery names what reached the event stream.
	Delivery DeliveryReport
}

// DoneOutput is a thing done to a prop (rpg-project#959): the verdict, the
// roll and the pause, exactly as [SpeakOutput] reports a line — the two share
// one machine.
type DoneOutput struct {
	// Prop and OptionID echo what was done, and to what.
	Prop     string `json:"prop"`
	OptionID string `json:"option_id"`

	// Beaten, Total, DC and Applied are [SpeakOutput]'s: the verdict (true
	// for a plain thing, which rolls nothing), and the check's arithmetic.
	Beaten  bool         `json:"beaten"`
	Total   int          `json:"total"`
	DC      int          `json:"dc"`
	Applied DoorApproach `json:"applied"`

	// Paused and Roll are [SpeakOutput.Paused]'s: the attempt stopped to ask
	// whether to spend a held benefit; answer it with [Manager.React].
	Paused bool `json:"paused,omitempty"`
	Roll   *int `json:"roll,omitempty"`
}

// Interact confirms a player can reach a placed world NPC and returns what
// it is — or, with a prop, does one of the prop's things to do.
//
// # A thing done to a prop (rpg-project#959)
//
// With [InteractInput.Prop] it is something done, seen and not heard: the
// session's one social machine, shared with [Manager.Speak] — the same
// doors in the same order, the same price (a checked thing is the action, a
// plain one free), the same roll, the same waiting check that holds its
// conversation, the same settle — landed through the composition's widened
// Interact. Its errors are Speak's for a prop: ErrNoOption (not offered, or
// a line to say), ErrNoProp (no such thing, or one this actor is not shown),
// ErrStaleDeclaration (a thing no longer within reach), ErrStaleOption,
// ErrLineReserved, ErrNoRoom (a far end with no room), ErrInBubble or
// ErrInCustody (a move it would make refused).
//
// # A world NPC
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
// already names for a monster), ErrBadNPC (the stored content's Inventory
// bytes are present but malformed — a record no valid npc.New/npcs.NewMerchant
// call could have produced), or ErrSaveFailed with a populated report.
func (m *Manager) Interact(ctx context.Context, in *InteractInput) (*InteractOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("interact: %w", ErrNilInput)
	}
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()
	if in.Prop != "" || in.OptionID != "" {
		return m.interactWithProp(ctx, in)
	}
	if in.Actor == "" || in.Target == "" {
		return nil, fmt.Errorf("interact: %w", ErrNoMemberID)
	}

	scope, err := m.openForChangeBy(ctx, "interact", in.Session, in.Actor)
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
	idx, err := findWorldNPCIndex(data, targetID)
	if err != nil {
		return WorldNPCDescriptor{}, err
	}
	content := data.WorldNPCs[idx].NPC
	ref := ""
	if content.Ref != nil {
		ref = content.Ref.String()
	}

	var stock []npcs.StockEntryView
	inventory, ok, err := npcs.VendorInventoryFromNPCData(&content)
	if err != nil {
		return WorldNPCDescriptor{}, fmt.Errorf("world npc %q inventory: %w: %w", targetID, ErrBadNPC, err)
	}
	if ok {
		stock = inventory.View().Entries
	}

	return WorldNPCDescriptor{
		TargetID:     targetID,
		Ref:          ref,
		DisplayName:  content.DisplayName,
		Capabilities: slices.Clone(content.Capabilities),
		CombatPolicy: content.CombatPolicy,
		Inventory:    stock,
	}, nil
}

// findWorldNPCIndex locates a placed world NPC's index in session's own
// WorldNPCs store by member ID — factored out of worldNPCDescriptor so
// trade.go can reach the same stored npc.Data to mutate it (decrementing
// stock) rather than walking WorldNPCs a second, independent way.
//
// Returns ErrNoSheet on a miss, the same sentinel worldNPCDescriptor already
// used for this defect: a confirmed KindWorld member with nothing recorded
// here is an internal inconsistency, not a caller mistake.
func findWorldNPCIndex(data *SessionData, targetID string) (int, error) {
	for i := range data.WorldNPCs {
		if data.WorldNPCs[i].MemberID == targetID {
			return i, nil
		}
	}
	return -1, fmt.Errorf("world npc %q: %w", targetID, ErrNoSheet)
}

// interactWithProp is [Manager.Interact] for a thing done to a prop
// (rpg-project#959): the session's one social machine ([Manager.social])
// under [VerbInteract], its answer in [DoneOutput]'s shape.
func (m *Manager) interactWithProp(ctx context.Context, in *InteractInput) (*InteractOutput, error) {
	if in.Target != "" || in.Prop == "" {
		return nil, fmt.Errorf("interact: a thing is done to exactly one prop, and to no creature: %w", ErrNoMemberID)
	}
	out, err := m.social(ctx, VerbInteract, &SpeakInput{
		Session: in.Session, Member: in.Actor, Prop: in.Prop, OptionID: in.OptionID, Revision: in.Revision,
	})
	if err != nil {
		return nil, thingRefused(err)
	}
	return &InteractOutput{
		Done: &DoneOutput{
			Prop: out.Prop, OptionID: out.OptionID, Beaten: out.Beaten, Total: out.Total, DC: out.DC,
			Applied: out.Applied, Paused: out.Paused, Roll: out.Roll,
		},
		Saved: out.Saved, Delivery: out.Delivery,
	}, nil
}

// ThingRefusal is a thing to do refused, with the sentence its doer is
// shown (rpg-project#959): in the second person, since only the doer's own
// verb is refused. It wraps the session's sentinel, so errors.Is still
// tells the refusals apart.
type ThingRefusal struct {
	// Sentence is what the doer is shown.
	Sentence string
	// Err is the session sentinel refused by.
	Err error
}

// Error is the sentence.
func (e *ThingRefusal) Error() string { return e.Sentence }

// Unwrap is the sentinel.
func (e *ThingRefusal) Unwrap() error { return e.Err }

// thingSentences are the refusals of a thing to do a doer may hear, each in
// the second person.
var thingSentences = []struct {
	err      error
	sentence string
}{
	{ErrStaleOption, "That has changed since you looked; look again."},
	{ErrLineReserved, "That waits on somebody's roll."},
	{ErrNoProp, "You see no such thing here."},
	{ErrNoOption, "That is not something you can do here."},
	{ErrStaleDeclaration, "You cannot reach that from here."},
	{ErrOutOfRange, "You cannot reach that from here."},
}

// thingRefused is a thing to do's refusal with its sentence, or err as it
// came when it is none of [thingSentences] (a refusal that already speaks —
// [ErrNoRoom], [ErrInCustody] — or a defect).
func thingRefused(err error) error {
	for _, t := range thingSentences {
		if errors.Is(err, t.err) {
			return &ThingRefusal{Sentence: t.sentence, Err: err}
		}
	}
	return err
}
