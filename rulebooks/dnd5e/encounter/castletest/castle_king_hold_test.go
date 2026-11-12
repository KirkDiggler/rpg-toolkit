// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_hold_test.go is A POST HOLDS A PASSAGE on the castle
// (rpg-project#852, S1b of the guard holds the gate, R5): the gate is a
// two-cell opening with a double door, and gate-1's `posted` and `alert`
// states hold the gate door's opening from either gatehouse-side cell of
// it. With the door open and gate-1 on his post, no step through either
// cell passes, and the stop names him; drawn off, cowed or downed, the
// opening passes; and a second holder keeps it held when one is downed.
//
// The dice are not walked: the stand-aside is won by the route host.
//
// S1c (rpg-project#857, R6): the post holds `against: entering` with the
// troupe's fact its pass. It bars the way from the gatehouse into the
// courtyard, lets the party walk back out, and lets the troupe through
// while he stays posted. Both ways, the default, is a test post here.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The opening's cells as offset seats, for [castleHost.at] and
// [castleHost.starts]: beneath the gate, beside it along the gate wall,
// and the courtyard cell past the door.
var (
	beneathTheGateSeat    = spatial.Position{X: 0, Y: 7}
	besideTheCrossingSeat = spatial.Position{X: -1, Y: 7}
	pastTheGateSeat       = spatial.Position{X: 0, Y: 6}
)

// openTheHeldGate opens the castle with the gate door open and the sneak's
// companion outside beside the stranger, whose word is a voice the post
// hears.
func openTheHeldGate(t *testing.T, host castleHost) *kingScene {
	t.Helper()
	host.doors = map[encounter.DoorID]encounter.DoorState{kingGateDoor: encounter.DoorIsOpen()}
	host.players = []encounter.MemberInput{{ID: sneakCompanion, Kind: encounter.KindPlayer, Position: besideTheStranger}}

	return openTheCastleAs(t, host)
}

// barred steps the hero and returns the stop's public reason, requiring a
// held step: refused as a body is, never as the door, moving nobody.
func (s *kingScene) barred(to spatial.Position) string {
	s.t.Helper()
	from := s.at(hero)
	out, err := s.enc.Step(&encounter.StepInput{Member: hero, To: to})
	require.NoError(s.t, err, "a bump is answered, not refused (rpg-project#842)")
	require.NotNil(s.t, out.Stopped, "the held opening stops the hero")
	require.Equal(s.t, from, s.at(hero), "and the refused step moved nobody")

	return out.Stopped.Reason
}

// crosses requires the hero's step to land.
func (s *kingScene) crosses(to spatial.Position, msg string) {
	s.t.Helper()
	_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: to})
	require.NoError(s.t, err, msg)
	require.Equal(s.t, to, s.at(hero), msg)
}

func TestMadKingThePostHoldsTheGatesPassage(t *testing.T) {
	t.Parallel()
	held := string(kingGate1) + " bars the way"

	t.Run("posted, neither cell of the open gate passes, and the stop names him", func(t *testing.T) {
		t.Parallel()
		s := openTheHeldGate(t, castleHost{at: besideTheCrossingSeat, soleGateHolder: true})
		require.Equal(t, beneathTheGate, s.at(kingGate1), "precondition: gate-1 is on his post beneath the gate")
		require.Equal(t, held, s.barred(pastTheGate), "beside him, through the opening's other cell")

		other := openTheHeldGate(t, castleHost{
			at: beneathTheGateSeat, soleGateHolder: true, starts: map[encounter.MemberID]spatial.Position{kingGate1: besideTheCrossingSeat},
		})
		require.Equal(t, besideTheCrossing, other.at(kingGate1), "precondition: gate-1 stands on his other post cell")
		require.Equal(t, held, other.barred(pastTheGate), "and from his other post cell, through the first")
	})

	t.Run("alert on his post, still held", func(t *testing.T) {
		t.Parallel()
		s := openTheHeldGate(t, castleHost{at: besideTheCrossingSeat, soleGateHolder: true})
		s.storedIn(kingGate1, "alert")
		require.Equal(t, beneathTheGate, s.at(kingGate1), "precondition: alert on his post")
		require.Equal(t, held, s.barred(pastTheGate), "alert on his post, he holds the gate")
	})

	t.Run("talk outside does not draw him off his post", func(t *testing.T) {
		t.Parallel()
		// GUARDS ANSWER SHOUTS, NOT TALK (rpg-project#933, the operator's
		// ruling on #930): the companion's spoken word with the stranger is
		// a voice the post hears and does not answer.
		s := openTheHeldGate(t, castleHost{at: besideTheCrossingSeat, soleGateHolder: true})
		require.Equal(t, held, s.barred(pastTheGate), "precondition: held")
		s.voiceOutside()
		// The same pacing the shout below draws him off within: talk never
		// does.
		offPost := func() bool {
			at := s.at(kingGate1)
			return at != beneathTheGate && at != besideTheCrossing
		}
		require.False(t, s.pace(besideTheCrossing, westDoorPost, 24, offPost), "talk does not draw him off his post")
		s.onHisPost(kingGate1, "he stands his post")
	})

	t.Run("drawn off by a shout, it opens", func(t *testing.T) {
		t.Parallel()
		s := openTheHeldGate(t, castleHost{at: besideTheCrossingSeat, soleGateHolder: true})
		require.Equal(t, held, s.barred(pastTheGate), "precondition: held")
		s.shoutOutside()
		// He answers the voice on the world's next round and walks back to
		// his post after it, so the hero paces until he is off both post
		// cells and the hero stands beside the crossing again.
		offPost := func() bool {
			at := s.at(kingGate1)
			return at != beneathTheGate && at != besideTheCrossing && s.at(hero) == besideTheCrossing
		}
		require.True(t, s.pace(besideTheCrossing, westDoorPost, 24, offPost), "the voice draws him off his post")
		s.crosses(pastTheGate, "drawn off, he holds nothing: the hero crosses")
	})

	t.Run("cowed, the opening passes", func(t *testing.T) {
		t.Parallel()
		s := openTheHeldGate(t, castleHost{at: gatehouseSeat})
		s.walkTo(s.at(kingCaptain))
		s.said(kingCaptain, "stand-aside", "intimidation", true)
		require.Equal(t, "cowed", machineState(t, s.enc, kingGate1), "precondition: he stood aside")
		s.stepTo(besideTheCrossing)
		s.crosses(pastTheGate, "cowed, he holds nothing")
	})

	t.Run("turned, the opening passes", func(t *testing.T) {
		t.Parallel()
		// The state alone, stored: still neutral to the party, so what
		// opens the gate is that `turned` holds nothing, not the stance.
		s := openTheHeldGate(t, castleHost{at: besideTheCrossingSeat, soleGateHolder: true})
		s.storedIn(kingGate1, "turned")
		require.Equal(t, beneathTheGate, s.at(kingGate1), "precondition: still standing beneath the gate")
		require.False(t, s.hostile(hero, kingGate1), "precondition: and still neutral")
		s.crosses(pastTheGate, "turned, he holds nothing")
	})

	t.Run("downed, the opening passes", func(t *testing.T) {
		t.Parallel()
		s := openTheHeldGate(t, castleHost{at: besideTheCrossingSeat, soleGateHolder: true})
		require.Equal(t, held, s.barred(pastTheGate), "precondition: held")
		s.standing.down[kingGate1] = true
		s.crosses(pastTheGate, "downed, he holds nothing")
	})

	t.Run("with gate-2 posted on the same door, downing one does not open it", func(t *testing.T) {
		t.Parallel()
		// A test post set both ways: the hero comes from the courtyard,
		// the way the castle's entering post lets through.
		s := openTheHeldGate(t, castleHost{
			at:          pastTheGateSeat,
			postedLike:  []encounter.MemberID{kingGate2},
			starts:      map[encounter.MemberID]spatial.Position{kingGate2: besideTheCrossingSeat},
			gateAgainst: encounter.PostBothWays,
			// gate-2 seated like gate-1, the captain's post set aside: the
			// group rule on two holders of one kind.
			soleGateHolder: true,
		})
		require.Equal(t, besideTheCrossing, s.at(kingGate2), "precondition: gate-2 stands on the post's other cell")

		s.standing.down[kingGate1] = true
		require.Equal(t, string(kingGate2)+" bars the way", s.barred(beneathTheGate),
			"gate-1 down, gate-2 still holds the opening, across his cell")

		s.standing.down[kingGate2] = true
		s.crosses(beneathTheGate, "both down, the opening passes")
	})
}

// storedIn saves the run, stands a creature in a state of its recipe in the
// record, and loads it back: an alert guard without the sound that would
// also walk him (the post's voice row draws him in the round he hears it).
func (s *kingScene) storedIn(id encounter.MemberID, state string) {
	s.t.Helper()
	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(s.t, err)
	var data encounter.EncounterData
	require.NoError(s.t, json.Unmarshal(blob, &data))
	found := false
	for i := range data.Members {
		if data.Members[i].ID == id {
			b := data.Members[i].Behaviors
			require.NotNil(s.t, b)
			for j := range b.Progress.Instances {
				if b.Progress.Instances[j].ID == b.BodyInstance {
					b.Progress.Instances[j].Blob.State, found = state, true
				}
			}
		}
	}
	require.True(s.t, found, "%s is in the run", id)
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Capabilities: s.host.capabilities(s.strikes, s.standing),
	})
	require.NoError(s.t, err, "the record loads with %s %s", id, state)
	s.enc = enc
	require.Equal(s.t, state, machineState(s.t, s.enc, id))
}

// TestMadKingTheHeldGateDoorRefusesTheParty is R7 (rpg-project#835): with
// gate-1 posted, the hero beside the shut gate door cannot open it, and is
// told who bars it; cowed, he opens it.
func TestMadKingTheHeldGateDoorRefusesTheParty(t *testing.T) {
	t.Parallel()
	s := openTheCastleAs(t, castleHost{at: besideTheCrossingSeat, soleGateHolder: true})
	require.Equal(t, beneathTheGate, s.at(kingGate1), "precondition: gate-1 is on his post")

	out, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: kingGateDoor, Actor: hero})
	require.NoError(t, err, "a held door's verb is answered, not refused (rpg-project#842)")
	require.NotNil(t, out.Refused, "the held gate door refuses the hero")
	require.Equal(t, string(kingGate1)+" bars the door", out.Refused.Reason, "and names the guard he sees")
	require.Equal(t, encounter.DoorClosed, out.State, "and stays shut")

	s.storedIn(kingGate1, "cowed")
	s.open(kingGateDoor)
}

// TestMadKingTheTroupePassesThePostedGuard (rpg-project#857, R6): the
// troupe line won, the hero crosses the gate inward while gate-1 stays
// posted on his cell. The captain's word makes the gate guards recognise the
// hero (rpg-project#928); it does not move the guard.
func TestMadKingTheTroupePassesThePostedGuard(t *testing.T) {
	t.Parallel()
	s := openTheCastle(t, partyStart(t))
	s.heardTheStranger(true)
	s.walkTo(s.at(kingCaptain))
	s.said(kingCaptain, "troupe", "performance", true)
	require.True(t, s.recognizes(kingGate1, hero), "precondition: gate-1 heard the troupe let through, and recognises the hero")

	s.stepTo(besideTheCrossing)
	s.backOnHisPost()
	require.Equal(t, beneathTheGate, s.at(kingGate1), "precondition: gate-1 stands on his post beneath the gate")
	s.onHisPost(kingGate1, "precondition: and is posted")
	held, _, err := s.enc.DoorHeldFor(hero, kingGateDoor)
	require.NoError(t, err)
	require.False(t, held, "the pass works the gate door")
	s.open(kingGateDoor)
	s.crosses(pastTheGate, "the troupe crosses inward past the posted guard")

	s.onHisPost(kingGate1, "at the crossing he is still posted")
	require.Equal(t, beneathTheGate, s.at(kingGate1), "on his cell")
	seen, err := s.enc.ObservedHolds(&encounter.ViewInput{Member: hero})
	require.NoError(t, err)
	require.NotContains(t, seen, kingGate1, "the hero, recognised, is told no hold")
}

// TestMadKingTheGateLetsThePartyOut (rpg-project#857, R6): from inside, the
// party walks out past the posted guard unchallenged, because the castle's
// post checks arrivals. The same gate with a test post set both ways bars
// the same party on the way out.
func TestMadKingTheGateLetsThePartyOut(t *testing.T) {
	t.Parallel()

	t.Run("against entering, as the file sets it, the party walks out", func(t *testing.T) {
		t.Parallel()
		s := openTheHeldGate(t, castleHost{at: pastTheGateSeat, soleGateHolder: true})
		require.Equal(t, beneathTheGate, s.at(kingGate1), "precondition: gate-1 is on his post")
		s.crosses(besideTheCrossing, "out of the courtyard, past the posted guard")
		s.onHisPost(kingGate1, "still posted")
		require.Equal(t, string(kingGate1)+" bars the way", s.barred(pastTheGate), "and the way back in is barred")
	})

	t.Run("a test post set both ways bars the same party on the way out", func(t *testing.T) {
		t.Parallel()
		s := openTheHeldGate(t, castleHost{at: pastTheGateSeat, soleGateHolder: true, gateAgainst: encounter.PostBothWays})
		require.Equal(t, string(kingGate1)+" bars the way", s.barred(besideTheCrossing))
	})

	t.Run("the arrivals stay barred at the shut gate door", func(t *testing.T) {
		t.Parallel()
		s := openTheCastleAs(t, castleHost{at: besideTheCrossingSeat, soleGateHolder: true})
		out, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: kingGateDoor, Actor: hero})
		require.NoError(t, err)
		require.NotNil(t, out.Refused, "from the gatehouse, the door is held against the hero")
		inside := openTheCastleAs(t, castleHost{at: pastTheGateSeat, soleGateHolder: true})
		inside.open(kingGateDoor)
	})
}

// onHisPost requires a gate guard in a state whose post holds the gate:
// posted (the recipe's initial state, which the record writes as empty) or
// alert, the state a voice he hears turns him to. Cowed and turned hold
// nothing.
func (s *kingScene) onHisPost(id encounter.MemberID, msg string) {
	s.t.Helper()
	state := machineState(s.t, s.enc, id)
	require.Contains(s.t, []string{"posted", "alert"}, state, msg)
}

// gateStandsAside stands every gate guard cowed in the record — gate-1, gate-2
// and the captain, each of whom holds the gate (rpg-project#928) — so a route
// that is about what lies past the gate opens it without the stand-aside's
// dice.
func (s *kingScene) gateStandsAside() {
	s.t.Helper()
	for _, id := range []encounter.MemberID{kingGate1, kingGate2, kingCaptain} {
		s.storedIn(id, "cowed")
	}
}
