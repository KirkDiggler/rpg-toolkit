package character

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type EquipmentTestSuite struct {
	suite.Suite
}

func TestEquipmentSuite(t *testing.T) {
	suite.Run(t, new(EquipmentTestSuite))
}

func (s *EquipmentTestSuite) TestProcessEquipmentChoices() {
	testCases := []struct {
		name     string
		choices  []string
		expected []string
	}{
		{
			name:     "individual items only",
			choices:  []string{"Longsword", "Shield", "Chain Mail"},
			expected: []string{"Longsword", "Shield", "Chain Mail"},
		},
		{
			name:    "dungeoneer's pack expansion",
			choices: []string{"Longsword", "Dungeoneer's Pack"},
			expected: []string{
				"Longsword",
				"Backpack",
				"Crowbar",
				"Hammer",
				"Piton (10)",
				"Torch (10)",
				"Tinderbox",
				"Rations (10 days)",
				"Waterskin",
				"Hempen Rope (50 feet)",
			},
		},
		{
			name:    "explorer's pack expansion",
			choices: []string{"Explorer's Pack", "Javelin (5)"},
			expected: []string{
				"Backpack",
				"Bedroll",
				"Mess Kit",
				"Tinderbox",
				"Torch (10)",
				"Rations (10 days)",
				"Waterskin",
				"Hempen Rope (50 feet)",
				"Javelin (5)",
			},
		},
		{
			name:    "multiple packs",
			choices: []string{"Scholar's Pack", "Priest's Pack"},
			expected: []string{
				// Scholar's Pack
				"Backpack",
				"Book of Lore",
				"Bottle of Ink",
				"Ink Pen",
				"Parchment (10 sheets)",
				"Little Bag of Sand",
				"Small Knife",
				// Priest's Pack
				"Backpack", // Duplicate is intentional - they get 2 backpacks
				"Blanket",
				"Candle (10)",
				"Tinderbox",
				"Alms Box",
				"Block of Incense (2)",
				"Censer",
				"Vestments",
				"Rations (2 days)",
				"Waterskin",
			},
		},
		{
			name:     "empty choices",
			choices:  []string{},
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := processEquipmentChoices(tc.choices)
			s.Assert().Equal(tc.expected, result)
		})
	}
}

func (s *EquipmentTestSuite) TestFormatEquipmentWithQuantity() {
	testCases := []struct {
		name     string
		itemID   string
		quantity int
		expected string
	}{
		{
			name:     "single item",
			itemID:   "Longsword",
			quantity: 1,
			expected: "Longsword",
		},
		{
			name:     "multiple items",
			itemID:   "Javelin",
			quantity: 5,
			expected: "Javelin (5)",
		},
		{
			name:     "zero quantity",
			itemID:   "Arrow",
			quantity: 0,
			expected: "Arrow",
		},
		{
			name:     "large quantity",
			itemID:   "Ball Bearings",
			quantity: 1000,
			expected: "Ball Bearings (1000)",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result := formatEquipmentWithQuantity(tc.itemID, tc.quantity)
			s.Assert().Equal(tc.expected, result)
		})
	}
}
