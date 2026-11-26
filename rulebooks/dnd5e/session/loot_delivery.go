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
// rules ([magicitems.Compile]), makes the item with its id (the site's for a unique
// item, the run's and the body's for any other), and puts
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
// Only an item whose author says `unique: true` is given this id
// (rpg-project#1182); any other is given [takenItemID].
//
// NOTHING IS MIGRATED. A sheet that holds an item under the id this function
// used to make, from a run and a body, keeps it: it is simply another item.
func siteItemID(siteKey, itemID string) string {
	sum := sha256.Sum256([]byte(siteKey + "\x00" + itemID))
	return itemID + "-" + hex.EncodeToString(sum[:])[:8]
}

// takenItemID is the id of a taken item whose author did not say it is
// unique: the author's item id, a '-', and the first eight hex digits of a
// SHA-256 of the session id, the body and the author's id, joined by NULs,
// which none of them can hold.
//
// It is the same when asked twice, differs for another session, another body
// and another item, and is made only of letters, digits, '-' and '_' (the
// rules' own rule for an item id). So a character who loots two bodies that
// wore the same ring, or the same body in two runs, owns two rings, each its
// own item with its own charges (rpg-project#1182).
func takenItemID(sessionID, body, itemID string) string {
	sum := sha256.Sum256([]byte(sessionID + "\x00" + body + "\x00" + itemID))
	return itemID + "-" + hex.EncodeToString(sum[:])[:8]
}

// leaveFor is the ids of the site items the looter already owns, read from
// the sheet alone: every owned item whose description names a site item
// ("site", "items", <x>) and whose id is [siteItemID] of this site and <x>.
// Those are the items the looter leaves on the body, and the sheet is all it
// takes to know them, with no knowledge of the body. A nil sheet owns nothing.
//
// It does NOT read the description's Unique: only a unique item is given the
// site's id now, so only a unique item is left, and a sheet saved under
// session v0.214.0 holds items under the site's id with no `unique` on their
// copied description, which must still be left so a later loot is not refused.
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
func deliverTaken(sessionID, siteKey, body string, sheet *character.Data, taken []encounter.TakenItem) error {
	for _, t := range taken {
		compiled, err := magicitems.Compile(&magicitems.CompileInput{ID: t.Grant.ID, Authored: t.Grant.Item})
		if err != nil {
			return fmt.Errorf("item %q taken from %q: %w: %v", t.Grant.ID, body, ErrUnknownContent, err)
		}
		id := takenItemID(sessionID, body, t.Grant.ID)
		if compiled.Description.Unique {
			id = siteItemID(siteKey, t.Grant.ID)
		}
		item, err := magicitems.New(id, compiled.Description)
		if err != nil {
			return fmt.Errorf("item %q taken from %q: %w", t.Grant.ID, body, err)
		}
		if err := character.AddItem(sheet, item); err != nil {
			return fmt.Errorf("item %q taken from %q: %w", t.Grant.ID, body, err)
		}
	}
	return nil
}
