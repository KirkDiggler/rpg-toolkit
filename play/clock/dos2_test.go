// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock_test

// The DOS2 split-party scenario (design AC1): four players on the world
// tick; two trigger a turn-based bubble; the distant pair keeps accruing
// from their own moves; one wanders close and falls in at a
// rulebook-chosen position; the round wraps; the fight ends; everyone
// returns to the world clock. Asserts the full milestone transcript and
// final state.

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/stretchr/testify/require"
)

func TestDOS2SplitParty(t *testing.T) {
	const (
		alice  core.EntityID = "alice"
		bob    core.EntityID = "bob"
		carl   core.EntityID = "carl"
		dana   core.EntityID = "dana"
		goblin core.EntityID = "goblin"
	)
	world, err := clock.NewTick()
	require.NoError(t, err)
	var transcript []clock.Milestone
	record := func(ms []clock.Milestone) { transcript = append(transcript, ms...) }

	// Everyone free-roams: four players and a goblin on the world clock.
	for _, id := range []core.EntityID{alice, bob, carl, dana, goblin} {
		out, jerr := world.Join(&clock.JoinInput{ID: id})
		require.NoError(t, jerr)
		record(out.Milestones)
	}

	// Alice and Bob trigger a fight with the goblin: bubble forms.
	// (Trigger detection is the composition's business; here the rulebook
	// has rolled initiative.)
	bubble := &clock.Turn{}
	for _, id := range []core.EntityID{alice, goblin, bob} {
		out, lerr := world.Leave(&clock.LeaveInput{ID: id})
		require.NoError(t, lerr)
		record(out.Milestones)
	}
	out, err := bubble.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{alice, goblin, bob}})
	require.NoError(t, err)
	record(out.Milestones)

	// The distant pair keeps exploring: their own moves drive the world.
	adv, err := world.Advance(&clock.AdvanceInput{Driver: carl, Displacement: 3})
	require.NoError(t, err)
	record(adv.Milestones)
	// carl is both driver and member; the grant reaches every member (design: Tick)
	require.Equal(t, []core.EntityID{carl, dana}, adv.Ready, "the distant pair accrues while the fight runs")

	// A round of combat in the bubble.
	for _, actor := range []core.EntityID{alice, goblin} {
		end, eerr := bubble.End(&clock.EndInput{Actor: actor})
		require.NoError(t, eerr)
		record(end.Milestones)
	}

	// Carl wanders too close and falls in, slotted after the goblin.
	tr, err := clock.Transfer(&clock.TransferInput{From: world, To: bubble, ID: carl, Pos: 2})
	require.NoError(t, err)
	record(tr.Milestones)
	inBubble, err := bubble.Contains(&clock.ContainsInput{ID: carl})
	require.NoError(t, err)
	require.True(t, inBubble)

	// Bob closes the round: the bubble wraps into round 2 with carl in the
	// order — AC1's "rounds advance with End" exercised across a wrap.
	end, err := bubble.End(&clock.EndInput{Actor: bob})
	require.NoError(t, err)
	require.True(t, end.RoundWrapped)
	record(end.Milestones)

	// Fight ends: dissolve and re-home everyone to the world clock.
	dis, err := bubble.Dissolve(&clock.DissolveInput{})
	require.NoError(t, err)
	record(dis.Milestones)
	require.ElementsMatch(t, []core.EntityID{alice, goblin, bob, carl}, dis.Members)
	for _, id := range dis.Members {
		out, jerr := world.Join(&clock.JoinInput{ID: id})
		require.NoError(t, jerr)
		record(out.Milestones)
	}

	// Final state: everyone back on the world clock; bubble idle.
	members, err := world.Members()
	require.NoError(t, err)
	require.ElementsMatch(t, []core.EntityID{alice, bob, carl, dana, goblin}, members)
	_, err = bubble.Active()
	require.ErrorIs(t, err, clock.ErrIdle)

	// The transcript is API: assert it end to end.
	require.Equal(t, []clock.Milestone{
		{Kind: clock.MemberJoined, Subject: alice},
		{Kind: clock.MemberJoined, Subject: bob},
		{Kind: clock.MemberJoined, Subject: carl},
		{Kind: clock.MemberJoined, Subject: dana},
		{Kind: clock.MemberJoined, Subject: goblin},
		{Kind: clock.MemberLeft, Subject: alice},
		{Kind: clock.MemberLeft, Subject: goblin},
		{Kind: clock.MemberLeft, Subject: bob},
		{Kind: clock.RoundStarted, Round: 1},
		{Kind: clock.TurnStarted, Subject: alice, Round: 1},
		{Kind: clock.Ticked, Subject: carl},
		{Kind: clock.TurnEnded, Subject: alice, Round: 1},
		{Kind: clock.TurnStarted, Subject: goblin, Round: 1},
		{Kind: clock.TurnEnded, Subject: goblin, Round: 1},
		{Kind: clock.TurnStarted, Subject: bob, Round: 1},
		{Kind: clock.MemberLeft, Subject: carl},
		{Kind: clock.MemberJoined, Subject: carl, Round: 1},
		{Kind: clock.TurnEnded, Subject: bob, Round: 1},
		{Kind: clock.RoundStarted, Round: 2},
		{Kind: clock.TurnStarted, Subject: alice, Round: 2},
		{Kind: clock.Dissolved, Round: 2},
		// Dissolve returns members in bubble order [alice, goblin, carl, bob]
		// (carl inserted at Pos 2), and the rejoin loop follows it.
		{Kind: clock.MemberJoined, Subject: alice},
		{Kind: clock.MemberJoined, Subject: goblin},
		{Kind: clock.MemberJoined, Subject: carl},
		{Kind: clock.MemberJoined, Subject: bob},
	}, transcript)
}
