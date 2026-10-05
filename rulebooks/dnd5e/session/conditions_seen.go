// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// What everyone is holding, in the sense of conditions: the third fact of a
// sighting after position and hands (rpg-project#520, ruling R16).
//
// It is answered where the sheets are, by the same seam that answers hands,
// for the same reasons that seam's godoc gives: the composition holds no sheet
// and cannot (law C1), this answer is TRUTH and is not told who is looking, and
// the composition snapshots it into each observer's own sight testimony at
// refresh and never reads it live. Every condition a member holds is reported;
// nothing is filtered as imperceptible (R16).

// COMPILE-TIME PROOF that the seam satisfies the composition's whole
// equipment contract, conditions included: an encounter refuses an Equipment
// that cannot answer them.
var _ encounter.EquipmentWithConditions = equipmentSeam{}

// Conditions reports which conditions each of the given members holds, as the
// (ref, source) address each one names itself by, in persisted sheet order.
//
// ONLY ABOUT WHO WAS ASKED, for the reason [equipmentSeam.Equipment] gives.
//
// # Nil means nothing to observe, and an empty set means seen holding none
//
//   - KindWorld — a door holds no conditions to observe. Nil.
//   - A player whose sheet is not found — the ordinary authored-content state.
//     Nil: unknown, never "holds nothing".
//   - A monster with no sheet in this session record. Nil.
//   - A player or monster whose sheet IS found always gets a non-nil set,
//     empty when the sheet holds no conditions, because that is a real
//     observation.
//
// # What a sheet does not load is not held
//
// A monster's stored list also carries its stat-block traits, which are not
// conditions; and the sheet's own projection is lenient — loading logs and
// drops a stored condition it cannot parse, and the rest of the sheet plays on
// (TestACorruptConditionIsDroppedRatherThanRejected). The loaded member holds
// neither, so this answer, which reports what the member holds, leaves both out
// the same way ([conditions.HeldAddresses]). Failing the verb instead would
// make one unreadable blob refuse every sight refresh in the encounter where
// the sheet itself would have loaded.
func (s equipmentSeam) Conditions(
	members []encounter.MemberID,
) (map[encounter.MemberID]*encounter.ConditionSet, error) {
	out := make(map[encounter.MemberID]*encounter.ConditionSet, len(members))

	for _, id := range members {
		name := string(id)
		kind, ok := s.kinds[name]
		if !ok {
			return nil, fmt.Errorf(
				"conditions member %q has no roster kind: %w", name, ErrInvalidSession)
		}

		var set *encounter.ConditionSet
		switch kind {
		case encounter.KindWorld:
			out[id] = nil
			continue
		case encounter.KindMonster:
			sheet, found := npcSheet(s.data, name)
			if !found {
				out[id] = nil
				continue
			}
			set = seenConditions(name, sheet.Conditions)
		case encounter.KindPlayer:
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
			set = seenConditions(name, data.Conditions)
		default:
			return nil, fmt.Errorf(
				"conditions member %q has unknown roster kind %q: %w", name, kind, ErrInvalidSession)
		}
		out[id] = set
	}

	return out, nil
}

// seenConditions reports what a sheet's stored conditions name themselves by,
// through the conditions package's own reader, so this seam names no condition
// type and holds none. The set is non-nil even when empty: a sheet was read.
func seenConditions(member string, raw []json.RawMessage) *encounter.ConditionSet {
	held := conditions.HeldAddresses(member, raw)
	set := &encounter.ConditionSet{Conditions: make([]encounter.SeenCondition, 0, len(held))}
	for _, address := range held {
		set.Conditions = append(set.Conditions, encounter.SeenCondition{
			Ref: address.ConditionRef, SourceID: address.SourceID,
		})
	}
	return set
}
