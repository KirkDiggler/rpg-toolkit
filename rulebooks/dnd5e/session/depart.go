// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// Leaving a run takes what was held on you with you (rpg-project#542, gate
// ruling on #1983): a player who Exits — or every player, when End closes
// the run — is put through resolution's Depart. Every effect another member's
// concentration held on the leaver comes off it, and that caster's hold drops
// the leaver and carries on for its other targets; a hold the leaver was
// concentrating on itself ends, its effects coming off every remaining
// target. The rules are resolution's; this file loads every sheet the run
// holds, hands them over, saves what came back, and tells every removal on
// the exit (or ended) beat.
//
// Because no member ever leaves a run still holding, or still held by,
// another's concentration, a rest can never meet a departed target: a hold
// naming a member the run does not hold is an invariant the run broke
// (ErrInvalidSession), not a bad sheet.

// departure is what a run's leavers took with them: every removal, by the
// member it came off, and the concentrations their leaving ended.
type departure struct {
	removedFrom map[string][]encounter.ActivationResult
	breaks      []encounter.ConcentrationBreak
}

// all is every removal the departure made, in leaver order then rulebook
// order, wherever it landed.
func (d *departure) all(order []string) []encounter.ActivationResult {
	var out []encounter.ActivationResult
	for _, member := range order {
		out = append(out, d.removedFrom[member]...)
	}
	return out
}

// depart resolves the departure of each leaver, in id order, over every
// other sheet the run holds as it stands after the earlier leavers left, and
// saves every changed sheet once all have resolved. A leaver who is not a
// player held nothing a character sheet can answer for and is passed over.
//
// A player with no sheet — the leaver or any other — is passed over, so a
// broken run can still be left and closed. Returns ErrInvalidSession for a
// hold whose caster the run does not hold, ErrBadCharacter for a sheet
// resolution cannot attach, or the store's SaveError.
func (m *Manager) depart(ctx context.Context, scope *writeScope, leavers []string) (*departure, error) {
	ordered := append([]string(nil), leavers...)
	sort.Strings(ordered)

	sheets := m.sheetsFor(scope)
	pending := &pendingSheets{}
	out := &departure{removedFrom: map[string][]encounter.ActivationResult{}}
	for _, leaver := range ordered {
		if scope.standing.kinds[leaver] != encounter.KindPlayer {
			continue
		}
		record, err := pending.load(ctx, sheets, "leaver", leaver)
		if errors.Is(err, ErrNoCharacter) {
			// A sheet nobody can read holds nothing anybody can take off it,
			// and Exit and End must still work around it.
			continue
		}
		if err != nil {
			return nil, err
		}
		others, err := m.othersOf(ctx, scope, sheets, pending, leaver, true)
		if err != nil {
			return nil, err
		}
		left, err := resolution.Depart(ctx, &resolution.DepartInput{Character: record, Others: others})
		if err != nil {
			return nil, translateDepart(leaver, err)
		}
		if left == nil || left.Character == nil {
			return nil, fmt.Errorf("leaver %q: depart returned no character data: %w", leaver, ErrBadCharacter)
		}

		if leaverChanged(leaver, left) {
			pending.hold(left.Character)
		}
		for _, dirty := range left.DirtyCharacters {
			if dirty != nil {
				pending.hold(dirty)
			}
		}
		for _, dirty := range left.DirtyMonsters {
			if dirty != nil {
				scope.replaceMonsterSheet(dirty)
			}
		}

		for _, removed := range left.Ended {
			out.add(removed)
		}
		for _, broken := range left.ConcentrationBreaks {
			for _, removed := range broken.Removed {
				out.add(removed)
			}
		}
		out.breaks = append(out.breaks, left.ConcentrationBreaks...)
	}

	for _, record := range pending.ordered() {
		if err := sheets.save(ctx, record); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// leaverChanged reports whether a departure changed the leaver's own sheet:
// something came off them, or a hold they were concentrating on ended. A
// leaver nothing held and holding nothing is not written.
func leaverChanged(leaver string, left *resolution.DepartOutput) bool {
	for _, removed := range left.Ended {
		if removed.Address != nil && string(removed.Address.MemberID) == leaver {
			return true
		}
	}
	for _, broken := range left.ConcentrationBreaks {
		if string(broken.Caster) == leaver {
			return true
		}
	}
	return false
}

// add files one removal under the member it came off.
func (d *departure) add(removed encounter.ActivationResult) {
	member := ""
	if removed.Address != nil {
		member = string(removed.Address.MemberID)
	}
	d.removedFrom[member] = append(d.removedFrom[member], removed)
}

// members is every member something came off, in id order.
func (d *departure) members() []string {
	out := make([]string, 0, len(d.removedFrom))
	for member := range d.removedFrom {
		out = append(out, member)
	}
	sort.Strings(out)
	return out
}

// translateDepart names a departure's refusal in this package's words: a
// hold whose caster the run does not hold is the run's fault
// (ErrInvalidSession); a sheet resolution cannot attach is ErrBadCharacter.
func translateDepart(leaver string, err error) error {
	if errors.Is(err, resolution.ErrBadParticipant) {
		return fmt.Errorf("leaver %q: %w: %v", leaver, ErrInvalidSession, err)
	}
	translated := translateResolution(err)
	if translated == err {
		return fmt.Errorf("leaver %q: %w: %v", leaver, ErrBadCharacter, err)
	}
	return translated
}
