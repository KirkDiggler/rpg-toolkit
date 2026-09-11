package choices

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
)

// RequirementsDetailTestSuite tests that all equipment items have resolved details.
type RequirementsDetailTestSuite struct {
	suite.Suite
}

func TestRequirementsDetailSuite(t *testing.T) {
	suite.Run(t, new(RequirementsDetailTestSuite))
}

// TestBardSpells1OffersOnlySpellsThisBuildCanCast catches widening the
// executable catalog to nominal-but-unsupported spells or changing the settled
// choice count.
//
// It pins the PROPERTY rather than the list. The list used to be pinned
// whole (s.Equal([]Spell{Bane}, req.Options)), which tested the catalog's
// length as much as its content and made every second spell an edit to a test
// that was never about that spell. What must stay true is that every option
// offered here resolves to something a caster can actually do — an option that
// produced nothing would be a choice with nothing behind it.
func (s *RequirementsDetailTestSuite) TestBardSpells1OffersOnlySpellsThisBuildCanCast() {
	req := GetClassRequirements(classes.Bard).Spellbook

	s.Require().NotNil(req)
	s.Equal(BardSpells1, req.ID)
	s.Equal(1, req.SpellLevel)
	s.Contains(req.Options, spells.Bane)
	s.Contains(req.Options, spells.Thunderwave)
	s.Contains(req.Options, spells.DissonantWhispers)
	for _, option := range req.Options {
		s.True(spells.HasCastProfile(option),
			"%s is offered as a levelled pick and must compile to a cast", option)
		data := spells.GetData(option)
		s.Require().NotNil(data, "%s", option)
		s.Equal(req.SpellLevel, data.Level, "%s is offered as a level-%d pick", option, req.SpellLevel)
	}
	// The count is asserted against the catalogue rather than against a
	// number. A literal here would be the thing somebody bumps to 4 with the
	// next spell instead of deleting, and it pins nothing this line does not:
	// the bard learns every level-1 spell this build can cast.
	s.Equal(len(req.Options), req.Count,
		"and knows all of them: the count tracks the supported catalogue rather than rationing it")
	s.Equal(4, classes.ClassData[classes.Bard].SpellsKnown,
		"the supported choice count must not rewrite factual class progression")
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
