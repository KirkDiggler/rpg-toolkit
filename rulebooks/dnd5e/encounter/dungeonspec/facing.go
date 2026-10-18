// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// facing.go LOWERS AN AUTHORED FACING (rpg-project#582, sneaking design R13):
// the compass word a placement or a start was written with becomes the
// bearing the encounter seats a member with, once, here, through the
// encounter's own [encounter.FacingFromCompass]. The word is still refused by
// its author's path where it is read (validate.go for the v2 dialect,
// single_room_decode.go for the single room); this is the conversion, not a
// second validation, and it refuses only what slipped past both.

// lowerFacing is one AUTHORED word as a bearing. Callers ask only for a word
// somebody wrote: no word is no facing, which stays nil on the compile and
// which the encounter seats as [encounter.UnstatedFacing].
//
// Errors: the word is not one of the eight ([encounter.ErrBadFacing]).
func lowerFacing(word string) (*float64, error) {
	degrees, err := encounter.FacingFromCompass(word)
	if err != nil {
		return nil, err
	}

	return &degrees, nil
}

// lowerFacings fills every placement's [MonsterPlacement.FacingDegrees] from
// its authored [MonsterPlacement.Facing].
//
// Errors: a [*ValidationError] (an [ErrBadSpec]) naming the placement whose
// word is not a compass direction.
func lowerFacings(monsters []MonsterPlacement) ([]MonsterPlacement, error) {
	for i := range monsters {
		if monsters[i].Facing == "" {
			continue
		}
		degrees, err := lowerFacing(monsters[i].Facing)
		if err != nil {
			return nil, &ValidationError{Errors: []FieldError{{
				Message: fmt.Sprintf("monster %s: facing: %v", describeMonster(monsters[i]), err),
			}}}
		}
		monsters[i].FacingDegrees = degrees
	}

	return monsters, nil
}
