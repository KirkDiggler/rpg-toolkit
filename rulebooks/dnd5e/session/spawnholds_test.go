// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// spawnholds_test.go closes the last gap between the dungeon file and the
// live game (rpg-project#368 P1, found from the rpg-api side wiring wave 2; carried to
// the intel record by rpg-project#372).
//
// Everything else in this slice was reachable from an AUTHORED captain, and
// the holdings suite next door proves it that way. But a host that resolves
// monster content at runtime builds its world empty of members and brings
// every monster in through Spawn, so an authored `holds:` that could only be
// read at construction was a record the game never saw. Path 2 — kill the
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

// spawnHoldingCaptain spawns a skeleton beside alice carrying the author's
// record for the veil, then puts it on the floor by zeroing its stored sheet —
// the same way the holdings suite makes a body, because it is the same
// mechanism: the composition is TOLD who is down.
func (s *HoldingsSuite) spawnHoldingCaptain(holds []string) {
	s.T().Helper()
	_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "latecomer", Ref: refs.Monsters.Skeleton().String(),
		Position: absolute(spatial.Position{X: 0, Y: 1}), Holds: holds,
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

// TestASpawnedMonsterCarriesTheRecordsItWasPlacedWith is the scene: spawn with
// Holds, loot, and the looter alone learns the way in.
func (s *HoldingsSuite) TestASpawnedMonsterCarriesTheRecordsItWasPlacedWith() {
	ctx := context.Background()
	s.start(false) // NOBODY was authored knowing anything
	s.spawnHoldingCaptain([]string{veilMap})
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

// TestASpawnedMonsterHoldingNothingRevealsNothing is the negative that makes
// the scene above a claim about Holds rather than about spawning.
//
// Same verb, same body, same loot — the ONE difference is the field — and the
// bystander's bytes are identical either way, which is design P3 asked of the
// live path.
func (s *HoldingsSuite) TestASpawnedMonsterHoldingNothingRevealsNothing() {
	ctx := context.Background()

	loot := func(holds []string) ([]session.EventKind, string) {
		s.start(false)
		s.spawnHoldingCaptain(holds)
		s.stream.published = nil
		_, err := s.mgr.Loot(ctx, &session.LootInput{
			Session: "sess", Member: "alice", Target: "latecomer", Range: 2})
		s.Require().NoError(err)
		return s.kinds("bob"), s.storyBytes("bob")
	}

	richKinds, richStory := loot([]string{veilMap})
	poorKinds, poorStory := loot(nil)

	s.Equal([]session.EventKind{session.EventLooted}, poorKinds)
	s.Equal(poorKinds, richKinds)
	s.Equal(poorStory, richStory,
		"a monster that arrived knowing nothing and one that arrived knowing the run's "+
			"only secret are indistinguishable to everybody but the looter")
}

// TestASpawnCannotHoldARecordThatIsNotThere is the fail-closed half at the
// seam: an unauthored record is refused by name rather than arriving
// ignorant, and the refusal crosses as this package's own sentinel.
//
// This is the failure a host hits by forwarding the AUTHOR's raw record id
// instead of the compiled `<key>/<id>` dungeonspec mints, which is the one
// mistake worth making loud.
func (s *HoldingsSuite) TestASpawnCannotHoldARecordThatIsNotThere() {
	s.start(false)

	_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "latecomer", Ref: refs.Monsters.Skeleton().String(),
		Position: absolute(spatial.Position{X: 0, Y: 1}), Holds: []string{"no-such-record"},
	})
	s.Require().ErrorIs(err, session.ErrNoIntel,
		"a record this dungeon does not declare — not ErrNoConnection, which is about geometry")

	var placed bool
	for _, npc := range s.sessions.byID["sess"].NPCs {
		if npc.ID == "latecomer" {
			placed = true
		}
	}
	s.False(placed, "the refusal left nothing behind")
	s.Empty(s.stream.published, "and told nobody")
}
