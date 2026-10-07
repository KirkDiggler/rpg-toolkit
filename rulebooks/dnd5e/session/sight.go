// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

// COMPILE-TIME PROOF that the one sheet seam answers the composition's sight
// question as well as its sheet question (sheets.go).
var _ encounter.Sight = sheetSeam{}

// Sight reports how far each member can see, in cells — asked of each
// member's own sheet through [combat.SightHolder], converted once via
// encounter.CellsFromFeet (rpg-project#538, R6, R12).
//
// THE DEFAULT IS THE SHEET'S, NOT OURS. A character answers from its race
// table and a monster from its stat block's senses; a sheet that states no
// range answers the rulebook's stated default itself ([combat.DefaultSightFeet]),
// so silence means the same for both kinds and this seam holds no number of
// its own. A world NPC's content states no senses at all and is not a
// combatant sheet the rulebook can load, so it is answered the same stated
// default directly — the rulebook's number, read from the rulebook.
//
// A MEMBER THE VERB HOLDS NO SHEET FOR IS REFUSED, never answered with a
// default. A default applied to a missing row is a sight range nobody stated
// for a member nobody can read, and the percept it builds would be a guess
// dressed as a fact. Nothing is cached between consults: a sheet is read at
// the moment the composition asks (sheets.go, "no cache").
//
// STILL LOS-BOUNDED: this answers RANGE alone. The composition's own
// rebuildPercepts walls off anything a wall or door blocks.
func (s sheetSeam) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		feet, err := s.sightFeetOf(id)
		if err != nil {
			return nil, err
		}
		out[id] = encounter.CellsFromFeet(feet)
	}

	return out, nil
}

// sightFeetOf asks one member's sheet how far it sees, in feet.
func (s sheetSeam) sightFeetOf(id encounter.MemberID) (int, error) {
	held, err := s.sheetOf(id)
	if err != nil {
		return 0, err
	}

	var holder combat.SightHolder
	switch {
	case held.character != nil:
		loaded, loadErr := character.Load(s.ctx, held.character)
		if loadErr != nil {
			return 0, fmt.Errorf("character %q: %w: %v", id, ErrBadCharacter, loadErr)
		}
		holder = loaded
	case held.monster != nil:
		loaded, loadErr := monster.Load(s.ctx, held.monster)
		if loadErr != nil {
			return 0, fmt.Errorf("monster %q: %w: %v", id, ErrInvalidSession, loadErr)
		}
		holder = loaded
	default:
		// A placed world NPC: its content states no senses (sheetOf found
		// its record, so this is a sheet that states none, not a missing one).
		return combat.DefaultSightFeet, nil
	}

	return holder.SightFeet(), nil
}
