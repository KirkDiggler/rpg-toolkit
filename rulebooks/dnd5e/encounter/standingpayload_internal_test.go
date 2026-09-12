// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// standingpayload_internal_test.go pins rpg-toolkit#1697: standing is now
// snapshotted into the sight payload at assembly time, from one pass-scoped
// reading, exactly like position and equipment already were. Down went from
// "declared, encoded, decoded, never written" to "written by every payload
// this composition assembles from here on" — see [Encounter.rebuildPercepts].

// toggleableStanding is the internal package's controllable participation
// capability. down is read fresh on every Assess call, so a test can flip one
// member's standing between two calls to rebuildPercepts and see exactly what
// that pass wrote — without dragging in Remove/initiative machinery that has
// nothing to do with sight.
type toggleableStanding struct {
	down map[MemberID]bool
}

func (*toggleableStanding) Standing([]MemberID) ([]MemberID, error) {
	return nil, nil
}

func (s *toggleableStanding) Assess(members []MemberID) (*ParticipationAssessment, error) {
	assessment := &ParticipationAssessment{}
	for _, id := range members {
		member := MemberParticipation{Member: id, Contact: true, Conscious: true, Turn: TurnParticipationWait}
		if s.down[id] {
			member.Down = true
			member.Conscious = false
		}
		assessment.Members = append(assessment.Members, member)
	}
	return assessment, nil
}

// standingPassEncounter is [perceptPassEncounter] with a controllable
// Standing capability instead of the fixed everyoneStanding{} — players, so
// first light never forms a fight and the pass is the only thing under test.
func standingPassEncounter(t *testing.T, at map[MemberID]spatial.Position, standing *toggleableStanding) *Encounter {
	t.Helper()

	members := make([]MemberInput, 0, len(at))
	for _, id := range slices.Sorted(maps.Keys(at)) {
		members = append(members, MemberInput{ID: id, Kind: KindPlayer, Position: at[id]})
	}

	enc, err := NewEncounter(&SetupInput{
		Sight:     everyoneSeesTheWholeMap{},
		Equipment: noHandsAreObserved{}, Standing: standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: FieldInput{
			Canvas:  openAir(),
			Regions: []RegionInput{rectRegion("standing-room", 0, 0, 10, 10)},
		},
		Members: members,
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)
	return enc
}

// holdingOn reads one observer's raw holding on one subject, Confirmed and
// Current included — [heldBy] throws those away and this needs them.
func holdingOn(t *testing.T, enc *Encounter, observer, subject MemberID) (perception.Holding, bool) {
	t.Helper()
	holdings, err := enc.intelLog.Held(observer)
	require.NoError(t, err)
	for _, h := range holdings {
		if h.Subject != subject {
			continue
		}
		return h, true
	}
	return perception.Holding{}, false
}

// TestStandingSubjectDecodesDownFalse is table row 1: an observer who sees a
// standing subject holds testimony whose Down is non-nil and false — observed,
// not merely defaulted.
func TestStandingSubjectDecodesDownFalse(t *testing.T) {
	const alice, bob MemberID = "alice", "bob"
	enc := standingPassEncounter(t, map[MemberID]spatial.Position{
		alice: {X: 1, Y: 1},
		bob:   {X: 2, Y: 1},
	}, &toggleableStanding{down: map[MemberID]bool{}})

	testimony, current, held := heldBy(t, enc, alice, bob)
	require.True(t, held)
	require.True(t, current)
	require.NotNil(t, testimony.Down, "standing was observed at assembly time")
	require.False(t, *testimony.Down)
}

// TestDownedSubjectDecodesDownTrue is table row 2: an observer who sees a
// downed subject holds testimony whose Down is non-nil and true.
func TestDownedSubjectDecodesDownTrue(t *testing.T) {
	const alice, bob MemberID = "alice", "bob"
	enc := standingPassEncounter(t, map[MemberID]spatial.Position{
		alice: {X: 1, Y: 1},
		bob:   {X: 2, Y: 1},
	}, &toggleableStanding{down: map[MemberID]bool{bob: true}})

	testimony, current, held := heldBy(t, enc, alice, bob)
	require.True(t, held)
	require.True(t, current)
	require.NotNil(t, testimony.Down)
	require.True(t, *testimony.Down)
}

// TestASubjectGoingDownInViewIsChanged is table row 3: standing is payload
// content now, so a subject falling while an observer still watches is a
// change that observer's next percept reports.
func TestASubjectGoingDownInViewIsChanged(t *testing.T) {
	const alice, bob MemberID = "alice", "bob"
	standing := &toggleableStanding{down: map[MemberID]bool{}}
	enc := standingPassEncounter(t, map[MemberID]spatial.Position{
		alice: {X: 1, Y: 1},
		bob:   {X: 2, Y: 1},
	}, standing)

	before, _, held := heldBy(t, enc, alice, bob)
	require.True(t, held)
	require.False(t, *before.Down, "first light: he is on his feet")

	standing.down[bob] = true
	deltas, err := enc.rebuildPercepts(enc.rosterIDs())
	require.NoError(t, err)
	require.NotNil(t, deltas[alice])
	require.Contains(t, deltas[alice].Changed, bob, "she watched him go down")

	after, current, held := heldBy(t, enc, alice, bob)
	require.True(t, held)
	require.True(t, current)
	require.True(t, *after.Down)
}

// TestUnchangedStandingIsRefreshedNotChanged is table row 5: a subject whose
// standing is the same on two consecutive passes is refreshed, not changed —
// a rebuilt payload that happens to be identical to the one before it is not
// news, exactly as an unmoved position is not (see
// TestAMoveWithinSightIsChangedAndAStillPassIsOnlyRefreshed).
func TestUnchangedStandingIsRefreshedNotChanged(t *testing.T) {
	const alice, bob MemberID = "alice", "bob"
	enc := standingPassEncounter(t, map[MemberID]spatial.Position{
		alice: {X: 1, Y: 1},
		bob:   {X: 2, Y: 1},
	}, &toggleableStanding{down: map[MemberID]bool{}})

	// First light already ran one pass over standing that never changes;
	// this is the SECOND consecutive pass over the same fact.
	deltas, err := enc.rebuildPercepts(enc.rosterIDs())
	require.NoError(t, err)
	require.NotNil(t, deltas[alice])
	require.Contains(t, deltas[alice].Refreshed, bob)
	require.NotContains(t, deltas[alice].Changed, bob, "standing did not change; a second look at the same fact is not news")
}

// TestGhostDoesNotDiscloseAStandingChangeItNeverWitnessed is table row 4 and
// the entire reason rpg-toolkit#1697 exists: an observer loses sight of a
// subject who is later reported down, but the observer's memory was taken
// before that fall and must go on saying what it said when it was taken. If
// this test ever starts passing for the wrong reason — Down simply absent
// from the ghost's payload rather than present and still false — that is the
// nil/false collapse the pointer exists to prevent, and this test would stop
// meaning what its name says.
func TestGhostDoesNotDiscloseAStandingChangeItNeverWitnessed(t *testing.T) {
	const alice, bob MemberID = "alice", "bob"
	standing := &toggleableStanding{down: map[MemberID]bool{}}
	enc := standingPassEncounter(t, map[MemberID]spatial.Position{
		alice: {X: 1, Y: 1},
		bob:   {X: 2, Y: 1},
	}, standing)

	firstLight, ok := holdingOn(t, enc, alice, bob)
	require.True(t, ok)
	require.True(t, firstLight.Current, "first light: she is watching him stand")
	firstTestimony, decoded := DecodeSightTestimony(firstLight.Payload)
	require.True(t, decoded)
	require.NotNil(t, firstTestimony.Down)
	require.False(t, *firstTestimony.Down)

	// The torch gutters: she can no longer see him, or anyone. She fades to
	// a ghost holding exactly what she last saw, while he is still standing.
	enc.sight = seesNobody{}
	deltas, err := enc.rebuildPercepts(enc.rosterIDs())
	require.NoError(t, err)
	require.Contains(t, deltas[alice].Faded, bob, "she stopped watching him")

	ghost, ok := holdingOn(t, enc, alice, bob)
	require.True(t, ok)
	require.False(t, ghost.Current, "she is not watching him any more")

	// NOW he goes down — a fact she has no way to witness, because she still
	// cannot see him.
	standing.down[bob] = true
	deltas, err = enc.rebuildPercepts(enc.rosterIDs())
	require.NoError(t, err)
	require.NotNil(t, deltas[alice], "she is still placed and in the pass, so this pass answers for her too")
	require.NotContains(t, deltas[alice].Changed, bob, "she never saw it happen")
	require.NotContains(t, deltas[alice].Refreshed, bob, "she never re-perceived him either — she isn't watching anyone")

	ghostAfter, ok := holdingOn(t, enc, alice, bob)
	require.True(t, ok)
	require.False(t, ghostAfter.Current)
	require.Equal(t, ghost.Payload, ghostAfter.Payload,
		"the ghost's payload is unchanged: she still remembers him standing")
	require.Equal(t, ghost.Confirmed, ghostAfter.Confirmed, "confirmed has not moved")

	staleTestimony, decoded := DecodeSightTestimony(ghostAfter.Payload)
	require.True(t, decoded)
	require.NotNil(t, staleTestimony.Down)
	require.False(t, *staleTestimony.Down, "her memory of him is exactly as wrong as it is honest: he WAS standing")
}
