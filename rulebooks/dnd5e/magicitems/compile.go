// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package magicitems

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/kits"
)

// CompileInput is one item as an author wrote it.
type CompileInput struct {
	// ID is the item's key in the file: "lieutenants-cloak".
	ID string
	// Authored is the item's block as JSON, exactly the author's words.
	Authored json.RawMessage
}

// CompileOutput is the description the engine runs.
type CompileOutput struct {
	Description Description
}

// The author's words.
const (
	wordName    = "name"
	wordSlot    = "slot"
	wordUnique  = "unique"
	wordCharges = "charges"
	wordLets    = "lets"
)

// Compile turns an item written in an author's words into the description
// the engine runs. It is strict: an unknown or missing word or value is
// refused with a sentence naming the item's id and the word.
//
// The item's own words are read here; its `charges` and `lets` are the kit's
// and are compiled by [kits.Compile] under the holder `item "<id>"`. That an
// item must let its wearer use something is the item's rule, so it is kept
// here: an absent or empty `lets` is refused after the kit's own blocks are
// read, in the order the item has always refused them.
func Compile(in *CompileInput) (*CompileOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("compiling an item requires input")
	}
	if !validName(in.ID) {
		return nil, fmt.Errorf("item id %q must be non-empty and made of letters, digits, '-' and '_'", in.ID)
	}
	c := compiler{id: in.ID}
	top, err := c.object(in.Authored, "the item")
	if err != nil {
		return nil, err
	}
	if err := c.allowed(top, "the item", wordsAt(PlaceItem)...); err != nil {
		return nil, err
	}

	d := Description{Ref: core.Ref{Module: "site", Type: "items", ID: in.ID}}
	if d.Name, err = c.name(top); err != nil {
		return nil, err
	}
	if d.Slots, err = c.slot(top); err != nil {
		return nil, err
	}
	if d.Unique, err = c.unique(top); err != nil {
		return nil, err
	}
	compiled, err := kits.Compile(&kits.CompileInput{
		Holder:  fmt.Sprintf("item %q", in.ID),
		Charges: top[wordCharges],
		Lets:    top[wordLets],
	})
	if err != nil {
		return nil, err
	}
	if _, ok := top[wordLets]; !ok {
		return nil, c.refuse("the item has no `lets`; it is a non-empty list of what the item lets its wearer use")
	}
	if len(compiled.Kit.Grants) == 0 {
		return nil, c.refuse("`lets` is empty; the item must let its wearer use something")
	}
	d.Pools, d.Grants = compiled.Kit.Pools, compiled.Kit.Grants
	if err := d.Validate(); err != nil {
		return nil, c.refuse("the item is not one the rulebook can run: %v", err)
	}
	return &CompileOutput{Description: d}, nil
}

type compiler struct {
	id string
}

func (c *compiler) refuse(format string, args ...any) error {
	return fmt.Errorf("item %q: %s", c.id, fmt.Sprintf(format, args...))
}

func (c *compiler) object(raw json.RawMessage, where string) (map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, c.refuse("%s must be an object of words", where)
	}
	var out map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &out); err != nil {
		return nil, c.refuse("%s must be an object of words: %v", where, err)
	}
	return out, nil
}

func (c *compiler) allowed(obj map[string]json.RawMessage, where string, words ...string) error {
	for _, word := range slices.Sorted(maps.Keys(obj)) {
		if !slices.Contains(words, word) {
			return c.refuse("%s: unknown word `%s`; it is one of %s", where, word, strings.Join(words, ", "))
		}
	}
	return nil
}

func (c *compiler) text(raw json.RawMessage, word string) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil || strings.TrimSpace(s) == "" {
		return "", c.refuse("`%s` must be non-empty text", word)
	}
	return s, nil
}

func (c *compiler) name(top map[string]json.RawMessage) (string, error) {
	raw, ok := top[wordName]
	if !ok {
		return "", c.refuse("the item has no `name`")
	}
	return c.text(raw, wordName)
}

// unique reads the optional word: JSON true or false, nothing else.
func (c *compiler) unique(top map[string]json.RawMessage) (bool, error) {
	raw, ok := top[wordUnique]
	if !ok {
		return false, nil
	}
	switch string(bytes.TrimSpace(raw)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	return false, c.refuse("`%s` is true or false", wordUnique)
}

func (c *compiler) slot(top map[string]json.RawMessage) ([]string, error) {
	raw, ok := top[wordSlot]
	if !ok {
		return nil, c.refuse("the item has no `slot`; it is one of %s", strings.Join(valueWords(PlaceItem, wordSlot), ", "))
	}
	var word string
	_ = json.Unmarshal(raw, &word)
	chosen, ok := valueOf(PlaceItem, wordSlot, word)
	if !ok {
		return nil, c.refuse("`slot` is %s; it is one of %s", string(bytes.TrimSpace(raw)), strings.Join(valueWords(PlaceItem, wordSlot), ", "))
	}
	return slices.Clone(chosen.slots), nil
}
