// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// compile-time proof the striker seam also answers the composition's
// Disengage ask: the two are the same actor, bound to the same scope.
var _ encounter.Disengager = strikerSeam{}

// Disengage is the sheet half of a monster's bonus-action Disengage
// (rpg-toolkit#2032): it runs resolution's DRIVEN activation of Disengage for
// member and saves the sheet it changed. The composition has already decided
// the bonus action is spent (a [encounter.SheetFacts.BonusDisengage] trait, a
// bonus action left, a first step out of a standing hostile's reach), and it
// tells the story itself after this returns, so this records NOTHING.
//
// THE SAME CONDITION A PLAYER'S DISENGAGE LEAVES, landed by the same owner.
// resolution's driven arm publishes the Disengaging condition through the
// monster side of the cast, so the walk the composition takes next provokes no
// opportunity attack, the condition clears itself on its owner's turn end when
// [announcerSeam] crosses the boundary, and a second Disengage in one turn
// replaces the first rather than stacking — every one of those is
// resolution's and the condition's, none of them this seam's. This seam builds
// no condition (TestSessionConstructsNoCheck).
//
// RUN THE WAY [strikerSeam.Strike] RUNS A SWING: the cast from [Manager.castFor],
// a pure world view, one Resolve, and one landing on the LIVE encounter — the
// composition is mid-verb over enc, so nothing is adopted and nothing is
// committed here. The landing saves the dirty monster sheet into the session
// record ([writeScope.replaceMonsterSheet]), which the verb's own commit
// persists. No Cost: the driver already spent the bonus action, and a driven
// activation charges nothing.
//
// Errors are malfunctions and abort the caller's verb: [ErrNoMember] for
// somebody the encounter does not hold, [ErrNoSheet] for a member with no
// stat block in this session (a player's Disengage is its own Activate), and
// whatever resolution refuses, translated as an activation's refusals are.
func (s strikerSeam) Disengage(ctx context.Context, enc *encounter.Encounter, member encounter.MemberID) error {
	roster, err := enc.Members()
	if err != nil {
		return fmt.Errorf("disengage: %w", translate(err))
	}
	if !slices.ContainsFunc(roster, func(m encounter.Member) bool { return m.ID == member }) {
		return fmt.Errorf("disengage: member %q: %w", member, ErrNoMember)
	}
	if _, found := npcSheet(s.scope.data, string(member)); !found {
		return fmt.Errorf("disengage: member %q has no stat block: %w", member, ErrNoSheet)
	}

	cast, err := s.m.castFor(ctx, s.scope, roster, nil)
	if err != nil {
		return fmt.Errorf("disengage: %w", err)
	}

	machine, err := resolution.NewActivation(&resolution.ActivationInput{
		MemberID: string(member),
		Ability:  refs.CombatAbilities.Disengage(),
		Driven:   true,
		Roller:   &diceSeam{roller: s.m.dice},
	})
	if err != nil {
		return fmt.Errorf("disengage: %w", translateResolution(err))
	}

	// A pure view for resolution's Input.World — a mid-verb read, never the
	// storage boundary (encounter v0.43.0, #1385).
	out, err := resolution.Resolve(ctx, s.m.resolutionInput(ctx, s.scope, resolutionAsk{
		World:        enc.WorldView(),
		Participants: cast,
		Machine:      machine,
	}))
	if err != nil {
		return fmt.Errorf("disengage: %w", translateResolution(err))
	}
	if _, ok := out.Outcome.(resolution.ActivationOutcome); !ok {
		return fmt.Errorf("disengage: %w: activation produced %T", ErrInvalidWorld, out.Outcome)
	}

	// Landed on the live encounter with no Record: the composition tells the
	// beat after this returns, and a second telling would say it twice.
	if _, err := s.m.land(ctx, s.scope, out, &landing{Live: enc}); err != nil {
		return fmt.Errorf("disengage: %w", err)
	}
	return nil
}
