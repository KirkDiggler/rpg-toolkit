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

// EquipmentChange is which way one item moved through a slot. The values
// are the wire words of the beat's `change` key.
type EquipmentChange string

const (
	// EquipDraw is an item entering a slot: a weapon drawn, a shield or a
	// suit of armour donned.
	EquipDraw EquipmentChange = "draw"

	// EquipStow is an item leaving a slot, back to the character's
	// inventory: a weapon put away, a shield or a suit of armour doffed.
	EquipStow EquipmentChange = "stow"
)

// RecordEquipInput is one equip a member made through one slot, as the
// rulebook reports it: an item stowed, an item drawn, or both for a swap.
type RecordEquipInput struct {
	// Member is who changed their equipment. Must be a member (ErrNotMember)
	// and placed (ErrBadPlacement).
	Member MemberID

	// Slot is the rulebook's key for the slot the items moved through —
	// "main_hand", "off_hand", "armor". CARRIED, NEVER READ (C1): which slots
	// exist and what fits in them is the rulebook's.
	Slot string

	// Stowed is the item that left the slot, as its full ref string
	// ("dnd5e:weapons:longsword"), or empty when the slot was empty before.
	// Carried as a name to show; this module does not know what an item is.
	Stowed string

	// Drawn is the item that entered the slot, as its full ref string, or
	// empty when the equip left the slot empty.
	//
	// AN EQUIP THAT NAMES NEITHER, OR NAMES ONE ITEM BOTH WAYS, IS REFUSED
	// (ErrInvalidData): nothing changed hands, which is not a change anybody
	// saw.
	Drawn string
}

// RecordEquipOutput reports the beats and what the re-look produced.
type RecordEquipOutput struct {
	// Seqs is the sequence of every equipment-changed beat, in the order
	// told: the stow before the draw.
	Seqs []uint64

	// Correlation is the token every beat of this equip carries, so a reader
	// tells the two halves of a swap as one act.
	Correlation string

	// Audience is every member told the beats: the actor and every member
	// whose sight reaches the actor's cell ([Encounter.Witnesses]). Sorted.
	Audience []MemberID

	// IntelDeltas is each observer's intel movement from the re-look, the
	// same shape [RecheckOutput] carries.
	IntelDeltas map[MemberID]*IntelDelta

	// Formed is a fight the re-look started — not expected, since nobody
	// moved, and reported rather than dropped if one ever is.
	Formed *FormedBubble
}

// RecordEquip records one equip: an `equipment-changed` beat per item that
// moved — the stow, then the draw — told to the members who see the actor
// and sharing one correlation, then a re-look declaring the actor changed.
//
// ONE BEAT PER ITEM, ONE WAY EACH. A swap is not a third kind of change; it
// is a stow and a draw, told in the order they happen, and the correlation
// is what says they were one act. The correlation is minted HERE, from the
// sequence of the equip's first beat — the story owns its own sequence, so
// no two acts in one story can share a token, and no caller has to mint one.
//
// THE SHEET MUST ALREADY CARRY THE CHANGE. The re-look asks [Equipment] for
// the actor's hands, so a caller that records before it writes the sheet
// teaches every watcher the hands the actor had a moment ago.
//
// THE AUDIENCE IS THE ACTOR'S WITNESSES, the set every act a member performs
// where they stand is told to ([Encounter.Witnesses]): a member who cannot see
// the actor is not told they drew a sword. The beats precede the re-look —
// a verb's own beat precedes its consequences ([Encounter.refreshSight]).
//
// IN A FIGHT, ONLY ON THE MEMBER'S OWN TURN: a member of a bubble the clock
// is not waiting on is refused with [ErrNotActive] and nothing is written —
// the turn gate [Encounter.Hold] and [Encounter.Step] keep. A member on the
// world clock passes.
//
// Validation order (R5): nil input → empty member → closed → not a member →
// empty slot, an equip naming nothing, or one item both ways → not their turn in a fight → not
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
	if in.Drawn == "" && in.Stowed == "" {
		return nil, fmt.Errorf("record equip: slot %q: an equip that names no item: %w", in.Slot, ErrInvalidData)
	}
	if in.Drawn == in.Stowed {
		return nil, fmt.Errorf("record equip: slot %q: %q stowed and drawn again changes nothing: %w",
			in.Slot, in.Drawn, ErrInvalidData)
	}
	if err := e.refuseOffTurn("record equip", in.Member); err != nil {
		return nil, err
	}

	_, witnesses, err := e.audienceOf(in.Member)
	if err != nil {
		return nil, fmt.Errorf("record equip: %w", err)
	}

	type moved struct {
		item   string
		change EquipmentChange
	}
	var changes []moved
	if in.Stowed != "" {
		changes = append(changes, moved{item: in.Stowed, change: EquipStow})
	}
	if in.Drawn != "" {
		changes = append(changes, moved{item: in.Drawn, change: EquipDraw})
	}

	first, err := e.story.NextSeq()
	if err != nil {
		return nil, fmt.Errorf("record equip: %w", err)
	}
	correlation := fmt.Sprintf("equip:%d", first)
	at := uint64(e.clock.ToData().HighWater)

	seqs := make([]uint64, 0, len(changes))
	for _, c := range changes {
		payload, err := json.Marshal(map[string]interface{}{
			"beat":   BeatEquipmentChanged,
			"member": string(in.Member),
			"slot":   in.Slot,
			"item":   c.item,
			"change": string(c.change),
		})
		if err != nil {
			return nil, fmt.Errorf("record equip: marshal beat: %w", err)
		}
		appended, err := e.appendBeat(&record.AppendInput{
			At:          at,
			Correlation: correlation,
			Audience:    witnesses,
			Tags:        map[string]string{"tag": "equip"},
			Payload:     payload,
		})
		if err != nil {
			return nil, fmt.Errorf("record equip: append beat: %w", err)
		}
		seqs = append(seqs, appended.Seq)
	}

	deltas, formed, err := e.refreshSightDeclaring(e.rosterIDs(), []MemberID{in.Member})
	if err != nil {
		return nil, fmt.Errorf("record equip: %w", err)
	}

	return &RecordEquipOutput{
		Seqs:        seqs,
		Correlation: correlation,
		Audience:    witnesses,
		IntelDeltas: deltas,
		Formed:      formed,
	}, nil
}
