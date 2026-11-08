// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// TestAMoveAttachesEachMonsterSheetOnceHoweverOftenStandingIsAsked is
// rpg-project#892 on the castle: alice walks up to the captain, and the world
// asks who is standing many times along the way, but each monster sheet is
// attached at most once for the whole verb. It fails if the per-assessment
// attach returns: then the monster sheets attached are the garrison times the
// number of assessments.
func TestAMoveAttachesEachMonsterSheetOnceHoweverOftenStandingIsAsked(t *testing.T) {
	ctx := context.Background()
	c := openTheCastleForTwo(t, 18)

	enc, err := c.mgr.LoadWorldForTest(ctx, "sess")
	require.NoError(t, err)
	roster, err := enc.Members()
	require.NoError(t, err)
	monsters := 0
	for _, m := range roster {
		if m.Kind == encounter.MemberKind(session.KindMonster) {
			monsters++
		}
	}
	require.Positive(t, monsters, "precondition: the castle holds a garrison")
	route, err := enc.Route(encounter.RouteInput{
		Mover: "alice", Policy: encounter.MoveToward, Anchor: standingAt(enc, "captain"), Budget: 200,
	})
	require.NoError(t, err)

	counted := session.CountParticipationAttachesForTest(t)
	_, err = c.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: route.Path})
	require.NoError(t, err)

	assessments, attached := counted()
	t.Logf("one Move: %d assessments, %d monster sheets attached, %d monsters", assessments, attached, monsters)
	require.Greater(t, assessments, 1, "precondition: standing is asked more than once in the verb")
	assert.LessOrEqual(t, attached, monsters, "each monster sheet is attached at most once in a verb that writes none")
}
