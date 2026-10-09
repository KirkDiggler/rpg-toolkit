package spells

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

type CatalogCoverageSuite struct {
	suite.Suite
}

func TestCatalogCoverageSuite(t *testing.T) {
	suite.Run(t, new(CatalogCoverageSuite))
}

func (s *CatalogCoverageSuite) TestEveryExecutableSpellHasCatalogueInformation() {
	for id, content := range castContent {
		s.Run(id, func() {
			data := GetData(id)
			s.Require().NotNil(data, "an executable spell must also be readable in the catalogue")
			s.Equal(id, data.ID)
			s.NotEmpty(data.Name)
			s.NotEmpty(data.Description)
			s.Equal(content.casting.Level, data.Level)
			s.False(data.NotYetImplemented)

			found := false
			for _, entry := range GetSpellsByLevel(data.Level) {
				if entry.ID == id {
					found = true
					s.Equal(data.Name, entry.Name)
					s.Equal(data.Description, entry.Description)
				}
			}
			s.True(found, "catalogue level reads must include the executable spell")
		})
	}
}

func (s *CatalogCoverageSuite) TestCatalogueAccessorsAgree() {
	for id, data := range SpellData {
		s.Run(id, func() {
			s.Equal(data.Name, Name(id))
			s.Equal(data.Description, Description(id))
		})
	}
}

func (s *CatalogCoverageSuite) TestUnknownSpellStaysUnknown() {
	const unknown Spell = "catalogue-unknown-spell"
	s.Nil(GetData(unknown))
	s.Empty(Name(unknown))
	s.Empty(Description(unknown))
	s.False(HasCastProfile(unknown))
}

func (s *CatalogCoverageSuite) TestNamesOutsideTheCatalogueArePreserved() {
	// The identifier set is larger than the catalogue. Consolidating existing
	// catalogue metadata must not remove these pre-existing name-only reads.
	s.Nil(GetData(ArcaneEye))
	s.Equal("Arcane Eye", Name(ArcaneEye))
	s.Empty(Description(ArcaneEye))
	for id, name := range uncataloguedSpellNames {
		s.Run(id, func() {
			s.Nil(GetData(id), "catalogued names must not be duplicated in the fallback")
			s.Equal(name, Name(id))
		})
	}
}

func (s *CatalogCoverageSuite) TestExecutableNamesAgreeWithTheCatalogue() {
	for id := range castContent {
		s.Run(id, func() {
			// Compile an execution consumer separately from the catalogue read.
			// The held weapon lets Shillelagh bind its required actual cast input.
			definition := CastDefinition(CastDefinitionInput{
				Spell: id, SpellSaveDC: 13, SpellAttackBonus: 5,
				SpellcastingAbility: abilities.WIS,
				HeldWeapons: []HeldWeapon{{
					Slot: "main_hand", ItemID: "held-quarterstaff",
					WeaponID: weapons.Quarterstaff, Name: "Quarterstaff",
				}},
			})
			s.Require().NotNil(definition)
			s.Require().NoError(definition.Validate())
			data := GetData(id)
			s.Require().NotNil(data)
			s.Equal(data.Name, definition.Name)
		})
	}
}

// coverageInput compiles a spell the way a caster with a quarterstaff would, so
// Shillelagh binds; twoWeapons adds the club that makes it a menu.
func coverageInput(id Spell, twoWeapons bool) CastDefinitionInput {
	held := []HeldWeapon{{Slot: "main_hand", ItemID: "held-quarterstaff", WeaponID: weapons.Quarterstaff, Name: "Quarterstaff"}}
	if twoWeapons {
		held = append(held, HeldWeapon{Slot: "off_hand", ItemID: "held-club", WeaponID: weapons.Club, Name: "Club"})
	}
	return CastDefinitionInput{
		Spell: id, SpellSaveDC: 13, SpellAttackBonus: 5,
		SpellcastingAbility: abilities.WIS, HeldWeapons: held,
	}
}

func (s *CatalogCoverageSuite) TestEveryDeclaredCastOptionIsDescribed() {
	menus := 0
	check := func(name string, input CastDefinitionInput) {
		s.Run(name, func() {
			definition := CastDefinition(input)
			s.Require().NotNil(definition)
			for _, option := range definition.Cast.Options {
				menus++
				s.NotEmpty(option.Label, option.ID)
				s.NotEmpty(option.Description, "option %q of %s is chosen before commitment", option.ID, name)
			}
		})
	}
	for id := range castContent {
		check(id, coverageInput(id, false))
	}
	check("shillelagh-two-weapons", coverageInput(Shillelagh, true))
	s.Greater(menus, 0, "the loop must reach at least one real menu")
}

func (s *CatalogCoverageSuite) TestShillelaghOptionNamesItsWeapon() {
	definition := CastDefinition(coverageInput(Shillelagh, true))
	s.Require().NotNil(definition)
	s.Require().Len(definition.Cast.Options, 2)
	for _, option := range definition.Cast.Options {
		s.Contains(option.Description, option.Label)
	}
}

func (s *CatalogCoverageSuite) TestEveryCastAppliedConditionHasDetail() {
	for id := range castContent {
		s.Run(id, func() {
			definition := CastDefinition(coverageInput(id, false))
			s.Require().NotNil(definition)
			for _, effect := range definition.Cast.Effects {
				display, ok := conditions.DisplayFor(effect.Ref)
				s.Require().True(ok, "%s applies %s, which has no display entry", id, effect.Ref.String())
				s.NotEmpty(display.Name)
				s.NotEmpty(display.Detail, "%s applies %s, a condition with no authored detail", id, effect.Ref.String())
			}
		})
	}
}
