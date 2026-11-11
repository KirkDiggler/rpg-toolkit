// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_recognition_test.go is ADMISSION IS RECOGNITION on the castle
// (rpg-project#928, ruled on #925 2026-11-11). The guest claim won, the
// front gate enters `admitted`, and its outcome makes the captain recognise
// the party he sees; gate-1 and gate-2, who witnessed it, recognise them
// having heard. Every gate guard holds the gate and lets through whom he
// recognises, so the recognised walk in and nobody else does: not a member
// on the road out of his sight, and not a listener who witnessed the answer
// from beyond his sight. The captain has a line for whom he recognises.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// listener is a party member who stands in earshot of the gate claim, sees
// it said, and stands beyond the captain's short sight.
const listener = encounter.MemberID("listener")

// theWayIn is the captain's line for whom he recognises.
const theWayIn = "the-way-in"

// gateGuards is every creature that holds the castle's gate.
var gateGuards = []encounter.MemberID{kingGate1, kingGate2, kingCaptain}

// recognitionBeats is every recognition on the record.
func (s *kingScene) recognitionBeats() int {
	s.t.Helper()
	n := 0
	for _, e := range s.enc.ToData().Log.Entries {
		if e.Tags["tag"] == encounter.BeatRecognized {
			n++
		}
	}

	return n
}

// offeredBy reports whether a creature offers a speaker a line now.
func (s *kingScene) offeredBy(speaker, target encounter.MemberID, option string) bool {
	s.t.Helper()
	options, err := s.enc.OptionsFor(speaker, target)
	require.NoError(s.t, err)
	for _, o := range options {
		if o.ID == option {
			return true
		}
	}

	return false
}

// heldAgainst is the gate guards a viewer is told hold the gate against
// them: [encounter.Encounter.ObservedHolds], which leaves out a post whose
// pass lets the viewer through.
func (s *kingScene) heldAgainst(viewer encounter.MemberID) []encounter.MemberID {
	s.t.Helper()
	held, err := s.enc.ObservedHolds(&encounter.ViewInput{Member: viewer})
	require.NoError(s.t, err)
	var out []encounter.MemberID
	for _, id := range gateGuards {
		if _, ok := held[id]; ok {
			out = append(out, id)
		}
	}

	return out
}

// roundTrip saves the run and loads it back, whatever the reload flag says.
func (s *kingScene) roundTrip() {
	s.t.Helper()
	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(s.t, err)
	var data encounter.EncounterData
	require.NoError(s.t, json.Unmarshal(blob, &data))
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Capabilities: s.host.capabilities(s.strikes, s.standing),
	})
	require.NoError(s.t, err, "the castle loads back with its recognitions")
	s.enc = enc
}

func TestMadKingTheGuardRecognisesWhomHeAdmitted(t *testing.T) {
	t.Parallel()
	start := partyStart(t)
	s := openTheCastleAs(t, castleHost{
		at: gatehouseSeat,
		players: []encounter.MemberInput{
			{ID: bard, Kind: encounter.KindPlayer, Position: spatial.Position{X: start.X + 2, Y: start.Y}},
		},
		// The captain sees two cells: the hero who speaks to him, and not
		// the listener three cells off who watches it said.
		sightOf: map[encounter.MemberID]int{kingCaptain: 10},
	})
	s.walkTo(s.at(kingCaptain))
	s.joinListener()

	require.False(t, s.offeredBy(hero, kingCaptain, theWayIn), "before the claim, the captain has no such line for the hero")
	require.NotEmpty(t, s.heldAgainst(hero), "and the gate is held against the hero")

	s.said(kingCaptain, "guest", "persuasion", true)
	require.Equal(t, "admitted", s.entrance(), "precondition: the guest claim admitted")
	require.NotEmpty(t, s.beatsOf(listener, encounter.BeatSpoke), "precondition: the listener witnessed the claim")

	t.Run("the captain recognises whom he sees, and gate-1 and gate-2 heard it", func(t *testing.T) {
		for _, id := range gateGuards {
			require.True(t, s.recognizes(id, hero), "%s recognises the hero", id)
			require.False(t, s.recognizes(id, bard), "%s does not recognise the bard, on the road out of his sight", id)
			require.False(t, s.recognizes(id, listener), "%s does not recognise the listener, who heard but was not seen", id)
		}
	})

	t.Run("the gate is held against whom no guard recognises, and open to whom all do", func(t *testing.T) {
		require.Empty(t, s.heldAgainst(hero), "every gate guard lets the hero through")
		require.Contains(t, s.heldAgainst(listener), kingCaptain, "the captain holds the gate against the listener")
	})

	t.Run("the captain's line for whom he recognises", func(t *testing.T) {
		require.True(t, s.offeredBy(hero, kingCaptain, theWayIn), "offered to the hero he recognises")
		require.False(t, s.offeredBy(listener, kingCaptain, theWayIn), "absent for the listener he does not")
	})

	t.Run("a save and load keeps the recognition, and grants nothing again", func(t *testing.T) {
		granted := s.recognitionBeats()
		require.Positive(t, granted)
		s.roundTrip()
		require.True(t, s.recognizes(kingCaptain, hero), "the captain still recognises the hero")
		require.Equal(t, granted, s.recognitionBeats(), "and no recognition was written again")
		_, err := s.enc.Speak(context.Background(), &encounter.SpeakInput{
			Range: speakReach, Actor: hero, Target: kingCaptain, OptionID: theWayIn, Roller: rollsOne{},
		})
		require.NoError(t, err, "the captain answers the line")
		require.Equal(t, granted, s.recognitionBeats(), "nor by speaking on")
	})

	t.Run("the hero walks in past the posted gate guard", func(t *testing.T) {
		s.stepTo(besideTheCrossing)
		s.onHisPost(kingGate1, "precondition: gate-1 is posted")
		s.open(kingGateDoor)
		s.crosses(pastTheGate, "recognised, the hero crosses")
	})
}

// joinListener seats the listener three cells from the hero, beyond the
// captain's sight and in plain view of the hero, with the hero's sight.
func (s *kingScene) joinListener() {
	s.t.Helper()
	captain := s.at(kingCaptain)
	for _, cell := range []spatial.Position{
		{X: captain.X - 1, Y: captain.Y + 4}, {X: captain.X, Y: captain.Y + 4}, {X: captain.X - 2, Y: captain.Y + 4},
		{X: captain.X + 1, Y: captain.Y + 3}, {X: captain.X - 3, Y: captain.Y + 3},
	} {
		if s.enc.Distance(cell, captain) <= 2 {
			continue
		}
		_, err := s.enc.Join(&encounter.JoinInput{Member: listener, Kind: encounter.KindPlayer, Cell: cell})
		if err == nil {
			return
		}
	}
	s.t.Fatal("no floor for the listener beyond the captain's sight")
}

func TestMadKingEveryGateGuardHoldsTheGate(t *testing.T) {
	t.Parallel()
	s := openTheHeldGate(t, castleHost{at: besideTheCrossingSeat})
	for _, id := range gateGuards {
		_, ok := s.heldDoorOf(id)
		require.True(t, ok, "precondition: %s holds the gate", id)
	}

	s.barred(pastTheGate)
	s.standing.down[kingGate1] = true
	s.barred(pastTheGate)
	s.standing.down[kingCaptain] = true
	s.barred(pastTheGate)
	s.standing.down[kingGate2] = true
	s.crosses(pastTheGate, "all three down, the gate is open")
}

// heldDoorOf is the door a gate guard holds against the hero, as the hero is
// told it.
func (s *kingScene) heldDoorOf(id encounter.MemberID) (encounter.DoorID, bool) {
	s.t.Helper()
	held, err := s.enc.ObservedHolds(&encounter.ViewInput{Member: hero})
	require.NoError(s.t, err)
	h, ok := held[id]

	return h.Door, ok
}
