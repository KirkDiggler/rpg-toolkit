// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// custom_items.go is A CUSTOM ITEM AN AUTHOR WROTE (rpg-project#850,
// rpg-project#1166): the root's `customItems:`, a creature's `equipment:` and
// a prop's `item:`.
//
//	customItems:
//	  lieutenants-cloak:
//	    name: Lieutenant's cloak
//	    slot: cloak
//	    charges: { max: 1, refills: { on: [short rest, long rest] } }
//	    lets:
//	      - use: second wind
//	        with: { level: 1 }
//	        cost: { charges: 1 }
//
// THIS FILE JUDGES THE SHAPE, NOT THE WORDS INSIDE IT, as statuses.go does.
// The four top-level words are settled; which slots exist, what `charges`
// holds and which abilities `lets` may name are the rules' own table, which
// this module may not import. So an item is refused here for a missing or
// misspelt top-level word and for an empty list, and carried as canonical JSON
// in the author's words; the rules judge what is inside when a run starts.
// NOTHING IN A RUN USES THE WORDS YET: looting a body for its equipment and
// taking an item prop are the next slice.

// CustomItemSpec is one authored custom item, kept in the author's words:
// `name`, `slot`, `lets` and the optional `charges`. Only those keys and their
// shape are judged here.
type CustomItemSpec map[string]any

// The words an item is written with, in the order a refusal lists them.
const (
	itemName    = "name"
	itemSlot    = "slot"
	itemCharges = "charges"
	itemLets    = "lets"
)

var itemWords = []string{itemName, itemSlot, itemCharges, itemLets}

// plainItem is an item with every map in it a plain map[string]any. YAML
// decodes the maps nested inside a named map type as that same named type, and
// [unwritable] knows only the plain one and a status's; an item is read here as
// the plain shape it is.
func plainItem(it CustomItemSpec) map[string]any {
	out, _ := plainValue(map[string]any(it)).(map[string]any)
	return out
}

// plainValue copies v with every [CustomItemSpec] in it as a map[string]any.
func plainValue(v any) any {
	switch x := v.(type) {
	case CustomItemSpec:
		return plainValue(map[string]any(x))
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = plainValue(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = plainValue(val)
		}
		return out
	default:
		return v
	}
}

// sortedItemIDs is every declared item id, in order.
func sortedItemIDs(items map[string]CustomItemSpec) []string {
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// itemIDWritable reports whether an id is non-empty and made of letters,
// digits, `-` and `_`.
func itemIDWritable(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

// customItemsDeclared judges the shape of every item under `customItems:` at
// its path and returns the ids a creature's `equipment:` and a prop's `item:`
// may name: every declared one, a refused one included, so a misshapen item is
// reported once, at itself, and not again at everything that names it.
func customItemsDeclared(items map[string]CustomItemSpec, add errSink) map[string]bool {
	declared := make(map[string]bool, len(items))
	for _, id := range sortedItemIDs(items) {
		at := "customItems." + id
		declared[id] = true
		if !itemIDWritable(id) {
			add(at, fmt.Sprintf("%q is not a custom item id: write letters, digits, - and _", id))
			continue
		}
		if items[id] == nil {
			add(at, "a custom item is written with "+strings.Join(itemWords, ", "))
			continue
		}
		it := plainItem(items[id])
		for _, key := range sortedKeys(it) {
			if !containsWord(itemWords, key) {
				add(at+"."+key, fmt.Sprintf("%q is not a word of a custom item: a custom item is written with %s",
					key, strings.Join(itemWords, ", ")))
			}
		}
		if name, ok := it[itemName].(string); !ok || strings.TrimSpace(name) == "" {
			add(at+"."+itemName, "a custom item needs a name: the words a player sees")
		}
		if slot, ok := it[itemSlot].(string); !ok || strings.TrimSpace(slot) == "" {
			add(at+"."+itemSlot, "a custom item needs a slot: where it is worn")
		}
		if lets, ok := it[itemLets].([]any); !ok || len(lets) == 0 {
			add(at+"."+itemLets, "a custom item needs lets as a non-empty list: what it lets its wearer do")
		}
		if charges, written := it[itemCharges]; written {
			if _, ok := charges.(map[string]any); !ok {
				add(at+"."+itemCharges, "charges, when written, is a mapping: how many uses the item has and when they come back")
			}
		}
		if where, value, bad := unwritable(it, ""); bad {
			add(at, fmt.Sprintf("the value %s at %s cannot be written: a custom item holds only text, whole numbers, "+
				"yes/no, and lists and maps of those", value, where))
		}
	}
	return declared
}

// customItemsOf compiles every declared item to canonical JSON — keys sorted
// at every depth — so an item hashes the same however its YAML was laid out.
// Empty when the site declares none.
//
// FAILS CLOSED: an item that cannot be written is an error naming it, never an
// item quietly dropped.
func customItemsOf(items map[string]CustomItemSpec) (map[string]json.RawMessage, error) {
	out := make(map[string]json.RawMessage, len(items))
	for _, id := range sortedItemIDs(items) {
		it := plainItem(items[id])
		if where, value, bad := unwritable(it, ""); bad {
			return nil, &itemError{id: id, message: fmt.Sprintf("the value %s at %s cannot be written", value, where)}
		}
		raw, err := json.Marshal(it)
		if err != nil {
			return nil, &itemError{id: id, message: "it cannot be written: " + err.Error()}
		}
		out[id] = raw
	}
	return out, nil
}

// itemError is a declared item the compiler cannot carry, at its path.
type itemError struct{ id, message string }

func (e *itemError) Error() string { return "customItems." + e.id + ": " + e.message }

// itemGrantOf is one item as the encounter carries it: the id and the item's
// canonical JSON, copied so no two holders share bytes.
//
// NEVER NIL FOR A NAMED ITEM: an item the site does not carry (refused by the
// grammar first) still compiles to a grant with no block, and the encounter
// refuses that grant when it is built.
func itemGrantOf(id string, items map[string]json.RawMessage) encounter.ItemGrant {
	return encounter.ItemGrant{ID: id, Item: append(json.RawMessage(nil), items[id]...)}
}

// equipmentOf is the grants a creature's `equipment:` names, in the author's
// order. Nil when it names none, so a creature with none compiles as it did
// before this word.
func equipmentOf(ids []string, items map[string]json.RawMessage) []encounter.ItemGrant {
	if len(ids) == 0 {
		return nil
	}
	out := make([]encounter.ItemGrant, len(ids))
	for i, id := range ids {
		out[i] = itemGrantOf(id, items)
	}
	return out
}

// declaredItemList is the declared ids as a refusal lists them.
func declaredItemList(declared map[string]bool) string {
	if len(declared) == 0 {
		return "this site declares no custom items"
	}
	ids := make([]string, 0, len(declared))
	for id := range declared {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return "the custom items declared are " + strings.Join(ids, ", ")
}

// bindingEquipment refuses a creature that has an item equipped this site does
// not declare, or one twice — [bindingHolds]' shape for the item an
// `equipment:` entry names, at the entry's own path.
func bindingEquipment(g *grammar, path, id string, equipment []string, declared map[string]bool) {
	seen := make(map[string]bool, len(equipment))
	for j, item := range equipment {
		at := fmt.Sprintf("%s.equipment[%d]", path, j)
		switch {
		case !declared[item]:
			g.fail(at, "%q has the item %q equipped, and no custom item in this site has that id (%s)",
				id, item, declaredItemList(declared))
		case seen[item]:
			g.fail(at, "%q has the item %q equipped twice: an item is equipped once", id, item)
		}
		seen[item] = true
	}
}

// bindingItem refuses a prop that is an item this site does not declare, and
// one that is also holdable or holds intel: a thing is held in the hands or
// taken as an item, not both, and what an item teaches is not built.
func bindingItem(g *grammar, path, id string, b RoomPropBinding, declared map[string]bool) {
	if b.Item == "" {
		return
	}
	if !declared[b.Item] {
		g.fail(path+".item", "%q is the item %q, and no custom item in this site has that id (%s)",
			id, b.Item, declaredItemList(declared))
	}
	if b.Holdable {
		g.fail(path+".holdable", "%q is an item and is holdable: a thing is held in the hands or taken as an item, "+
			"not both: write item or holdable", id)
	}
	if len(b.Holds) > 0 {
		g.fail(path+".holds", "%q is an item and holds intel: what an item teaches is not built: write item or holds", id)
	}
}
