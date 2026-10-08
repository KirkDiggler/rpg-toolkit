// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// Equip and Unequip (rpg-project#542, "Equip and unequip").
//
// The one equip path for every host surface. The rulebook's own equip rules
// (occupancy, two-handed weapons, a swap on an occupied slot) run inside
// resolution's equip entry unchanged; this file decides only which path the
// change takes, and the seat decides that:
//
//   - UNSEATED: a plain sheet verb under the character's own guard. No cost,
//     no story, no session touched.
//   - SEATED, FREE ROAM: under the session's guard, no cost (the economy is a
//     fight's). The record is saved, then each move is told as a beat and
//     sight is rechecked inside the encounter's verb, so watchers learn the
//     new appearance without a second call (R8: every seated equip tells a
//     beat, a draw into an empty hand included).
//   - SEATED, IN A FIGHT: a turn action. Off the member's turn it refuses
//     with ErrNotYourTurn; a downed member refuses with ErrDowned; the price
//     is the rulebook's, charged at resolution's door against the member's
//     readied turn, and an unpayable price refuses with ErrCannotAfford.
//     Body armour refuses with ErrArmorInFight (R6). Nothing is written on
//     any refusal.
//
// The session compiles no price and pays nothing itself. The attack
// projection needs no refresh step: the encounter re-asks equipment and sheet
// facts at every consult, so the next consult reads the new hands.

// EquipInput puts an item from a character's inventory into a slot.
type EquipInput struct {
	// Character is the character id. Equip takes no session: the character's
	// seat says which run, if any, holds it.
	Character string

	// Slot is the equipment slot, in the rulebook's words ("main_hand",
	// "off_hand", "armor", ...).
	Slot string

	// Item is the inventory item id to put there.
	Item string
}

// UnequipInput empties one of a character's equipment slots.
type UnequipInput struct {
	// Character is the character id. Unequip takes no session: the
	// character's seat says which run, if any, holds it.
	Character string

	// Slot is the equipment slot to empty, in the rulebook's words.
	Slot string
}

// EquipmentMove is one item that moved: the slot it left or entered and the
// item's full ref.
type EquipmentMove struct {
	// Slot is the slot the item left (a stow) or entered (a draw).
	Slot string

	// Item is the item's full ref, the same ref the beat names.
	Item string
}

// EquipOutput reports an equipment change. Equip and Unequip answer the same
// shape: what moved, the record that was saved, and — for a seated
// character — the beats told and what they revealed.
type EquipOutput struct {
	// Session is the run that holds the character, or empty for an unseated
	// character, whose change touched no run.
	Session string

	// Character is the record the verb saved. A host projects what it shows
	// (armour class, hands) from this record and from nothing it kept.
	Character *character.Data

	// Stowed is every item put away, in the order the beats tell them.
	Stowed []EquipmentMove

	// Drawn is every item taken into hand or donned, after every stow.
	Drawn []EquipmentMove

	// Seqs are the equip beats' sequences in the character's own delivered
	// numbering (stream.go), stows first. Empty for an unseated character.
	Seqs []uint64

	// Discovered is what changed in each observer's perception when sight
	// was rechecked. Absent observers saw nothing new.
	Discovered map[string]Discovery

	// Formed is present if the new appearance put the character in sight of
	// the other side and a fight started around them.
	Formed *Formed

	// Saved names what was persisted.
	Saved SaveReport

	// Delivery names what reached the event stream. Empty for an unseated
	// character.
	Delivery DeliveryReport
}

// UnequipOutput reports an emptied slot; it is the same report as Equip's.
type UnequipOutput = EquipOutput

// Equip puts an inventory item into an equipment slot along the path the
// character's seat decides (see the top of this file).
//
// Returns ErrNilInput, ErrNoMemberID, ErrNoCharacter, ErrBadCharacter,
// ErrBadEquip for a change the sheet cannot make (no such item, the wrong
// slot), ErrNoMember for a seat naming a run that does not hold the
// character, ErrNotYourTurn, ErrDowned, ErrCannotAfford, ErrArmorInFight,
// ErrClosed, a WindowOpenError while an interrupt window is open, or
// ErrSaveFailed with a populated report.
func (m *Manager) Equip(ctx context.Context, in *EquipInput) (*EquipOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("equip: %w", ErrNilInput)
	}
	out, err := m.changeEquipment(ctx, in.Character, character.InventorySlot(in.Slot), in.Item)
	if err != nil {
		return nil, fmt.Errorf("equip: %w", err)
	}
	return out, nil
}

// Unequip empties an equipment slot along the path the character's seat
// decides (see the top of this file). Putting a held item away in a fight
// costs the action.
//
// Returns what Equip returns.
func (m *Manager) Unequip(ctx context.Context, in *UnequipInput) (*UnequipOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("unequip: %w", ErrNilInput)
	}
	out, err := m.changeEquipment(ctx, in.Character, character.InventorySlot(in.Slot), "")
	if err != nil {
		return nil, fmt.Errorf("unequip: %w", err)
	}
	return out, nil
}

// changeEquipment is the one body behind Equip and Unequip: take the guard the
// seat decides, then run the change along that seat's path. item empty
// empties the slot.
func (m *Manager) changeEquipment(
	ctx context.Context, id string, slot character.InventorySlot, item string,
) (*EquipOutput, error) {
	if id == "" {
		return nil, ErrNoMemberID
	}
	guard, err := m.guardBySeat(ctx, id)
	if err != nil {
		return nil, err
	}
	defer guard.release()

	if guard.session == "" {
		return m.equipUnseated(ctx, id, slot, item)
	}
	return m.equipSeated(ctx, guard.session, id, slot, item)
}

// equipUnseated is the plain sheet verb: the rulebook's change applied
// through resolution's door with no turn, so nothing is charged, then the
// record saved. No beat is told and no session is touched.
func (m *Manager) equipUnseated(
	ctx context.Context, id string, slot character.InventorySlot, item string,
) (*EquipOutput, error) {
	report := &writeScope{}
	sheets := m.sheetsFor(report)
	record, err := sheets.load(ctx, "character", id)
	if err != nil {
		return nil, err
	}
	changed, err := resolution.Equip(ctx, &resolution.EquipInput{Character: record, Slot: slot, ItemID: item})
	if err != nil {
		return nil, translateResolution(err)
	}
	if err := sheets.save(ctx, changed.Character); err != nil {
		return nil, err
	}
	return &EquipOutput{
		Character: changed.Character,
		Stowed:    equipmentMoves(changed.Stowed),
		Drawn:     equipmentMoves(changed.Drawn),
		Saved:     SaveReport{Written: report.written},
	}, nil
}

// equipSeated is the change inside the character's run, under the session's
// guard the caller holds: refused off turn or downed in a fight before
// anything is loaded for writing, priced on the member's readied turn in a
// fight, free in free roam; the record saved first, then one beat per move,
// stows before draws, then the commit.
func (m *Manager) equipSeated(
	ctx context.Context, session, id string, slot character.InventorySlot, item string,
) (*EquipOutput, error) {
	scope, err := m.openForChange(ctx, session)
	if err != nil {
		return nil, err
	}
	if kind, ok := scope.standing.kinds[id]; !ok || kind != encounter.KindPlayer {
		// The seat names this run and the run does not hold the character
		// as a player: the two records disagree, and acting on either would
		// be a guess.
		return nil, fmt.Errorf("character %q is seated in session %q and is not a player in it: %w",
			id, session, ErrNoMember)
	}

	sheets := m.sheetsFor(scope)
	record, err := sheets.load(ctx, "character", id)
	if err != nil {
		return nil, err
	}
	fight, record, err := m.equipTurn(ctx, scope, id, record)
	if err != nil {
		return nil, err
	}

	changed, err := resolution.Equip(ctx, &resolution.EquipInput{
		Character: record, Slot: slot, ItemID: item, Fight: fight,
	})
	if err != nil {
		return nil, translateResolution(err)
	}

	// THE RECORD FIRST, THEN THE STORY: the beats and the sight recheck read
	// the member's hands through the equipment seam, which reads this saved
	// record.
	if err := sheets.save(ctx, changed.Character); err != nil {
		return nil, err
	}

	stowed, drawn := equipmentMoves(changed.Stowed), equipmentMoves(changed.Drawn)
	var (
		seqs   []uint64
		deltas = map[encounter.MemberID]*encounter.IntelDelta{}
		formed *encounter.FormedBubble
	)
	for _, move := range equipBeats(stowed, drawn) {
		told, err := scope.enc.RecordEquip(&encounter.RecordEquipInput{
			Member: encounter.MemberID(id), Slot: move.slot, Stowed: move.stowed, Drawn: move.drawn,
		})
		if err != nil {
			return nil, reportUnrecorded(scope, translate(err))
		}
		seqs = append(seqs, told.Seqs...)
		mergeIntelDeltas(deltas, told.IntelDeltas)
		if told.Formed != nil {
			formed = told.Formed
		}
	}

	report, delivery, err := m.commit(ctx, scope)
	if err != nil {
		return nil, err
	}

	delivered := make([]uint64, 0, len(seqs))
	for _, seq := range seqs {
		delivered = append(delivered, scope.deliveredSeq(id, seq))
	}
	return &EquipOutput{
		Session:    session,
		Character:  changed.Character,
		Stowed:     stowed,
		Drawn:      drawn,
		Seqs:       delivered,
		Discovered: projectDiscoveries(deltas),
		Formed:     projectFormedFor(scope, id, formed),
		Saved:      report,
		Delivery:   delivery,
	}, nil
}

// equipTurn is the turn an in-fight change is priced against, and the record
// to price it on; nil and the record unchanged in free roam. In a fight it
// refuses, before anything is written, a member whose turn it is not
// (ErrNotYourTurn) and a downed member (ErrDowned). The turn number is the
// bubble's round, read rather than invented, as for a swing (economy.go).
//
// THE RECORD IS READIED FOR THE TURN HERE, as [Manager.priceSwing] readies
// its swing's: a sheet whose economy was never lit (a world authored straight
// into a fight, a stale stored sheet) has no readied turn for the door's
// refresh to find, and the rulebook refuses to price against an absent
// economy. [readyForTurn] is idempotent on a readied sheet, so the door's own
// refresh then finds nothing left to do. Nothing is written by readying: the
// readied record is only saved if the change lands.
func (m *Manager) equipTurn(
	ctx context.Context, scope *writeScope, id string, record *character.Data,
) (*resolution.Turn, *character.Data, error) {
	clock, err := scope.enc.ClockOf(&encounter.ClockOfInput{Member: encounter.MemberID(id)})
	if err != nil {
		return nil, nil, translate(err)
	}
	if ClockKind(clock.Kind) != ClockTurn {
		return nil, record, nil
	}
	if string(clock.Active) != id {
		return nil, nil, fmt.Errorf("character %q: %w", id, ErrNotYourTurn)
	}
	sheet, err := character.Load(ctx, record)
	if err != nil {
		return nil, nil, fmt.Errorf("character %q: %w: %v", id, ErrBadCharacter, err)
	}
	if combat.IsDown(sheet) {
		return nil, nil, fmt.Errorf("character %q: %w", id, ErrDowned)
	}
	if err := readyForTurn(ctx, sheet, clock.Round); err != nil {
		return nil, nil, fmt.Errorf("character %q: %w: %v", id, ErrBadCost, err)
	}
	readied, err := sheet.ToData()
	if err != nil {
		return nil, nil, fmt.Errorf("character %q: %w: %v", id, ErrBadCharacter, err)
	}
	return &resolution.Turn{Number: clock.Round, Speed: sheet.GetSpeed()}, readied, nil
}

// equipBeat is one RecordEquip call: one item moving in one slot.
type equipBeat struct {
	slot   string
	stowed string
	drawn  string
}

// equipBeats orders the beats an equipment change tells: every stow, then
// every draw, one item per beat (one RecordEquip call each, so each item's
// slot is its own).
func equipBeats(stowed, drawn []EquipmentMove) []equipBeat {
	beats := make([]equipBeat, 0, len(stowed)+len(drawn))
	for _, move := range stowed {
		beats = append(beats, equipBeat{slot: move.Slot, stowed: move.Item})
	}
	for _, move := range drawn {
		beats = append(beats, equipBeat{slot: move.Slot, drawn: move.Item})
	}
	return beats
}

// equipmentMoves projects resolution's moves onto this package's.
func equipmentMoves(moves []resolution.EquipMove) []EquipmentMove {
	if len(moves) == 0 {
		return nil
	}
	out := make([]EquipmentMove, 0, len(moves))
	for _, move := range moves {
		out = append(out, EquipmentMove{Slot: string(move.Slot), Item: move.Ref})
	}
	return out
}

// mergeIntelDeltas folds one recheck's deltas into the verb's running set,
// observer by observer; a later delta for the same observer appends what it
// added.
func mergeIntelDeltas(into, from map[encounter.MemberID]*encounter.IntelDelta) {
	for observer, delta := range from {
		if delta == nil {
			continue
		}
		existing, ok := into[observer]
		if !ok || existing == nil {
			copied := *delta
			into[observer] = &copied
			continue
		}
		existing.KnowledgeChanged = existing.KnowledgeChanged || delta.KnowledgeChanged
		existing.FirstContact = append(existing.FirstContact, delta.FirstContact...)
		existing.Refreshed = append(existing.Refreshed, delta.Refreshed...)
		existing.Faded = append(existing.Faded, delta.Faded...)
		existing.Changed = append(existing.Changed, delta.Changed...)
		existing.Reacquired = append(existing.Reacquired, delta.Reacquired...)
	}
}
