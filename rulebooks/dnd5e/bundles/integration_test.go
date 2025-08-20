package bundles_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/bundles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type IntegrationTestSuite struct {
	suite.Suite
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) TestBundleWithChoiceValidator() {
	// Create a choice that includes bundles
	choice := choices.Choice{
		ID:          choices.ChoiceID("starting-pack"),
		Category:    choices.CategoryEquipment,
		Description: "Choose your starting equipment pack",
		Choose:      1,
		Source:      choices.SourceClass,
		Options: []choices.Option{
			choices.BundleOption{
				ID: string(bundles.ExplorersPack),
				Items: []choices.CountedItem{
					{ItemType: choices.ItemTypeGear, ItemID: "backpack", Quantity: 1},
					{ItemType: choices.ItemTypeGear, ItemID: "bedroll", Quantity: 1},
					{ItemType: choices.ItemTypeGear, ItemID: "torch", Quantity: 10},
				},
			},
			choices.BundleOption{
				ID: string(bundles.ScholarsPack),
				Items: []choices.CountedItem{
					{ItemType: choices.ItemTypeGear, ItemID: "backpack", Quantity: 1},
					{ItemType: choices.ItemTypeGear, ItemID: "book", Quantity: 1},
					{ItemType: choices.ItemTypeGear, ItemID: "ink", Quantity: 1},
				},
			},
		},
	}

	// Test valid selection
	selections := []string{string(bundles.ExplorersPack)}
	err := choices.ValidateSelection(choice, selections)
	assert.NoError(s.T(), err)

	// Test invalid selection
	selections = []string{"invalid-pack"}
	err = choices.ValidateSelection(choice, selections)
	assert.Error(s.T(), err)
}

func (s *IntegrationTestSuite) TestBundleResolutionWorkflow() {
	// Simulate a workflow where a player selects a bundle
	// and the system needs to resolve it to actual items

	// Step 1: Player is presented with a choice
	choice := &choices.Choice{
		ID:          choices.ChoiceID("equipment-pack"),
		Category:    choices.CategoryEquipment,
		Description: "Choose your adventuring pack",
		Choose:      1,
		Source:      choices.SourceBackground,
		Options: []choices.Option{
			choices.BundleOption{
				ID: string(bundles.DungeoneersPack),
			},
			choices.BundleOption{
				ID: string(bundles.BurglarsPack),
			},
		},
	}

	// Step 2: Expand the bundles to show what they contain
	expandInput := &bundles.ExpandChoiceOptionsInput{
		Choice: choice,
	}
	expandOutput, err := bundles.ExpandChoiceOptions(expandInput)
	require.NoError(s.T(), err)

	// Verify bundles were expanded
	for _, option := range expandOutput.ExpandedOptions {
		bundleOpt, ok := option.(choices.BundleOption)
		require.True(s.T(), ok)
		assert.NotEmpty(s.T(), bundleOpt.Items, "Bundle should have items after expansion")
	}

	// Step 3: Player selects Burglar's Pack
	selectedBundleID := bundles.BurglarsPack

	// Step 4: Resolve the selected bundle to get actual items
	resolveInput := &bundles.ResolveInput{
		BundleID: selectedBundleID,
	}
	resolveOutput, err := bundles.Resolve(resolveInput)
	require.NoError(s.T(), err)

	// Step 5: Verify we got the expected items
	assert.NotEmpty(s.T(), resolveOutput.Items)

	// Check for burglar-specific items
	hasBallBearings := false
	hasHoodedLantern := false
	hasCrowbar := false

	for _, item := range resolveOutput.Items {
		switch item.ItemID {
		case "ball-bearings":
			hasBallBearings = true
		case "lantern-hooded":
			hasHoodedLantern = true
		case "crowbar":
			hasCrowbar = true
		}
	}

	assert.True(s.T(), hasBallBearings, "Burglar's Pack should contain ball bearings")
	assert.True(s.T(), hasHoodedLantern, "Burglar's Pack should contain a hooded lantern")
	assert.True(s.T(), hasCrowbar, "Burglar's Pack should contain a crowbar")
}

func (s *IntegrationTestSuite) TestBundleCreationFromID() {
	// Test creating bundle options dynamically from IDs
	bundleIDs := []bundles.BundleID{
		bundles.ExplorersPack,
		bundles.DiplomatsPack,
		bundles.EntertainersPack,
	}

	options := make([]choices.Option, 0, len(bundleIDs))

	for _, id := range bundleIDs {
		bundleOpt, err := bundles.CreateBundleOption(id)
		require.NoError(s.T(), err)
		options = append(options, bundleOpt)
	}

	// Create a choice with these options
	choice := choices.Choice{
		ID:          choices.ChoiceID("pack-selection"),
		Category:    choices.CategoryEquipment,
		Description: "Choose your equipment pack",
		Choose:      1,
		Source:      choices.SourceClass,
		Options:     options,
	}

	// Validate the choice structure
	assert.Len(s.T(), choice.Options, 3)

	// Verify each option has items
	for i, opt := range choice.Options {
		bundleOpt, ok := opt.(choices.BundleOption)
		require.True(s.T(), ok, "Option %d should be a BundleOption", i)
		assert.NotEmpty(s.T(), bundleOpt.Items, "Bundle option %d should have items", i)
		assert.NotEmpty(s.T(), bundleOpt.ID, "Bundle option %d should have an ID", i)
	}
}

func (s *IntegrationTestSuite) TestAllBundlesAreValid() {
	// Comprehensive test to ensure all defined bundles are valid
	allBundles := bundles.ListAllBundles()

	assert.NotEmpty(s.T(), allBundles, "Should have at least some bundles defined")

	for _, bundleID := range allBundles {
		s.Run(string(bundleID), func() {
			// Test GetBundle
			bundle, err := bundles.GetBundle(bundleID)
			require.NoError(s.T(), err)
			assert.NotNil(s.T(), bundle)
			assert.Equal(s.T(), bundleID, bundle.ID)
			assert.NotEmpty(s.T(), bundle.Name)
			assert.NotEmpty(s.T(), bundle.Description)
			assert.NotEmpty(s.T(), bundle.Items)

			// Test Resolve
			resolveInput := &bundles.ResolveInput{BundleID: bundleID}
			resolveOutput, err := bundles.Resolve(resolveInput)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), bundle.Items, resolveOutput.Items)

			// Test CreateBundleOption
			bundleOpt, err := bundles.CreateBundleOption(bundleID)
			require.NoError(s.T(), err)
			assert.Equal(s.T(), string(bundleID), bundleOpt.ID)
			assert.Equal(s.T(), bundle.Items, bundleOpt.Items)

			// Validate the bundle option
			err = bundleOpt.Validate()
			assert.NoError(s.T(), err)
		})
	}
}
