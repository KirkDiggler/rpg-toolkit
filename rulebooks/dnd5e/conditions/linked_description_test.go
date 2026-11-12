// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

func (s *linkedDataSuite) TestDescriptionNamesMaintenanceAndItsKnownClock() {
	owner := s.create("caster-a", linkedOwnerConfig())
	description, err := DescribeCondition(&DescribeConditionInput{Condition: owner})
	s.Require().NoError(err)
	s.Equal("Concentrating on Bless", description.Display.Name)
	s.Contains(description.Display.Detail, "10 maintainer turn ends remaining")
	s.Contains(description.Display.Detail, "grace")
	s.Require().NotNil(description.Duration)
	s.Equal(10, description.Duration.Remaining)
	child := s.create("recipient", linkedRecipientConfig())
	local, err := DescribeCondition(&DescribeConditionInput{Condition: child})
	s.Require().NoError(err)
	s.Nil(local.Duration)
	s.NotContains(local.Display.Detail, "remaining")
}
