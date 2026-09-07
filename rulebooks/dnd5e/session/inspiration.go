// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// inspirationReachFeet is how far a bard's inspiration reaches: 60 feet, RAW.
//
// UNLIKE HELP'S FIVE, this one is the rule rather than a ruling. Help's reach
// was narrowed to "stand next to a friend" on purpose (see [helpReachFeet]);
// Bardic Inspiration says 60 feet and there is nothing fiddly about it to
// simplify. What IS diverged is the other half of RAW's sentence — "a creature
// who can hear you" — because no hearing primitive exists in this stack. Sight
// does, hearing does not, and inventing one to gate a level-1 feature would be
// a capability arriving ahead of the design that needs it. So the rule here is
// range only, and deafness, silence and hearing are named on the shelf rather
// than discovered later.
const inspirationReachFeet = 60

// allyCandidatesFor builds the candidate universe for one ability that takes an
// ally.
//
// A SWITCH ON THE REF, and deliberately not a table of reaches. The two
// abilities here differ by more than a number — inspiration also refuses a
// creature who already holds a die — and a shared table keyed by reach would
// have hidden that second difference behind the first. A ref this seam does
// not recognise falls back to Help's rule, which is the narrowest one: a new
// targeted ability reaching this switch is offered adjacent allies rather than
// the whole room, and the fix is a case here.
func (m *Manager) allyCandidatesFor(
	ctx context.Context,
	ability *core.Ref,
	enc *encounter.Encounter,
	standing encounter.Standing,
	roster []encounter.Member,
	positions map[string]spatial.Position,
	holdings []intel.Holding,
	member string,
) ([]targetPreflight, error) {
	if ability != nil && ability.ID == refs.Features.BardicInspiration().ID {
		return m.inspirationCandidates(ctx, enc, standing, roster, positions, holdings, member)
	}
	return helpCandidates(enc, standing, roster, positions, holdings, member)
}

// inspirationCandidates is Bardic Inspiration's candidate universe: allies this
// member can see, standing, within 60 feet, who are not already holding a die.
//
// # Not yourself, and that is free here
//
// RAW says "a creature other than yourself". The shared ally builder already
// skips the actor when it walks the holdings — you do not hold a sighting of
// yourself — so the rule is enforced by the mechanism rather than by a second
// check. The feature refuses a self-grant again at the point of activation,
// where both identities are in hand.
//
// # Already inspired is a SHORTFALL, not an error
//
// RAW allows one die at a time, and replacing one would spend a use to
// overwrite a use — the player would see a charge and no change. So the row
// stays, with "already inspired" on it, the way "ally is down" and "ally out of
// reach" do: visible before it is chosen rather than discovered by choosing it.
//
// # It reads sheets, and only the ones already in the answer
//
// Whether somebody holds a die is written on their sheet and nowhere else, so
// this asks. It asks ONLY about candidates that already passed sight, kind,
// standing and reach — the same cost argument the standing consult makes one
// layer up — and a sheet it cannot read leaves the candidate available rather
// than silently unavailable, because "we could not check" and "they already
// have one" are different answers and only one of them is about the game.
func (m *Manager) inspirationCandidates(
	ctx context.Context,
	enc *encounter.Encounter,
	standing encounter.Standing,
	roster []encounter.Member,
	positions map[string]spatial.Position,
	holdings []intel.Holding,
	member string,
) ([]targetPreflight, error) {
	allies, err := allyCandidates(
		enc, standing, roster, positions, holdings, member, inspirationReachFeet, "inspiration",
	)
	if err != nil {
		return nil, err
	}

	for i := range allies {
		if !allies[i].available {
			continue
		}
		held, err := m.holdsInspiration(ctx, string(allies[i].member))
		if err != nil {
			return nil, fmt.Errorf("inspiration offers: %w", err)
		}
		if held {
			why := Shortfall{Reason: ShortfallUnavailable, Text: "already inspired"}
			allies[i].available = false
			allies[i].why = &why
		}
	}
	return allies, nil
}

// holdsInspiration reports whether a member's stored sheet already carries a
// Bardic Inspiration die.
//
// It PEEKS at the ref rather than loading the condition, because the question
// is "is one of these an inspiration die" and loading every condition to answer
// it would make a candidate list pay for the whole sheet's behaviour. A member
// with no stored sheet — a monster ally, a member the repository cannot
// produce — holds nothing this seam can see, which is the honest answer to a
// question about a character sheet that does not exist.
func (m *Manager) holdsInspiration(ctx context.Context, member string) (bool, error) {
	data, err := m.characters.GetCharacter(ctx, member)
	if err != nil || data == nil {
		return false, nil
	}
	for _, raw := range data.Conditions {
		var peek struct {
			Ref *core.Ref `json:"ref"`
		}
		if json.Unmarshal(raw, &peek) != nil || peek.Ref == nil {
			// A condition this build cannot read is not a die it can name.
			// The sheet's own loader is what refuses an unreadable blob; a
			// candidate list is not the place to fail a whole Afford over one.
			continue
		}
		if peek.Ref.ID == refs.Conditions.Inspired().ID {
			return true, nil
		}
	}
	return false, nil
}
