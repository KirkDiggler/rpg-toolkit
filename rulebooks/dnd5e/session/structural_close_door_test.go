// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// Keep the structural integration assertion here; the standalone CloseDoor
// adapter and its released-provider tests are owned by the merged main version.
func (s *CloseDoorSuite) TestTheStructuralLayoutIsUntouched() {
	ctx := context.Background()
	s.startWith(structuralRoomWorld())
	_, err := s.mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "alice", Door: "gate"})
	s.Require().NoError(err)
	before, err := s.mgr.Knowledge(ctx, &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"})
	s.Require().NoError(err)
	s.Require().NotEmpty(before.Atlas.StructuralWalls)

	_, err = s.mgr.CloseDoor(ctx, &session.CloseDoorInput{Session: "sess", Member: "alice", Door: "gate"})
	s.Require().NoError(err)
	after, err := s.mgr.Knowledge(ctx, &session.KnowledgeInput{Session: "sess", Member: "alice", Player: "player-alice"})
	s.Require().NoError(err)
	s.Equal(before.Atlas.StructuralWalls, after.Atlas.StructuralWalls)
	s.Equal(before.Atlas.StructuralDoors, after.Atlas.StructuralDoors)
}
