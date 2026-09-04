package npcs

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/npc"
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
}

// StockEntryData is the serializable form of one vendor stock row.
type StockEntryData struct {
	Type         shared.EquipmentType `json:"type"`
	ID           string               `json:"id"`
	Availability Availability         `json:"availability"`
}

// VendorInventory is resolved D&D vendor display stock.
type VendorInventory struct {
	entries []StockEntry
}

// StockEntry is one resolved vendor display stock row.
type StockEntry struct {
	equipment    equipment.Equipment
	availability Availability
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
// quantity exceeds what remains. A StockModeUnlimited row is left
// untouched — decrementing an always-available row would be meaningless —
// and still reports success, since the item was available either way. An
// item that names no stocked row at all is also ErrOutOfStock: a vendor that
// never carried something and a vendor that ran out of it are the same
// refusal from a buyer's side.
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
		entry := &inventory.Entries[i]
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
			entry.Availability.Quantity -= quantity

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

// Entries returns resolved stock rows.
func (v VendorInventory) Entries() []StockEntry {
	return cloneEntries(v.entries)
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
		})
	}

	return &VendorInventoryData{Entries: entries}
}

// Equipment returns the resolved D&D equipment for this stock row.
func (s StockEntry) Equipment() equipment.Equipment {
	return s.equipment
}

// Availability returns this stock row's display availability.
func (s StockEntry) Availability() Availability {
	return s.availability
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
		})
	}

	return VendorInventory{entries: entries}, nil
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
