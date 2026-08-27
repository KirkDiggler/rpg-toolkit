// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// threePlayerMonsterBubble is alice, bob and goblin, in that initiative
// order (orderAsGiven preserves Members' own literal order), sharing one
// room so first light forms the bubble immediately.
func threePlayerMonsterBubble(t *testing.T) *Encounter {
	t.Helper()
	enc, err := NewEncounter(&SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field: FieldInput{
			Canvas:  openAir(),
			Regions: []RegionInput{rectRegion("room-1", 0, 0, 8, 8)},
		},
		Members: []MemberInput{
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 1}},
			{ID: "goblin", Kind: KindMonster, Position: spatial.Position{X: 3, Y: 1}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)
	require.Len(t, enc.bubbles, 1, "first light forms the bubble")
	return enc
}

// TestNestedDriveMonsterTurnsWhileOneIsRunningIsANoOp pins guard 2 of
// rpg-toolkit#1207's fix directly against driveMonsterTurns, independent of
// how a caller might reach a nested call in practice (Transfer's own
// path is guard 1's — pinned separately below).
//
// goblin is put into "active, and genuinely UNDRIVEN" by ending bob's own
// turn through play/clock's OWN End primitive rather than
// [Encounter.EndTurn] — the only way to reach that state at all, since
// every public path (EndTurn, form, Transfer) drives an unplayed leading
// member to completion before returning; there is no caller-observable
// moment where an unplayed member is active and waiting.
func TestNestedDriveMonsterTurnsWhileOneIsRunningIsANoOp(t *testing.T) {
	enc := threePlayerMonsterBubble(t)
	bubble := enc.bubbles[0]

	_, err := enc.EndTurn(&EndTurnInput{Member: "alice"})
	require.NoError(t, err)

	_, err = bubble.End(&clock.EndInput{Actor: "bob"})
	require.NoError(t, err)

	active, err := bubble.Active()
	require.NoError(t, err)
	require.Equal(t, core.EntityID("goblin"), active, "goblin is active and undriven — the state this guard is about")

	// Simulate an outer driveMonsterTurns call already owning this
	// encounter's own drive — exactly the state Strike -> Record ->
	// noticeDown -> Transfer -> driveIfStillRunning reaches mid-Strike.
	enc.driving = true
	wrapped, seq, err := enc.driveMonsterTurns(bubble)
	require.NoError(t, err)
	require.False(t, wrapped)
	require.Zero(t, seq, "a re-entrant call must not touch the story at all")

	stillActive, err := bubble.Active()
	require.NoError(t, err)
	require.Equal(t, core.EntityID("goblin"), stillActive,
		"goblin's own turn is untouched — the nested call was a no-op, not a second drive")
}

// TestTransferOfANonActiveMemberNeverDrives pins guard 1 of
// rpg-toolkit#1207's fix: Transfer's ClockWorld case must not call
// driveIfStillRunning for a departing member who was never the bubble's
// own active one — there is nothing stalled to rescue.
//
// goblin is put into "active, undriven" the same way the sibling test
// does. alice — NOT active, still sitting in the order — is then
// transferred out. Without this guard, the old unconditional call would
// still fire and drive goblin's own turn to completion (passDriver ->
// Pass), which is exactly the observable difference this test checks for.
func TestTransferOfANonActiveMemberNeverDrives(t *testing.T) {
	enc := threePlayerMonsterBubble(t)
	bubble := enc.bubbles[0]

	_, err := enc.EndTurn(&EndTurnInput{Member: "alice"})
	require.NoError(t, err)

	_, err = bubble.End(&clock.EndInput{Actor: "bob"})
	require.NoError(t, err)

	active, err := bubble.Active()
	require.NoError(t, err)
	require.Equal(t, core.EntityID("goblin"), active, "goblin is active and undriven — the state this guard is about")

	_, err = enc.Transfer(&TransferInput{Member: "alice", To: ClockWorld})
	require.NoError(t, err)

	stillActive, err := bubble.Active()
	require.NoError(t, err)
	require.Equal(t, core.EntityID("goblin"), stillActive,
		"transferring a non-active member must never drive — goblin's own turn is still exactly where it was")
}

// TestTransferOfTheActiveMemberStillRescuesAnUnplayedSuccessor pins that
// guard 1 does not OVER-restrict: rpg-toolkit#1162's own rescue — the
// departing member genuinely WAS active, and whoever inherits the slot has
// no player — must still fire.
func TestTransferOfTheActiveMemberStillRescuesAnUnplayedSuccessor(t *testing.T) {
	enc := threePlayerMonsterBubble(t)
	bubble := enc.bubbles[0]

	_, err := enc.EndTurn(&EndTurnInput{Member: "alice"})
	require.NoError(t, err)

	active, err := bubble.Active()
	require.NoError(t, err)
	require.Equal(t, core.EntityID("bob"), active)

	// bob himself is the active member. Transferring him out hands the
	// active slot to goblin, who has no player — exactly rpg-toolkit#1162's
	// own stalled-slot case.
	_, err = enc.Transfer(&TransferInput{Member: "bob", To: ClockWorld})
	require.NoError(t, err)

	// goblin's own turn (passDriver -> Pass) already resolved and handed
	// back to alice — the only player left in the now two-member order.
	stillActive, err := bubble.Active()
	require.NoError(t, err)
	require.Equal(t, core.EntityID("alice"), stillActive,
		"the rescue must still fire when the departing member genuinely was active")
}
