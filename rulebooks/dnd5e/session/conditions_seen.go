// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

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
// # A trait is not a condition, and an unreadable condition is unknown
//
// A monster's stored list also carries its stat-block traits, which are not
// conditions, so they are left out ([conditions.HeldAddresses]). A stored
// condition that cannot be read makes the member's holdings unknown (nil):
// the sheet's own projection is lenient — it drops the blob and plays on
// (TestACorruptConditionIsDroppedRatherThanRejected) — so failing the verb
// would refuse every sight refresh where the sheet itself loads, and listing
// the rest would claim the unreadable one is known to be absent.
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
//
// A sheet holding a condition that cannot be read is reported as nil —
// nothing observed — rather than failing the verb or listing the rest: the
// sheet still plays on under its lenient projection, but a list that left the
// unreadable one out would claim, known, that the member does not hold it,
// and unknown is never read as false.
func seenConditions(member string, raw []json.RawMessage) *encounter.ConditionSet {
	held, err := conditions.HeldAddresses(member, raw)
	if err != nil {
		return nil
	}
	set := &encounter.ConditionSet{Conditions: make([]encounter.ConditionKey, 0, len(held))}
	for _, address := range held {
		set.Conditions = append(set.Conditions, encounter.ConditionKey{
			ConditionRef: address.ConditionRef, SourceID: address.SourceID,
		})
	}
	return set
}

// conditionKey is one member's holdings as a comparable string: its sorted
// addresses, or a marker no address can spell when nothing was observed.
func conditionKey(set *encounter.ConditionSet) string {
	if set == nil {
		return "\x00unobserved"
	}
	parts := make([]string, 0, len(set.Conditions))
	for _, held := range set.Conditions {
		parts = append(parts, held.ConditionRef+"\x1f"+held.SourceID)
	}
	sort.Strings(parts)
	return strings.Join(parts, "\x1e")
}

// recheckChangedConditions is condition freshness (rpg-project#520, R19): a
// CHANGE to a member's conditions refreshes the sightings of that member at
// commit, the same way an equipment change does through [Manager.Recheck],
// because a row must not outlive the condition it describes.
//
// # It is an event, not a standing diff
//
// Only members this verb touched can have changed: a player whose sheet the
// verb wrote ([writeScope.sheetsWritten]), or a monster whose sheet the verb
// replaced with different conditions than it held before
// ([writeScope.npcConditionsBefore]). Both are recorded where the write
// happens; nothing is keyed at the verb's open, and a monster this verb never
// wrote is never fingerprinted. A member the verb never touched is never
// compared, so a sighting that is legitimately stale — a watcher who cannot
// see the member now, or a change no verb declared — draws no re-look, no
// beat and no sheet read on any later commit. For each touched member, what
// it holds now is compared with every observer's current sight of it
// ([encounter.Encounter.ObservedContext], testimony); the members some
// sighting describes differently are re-looked, in sorted order. A sight
// refresh earlier in the verb already wrote the truth into what it refreshed,
// so a touched member already seen afresh draws nothing.
//
// The sheets read here are only those the verb itself wrote, after it acted;
// no sheet is read before the verb has acted, which the verbs' load-order laws
// forbid. A sighting that observed no conditions (testimony from before
// sightings carried them) is unknown rather than stale. A finished encounter
// has no one left to look.
//
// Accepted cost: every condition change a watcher can see sends that watcher a
// sighting "changed" beat naming the member — never the condition.
func (m *Manager) recheckChangedConditions(scope *writeScope) error {
	touched := make(map[encounter.MemberID]bool, len(scope.sheetsWritten))
	for id := range scope.sheetsWritten {
		touched[id] = true
	}
	for id, before := range scope.npcConditionsBefore {
		if sheet, found := npcSheet(scope.data, string(id)); found &&
			conditionKey(seenConditions(sheet.ID, sheet.Conditions)) != before {
			touched[id] = true
		}
	}
	if len(touched) == 0 {
		return nil
	}

	roster, err := scope.enc.Members()
	if err != nil {
		return fmt.Errorf("condition freshness: %w", translate(err))
	}
	asked := make([]encounter.MemberID, 0, len(touched))
	for _, member := range roster {
		if touched[member.ID] {
			asked = append(asked, member.ID)
		}
	}
	if len(asked) == 0 {
		return nil
	}
	truth, err := equipmentBeside(scope.standing).Conditions(asked)
	if err != nil {
		return err
	}

	stale := make(map[encounter.MemberID]bool)
	for _, observer := range roster {
		if observer.Kind == encounter.KindWorld {
			continue
		}
		observed, err := scope.enc.ObservedContext(&encounter.ViewInput{Member: observer.ID})
		if err != nil {
			return fmt.Errorf("condition freshness for %q: %w", observer.ID, translate(err))
		}
		for _, seen := range observed.Members {
			now, isTouched := truth[seen.ID]
			if !isTouched || seen.Conditions == nil || stale[seen.ID] {
				continue
			}
			if conditionKey(seen.Conditions) != conditionKey(now) {
				stale[seen.ID] = true
			}
		}
	}
	if len(stale) == 0 {
		return nil
	}
	changed := make([]encounter.MemberID, 0, len(stale))
	for id := range stale {
		changed = append(changed, id)
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i] < changed[j] })
	if _, err := scope.enc.Recheck(&encounter.RecheckInput{Members: changed}); err != nil {
		if errors.Is(err, encounter.ErrClosed) {
			return nil
		}
		return fmt.Errorf("condition freshness: %w", translate(err))
	}
	return nil
}
