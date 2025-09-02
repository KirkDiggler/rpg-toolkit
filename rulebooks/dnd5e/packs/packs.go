// Package packs provides D&D 5e equipment pack definitions
package packs

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// PackID represents unique identifier for equipment packs (alias of shared.EquipmentID)
type PackID = shared.EquipmentID

// Standard equipment packs
const (
	BurglarPack     PackID = "burglar-pack"
	DiplomatPack    PackID = "diplomat-pack"
	DungeoneerPack  PackID = "dungeoneer-pack"
	EntertainerPack PackID = "entertainer-pack"
	ExplorerPack    PackID = "explorer-pack"
	PriestPack      PackID = "priest-pack"
	ScholarPack     PackID = "scholar-pack"
)

// PackItem represents an item contained in a pack
type PackItem struct {
	ItemID   string // The equipment ID
	Quantity int    // How many of this item
}

// Pack represents an equipment pack
type Pack struct {
	ID          PackID
	Name        string
	Cost        string // e.g., "12 gp"
	Weight      float32
	Contents    []PackItem // What's inside the pack
	Description string
}

// EquipmentID returns the unique identifier for this pack
func (p *Pack) EquipmentID() string {
	return string(p.ID)
}

// EquipmentType returns the equipment type (always TypePack)
func (p *Pack) EquipmentType() shared.EquipmentType {
	return shared.EquipmentTypePack
}

// EquipmentName returns the name of the pack
func (p *Pack) EquipmentName() string {
	return p.Name
}

// EquipmentWeight returns the weight in pounds
func (p *Pack) EquipmentWeight() float32 {
	return p.Weight
}

// EquipmentValue returns the value in copper pieces
func (p *Pack) EquipmentValue() int {
	// TODO: Parse cost string (e.g., "12 gp") and convert to copper
	// For now, return a placeholder
	return 0
}

// EquipmentDescription returns a description of the pack
func (p *Pack) EquipmentDescription() string {
	if p.Description != "" {
		return p.Description
	}
	// Generate description from contents
	return fmt.Sprintf("A pack containing %d types of items", len(p.Contents))
}

// All pack definitions
var All = map[PackID]Pack{
	BurglarPack: {
		ID:     BurglarPack,
		Name:   "Burglar's Pack",
		Cost:   "16 gp",
		Weight: 46.5,
		Contents: []PackItem{
			{ItemID: "backpack", Quantity: 1},
			{ItemID: "ball-bearings", Quantity: 1000},
			{ItemID: "string", Quantity: 10}, // 10 feet
			{ItemID: "bell", Quantity: 1},
			{ItemID: "candle", Quantity: 5},
			{ItemID: "crowbar", Quantity: 1},
			{ItemID: "hammer", Quantity: 1},
			{ItemID: "piton", Quantity: 10},
			{ItemID: "hooded-lantern", Quantity: 1},
			{ItemID: "oil", Quantity: 2},
			{ItemID: "rations", Quantity: 5},
			{ItemID: "tinderbox", Quantity: 1},
			{ItemID: "waterskin", Quantity: 1},
			{ItemID: "hempen-rope", Quantity: 50}, // 50 feet
		},
		Description: "Equipment for breaking and entering",
	},
	DiplomatPack: {
		ID:     DiplomatPack,
		Name:   "Diplomat's Pack",
		Cost:   "39 gp",
		Weight: 46,
		Contents: []PackItem{
			{ItemID: "chest", Quantity: 1},
			{ItemID: "case-map", Quantity: 2},
			{ItemID: "fine-clothes", Quantity: 1},
			{ItemID: "ink", Quantity: 1},
			{ItemID: "ink-pen", Quantity: 1},
			{ItemID: "lamp", Quantity: 1},
			{ItemID: "oil", Quantity: 2},
			{ItemID: "paper", Quantity: 5},
			{ItemID: "perfume", Quantity: 1},
			{ItemID: "sealing-wax", Quantity: 1},
			{ItemID: "soap", Quantity: 1},
		},
		Description: "Equipment for diplomacy and negotiation",
	},
	DungeoneerPack: {
		ID:     DungeoneerPack,
		Name:   "Dungeoneer's Pack",
		Cost:   "12 gp",
		Weight: 61.5,
		Contents: []PackItem{
			{ItemID: "backpack", Quantity: 1},
			{ItemID: "crowbar", Quantity: 1},
			{ItemID: "hammer", Quantity: 1},
			{ItemID: "piton", Quantity: 10},
			{ItemID: "torch", Quantity: 10},
			{ItemID: "tinderbox", Quantity: 1},
			{ItemID: "rations", Quantity: 10},
			{ItemID: "waterskin", Quantity: 1},
			{ItemID: "hempen-rope", Quantity: 50}, // 50 feet
		},
		Description: "Equipment for dungeon exploration",
	},
	EntertainerPack: {
		ID:     EntertainerPack,
		Name:   "Entertainer's Pack",
		Cost:   "40 gp",
		Weight: 38,
		Contents: []PackItem{
			{ItemID: "backpack", Quantity: 1},
			{ItemID: "bedroll", Quantity: 1},
			{ItemID: "costume", Quantity: 2},
			{ItemID: "candle", Quantity: 5},
			{ItemID: "rations", Quantity: 5},
			{ItemID: "waterskin", Quantity: 1},
			{ItemID: "disguise-kit", Quantity: 1},
		},
		Description: "Equipment for performers and entertainers",
	},
	ExplorerPack: {
		ID:     ExplorerPack,
		Name:   "Explorer's Pack",
		Cost:   "10 gp",
		Weight: 59,
		Contents: []PackItem{
			{ItemID: "backpack", Quantity: 1},
			{ItemID: "bedroll", Quantity: 1},
			{ItemID: "mess-kit", Quantity: 1},
			{ItemID: "tinderbox", Quantity: 1},
			{ItemID: "torch", Quantity: 10},
			{ItemID: "rations", Quantity: 10},
			{ItemID: "waterskin", Quantity: 1},
			{ItemID: "hempen-rope", Quantity: 50}, // 50 feet
		},
		Description: "Equipment for wilderness exploration",
	},
	PriestPack: {
		ID:     PriestPack,
		Name:   "Priest's Pack",
		Cost:   "19 gp",
		Weight: 24,
		Contents: []PackItem{
			{ItemID: "backpack", Quantity: 1},
			{ItemID: "blanket", Quantity: 1},
			{ItemID: "candle", Quantity: 10},
			{ItemID: "tinderbox", Quantity: 1},
			{ItemID: "alms-box", Quantity: 1},
			{ItemID: "incense", Quantity: 2},
			{ItemID: "censer", Quantity: 1},
			{ItemID: "vestments", Quantity: 1},
			{ItemID: "rations", Quantity: 2},
			{ItemID: "waterskin", Quantity: 1},
		},
		Description: "Equipment for religious ceremonies",
	},
	ScholarPack: {
		ID:     ScholarPack,
		Name:   "Scholar's Pack",
		Cost:   "40 gp",
		Weight: 10,
		Contents: []PackItem{
			{ItemID: "backpack", Quantity: 1},
			{ItemID: "book-lore", Quantity: 1},
			{ItemID: "ink", Quantity: 1},
			{ItemID: "ink-pen", Quantity: 1},
			{ItemID: "parchment", Quantity: 10},
			{ItemID: "bag-sand", Quantity: 1},
			{ItemID: "small-knife", Quantity: 1},
		},
		Description: "Equipment for research and study",
	},
}

// GetByID returns a pack by its ID
func GetByID(id PackID) (Pack, error) {
	pack, ok := All[id]
	if !ok {
		validPacks := make([]string, 0, len(All))
		for k := range All {
			validPacks = append(validPacks, string(k))
		}
		return Pack{}, rpgerr.New(rpgerr.CodeInvalidArgument, "invalid pack",
			rpgerr.WithMeta("provided", string(id)),
			rpgerr.WithMeta("valid_options", validPacks))
	}
	return pack, nil
}
