package spells

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
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
