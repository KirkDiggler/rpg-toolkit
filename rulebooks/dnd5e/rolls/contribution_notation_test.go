// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package rolls_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/rolls"
)

type contributionNotationSuite struct{ suite.Suite }

func TestContributionNotationSuite(t *testing.T) { suite.Run(t, new(contributionNotationSuite)) }

func (s *contributionNotationSuite) TestUnboundNotationMatchesNativeConsumer() {
	for _, tc := range []struct {
		notation    string
		count, size int
	}{{"1d4", 1, 4}, {"d6", 1, 6}, {"2D8", 2, 8}} {
		out, err := rolls.ParseContributionDice(&rolls.ParseContributionDiceInput{Notation: tc.notation})
		s.Require().NoError(err)
		s.Equal(tc.count, out.Count)
		s.Equal(tc.size, out.Size)
	}
	for _, notation := range []string{"", "0d4", "1d0", "-1d4", "1d4+1", "1d4+1d6", " 1d4", "99999999999999999999999999d4", "1d99999999999999999999999999"} {
		out, err := rolls.ParseContributionDice(&rolls.ParseContributionDiceInput{Notation: notation})
		s.Error(err, notation)
		s.Nil(out)
	}
	out, err := rolls.ParseContributionDice(nil)
	s.Require().Error(err)
	s.Nil(out)
}
