// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// dwell_test.go pins the dwell's reaction-window rule (rpg-project#624,
// design R18): a walk to a waypoint with a wait that stops for a reaction
// window and resumes still arrives with the whole wait ahead of it — the
// resumed turn is the same turn, and spends none of it.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// waypointWalkDriver walks one member two cells east on its first turn, to a
// waypoint whose pause states a wait of two and a south facing, and passes
// after.
type waypointWalkDriver struct {
	member encounter.MemberID
	used   bool
}

func (d *waypointWalkDriver) Act(view encounter.MonsterView) (encounter.Decision, error) {
	if view.Self != d.member || d.used {
		return encounter.Decision{Intent: encounter.Pass{}}, nil
	}
	d.used = true
	first := spatial.Position{X: view.Position.X + 1, Y: view.Position.Y}
	waypoint := spatial.Position{X: view.Position.X + 2, Y: view.Position.Y}
	south := 180.0

	return encounter.Decision{Intent: encounter.Move{
		Path:  []spatial.Position{first, waypoint},
		Pause: &encounter.WaypointPause{Cell: waypoint, Facing: &south, Wait: 2},
	}}, nil
}

// TestAReactionWindowSpendsNoDwell: the goblin's walk to the waypoint stops
// for alice's window on its first step; resumed, it ends on the waypoint
// facing the stated south with both of the wait's rounds still ahead.
func TestAReactionWindowSpendsNoDwell(t *testing.T) {
	members := []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
		{ID: goblin, Kind: encounter.KindMonster, Position: spatial.Position{X: 3, Y: 2}},
	}
	mover := &pausingMover{pauseAt: map[int]bool{0: true}}
	enc := facingFight(t, members, &waypointWalkDriver{member: goblin}, &scriptedStriker{kind: encounter.OutcomeMissed}, mover)
	start := memberOf(t, enc, goblin).Position

	_, err := enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	require.NoError(t, err)
	require.True(t, enc.Paused(), "precondition: the walk stopped for a window")

	_, err = enc.Resume(context.Background())
	require.NoError(t, err)
	require.False(t, enc.Paused())

	waypoint := spatial.Position{X: start.X + 2, Y: start.Y}
	require.Equal(t, waypoint, memberOf(t, enc, goblin).Position, "the resumed walk ended on the waypoint")
	require.InDelta(t, 180, memberOf(t, enc, goblin).Facing, 1e-9, "standing there it faces the stated south")
	var dwell *encounter.DwellData
	for _, m := range enc.ToData().Members {
		if m.ID == goblin {
			dwell = m.Dwell
		}
	}
	require.NotNil(t, dwell, "the walk that ended on the waypoint began a dwell")
	require.Equal(t, 2, dwell.Rounds, "the window and the resumed turn spent none of the wait")
}
