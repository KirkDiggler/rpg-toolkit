package bundles_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/bundles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// Test constants for repeated item IDs
const (
	inkItemID     = "ink"
	incenseItemID = "incense"
	censerItemID  = "censer"
)

type ResolverTestSuite struct {
	suite.Suite
}

func TestResolverTestSuite(t *testing.T) {
	suite.Run(t, new(ResolverTestSuite))
}

func (s *ResolverTestSuite) TestResolve() {
	s.Run("Valid Bundle", func() {
		input := &bundles.ResolveInput{
			BundleID: bundles.ExplorersPack,
		}

		output, err := bundles.Resolve(input)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), output)
		assert.NotEmpty(s.T(), output.Items)
		assert.Len(s.T(), output.Items, 8) // Explorer's Pack has 8 item types
	})

	s.Run("Invalid Bundle", func() {
		input := &bundles.ResolveInput{
			BundleID: bundles.BundleID("invalid"),
		}

		output, err := bundles.Resolve(input)
		require.Error(s.T(), err)
		assert.Nil(s.T(), output)
		assert.Contains(s.T(), err.Error(), "failed to get bundle")
	})

	s.Run("Empty Bundle ID", func() {
		input := &bundles.ResolveInput{
			BundleID: "",
		}

		output, err := bundles.Resolve(input)
		require.Error(s.T(), err)
		assert.Nil(s.T(), output)
		assert.Contains(s.T(), err.Error(), "bundle ID is required")
	})

	s.Run("Nil Input", func() {
		output, err := bundles.Resolve(nil)
		require.Error(s.T(), err)
		assert.Nil(s.T(), output)
		assert.Contains(s.T(), err.Error(), "input is required")
	})
}

func (s *ResolverTestSuite) TestResolveBundleOption() {
	s.Run("Bundle Option with Predefined Items", func() {
		option := choices.BundleOption{
			ID: "custom-bundle",
			Items: []choices.CountedItem{
				{ItemType: choices.ItemTypeGear, ItemID: "rope", Quantity: 1},
				{ItemType: choices.ItemTypeGear, ItemID: "torch", Quantity: 5},
			},
		}

		items, err := bundles.ResolveBundleOption(option)
		require.NoError(s.T(), err)
		assert.Len(s.T(), items, 2)
		assert.Equal(s.T(), "rope", items[0].ItemID)
		assert.Equal(s.T(), "torch", items[1].ItemID)
	})

	s.Run("Bundle Option by ID", func() {
		option := choices.BundleOption{
			ID:    string(bundles.ScholarsPack),
			Items: nil, // No predefined items
		}

		items, err := bundles.ResolveBundleOption(option)
		require.NoError(s.T(), err)
		assert.NotEmpty(s.T(), items)

		// Check for scholar-specific items
		hasBook := false
		hasInk := false
		for _, item := range items {
			if item.ItemID == "book" {
				hasBook = true
			}
			if item.ItemID == inkItemID {
				hasInk = true
			}
		}
		assert.True(s.T(), hasBook, "Scholar's Pack should contain a book")
		assert.True(s.T(), hasInk, "Scholar's Pack should contain ink")
	})

	s.Run("Invalid Bundle ID", func() {
		option := choices.BundleOption{
			ID:    "nonexistent-bundle",
			Items: nil,
		}

		items, err := bundles.ResolveBundleOption(option)
		require.Error(s.T(), err)
		assert.Nil(s.T(), items)
	})
}

func (s *ResolverTestSuite) TestCreateBundleOption() {
	s.Run("Valid Bundle ID", func() {
		option, err := bundles.CreateBundleOption(bundles.PriestsPack)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), string(bundles.PriestsPack), option.ID)
		assert.NotEmpty(s.T(), option.Items)

		// Check for priest-specific items
		hasIncense := false
		hasCenser := false
		for _, item := range option.Items {
			if item.ItemID == incenseItemID {
				hasIncense = true
			}
			if item.ItemID == censerItemID {
				hasCenser = true
			}
		}
		assert.True(s.T(), hasIncense, "Priest's Pack should contain incense")
		assert.True(s.T(), hasCenser, "Priest's Pack should contain a censer")
	})

	s.Run("Invalid Bundle ID", func() {
		option, err := bundles.CreateBundleOption(bundles.BundleID("fake"))
		require.Error(s.T(), err)
		assert.Empty(s.T(), option.ID)
		assert.Nil(s.T(), option.Items)
	})
}

func (s *ResolverTestSuite) TestExpandChoiceOptions() {
	s.Run("Choice with Bundle References", func() {
		choice := &choices.Choice{
			ID:          choices.ChoiceID("test-choice"),
			Category:    choices.CategoryEquipment,
			Description: "Choose your pack",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.BundleOption{
					ID:    string(bundles.ExplorersPack),
					Items: nil, // No items, should be resolved
				},
				choices.BundleOption{
					ID:    string(bundles.DungeoneersPack),
					Items: nil, // No items, should be resolved
				},
			},
		}

		input := &bundles.ExpandChoiceOptionsInput{
			Choice: choice,
		}

		output, err := bundles.ExpandChoiceOptions(input)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), output)
		assert.Len(s.T(), output.ExpandedOptions, 2)

		// Check that bundles were expanded
		for _, option := range output.ExpandedOptions {
			bundleOpt, ok := option.(choices.BundleOption)
			require.True(s.T(), ok, "Option should be a BundleOption")
			assert.NotEmpty(s.T(), bundleOpt.Items, "Bundle should have items after expansion")
		}
	})

	s.Run("Choice with Pre-populated Bundle", func() {
		predefinedItems := []choices.CountedItem{
			{ItemType: choices.ItemTypeGear, ItemID: "custom-item", Quantity: 1},
		}

		choice := &choices.Choice{
			ID:          choices.ChoiceID("test-choice"),
			Category:    choices.CategoryEquipment,
			Description: "Choose your pack",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.BundleOption{
					ID:    "custom-bundle",
					Items: predefinedItems,
				},
			},
		}

		input := &bundles.ExpandChoiceOptionsInput{
			Choice: choice,
		}

		output, err := bundles.ExpandChoiceOptions(input)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), output)
		assert.Len(s.T(), output.ExpandedOptions, 1)

		// Check that predefined items were preserved
		bundleOpt, ok := output.ExpandedOptions[0].(choices.BundleOption)
		require.True(s.T(), ok)
		assert.Equal(s.T(), predefinedItems, bundleOpt.Items)
	})

	s.Run("Choice with Mixed Options", func() {
		choice := &choices.Choice{
			ID:          choices.ChoiceID("test-choice"),
			Category:    choices.CategoryEquipment,
			Description: "Choose equipment",
			Choose:      1,
			Source:      choices.SourceClass,
			Options: []choices.Option{
				choices.SingleOption{
					ItemType: choices.ItemTypeWeapon,
					ItemID:   "longsword",
				},
				choices.BundleOption{
					ID:    string(bundles.BurglarsPack),
					Items: nil,
				},
			},
		}

		input := &bundles.ExpandChoiceOptionsInput{
			Choice: choice,
		}

		output, err := bundles.ExpandChoiceOptions(input)
		require.NoError(s.T(), err)
		require.NotNil(s.T(), output)
		assert.Len(s.T(), output.ExpandedOptions, 2)

		// Check first option is unchanged
		singleOpt, ok := output.ExpandedOptions[0].(choices.SingleOption)
		require.True(s.T(), ok)
		assert.Equal(s.T(), "longsword", singleOpt.ItemID)

		// Check second option was expanded
		bundleOpt, ok := output.ExpandedOptions[1].(choices.BundleOption)
		require.True(s.T(), ok)
		assert.NotEmpty(s.T(), bundleOpt.Items)
	})

	s.Run("Nil Input", func() {
		output, err := bundles.ExpandChoiceOptions(nil)
		require.Error(s.T(), err)
		assert.Nil(s.T(), output)
		assert.Contains(s.T(), err.Error(), "choice is required")
	})

	s.Run("Nil Choice", func() {
		input := &bundles.ExpandChoiceOptionsInput{
			Choice: nil,
		}

		output, err := bundles.ExpandChoiceOptions(input)
		require.Error(s.T(), err)
		assert.Nil(s.T(), output)
		assert.Contains(s.T(), err.Error(), "choice is required")
	})
}
