// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package magicitems is the data of an item a character owns and keeps its
// own counts for: a description copied onto the character with the item, and
// the item's own record of what is left in each pool it carries.
//
// An item holds a [kits.Kit], the counts and what they let its wearer use, and
// keeps its own fields for them so that its saved shape is what it always was.
//
// It is a leaf. It knows the rulebook's kit (which knows the named abilities)
// and nothing of the sheet that holds an item, the slots a sheet reads as
// weapons and armour, or any gate that asks an item to pay.
package magicitems

import (
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/kits"
)

// Description is the whole of what an item is, as an author writes it.
type Description struct {
	// Ref is which item this is a copy of.
	Ref core.Ref `json:"ref"`
	// Name is what a player sees, "Lieutenant's cloak".
	Name string `json:"name"`
	// Slots are where it may be worn, as sheet slot names.
	Slots []string `json:"slots"`
	// Unique says a character owns at most one of this item. Absent reads as
	// false: an item owned in number.
	Unique bool `json:"unique,omitempty"`
	// Pools are its own counts, by name.
	Pools map[string]Pool `json:"pools,omitempty"`
	// Grants are what it grants while worn, by name.
	Grants map[string]Grant `json:"grants"`
}

// Pool is one count the item keeps for itself. It is the kit's.
type Pool = kits.Pool

// Grant is one ability the item grants, with the values the item supplies. It
// is the kit's.
type Grant = kits.Grant

// Data is one item a character owns: its description and its own record.
type Data struct {
	// ID is this item, not its kind.
	ID string `json:"id"`
	// Description is copied onto the character with the item.
	Description Description `json:"description"`
	// Left is what is left in each pool.
	Left map[string]int `json:"left,omitempty"`
}

// New returns an item with every pool full. The description is copied.
func New(id string, description Description) (Data, error) {
	d := Data{ID: id, Description: description.Clone()}
	if len(d.Description.Pools) > 0 {
		d.Left = make(map[string]int, len(d.Description.Pools))
		for name, pool := range d.Description.Pools {
			d.Left[name] = pool.Maximum
		}
	}
	if err := d.Validate(); err != nil {
		return Data{}, err
	}
	return d, nil
}

// Kit returns the item's counts and what they let its wearer use, as a copy:
// changing it changes nothing in the description.
func (d Description) Kit() kits.Kit {
	return kits.Kit{Pools: d.Pools, Grants: d.Grants}.Clone()
}

// Clone returns a copy that shares no map, slice or byte with the original.
func (d Description) Clone() Description {
	out := d
	out.Slots = slices.Clone(d.Slots)
	k := d.Kit()
	out.Pools = k.Pools
	out.Grants = k.Grants
	return out
}

// Clone returns a copy that shares no map, slice or byte with the original.
func (d Data) Clone() Data {
	out := d
	out.Description = d.Description.Clone()
	out.Left = maps.Clone(d.Left)
	return out
}

// Validate refuses a description the rulebook cannot run, naming the thing.
func (d Description) Validate() error {
	if err := d.Ref.IsValid(); err != nil {
		return fmt.Errorf("item ref %q is invalid: %w", d.Ref.String(), err)
	}
	if d.Name == "" {
		return fmt.Errorf("item %s has an empty name", d.Ref.String())
	}
	if len(d.Slots) == 0 {
		return fmt.Errorf("item %q names no slot it may be worn in", d.Name)
	}
	seen := make(map[string]struct{}, len(d.Slots))
	for _, slot := range d.Slots {
		if slot == "" {
			return fmt.Errorf("item %q names an empty slot", d.Name)
		}
		if _, dup := seen[slot]; dup {
			return fmt.Errorf("item %q names slot %q more than once", d.Name, slot)
		}
		seen[slot] = struct{}{}
	}
	if len(d.Grants) == 0 {
		return fmt.Errorf("item %q grants nothing: it names no grant", d.Name)
	}
	return kits.Kit{Pools: d.Pools, Grants: d.Grants}.Validate(fmt.Sprintf("item %q", d.Name))
}

// Validate refuses an item whose description or record cannot stand.
func (d Data) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("item id is empty")
	}
	if err := d.Description.Validate(); err != nil {
		return fmt.Errorf("item %q: %w", d.ID, err)
	}
	for name := range d.Left {
		if _, ok := d.Description.Pools[name]; !ok {
			return fmt.Errorf("item %q keeps a count for pool %q, which its description does not declare", d.ID, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(d.Description.Pools)) {
		left, ok := d.Left[name]
		if !ok {
			return fmt.Errorf("item %q keeps no count for pool %q", d.ID, name)
		}
		if limit := d.Description.Pools[name].Maximum; left < 0 || left > limit {
			return fmt.Errorf("item %q pool %q holds %d, outside 0 to %d", d.ID, name, left, limit)
		}
	}
	return nil
}

func validName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
