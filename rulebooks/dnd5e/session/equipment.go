// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// What everyone is holding, answered where the sheets are.
//
// The composition holds no inventory and cannot ever hold any (law C1), so it
// takes a capability instead — member IDs in, two hands out per member — and
// the session is the only layer that both knows what a sheet is and holds every
// one of them for the call in progress. That is the same wire [standingSeam] is
// for hit points, built the same way and for the same reason.
//
// # This answers TRUTH, and the lie happens later
//
// What comes back is what each member IS holding. Nothing here is told who is
// looking, and nothing here dissembles. The composition snapshots this answer
// into each observer's own sight testimony when they see somebody, and from
// that moment it is that observer's claim — free to disagree with the world,
// with the sheet, and with every other observer. A charmed player who believes
// the ogre holds a toy is a false payload written into THAT player's holding by
// whatever charmed them.
//
// The alternative — letting a client read the sheet when it wants to draw a
// weapon — was considered and refused (Kirk, rpg-toolkit#1615): a live read can
// only ever return the truth, so it forecloses illusion, disguise and
// enchantment permanently rather than deferring them.

// equipmentSeam answers the composition's equipment question out of the sheets
// this one verb holds.
//
// COMPILE-TIME PROOF that this package satisfies the composition's contract.
var _ encounter.Equipment = equipmentSeam{}

// # It is per call, and it has to be
//
// The context rides on the struct for exactly the reason [standingSeam]'s does:
// the capability's method takes no context because the composition calling it
// has none to give, so the verb's own is captured when the seam is built, and
// the seam is built fresh for every verb (S1, S4).
//
// # No cache, deliberately
//
// Every consult re-reads. The composition asks again rather than remembering
// (its [encounter.Equipment] doc says why), and this side must not quietly undo
// that by remembering on its behalf: a verb that equips a shield writes the
// sheet part-way through itself, and an answer cached at the top of the call
// would be describing a world that no longer exists.
type equipmentSeam struct {
	// ctx is the verb's own. See the type's godoc.
	ctx context.Context

	// chars is the host's sheet store, for the members it owns.
	chars CharacterRepository

	// kinds is the authoritative encounter roster classification copied at the
	// load/setup boundary, so nothing guesses kind from whichever storage
	// record happens to answer.
	kinds map[string]encounter.MemberKind
}

// Equipment reports what each of the given members is holding.
//
// ONLY ABOUT WHO WAS ASKED, structurally: the loop is over the question, for
// the reason [standingSeam.Standing] gives — the composition refuses an answer
// naming a stranger (ErrNotMember) rather than ignoring it, and the sheet store
// really does hold strangers.
//
// # Three ways to have no hands, and all of them are nil
//
// Nil is a fact, not a gap, and the composition accepts it (it refuses only a
// member left OUT of the answer):
//
//   - KindWorld — a door has no hands. Never consulted.
//   - KindMonster — monster sheets carry actions, not equipment slots. A
//     skeleton observed empty-handed would be testimony nobody is entitled to.
//   - A player whose sheet is not found — the ordinary authored-content state
//     [standingSeam.recordsFor] describes, not a defect.
//
// A player whose sheet IS found always gets a value, even when both hands are
// empty, because that is a real observation: hands were looked at, and there
// was nothing in them. The rulebook turns an empty main hand into an unarmed
// strike, so empty is a thing rather than the absence of one.
//
// An error aborts whatever verb was running, atomically (R5), the same as
// [standingSeam.Standing]'s: a world that cannot find out what somebody is
// holding does not half-build a percept on a guess.
func (s equipmentSeam) Equipment(
	members []encounter.MemberID,
) (map[encounter.MemberID]*encounter.HeldEquipment, error) {
	out := make(map[encounter.MemberID]*encounter.HeldEquipment, len(members))

	for _, id := range members {
		name := string(id)
		kind, ok := s.kinds[name]
		if !ok {
			return nil, fmt.Errorf(
				"equipment member %q has no roster kind: %w", name, ErrInvalidSession)
		}

		switch kind {
		case encounter.KindWorld, encounter.KindMonster:
			out[id] = nil
			continue
		case encounter.KindPlayer:
		default:
			return nil, fmt.Errorf(
				"equipment member %q has unknown roster kind %q: %w", name, kind, ErrInvalidSession)
		}

		data, fetchErr := s.chars.GetCharacter(s.ctx, name)
		if fetchErr != nil {
			if errors.Is(fetchErr, ErrNotFound) {
				out[id] = nil
				continue
			}
			return nil, fetchErr
		}
		if data == nil {
			return nil, fmt.Errorf(
				"character %q: GetCharacter reported success with no data: %w", name, ErrBadRepository)
		}

		out[id] = &encounter.HeldEquipment{
			MainHand: data.EquipmentSlots[character.SlotMainHand],
			OffHand:  data.EquipmentSlots[character.SlotOffHand],
		}
	}

	return out, nil
}

// equipmentBeside builds the equipment capability from the verb-scoped facts a
// [standingSeam] already carries.
//
// FROM THE SAME SEAM, NOT REBUILT, and that is correctness rather than thrift.
// The roster-kind snapshot is replaced deliberately at some load boundaries so
// that every assessment made during one load sees exactly one classification
// (write.go's own comment says why). Rebuilding kinds here would open a second
// snapshot that could disagree with it, and a member classified two ways inside
// one verb is the shape of bug that whole precaution exists to prevent.
//
// The two capabilities stay separate contracts answering separate questions —
// who is down, and what is in their hands. They only share where the sheets are
// and which verb is asking.
func equipmentBeside(s standingSeam) equipmentSeam {
	return equipmentSeam{ctx: s.ctx, chars: s.chars, kinds: s.kinds}
}
