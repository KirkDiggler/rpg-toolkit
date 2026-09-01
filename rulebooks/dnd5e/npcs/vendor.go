package npcs

import (
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/npc"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
)

// VendorConfig provides consumer-authored D&D vendor values.
type VendorConfig struct {
	NPC       npc.Config
	Inventory VendorInventoryConfig
}

// VendorData is the serializable D&D vendor role/profile.
type VendorData struct {
	NPC       *npc.Data            `json:"npc"`
	Inventory *VendorInventoryData `json:"inventory"`
}

// Vendor is a D&D NPC role that exposes display inventory.
type Vendor struct {
	npc       *npc.NPC
	inventory VendorInventory
}

// NewVendor creates a D&D vendor from consumer-authored NPC and inventory data.
func NewVendor(config VendorConfig) (*Vendor, error) {
	config.NPC.Capabilities = withVendorCapability(config.NPC.Capabilities)

	base, err := npc.New(config.NPC)
	if err != nil {
		return nil, err
	}

	inventory, err := NewVendorInventory(config.Inventory)
	if err != nil {
		return nil, err
	}

	return &Vendor{npc: base, inventory: inventory}, nil
}

// LoadVendor turns serialized D&D vendor data into a vendor.
func LoadVendor(data *VendorData) (*Vendor, error) {
	if data == nil {
		return nil, ErrNoVendorData
	}
	if data.NPC == nil {
		return nil, ErrNoVendorNPC
	}
	if data.Inventory == nil {
		return nil, ErrNoInventory
	}

	base, err := npc.Load(npcDataWithVendorCapability(data.NPC))
	if err != nil {
		return nil, err
	}

	inventory, err := LoadVendorInventory(data.Inventory)
	if err != nil {
		return nil, err
	}

	return &Vendor{npc: base, inventory: inventory}, nil
}

// NPC returns the generic NPC content for this vendor.
func (v *Vendor) NPC() *npc.NPC {
	if v == nil || v.npc == nil {
		return nil
	}

	clone, err := npc.Load(v.npc.ToData())
	if err != nil {
		return nil
	}

	return clone
}

// Inventory returns the vendor's display inventory.
func (v *Vendor) Inventory() VendorInventory {
	if v == nil {
		return VendorInventory{}
	}

	return v.inventory.clone()
}

// View returns UI-ready display data for this vendor.
func (v *Vendor) View() VendorView {
	if v == nil || v.npc == nil {
		return VendorView{}
	}

	return VendorView{
		Ref:         cloneRef(v.npc.Ref()),
		DisplayName: v.npc.DisplayName(),
		Inventory:   v.inventory.View(),
	}
}

// ToData returns a serializable copy of this vendor.
func (v *Vendor) ToData() *VendorData {
	if v == nil || v.npc == nil {
		return nil
	}

	return &VendorData{
		NPC:       v.npc.ToData(),
		Inventory: v.inventory.ToData(),
	}
}

// VendorView is structured display data for rendering a vendor.
type VendorView struct {
	Ref         *core.Ref     `json:"ref"`
	DisplayName string        `json:"display_name"`
	Inventory   InventoryView `json:"inventory"`
}

// InventoryView is structured display data for a vendor inventory.
type InventoryView struct {
	Entries []StockEntryView `json:"entries"`
}

// StockEntryView is structured display data for one vendor stock row.
type StockEntryView struct {
	Type     shared.EquipmentType `json:"type"`
	ID       string               `json:"id"`
	Name     string               `json:"name"`
	Mode     StockMode            `json:"mode"`
	Quantity int                  `json:"quantity,omitempty"`
}

func stockEntryView(entry StockEntry) StockEntryView {
	view := StockEntryView{
		Type: entry.equipment.EquipmentType(),
		ID:   entry.equipment.EquipmentID(),
		Name: entry.equipment.EquipmentName(),
		Mode: entry.availability.Mode,
	}
	if entry.availability.Mode == StockModeLimited {
		view.Quantity = entry.availability.Quantity
	}

	return view
}

func cloneEntries(entries []StockEntry) []StockEntry {
	out := make([]StockEntry, len(entries))
	copy(out, entries)
	return out
}

func withVendorCapability(capabilities []npc.Capability) []npc.Capability {
	out := slices.Clone(capabilities)
	if !slices.Contains(out, npc.CapabilityVendor) {
		out = append(out, npc.CapabilityVendor)
	}

	return out
}

func npcDataWithVendorCapability(data *npc.Data) *npc.Data {
	if data == nil {
		return nil
	}

	copied := *data
	copied.Capabilities = withVendorCapability(data.Capabilities)
	return &copied
}

func cloneRef(ref *core.Ref) *core.Ref {
	if ref == nil {
		return nil
	}

	return &core.Ref{Module: ref.Module, Type: ref.Type, ID: ref.ID}
}

func resolveStockEquipment(data StockEntryData) (equipment.Equipment, error) {
	item, err := equipment.GetByID(shared.SelectionID(data.ID))
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrEquipmentNotFound, data.ID)
	}
	if item.EquipmentType() != data.Type {
		return nil, fmt.Errorf("%w: got %q want %q", ErrEquipmentTypeMismatch, item.EquipmentType(), data.Type)
	}

	return item, nil
}
