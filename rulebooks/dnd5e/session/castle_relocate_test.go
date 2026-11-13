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
	"sync"
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

// relocationRead is one relocation's read-back: where alice was and was
// set down, and each player's catch-up acts.
type relocationRead struct {
	from, to spatial.Position
	acts     map[string][]session.Act
	// reloaded is alice's acts read again by a fresh manager.
	reloaded []session.Act
}

// relocationKey names one played relocation: the dial's stop and where the
// cleric stood.
type relocationKey struct {
	sharing  encounter.Sharing
	clericAt spatial.Position
}

// relocations are alice's relocation at each stop with the cleric watching
// the road or off down it, played ONCE in [TestMain].
var relocations struct {
	once  sync.Once
	err   error
	reads map[relocationKey]relocationRead
}

// relocationTests are the tests that read [relocations].
var relocationTests = []string{"TestCastleARelocationAtEachTheirOwn", "TestCastleARelocationAtSharedView"}

// buildTheRelocations plays all four. It takes no *testing.T because
// [TestMain] runs it.
func buildTheRelocations() {
	warnedGate.once.Do(func() { warnedGate.stores, warnedGate.err = buildTheWarnedGate() })
	if warnedGate.err != nil {
		relocations.err = warnedGate.err
		return
	}
	relocations.reads = map[relocationKey]relocationRead{}
	for _, sharing := range []encounter.Sharing{encounter.SharingOwn, encounter.SharingView} {
		for _, at := range []spatial.Position{watchingTheRoad, downTheRoad} {
			r, err := alicesRelocation(sharing, at)
			if err != nil {
				relocations.err = err
				return
			}
			relocations.reads[relocationKey{sharing, at}] = r
		}
	}
}

// alicesRelocation is the warned gate at the sharing stop given, the cleric
// stood at clericAt, and alice set down in the cells: the stored world
// relocated and saved, as a verb's commit would leave it, then read back.
func alicesRelocation(sharing encounter.Sharing, clericAt spatial.Position) (relocationRead, error) {
	ctx := context.Background()
	c, err := newJailHost(warnedGate.stores, sharing, 1)
	if err != nil {
		return relocationRead{}, err
	}
	standIn(c, "cleric", clericAt)
	enc, err := c.mgr.LoadWorldForTest(ctx, "sess")
	if err != nil {
		return relocationRead{}, err
	}
	r := relocationRead{from: placedAt(c, "alice"), acts: map[string][]session.Act{}}
	cells, err := enc.FreeCellsIn(theCellsRegion, 1)
	if err != nil {
		return relocationRead{}, err
	}
	r.to = cells[0]
	cause, err := core.ParseString(theMarchCause)
	if err != nil {
		return relocationRead{}, err
	}
	if _, err := enc.Relocate(&encounter.RelocateInput{
		Moves: []encounter.Relocation{{Member: "alice", To: r.to}},
		Cause: encounter.RelocationCause{Ref: *cause, Words: theMarchWords},
		Reach: session.InteractionReach,
	}); err != nil {
		return relocationRead{}, err
	}
	moved := enc.ToData()
	c.encounters.byID["sess"] = &moved
	for _, member := range []string{"alice", "cleric"} {
		read, err := c.mgr.Acts(ctx, &session.ActsInput{Session: "sess", Member: member})
		if err != nil {
			return relocationRead{}, err
		}
		r.acts[member] = read.Acts
	}
	// A RELOAD READS THE SAME ACTS: a fresh manager over the same stores.
	fresh, err := newJailHost(storesOf(c), "", 1)
	if err != nil {
		return relocationRead{}, err
	}
	read, err := fresh.mgr.Acts(ctx, &session.ActsInput{Session: "sess", Member: "alice"})
	if err != nil {
		return relocationRead{}, err
	}
	r.reloaded = read.Acts
	return r, nil
}

// theRelocation is one played relocation.
func theRelocation(t *testing.T, sharing encounter.Sharing, clericAt spatial.Position) relocationRead {
	t.Helper()
	relocations.once.Do(buildTheRelocations)
	require.NoError(t, relocations.err, "the relocations")
	return relocations.reads[relocationKey{sharing, clericAt}]
}

// relocationsFor is every RelocateAct in member's read.
func relocationsFor(t *testing.T, r relocationRead, member string) []found[session.RelocateAct] {
	t.Helper()
	for _, a := range r.acts[member] {
		require.Empty(t, a.Undecoded, "%s: no undecoded act", member)
	}
	return where[session.RelocateAct](r.acts[member], func(session.RelocateAct) bool { return true })
}

// TestCastleARelocationAtEachTheirOwn: alice is shown both ends and her
// words; the cleric watching the road saw her leave and is shown that
// only — where from, never where to, no words; a cleric off down the road,
// who saw neither end, is shown nothing. A reload reads the same acts.
func TestCastleARelocationAtEachTheirOwn(t *testing.T) {
	r := theRelocation(t, encounter.SharingOwn, watchingTheRoad)
	from, to := r.from, r.to

	mine := relocationsFor(t, r, "alice")
	require.Len(t, mine, 1, "alice is shown her move")
	require.Equal(t, session.RelocateAct{
		Mover: aliceParty, From: &from, To: &to, Cause: theMarchCause, Words: theMarchWords, End: session.RelocateBoth,
	}, mine[0].body)
	require.Nil(t, mine[0].act.Perceived, "her own")

	watched := relocationsFor(t, r, "cleric")
	require.Len(t, watched, 1, "the cleric saw her leave")
	require.Equal(t, session.RelocateAct{
		Mover: aliceParty, From: &from, Cause: theMarchCause, End: session.RelocateLeft,
	}, watched[0].body, "where from, never where to, and no words")

	require.Equal(t, r.acts["alice"], r.reloaded, "the record replays to the same acts")

	away := theRelocation(t, encounter.SharingOwn, downTheRoad)
	require.Empty(t, relocationsFor(t, away, "cleric"), "who saw neither end is shown nothing")
}

// TestCastleARelocationAtSharedView: what came through a friend is shown
// like your own (the operator's ruling). The cleric off down the road, who
// saw neither end, is shown alice's own version through her: both ends and
// her words. The cleric watching the road, who saw her leave, is shown one
// act: her own leaving, with the destination and the words through alice.
func TestCastleARelocationAtSharedView(t *testing.T) {
	r := theRelocation(t, encounter.SharingView, downTheRoad)
	from, to := r.from, r.to
	through := relocationsFor(t, r, "cleric")
	require.Len(t, through, 1, "the cleric is shown alice's move")
	require.Equal(t, session.RelocateAct{
		Mover: aliceParty, From: &from, To: &to, Cause: theMarchCause, Words: theMarchWords, End: session.RelocateBoth,
	}, through[0].body)
	require.NotNil(t, through[0].act.Perceived)
	require.Equal(t, []session.Party{aliceParty}, through[0].act.Perceived.Via, "through alice")

	r = theRelocation(t, encounter.SharingView, watchingTheRoad)
	from, to = r.from, r.to
	merged := relocationsFor(t, r, "cleric")
	require.Len(t, merged, 1, "one act for one move")
	require.Equal(t, session.RelocateAct{
		Mover: aliceParty, From: &from, To: &to, Cause: theMarchCause, Words: theMarchWords, End: session.RelocateBoth,
	}, merged[0].body)
	require.Equal(t, []session.Party{aliceParty}, merged[0].act.Perceived.Via, "the parts she lacked came through alice")

	mine := relocationsFor(t, r, "alice")
	require.Len(t, mine, 1)
	require.Nil(t, mine[0].act.Perceived, "alice's own is hers")
}
