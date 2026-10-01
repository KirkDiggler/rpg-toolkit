// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type equipmentCharacters struct {
	byID map[string]*character.Data
	gets []string
}

func (r *equipmentCharacters) GetCharacter(_ context.Context, id string) (*character.Data, error) {
	r.gets = append(r.gets, id)
	data, ok := r.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return data, nil
}

func (r *equipmentCharacters) SaveCharacter(_ context.Context, data *character.Data) error {
	r.byID[data.ID] = data
	return nil
}

type MonsterEquipmentSeamSuite struct {
	suite.Suite
	data  *SessionData
	chars *equipmentCharacters
	seam  equipmentSeam
}

func TestMonsterEquipmentSeamSuite(t *testing.T) {
	suite.Run(t, new(MonsterEquipmentSeamSuite))
}

func (s *MonsterEquipmentSeamSuite) SetupTest() {
	s.data = &SessionData{}
	s.chars = &equipmentCharacters{byID: map[string]*character.Data{}}
	s.seam = equipmentBeside(standingSeam{
		ctx: context.Background(), chars: s.chars, data: s.data,
		kinds: map[string]encounter.MemberKind{
			"goblin": encounter.KindMonster, "player": encounter.KindPlayer, "door": encounter.KindWorld,
		},
	})
}

func (s *MonsterEquipmentSeamSuite) storeGoblin(ids ...weapons.WeaponID) {
	m := monsters.NewGoblin("goblin")
	if len(ids) > 0 {
		s.Require().NoError(m.SetWeapons(ids))
	}
	s.data.NPCs = []monster.Data{*m.ToData()}
}

func (s *MonsterEquipmentSeamSuite) TestReadsAliasedRecordAfterSeamConstructionAndWithoutCaching() {
	before, err := s.seam.Equipment([]encounter.MemberID{"goblin"})
	s.Require().NoError(err)
	s.Contains(before, encounter.MemberID("goblin"))
	s.Nil(before["goblin"], "sheetless is unknown, not observed empty hands")

	s.storeGoblin(weapons.Shortbow, weapons.Scimitar)
	first, err := s.seam.Equipment([]encounter.MemberID{"goblin"})
	s.Require().NoError(err)
	s.Require().NotNil(first["goblin"])
	s.Equal("shortbow", first["goblin"].MainHand)
	s.Empty(first["goblin"].OffHand)

	s.storeGoblin(weapons.Scimitar, weapons.Shortbow)
	second, err := s.seam.Equipment([]encounter.MemberID{"goblin"})
	s.Require().NoError(err)
	s.Equal("scimitar", second["goblin"].MainHand)
	s.Equal("shortbow", first["goblin"].MainHand, "previous answer is not a live sheet alias")
	s.Empty(s.chars.gets, "monsters never query the character repository")
}

func (s *MonsterEquipmentSeamSuite) TestCarriesProviderAnswerPastMultiattackWithoutInterpretingIt() {
	s.storeGoblin(weapons.Scimitar, weapons.Shortbow)
	// A recursive interpretation would pick shortbow. The first flat weapon
	// is scimitar, so this fixture distinguishes skipping from recursing.
	s.data.NPCs[0].Actions = append([]combatActions.Definition{{
		Ref: *refs.MonsterActions.GoblinBossMultiattack(), Name: "Multiattack",
		Sequence: &combatActions.SequenceProfile{Steps: []combatActions.SequenceStep{
			{Action: *refs.Weapons.Shortbow()},
		}},
	}}, s.data.NPCs[0].Actions...)
	out, err := s.seam.Equipment([]encounter.MemberID{"goblin"})
	s.Require().NoError(err)
	s.Require().NotNil(out["goblin"])
	s.Equal("scimitar", out["goblin"].MainHand)
}

func (s *MonsterEquipmentSeamSuite) TestRosterKindSelectsOneStoreAndOnlyAskedMembersAreReturned() {
	s.storeGoblin(weapons.Shortbow)
	s.chars.byID["goblin"] = &character.Data{ID: "goblin", EquipmentSlots: character.EquipmentSlots{
		character.SlotMainHand: "greataxe",
	}}
	s.chars.byID["player"] = &character.Data{ID: "player", EquipmentSlots: character.EquipmentSlots{
		character.SlotMainHand: "longsword", character.SlotOffHand: "shield",
	}}
	s.data.NPCs = append(s.data.NPCs, *monsters.NewGoblin("player").ToData(), *monsters.NewGoblin("door").ToData())
	out, err := s.seam.Equipment([]encounter.MemberID{"player", "goblin", "door"})
	s.Require().NoError(err)
	s.Len(out, 3)
	s.Equal(&encounter.HeldEquipment{MainHand: "longsword", OffHand: "shield"}, out["player"])
	s.Equal("shortbow", out["goblin"].MainHand)
	s.Nil(out["door"])
	s.Equal([]string{"player"}, s.chars.gets)

	out, err = s.seam.Equipment([]encounter.MemberID{"goblin"})
	s.Require().NoError(err)
	s.Len(out, 1)
	s.Contains(out, encounter.MemberID("goblin"))
}

func (s *MonsterEquipmentSeamSuite) TestAbsentAndNonCatalogWeaponAnswersStayUnknown() {
	s.storeGoblin()
	s.data.NPCs[0].Actions = []combatActions.Definition{
		{Ref: core.Ref{Module: "dnd5e", Type: "monster_actions", ID: "bite"}},
	}
	out, err := s.seam.Equipment([]encounter.MemberID{"goblin", "player"})
	s.Require().NoError(err)
	s.Nil(out["goblin"])
	s.Nil(out["player"])

	s.data.NPCs[0].Actions = nil
	out, err = s.seam.Equipment([]encounter.MemberID{"goblin"})
	s.Require().NoError(err)
	s.Nil(out["goblin"])
}

func (s *MonsterEquipmentSeamSuite) TestUnarmedStrikeIDIsCarriedWithoutInventingAVisualOrEquipmentSlot() {
	s.storeGoblin(weapons.UnarmedStrike)
	out, err := s.seam.Equipment([]encounter.MemberID{"goblin"})
	s.Require().NoError(err)
	s.Require().NotNil(out["goblin"])
	s.Equal("unarmed-strike", out["goblin"].MainHand)
}

func (s *MonsterEquipmentSeamSuite) TestProviderRefusalIsInvalidSessionAndNotAFallbackWeapon() {
	s.storeGoblin(weapons.Shortbow, weapons.Scimitar)
	s.data.NPCs[0].Actions[0].Ref.ID = "trebuchet"
	out, err := s.seam.Equipment([]encounter.MemberID{"goblin"})
	s.Require().ErrorIs(err, ErrInvalidSession)
	s.ErrorContains(err, "goblin")
	s.Nil(out)
}

func (s *MonsterEquipmentSeamSuite) TestMissingRosterKindIsRefused() {
	out, err := s.seam.Equipment([]encounter.MemberID{"stranger"})
	s.Require().ErrorIs(err, ErrInvalidSession)
	s.Nil(out)
}
