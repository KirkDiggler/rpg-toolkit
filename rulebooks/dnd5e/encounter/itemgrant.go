// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ItemGrant is one custom item as its author wrote it, carried unread. The
// encounter never reads inside Item; the rules do, when the item is handed over.
//
// It is [StatusGrant]'s model for an item (rpg-project#850): the site format
// judged the SHAPE of the author's block, and what is inside it (which slots
// exist, which abilities an item lets its wearer use) is the rules' table,
// which this module may not import.
type ItemGrant struct {
	// ID is the item's id in its site's `customItems:`; the same id is the
	// same item.
	ID string `json:"id"`

	// Item is the authored block — {name, slot[, charges], lets} — as
	// canonical JSON. The encounter never reads inside it.
	Item json.RawMessage `json:"item"`
}

// validateItemGrant refuses a grant nobody could author on purpose: no id, or
// an item that is not a JSON object. What the object holds is the rules' to
// judge, not this module's.
func validateItemGrant(what string, g ItemGrant) error {
	if g.ID == "" {
		return fmt.Errorf("%s has an item with no id", what)
	}
	trimmed := bytes.TrimSpace(g.Item)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(trimmed) {
		return fmt.Errorf("%s has item %q, whose authored block is not a JSON object", what, g.ID)
	}
	return nil
}

// validateEquipment refuses a member's equipment when any grant is
// malformed ([validateItemGrant]) or one id is named twice: an item is
// equipped once.
func validateEquipment(what string, equipment []ItemGrant) error {
	seen := make(map[string]bool, len(equipment))
	for _, g := range equipment {
		if err := validateItemGrant(what, g); err != nil {
			return err
		}
		if seen[g.ID] {
			return fmt.Errorf("%s has item %q equipped twice", what, g.ID)
		}
		seen[g.ID] = true
	}
	return nil
}

// cloneItemGrant is a grant whose block no caller's slice can reach.
func cloneItemGrant(g ItemGrant) ItemGrant {
	return ItemGrant{ID: g.ID, Item: append(json.RawMessage(nil), g.Item...)}
}

// cloneEquipment is a deep copy of a member's equipment; nil stays nil, so a
// member with none writes nothing.
func cloneEquipment(equipment []ItemGrant) []ItemGrant {
	if len(equipment) == 0 {
		return nil
	}
	out := make([]ItemGrant, len(equipment))
	for i, g := range equipment {
		out[i] = cloneItemGrant(g)
	}
	return out
}

// cloneItem is a deep copy of a prop's item; nil stays nil.
func cloneItem(g *ItemGrant) *ItemGrant {
	if g == nil {
		return nil
	}
	c := cloneItemGrant(*g)
	return &c
}
