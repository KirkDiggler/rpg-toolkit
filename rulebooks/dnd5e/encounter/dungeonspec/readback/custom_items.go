// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"
	"sort"
)

// custom_items.go says the site's custom items (rpg-project#850,
// rpg-project#1166): one statement per declared item, one under a creature
// that has items equipped, one under a prop that is an item.
//
// IT SAYS ONLY WHAT IS TRUE IN THIS VERSION. Looting a creature's body gives
// its equipped items to the looter (rpg-project#1167); nothing takes a prop
// yet, so no sentence here says taking a prop gives an item.
// What an item lets its wearer do is written in its own words and judged by the
// rules when a run starts; the read-back says its id and name.

// Kinds of the custom-item statements.
const (
	// KindItem is a declared item, about {item, <id>}, at `customItems.<id>`.
	KindItem Kind = "place.item"
	// KindEquipment is a creature's equipped items, about the creature, at its
	// binding's `.equipment`.
	KindEquipment Kind = "creature.equipment"
	// KindPropItem is a prop that is an item, about the prop, at its binding's
	// `.item`.
	KindPropItem Kind = "place.prop.item"
)

// ConnItem is the kind of a connection to a declared item: {item, <id>}.
const ConnItem = "item"

// RoleEquips is the role of a creature's connection to an item it has equipped.
const RoleEquips = "equips"

// RoleIsItem is the role of a prop's connection to the item it is.
const RoleIsItem = "is-item"

// itemName is a declared item's name, or its id when it has none.
func (b *builder) itemName(id string) string {
	if it, ok := b.spec.CustomItems[id]; ok {
		if name, ok := it["name"].(string); ok && name != "" {
			return name
		}
	}
	return id
}

// itemStatements is one statement per declared item, in id order.
func (b *builder) itemStatements() []Statement {
	ids := make([]string, 0, len(b.spec.CustomItems))
	for id := range b.spec.CustomItems {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Statement, 0, len(ids))
	for _, id := range ids {
		s := fmt.Sprintf("The custom item %s, named %s. What it lets its wearer do is written in its own words under %s, "+
			"and the rules judge them when a run starts.", quote(id), said(b.itemName(id)), quote("customItems."+id))
		out = append(out, b.fstmt(KindItem, Subject{Kind: ConnItem, ID: id}, "customItems."+id, s,
			foldInfo{site: "the custom item " + quote(id)}))
	}
	return out
}

// equipmentStatement says the items a creature has equipped, by name, or
// reports false when it has none.
func (b *builder) equipmentStatement(name string, about Subject, path string, equipment []string) (Statement, bool) {
	if len(equipment) == 0 {
		return Statement{}, false
	}
	var names []string
	var conns []Connection
	for _, id := range equipment {
		names = append(names, said(b.itemName(id)))
		conns = append(conns, Connection{Kind: ConnItem, ID: id, Role: RoleEquips})
	}
	list := joinAnd(names)
	return b.fstmt(KindEquipment, about, path+".equipment",
		fmt.Sprintf("%s has %s equipped. Looting its body gives %s to the looter.", name, list, list),
		foldInfo{text: "{Self} has " + list + " equipped. Looting its body gives " + list + " to the looter."}, conns...), true
}

// propItemStatement says that a prop is an item, by name, or reports false
// when the prop is none.
func (b *builder) propItemStatement(name string, about Subject, path, item string) (Statement, bool) {
	if item == "" {
		return Statement{}, false
	}
	return b.fstmt(KindPropItem, about, path+".item",
		fmt.Sprintf("The prop %s is the item %s.", name, said(b.itemName(item))),
		foldInfo{site: "the prop " + name + " as an item"},
		Connection{Kind: ConnItem, ID: item, Role: RoleIsItem}), true
}
