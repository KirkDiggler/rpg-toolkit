package choices

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
)

// RequirementsDetailTestSuite tests that all equipment items have resolved details.
type RequirementsDetailTestSuite struct {
	suite.Suite
}

func TestRequirementsDetailSuite(t *testing.T) {
	suite.Run(t, new(RequirementsDetailTestSuite))
}

func (s *RequirementsDetailTestSuite) TestFighterEquipmentItemsHaveDetails() {
	reqs := GetClassRequirements(classes.Fighter)
	s.Require().NotNil(reqs)
	s.Require().NotEmpty(reqs.Equipment, "fighter should have equipment requirements")

	for _, req := range reqs.Equipment {
		for _, opt := range req.Options {
			for _, item := range opt.Items {
				s.Assert().NotNilf(item.Detail,
					"equipment item %q in option %q should have detail", item.ID, opt.Label)
			}
		}
	}
}

func (s *RequirementsDetailTestSuite) TestBarbarianEquipmentItemsHaveDetails() {
	reqs := GetClassRequirements(classes.Barbarian)
	s.Require().NotNil(reqs)
	s.Require().NotEmpty(reqs.Equipment, "barbarian should have equipment requirements")

	for _, req := range reqs.Equipment {
		for _, opt := range req.Options {
			for _, item := range opt.Items {
				s.Assert().NotNilf(item.Detail,
					"equipment item %q in option %q should have detail", item.ID, opt.Label)
			}
		}
	}
}

func (s *RequirementsDetailTestSuite) TestAllClassesEquipmentItemsHaveDetails() {
	allClasses := []classes.Class{
		classes.Fighter,
		classes.Barbarian,
		classes.Wizard,
		classes.Rogue,
		classes.Cleric,
		classes.Bard,
		classes.Druid,
		classes.Monk,
		classes.Paladin,
		classes.Ranger,
		classes.Sorcerer,
		classes.Warlock,
	}

	for _, classID := range allClasses {
		s.Run(string(classID), func() {
			reqs := GetClassRequirements(classID)
			s.Require().NotNil(reqs)

			for _, req := range reqs.Equipment {
				for _, opt := range req.Options {
					for _, item := range opt.Items {
						s.Assert().NotNilf(item.Detail,
							"class %q: equipment item %q in option %q should have detail",
							classID, item.ID, opt.Label)
						if item.Detail != nil {
							s.Assert().NotEmpty(item.Detail.Name,
								"class %q: equipment item %q detail should have a name",
								classID, item.ID)
						}
					}
				}
			}
		})
	}
}
