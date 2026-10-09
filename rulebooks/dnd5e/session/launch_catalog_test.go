// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// LaunchCatalogSuite is the Launch twin of what the deleted Spawn suite pinned
// about content that lives in code: a placement names WHAT to build and never
// builds it. Nothing asserted here is passed in by the caller; the hit points,
// armour class and speed exist only because the catalog constructor ran.
type LaunchCatalogSuite struct {
	suite.Suite

	sessions *fakeSessions
	mgr      *session.Manager
}

func TestLaunchCatalogSuite(t *testing.T) { suite.Run(t, new(LaunchCatalogSuite)) }

func (s *LaunchCatalogSuite) SetupTest() {
	s.sessions = newFakeSessions()
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: newFakeEncounters(),
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

func (s *LaunchCatalogSuite) launch(monsters ...dungeonspec.MonsterPlacement) *session.LaunchOutput {
	sc := hexWorld()
	sc.Monsters = monsters
	return launchScene(s.T(), s.mgr, sc)
}

func (s *LaunchCatalogSuite) stored(id string) *monster.Data {
	s.T().Helper()
	for i := range s.sessions.byID[testSession].NPCs {
		if s.sessions.byID[testSession].NPCs[i].ID == id {
			return &s.sessions.byID[testSession].NPCs[i]
		}
	}
	s.Require().Failf("no stored sheet", "no sheet recorded for %s", id)
	return nil
}

// A placement by catalog ref records the catalog entry's sheet under the
// placement's own identity.
func (s *LaunchCatalogSuite) TestAPlacementRecordsTheCatalogEntry() {
	skeleton := refs.Monsters.Skeleton().String()
	out := s.launch(monsterAt("skel-1", skeleton, 8, 4))

	sheet := s.stored("skel-1")
	s.Equal("Skeleton", sheet.Name)
	s.Equal(skeleton, sheet.Ref.String(), "it reports what it was built from")
	s.Equal(13, sheet.HitPoints)
	s.Equal(13, sheet.MaxHitPoints)
	s.Equal(13, sheet.ArmorClass)
	s.Equal(30, sheet.Speed.Walk)
	s.Contains(memberIDs(out), "skel-1", "and it stands on the board")
}

// Three refs, three different sheets, differing in both directions, so no
// defaulted value can pass.
func (s *LaunchCatalogSuite) TestDifferentRefsBuildDifferentMonsters() {
	out := s.launch(
		monsterAt("skeleton", refs.Monsters.Skeleton().String(), 8, 4),
		monsterAt("zombie", refs.Monsters.Zombie().String(), 9, 4),
		monsterAt("rat", refs.Monsters.GiantRat().String(), 10, 4),
	)
	for _, tc := range []struct {
		id, name string
		hp, ac   int
	}{
		{"skeleton", "Skeleton", 13, 13},
		{"zombie", "Zombie", 22, 8},
		{"rat", "Giant Rat", 7, 12},
	} {
		sheet := s.stored(tc.id)
		s.Equal(tc.name, sheet.Name)
		s.Equal(tc.hp, sheet.HitPoints)
		s.Equal(tc.ac, sheet.ArmorClass)
		s.Contains(memberIDs(out), tc.id)
	}
}

// One ref makes many members: a template carries no identity, so the second
// skeleton does not collide with the first.
func (s *LaunchCatalogSuite) TestOneRefMakesManyMembers() {
	skeleton := refs.Monsters.Skeleton().String()
	out := s.launch(
		monsterAt("skel-1", skeleton, 8, 4),
		monsterAt("skel-2", skeleton, 9, 4),
		monsterAt("skel-3", skeleton, 10, 4),
	)

	for _, id := range []string{"skel-1", "skel-2", "skel-3"} {
		s.Equal(skeleton, s.stored(id).Ref.String(), "built from the same catalog entry")
		s.Contains(memberIDs(out), id, "%s is on the board", id)
	}
	s.Len(s.sessions.byID[testSession].NPCs, 3, "and each is remembered separately")
}

func memberIDs(out *session.LaunchOutput) []string {
	ids := make([]string, 0, len(out.Members))
	for _, member := range out.Members {
		ids = append(ids, member.ID)
	}
	return ids
}
