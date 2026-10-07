package contributions_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
)

type classLevelsSuite struct{ suite.Suite }

func TestClassLevelsSuite(t *testing.T) { suite.Run(t, new(classLevelsSuite)) }

// A frame whose actor class levels are malformed fails validation: an empty
// class, a class listed twice, or levels below one.
func (s *classLevelsSuite) TestFrameValidateRejectsMalformedClassLevels() {
	for name, levels := range map[string]contributions.ClassLevels{
		"empty class": contributions.KnownClassLevels(contributions.ClassLevel{Class: "", Levels: 2}),
		"repeated class": contributions.KnownClassLevels(
			contributions.ClassLevel{Class: classes.Rogue, Levels: 1},
			contributions.ClassLevel{Class: classes.Rogue, Levels: 2}),
		"zero levels":     contributions.KnownClassLevels(contributions.ClassLevel{Class: classes.Rogue, Levels: 0}),
		"negative levels": contributions.KnownClassLevels(contributions.ClassLevel{Class: classes.Rogue, Levels: -1}),
	} {
		frame := validFrame()
		frame.ActorClassLevels = levels
		s.Error(frame.Validate(), name)
	}
}

// Unknown class levels and known-empty class levels are both valid, and they
// are different answers: only known-empty says the member holds no levels.
func (s *classLevelsSuite) TestUnknownAndKnownEmptyAreValidAndDistinct() {
	unknown := validFrame()
	s.NoError(unknown.Validate(), "the zero value is unknown and valid")
	_, known := unknown.ActorClassLevels.Get()
	s.False(known)
	_, known = unknown.ActorClassLevels.Of(classes.Rogue)
	s.False(known, "unknown never answers zero levels")

	monster := validFrame()
	monster.ActorClassLevels = contributions.KnownClassLevels()
	s.NoError(monster.Validate())
	levels, known := monster.ActorClassLevels.Of(classes.Rogue)
	s.True(known, "a stat block's empty class levels are an answer")
	s.Zero(levels)
}

// Of reads the levels in one class; a class the known list does not name is
// held at zero.
func (s *classLevelsSuite) TestOfReadsTheClassNotTheTotal() {
	levels := contributions.KnownClassLevels(
		contributions.ClassLevel{Class: classes.Fighter, Levels: 5},
		contributions.ClassLevel{Class: classes.Rogue, Levels: 3})

	rogue, known := levels.Of(classes.Rogue)
	s.True(known)
	s.Equal(3, rogue)
	monk, known := levels.Of(classes.Monk)
	s.True(known)
	s.Zero(monk)
}

// The fact holds copies: neither the producer's slice nor a reader's copy
// can edit it, and Clone detaches a frame's class levels.
func (s *classLevelsSuite) TestClassLevelsAreDetached() {
	supplied := []contributions.ClassLevel{{Class: classes.Rogue, Levels: 3}}
	levels := contributions.KnownClassLevels(supplied...)
	supplied[0].Levels = 9
	read, _ := levels.Get()
	read[0].Levels = 7

	rogue, _ := levels.Of(classes.Rogue)
	s.Equal(3, rogue)

	frame := validFrame()
	frame.ActorClassLevels = levels
	clone := frame.Clone()
	cloned, _ := clone.ActorClassLevels.Get()
	s.Equal([]contributions.ClassLevel{{Class: classes.Rogue, Levels: 3}}, cloned)
}
