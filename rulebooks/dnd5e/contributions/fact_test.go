package contributions_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	"github.com/stretchr/testify/suite"
)

type factSuite struct{ suite.Suite }

func TestFactSuite(t *testing.T) { suite.Run(t, new(factSuite)) }

func (s *factSuite) TestZeroIsUnknownNotFalse() {
	var zero contributions.Fact[bool]
	value, known := zero.Get()
	s.False(value)
	s.False(known)
	s.Equal(zero, contributions.Unknown[bool]())
	value, known = contributions.Known(false).Get()
	s.False(value)
	s.True(known)
	s.NotEqual(zero, contributions.Known(false))
}

func (s *factSuite) TestKnownZeroEmptyAndNamedValues() {
	integer, known := contributions.Known(0).Get()
	s.Zero(integer)
	s.True(known)
	text, known := contributions.Known("").Get()
	s.Empty(text)
	s.True(known)
	distance, known := contributions.Known(0.0).Get()
	s.Zero(distance)
	s.True(known)
	ability, known := contributions.Known(abilities.STR).Get()
	s.Equal(abilities.STR, ability)
	s.True(known)
}
