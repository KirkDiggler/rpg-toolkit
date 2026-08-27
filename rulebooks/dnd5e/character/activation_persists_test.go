// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/stretchr/testify/suite"
)

// ActivationPersistsTestSuite is the test rpg-toolkit#1272 needed and did not
// have: it asks what an activation leaves ON THE SHEET, not what it does to a
// bus.
//
// Disengage shipped applying its condition directly to the interaction's bus.
// Every test it had passed — the condition really did suppress opportunity
// attacks, right up until the bus was torn down at the end of the call. Nothing
// asked whether the character was still disengaging afterwards, so nothing
// noticed that the answer was no.
type ActivationPersistsTestSuite struct {
	suite.Suite
	ctx context.Context
	bus events.EventBus
}

func TestActivationPersistsSuite(t *testing.T) {
	suite.Run(t, new(ActivationPersistsTestSuite))
}

func (s *ActivationPersistsTestSuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
}

func (s *ActivationPersistsTestSuite) sheet() *Data {
	return &Data{
		ID: "persist-fighter", PlayerID: "p1", Name: "Persist",
		Level: 3, ProficiencyBonus: 2,
		RaceID: races.Human, ClassID: classes.Fighter,
		AbilityScores: shared.AbilityScores{
			abilities.STR: 16, abilities.DEX: 14, abilities.CON: 14,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 8,
		},
		HitPoints: 24, MaxHitPoints: 28, ArmorClass: 16,
	}
}

// activated loads a character ONTO THE BUS (which is what attaches its keeper),
// puts it on a turn, activates one ability, and hands back the sheet as it
// would be written.
func (s *ActivationPersistsTestSuite) activated(ability *core.Ref) *Data {
	char, err := LoadFromData(s.ctx, s.sheet(), s.bus)
	s.Require().NoError(err)

	_, err = char.StartTurn(s.ctx, &StartTurnInput{TurnNumber: 1, Speed: 30})
	s.Require().NoError(err)

	// ONE ability per sheet. Both of these cost the action, so activating a
	// second on the same turn would come back "no actions remaining" and the
	// test would be reading an empty condition list for the wrong reason.
	out, err := char.ActivateAbility(s.ctx, &ActivateAbilityInput{AbilityRef: ability})
	s.Require().NoError(err)
	s.Require().True(out.Success, "activation must succeed: %s", out.Error)

	return char.ToData()
}

func conditionRefs(data *Data) []string {
	out := make([]string, 0, len(data.Conditions))
	for _, raw := range data.Conditions {
		var envelope struct {
			Ref string `json:"ref"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			continue
		}
		out = append(out, envelope.Ref)
	}
	return out
}

// THE REGRESSION. Before rpg-toolkit#1272 this came back empty: Disengage
// applied its own condition to the bus, the keeper never heard a
// ConditionAppliedEvent, and the sheet was written without it.
func (s *ActivationPersistsTestSuite) TestDisengagingReachesTheSheet() {
	s.Contains(conditionRefs(s.activated(refs.CombatAbilities.Disengage())), "dnd5e:conditions:disengaging")
}

// The control, and the reason the bug was findable at all: Dodge has always
// published, so it has always persisted. Two abilities three files apart
// behaving differently is what the sweep below now prevents.
func (s *ActivationPersistsTestSuite) TestDodgingReachesTheSheet() {
	s.Contains(conditionRefs(s.activated(refs.CombatAbilities.Dodge())), "dnd5e:conditions:dodging")
}
