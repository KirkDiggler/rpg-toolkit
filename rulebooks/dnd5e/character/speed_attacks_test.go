package character

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/stretchr/testify/suite"
)

// SpeedAttacksTestSuite tests GetSpeed and GetExtraAttacksCount methods
type SpeedAttacksTestSuite struct {
	suite.Suite
}

func TestSpeedAttacksSuite(t *testing.T) {
	suite.Run(t, new(SpeedAttacksTestSuite))
}

// GetSpeed tests

func (s *SpeedAttacksTestSuite) TestGetSpeed_Human() {
	char := &Character{raceID: races.Human}
	s.Assert().Equal(30, char.GetSpeed())
}

func (s *SpeedAttacksTestSuite) TestGetSpeed_Dwarf() {
	char := &Character{raceID: races.Dwarf}
	s.Assert().Equal(25, char.GetSpeed())
}

func (s *SpeedAttacksTestSuite) TestGetSpeed_Elf() {
	char := &Character{raceID: races.Elf}
	s.Assert().Equal(30, char.GetSpeed())
}

func (s *SpeedAttacksTestSuite) TestGetSpeed_Halfling() {
	char := &Character{raceID: races.Halfling}
	s.Assert().Equal(25, char.GetSpeed())
}

func (s *SpeedAttacksTestSuite) TestGetSpeed_Gnome() {
	char := &Character{raceID: races.Gnome}
	s.Assert().Equal(25, char.GetSpeed())
}

func (s *SpeedAttacksTestSuite) TestGetSpeed_UnknownRace_DefaultsTo30() {
	char := &Character{raceID: "nonexistent"}
	s.Assert().Equal(30, char.GetSpeed())
}

// GetExtraAttacksCount tests - Fighter

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Fighter_Level1() {
	char := &Character{classID: classes.Fighter, levels: syntheticLevels(classes.Fighter, 1)}
	s.Assert().Equal(0, char.GetExtraAttacksCount())
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Fighter_Level4() {
	char := &Character{classID: classes.Fighter, levels: syntheticLevels(classes.Fighter, 4)}
	s.Assert().Equal(0, char.GetExtraAttacksCount())
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Fighter_Level5() {
	char := &Character{classID: classes.Fighter, levels: syntheticLevels(classes.Fighter, 5)}
	s.Assert().Equal(1, char.GetExtraAttacksCount())
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Fighter_Level10() {
	char := &Character{classID: classes.Fighter, levels: syntheticLevels(classes.Fighter, 10)}
	s.Assert().Equal(1, char.GetExtraAttacksCount())
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Fighter_Level11() {
	char := &Character{classID: classes.Fighter, levels: syntheticLevels(classes.Fighter, 11)}
	s.Assert().Equal(2, char.GetExtraAttacksCount())
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Fighter_Level19() {
	char := &Character{classID: classes.Fighter, levels: syntheticLevels(classes.Fighter, 19)}
	s.Assert().Equal(2, char.GetExtraAttacksCount())
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Fighter_Level20() {
	char := &Character{classID: classes.Fighter, levels: syntheticLevels(classes.Fighter, 20)}
	s.Assert().Equal(3, char.GetExtraAttacksCount())
}

// GetExtraAttacksCount tests - Martial classes (Extra Attack at level 5)

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Barbarian_Level4() {
	char := &Character{classID: classes.Barbarian, levels: syntheticLevels(classes.Barbarian, 4)}
	s.Assert().Equal(0, char.GetExtraAttacksCount())
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Barbarian_Level5() {
	char := &Character{classID: classes.Barbarian, levels: syntheticLevels(classes.Barbarian, 5)}
	s.Assert().Equal(1, char.GetExtraAttacksCount())
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Monk_Level5() {
	char := &Character{classID: classes.Monk, levels: syntheticLevels(classes.Monk, 5)}
	s.Assert().Equal(1, char.GetExtraAttacksCount())
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Paladin_Level5() {
	char := &Character{classID: classes.Paladin, levels: syntheticLevels(classes.Paladin, 5)}
	s.Assert().Equal(1, char.GetExtraAttacksCount())
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Ranger_Level5() {
	char := &Character{classID: classes.Ranger, levels: syntheticLevels(classes.Ranger, 5)}
	s.Assert().Equal(1, char.GetExtraAttacksCount())
}

// GetExtraAttacksCount tests - Non-martial classes

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Rogue_Level5() {
	char := &Character{classID: classes.Rogue, levels: syntheticLevels(classes.Rogue, 5)}
	s.Assert().Equal(0, char.GetExtraAttacksCount())
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Wizard_Level20() {
	char := &Character{classID: classes.Wizard, levels: syntheticLevels(classes.Wizard, 20)}
	s.Assert().Equal(0, char.GetExtraAttacksCount())
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Cleric_Level5() {
	char := &Character{classID: classes.Cleric, levels: syntheticLevels(classes.Cleric, 5)}
	s.Assert().Equal(0, char.GetExtraAttacksCount())
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Warlock_Level5() {
	char := &Character{classID: classes.Warlock, levels: syntheticLevels(classes.Warlock, 5)}
	s.Assert().Equal(0, char.GetExtraAttacksCount())
}

// GetExtraAttacksCount tests - Martial classes don't get more than 1 extra

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Barbarian_Level20() {
	char := &Character{classID: classes.Barbarian, levels: syntheticLevels(classes.Barbarian, 20)}
	s.Assert().Equal(1, char.GetExtraAttacksCount(), "barbarian caps at 1 extra attack")
}

func (s *SpeedAttacksTestSuite) TestExtraAttacks_Monk_Level20() {
	char := &Character{classID: classes.Monk, levels: syntheticLevels(classes.Monk, 20)}
	s.Assert().Equal(1, char.GetExtraAttacksCount(), "monk caps at 1 extra attack")
}
