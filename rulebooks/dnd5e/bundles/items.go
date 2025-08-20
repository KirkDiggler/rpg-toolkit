package bundles

// Item represents a basic item that can be in a bundle
type Item struct {
	ID     string
	Name   string
	Weight float64 // in pounds
	Cost   string  // e.g., "2 gp"
}

// Common adventuring gear items used in bundles
var adventuringGear = map[string]Item{
	// Ammunition
	"arrow": {
		ID:     "arrow",
		Name:   "Arrow",
		Weight: 0.05,
		Cost:   "1 cp",
	},
	"crossbow-bolt": {
		ID:     "crossbow-bolt",
		Name:   "Crossbow Bolt",
		Weight: 0.075,
		Cost:   "1 cp",
	},

	// Exploration gear
	"bedroll": {
		ID:     "bedroll",
		Name:   "Bedroll",
		Weight: 7,
		Cost:   "1 gp",
	},
	"blanket": {
		ID:     "blanket",
		Name:   "Blanket",
		Weight: 3,
		Cost:   "5 sp",
	},
	"candle": {
		ID:     "candle",
		Name:   "Candle",
		Weight: 0,
		Cost:   "1 cp",
	},
	"tinderbox": {
		ID:     "tinderbox",
		Name:   "Tinderbox",
		Weight: 1,
		Cost:   "5 sp",
	},
	"torch": {
		ID:     "torch",
		Name:   "Torch",
		Weight: 1,
		Cost:   "1 cp",
	},
	"hempen-rope": {
		ID:     "hempen-rope",
		Name:   "Hempen Rope (50 feet)",
		Weight: 10,
		Cost:   "1 gp",
	},
	"silk-rope": {
		ID:     "silk-rope",
		Name:   "Silk Rope (50 feet)",
		Weight: 5,
		Cost:   "10 gp",
	},
	"rations": {
		ID:     "rations",
		Name:   "Rations (1 day)",
		Weight: 2,
		Cost:   "5 sp",
	},
	"waterskin": {
		ID:     "waterskin",
		Name:   "Waterskin",
		Weight: 5,
		Cost:   "2 sp",
	},
	"backpack": {
		ID:     "backpack",
		Name:   "Backpack",
		Weight: 5,
		Cost:   "2 gp",
	},
	"mess-kit": {
		ID:     "mess-kit",
		Name:   "Mess Kit",
		Weight: 1,
		Cost:   "2 sp",
	},

	// Tools and utility
	"crowbar": {
		ID:     "crowbar",
		Name:   "Crowbar",
		Weight: 5,
		Cost:   "2 gp",
	},
	"hammer": {
		ID:     "hammer",
		Name:   "Hammer",
		Weight: 3,
		Cost:   "1 gp",
	},
	"piton": {
		ID:     "piton",
		Name:   "Piton",
		Weight: 0.25,
		Cost:   "5 cp",
	},
	"grappling-hook": {
		ID:     "grappling-hook",
		Name:   "Grappling Hook",
		Weight: 4,
		Cost:   "2 gp",
	},
	"shovel": {
		ID:     "shovel",
		Name:   "Shovel",
		Weight: 5,
		Cost:   "2 gp",
	},
	"iron-spikes": {
		ID:     "iron-spikes",
		Name:   "Iron Spikes",
		Weight: 0.5,
		Cost:   "1 sp",
	},

	// Light sources
	"lantern-hooded": {
		ID:     "lantern-hooded",
		Name:   "Hooded Lantern",
		Weight: 2,
		Cost:   "5 gp",
	},
	"lantern-bullseye": {
		ID:     "lantern-bullseye",
		Name:   "Bullseye Lantern",
		Weight: 2,
		Cost:   "10 gp",
	},
	"oil-flask": {
		ID:     "oil-flask",
		Name:   "Oil (flask)",
		Weight: 1,
		Cost:   "1 sp",
	},

	// Scholar's supplies
	"ink": {
		ID:     "ink",
		Name:   "Ink (1 ounce bottle)",
		Weight: 0,
		Cost:   "10 gp",
	},
	"ink-pen": {
		ID:     "ink-pen",
		Name:   "Ink Pen",
		Weight: 0,
		Cost:   "2 cp",
	},
	"parchment": {
		ID:     "parchment",
		Name:   "Parchment (one sheet)",
		Weight: 0,
		Cost:   "1 sp",
	},
	"case-map": {
		ID:     "case-map",
		Name:   "Map or Scroll Case",
		Weight: 1,
		Cost:   "1 gp",
	},
	"book": {
		ID:     "book",
		Name:   "Book",
		Weight: 5,
		Cost:   "25 gp",
	},
	"little-bag-of-sand": {
		ID:     "little-bag-of-sand",
		Name:   "Little Bag of Sand",
		Weight: 1,
		Cost:   "1 cp",
	},
	"small-knife": {
		ID:     "small-knife",
		Name:   "Small Knife",
		Weight: 0.25,
		Cost:   "5 sp",
	},

	// Priest's supplies
	"holy-symbol": {
		ID:     "holy-symbol",
		Name:   "Holy Symbol",
		Weight: 1,
		Cost:   "5 gp",
	},
	"prayer-book": {
		ID:     "prayer-book",
		Name:   "Prayer Book",
		Weight: 5,
		Cost:   "25 gp",
	},
	"incense": {
		ID:     "incense",
		Name:   "Incense (1 stick)",
		Weight: 0,
		Cost:   "1 cp",
	},
	"censer": {
		ID:     "censer",
		Name:   "Censer",
		Weight: 1,
		Cost:   "5 gp",
	},
	"vestments": {
		ID:     "vestments",
		Name:   "Vestments",
		Weight: 4,
		Cost:   "1 gp",
	},

	// Burglar's tools
	"ball-bearings": {
		ID:     "ball-bearings",
		Name:   "Ball Bearings (bag of 1,000)",
		Weight: 2,
		Cost:   "1 gp",
	},
	"string": {
		ID:     "string",
		Name:   "String (10 feet)",
		Weight: 0,
		Cost:   "1 cp",
	},
	"bell": {
		ID:     "bell",
		Name:   "Bell",
		Weight: 0,
		Cost:   "1 gp",
	},

	// Entertainer's supplies
	"costume": {
		ID:     "costume",
		Name:   "Costume",
		Weight: 4,
		Cost:   "5 gp",
	},
	"disguise-kit": {
		ID:     "disguise-kit",
		Name:   "Disguise Kit",
		Weight: 3,
		Cost:   "25 gp",
	},

	// Diplomat's supplies
	"sealing-wax": {
		ID:     "sealing-wax",
		Name:   "Sealing Wax",
		Weight: 0,
		Cost:   "5 sp",
	},
	"soap": {
		ID:     "soap",
		Name:   "Soap",
		Weight: 0,
		Cost:   "2 cp",
	},
	"perfume": {
		ID:     "perfume",
		Name:   "Perfume (vial)",
		Weight: 0,
		Cost:   "5 gp",
	},
	"fine-clothes": {
		ID:     "fine-clothes",
		Name:   "Fine Clothes",
		Weight: 6,
		Cost:   "15 gp",
	},
	"signet-ring": {
		ID:     "signet-ring",
		Name:   "Signet Ring",
		Weight: 0,
		Cost:   "5 gp",
	},

	// Containers
	"chest": {
		ID:     "chest",
		Name:   "Chest",
		Weight: 25,
		Cost:   "5 gp",
	},
	"case-crossbow-bolt": {
		ID:     "case-crossbow-bolt",
		Name:   "Case, Crossbow Bolt",
		Weight: 1,
		Cost:   "1 gp",
	},
}
