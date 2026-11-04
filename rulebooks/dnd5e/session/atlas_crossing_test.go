// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// atlas_crossing_test.go is the crossing mark on the session's map
// (rpg-project#852, H6): a footprint door's crossings are doorways of the
// map, each marked Crossing, so a planner reads the castle gate's opening as
// it reads an edge door's; an edge door's doorways are not marked.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

func TestCastleTheGatesCrossingsAreDoorwaysOfTheMap(t *testing.T) {
	c := openTheCastleForTwo(t, 18)
	atlas, err := c.mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)

	var gate []session.AtlasDoorway
	for _, d := range atlas.Doorways {
		if d.Door == castleGateDoor {
			gate = append(gate, d)
		}
	}
	require.ElementsMatch(t, []session.AtlasDoorway{
		{Door: castleGateDoor, From: besideTheCastleGate, To: pastTheCastleGate, Crossing: true},
		{Door: castleGateDoor, From: pastTheCastleGate, To: beneathTheCastleGate, Crossing: true},
	}, gate, "the gate's two crossings are doorways of the map, each marked")
}

func (s *AtlasMapSuite) TestAnEdgeDoorsDoorwayIsNoCrossing() {
	doorways := s.atlas().Doorways
	s.Require().NotEmpty(doorways, "precondition: the map has an edge door")
	for _, d := range doorways {
		s.False(d.Crossing, "an edge door's doorway %s %v-%v is not a footprint door's crossing", d.Door, d.From, d.To)
	}
}
