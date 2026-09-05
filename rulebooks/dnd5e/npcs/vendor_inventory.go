package npcs

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// StockMode describes whether a vendor stock row is limited or always available.
type StockMode string

const (
	// StockModeLimited means a vendor displays a positive finite quantity.
	StockModeLimited StockMode = "limited"

	// StockModeUnlimited means a vendor displays an item as always available.
	StockModeUnlimited StockMode = "unlimited"
)

// Availability describes whether stock is limited or unlimited for display.
type Availability struct {
	Mode     StockMode `json:"mode"`
	Quantity int       `json:"quantity,omitempty"`
}

// VendorInventoryConfig provides consumer-authored vendor stock.
type VendorInventoryConfig struct {
	Entries []StockEntryData
}

// VendorInventoryData is the serializable form of a vendor inventory.
type VendorInventoryData struct {
	Entries []StockEntryData `json:"entries"`

	// Wallet is the vendor's own buying power, checked when Sell pays a
	// player out (rpg-toolkit#1537). Nil means unlimited — every vendor
	// authored before Sell existed, and every vendor that never sets a
	// limit — and is never checked. A set Wallet is enforced the same way
	// a player's is (CanAfford/Sub), refusing ErrInsufficientFunds if it
	// cannot cover a payout. Lives here, not on npc.Data: npc is a
	// separate, dependency-free module, and this is D&D-specific content
	// this package already owns and structures inside npc.Data's opaque
	// Inventory bytes, the same way Entries itself does.
	Wallet *currency.Money `json:"wallet,omitempty"`
}

// StockEntryData is the serializable form of one vendor stock row.
type StockEntryData struct {
	Type         shared.EquipmentType `json:"type"`
	ID           string               `json:"id"`
	Availability Availability         `json:"availability"`

	// PlayerSold marks a row a player's sale created or added to, rather
	// than one the vendor was authored with. Omitted (false) for every row
	// that predates Sell, which is the correct default: authored stock.
	// This package only carries the tag faithfully; display treatment is
	// the client's call.
	PlayerSold bool `json:"player_sold,omitempty"`
}

// VendorInventory is resolved D&D vendor display stock.
type VendorInventory struct {
	entries []StockEntry
	wallet  *currency.Money
}

// StockEntry is one resolved vendor display stock row.
type StockEntry struct {
	equipment    equipment.Equipment
	availability Availability
	playerSold   bool
}

// NewVendorInventory creates vendor display inventory from consumer-authored stock.
func NewVendorInventory(config VendorInventoryConfig) (VendorInventory, error) {
	return loadInventory(&VendorInventoryData{Entries: config.Entries})
}

// LoadVendorInventory turns serialized vendor inventory data into resolved inventory.
func LoadVendorInventory(data *VendorInventoryData) (VendorInventory, error) {
	if data == nil {
		return VendorInventory{}, ErrNoInventory
	}

	return loadInventory(data)
}

// DecrementVendorStock decrements one stock row of a placed vendor's stored
// content by quantity, mutating data.Inventory in place — the runtime
// counterpart to VendorInventoryFromNPCData's read-only resolve, for a
// caller (session.Trade) that needs to actually spend a unit of stock rather
// than just display it.
//
// A StockModeLimited row is decremented, and refused with ErrOutOfStock if
// quantity exceeds what remains. A row decremented to exactly zero is
// REMOVED from the stored entries rather than left at Quantity: 0 — a
// Limited row with a zero or negative quantity is not a value this package's
// own validateAvailability accepts (vendor_test.go's "limited stock without
// quantity" case pins that for authoring), so leaving one behind would write
// data the very next read of this vendor could not unmarshal. A vendor whose
// every row was Limited and is now fully sold out serializes to an empty
// Entries list, which LoadVendorInventory also refuses (ErrNoStock) — a
// known limit of the current shape, not silently papered over here; the
// toolkit's shipped demo vendor always keeps one StockModeUnlimited row, so
// this case is not reachable through it today.
//
// A StockModeUnlimited row is left untouched — decrementing an
// always-available row would be meaningless — and still reports success,
// since the item was available either way. An item that names no stocked
// row at all is also ErrOutOfStock: a vendor that never carried something
// and a vendor that ran out of it are the same refusal from a buyer's side.
func DecrementVendorStock(data *npc.Data, itemType shared.EquipmentType, id string, quantity int) error {
	if data == nil {
		return ErrNoVendorNPC
	}
	if quantity <= 0 {
		return fmt.Errorf("%w: decrement quantity must be positive, got %d", ErrInvalidStockQuantity, quantity)
	}
	if len(data.Inventory) == 0 {
		return ErrNoInventory
	}

	var inventory VendorInventoryData
	if err := json.Unmarshal(data.Inventory, &inventory); err != nil {
		return fmt.Errorf("unmarshal vendor inventory: %w", err)
	}

	for i := range inventory.Entries {
		entry := inventory.Entries[i]
		if entry.Type != itemType || entry.ID != id {
			continue
		}

		switch entry.Availability.Mode {
		case StockModeUnlimited:
			return nil
		case StockModeLimited:
			if entry.Availability.Quantity < quantity {
				return ErrOutOfStock
			}
			remaining := entry.Availability.Quantity - quantity
			if remaining == 0 {
				inventory.Entries = append(inventory.Entries[:i:i], inventory.Entries[i+1:]...)
			} else {
				inventory.Entries[i].Availability.Quantity = remaining
			}

			marshaled, err := json.Marshal(inventory)
			if err != nil {
				return fmt.Errorf("marshal vendor inventory: %w", err)
			}
			data.Inventory = marshaled
			return nil
		default:
			return fmt.Errorf("%w: %q", ErrUnknownStockMode, entry.Availability.Mode)
		}
	}

	return ErrOutOfStock
}

// AddToVendorStock adds quantity units of an item to a placed vendor's
// stored stock, mutating data.Inventory in place — Sell's vendor-side
// primitive (rpg-toolkit#1537), the mirror of DecrementVendorStock.
//
// Incrementing an existing StockModeLimited row leaves its PlayerSold tag
// untouched — only a brand-new row gets tagged, since a row already
// authored (or already player-sold) does not stop being what it was
// because one more unit joined it. A matching StockModeUnlimited row is
// left untouched and still reports success, the same convention
// DecrementVendorStock already applies. An item stocked nowhere gets a
// brand-new StockModeLimited row, quantity = quantity, tagged
// PlayerSold: true — this function has exactly one caller, a completed
// sale, so there is no "false" case to accept as a parameter.
func AddToVendorStock(data *npc.Data, itemType shared.EquipmentType, id string, quantity int) error {
	if data == nil {
		return ErrNoVendorNPC
	}
	if quantity <= 0 {
		return fmt.Errorf("%w: add quantity must be positive, got %d", ErrInvalidStockQuantity, quantity)
	}
	if len(data.Inventory) == 0 {
		return ErrNoInventory
	}

	var inventory VendorInventoryData
	if err := json.Unmarshal(data.Inventory, &inventory); err != nil {
		return fmt.Errorf("unmarshal vendor inventory: %w", err)
	}

	for i := range inventory.Entries {
		entry := &inventory.Entries[i]
		if entry.Type != itemType || entry.ID != id {
			continue
		}

		switch entry.Availability.Mode {
		case StockModeUnlimited:
			return nil
		case StockModeLimited:
			entry.Availability.Quantity += quantity
		default:
			return fmt.Errorf("%w: %q", ErrUnknownStockMode, entry.Availability.Mode)
		}

		marshaled, err := json.Marshal(inventory)
		if err != nil {
			return fmt.Errorf("marshal vendor inventory: %w", err)
		}
		data.Inventory = marshaled
		return nil
	}

	inventory.Entries = append(inventory.Entries, StockEntryData{
		Type:         itemType,
		ID:           id,
		Availability: Availability{Mode: StockModeLimited, Quantity: quantity},
		PlayerSold:   true,
	})

	marshaled, err := json.Marshal(inventory)
	if err != nil {
		return fmt.Errorf("marshal vendor inventory: %w", err)
	}
	data.Inventory = marshaled
	return nil
}

// DebitVendorWallet pays amount out of a placed vendor's own Wallet,
// mutating data.Inventory in place — Sell's other vendor-side primitive
// (rpg-toolkit#1537), checked before AddToVendorStock completes a sale.
//
// A nil Wallet is unlimited buying power and is never checked — this
// includes every vendor authored before Sell existed. A set Wallet is
// enforced the same way a player's is: refused with
// currency.ErrInsufficientFunds (session wraps this in its own
// ErrInsufficientFunds, the same sentinel a player's shortfall already
// uses) if it cannot cover amount.
func DebitVendorWallet(data *npc.Data, amount currency.Money) error {
	if data == nil {
		return ErrNoVendorNPC
	}
	if len(data.Inventory) == 0 {
		return ErrNoInventory
	}

	var inventory VendorInventoryData
	if err := json.Unmarshal(data.Inventory, &inventory); err != nil {
		return fmt.Errorf("unmarshal vendor inventory: %w", err)
	}

	if inventory.Wallet == nil {
		return nil
	}

	remaining, err := inventory.Wallet.Sub(amount)
	if err != nil {
		return err
	}
	inventory.Wallet = &remaining

	marshaled, err := json.Marshal(inventory)
	if err != nil {
		return fmt.Errorf("marshal vendor inventory: %w", err)
	}
	data.Inventory = marshaled
	return nil
}

// Entries returns resolved stock rows.
func (v VendorInventory) Entries() []StockEntry {
	return cloneEntries(v.entries)
}

// Wallet returns the vendor's own buying power, or nil for unlimited. See
// VendorInventoryData.Wallet's own doc for what nil vs. set means.
func (v VendorInventory) Wallet() *currency.Money {
	if v.wallet == nil {
		return nil
	}
	clone := *v.wallet
	return &clone
}

// View returns UI-ready display data for this inventory.
func (v VendorInventory) View() InventoryView {
	views := make([]StockEntryView, 0, len(v.entries))
	for _, entry := range v.entries {
		views = append(views, stockEntryView(entry))
	}

	return InventoryView{Entries: views}
}

// ToData returns a serializable copy of this inventory.
func (v VendorInventory) ToData() *VendorInventoryData {
	entries := make([]StockEntryData, 0, len(v.entries))
	for _, entry := range v.entries {
		entries = append(entries, StockEntryData{
			Type:         entry.equipment.EquipmentType(),
			ID:           entry.equipment.EquipmentID(),
			Availability: normalizeAvailability(entry.availability),
			PlayerSold:   entry.playerSold,
		})
	}

	return &VendorInventoryData{Entries: entries, Wallet: v.Wallet()}
}

// Equipment returns the resolved D&D equipment for this stock row.
func (s StockEntry) Equipment() equipment.Equipment {
	return s.equipment
}

// Availability returns this stock row's display availability.
func (s StockEntry) Availability() Availability {
	return s.availability
}

// PlayerSold reports whether a player's sale created or added to this row,
// rather than the vendor being authored with it.
func (s StockEntry) PlayerSold() bool {
	return s.playerSold
}

func loadInventory(data *VendorInventoryData) (VendorInventory, error) {
	if len(data.Entries) == 0 {
		return VendorInventory{}, ErrNoStock
	}

	entries := make([]StockEntry, 0, len(data.Entries))
	for i, stock := range data.Entries {
		if err := validateStockData(stock); err != nil {
			return VendorInventory{}, fmt.Errorf("stock entry %d: %w", i, err)
		}

		item, err := resolveStockEquipment(stock)
		if err != nil {
			return VendorInventory{}, fmt.Errorf("stock entry %d: %w", i, err)
		}

		entries = append(entries, StockEntry{
			equipment:    item,
			availability: normalizeAvailability(stock.Availability),
			playerSold:   stock.PlayerSold,
		})
	}

	var wallet *currency.Money
	if data.Wallet != nil {
		clone := *data.Wallet
		wallet = &clone
	}

	return VendorInventory{entries: entries, wallet: wallet}, nil
}

func validateStockData(data StockEntryData) error {
	if data.Type == "" {
		return ErrNoEquipmentType
	}
	if data.ID == "" {
		return ErrNoEquipmentID
	}
	if err := validateAvailability(data.Availability); err != nil {
		return err
	}

	return nil
}

func validateAvailability(availability Availability) error {
	switch availability.Mode {
	case "":
		return ErrNoStockMode
	case StockModeLimited:
		if availability.Quantity <= 0 {
			return ErrInvalidStockQuantity
		}
		return nil
	case StockModeUnlimited:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrUnknownStockMode, availability.Mode)
	}
}

func normalizeAvailability(availability Availability) Availability {
	if availability.Mode == StockModeUnlimited {
		return Availability{Mode: StockModeUnlimited}
	}

	return availability
}
