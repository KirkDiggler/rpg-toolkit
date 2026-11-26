// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package magicitems is the data of an item a character owns and keeps its
// own counts for: a description copied onto the character with the item, and
// the item's own record of what is left in each pool it carries.
//
// It is a leaf. It knows the rulebook's named abilities (to refuse a grant the
// rulebook cannot describe) and nothing of the sheet that holds an item, the
// slots a sheet reads as weapons and armour, or any gate that asks an item to
// pay.
package magicitems

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
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

// Pool is one count the item keeps for itself.
type Pool struct {
	// Maximum is how many the pool holds when full.
	Maximum int `json:"maximum"`
	// RestoreOn names the rests that fill it. Empty never comes back.
	RestoreOn []coreResources.ResetType `json:"restore_on,omitempty"`
}

// Grant is one ability the item grants, with the values the item supplies.
type Grant struct {
	// Ability is a named rulebook ability.
	Ability core.Ref `json:"ability"`
	// Parameters are its values, for example {"level": 1}.
	Parameters json.RawMessage `json:"parameters"`
	// Pool is which of the item's pools pays.
	Pool string `json:"pool,omitempty"`
	// Uses is how many it costs.
	Uses int `json:"uses,omitempty"`
}

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

// Clone returns a copy that shares no map, slice or byte with the original.
func (d Description) Clone() Description {
	out := d
	out.Slots = slices.Clone(d.Slots)
	if d.Pools != nil {
		out.Pools = make(map[string]Pool, len(d.Pools))
		for name, pool := range d.Pools {
			pool.RestoreOn = slices.Clone(pool.RestoreOn)
			out.Pools[name] = pool
		}
	}
	if d.Grants != nil {
		out.Grants = make(map[string]Grant, len(d.Grants))
		for name, grant := range d.Grants {
			grant.Parameters = slices.Clone(grant.Parameters)
			out.Grants[name] = grant
		}
	}
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

	for _, name := range slices.Sorted(maps.Keys(d.Pools)) {
		if !validName(name) {
			return fmt.Errorf("item %q pool name %q must be letters, digits, '-' and '_'", d.Name, name)
		}
		if err := validatePool(d.Name, name, d.Pools[name]); err != nil {
			return err
		}
	}
	for _, name := range slices.Sorted(maps.Keys(d.Grants)) {
		if !validName(name) {
			return fmt.Errorf("item %q grant name %q must be letters, digits, '-' and '_'", d.Name, name)
		}
		if err := d.validateGrant(name, d.Grants[name]); err != nil {
			return err
		}
	}
	return nil
}

func validatePool(item, name string, pool Pool) error {
	if pool.Maximum < 1 {
		return fmt.Errorf("item %q pool %q maximum %d must be at least 1", item, name, pool.Maximum)
	}
	seen := make(map[coreResources.ResetType]struct{}, len(pool.RestoreOn))
	for _, rest := range pool.RestoreOn {
		if rest != coreResources.ResetShortRest && rest != coreResources.ResetLongRest {
			return fmt.Errorf("item %q pool %q restore_on %q must be short_rest or long_rest", item, name, rest)
		}
		if _, dup := seen[rest]; dup {
			return fmt.Errorf("item %q pool %q restore_on names %q more than once", item, name, rest)
		}
		seen[rest] = struct{}{}
	}
	return nil
}

func (d Description) validateGrant(name string, grant Grant) error {
	if err := features.ValidateParameters(&features.ValidateParametersInput{
		Ref:        grant.Ability,
		Parameters: grant.Parameters,
	}); err != nil {
		return fmt.Errorf("item %q grant %q: %w", d.Name, name, err)
	}
	if grant.Pool == "" {
		if grant.Uses != 0 {
			return fmt.Errorf("item %q grant %q has uses %d but names no pool to pay from", d.Name, name, grant.Uses)
		}
		return nil
	}
	if _, ok := d.Pools[grant.Pool]; !ok {
		return fmt.Errorf("item %q grant %q names pool %q, which the item does not declare", d.Name, name, grant.Pool)
	}
	if grant.Uses < 1 {
		return fmt.Errorf("item %q grant %q pays from pool %q but its uses %d is below 1", d.Name, name, grant.Pool, grant.Uses)
	}
	return nil
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
