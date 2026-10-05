// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// OneConditionPerIdentitySuite applies conditions through the sheet's own
// door — a ConditionAppliedEvent on its bus, the path a directly activated
// ability (Hide, Help, a feature) takes without resolution in between.
type OneConditionPerIdentitySuite struct {
	suite.Suite
	ctx  context.Context
	bus  events.EventBus
	char *Character
}

func TestOneConditionPerIdentitySuite(t *testing.T) {
	suite.Run(t, new(OneConditionPerIdentitySuite))
}

func (s *OneConditionPerIdentitySuite) SetupTest() {
	s.ctx = context.Background()
	s.bus = events.NewEventBus()
	data := fullSheet(&s.Suite)
	data.Conditions = nil
	char, err := LoadFromData(s.ctx, data, s.bus)
	s.Require().NoError(err)
	s.char = char
}

func (s *OneConditionPerIdentitySuite) apply(condition dnd5eEvents.ConditionBehavior) {
	s.Require().NoError(dnd5eEvents.ConditionAppliedTopic.On(s.bus).Publish(s.ctx, dnd5eEvents.ConditionAppliedEvent{
		Target: s.char, Type: dnd5eEvents.ConditionType(condition.Ref().ID),
		Source: dnd5eEvents.ConditionSourceCombatAbility, Condition: condition,
	}))
}

func (s *OneConditionPerIdentitySuite) held(address dnd5eEvents.ConditionAddress) []dnd5eEvents.ConditionBehavior {
	var found []dnd5eEvents.ConditionBehavior
	for _, condition := range s.char.GetConditions() {
		if conditions.ConditionAddressOf(s.char.GetID(), condition) == address {
			found = append(found, condition)
		}
	}
	return found
}

func (s *OneConditionPerIdentitySuite) TestHidingAgainReplacesTheFirstHidden() {
	first := conditions.NewHiddenCondition(s.char.GetID())
	second := conditions.NewHiddenCondition(s.char.GetID())

	s.apply(first)
	s.apply(second)

	held := s.held(conditions.ConditionAddressOf(s.char.GetID(), second))
	s.Require().Len(held, 1, "exactly one Hidden on the sheet")
	s.Same(second, held[0], "the sheet carries the newer Hidden")
	s.False(first.IsApplied(), "the replaced Hidden's handlers are detached")
	s.True(second.IsApplied())
}

func (s *OneConditionPerIdentitySuite) TestASecondHelpReplacesTheFirst() {
	for name, helpers := range map[string][2]string{
		"same helper":       {"cleric", "cleric"},
		"different helpers": {"cleric", "fighter"},
	} {
		s.SetupTest()
		first := conditions.NewHelpedCondition(s.char.GetID(), helpers[0])
		second := conditions.NewHelpedCondition(s.char.GetID(), helpers[1])

		s.apply(first)
		s.apply(second)

		held := s.held(conditions.ConditionAddressOf(s.char.GetID(), second))
		s.Require().Len(held, 1, "%s: two Helps do not stack", name)
		s.Same(second, held[0], name)
		s.Equal(helpers[1], held[0].(*conditions.HelpedCondition).HelperID, name)
		s.False(first.IsApplied(), "%s: the replaced Helped is detached", name)
	}
}

func (s *OneConditionPerIdentitySuite) TestDifferentSourcesAreDifferentIdentities() {
	a, err := conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
		MemberID: s.char.GetID(), SourceID: "cleric-a", SourceRef: refs.Spells.Bless()})
	s.Require().NoError(err)
	b, err := conditions.NewBlessedCondition(conditions.NewBlessedConditionInput{
		MemberID: s.char.GetID(), SourceID: "cleric-b", SourceRef: refs.Spells.Bless()})
	s.Require().NoError(err)

	s.apply(a)
	s.apply(b)

	s.Len(s.held(a.ConditionAddress()), 1)
	s.Len(s.held(b.ConditionAddress()), 1, "a second caster's Bless keeps its own instance")
	s.True(a.IsApplied())
}
