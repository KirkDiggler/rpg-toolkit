package bundles_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/bundles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BundlesTestSuite struct {
	suite.Suite
}

func TestBundlesTestSuite(t *testing.T) {
	suite.Run(t, new(BundlesTestSuite))
}

func (s *BundlesTestSuite) TestGetBundle() {
	tests := []struct {
		name      string
		bundleID  bundles.BundleID
		wantErr   bool
		checkFunc func(*testing.T, *bundles.Bundle)
	}{
		{
			name:     "Explorer's Pack",
			bundleID: bundles.ExplorersPack,
			wantErr:  false,
			checkFunc: func(t *testing.T, b *bundles.Bundle) {
				assert.Equal(t, bundles.ExplorersPack, b.ID)
				assert.Equal(t, "Explorer's Pack", b.Name)
				assert.NotEmpty(t, b.Description)
				assert.Len(t, b.Items, 8)

				// Check specific items
				hasBackpack := false
				hasTorches := false
				for _, item := range b.Items {
					if item.ItemID == "backpack" {
						hasBackpack = true
						assert.Equal(t, 1, item.Quantity)
					}
					if item.ItemID == "torch" {
						hasTorches = true
						assert.Equal(t, 10, item.Quantity)
					}
				}
				assert.True(t, hasBackpack, "Explorer's Pack should contain a backpack")
				assert.True(t, hasTorches, "Explorer's Pack should contain torches")
			},
		},
		{
			name:     "Dungeoneer's Pack",
			bundleID: bundles.DungeoneersPack,
			wantErr:  false,
			checkFunc: func(t *testing.T, b *bundles.Bundle) {
				assert.Equal(t, bundles.DungeoneersPack, b.ID)
				assert.Equal(t, "Dungeoneer's Pack", b.Name)
				assert.Len(t, b.Items, 9)

				// Check for dungeoneering-specific items
				hasCrowbar := false
				hasPitons := false
				for _, item := range b.Items {
					if item.ItemID == "crowbar" {
						hasCrowbar = true
					}
					if item.ItemID == "piton" {
						hasPitons = true
						assert.Equal(t, 10, item.Quantity)
					}
				}
				assert.True(t, hasCrowbar, "Dungeoneer's Pack should contain a crowbar")
				assert.True(t, hasPitons, "Dungeoneer's Pack should contain pitons")
			},
		},
		{
			name:     "Scholar's Pack",
			bundleID: bundles.ScholarsPack,
			wantErr:  false,
			checkFunc: func(t *testing.T, b *bundles.Bundle) {
				assert.Equal(t, bundles.ScholarsPack, b.ID)
				assert.Equal(t, "Scholar's Pack", b.Name)
				assert.Len(t, b.Items, 7)

				// Check for scholarly items
				hasBook := false
				hasInk := false
				hasParchment := false
				for _, item := range b.Items {
					if item.ItemID == "book" {
						hasBook = true
					}
					if item.ItemID == "ink" {
						hasInk = true
					}
					if item.ItemID == "parchment" {
						hasParchment = true
						assert.Equal(t, 10, item.Quantity)
					}
				}
				assert.True(t, hasBook, "Scholar's Pack should contain a book")
				assert.True(t, hasInk, "Scholar's Pack should contain ink")
				assert.True(t, hasParchment, "Scholar's Pack should contain parchment")
			},
		},
		{
			name:     "Priest's Pack",
			bundleID: bundles.PriestsPack,
			wantErr:  false,
			checkFunc: func(t *testing.T, b *bundles.Bundle) {
				assert.Equal(t, bundles.PriestsPack, b.ID)
				assert.Equal(t, "Priest's Pack", b.Name)
				assert.Len(t, b.Items, 9)

				// Check for religious items
				hasIncense := false
				hasCenser := false
				hasVestments := false
				for _, item := range b.Items {
					if item.ItemID == "incense" {
						hasIncense = true
					}
					if item.ItemID == "censer" {
						hasCenser = true
					}
					if item.ItemID == "vestments" {
						hasVestments = true
					}
				}
				assert.True(t, hasIncense, "Priest's Pack should contain incense")
				assert.True(t, hasCenser, "Priest's Pack should contain a censer")
				assert.True(t, hasVestments, "Priest's Pack should contain vestments")
			},
		},
		{
			name:     "Invalid Bundle",
			bundleID: bundles.BundleID("invalid-bundle"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			bundle, err := bundles.GetBundle(tt.bundleID)

			if tt.wantErr {
				require.Error(s.T(), err)
				assert.Contains(s.T(), err.Error(), "invalid bundle ID")
			} else {
				require.NoError(s.T(), err)
				require.NotNil(s.T(), bundle)
				if tt.checkFunc != nil {
					tt.checkFunc(s.T(), bundle)
				}
			}
		})
	}
}

func (s *BundlesTestSuite) TestGetBundleItems() {
	s.Run("Valid Bundle", func() {
		items, err := bundles.GetBundleItems(bundles.BurglarsPack)
		require.NoError(s.T(), err)
		assert.NotEmpty(s.T(), items)

		// Burglar's Pack should have specific thieving tools
		hasBallBearings := false
		hasString := false
		hasBell := false
		for _, item := range items {
			assert.Equal(s.T(), choices.ItemTypeGear, item.ItemType)
			if item.ItemID == "ball-bearings" {
				hasBallBearings = true
			}
			if item.ItemID == "string" {
				hasString = true
			}
			if item.ItemID == "bell" {
				hasBell = true
			}
		}
		assert.True(s.T(), hasBallBearings, "Burglar's Pack should contain ball bearings")
		assert.True(s.T(), hasString, "Burglar's Pack should contain string")
		assert.True(s.T(), hasBell, "Burglar's Pack should contain a bell")
	})

	s.Run("Invalid Bundle", func() {
		items, err := bundles.GetBundleItems(bundles.BundleID("nonexistent"))
		require.Error(s.T(), err)
		assert.Nil(s.T(), items)
	})
}

func (s *BundlesTestSuite) TestListAllBundles() {
	bundleList := bundles.ListAllBundles()

	// Should have all the expected bundles
	assert.GreaterOrEqual(s.T(), len(bundleList), 7)

	// Check that specific bundles are in the list
	expectedBundles := []bundles.BundleID{
		bundles.ExplorersPack,
		bundles.DungeoneersPack,
		bundles.ScholarsPack,
		bundles.PriestsPack,
		bundles.BurglarsPack,
		bundles.EntertainersPack,
		bundles.DiplomatsPack,
	}

	for _, expected := range expectedBundles {
		found := false
		for _, actual := range bundleList {
			if actual == expected {
				found = true
				break
			}
		}
		assert.True(s.T(), found, "Bundle %s should be in list", expected)
	}
}

func (s *BundlesTestSuite) TestValidateBundleID() {
	tests := []struct {
		name     string
		bundleID bundles.BundleID
		want     bool
	}{
		{
			name:     "Valid Explorer's Pack",
			bundleID: bundles.ExplorersPack,
			want:     true,
		},
		{
			name:     "Valid Entertainer's Pack",
			bundleID: bundles.EntertainersPack,
			want:     true,
		},
		{
			name:     "Valid Diplomat's Pack",
			bundleID: bundles.DiplomatsPack,
			want:     true,
		},
		{
			name:     "Invalid Bundle",
			bundleID: bundles.BundleID("fake-bundle"),
			want:     false,
		},
		{
			name:     "Empty Bundle ID",
			bundleID: bundles.BundleID(""),
			want:     false,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			got := bundles.ValidateBundleID(tt.bundleID)
			assert.Equal(s.T(), tt.want, got)
		})
	}
}

func (s *BundlesTestSuite) TestBundleItemTypes() {
	// All items in bundles should be of type Gear
	bundleList := bundles.ListAllBundles()

	for _, bundleID := range bundleList {
		s.Run(string(bundleID), func() {
			bundle, err := bundles.GetBundle(bundleID)
			require.NoError(s.T(), err)

			for _, item := range bundle.Items {
				assert.Equal(s.T(), choices.ItemTypeGear, item.ItemType,
					"All bundle items should be of type Gear")
				assert.NotEmpty(s.T(), item.ItemID,
					"Item ID should not be empty")
				assert.Greater(s.T(), item.Quantity, 0,
					"Item quantity should be positive")
			}
		})
	}
}

func (s *BundlesTestSuite) TestEntertainersPackSpecifics() {
	bundle, err := bundles.GetBundle(bundles.EntertainersPack)
	require.NoError(s.T(), err)

	// Entertainer's Pack should have costume and disguise kit
	hasCostume := false
	hasDisguiseKit := false
	costumeCount := 0

	for _, item := range bundle.Items {
		if item.ItemID == "costume" {
			hasCostume = true
			costumeCount = item.Quantity
		}
		if item.ItemID == "disguise-kit" {
			hasDisguiseKit = true
		}
	}

	assert.True(s.T(), hasCostume, "Entertainer's Pack should contain costumes")
	assert.Equal(s.T(), 2, costumeCount, "Entertainer's Pack should contain 2 costumes")
	assert.True(s.T(), hasDisguiseKit, "Entertainer's Pack should contain a disguise kit")
}

func (s *BundlesTestSuite) TestDiplomatsPackSpecifics() {
	bundle, err := bundles.GetBundle(bundles.DiplomatsPack)
	require.NoError(s.T(), err)

	// Diplomat's Pack should have fine clothes and writing supplies
	hasFineClothes := false
	hasSealingWax := false
	hasInk := false

	for _, item := range bundle.Items {
		if item.ItemID == "fine-clothes" {
			hasFineClothes = true
		}
		if item.ItemID == "sealing-wax" {
			hasSealingWax = true
		}
		if item.ItemID == "ink" {
			hasInk = true
		}
	}

	assert.True(s.T(), hasFineClothes, "Diplomat's Pack should contain fine clothes")
	assert.True(s.T(), hasSealingWax, "Diplomat's Pack should contain sealing wax")
	assert.True(s.T(), hasInk, "Diplomat's Pack should contain ink")
}
