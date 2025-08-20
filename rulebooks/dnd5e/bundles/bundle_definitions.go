package bundles

import (
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/choices"
)

// Bundle represents a collection of items bundled together
type Bundle struct {
	ID          BundleID
	Name        string
	Description string
	Items       []choices.CountedItem
}

// bundleDefinitions contains all the predefined bundles
var bundleDefinitions = map[BundleID]Bundle{
	// Explorer's Pack (39 gp)
	ExplorersPack: {
		ID:          ExplorersPack,
		Name:        "Explorer's Pack",
		Description: "Includes a backpack, bedroll, mess kit, tinderbox, torches, rations, waterskin, and rope",
		Items: []choices.CountedItem{
			{ItemType: choices.ItemTypeGear, ItemID: "backpack", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "bedroll", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "mess-kit", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "tinderbox", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "torch", Quantity: 10},
			{ItemType: choices.ItemTypeGear, ItemID: "rations", Quantity: 10},
			{ItemType: choices.ItemTypeGear, ItemID: "waterskin", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "hempen-rope", Quantity: 1},
		},
	},

	// Dungeoneer's Pack (12 gp)
	DungeoneersPack: {
		ID:          DungeoneersPack,
		Name:        "Dungeoneer's Pack",
		Description: "Includes a backpack, crowbar, hammer, pitons, torches, tinderbox, rations, waterskin, and rope",
		Items: []choices.CountedItem{
			{ItemType: choices.ItemTypeGear, ItemID: "backpack", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "crowbar", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "hammer", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "piton", Quantity: 10},
			{ItemType: choices.ItemTypeGear, ItemID: "torch", Quantity: 10},
			{ItemType: choices.ItemTypeGear, ItemID: "tinderbox", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "rations", Quantity: 10},
			{ItemType: choices.ItemTypeGear, ItemID: "waterskin", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "hempen-rope", Quantity: 1},
		},
	},

	// Burglar's Pack (16 gp)
	BurglarsPack: {
		ID:          BurglarsPack,
		Name:        "Burglar's Pack",
		Description: "Includes a backpack, ball bearings, string, bell, candles, crowbar, hammer, pitons, hooded lantern, oil, rations, tinderbox, waterskin, and rope",
		Items: []choices.CountedItem{
			{ItemType: choices.ItemTypeGear, ItemID: "backpack", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "ball-bearings", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "string", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "bell", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "candle", Quantity: 5},
			{ItemType: choices.ItemTypeGear, ItemID: "crowbar", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "hammer", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "piton", Quantity: 10},
			{ItemType: choices.ItemTypeGear, ItemID: "lantern-hooded", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "oil-flask", Quantity: 2},
			{ItemType: choices.ItemTypeGear, ItemID: "rations", Quantity: 5},
			{ItemType: choices.ItemTypeGear, ItemID: "tinderbox", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "waterskin", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "hempen-rope", Quantity: 1},
		},
	},

	// Entertainer's Pack (40 gp)
	EntertainersPack: {
		ID:          EntertainersPack,
		Name:        "Entertainer's Pack",
		Description: "Includes a backpack, bedroll, costumes, candles, rations, waterskin, and disguise kit",
		Items: []choices.CountedItem{
			{ItemType: choices.ItemTypeGear, ItemID: "backpack", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "bedroll", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "costume", Quantity: 2},
			{ItemType: choices.ItemTypeGear, ItemID: "candle", Quantity: 5},
			{ItemType: choices.ItemTypeGear, ItemID: "rations", Quantity: 5},
			{ItemType: choices.ItemTypeGear, ItemID: "waterskin", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "disguise-kit", Quantity: 1},
		},
	},

	// Diplomat's Pack (39 gp)
	DiplomatsPack: {
		ID:          DiplomatsPack,
		Name:        "Diplomat's Pack",
		Description: "Includes a chest, cases, fine clothes, ink, pen, lamp, oil, paper, perfume, sealing wax, and soap",
		Items: []choices.CountedItem{
			{ItemType: choices.ItemTypeGear, ItemID: "chest", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "case-map", Quantity: 2},
			{ItemType: choices.ItemTypeGear, ItemID: "fine-clothes", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "ink", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "ink-pen", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "lantern-bullseye", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "oil-flask", Quantity: 2},
			{ItemType: choices.ItemTypeGear, ItemID: "parchment", Quantity: 5},
			{ItemType: choices.ItemTypeGear, ItemID: "perfume", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "sealing-wax", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "soap", Quantity: 1},
		},
	},

	// Scholar's Pack (40 gp)
	ScholarsPack: {
		ID:          ScholarsPack,
		Name:        "Scholar's Pack",
		Description: "Includes a backpack, book of lore, ink, pen, parchment, little bag of sand, and small knife",
		Items: []choices.CountedItem{
			{ItemType: choices.ItemTypeGear, ItemID: "backpack", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "book", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "ink", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "ink-pen", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "parchment", Quantity: 10},
			{ItemType: choices.ItemTypeGear, ItemID: "little-bag-of-sand", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "small-knife", Quantity: 1},
		},
	},

	// Priest's Pack (19 gp)
	PriestsPack: {
		ID:          PriestsPack,
		Name:        "Priest's Pack",
		Description: "Includes a backpack, blanket, candles, tinderbox, alms box, incense, censer, vestments, rations, and waterskin",
		Items: []choices.CountedItem{
			{ItemType: choices.ItemTypeGear, ItemID: "backpack", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "blanket", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "candle", Quantity: 10},
			{ItemType: choices.ItemTypeGear, ItemID: "tinderbox", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "incense", Quantity: 2},
			{ItemType: choices.ItemTypeGear, ItemID: "censer", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "vestments", Quantity: 1},
			{ItemType: choices.ItemTypeGear, ItemID: "rations", Quantity: 2},
			{ItemType: choices.ItemTypeGear, ItemID: "waterskin", Quantity: 1},
		},
	},
}

// GetBundle returns the bundle definition for the given ID
func GetBundle(id BundleID) (*Bundle, error) {
	bundle, ok := bundleDefinitions[id]
	if !ok {
		validBundles := make([]string, 0, len(bundleDefinitions))
		for k := range bundleDefinitions {
			validBundles = append(validBundles, string(k))
		}
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "invalid bundle ID",
			rpgerr.WithMeta("provided", string(id)),
			rpgerr.WithMeta("valid_options", validBundles))
	}
	return &bundle, nil
}

// GetBundleItems returns just the items from a bundle
func GetBundleItems(id BundleID) ([]choices.CountedItem, error) {
	bundle, err := GetBundle(id)
	if err != nil {
		return nil, err
	}
	return bundle.Items, nil
}

// ListAllBundles returns all available bundle IDs
func ListAllBundles() []BundleID {
	bundles := make([]BundleID, 0, len(bundleDefinitions))
	for id := range bundleDefinitions {
		bundles = append(bundles, id)
	}
	return bundles
}

// ValidateBundleID checks if a bundle ID exists
func ValidateBundleID(id BundleID) bool {
	_, exists := bundleDefinitions[id]
	return exists
}
