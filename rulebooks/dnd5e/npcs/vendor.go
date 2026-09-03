package npcs

import (
	"encoding/json"
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
//
// Inventory is a convenience projection, not a second stored copy
// (rpg-toolkit#1444): the one value this package actually keeps is the
// opaque bytes inside NPC.Inventory (npc.Data's own field — see its doc).
// ToData recomputes this field fresh from those bytes on every call, so
// there is nothing here that can drift from NPC.Inventory by construction,
// only a structured view of the same data for a caller who wants it without
// unmarshaling npc.Data.Inventory by hand.
type VendorData struct {
	NPC       *npc.Data            `json:"npc"`
	Inventory *VendorInventoryData `json:"inventory"`
}

// Vendor is a D&D NPC role that exposes display inventory.
//
// Holds only the generic npc.NPC (rpg-toolkit#1444) — inventory is not a
// second stored field. A vendor's stock lives entirely inside npc.NPC's own
// opaque Inventory bytes, the same way its ref, name, and capabilities do,
// so it survives anywhere a *npc.Data already travels (session.PlaceNPC,
// in particular) with no separate plumbing. Inventory() recomputes the
// resolved runtime view from those bytes on every call rather than caching
// a second copy that could drift from them.
type Vendor struct {
	npc *npc.NPC
}

// NewVendor creates a D&D vendor from consumer-authored NPC and inventory data.
func NewVendor(config VendorConfig) (*Vendor, error) {
	config.NPC.Capabilities = withVendorCapability(config.NPC.Capabilities)

	inventory, err := NewVendorInventory(config.Inventory)
	if err != nil {
		return nil, err
	}

	marshaled, err := json.Marshal(inventory.ToData())
	if err != nil {
		return nil, fmt.Errorf("marshal vendor inventory: %w", err)
	}
	config.NPC.Inventory = marshaled

	base, err := npc.New(config.NPC)
	if err != nil {
		return nil, err
	}

	return &Vendor{npc: base}, nil
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

	inventory, err := LoadVendorInventory(data.Inventory)
	if err != nil {
		return nil, err
	}

	marshaled, err := json.Marshal(inventory.ToData())
	if err != nil {
		return nil, fmt.Errorf("marshal vendor inventory: %w", err)
	}

	npcData := npcDataWithVendorCapability(data.NPC)
	npcData.Inventory = marshaled

	base, err := npc.Load(npcData)
	if err != nil {
		return nil, err
	}

	return &Vendor{npc: base}, nil
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

// Inventory returns the vendor's display inventory, resolved fresh from the
// opaque bytes npc.NPC carries (rpg-toolkit#1444) — the single source of
// truth for this vendor's stock, not a separately cached copy. A resolve
// failure here would mean the bytes this same package wrote at construction
// or load time no longer unmarshal — an internal defect rather than a
// caller mistake — so it fails the same defensive way NPC() already does:
// reporting the zero value rather than panicking or returning an error this
// method was never shaped to carry.
func (v *Vendor) Inventory() VendorInventory {
	if v == nil || v.npc == nil {
		return VendorInventory{}
	}

	inventory, err := inventoryFromNPC(v.npc)
	if err != nil {
		return VendorInventory{}
	}

	return inventory
}

// View returns UI-ready display data for this vendor.
func (v *Vendor) View() VendorView {
	if v == nil || v.npc == nil {
		return VendorView{}
	}

	return VendorView{
		Ref:         cloneRef(v.npc.Ref()),
		DisplayName: v.npc.DisplayName(),
		Inventory:   v.Inventory().View(),
	}
}

// ToData returns a serializable copy of this vendor. Inventory is a
// convenience projection recomputed from npc.NPC's own opaque bytes — see
// [VendorData]'s own doc for why this cannot drift from NPC.Inventory.
func (v *Vendor) ToData() *VendorData {
	if v == nil || v.npc == nil {
		return nil
	}

	return &VendorData{
		NPC:       v.npc.ToData(),
		Inventory: v.Inventory().ToData(),
	}
}

// inventoryFromNPC unmarshals and resolves the opaque inventory bytes an
// npc.NPC carries. Returns ErrNoInventory if the NPC carries none — a
// vendor's underlying npc.NPC always should, since both NewVendor and
// LoadVendor set it before this package ever returns a *Vendor.
func inventoryFromNPC(n *npc.NPC) (VendorInventory, error) {
	raw := n.Inventory()
	if raw == nil {
		return VendorInventory{}, ErrNoInventory
	}

	var data VendorInventoryData
	if err := json.Unmarshal(raw, &data); err != nil {
		return VendorInventory{}, fmt.Errorf("unmarshal vendor inventory: %w", err)
	}

	return LoadVendorInventory(&data)
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
