// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// spawnknows_test.go closes the last gap between the dungeon file and the
// live game (rpg-project#368 P1, found from the rpg-api side wiring wave 2).
//
// Everything else in this slice was reachable from an AUTHORED captain, and
// the holdings suite next door proves it that way. But a host that resolves
// monster content at runtime builds its world empty of members and brings
// every monster in through Spawn, so an authored `knows:` that could only be
// read at construction was a link the game never saw. Path 2 — kill the
// captain, loot the way in — could not happen live while every test passed.
//
// The scene here is the live shape: nobody is authored knowing anything, the
// captain ARRIVES knowing the vault door, and looting the body does exactly
// what looting an authored captain does.

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// spawnKnowingCaptain spawns a skeleton beside alice carrying the author's
// link to the veil, then puts it on the floor by zeroing its stored sheet —
// the same way the holdings suite makes a body, because it is the same
// mechanism: the composition is TOLD who is down.
func (s *HoldingsSuite) spawnKnowingCaptain(knows []string) {
	s.T().Helper()
	_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "latecomer", Ref: refs.Monsters.Skeleton().String(),
		Position: absolute(spatial.Position{X: 0, Y: 1}), Knows: knows,
	})
	s.Require().NoError(err)

	stored := s.sessions.byID["sess"]
	for i := range stored.NPCs {
		if stored.NPCs[i].ID == "latecomer" {
			stored.NPCs[i].HitPoints = 0
			return
		}
	}
	s.Require().Fail("the spawn recorded no sheet to put on the floor")
}

// TestASpawnedMonsterCarriesTheAuthorsKnowledgeLinks is the scene: spawn with
// Knows, loot, and the looter alone learns the way in.
func (s *HoldingsSuite) TestASpawnedMonsterCarriesTheAuthorsKnowledgeLinks() {
	ctx := context.Background()
	s.start(false) // NOBODY was authored knowing anything
	s.spawnKnowingCaptain([]string{"veil"})
	s.stream.published = nil

	_, err := s.mgr.Loot(ctx, &session.LootInput{
		Session: "sess", Member: "alice", Target: "latecomer", Range: 2})
	s.Require().NoError(err)

	s.Run("the looter alone is told about the door", func() {
		s.Equal([]session.EventKind{session.EventLooted, session.EventDoorRevealed},
			s.kinds("alice"))
		body, ok := s.bodyOf("alice", session.EventDoorRevealed).(session.DoorRevealedBody)
		s.Require().True(ok)
		s.Equal("veil", body.Door, "the way in came off the body that ARRIVED carrying it")
	})

	s.Run("the bystander hears the beat and learns nothing", func() {
		s.Equal([]session.EventKind{session.EventLooted}, s.kinds("bob"))
		blind, err := s.mgr.Doors(ctx, &session.DoorsInput{Session: "sess", Member: "bob"})
		s.Require().NoError(err)
		s.Empty(blind.Doors)
	})

	s.Run("it is the same reveal an authored captain gives", func() {
		authored := s.bodyOf("alice", session.EventDoorRevealed)
		s.start(true)
		_, err := s.mgr.Loot(ctx, &session.LootInput{
			Session: "sess", Member: "alice", Target: "captain"})
		s.Require().NoError(err)
		s.Equal(authored, s.bodyOf("alice", session.EventDoorRevealed),
			"arriving and being authored are two ways into the world, not two mechanisms")
	})
}

// TestASpawnedMonsterKnowingNothingRevealsNothing is the negative that makes
// the scene above a claim about Knows rather than about spawning.
//
// Same verb, same body, same loot — the ONE difference is the field — and the
// bystander's bytes are identical either way, which is design P3 asked of the
// live path.
func (s *HoldingsSuite) TestASpawnedMonsterKnowingNothingRevealsNothing() {
	ctx := context.Background()

	loot := func(knows []string) ([]session.EventKind, string) {
		s.start(false)
		s.spawnKnowingCaptain(knows)
		s.stream.published = nil
		_, err := s.mgr.Loot(ctx, &session.LootInput{
			Session: "sess", Member: "alice", Target: "latecomer", Range: 2})
		s.Require().NoError(err)
		return s.kinds("bob"), s.storyBytes("bob")
	}

	richKinds, richStory := loot([]string{"veil"})
	poorKinds, poorStory := loot(nil)

	s.Equal([]session.EventKind{session.EventLooted}, poorKinds)
	s.Equal(poorKinds, richKinds)
	s.Equal(poorStory, richStory,
		"a monster that arrived knowing nothing and one that arrived knowing the run's "+
			"only secret are indistinguishable to everybody but the looter")
}

// TestASpawnCannotKnowADoorThatIsNotThere is the fail-closed half at the
// seam: an unauthored link is refused by name rather than arriving ignorant,
// and the refusal crosses as this package's own sentinel.
//
// This is the failure a host hits by forwarding the AUTHOR's raw door id
// instead of the compiled `<key>/<id>` dungeonspec mints, which is the one
// mistake worth making loud.
func (s *HoldingsSuite) TestASpawnCannotKnowADoorThatIsNotThere() {
	s.start(false)

	_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "latecomer", Ref: refs.Monsters.Skeleton().String(),
		Position: absolute(spatial.Position{X: 0, Y: 1}), Knows: []string{"no-such-door"},
	})
	s.Require().ErrorIs(err, session.ErrNoConnection,
		"a door this field does not declare, named at the seam that declares doors")

	var placed bool
	for _, npc := range s.sessions.byID["sess"].NPCs {
		if npc.ID == "latecomer" {
			placed = true
		}
	}
	s.False(placed, "the refusal left nothing behind")
	s.Empty(s.stream.published, "and told nobody")
}
