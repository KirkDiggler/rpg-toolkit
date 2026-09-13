// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// perceptpass_internal_test.go pins what [Encounter.rebuildPercepts] hands to
// mind/perception, and the one thing adoption could have changed in silence:
// who is IN the pass (rpg-toolkit#1691).

// seesNobody is the Sight capability for an observer whose light has gone out:
// every member is answered, and every answer is zero cells.
type seesNobody struct{}

func (seesNobody) Sight(members []MemberID) (map[MemberID]int, error) {
	out := make(map[MemberID]int, len(members))
	for _, id := range members {
		out[id] = 0
	}
	return out, nil
}

// perceptPassEncounter is a flat open room of players — players, so first
// light never forms a fight and the pass is the only thing under test.
func perceptPassEncounter(t *testing.T, at map[MemberID]spatial.Position) *Encounter {
	t.Helper()

	members := make([]MemberInput, 0, len(at))
	for _, id := range slices.Sorted(maps.Keys(at)) {
		members = append(members, MemberInput{ID: id, Kind: KindPlayer, Position: at[id]})
	}

	enc, err := NewEncounter(&SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: FieldInput{
			Canvas:  openAir(),
			Regions: []RegionInput{rectRegion("pass-room", 0, 0, 10, 10)},
		},
		Members: members,
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)
	return enc
}

// heldBy reads one observer's holding on one subject straight off the store.
func heldBy(t *testing.T, enc *Encounter, observer, subject MemberID) (SightTestimony, bool, bool) {
	t.Helper()
	holdings, err := enc.intelLog.Held(observer)
	require.NoError(t, err)
	for _, h := range holdings {
		if h.Subject != subject {
			continue
		}
		testimony, ok := DecodeSightTestimony(h.Payload)
		require.True(t, ok, "the composition must decode its own testimony")
		return testimony, h.CurrentOn(perception.Sight), true
	}
	return SightTestimony{}, false, false
}

// TestAnUnplacedObserverIsNotInThePassAtAll is THE TRAP (rpg-toolkit#1691).
//
// The nested loop this replaced skipped an observer it could not place: no
// Surveil ran for them, so nothing of theirs faded. Under mind/perception an
// observer that is IN the pass and reaches nothing lands a complete empty
// percept and fades EVERYTHING (R4) — the right rule, and the exact opposite
// behaviour. R5 is the other half: an observer absent from the pass is not
// touched at all. So "unplaced" has to mean ABSENT from Pass.Observers, and
// this test fails the moment it stops meaning that.
func TestAnUnplacedObserverIsNotInThePassAtAll(t *testing.T) {
	const alice, bob MemberID = "alice", "bob"
	enc := perceptPassEncounter(t, map[MemberID]spatial.Position{
		alice: {X: 1, Y: 1},
		bob:   {X: 2, Y: 1},
	})

	before, current, held := heldBy(t, enc, alice, bob)
	require.True(t, held, "first light: she is watching him")
	require.True(t, current)

	// Off the canvas, still on the roster — the one state the filter is about.
	require.NoError(t, enc.canvas.RemoveEntity(string(alice)))

	deltas, err := enc.rebuildPercepts(enc.rosterIDs())
	require.NoError(t, err)

	require.Nil(t, deltas[alice], "an observer nobody could place was never asked")

	after, stillCurrent, stillHeld := heldBy(t, enc, alice, bob)
	require.True(t, stillHeld, "and so nothing of hers faded")
	require.True(t, stillCurrent, "her sight of him is exactly as she left it")
	require.Equal(t, before, after)

	// The other half of the same pass, for contrast: bob IS placed, so the
	// woman who left the canvas stops being a presence and fades for him.
	require.Equal(t, []MemberID{alice}, deltas[bob].Faded)
}

// TestAPlacedObserverReachingNobodyFadesEverything is the trap's other half
// (mind/perception R4): a percept that found nothing is still a complete
// percept, and skipping the call is how ghosts stay falsely current forever.
func TestAPlacedObserverReachingNobodyFadesEverything(t *testing.T) {
	const alice, bob MemberID = "alice", "bob"
	enc := perceptPassEncounter(t, map[MemberID]spatial.Position{
		alice: {X: 1, Y: 1},
		bob:   {X: 2, Y: 1},
	})

	was, current, held := heldBy(t, enc, alice, bob)
	require.True(t, held)
	require.True(t, current)

	// The torch gutters: everybody is still standing exactly where they were.
	enc.sight = seesNobody{}

	deltas, err := enc.rebuildPercepts(enc.rosterIDs())
	require.NoError(t, err)
	require.Equal(t, []MemberID{bob}, deltas[alice].Faded)

	ghost, stillCurrent, stillHeld := heldBy(t, enc, alice, bob)
	require.True(t, stillHeld, "the ghost survives — she remembers where he was")
	require.False(t, stillCurrent, "but she is not watching him any more")
	require.Equal(t, was, ghost, "and the memory says what it said when she took it")
}

// TestAMoveWithinSightIsChangedAndAStillPassIsOnlyRefreshed pins the field
// perception carries up from play/intel v0.4.0.
//
// Changed REFINES Refreshed rather than partitioning it, so the mover appears
// in both for the watchers. And an observer whose own subjects did not move
// reports a refresh with nothing changed — which is the pass that would read
// as a change if Changed were reconstructed from stamps instead of consumed.
func TestAMoveWithinSightIsChangedAndAStillPassIsOnlyRefreshed(t *testing.T) {
	const alice, bob, carol MemberID = "alice", "bob", "carol"
	enc := perceptPassEncounter(t, map[MemberID]spatial.Position{
		alice: {X: 1, Y: 1},
		bob:   {X: 2, Y: 1},
		carol: {X: 3, Y: 1},
	})

	out, err := enc.Step(&StepInput{Member: alice, To: spatial.Position{X: 1, Y: 2}})
	require.NoError(t, err)

	watcher := out.IntelDeltas[bob]
	require.NotNil(t, watcher)
	require.Contains(t, watcher.Changed, alice, "she moved where he could see her")
	require.Contains(t, watcher.Refreshed, alice, "Changed refines Refreshed, it does not carve it up")
	require.NotContains(t, watcher.Changed, carol, "carol did not move")

	mover := out.IntelDeltas[alice]
	require.NotNil(t, mover)
	require.Equal(t, []MemberID{bob, carol}, mover.Refreshed)
	require.Empty(t, mover.Changed, "neither of them moved, and neither of them swapped a hand")
}

// TestOnePassEncodesOnePayloadPerMember is the reason rpg-toolkit#1691
// exists. The nested loop it replaced encoded a subject's testimony once per
// observer who could see them — N² encodes of identical bytes for N members,
// because what a member looks like is a fact about THEM and not about who is
// looking.
func TestOnePassEncodesOnePayloadPerMember(t *testing.T) {
	roster := map[MemberID]spatial.Position{
		"alice": {X: 1, Y: 1},
		"bob":   {X: 2, Y: 1},
		"carol": {X: 3, Y: 1},
		"dave":  {X: 4, Y: 1},
	}
	enc := perceptPassEncounter(t, roster)

	// Counted around one pass only: first light ran its own.
	encodes := 0
	original := encodeSighting
	encodeSighting = func(testimony SightTestimony) ([]byte, error) {
		encodes++
		return original(testimony)
	}
	defer func() { encodeSighting = original }()

	_, err := enc.rebuildPercepts(enc.rosterIDs())
	require.NoError(t, err)

	require.Equal(t, len(roster), encodes,
		"one payload per member, once — not one per (observer, subject) pair")
}
