// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type PresentedWeaponSuite struct {
	suite.Suite
}

func TestPresentedWeaponSuite(t *testing.T) {
	suite.Run(t, new(PresentedWeaponSuite))
}

func (s *PresentedWeaponSuite) TestAuthoredOrderSelectsOnlyTheTopWeapon() {
	for _, ids := range [][]weapons.WeaponID{
		{weapons.Scimitar},
		{weapons.Shortbow},
		{weapons.Shortsword},
		{weapons.UnarmedStrike},
		{weapons.Scimitar, weapons.Shortbow},
		{weapons.Shortbow, weapons.Scimitar},
	} {
		s.Run(fmt.Sprintf("%s/%d", ids[0], len(ids)), func() {
			m := goblin()
			s.Require().NoError(m.SetWeapons(ids))
			before := m.Actions()
			out, err := monster.PresentedWeapon(&monster.PresentedWeaponInput{Actions: before})
			s.Require().NoError(err)
			s.Require().NotNil(out)
			s.Equal(ids[0], out.WeaponID)
			s.Equal(before, m.Actions(), "presentation does not modify combat actions")
		})
	}
}

func (s *PresentedWeaponSuite) TestDefaultStatBlockUsesItsExistingTopWeapon() {
	for _, m := range []*monster.Monster{
		monsters.NewGoblin("goblin"), monsters.NewSkeleton("skeleton"),
	} {
		actions := m.Actions()
		out, err := monster.PresentedWeapon(&monster.PresentedWeaponInput{Actions: actions})
		s.Require().NoError(err)
		s.Require().NotNil(out)
		s.Equal(weapons.WeaponID(actions[0].Ref.ID), out.WeaponID)
		s.Equal(actions, m.Actions())
	}
}

func (s *PresentedWeaponSuite) TestMultiattackAtTopIsSkippedForTheFirstWeaponEntry() {
	m := monsters.NewGoblinBoss("boss")
	actions := m.Actions()
	s.Require().NotNil(actions[0].Sequence, "real boss factory puts Multiattack first")
	s.Require().Greater(len(actions), 1)
	out, err := monster.PresentedWeapon(&monster.PresentedWeaponInput{Actions: actions})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Equal(weapons.Scimitar, out.WeaponID, "skip Multiattack, then select the first weapon definition")
	s.Equal(actions, m.Actions())
}

func (s *PresentedWeaponSuite) TestStoredOrderSurvivesRoundTripAndFreshLoad() {
	m := goblin()
	s.Require().NoError(m.SetWeapons([]weapons.WeaponID{weapons.Shortbow, weapons.Scimitar}))
	stored, err := json.Marshal(m.ToData())
	s.Require().NoError(err)
	var data monster.Data
	s.Require().NoError(json.Unmarshal(stored, &data))
	loaded, err := monster.Load(context.Background(), &data)
	s.Require().NoError(err)
	out, err := monster.PresentedWeapon(&monster.PresentedWeaponInput{Actions: loaded.Actions()})
	s.Require().NoError(err)
	s.Equal(weapons.Shortbow, out.WeaponID)
}

func (s *PresentedWeaponSuite) TestNonWeaponPrefixIsSkippedWithoutReorderingWeapons() {
	actions := []combatActions.Definition{
		{Ref: core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "bite"}},
		{Ref: core.Ref{Module: "dnd5e", Type: "weapons", ID: "shortbow"}},
	}
	out, err := monster.PresentedWeapon(&monster.PresentedWeaponInput{Actions: actions})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Equal(weapons.Shortbow, out.WeaponID, "the first weapon entry is the presentation authority")
}

func (s *PresentedWeaponSuite) TestNamespaceCollisionsDoNotInventWeapons() {
	for _, ref := range []core.Ref{
		{Module: "homebrew", Type: "weapons", ID: "scimitar"},
		{Module: "dnd5e", Type: "monster_actions", ID: "shortbow"},
	} {
		out, err := monster.PresentedWeapon(&monster.PresentedWeaponInput{
			Actions: []combatActions.Definition{{Ref: ref}},
		})
		s.Require().NoError(err)
		s.Require().NotNil(out)
		s.Empty(out.WeaponID)
	}
}

func (s *PresentedWeaponSuite) TestNaturalOnlyListDoesNotInventAWeapon() {
	out, err := monster.PresentedWeapon(&monster.PresentedWeaponInput{
		Actions: []combatActions.Definition{
			{Ref: core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "bite"}},
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Empty(out.WeaponID)
}

func (s *PresentedWeaponSuite) TestUnknownFirstWeaponDoesNotFallThroughToKnownWeapon() {
	out, err := monster.PresentedWeapon(&monster.PresentedWeaponInput{
		Actions: []combatActions.Definition{
			{Ref: core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "multiattack"}},
			{Ref: core.Ref{Module: "dnd5e", Type: "weapons", ID: "trebuchet"}},
			{Ref: core.Ref{Module: "dnd5e", Type: "weapons", ID: "scimitar"}},
		},
	})
	s.Require().Error(err)
	s.Nil(out)
}

func (s *PresentedWeaponSuite) TestEntriesAfterFirstWeaponAreNotConsulted() {
	out, err := monster.PresentedWeapon(&monster.PresentedWeaponInput{
		Actions: []combatActions.Definition{
			{Ref: core.Ref{Module: "dnd5e", Type: "weapons", ID: "shortbow"}},
			{},
		},
	})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Equal(weapons.Shortbow, out.WeaponID)
}

func (s *PresentedWeaponSuite) TestAbsentListIsUnknownNotObservedEmptyHands() {
	out, err := monster.PresentedWeapon(&monster.PresentedWeaponInput{})
	s.Require().NoError(err)
	s.Require().NotNil(out)
	s.Empty(out.WeaponID)
}

func (s *PresentedWeaponSuite) TestRefusesInvalidInputs() {
	for _, input := range []*monster.PresentedWeaponInput{
		nil,
		{Actions: []combatActions.Definition{{}}},
		{Actions: []combatActions.Definition{{Ref: core.Ref{Module: "dnd5e", Type: "weapons", ID: "trebuchet"}}}},
	} {
		out, err := monster.PresentedWeapon(input)
		s.Require().Error(err)
		s.Nil(out)
	}
}

func (s *PresentedWeaponSuite) TestInputRemainsUnchanged() {
	m := goblin()
	s.Require().NoError(m.SetWeapons([]weapons.WeaponID{weapons.Scimitar, weapons.Shortbow}))
	input := &monster.PresentedWeaponInput{Actions: m.Actions()}
	before, err := json.Marshal(input)
	s.Require().NoError(err)
	_, err = monster.PresentedWeapon(input)
	s.Require().NoError(err)
	after, err := json.Marshal(input)
	s.Require().NoError(err)
	s.Equal(before, after)
}
