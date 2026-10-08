// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// mintMemberIDs fills every placement's [MonsterPlacement.MemberID], in
// authored order, and refuses a dungeon in which two monsters would join
// under one id.
//
// THE COMPILE MINTS, ONCE, FOR BOTH DIALECTS (rpg-project#542, "Launch"). A
// host that minted its own would be a second answer to "which member is this
// placement", and the file's own `mind:` and `{ down }` already name members
// by the id the compile hands out.
//
// THE REF IS PARSED, for a named placement too, so a malformed one is refused
// here rather than minting a member called "dnd5e:monsters:skeleton-1" that
// nothing downstream can read.
//
// ONE ID, ONE MONSTER. An authored id that spells what the ordinal mints for
// another placement (`id: skeleton-1` on the second skeleton) is refused,
// naming both placements, rather than letting the second spawn fail halfway
// through a launch.
//
// Errors: a [*ValidationError] (an [ErrBadSpec]) for a malformed ref or a
// claimed-twice id.
func mintMemberIDs(monsters []MonsterPlacement) ([]MonsterPlacement, error) {
	ordinals := make(map[string]int, len(monsters))
	claimed := make(map[string]int, len(monsters))
	for i := range monsters {
		m := &monsters[i]
		parsed, err := core.ParseString(m.Ref)
		if err != nil {
			return nil, &ValidationError{Errors: []FieldError{{
				Message: fmt.Sprintf("monster %s: ref %q: %v", describeMonster(*m), m.Ref, err),
			}}}
		}
		ordinals[m.Ref]++
		id := m.ID
		if id == "" {
			id = fmt.Sprintf("%s-%d", parsed.ID, ordinals[m.Ref])
		}
		if prev, taken := claimed[id]; taken {
			return nil, &ValidationError{Errors: []FieldError{{
				Message: fmt.Sprintf(
					"member id %q is claimed twice: by %s and by %s — a monster's member id is its authored id, "+
						"or its ref's id plus an ordinal when it has none, and two monsters cannot share one",
					id, describeMonster(monsters[prev]), describeMonster(*m)),
			}}}
		}
		claimed[id] = i
		m.MemberID = id
	}

	return monsters, nil
}

// describeMonster names one placement the way an author would find it in the
// file: by its id when it has one, and by its ref and authored cell either way.
func describeMonster(m MonsterPlacement) string {
	where := fmt.Sprintf("%s at [%d,%d]", m.Ref, int(m.At.X), int(m.At.Y))
	if m.ID == "" {
		return where
	}

	return fmt.Sprintf("%q (%s)", m.ID, where)
}
