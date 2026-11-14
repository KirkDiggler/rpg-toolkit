// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"

// Characterize the existing first-match rule before splitting maintenance.
// Checking the clock part before activity would change this reported reason.
func (s *RagingConditionTestSuite) TestIdleWinsAtTheDurationBoundary() {
	_, removed := s.ragingWithRemovalWatch()
	ends := dndevents.TurnEndTopic.On(s.bus)
	s.Require().NoError(ends.Publish(s.ctx, dndevents.TurnEndEvent{SubjectID: "barbarian-1", Round: 4}))
	s.Require().Nil(removed())
	// Both lack of activity and elapsed ten-round duration apply here.
	s.Require().NoError(ends.Publish(s.ctx, dndevents.TurnEndEvent{SubjectID: "barbarian-1", Round: 13}))
	s.Require().NotNil(removed())
	s.Equal("no_combat_activity", removed().Reason)
}

func (s *RagingConditionTestSuite) TestIdleWinsWhenTheClockAlsoResets() {
	_, removed := s.ragingWithRemovalWatch()
	ends := dndevents.TurnEndTopic.On(s.bus)
	s.Require().NoError(ends.Publish(s.ctx, dndevents.TurnEndEvent{SubjectID: "barbarian-1", Round: 7}))
	s.Require().Nil(removed())
	// With no intervening activity, inactivity wins over backwards-clock cleanup.
	s.Require().NoError(ends.Publish(s.ctx, dndevents.TurnEndEvent{SubjectID: "barbarian-1", Round: 1}))
	s.Require().NotNil(removed())
	s.Equal("no_combat_activity", removed().Reason)
}
