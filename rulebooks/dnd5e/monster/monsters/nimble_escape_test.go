// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// NimbleEscapeContentSuite asks the stat blocks the question the engine will
// ask (rpg-toolkit#2032): does this sheet's stored record carry Nimble
// Escape? It reads the persisted conditions exactly as a seam does, so a
// trait carried only in memory would fail here.
type NimbleEscapeContentSuite struct {
	suite.Suite
}

func TestNimbleEscapeContentSuite(t *testing.T) {
	suite.Run(t, new(NimbleEscapeContentSuite))
}

func (s *NimbleEscapeContentSuite) carries(m *monster.Monster) bool {
	s.T().Helper()
	holds, err := conditions.HoldsRef(m.ToData().Conditions, refs.MonsterTraits.NimbleEscape())
	s.Require().NoError(err)
	return holds
}

func (s *NimbleEscapeContentSuite) TestGoblinCarriesNimbleEscape() {
	s.True(s.carries(monsters.NewGoblin("goblin-1")))
}

func (s *NimbleEscapeContentSuite) TestGoblinBossCarriesNimbleEscape() {
	s.True(s.carries(monsters.NewGoblinBoss("goblin-boss-1")))
}

// TestWolfDoesNotCarryNimbleEscape: the trait is the goblins', not every
// monster's.
func (s *NimbleEscapeContentSuite) TestWolfDoesNotCarryNimbleEscape() {
	s.False(s.carries(monsters.NewWolf("wolf-1")))
}

// TestTheCarriedTraitNamesItsGoblin: the stored blob is owned by the goblin
// that carries it, and loads through the one loader.
func (s *NimbleEscapeContentSuite) TestTheCarriedTraitNamesItsGoblin() {
	want := refs.MonsterTraits.NimbleEscape()
	for _, blob := range monsters.NewGoblin("goblin-7").ToData().Conditions {
		loaded, err := conditions.LoadJSON(blob)
		s.Require().NoError(err)
		if loaded.Ref() != want {
			continue
		}
		s.JSONEq(string(conditions.MustNimbleEscapeJSON("goblin-7")), string(blob))
		return
	}
	s.Fail("the goblin's stored conditions carry no Nimble Escape")
}
