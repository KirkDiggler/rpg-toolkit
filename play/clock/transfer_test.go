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
	out, err := clock.Transfer(&clock.TransferInput{From: tick, To: idleTurn, ID: carl, Pos: 0})
	require.Nil(t, out, "failed transfer emits nothing")
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

	tickBefore := tick.ToData()
	turnBefore := turn.ToData()
	out, err := clock.Transfer(&clock.TransferInput{From: tick, To: turn, ID: ghost, Pos: 0})
	require.Nil(t, out, "failed transfer emits nothing")
	require.ErrorIs(t, err, clock.ErrNotMember)
	require.Equal(t, tickBefore, tick.ToData(), "From unchanged (R6)")
	require.Equal(t, turnBefore, turn.ToData(), "join was compensated (R6)")
}

func TestTransferSameClockRefused(t *testing.T) {
	const ghost core.EntityID = "ghost"
	turn := &clock.Turn{}
	_, err := turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
	require.NoError(t, err)
	before := turn.ToData()
	// present entity
	_, err = clock.Transfer(&clock.TransferInput{From: turn, To: turn, ID: "a", Pos: 0})
	require.ErrorIs(t, err, clock.ErrSameClock)
	require.Equal(t, before, turn.ToData())
	// absent entity — the case that silently "succeeded" pre-guard
	_, err = clock.Transfer(&clock.TransferInput{From: turn, To: turn, ID: ghost, Pos: 0})
	require.ErrorIs(t, err, clock.ErrSameClock)
	require.Equal(t, before, turn.ToData())
}

// TestTransferTurnToTick pins the consumer-facing direction (fleeing a
// bubble back to the world clock — rpg-api's monster free-roam path):
// Pos is ignored by the unordered world clock, the new member starts at
// budget zero, and milestones report leave-then-join.
func TestTransferTurnToTick(t *testing.T) {
	const dana core.EntityID = "dana"
	turn := &clock.Turn{}
	_, err := turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", dana}})
	require.NoError(t, err)
	world, err := clock.NewTick()
	require.NoError(t, err)

	out, err := clock.Transfer(&clock.TransferInput{From: turn, To: world, ID: dana, Pos: 99})
	require.NoError(t, err)
	require.Equal(t, []clock.Milestone{
		{Kind: clock.MemberLeft, Subject: dana, Round: 1},
		{Kind: clock.MemberJoined, Subject: dana},
	}, out.Milestones)
	inWorld, err := world.Contains(&clock.ContainsInput{ID: dana})
	require.NoError(t, err)
	require.True(t, inWorld)
	inTurn, err := turn.Contains(&clock.ContainsInput{ID: dana})
	require.NoError(t, err)
	require.False(t, inTurn, "one clock per entity")
	b, err := world.Budget(&clock.BudgetInput{ID: dana})
	require.NoError(t, err)
	require.Zero(t, b, "fresh world member starts at zero budget")

	// error branch of the Tick-side Joiner adapter: duplicate propagates
	_, err = world.JoinMember(&clock.JoinMemberInput{ID: dana, Pos: 0})
	require.ErrorIs(t, err, clock.ErrDuplicateMember)
}

// TestTransferTurnToTickAbsentCompensates covers Turn.LeaveMember's error
// branch and the Tick-side compensation (R6 both directions).
func TestTransferTurnToTickAbsentCompensates(t *testing.T) {
	const ghost core.EntityID = "ghost"
	turn := &clock.Turn{}
	_, err := turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
	require.NoError(t, err)
	world, err := clock.NewTick()
	require.NoError(t, err)
	turnBefore, worldBefore := turn.ToData(), world.ToData()

	out, err := clock.Transfer(&clock.TransferInput{From: turn, To: world, ID: ghost, Pos: 0})
	require.ErrorIs(t, err, clock.ErrNotMember)
	require.Nil(t, out, "failed transfer emits nothing")
	require.Equal(t, turnBefore, turn.ToData(), "From unchanged (R6)")
	require.Equal(t, worldBefore, world.ToData(), "To unchanged — Tick-side compensation (R6)")
}
