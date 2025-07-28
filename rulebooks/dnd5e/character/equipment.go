// Package character provides D&D 5e character creation and management functionality
package character

import "fmt"

// equipmentBundles defines the contents of equipment packs
// These match the standard D&D 5e equipment packs
var equipmentBundles = map[string][]string{
	"Burglar's Pack": {
		"Backpack",
		"Ball Bearings (bag of 1,000)",
		"String (10 feet)",
		"Bell",
		"Candle (5)",
		"Crowbar",
		"Hammer",
		"Piton (10)",
		"Hooded Lantern",
		"Flask of Oil (2)",
		"Rations (5 days)",
		"Tinderbox",
		"Waterskin",
		"Hempen Rope (50 feet)",
	},
	"Diplomat's Pack": {
		"Chest",
		"Case, Map or Scroll (2)",
		"Fine Clothes",
		"Bottle of Ink",
		"Ink Pen",
		"Lamp",
		"Flask of Oil (2)",
		"Paper (5 sheets)",
		"Vial of Perfume",
		"Sealing Wax",
		"Soap",
	},
	"Dungeoneer's Pack": {
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
	"Entertainer's Pack": {
		"Backpack",
		"Bedroll",
		"Costume (2)",
		"Candle (5)",
		"Rations (5 days)",
		"Waterskin",
		"Disguise Kit",
	},
	"Explorer's Pack": {
		"Backpack",
		"Bedroll",
		"Mess Kit",
		"Tinderbox",
		"Torch (10)",
		"Rations (10 days)",
		"Waterskin",
		"Hempen Rope (50 feet)",
	},
	"Priest's Pack": {
		"Backpack",
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
	"Scholar's Pack": {
		"Backpack",
		"Book of Lore",
		"Bottle of Ink",
		"Ink Pen",
		"Parchment (10 sheets)",
		"Little Bag of Sand",
		"Small Knife",
	},
}

// processEquipmentChoices takes equipment choices and expands bundles into individual items
func processEquipmentChoices(choices []string) []string {
	equipment := make([]string, 0)

	for _, item := range choices {
		// Check if this is a bundle/pack
		if bundle, isBundle := equipmentBundles[item]; isBundle {
			// Add all items from the bundle
			equipment = append(equipment, bundle...)
		} else {
			// Add the individual item
			equipment = append(equipment, item)
		}
	}

	return equipment
}

// formatEquipmentWithQuantity formats an equipment item with its quantity
func formatEquipmentWithQuantity(itemID string, quantity int) string {
	if quantity > 1 {
		return fmt.Sprintf("%s (%d)", itemID, quantity)
	}
	return itemID
}
