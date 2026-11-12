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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
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

	require.ErrorIs(t, settleTheCase(t, enc, at, true), ErrNoOption,
		"a second settle is refused: turned away, bob is no longer offered the case")
	require.False(t, enc.recognizes(postedGuard, "bob", admittedTopic), "and admits nobody")
}

func TestAnAbandonedHitAdmitsTheStraggler(t *testing.T) {
	enc, at := walkedAway(t)
	require.NoError(t, settleTheCase(t, enc, at, true))
	require.True(t, enc.recognizes(postedGuard, "bob", admittedTopic), "judged as spoken, the hit admits bob")
	require.True(t, enc.recognizes("gate-2", "bob", admittedTopic))
	require.False(t, enc.recognizes(postedGuard, "bob", turnedAwayTopic))
	require.ErrorIs(t, settleTheCase(t, enc, at, false), ErrNoOption,
		"once: admitted, bob is no longer offered the case")
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
	require.ErrorIs(t, settleTheCase(t, settled, at, false), ErrNoOption, "and a settle after a load is refused")
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

// openCase is the straggler's case with no requirement but the admitted
// gate: still offered to bob once it has landed, so a second settle meets
// the moved revision.
func openCase() Option {
	o := oneTryLine()
	o.Requires = []Trigger{atTheAdmittedGate}
	return o
}

func TestASecondSettleOfAKeptHitIsStale(t *testing.T) {
	g := oneTryGuard(t, postedGuard, holdNorthWest)
	for i, o := range g.Options {
		if o.ID == "straggler" {
			g.Options[i] = openCase()
		}
	}
	in := recognisingSetup(t, true, g, MemberInput{ID: "bob", Kind: KindPlayer, Position: farDownTheYard})
	in.Sight = shortSighted{member: postedGuard, cells: 2}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	admit(t, enc, postedGuard)
	placeAt(t, enc, "alice", farDownTheYard)
	placeAt(t, enc, "bob", holdSouthEast)
	at := reserveTheCase(t, enc)
	placeAt(t, enc, "bob", farDownTheYard)
	require.NoError(t, enc.sweepHolds())
	require.True(t, enc.interactions[hailGate].reservation.Abandoned, "precondition: abandoned")

	require.NoError(t, settleTheCase(t, enc, at, true))
	require.True(t, enc.recognizes(postedGuard, "bob", admittedTopic))
	require.True(t, offers(t, enc, "bob", postedGuard, "straggler"), "precondition: the line is still offered")
	require.ErrorIs(t, settleTheCase(t, enc, at, true), ErrStaleOption,
		"a second settle at the reserved revision is a reply from an old menu")
}

// fewSighted gives each named member that many cells of sight, and the
// whole map to the rest.
type fewSighted map[MemberID]int

// Sight answers each named member's few cells.
func (f fewSighted) Sight(members []MemberID) (map[MemberID]int, error) {
	out, _ := everyoneSeesTheWholeMap{}.Sight(members)
	for id, cells := range f {
		if _, ok := out[id]; ok {
			out[id] = cells
		}
	}
	return out, nil
}

// TestAnAbandonedSettleTellsWhatEachWitnessSawThen: alice witnessed bob's
// case and could not see carol; by the settle, bob is out of alice's sight
// and carol beside her. The recognition of bob's party is told to alice as
// she saw it when the words were said: bob, and not carol.
func TestAnAbandonedSettleTellsWhatEachWitnessSawThen(t *testing.T) {
	g := oneTryGuard(t, postedGuard, holdNorthWest)
	for i, o := range g.Options {
		if o.ID == "straggler" {
			party := oneTryLine()
			party.Grants[0].Consequence.Recognize.About = RecognitionPartyInSight
			g.Options[i] = party
		}
	}
	in := recognisingSetup(t, true, g,
		MemberInput{ID: "bob", Kind: KindPlayer, Position: farDownTheYard},
		MemberInput{ID: "carol", Kind: KindPlayer, Position: spatial.Position{X: 9, Y: 4}},
	)
	in.Sight = fewSighted{postedGuard: 2, "alice": 1}
	enc, err := NewEncounter(in)
	require.NoError(t, err)
	admit(t, enc, postedGuard)
	placeAt(t, enc, "alice", holdBeside)
	placeAt(t, enc, "bob", holdSouthEast)
	placeAt(t, enc, "carol", holdSouthWest)
	at := reserveTheCase(t, enc)
	scene := enc.interactions[hailGate].reservation.Scene
	require.Contains(t, scene.Seen, MemberID("carol"), "precondition: the guard saw carol")
	require.Contains(t, scene.Perceived["alice"], MemberID("bob"), "precondition: alice saw bob")
	require.NotContains(t, scene.Perceived["alice"], MemberID("carol"), "precondition: and not carol")

	placeAt(t, enc, "bob", farDownTheYard)
	require.NoError(t, enc.sweepHolds())
	require.True(t, enc.interactions[hailGate].reservation.Abandoned, "precondition: abandoned")
	placeAt(t, enc, "carol", spatial.Position{X: 6, Y: 1})

	before := len(recognizedBeats(t, enc))
	require.NoError(t, settleTheCase(t, enc, at, true))
	require.True(t, enc.recognizes(postedGuard, "carol", admittedTopic), "the guard recognises the party he saw then")
	entries, err := enc.Story(&StoryInput{Audience: "alice"})
	require.NoError(t, err)
	var told [][]any
	for _, e := range entries {
		if e.Tags["tag"] != BeatRecognized {
			continue
		}
		var b map[string]any
		require.NoError(t, json.Unmarshal(e.Payload, &b))
		if b["holder"] == string(postedGuard) && b["option"] == "straggler" {
			told = append(told, b["subjects"].([]any))
		}
	}
	require.Equal(t, [][]any{{"alice", "bob"}}, told,
		"alice is told herself and bob, whom she saw then, and not carol, whom she sees only now")
	require.Greater(t, len(recognizedBeats(t, enc)), before)
}
