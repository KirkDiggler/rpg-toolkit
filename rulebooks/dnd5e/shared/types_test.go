package shared

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
)

type AbilityScoresTestSuite struct {
	suite.Suite
}

func TestAbilityScoresSuite(t *testing.T) {
	suite.Run(t, new(AbilityScoresTestSuite))
}

func (s *AbilityScoresTestSuite) TestModifierRoundsNegativeHalvesDown() {
	tests := map[string]struct {
		score int
		want  int
	}{
		"score 9": {score: 9, want: -1},
		"score 3": {score: 3, want: -4},
	}

	for name, test := range tests {
		s.Run(name, func() {
			scores := AbilityScores{abilities.INT: test.score}

			s.Equal(test.want, scores.Modifier(abilities.INT))
		})
	}
}
