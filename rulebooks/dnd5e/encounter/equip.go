// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// equip.go is THE COMPOSITION'S HALF OF AN EQUIP (rpg-project#542, "Equip and
// unequip"): where the change happened, whose turn it was, and who saw it.
//
// # The sheet is written once, and somewhere else
//
// What a member holds lives on a sheet this module cannot read (C1). The
// session's Equip verb is the one writer: the rulebook's equip rules run
// there, the price is charged at resolution's door, and the record is saved
// there. This verb is TOLD the change after the sheet carries it, and does
// the three things only the composition can: refuse it off the member's turn
// in a fight, tell the beat to the members who see the actor, and make every
// watcher look again so their testimony of the actor's hands is re-read
// through [Equipment].
//
// # No cost, here or on the world clock
//
// In a fight the price is the rulebook's and is paid before this is called;
// out of one an equip costs nothing (R1) — not even a round on the world
// clock, which [Encounter.spendWorldAction] charges only for what the turn
// clock would price as an action. A free-roam equip is the economy-free verb
// the design rules it.

// RecordEquipInput is one equipment change a member made, as the rulebook
// reports it.
type RecordEquipInput struct {
	// Member is who changed their equipment. Must be a member (ErrNotMember)
	// and placed (ErrBadPlacement).
	Member MemberID

	// Slot is the rulebook's name for the slot that changed — "main_hand",
	// "off_hand", "armor". CARRIED, NEVER READ (C1): which slots exist and
	// what fits in them is the rulebook's.
	Slot string

	// Item is the bare item id now in the slot ([HeldEquipment]'s
	// convention), or empty when the change left it empty.
	Item string

	// Removed is the bare item id the change took out of the slot, or empty
	// when the slot was empty before. A swap names both.
	//
	// A CHANGE THAT NAMES NEITHER IS REFUSED (ErrInvalidData): nothing went
	// into the slot and nothing came out, which is not a change anybody saw.
	Removed string
}

// RecordEquipOutput reports the beat and what the re-look produced.
type RecordEquipOutput struct {
	// Seq is the sequence of the equipped beat.
	Seq uint64

	// Audience is every member told the beat: the actor and every member
	// whose sight reaches the actor's cell ([Encounter.Witnesses]). Sorted.
	Audience []MemberID

	// IntelDeltas is each observer's intel movement from the re-look, the
	// same shape [RecheckOutput] carries.
	IntelDeltas map[MemberID]*IntelDelta

	// Formed is a fight the re-look started — not expected, since nobody
	// moved, and reported rather than dropped if one ever is.
	Formed *FormedBubble
}

// RecordEquip records one equipment change: an `equipped` beat told to the
// members who see the actor, then a re-look declaring the actor changed.
//
// THE SHEET MUST ALREADY CARRY THE CHANGE. The re-look asks [Equipment] for
// the actor's hands, so a caller that records before it writes the sheet
// teaches every watcher the hands the actor had a moment ago.
//
// THE AUDIENCE IS THE ACTOR'S WITNESSES, the set every act a member performs
// where they stand is told to ([Encounter.Witnesses]): a member who cannot see
// the actor is not told they drew a sword. The beat precedes the re-look —
// a verb's own beat precedes its consequences ([Encounter.refreshSight]).
//
// IN A FIGHT, ONLY ON THE MEMBER'S OWN TURN: a member of a bubble the clock
// is not waiting on is refused with [ErrNotActive] and nothing is written —
// the turn gate [Encounter.Hold] and [Encounter.Step] keep. A member on the
// world clock passes.
//
// Validation order (R5): nil input → empty member → closed → not a member →
// empty slot or a change naming nothing → not their turn in a fight → not
// placed.
//
// Errors: ErrNilInput, ErrNoMember, ErrClosed, ErrNotMember, ErrInvalidData,
// ErrNotActive, ErrBadPlacement, or a sight-refresh failure (drop the
// encounter unsaved — doc.go's caller rule).
func (e *Encounter) RecordEquip(in *RecordEquipInput) (*RecordEquipOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("record equip: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("record equip: %w", ErrNoMember)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("record equip: %w", ErrClosed)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("record equip: member %q: %w", in.Member, ErrNotMember)
	}
	if in.Slot == "" {
		return nil, fmt.Errorf("record equip: slot: %w", ErrInvalidData)
	}
	if in.Item == "" && in.Removed == "" {
		return nil, fmt.Errorf("record equip: slot %q: a change that names no item: %w", in.Slot, ErrInvalidData)
	}
	if err := e.refuseOffTurn("record equip", in.Member); err != nil {
		return nil, err
	}

	_, witnesses, err := e.audienceOf(in.Member)
	if err != nil {
		return nil, fmt.Errorf("record equip: %w", err)
	}

	payload, err := json.Marshal(map[string]interface{}{
		"beat":    BeatEquipped,
		"member":  string(in.Member),
		"slot":    in.Slot,
		"item":    in.Item,
		"removed": in.Removed,
	})
	if err != nil {
		return nil, fmt.Errorf("record equip: marshal beat: %w", err)
	}
	appended, err := e.appendBeat(&record.AppendInput{
		At:       uint64(e.clock.ToData().HighWater),
		Audience: witnesses,
		Tags:     map[string]string{"tag": "equip"},
		Payload:  payload,
	})
	if err != nil {
		return nil, fmt.Errorf("record equip: append beat: %w", err)
	}

	deltas, formed, err := e.refreshSightDeclaring(e.rosterIDs(), []MemberID{in.Member})
	if err != nil {
		return nil, fmt.Errorf("record equip: %w", err)
	}

	return &RecordEquipOutput{
		Seq:         appended.Seq,
		Audience:    witnesses,
		IntelDeltas: deltas,
		Formed:      formed,
	}, nil
}
