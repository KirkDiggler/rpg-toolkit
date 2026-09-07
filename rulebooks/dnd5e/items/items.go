// Package items provides D&D 5e miscellaneous items and equipment
package items

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// ItemID represents unique identifier for items (alias of shared.EquipmentID)
type ItemID = shared.EquipmentID

// Spellcasting focuses
const (
	ComponentPouch ItemID = "component-pouch"
	ArcaneFocus    ItemID = "arcane-focus"
	DruidicFocus   ItemID = "druidic-focus"
	HolySymbol     ItemID = "holy-symbol"
	Spellbook      ItemID = "spellbook"
)

// Adventuring gear
const (
	Backpack        ItemID = "backpack"
	Bedroll         ItemID = "bedroll"
	Blanket         ItemID = "blanket"
	ClothesCommon   ItemID = "clothes-common"
	ClothesTraveler ItemID = "clothes-traveler"
	Crowbar         ItemID = "crowbar"
	Hammer          ItemID = "hammer"
	HempenRope      ItemID = "hempen-rope"
	HuntingTrap     ItemID = "hunting-trap"
	Lantern         ItemID = "lantern"
	Mess            ItemID = "mess-kit"
	Oil             ItemID = "oil"
	Piton           ItemID = "piton"
	Rations         ItemID = "rations"
	SilkRope        ItemID = "silk-rope"
	Tinderbox       ItemID = "tinderbox"
	Torch           ItemID = "torch"
	Waterskin       ItemID = "waterskin"
)

// Pack sundries with an official PHB/SRD price (Adventuring Gear table),
// surfaced by rpg-toolkit#1544's Unpack: every one of these IDs is already
// referenced by a starting pack's Contents (packs.go), verified directly
// against every pack's official published contents list, not assumed.
const (
	BallBearings ItemID = "ball-bearings"
	Bell         ItemID = "bell"
	BookOfLore   ItemID = "book-lore"
	CaseMap      ItemID = "case-map"
	Chest        ItemID = "chest"
	Costume      ItemID = "costume"
	Candle       ItemID = "candle"
	FineClothes  ItemID = "fine-clothes"
	Ink          ItemID = "ink"
	InkPen       ItemID = "ink-pen"
	Lamp         ItemID = "lamp"
	Paper        ItemID = "paper"
	Parchment    ItemID = "parchment"
	Perfume      ItemID = "perfume"
	SealingWax   ItemID = "sealing-wax"
	Soap         ItemID = "soap"
)

// Pack sundries with NO official PHB/SRD individual price — the PHB
// describes each only as pack flavor text ("an alms box," "a censer,"
// "vestments," "2 blocks of incense," "a little bag of sand," "a small
// knife," "10 feet of string"), never as its own Adventuring Gear table
// row. ESTIMATED, not sourced: each is priced by comparison to a similar
// already-costed item (vestments ≈ fine clothes' 15 gp/6 lb; censer ≈ a
// holy symbol's 5 gp/1 lb), the same reasoning a documented third-party
// fill-in already used to close this exact gap. Distinct on purpose from
// the sourced block above — a caller has no way to tell them apart from
// the ID alone, which is why this comment exists.
const (
	AlmsBox    ItemID = "alms-box"
	BagOfSand  ItemID = "bag-sand"
	Censer     ItemID = "censer"
	Incense    ItemID = "incense"
	SmallKnife ItemID = "small-knife"
	String     ItemID = "string"
	Vestments  ItemID = "vestments"
)

// Item represents a miscellaneous item with basic stats.
type Item struct {
	ID     ItemID
	Name   string
	Weight float64
	Cost   string
}

// EquipmentID returns the unique identifier for this item.
func (i *Item) EquipmentID() string {
	return i.ID
}

// EquipmentType returns the equipment type (always TypeItem).
func (i *Item) EquipmentType() shared.EquipmentType {
	return shared.EquipmentTypeItem
}

// EquipmentCategories reports the shared.EquipmentCategory tags a category
// choice can query this item by. Only the three focus/symbol items are
// tagged — each is really "always this one specific item," not a category
// with multiple members, but expressing it as a category keeps a Cleric's
// or Druid's equipment choice the same shape as every other category
// choice rather than needing a distinct "always this fixed item" case.
func (i *Item) EquipmentCategories() []shared.EquipmentCategory {
	switch i.ID {
	case DruidicFocus:
		return []shared.EquipmentCategory{shared.CategoryDruidicFoci}
	case HolySymbol:
		return []shared.EquipmentCategory{shared.CategoryHolySymbols}
	case ArcaneFocus:
		return []shared.EquipmentCategory{shared.CategoryArcaneFoci}
	default:
		return nil
	}
}

// EquipmentName returns the display name of the item.
func (i *Item) EquipmentName() string {
	return i.Name
}

// EquipmentWeight returns the weight in pounds.
func (i *Item) EquipmentWeight() float32 {
	return float32(i.Weight)
}

// EquipmentValue returns the value in copper pieces.
func (i *Item) EquipmentValue() int {
	// TODO: Parse cost string and convert to copper
	return 0
}

// EquipmentDescription returns a description of the item.
func (i *Item) EquipmentDescription() string {
	return i.Name
}

// All maps item IDs to their definitions.
var All = map[ItemID]Item{
	ComponentPouch: {ID: ComponentPouch, Name: "Component Pouch", Weight: 2, Cost: "25 gp"},
	ArcaneFocus:    {ID: ArcaneFocus, Name: "Arcane Focus", Weight: 1, Cost: "10 gp"},
	DruidicFocus:   {ID: DruidicFocus, Name: "Druidic Focus", Weight: 0, Cost: "1 gp"},
	HolySymbol:     {ID: HolySymbol, Name: "Holy Symbol", Weight: 0, Cost: "5 gp"},
	Spellbook:      {ID: Spellbook, Name: "Spellbook", Weight: 3, Cost: "50 gp"},

	// Adventuring gear (PHB Chapter 5 equipment table / SRD 5.1, cross-checked
	// against two independent SRD mirrors rather than transcribed from memory).
	// Every one of these IDs is already the exact string a starting pack's
	// Contents references (packs.go) — verified directly, not assumed — so
	// populating this map is what makes ResolveEquipmentDetail (and the
	// PriceOf built on top of it) actually resolve pack-granted gear.
	Backpack:        {ID: Backpack, Name: "Backpack", Weight: 5, Cost: "2 gp"},
	Bedroll:         {ID: Bedroll, Name: "Bedroll", Weight: 7, Cost: "1 gp"},
	Blanket:         {ID: Blanket, Name: "Blanket", Weight: 3, Cost: "5 sp"},
	ClothesCommon:   {ID: ClothesCommon, Name: "Clothes, Common", Weight: 3, Cost: "5 sp"},
	ClothesTraveler: {ID: ClothesTraveler, Name: "Clothes, Traveler's", Weight: 4, Cost: "2 gp"},
	Crowbar:         {ID: Crowbar, Name: "Crowbar", Weight: 5, Cost: "2 gp"},
	Hammer:          {ID: Hammer, Name: "Hammer", Weight: 3, Cost: "1 gp"},
	HempenRope:      {ID: HempenRope, Name: "Hempen Rope (50 feet)", Weight: 10, Cost: "1 gp"},
	HuntingTrap:     {ID: HuntingTrap, Name: "Hunting Trap", Weight: 25, Cost: "5 gp"},
	Lantern:         {ID: Lantern, Name: "Lantern, Hooded", Weight: 2, Cost: "5 gp"},
	Mess:            {ID: Mess, Name: "Mess Kit", Weight: 1, Cost: "2 sp"},
	Oil:             {ID: Oil, Name: "Oil (Flask)", Weight: 1, Cost: "1 sp"},
	Piton:           {ID: Piton, Name: "Piton", Weight: 0.25, Cost: "5 cp"},
	Rations:         {ID: Rations, Name: "Rations (1 Day)", Weight: 2, Cost: "5 sp"},
	SilkRope:        {ID: SilkRope, Name: "Rope, Silk (50 feet)", Weight: 5, Cost: "10 gp"},
	Tinderbox:       {ID: Tinderbox, Name: "Tinderbox", Weight: 1, Cost: "5 sp"},
	Torch:           {ID: Torch, Name: "Torch", Weight: 1, Cost: "1 cp"},
	Waterskin:       {ID: Waterskin, Name: "Waterskin", Weight: 5, Cost: "2 sp"},

	// Pack sundries, official PHB/SRD price (Adventuring Gear table), same
	// two-source cross-check as the adventuring gear above.
	BallBearings: {ID: BallBearings, Name: "Ball Bearings (1,000)", Weight: 2, Cost: "1 gp"},
	Bell:         {ID: Bell, Name: "Bell", Weight: 0, Cost: "1 gp"},
	BookOfLore:   {ID: BookOfLore, Name: "Book of Lore", Weight: 5, Cost: "25 gp"},
	CaseMap:      {ID: CaseMap, Name: "Case, Map or Scroll", Weight: 1, Cost: "1 gp"},
	Chest:        {ID: Chest, Name: "Chest", Weight: 25, Cost: "5 gp"},
	Costume:      {ID: Costume, Name: "Costume", Weight: 4, Cost: "5 gp"},
	Candle:       {ID: Candle, Name: "Candle", Weight: 0, Cost: "1 cp"},
	FineClothes:  {ID: FineClothes, Name: "Fine Clothes", Weight: 6, Cost: "15 gp"},
	Ink:          {ID: Ink, Name: "Ink (1 Ounce Bottle)", Weight: 0, Cost: "10 gp"},
	InkPen:       {ID: InkPen, Name: "Ink Pen", Weight: 0, Cost: "2 cp"},
	Lamp:         {ID: Lamp, Name: "Lamp", Weight: 1, Cost: "5 sp"},
	Paper:        {ID: Paper, Name: "Paper (One Sheet)", Weight: 0, Cost: "2 sp"},
	Parchment:    {ID: Parchment, Name: "Parchment (One Sheet)", Weight: 0, Cost: "1 sp"},
	Perfume:      {ID: Perfume, Name: "Perfume (Vial)", Weight: 0, Cost: "5 gp"},
	SealingWax:   {ID: SealingWax, Name: "Sealing Wax", Weight: 0, Cost: "5 sp"},
	Soap:         {ID: Soap, Name: "Soap", Weight: 0, Cost: "2 cp"},

	// Pack sundries, ESTIMATED — no official PHB/SRD individual price
	// exists (see this block's own const doc for why and how). Do not
	// treat these Cost/Weight values as sourced the way everything above
	// this comment is.
	AlmsBox:    {ID: AlmsBox, Name: "Alms Box", Weight: 1, Cost: "1 gp"},
	BagOfSand:  {ID: BagOfSand, Name: "Bag of Sand", Weight: 1, Cost: "1 gp"},
	Censer:     {ID: Censer, Name: "Censer", Weight: 1, Cost: "5 gp"},
	Incense:    {ID: Incense, Name: "Incense (Block)", Weight: 0, Cost: "5 sp"},
	SmallKnife: {ID: SmallKnife, Name: "Small Knife", Weight: 0, Cost: "2 gp"},
	String:     {ID: String, Name: "String (10 Feet)", Weight: 0, Cost: "1 cp"},
	Vestments:  {ID: Vestments, Name: "Vestments", Weight: 6, Cost: "15 gp"},
}

// categorizedItems lists, in declaration order, the items that carry an
// EquipmentCategories() tag. Most items never do; this package has no
// full registry-order slice the way tools/weapons/armor do, so this stays
// a short explicit list rather than iterating all of All for the rare
// case a category actually applies.
var categorizedItems = []ItemID{ArcaneFocus, DruidicFocus, HolySymbol}

// EligibleForCategory returns every item tagged with the given
// shared.EquipmentCategory (the public vocabulary equipment.GetByCategory
// callers use). Checks membership via Item.EquipmentCategories rather than
// hardcoding which category maps to which item here.
func EligibleForCategory(cat shared.EquipmentCategory) []Item {
	var result []Item
	for _, id := range categorizedItems {
		item := All[id]
		for _, c := range item.EquipmentCategories() {
			if c == cat {
				result = append(result, item)
				break
			}
		}
	}
	return result
}
