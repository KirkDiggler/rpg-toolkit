// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// members_test.go is the compile minting every monster's member id
// (rpg-project#542, "Launch"): a host spawns the members a compiled dungeon
// names and mints none of its own.

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// The heirloom tomb's two unnamed skeletons, spelled once so a scene can
// replace them.
const heirloomSkeletons = `  - { ref: "dnd5e:monsters:skeleton", at: [11,3], targeting: lowest-health }
  - { ref: "dnd5e:monsters:skeleton", at: [13,5], targeting: lowest-health }
`

// heirloomWith is the heirloom tomb with its two skeletons replaced.
func heirloomWith(t *testing.T, placements string) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/reference-tomb-heirloom.yaml")
	require.NoError(t, err)
	source := string(raw)
	require.Contains(t, source, heirloomSkeletons, "the scene edits the lines the file ships")

	return strings.Replace(source, heirloomSkeletons, placements, 1)
}

// memberIDs is every compiled monster's member id, keyed by its authored id
// or, for an unnamed one, its authored cell.
func memberIDs(compiled dungeonspec.Compiled) map[string]string {
	out := make(map[string]string, len(compiled.Monsters))
	for _, m := range compiled.Monsters {
		key := m.ID
		if key == "" {
			key = fmt.Sprintf("%s@%d,%d", m.Ref, int(m.At.X), int(m.At.Y))
		}
		out[key] = m.MemberID
	}
	return out
}

// TestCompileMintsEveryMonstersMemberID is the done-when: two unnamed goblins
// and one named placement yield three distinct member ids, the named one
// verbatim — and the named one between them still spends its ordinal, so
// naming it does not renumber the goblin after it.
func TestCompileMintsEveryMonstersMemberID(t *testing.T) {
	compiled, err := dungeonspec.Load([]byte(heirloomWith(t,
		`  - { ref: "dnd5e:monsters:goblin", at: [11,3] }
  - { id: lookout, ref: "dnd5e:monsters:goblin", at: [12,3] }
  - { ref: "dnd5e:monsters:goblin", at: [13,5] }
`)))
	require.NoError(t, err)

	ids := memberIDs(compiled)
	require.Equal(t, "goblin-1", ids["dnd5e:monsters:goblin@11,3"])
	require.Equal(t, "lookout", ids["lookout"], "a named placement keeps its id verbatim")
	require.Equal(t, "goblin-3", ids["dnd5e:monsters:goblin@13,5"], "the named goblin spent ordinal 2")
	require.Equal(t, "captain", ids["captain"])

	seen := map[string]bool{}
	for _, m := range compiled.Monsters {
		require.NotEmpty(t, m.MemberID, "every monster carries a member id")
		require.False(t, seen[m.MemberID], "member id %q is minted twice", m.MemberID)
		seen[m.MemberID] = true
	}
}

// An authored id that spells what the ordinal mints for another placement is
// refused, naming both, rather than failing as a duplicate at spawn.
func TestCompileRefusesAMemberIDClaimedTwice(t *testing.T) {
	_, err := dungeonspec.Load([]byte(heirloomWith(t,
		`  - { id: goblin-2, ref: "dnd5e:monsters:goblin", at: [11,3] }
  - { ref: "dnd5e:monsters:goblin", at: [13,5] }
`)))
	require.ErrorIs(t, err, dungeonspec.ErrBadSpec)
	var verr *dungeonspec.ValidationError
	require.ErrorAs(t, err, &verr)
	require.Len(t, verr.Errors, 1)
	require.Contains(t, verr.Errors[0].Message, `member id "goblin-2" is claimed twice`)
	require.Contains(t, verr.Errors[0].Message, `"goblin-2" (dnd5e:monsters:goblin at [11,3])`)
	require.Contains(t, verr.Errors[0].Message, "dnd5e:monsters:goblin at [13,5]")
}

// The single-room dialect is minted by the same compile: every declaration
// there is named, and joins under its name.
func TestSingleRoomMembersJoinUnderTheirNames(t *testing.T) {
	raw, err := os.ReadFile("testdata/world-builder-v4-front-room.yaml")
	require.NoError(t, err)
	compiled, err := dungeonspec.Load(raw)
	require.NoError(t, err)

	require.NotEmpty(t, compiled.Monsters)
	for _, m := range compiled.Monsters {
		require.NotEmpty(t, m.ID)
		require.Equal(t, m.ID, m.MemberID)
	}
}
