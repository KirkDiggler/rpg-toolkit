// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package magicitems

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
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
	wordMax     = "max"
	wordRefills = "refills"
	wordOn      = "on"
	wordUse     = "use"
	wordWith    = "with"
	wordCost    = "cost"

	chargesPool = "charges"
	maxCharges  = 1000
)

// slotWords are the slots an author names, and the sheet slots each is.
var slotWords = map[string][]string{
	"cloak":  {"cloak"},
	"amulet": {"amulet"},
	"ring":   {"ring_left", "ring_right"},
	"belt":   {"belt"},
	"helm":   {"helm"},
	"boots":  {"boots"},
}

// slotOrder is the order the words are offered in a refusal.
var slotOrder = []string{"cloak", "amulet", "ring", "belt", "helm", "boots"}

// restWords are the rests an author names for a refill.
var restWords = map[string]coreResources.ResetType{
	"short rest": coreResources.ResetShortRest,
	"long rest":  coreResources.ResetLongRest,
}

var restOrder = []string{"short rest", "long rest"}

// Compile turns an item written in an author's words into the description
// the engine runs. It is strict: an unknown or missing word or value is
// refused with a sentence naming the item's id and the word.
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
	if err := c.allowed(top, "the item", wordName, wordSlot, wordUnique, wordCharges, wordLets); err != nil {
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
	if raw, ok := top[wordCharges]; ok {
		pool, err := c.charges(raw)
		if err != nil {
			return nil, err
		}
		d.Pools = map[string]Pool{chargesPool: pool}
	}
	if d.Grants, err = c.lets(top, d.Pools[chargesPool].Maximum); err != nil {
		return nil, err
	}
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

// whole reads a whole number written as digits: 1 passes, 1.5 and 1e0 do not.
func (c *compiler) whole(raw json.RawMessage, word string) (int, error) {
	n, err := strconv.Atoi(string(bytes.TrimSpace(raw)))
	if err != nil {
		return 0, c.refuse("`%s` is %s; it must be a whole number", word, string(bytes.TrimSpace(raw)))
	}
	return n, nil
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
		return nil, c.refuse("the item has no `slot`; it is one of %s", strings.Join(slotOrder, ", "))
	}
	var word string
	_ = json.Unmarshal(raw, &word)
	slots, ok := slotWords[word]
	if !ok {
		return nil, c.refuse("`slot` is %s; it is one of %s", string(bytes.TrimSpace(raw)), strings.Join(slotOrder, ", "))
	}
	return slices.Clone(slots), nil
}

func (c *compiler) charges(raw json.RawMessage) (Pool, error) {
	obj, err := c.object(raw, "`charges`")
	if err != nil {
		return Pool{}, err
	}
	if err := c.allowed(obj, "`charges`", wordMax, wordRefills); err != nil {
		return Pool{}, err
	}
	maxRaw, ok := obj[wordMax]
	if !ok {
		return Pool{}, c.refuse("`charges` has no `max`; it is a whole number from 1 to %d", maxCharges)
	}
	limit, err := c.whole(maxRaw, "charges.max")
	if err != nil {
		return Pool{}, err
	}
	if limit < 1 || limit > maxCharges {
		return Pool{}, c.refuse("`charges.max` is %d; it is a whole number from 1 to %d", limit, maxCharges)
	}
	pool := Pool{Maximum: limit}
	refillsRaw, ok := obj[wordRefills]
	if !ok {
		return pool, nil
	}
	refills, err := c.object(refillsRaw, "`refills`")
	if err != nil {
		return Pool{}, err
	}
	if err := c.allowed(refills, "`refills`", wordOn); err != nil {
		return Pool{}, err
	}
	var on []string
	if err := json.Unmarshal(refills[wordOn], &on); err != nil || len(on) == 0 {
		return Pool{}, c.refuse("`refills.on` must be a non-empty list of rests: %s", strings.Join(restOrder, ", "))
	}
	seen := map[string]bool{}
	for _, word := range on {
		rest, ok := restWords[word]
		if !ok {
			return Pool{}, c.refuse("`refills.on` has %q; it is one of %s", word, strings.Join(restOrder, ", "))
		}
		if seen[word] {
			return Pool{}, c.refuse("`refills.on` names %q more than once", word)
		}
		seen[word] = true
		pool.RestoreOn = append(pool.RestoreOn, rest)
	}
	return pool, nil
}

func (c *compiler) lets(top map[string]json.RawMessage, held int) (map[string]Grant, error) {
	raw, ok := top[wordLets]
	if !ok {
		return nil, c.refuse("the item has no `lets`; it is a non-empty list of what the item lets its wearer use")
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, c.refuse("`lets` must be a list of entries")
	}
	if len(entries) == 0 {
		return nil, c.refuse("`lets` is empty; the item must let its wearer use something")
	}
	grants := make(map[string]Grant, len(entries))
	for i, entry := range entries {
		where := fmt.Sprintf("lets[%d]", i)
		name, grant, err := c.entry(entry, where, held)
		if err != nil {
			return nil, err
		}
		if _, dup := grants[name]; dup {
			return nil, c.refuse("`use` names %q more than once; each entry uses a different ability", strings.ReplaceAll(name, "-", " "))
		}
		grants[name] = grant
	}
	return grants, nil
}

func (c *compiler) entry(raw json.RawMessage, where string, held int) (string, Grant, error) {
	obj, err := c.object(raw, where)
	if err != nil {
		return "", Grant{}, err
	}
	if err := c.allowed(obj, where, wordUse, wordWith, wordCost); err != nil {
		return "", Grant{}, err
	}
	useRaw, ok := obj[wordUse]
	if !ok {
		return "", Grant{}, c.refuse("%s has no `use`; it is one of %s", where, strings.Join(features.NamedAbilities(), ", "))
	}
	var use string
	_ = json.Unmarshal(useRaw, &use)
	ref, ok := features.NamedAbility(use)
	if !ok {
		return "", Grant{}, c.refuse("%s: `use` is %s; it is one of %s", where, string(bytes.TrimSpace(useRaw)), strings.Join(features.NamedAbilities(), ", "))
	}
	grant := Grant{Ability: ref}
	if with, ok := obj[wordWith]; ok {
		if _, err := c.object(with, where+" `with`"); err != nil {
			return "", Grant{}, err
		}
		grant.Parameters = slices.Clone(bytes.TrimSpace(with))
	}
	if err := features.ValidateParameters(&features.ValidateParametersInput{Ref: ref, Parameters: grant.Parameters}); err != nil {
		return "", Grant{}, c.refuse("%s `with` for %q: %v", where, use, err)
	}
	if costRaw, ok := obj[wordCost]; ok {
		cost, err := c.object(costRaw, where+" `cost`")
		if err != nil {
			return "", Grant{}, err
		}
		if err := c.allowed(cost, where+" `cost`", wordCharges); err != nil {
			return "", Grant{}, err
		}
		chargesRaw, ok := cost[wordCharges]
		if !ok {
			return "", Grant{}, c.refuse("%s `cost` names no `charges`; it is `charges`, a whole number of at least 1", where)
		}
		if held == 0 {
			return "", Grant{}, c.refuse("%s `cost` is paid in `charges`, but the item declares no `charges`", where)
		}
		uses, err := c.whole(chargesRaw, where+" cost.charges")
		if err != nil {
			return "", Grant{}, err
		}
		if uses < 1 {
			return "", Grant{}, c.refuse("%s `cost.charges` is %d; it is a whole number of at least 1", where, uses)
		}
		if uses > held {
			return "", Grant{}, c.refuse("%s cost.charges is %d; the item holds at most %d", where, uses, held)
		}
		grant.Pool = chargesPool
		grant.Uses = uses
	}
	return strings.ReplaceAll(use, " ", "-"), grant, nil
}
