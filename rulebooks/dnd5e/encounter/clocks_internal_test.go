// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// TestClockOfReportsAMemberOnNoClockInsteadOfGuessing pins ClockOf's
// on-no-clock check — the net under the clock verbs' non-atomic mutate
// phases (doc.go). It was added in step 4.1 explicitly unreachable, with a
// promise that "a test lands with the verb that can reach it"; this is that
// test, and it is deliberately WHITE-BOX: every public verb upholds R6, and
// the load seam re-homes anyone it finds on no clock, so the only way to
// probe the net is to fabricate the exact defect it exists to catch — a verb
// that dropped somebody from a bubble and forgot to re-home them (the
// mid-Dissolve failure window, frozen).
//
// What makes the check worth a test at all is the SHAPE of the wrong answer
// it prevents: without it, a member on no clock reports ClockWorld — "free
// roaming" — which is plausible, silent, and would be persisted by the next
// save. With it, the defect is loud: ErrInvalidData, never a guess.
func TestClockOfReportsAMemberOnNoClockInsteadOfGuessing(t *testing.T) {
	enc, err := NewEncounter(&SetupInput{
		Field: FieldInput{
			Rooms: []RoomInput{{ID: "room-1", Width: 10, Height: 10}},
		},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Room: "room-1", Position: spatial.Position{X: 2, Y: 2}},
			{ID: "goblin", Kind: KindMonster, Room: "room-1", Position: spatial.Position{X: 7, Y: 7}},
		},
		Endings: []EndingInput{
			{Key: "called", Trigger: TriggerExternal{}},
		},
	})
	require.NoError(t, err)

	_, err = enc.Form(&FormInput{Order: []MemberID{"alice", "goblin"}})
	require.NoError(t, err)

	// The bug, fabricated: alice leaves the fight and nobody re-homes her.
	// No public verb can do this — that is the point of the net.
	_, err = enc.bubbles[0].Remove(&clock.RemoveInput{ID: "alice"})
	require.NoError(t, err)

	_, err = enc.ClockOf(&ClockOfInput{Member: "alice"})
	require.Error(t, err, "a member on no clock must be a loud defect, not a free-roamer")
	require.ErrorIs(t, err, ErrInvalidData)
	require.ErrorContains(t, err, "on no clock")

	// The member still IN the fight is unaffected — the defect is reported
	// exactly where it is, not smeared across the encounter.
	out, err := enc.ClockOf(&ClockOfInput{Member: "goblin"})
	require.NoError(t, err)
	require.Equal(t, ClockTurn, out.Kind)
}
