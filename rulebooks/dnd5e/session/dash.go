// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// compile-time proof the striker seam also answers the composition's run ask:
// the same actor, bound to the same scope, as for a swing and a Disengage.
var _ encounter.Dasher = strikerSeam{}

// Dash is the sheet half of a monster's run (rpg-project#1131, 2026-11-27): it
// has the monster's own sheet pay the action a run costs, at resolution's
// door, and saves the sheet it changed. The composition has already decided to
// run (a driver asked, the facts it was given say an action is left) and it
// adds the run's speed to the movement left and tells the run in its walk's
// steps itself, so this records NOTHING and grants NOTHING: the feet stay the
// encounter's, the payment is the sheet's.
//
// WRITTEN THE WAY [strikerSeam.Disengage] IS WRITTEN: the cast from
// [Manager.castFor], a pure world view, one Resolve of the DRIVEN activation
// of Dash with the price [monster.CostOfDash] reads off the stored sheet, and
// one landing on the LIVE encounter — the composition is mid-verb over enc, so
// nothing is adopted and nothing is committed here. The landing saves the
// dirty monster sheet into the session record
// ([writeScope.replaceMonsterSheet]), which the verb's own commit persists.
//
// A driven Dash is charged one action and lands nothing; one sent without its
// payment is refused by resolution (resolution v0.84.0), so the price is
// never optional here.
//
// A refusal to pay is a malfunction, and it should not happen: the encounter
// asks for a run only when the facts [sheetSeam.readFacts] just gave it say an
// action is left on the very sheet the door charges. The error returns like
// any other, aborts the caller's verb, tells nothing and saves nothing.
//
// Errors: [ErrNoMember] for somebody the encounter does not hold,
// [ErrNoSheet] for a member with no stat block in this session (a player's
// Dash is its own Activate), and whatever resolution refuses, translated as an
// activation's refusals are.
func (s strikerSeam) Dash(ctx context.Context, enc *encounter.Encounter, member encounter.MemberID) error {
	roster, err := enc.Members()
	if err != nil {
		return fmt.Errorf("dash: %w", translate(err))
	}
	if !slices.ContainsFunc(roster, func(m encounter.Member) bool { return m.ID == member }) {
		return fmt.Errorf("dash: member %q: %w", member, ErrNoMember)
	}
	sheet, found := npcSheet(s.scope.data, string(member))
	if !found {
		return fmt.Errorf("dash: member %q has no stat block: %w", member, ErrNoSheet)
	}
	price, err := monster.CostOfDash(&monster.CostOfDashInput{Sheet: sheet})
	if err != nil {
		return fmt.Errorf("dash: member %q: %w", member, err)
	}

	cast, err := s.m.castFor(ctx, s.scope, roster, nil)
	if err != nil {
		return fmt.Errorf("dash: %w", err)
	}

	machine, err := resolution.NewActivation(&resolution.ActivationInput{
		MemberID: string(member),
		Ability:  refs.CombatAbilities.Dash(),
		Driven:   true,
		Roller:   &diceSeam{roller: s.m.dice},
	})
	if err != nil {
		return fmt.Errorf("dash: %w", translateResolution(err))
	}

	// THE VERB'S OWN ENCOUNTER, lent, mid-verb (rpg-project#753, T12).
	out, err := resolution.Resolve(ctx, s.m.resolutionInput(ctx, s.scope, resolutionAsk{
		Live:         enc,
		Participants: cast,
		Machine:      machine,
		Cost:         &resolution.Cost{PayerID: string(member), Profile: price.Cost},
	}))
	if err != nil {
		return fmt.Errorf("dash: %w", translateResolution(err))
	}
	if _, ok := out.Outcome.(resolution.ActivationOutcome); !ok {
		return fmt.Errorf("dash: %w: activation produced %T", ErrInvalidWorld, out.Outcome)
	}

	// Landed on the live encounter with no Record: the run is told by the
	// walk's steps after this returns, and a second telling would say it twice.
	if _, err := s.m.land(ctx, s.scope, out, &landing{Live: enc}); err != nil {
		return fmt.Errorf("dash: %w", err)
	}
	return nil
}
