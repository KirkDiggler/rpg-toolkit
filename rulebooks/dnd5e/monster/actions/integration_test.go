// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

type IntegrationTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestIntegrationSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func (s *IntegrationTestSuite) TestLoadFromData() {
	data := &monster.Data{
		ID:           "goblin-1",
		Name:         "Goblin",
		Ref:          refs.Monsters.Goblin(),
		HitPoints:    7,
		MaxHitPoints: 7,
		ArmorClass:   15,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8,
			abilities.DEX: 14,
			abilities.CON: 10,
			abilities.INT: 10,
			abilities.WIS: 8,
			abilities.CHA: 8,
		},
		Speed:  monster.SpeedData{Walk: 30},
		Senses: monster.SensesData{Darkvision: 60, PassivePerception: 9},
		Actions: []monster.ActionData{
			{
				Ref:    *refs.MonsterActions.Scimitar(),
				Config: []byte(`{"attack_bonus": 4, "damage_dice": "1d6+2"}`),
			},
		},
		Proficiencies: []monster.ProficiencyData{
			{Skill: "stealth", Bonus: 6},
		},
	}

	m, err := monster.LoadFromData(s.ctx, data, s.bus)

	s.Require().NoError(err)
	s.Require().NotNil(m)
	s.Equal("goblin-1", m.GetID())
	s.Equal("Goblin", m.Name())
	s.Equal(7, m.HP())
	s.Equal(7, m.MaxHP())
	s.Equal(15, m.AC())
	s.Equal(30, m.Speed().Walk)
	s.Equal(60, m.Senses().Darkvision)

	// Load actions using helper (to avoid import cycle)
	err = actions.LoadMonsterActions(m, data.Actions)
	s.Require().NoError(err)

	// Verify action was loaded
	monsterActions := m.Actions()
	s.Require().Len(monsterActions, 1)
	s.Equal("scimitar", monsterActions[0].GetID())
}

func (s *IntegrationTestSuite) TestActionRoundTrip() {
	// Create a goblin with the factory (includes scimitar)
	original := monster.NewGoblin("goblin-1")

	// Verify original has action
	s.Require().Len(original.Actions(), 1)

	// Convert to data
	data := original.ToData()

	// Verify action was serialized
	s.Require().Len(data.Actions, 1)
	s.Equal("scimitar", data.Actions[0].Ref.ID)

	// Load from data
	loaded, err := monster.LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)

	// Load actions using helper (to avoid import cycle)
	err = actions.LoadMonsterActions(loaded, data.Actions)
	s.Require().NoError(err)

	// Verify action was deserialized
	loadedActions := loaded.Actions()
	s.Require().Len(loadedActions, 1)
	s.Equal("scimitar", loadedActions[0].GetID())
	s.Equal(monster.TypeMeleeAttack, loadedActions[0].ActionType())

	// The action should be functional - verify it can be serialized again
	reData := loaded.ToData()
	s.Require().Len(reData.Actions, 1)
	s.Equal("scimitar", reData.Actions[0].Ref.ID)
}
