// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_hail_test.go is THE GUARD HAILS on the castle (rpg-project#923,
// Gate S3 of #769): walking from the party's start toward the held gate, the
// first gate speaker who sees the hero come within the front gate's approach
// addresses her, the walk stops there, and he says his hail line. The
// entrance's progress is untouched.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// The hail lines, as the castle file writes them.
var castleHails = map[encounter.MemberID]string{
	kingGate1:   "Easy there, friend. What brings you to the gate?",
	kingGate2:   "Hold up. State your business at the gate.",
	kingCaptain: "You there. Nobody passes this gate without my say. Speak.",
}

func TestMadKingTheGateHailsTheApproach(t *testing.T) {
	t.Parallel()
	s := openTheScript(t, partyStart(t), nil)
	gateState := s.enc.ToData().Interactions[0].Blob
	gateRevision := s.enc.ToData().Interactions[0].Revision

	route, err := s.enc.Route(encounter.RouteInput{Mover: hero, Policy: encounter.MoveToward, Anchor: besideTheCrossing, Budget: 200})
	require.NoError(t, err)
	require.Empty(t, route.StoppedBy, "precondition: a way to the gate")

	var hailed *encounter.Addressed
	taken := 0
	for i, cell := range route.Path {
		out, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell, EndWalk: i == len(route.Path)-1, Reach: speakReach})
		require.NoError(t, err)
		taken++
		if len(out.Addressed) > 0 {
			hailed = &out.Addressed[0]
			break // the host drops the walk's remaining legs
		}
	}
	require.NotNil(t, hailed, "somebody at the gate hails the hero on the way in")
	require.Less(t, taken, len(route.Path), "the hail stops the walk short of the crossing")
	require.Equal(t, encounter.InteractionID("front-gate"), hailed.Interaction)
	line, carrier := castleHails[hailed.Guard]
	require.True(t, carrier, "the hailer is a gate speaker, not %s", hailed.Guard)
	require.LessOrEqual(t, s.enc.Distance(s.at(hero), s.at(hailed.Guard)), 2.0, "inside the approach")

	require.Contains(t, s.saidBy(hero, hailed.Guard), line, "he says his hail")

	standing, err := s.enc.AddressFor(&encounter.AddressForInput{Member: hero, Reach: speakReach})
	require.NoError(t, err)
	require.Equal(t, hailed.Guard, standing.Address.Guard, "the address stands for the session to project")
	require.Equal(t, gateState, s.enc.ToData().Interactions[0].Blob, "a hail is not progress")
	require.Equal(t, gateRevision, s.enc.ToData().Interactions[0].Revision, "nor does it move the conversation's revision (rpg-project#924)")
	require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, hero), "a hail is not a fight")
}
