// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// roll_kept_internal_test.go is a roll is kept (rpg-project#938, ruled
// 2026-11-12: "a roll is a roll that is kept"). A straggler whose checked
// claim waits on their held benefit walks out of the guard's sight: the
// reservation is abandoned, not released, and the host's settle — the roll
// resumed declined — is judged in the scene the words were spoken in, so a
// miss turns them away and a hit admits them, once.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// farDownTheYard is out of the posted guard's two cells of sight and of
// speaking reach.
var farDownTheYard = holdOffPost

// reserveTheCase has bob reserve the straggler's case at the posted guard,
// at the revision it is offered at now.
func reserveTheCase(t *testing.T, enc *Encounter) uint64 {
	t.Helper()
	at := enc.interactions[hailGate].revision
	_, err := enc.Reserve(&ReserveInput{Actor: "bob", Target: postedGuard, OptionID: "straggler", Revision: at, Range: 3})
	require.NoError(t, err)
	return at
}

// settleTheCase is the host's settle of bob's case at the reserved
// revision, as rolled: beaten or missed.
func settleTheCase(t *testing.T, enc *Encounter, at uint64, beaten bool) error {
	t.Helper()
	_, err := enc.Speak(context.Background(), &SpeakInput{
		Actor: "bob", Target: postedGuard, OptionID: "straggler", Roller: wakeRollsOne{}, Range: 3, Revision: at,
		Skill: "persuasion", Beaten: beaten, DC: 13, Total: map[bool]int{true: 18, false: 4}[beaten],
	})
	return err
}

// walkedAway is the one-try hall with bob's case reserved and bob gone down
// the yard, the sweep run as a participation pass runs it.
func walkedAway(t *testing.T) (*Encounter, uint64) {
	t.Helper()
	enc := oneTryHall(t)
	at := reserveTheCase(t, enc)
	placeAt(t, enc, "bob", farDownTheYard)
	require.NoError(t, enc.sweepHolds())
	return enc, at
}

func TestAHolderWhoWalksAwayLeavesTheHoldAbandoned(t *testing.T) {
	enc, at := walkedAway(t)
	hold := enc.interactions[hailGate].reservation
	require.NotNil(t, hold, "the roll is kept: the hold is not released")
	require.True(t, hold.Abandoned)
	require.Equal(t, at+1, enc.interactions[hailGate].revision, "abandoning moves no revision")
	require.Contains(t, hold.Scene.Seen, MemberID("bob"), "the guard saw bob when the words were said")

	require.NoError(t, enc.sweepHolds())
	require.True(t, enc.interactions[hailGate].reservation.Abandoned, "a second sweep leaves it abandoned")
}

func TestAnAbandonedMissTurnsTheStragglerAway(t *testing.T) {
	enc, at := walkedAway(t)
	require.NoError(t, settleTheCase(t, enc, at, false), "the settle from down the yard is admitted")
	require.True(t, enc.recognizes(postedGuard, "bob", turnedAwayTopic), "judged as spoken, the miss turns bob away")
	require.True(t, enc.recognizes("gate-2", "bob", turnedAwayTopic), "and the other guard heard it")
	require.False(t, enc.recognizes(postedGuard, "bob", admittedTopic))
	require.Nil(t, enc.interactions[hailGate].reservation, "the reservation clears")
	require.False(t, offers(t, enc, "bob", postedGuard, "straggler"), "one try: the case is gone")

	require.Error(t, settleTheCase(t, enc, at, true), "a second settle is refused")
	require.False(t, enc.recognizes(postedGuard, "bob", admittedTopic), "and admits nobody")
}

func TestAnAbandonedHitAdmitsTheStraggler(t *testing.T) {
	enc, at := walkedAway(t)
	require.NoError(t, settleTheCase(t, enc, at, true))
	require.True(t, enc.recognizes(postedGuard, "bob", admittedTopic), "judged as spoken, the hit admits bob")
	require.True(t, enc.recognizes("gate-2", "bob", admittedTopic))
	require.False(t, enc.recognizes(postedGuard, "bob", turnedAwayTopic))
	require.Error(t, settleTheCase(t, enc, at, false), "once")
	require.False(t, enc.recognizes(postedGuard, "bob", turnedAwayTopic), "a second settle turns nobody away")
}

func TestAnAbandonedHoldSurvivesTheRecord(t *testing.T) {
	enc, at := walkedAway(t)
	capabilities := postedSetup(shortSighted{member: postedGuard, cells: 2}).Capabilities
	loaded := reload(t, enc, capabilities)
	hold := loaded.interactions[hailGate].reservation
	require.NotNil(t, hold)
	require.True(t, hold.Abandoned, "abandoned comes back abandoned")
	require.Equal(t, enc.interactions[hailGate].reservation.Scene, hold.Scene, "with its scene")

	require.NoError(t, settleTheCase(t, loaded, at, false))
	settled := reload(t, loaded, capabilities)
	require.Nil(t, settled.interactions[hailGate].reservation)
	require.True(t, settled.recognizes(postedGuard, "bob", turnedAwayTopic), "the verdict is in the record")
	require.Error(t, settleTheCase(t, settled, at, false), "and a settle after a load is refused")
}

func TestANormalSettleStillAsksTheDoors(t *testing.T) {
	enc := oneTryHall(t)
	at := reserveTheCase(t, enc)
	// Moved with no participation pass, so nothing has swept the hold.
	placeAt(t, enc, "bob", farDownTheYard)
	require.False(t, enc.interactions[hailGate].reservation.Abandoned, "precondition: not abandoned")
	err := settleTheCase(t, enc, at, false)
	require.ErrorIs(t, err, ErrUnwitnessed, "an unabandoned settle out of sight is refused as before")
	require.NotNil(t, enc.interactions[hailGate].reservation, "and the hold stands")
	require.False(t, enc.recognizes(postedGuard, "bob", turnedAwayTopic))
}

func TestAnAbandonedHoldWhoseTargetLeavesIsReleased(t *testing.T) {
	enc, _ := walkedAway(t)
	_, err := enc.Exit(&ExitInput{Member: postedGuard})
	require.NoError(t, err)
	require.Nil(t, enc.interactions[hailGate].reservation, "nobody is left to answer: released")
	require.False(t, enc.recognizes("gate-2", "bob", turnedAwayTopic), "with no verdict")
}

func TestAHoldersExitStillDropsTheHold(t *testing.T) {
	enc, _ := walkedAway(t)
	_, err := enc.Exit(&ExitInput{Member: "bob"})
	require.NoError(t, err)
	require.Nil(t, enc.interactions[hailGate].reservation, "a holder who leaves the run holds nothing")
}

func TestTheHostAbandonsAReservation(t *testing.T) {
	enc := oneTryHall(t)
	at := reserveTheCase(t, enc)
	out, err := enc.AbandonReservation(&AbandonReservationInput{Member: "bob"})
	require.NoError(t, err)
	require.Equal(t, []InteractionRevision{{Interaction: hailGate, Revision: at + 1}}, out.Abandoned)
	require.Empty(t, out.Released)
	require.True(t, enc.interactions[hailGate].reservation.Abandoned)

	_, err = enc.AbandonReservation(&AbandonReservationInput{Member: "carol"})
	require.ErrorIs(t, err, ErrNoReservation, "a member holding none is refused")

	t.Run("a reservation saved before scenes were kept is released", func(t *testing.T) {
		enc := oneTryHall(t)
		reserveTheCase(t, enc)
		enc.interactions[hailGate].reservation.Scene = nil
		out, err := enc.AbandonReservation(&AbandonReservationInput{Member: "bob"})
		require.NoError(t, err)
		require.Len(t, out.Released, 1)
		require.Nil(t, enc.interactions[hailGate].reservation)
	})
}

func TestASweepReleasesAHoldWithNoScene(t *testing.T) {
	enc := oneTryHall(t)
	reserveTheCase(t, enc)
	enc.interactions[hailGate].reservation.Scene = nil
	placeAt(t, enc, "bob", farDownTheYard)
	require.NoError(t, enc.sweepHolds())
	require.Nil(t, enc.interactions[hailGate].reservation, "without the scene it cannot be judged as spoken: released")
}

func TestAnAbandonedHoldWithNoSceneIsRefusedOnLoad(t *testing.T) {
	enc, _ := walkedAway(t)
	data := enc.ToData()
	for i := range data.Interactions {
		if data.Interactions[i].Reservation != nil {
			data.Interactions[i].Reservation.Scene = nil
		}
	}
	_, err := LoadEncounter(&LoadEncounterInput{Data: data, Capabilities: postedSetup(shortSighted{member: postedGuard, cells: 2}).Capabilities})
	require.ErrorIs(t, err, ErrInvalidData)
}
