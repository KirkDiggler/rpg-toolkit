package npcs

import (
	"fmt"

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
