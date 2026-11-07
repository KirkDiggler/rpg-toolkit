// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// ragingForTest constructs current data through the real shared creator rather
// than retaining a fixture dependency on the removed handwritten runtime type.
func ragingForTest(t testing.TB, member string) dndevents.ConditionBehavior {
	t.Helper()
	out, err := conditions.CreateFromRef(&conditions.CreateFromRefInput{Ref: refs.Conditions.Raging().String(), MemberID: member, Origin: &dndevents.ConditionOrigin{Ref: *refs.Features.Rage(), EntityID: member}, Config: json.RawMessage(`{"damage_bonus":{"barbarian_level":true}}`)})
	require.NoError(t, err)
	return out.Condition
}

func (s *ActivationTestSuite) TestRagingCompositionSurvivesInteractionReload() {
	activated, err := s.run(&ActivationInput{MemberID: heroID, Ability: refs.Features.Rage(), Roller: dice.NewRoller()}, s.world(), []Participant{{Character: s.barbarian(2)}})
	s.Require().NoError(err)
	sheet := dirty(activated, heroID)
	s.Require().NotNil(sheet)
	s.Equal(1, sheet.Resources[resources.RageCharges].Current)
	boundary, err := NewBoundary(&BoundaryInput{Crossed: []encounter.Boundary{{Kind: encounter.TurnEnded, Subject: heroID, Round: 4}}})
	s.Require().NoError(err)
	first, err := s.resolveMachine(boundary, activated.World, []Participant{{Character: sheet}})
	s.Require().NoError(err)
	changed := dirty(first, heroID)
	s.Require().NotNil(changed, "first nonterminal transition must leave a dirty sheet across the real resolution boundary")
	var stored conditions.ComposedData
	s.Require().NoError(json.Unmarshal(conditionWithRefOrNil(changed.Conditions, refs.Conditions.Raging()), &stored))
	s.Require().NotNil(stored.Runtime)
	s.Equal("raging", stored.Runtime.State)
	s.Equal(float64(1), stored.Runtime.Bag["turns_active"])
	s.Equal(heroID, stored.Origin.EntityID)
	s.Equal(1, changed.Resources[resources.RageCharges].Current)
	boundary, err = NewBoundary(&BoundaryInput{Crossed: []encounter.Boundary{{Kind: encounter.TurnEnded, Subject: heroID, Round: 5}}})
	s.Require().NoError(err)
	second, err := s.resolveMachine(boundary, first.World, []Participant{{Character: changed}})
	s.Require().NoError(err)
	ended := dirty(second, heroID)
	s.Require().NotNil(ended)
	s.NotContains(conditionRefs(ended), refs.Conditions.Raging().String(), "a new interaction must not re-grant first-turn grace")
	s.Equal(1, ended.Resources[resources.RageCharges].Current)
}
