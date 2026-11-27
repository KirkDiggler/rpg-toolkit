// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"
	"slices"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// compile-time proof the striker seam also answers the composition's ask to
// have a creature use an ability: the same actor, bound to the same scope, as
// for a swing, a Disengage and a run.
var _ encounter.Activator = strikerSeam{}

// Activate has a monster use one ability its stat block lets it, paid at
// resolution's door and told in the story (rpg-project#1131, ADR-0051). The
// composition has already decided to (a table row said `activate`, and the
// facts [sheetSeam.readFacts] just gave it listed the ability as payable on a
// turn); this resolves it, saves the sheet it changed and TELLS it: unlike a
// run, the composition cannot know what an ability did, so the story is this
// seam's.
//
// WRITTEN THE WAY [strikerSeam.Dash] IS WRITTEN: the roster check, the stored
// sheet ([npcSheet]), the cast from [Manager.castFor], one Resolve on the
// LIVE encounter and one landing with Live set, so nothing is adopted and
// nothing is committed here: the calling verb commits. The differences are
// the three the ability brings. The description is the rules': the entry of
// [monster.AbilitiesOf] whose word was asked, which names the creature's own
// pool. The machine is the one a player's described ability runs
// ([resolution.NewAction]), and the cost is that description's, cloned for the
// reason [Manager.Activate] gives. And the landing carries a Record, the one
// a player's activation tells: an activated beat naming the ability and one
// activation-result for each thing it did. The landing is the strike's, which
// also lands with both Live and Record ([Manager.attackLanding]).
//
// Errors: [ErrNoMember] for somebody the encounter does not hold,
// [ErrNoSheet] for a member with no stat block in this session, an error
// naming the member and the word when the rules list no such ability (the
// encounter asked for a word its facts had just listed: a malfunction), and
// whatever resolution refuses, translated as an activation's refusals are.
func (s strikerSeam) Activate(ctx context.Context, enc *encounter.Encounter, member encounter.MemberID, word string) error {
	roster, err := enc.Members()
	if err != nil {
		return fmt.Errorf("activate npc: %w", translate(err))
	}
	if !slices.ContainsFunc(roster, func(m encounter.Member) bool { return m.ID == member }) {
		return fmt.Errorf("activate npc: member %q: %w", member, ErrNoMember)
	}
	sheet, found := npcSheet(s.scope.data, string(member))
	if !found {
		return fmt.Errorf("activate npc: member %q has no stat block: %w", member, ErrNoSheet)
	}
	read, err := monster.AbilitiesOf(&monster.AbilitiesOfInput{Sheet: sheet})
	if err != nil {
		return fmt.Errorf("activate npc: member %q: %w", member, err)
	}
	var definition *combatActions.Definition
	for _, a := range read.Abilities {
		if a.Word == word {
			definition = a.Definition
			break
		}
	}
	if definition == nil {
		return fmt.Errorf("activate npc: member %q was asked to use %q, which its stat block does not let it", member, word)
	}

	cast, err := s.m.castFor(ctx, s.scope, roster, nil)
	if err != nil {
		return fmt.Errorf("activate npc: %w", err)
	}

	machine, err := resolution.NewAction(&resolution.ActionInput{
		AttackerID: string(member),
		Definition: *definition,
		Roller:     &diceSeam{roller: s.m.dice},
	})
	if err != nil {
		return fmt.Errorf("activate npc: %w", translateResolution(err))
	}

	// THE VERB'S OWN ENCOUNTER, lent, mid-verb (rpg-project#753, T12).
	out, err := resolution.Resolve(ctx, s.m.resolutionInput(ctx, s.scope, resolutionAsk{
		Live:         enc,
		Participants: cast,
		Machine:      machine,
		// Cloned as a cast clones it, so the charge and the description cannot alias.
		Cost: &resolution.Cost{PayerID: string(member), Profile: combatActions.CloneSpendProfile(definition.Cost)},
	}))
	if err != nil {
		return fmt.Errorf("activate npc: %w", translateResolution(err))
	}
	activated, ok := out.Outcome.(resolution.ActivationOutcome)
	if !ok {
		return fmt.Errorf("activate npc: %w: activation produced %T", ErrInvalidWorld, out.Outcome)
	}

	// Landed on the live encounter WITH a Record: the composition cannot tell
	// what an ability did, so this seam tells it, as a player's activation is.
	if _, err := s.m.land(ctx, s.scope, out, &landing{
		Live: enc,
		Record: func(enc *encounter.Encounter, _ concentration) error {
			_, err := enc.RecordActivation(&encounter.RecordActivationInput{
				Actor: member,
				Ability: encounter.ActivationIdentity{
					Ref:  definition.Ref.String(),
					Name: definition.Name,
				},
				Results: activationResults(activated.Effects),
			})
			return err
		},
	}); err != nil {
		return fmt.Errorf("activate npc: %w", err)
	}
	return nil
}
