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
// rules ([magicitems.Compile]), makes the item with an id of its own, and puts
// it on the looter's sheet. Nothing is put on: the item is in the inventory
// and the player wears it with [Manager.Equip].

// lootedItemID is the id the looted item has on the looter's sheet: the
// author's item id, a '-', and the first eight hex digits of a SHA-256 of the
// session, the body and the author's id, joined by a NUL, which none of the
// three can hold.
//
// It is the same when asked twice, differs for another body and for another
// session, and is made only of letters, digits, '-' and '_' (the rules'
// own rule for an item id), so one player looting two bodies that wore the
// same cloak owns two cloaks, and so does another run.
func lootedItemID(sessionID, body, itemID string) string {
	sum := sha256.Sum256([]byte(sessionID + "\x00" + body + "\x00" + itemID))
	return itemID + "-" + hex.EncodeToString(sum[:])[:8]
}

// deliverTaken puts every taken item on the looter's sheet, in the order the
// encounter gave them. It changes the sheet in memory and nothing else: when
// any item cannot be delivered it returns an error naming that item and the
// caller saves nothing.
//
// Errors: [ErrUnknownContent] when the rules refuse the author's block (the
// rules' sentence names the item and the word), or the rulebook's own error
// when the item cannot be made or added.
func deliverTaken(sessionID, body string, sheet *character.Data, taken []encounter.TakenItem) error {
	for _, t := range taken {
		compiled, err := magicitems.Compile(&magicitems.CompileInput{ID: t.Grant.ID, Authored: t.Grant.Item})
		if err != nil {
			return fmt.Errorf("item %q taken from %q: %w: %v", t.Grant.ID, body, ErrUnknownContent, err)
		}
		item, err := magicitems.New(lootedItemID(sessionID, body, t.Grant.ID), compiled.Description)
		if err != nil {
			return fmt.Errorf("item %q taken from %q: %w", t.Grant.ID, body, err)
		}
		if err := character.AddItem(sheet, item); err != nil {
			return fmt.Errorf("item %q taken from %q: %w", t.Grant.ID, body, err)
		}
	}
	return nil
}
