// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/stretchr/testify/require"
)

func TestTransferTickToTurn(t *testing.T) {
	const carl core.EntityID = "carl"
	tick, err := clock.NewTick()
	require.NoError(t, err)
	_, err = tick.Join(&clock.JoinInput{ID: carl})
	require.NoError(t, err)
	turn := &clock.Turn{}
	_, err = turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
	require.NoError(t, err)

	out, err := clock.Transfer(&clock.TransferInput{From: tick, To: turn, ID: carl, Pos: 1})
	require.NoError(t, err)
	// milestones reported leave-then-join (design: Transfer), regardless of execution order
	require.Equal(t, []clock.Milestone{
		{Kind: clock.MemberLeft, Subject: carl},
		{Kind: clock.MemberJoined, Subject: carl, Round: 1},
	}, out.Milestones)
	inTurn, err := turn.Contains(&clock.ContainsInput{ID: carl})
	require.NoError(t, err)
	require.True(t, inTurn)
	inTick, err := tick.Contains(&clock.ContainsInput{ID: carl})
	require.NoError(t, err)
	require.False(t, inTick, "one clock per entity")
}

func TestTransferFailureLeavesBothUnchanged(t *testing.T) {
	const carl core.EntityID = "carl"
	tick, err := clock.NewTick()
	require.NoError(t, err)
	_, err = tick.Join(&clock.JoinInput{ID: carl})
	require.NoError(t, err)
	_, err = tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 3})
	require.NoError(t, err)
	idleTurn := &clock.Turn{} // join will refuse: ErrIdle

	tickBefore, turnBefore := tick.ToData(), idleTurn.ToData()
	_, err = clock.Transfer(&clock.TransferInput{From: tick, To: idleTurn, ID: carl, Pos: 0})
	require.ErrorIs(t, err, clock.ErrIdle, "underlying sentinel propagates")
	require.Equal(t, tickBefore, tick.ToData(), "From unchanged (R6)")
	require.Equal(t, turnBefore, idleTurn.ToData(), "To unchanged (R6)")
}

func TestTransferAbsentMemberCompensates(t *testing.T) {
	const ghost core.EntityID = "ghost"
	tick, err := clock.NewTick()
	require.NoError(t, err)
	turn := &clock.Turn{}
	_, err = turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
	require.NoError(t, err)

	turnBefore := turn.ToData()
	_, err = clock.Transfer(&clock.TransferInput{From: tick, To: turn, ID: ghost, Pos: 0})
	require.ErrorIs(t, err, clock.ErrNotMember)
	require.Equal(t, turnBefore, turn.ToData(), "join was compensated (R6)")
}
