// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
)

// loot_delivery.go is the session's half of "a looted item lands in the
// looter's inventory" (rpg-project#850, #1169). The encounter takes an item
// off a body and hands it back as an [encounter.TakenItem]; it knows no rules
// for what is inside the author's block. The session reads the block with the
// rules ([magicitems.Compile]), makes the item with the site's id for it, and puts
// it on the looter's sheet. Nothing is put on: the item is in the inventory
// and the player wears it with [Manager.Equip].

// siteItemID is the id a site's custom item has on a character's sheet: the
// author's item id, a '-', and the first eight hex digits of a SHA-256 of the
// site key and the author's id, joined by a NUL, which neither can hold.
//
// It is the same for every session and every body of one site, differs for
// another site's item of the same id and for another item, and is made only
// of letters, digits, '-' and '_' (the rules' own rule for an item id). That
// is what lets a character own at most one of a custom item: a second one
// has the id of the first, so the sheet can tell it is the same item
// (rpg-project#850, #1177). An empty site key is allowed and is its own site.
//
// NOTHING IS MIGRATED. A sheet that holds an item under the id this function
// used to make, from a run and a body, keeps it: it is simply another item.
func siteItemID(siteKey, itemID string) string {
	sum := sha256.Sum256([]byte(siteKey + "\x00" + itemID))
	return itemID + "-" + hex.EncodeToString(sum[:])[:8]
}

// leaveFor is the ids of the site items the looter already owns, read from
// the sheet alone: every owned item whose description names a site item
// ("site", "items", <x>) and whose id is [siteItemID] of this site and <x>.
// Those are the items the looter leaves on the body, and the sheet is all it
// takes to know them, with no knowledge of the body. A nil sheet owns nothing.
func leaveFor(siteKey string, sheet *character.Data) []string {
	if sheet == nil {
		return nil
	}
	var leave []string
	for _, item := range sheet.Items {
		ref := item.Description.Ref
		if ref.Module != "site" || ref.Type != "items" {
			continue
		}
		if item.ID == siteItemID(siteKey, ref.ID) {
			leave = append(leave, ref.ID)
		}
	}
	return leave
}

// deliverTaken puts every taken item on the looter's sheet, in the order the
// encounter gave them. It changes the sheet in memory and nothing else: when
// any item cannot be delivered it returns an error naming that item and the
// caller saves nothing.
//
// Errors: [ErrUnknownContent] when the rules refuse the author's block (the
// rules' sentence names the item and the word), or the rulebook's own error
// when the item cannot be made or added.
func deliverTaken(siteKey, body string, sheet *character.Data, taken []encounter.TakenItem) error {
	for _, t := range taken {
		compiled, err := magicitems.Compile(&magicitems.CompileInput{ID: t.Grant.ID, Authored: t.Grant.Item})
		if err != nil {
			return fmt.Errorf("item %q taken from %q: %w: %v", t.Grant.ID, body, ErrUnknownContent, err)
		}
		item, err := magicitems.New(siteItemID(siteKey, t.Grant.ID), compiled.Description)
		if err != nil {
			return fmt.Errorf("item %q taken from %q: %w", t.Grant.ID, body, err)
		}
		if err := character.AddItem(sheet, item); err != nil {
			return fmt.Errorf("item %q taken from %q: %w", t.Grant.ID, body, err)
		}
	}
	return nil
}
