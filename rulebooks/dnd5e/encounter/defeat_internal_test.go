// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// oneDown reports a single member down, chosen by the test and changeable
// mid-scene — the internal twin of downList, and for the same reason: a pull
// can only be tested by changing the answer.
type oneDown struct {
	who MemberID
}

func (d *oneDown) Standing(members []MemberID) ([]MemberID, error) {
	for _, id := range members {
		if id == d.who {
			return []MemberID{id}, nil
		}
	}

	return nil, nil
}

// TestADecidedFightRefusesToCountAGhost is the net under fightIsDecided, and it
// is deliberately WHITE-BOX for the same reason ClockOf's on-no-clock check is:
// the load seam refuses a bubble holding a non-member and no verb can create
// one, so the only way to probe the net is to fabricate the exact defect it
// exists to catch.
//
// The defect fabricated here is a verb that dropped somebody from the ROSTER and
// forgot the clock — the mirror of the one ClockOf nets, and the shape a future
// regression in Exit would take (it leaves the clock first today, on purpose).
//
// What makes it worth a check rather than a skip is the SHAPE of the wrong
// answer. A ghost in the order is a member this pass cannot classify, so
// skipping it counts one fewer standing monster — which makes the fight look
// MORE decided than it is. Silently, plausibly, and permanently: the ending
// would be written into the story and saved. Raised in Copilot review on PR
// #1080 and verified before it was believed.
func TestADecidedFightRefusesToCountAGhost(t *testing.T) {
	rulebook := &oneDown{}
	enc, err := NewEncounter(&SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: rulebook, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{},
		Field: FieldInput{Canvas: openAir(), Regions: []RegionInput{rectRegion("crypt", 0, 0, 12, 12)}},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 0, Y: 2}},
			{ID: "goblin", Kind: KindMonster, Position: spatial.Position{X: 0, Y: 10}},
			{ID: "wolf", Kind: KindMonster, Position: spatial.Position{X: 2, Y: 10}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)

	require.Len(t, enc.bubbles, 1, "first light started the fight")
	order, err := enc.bubbles[0].Order()
	require.NoError(t, err)
	require.Equal(t, []MemberID{"alice", "goblin", "wolf"}, order)

	// The goblin falls, which is what sends the pass looking at this fight at
	// all — and the wolf is what should keep it a fight.
	rulebook.who = "goblin"

	// The bug, fabricated: the wolf leaves the ROSTER and stays in the order.
	delete(enc.members, "wolf")

	_, err = enc.Pump(&PumpInput{})
	require.Error(t, err, "a fight cannot be decided by counting who is left out of the roster")
	require.ErrorIs(t, err, ErrInvalidData)
	require.ErrorContains(t, err, "not a member")

	// And it refused rather than ended: the caller's obligation is to drop the
	// encounter unsaved, which only means anything if the fight is still there
	// to drop.
	require.Len(t, enc.bubbles, 1, "the ghost did not get to end the fight")
}
