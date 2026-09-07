package equipment_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/tools"
)

// CategorySuite covers equipment.GetByCategory after its switch-based
// dispatch was replaced with a generic filter over each item's own
// EquipmentCategories() (rpg-toolkit#1554 PR 1). The regression proof for
// existing behavior lives in character/choices' own category-choice
// tests, which must pass unmodified; this suite covers the new
// CategoryArtisanTools tag and pins the semantic traps found in review so
// they can't silently regress: the tool/item internal-to-public category
// translation, and holy symbols/foci being returned for a tool-shaped
// query despite reporting EquipmentTypeItem themselves.
type CategorySuite struct {
	suite.Suite
}

func TestCategorySuite(t *testing.T) {
	suite.Run(t, new(CategorySuite))
}

// TestArtisanToolsReturnsAllSeventeenKinds is the new category this PR
// adds. equipType is passed as EquipmentTypeTool for realism (matching
// how a real EquipmentCategoryChoice.Type is set today), but per the
// no-type-gating design, any value would return the same result.
func (s *CategorySuite) TestArtisanToolsReturnsAllSeventeenKinds() {
	result, err := equipment.GetByCategory(shared.EquipmentTypeTool, []shared.EquipmentCategory{equipment.CategoryArtisanTools})
	s.Require().NoError(err)

	ids := make([]string, 0, len(result))
	for _, e := range result {
		ids = append(ids, e.EquipmentID())
	}

	s.ElementsMatch([]string{
		string(tools.AlchemistSupplies), string(tools.BrewerSupplies), string(tools.CalligrapherSupplies),
		string(tools.CarpenterTools), string(tools.CartographerTools), string(tools.CobblerTools),
		string(tools.CookUtensils), string(tools.GlassblowerTools), string(tools.JewelerTools),
		string(tools.LeatherworkerTools), string(tools.MasonTools), string(tools.PainterSupplies),
		string(tools.PotterTools), string(tools.SmithTools), string(tools.TinkerTools),
		string(tools.WeaverTools), string(tools.WoodcarverTools),
	}, ids, "every artisan's tool kind must be tagged CategoryArtisanTools")
}

// TestMusicalInstrumentsStillResolveThroughTheNewTranslation pins the
// exact bug the review found: tools.CategoryMusical ("musical") and
// equipment.CategoryMusicalInstruments ("musical-instruments") are
// different strings, and only Tool.EquipmentCategories' explicit
// translation bridges them now that the old switch is gone.
func (s *CategorySuite) TestMusicalInstrumentsStillResolveThroughTheNewTranslation() {
	result, err := equipment.GetByCategory(shared.EquipmentTypeTool, []shared.EquipmentCategory{equipment.CategoryMusicalInstruments})
	s.Require().NoError(err)
	s.Len(result, 10, "all ten musical instruments must still resolve")
}

// TestHolySymbolResolvesDespiteReportingItemType pins the other trap: a
// holy symbol's own EquipmentType() is EquipmentTypeItem, not
// EquipmentTypeTool, but Cleric's equipment requirement queries it with
// Type: "tool". The new filter must not gate on EquipmentType matching
// the query's Type, or this silently returns nothing.
func (s *CategorySuite) TestHolySymbolResolvesDespiteReportingItemType() {
	result, err := equipment.GetByCategory(shared.EquipmentTypeTool, []shared.EquipmentCategory{equipment.CategoryHolySymbols})
	s.Require().NoError(err)
	s.Require().Len(result, 1)
	s.Equal(shared.EquipmentTypeItem, result[0].EquipmentType(),
		"the holy symbol itself must still report EquipmentTypeItem")
}
