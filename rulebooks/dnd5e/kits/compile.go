// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package kits

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
)

// CompileInput is the kit's two blocks as an author wrote them.
type CompileInput struct {
	// Holder says who holds the kit in a refusal: `item "cloak"` and "the item".
	Holder Holder
	// Charges is the author's `charges` block; nil when the author wrote none.
	Charges json.RawMessage
	// Lets is the author's `lets` list; nil when the author wrote none.
	Lets json.RawMessage
}

// CompileOutput is the kit the engine runs.
type CompileOutput struct {
	Kit Kit
}

// The author's words.
const (
	wordCharges = "charges"
	wordMax     = "max"
	wordRefills = "refills"
	wordOn      = "on"
	wordUse     = "use"
	wordWith    = "with"
	wordCost    = "cost"
	wordUses    = "uses"

	chargesPool = "charges"
)

// Compile turns a kit written in an author's words into the kit the engine
// runs. It is strict: an unknown or missing word or value is refused with a
// sentence that starts with the holder and names the word.
//
// Whether `lets` may be absent or empty is the holder's rule, not the kit's:
// an absent `lets` compiles to a kit with no grants, as does an empty list.
// The kit is not validated here; a holder validates it with its own name.
func Compile(in *CompileInput) (*CompileOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("compiling a kit requires input")
	}
	if err := in.Holder.check(); err != nil {
		return nil, err
	}
	c := compiler{holder: in.Holder.Named, noun: in.Holder.Noun}
	var k Kit
	if len(in.Charges) > 0 {
		pool, err := c.charges(in.Charges)
		if err != nil {
			return nil, err
		}
		k.Pools = map[string]Pool{chargesPool: pool}
	}
	if len(in.Lets) > 0 {
		grants, own, err := c.lets(in.Lets, k.Pools[chargesPool].Maximum)
		if err != nil {
			return nil, err
		}
		k.Grants = grants
		if len(own) > 0 {
			if k.Pools == nil {
				k.Pools = map[string]Pool{}
			}
			maps.Copy(k.Pools, own)
		}
	}
	return &CompileOutput{Kit: k}, nil
}

type compiler struct {
	holder string
	noun   string
}

func (c *compiler) refuse(format string, args ...any) error {
	return fmt.Errorf("%s: %s", c.holder, fmt.Sprintf(format, args...))
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

// whole reads a whole number written as digits: 1 passes, 1.5 and 1e0 do not.
func (c *compiler) whole(raw json.RawMessage, word string) (int, error) {
	n, err := strconv.Atoi(string(bytes.TrimSpace(raw)))
	if err != nil {
		return 0, c.refuse("`%s` is %s; it must be a whole number", word, string(bytes.TrimSpace(raw)))
	}
	return n, nil
}

func (c *compiler) charges(raw json.RawMessage) (Pool, error) {
	return c.pool(raw, poolBlock{
		place: PlaceCharges, block: "`charges`", max: "`charges.max`",
		refills: "`refills`", on: "`refills.on`",
	})
}

// poolBlock is how one block that declares a count speaks of itself in a
// refusal: the shared `charges` and an entry's own `uses` are read by the same
// code and differ only in these words.
type poolBlock struct {
	place   Place
	block   string
	max     string
	refills string
	on      string
}

func (c *compiler) pool(raw json.RawMessage, b poolBlock) (Pool, error) {
	obj, err := c.object(raw, b.block)
	if err != nil {
		return Pool{}, err
	}
	maxBounds := find(b.place, wordMax).bounds
	if err := c.allowed(obj, b.block, wordsAt(b.place)...); err != nil {
		return Pool{}, err
	}
	maxRaw, ok := obj[wordMax]
	if !ok {
		return Pool{}, c.refuse("%s has no `max`; it is a whole number from %d to %d", b.block, maxBounds.Min, maxBounds.Max)
	}
	limit, err := strconv.Atoi(string(bytes.TrimSpace(maxRaw)))
	if err != nil {
		return Pool{}, c.refuse("%s is %s; it must be a whole number", b.max, string(bytes.TrimSpace(maxRaw)))
	}
	if limit < maxBounds.Min || limit > maxBounds.Max {
		return Pool{}, c.refuse("%s is %d; it is a whole number from %d to %d", b.max, limit, maxBounds.Min, maxBounds.Max)
	}
	pool := Pool{Maximum: limit}
	refillsRaw, ok := obj[wordRefills]
	if !ok {
		return pool, nil
	}
	refills, err := c.object(refillsRaw, b.refills)
	if err != nil {
		return Pool{}, err
	}
	if err := c.allowed(refills, b.refills, wordsAt(PlaceRefills)...); err != nil {
		return Pool{}, err
	}
	var on []string
	if err := json.Unmarshal(refills[wordOn], &on); err != nil || len(on) == 0 {
		return Pool{}, c.refuse("%s must be a non-empty list of rests: %s", b.on, strings.Join(valueWords(PlaceRefills, wordOn), ", "))
	}
	seen := map[string]bool{}
	for _, word := range on {
		rest, ok := valueOf(PlaceRefills, wordOn, word)
		if !ok {
			return Pool{}, c.refuse("%s has %q; it is one of %s", b.on, word, strings.Join(valueWords(PlaceRefills, wordOn), ", "))
		}
		if seen[word] {
			return Pool{}, c.refuse("%s names %q more than once", b.on, word)
		}
		seen[word] = true
		pool.RestoreOn = append(pool.RestoreOn, rest.reset)
	}
	return pool, nil
}

// lets reads every entry. It returns the grants and the pools the entries
// declare for themselves with `uses`, each named by its grant.
func (c *compiler) lets(raw json.RawMessage, held int) (map[string]Grant, map[string]Pool, error) {
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, nil, c.refuse("`lets` must be a list of entries")
	}
	grants := make(map[string]Grant, len(entries))
	own := map[string]Pool{}
	for i, entry := range entries {
		where := fmt.Sprintf("lets[%d]", i)
		name, grant, pool, err := c.entry(entry, where, held)
		if err != nil {
			return nil, nil, err
		}
		if _, dup := grants[name]; dup {
			return nil, nil, c.refuse("`use` names %q more than once; each entry uses a different ability", strings.ReplaceAll(name, "-", " "))
		}
		grants[name] = grant
		if pool != nil {
			own[name] = *pool
		}
	}
	return grants, own, nil
}

func (c *compiler) entry(raw json.RawMessage, where string, held int) (string, Grant, *Pool, error) {
	obj, err := c.object(raw, where)
	if err != nil {
		return "", Grant{}, nil, err
	}
	if err := c.allowed(obj, where, wordsAt(PlaceLetsEntry)...); err != nil {
		return "", Grant{}, nil, err
	}
	useRaw, ok := obj[wordUse]
	if !ok {
		return "", Grant{}, nil, c.refuse("%s has no `use`; it is one of %s", where, strings.Join(features.NamedAbilities(), ", "))
	}
	var use string
	_ = json.Unmarshal(useRaw, &use)
	ref, ok := features.NamedAbility(use)
	if !ok {
		return "", Grant{}, nil, c.refuse("%s: `use` is %s; it is one of %s", where, string(bytes.TrimSpace(useRaw)), strings.Join(features.NamedAbilities(), ", "))
	}
	grant := Grant{Ability: ref}
	if with, ok := obj[wordWith]; ok {
		if _, err := c.object(with, where+" `with`"); err != nil {
			return "", Grant{}, nil, err
		}
		grant.Parameters = slices.Clone(bytes.TrimSpace(with))
	}
	if err := features.ValidateParameters(&features.ValidateParametersInput{Ref: ref, Parameters: grant.Parameters}); err != nil {
		return "", Grant{}, nil, c.refuse("%s `with` for %q: %v", where, use, err)
	}
	if _, ok := obj[wordUses]; ok {
		if _, ok := obj[wordCost]; ok {
			return "", Grant{}, nil, c.refuse("%s has both `cost` and `uses`; a use is paid from %s's charges or from the entry's own uses, not both", where, c.noun)
		}
	}
	name := strings.ReplaceAll(use, " ", "-")
	var own *Pool
	if usesRaw, ok := obj[wordUses]; ok {
		if name == chargesPool {
			return "", Grant{}, nil, c.refuse("%s would keep its own uses in a pool named `charges`, which is the shared pool's name", where)
		}
		pool, err := c.pool(usesRaw, poolBlock{
			place: PlaceUses, block: where + " `uses`", max: where + " `uses.max`",
			refills: where + " `uses.refills`", on: where + " `uses.refills.on`",
		})
		if err != nil {
			return "", Grant{}, nil, err
		}
		own = &pool
		grant.Pool = name
		grant.Uses = 1
	}
	if costRaw, ok := obj[wordCost]; ok {
		cost, err := c.object(costRaw, where+" `cost`")
		if err != nil {
			return "", Grant{}, nil, err
		}
		if err := c.allowed(cost, where+" `cost`", wordsAt(PlaceCost)...); err != nil {
			return "", Grant{}, nil, err
		}
		chargesRaw, ok := cost[wordCharges]
		if !ok {
			return "", Grant{}, nil, c.refuse("%s `cost` names no `charges`; it is `charges`, a whole number of at least 1", where)
		}
		if held == 0 {
			return "", Grant{}, nil, c.refuse("%s `cost` is paid in `charges`, but %s declares no `charges`", where, c.noun)
		}
		uses, err := c.whole(chargesRaw, where+" cost.charges")
		if err != nil {
			return "", Grant{}, nil, err
		}
		if uses < 1 {
			return "", Grant{}, nil, c.refuse("%s `cost.charges` is %d; it is a whole number of at least 1", where, uses)
		}
		if uses > held {
			return "", Grant{}, nil, c.refuse("%s cost.charges is %d; %s holds at most %d", where, uses, c.noun, held)
		}
		grant.Pool = chargesPool
		grant.Uses = uses
	}
	if grant.Pool == "" && features.MustBeCounted(ref) {
		return "", Grant{}, nil, c.refuse("%s: `use` is %q, which gives its user something for its count alone; write `uses` or a `cost`", where, use)
	}
	return name, grant, own, nil
}
