// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_relocate_test.go is a member set down at a place without walking,
// at the session door (rpg-project#925): the encounter's `relocated` beat,
// one version per audience, folded into a [session.RelocateAct] per viewer.
// No session verb relocates yet (the engine's jail consequence will), so the
// move is made on the stored world and read back as a host reads acts:
// through [session.Manager.Acts].

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// theMarchWords are the words alice is told when she is set down.
const theMarchWords = "The guards march you to the cells."

// theMarchCause is the cause the relocation names.
const theMarchCause = "dnd5e:consequences:march-to-the-cells"

// alicesRelocation is the warned gate at the sharing stop given, the cleric
// stood at clericAt, and alice set down in the cells: the stored world
// relocated and saved, as a verb's commit would leave it. It returns the
// run, where alice was and where she was set down.
func alicesRelocation(t *testing.T, sharing encounter.Sharing, clericAt spatial.Position) (*freeRoamCastle, spatial.Position, spatial.Position) {
	t.Helper()
	warnedGate.once.Do(func() { warnedGate.stores, warnedGate.err = buildTheWarnedGate() })
	require.NoError(t, warnedGate.err)
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{1}},
		sessions: newFakeSessions(), encounters: newFakeEncounters(),
		characters: newFakeCharacters(), seats: newFakeSeats(),
	}
	require.NoError(t, restoreInto(c, warnedGate.stores))
	c.encounters.byID["sess"].Sharing = sharing
	c.standInTheRecord(t, "cleric", clericAt)
	c.mgr = c.manager(t)

	enc := c.world(t)
	from := memberOf(t, enc, "alice").Position
	cells, err := enc.FreeCellsIn("region-cells", 1)
	require.NoError(t, err)
	cause, err := core.ParseString(theMarchCause)
	require.NoError(t, err)
	_, err = enc.Relocate(&encounter.RelocateInput{
		Moves: []encounter.Relocation{{Member: "alice", To: cells[0]}},
		Cause: encounter.RelocationCause{Ref: *cause, Words: theMarchWords},
		Reach: session.InteractionReach,
	})
	require.NoError(t, err)
	moved := enc.ToData()
	c.encounters.byID["sess"] = &moved
	return c, from, cells[0]
}

// relocationsFor is every RelocateAct member reads back, and the read.
func relocationsFor(t *testing.T, c *freeRoamCastle, member string) ([]found[session.RelocateAct], []session.Act) {
	t.Helper()
	read, err := c.mgr.Acts(context.Background(), &session.ActsInput{Session: "sess", Member: member})
	require.NoError(t, err)
	for _, a := range read.Acts {
		require.Empty(t, a.Undecoded, "%s: no undecoded act", member)
	}
	return where[session.RelocateAct](read.Acts, func(session.RelocateAct) bool { return true }), read.Acts
}

// TestCastleARelocationAtEachTheirOwn: alice is shown both ends and her
// words; the cleric watching the road saw her leave and is shown that
// only — where from, never where to, no words; a cleric off down the road,
// who saw neither end, is shown nothing. A reload reads the same acts.
func TestCastleARelocationAtEachTheirOwn(t *testing.T) {
	c, from, to := alicesRelocation(t, encounter.SharingOwn, watchingTheRoad)

	mine, aliceActs := relocationsFor(t, c, "alice")
	require.Len(t, mine, 1, "alice is shown her move")
	require.Equal(t, session.RelocateAct{
		Mover: aliceParty, From: &from, To: &to, Cause: theMarchCause, Words: theMarchWords, End: session.RelocateBoth,
	}, mine[0].body)
	require.Nil(t, mine[0].act.Perceived, "her own")

	watched, _ := relocationsFor(t, c, "cleric")
	require.Len(t, watched, 1, "the cleric saw her leave")
	require.Equal(t, session.RelocateAct{
		Mover: aliceParty, From: &from, Cause: theMarchCause, End: session.RelocateLeft,
	}, watched[0].body, "where from, never where to, and no words")

	// A RELOAD READS THE SAME ACTS: a fresh manager over the same stores.
	c.mgr = c.manager(t)
	_, again := relocationsFor(t, c, "alice")
	require.Equal(t, aliceActs, again, "the record replays to the same acts")

	away, _, _ := alicesRelocation(t, encounter.SharingOwn, downTheRoad)
	none, _ := relocationsFor(t, away, "cleric")
	require.Empty(t, none, "who saw neither end is shown nothing")
}

// TestCastleARelocationAtSharedView: what came through a friend is shown
// like your own (the operator's ruling). The cleric off down the road, who
// saw neither end, is shown alice's own version through her: both ends and
// her words. The cleric watching the road, who saw her leave, is shown one
// act: her own leaving, with the destination and the words through alice.
func TestCastleARelocationAtSharedView(t *testing.T) {
	c, from, to := alicesRelocation(t, encounter.SharingView, downTheRoad)
	through, _ := relocationsFor(t, c, "cleric")
	require.Len(t, through, 1, "the cleric is shown alice's move")
	require.Equal(t, session.RelocateAct{
		Mover: aliceParty, From: &from, To: &to, Cause: theMarchCause, Words: theMarchWords, End: session.RelocateBoth,
	}, through[0].body)
	require.NotNil(t, through[0].act.Perceived)
	require.Equal(t, []session.Party{aliceParty}, through[0].act.Perceived.Via, "through alice")

	c, from, to = alicesRelocation(t, encounter.SharingView, watchingTheRoad)
	merged, _ := relocationsFor(t, c, "cleric")
	require.Len(t, merged, 1, "one act for one move")
	require.Equal(t, session.RelocateAct{
		Mover: aliceParty, From: &from, To: &to, Cause: theMarchCause, Words: theMarchWords, End: session.RelocateBoth,
	}, merged[0].body)
	require.Equal(t, []session.Party{aliceParty}, merged[0].act.Perceived.Via, "the parts she lacked came through alice")

	mine, _ := relocationsFor(t, c, "alice")
	require.Len(t, mine, 1)
	require.Nil(t, mine[0].act.Perceived, "alice's own is hers")
}
