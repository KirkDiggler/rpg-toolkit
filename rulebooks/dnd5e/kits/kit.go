// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package kits is counts and what they let a holder use: the pools a holder
// keeps for itself and the named abilities it may use, each ability paying
// from a pool or from none.
//
// A holder is an item a character wears today and a creature's stat block
// next. The kit does not know which: it takes the holder's name as text, to
// put at the head of a refusal, and nothing more.
//
// It is a leaf. It knows the rulebook's named abilities (to refuse a grant the
// rulebook cannot describe) and nothing of the holder, the sheet that holds
// it, or any gate that asks it to pay.
package kits

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
)

// Pool is one count the holder keeps for itself.
type Pool struct {
	// Maximum is how many the pool holds when full.
	Maximum int `json:"maximum"`
	// RestoreOn names the rests that fill it. Empty never comes back.
	RestoreOn []coreResources.ResetType `json:"restore_on,omitempty"`
}

// Grant is one ability the holder may use, with the values the holder supplies.
type Grant struct {
	// Ability is a named rulebook ability.
	Ability core.Ref `json:"ability"`
	// Parameters are its values, for example {"level": 1}.
	Parameters json.RawMessage `json:"parameters"`
	// Pool is which of the holder's pools pays.
	Pool string `json:"pool,omitempty"`
	// Uses is how many it costs.
	Uses int `json:"uses,omitempty"`
}

// Kit is counts and what they let a holder use: an item's, or a creature's.
type Kit struct {
	// Pools are the holder's own counts, by name.
	Pools map[string]Pool `json:"pools,omitempty"`
	// Grants are what the holder may use, by name.
	Grants map[string]Grant `json:"grants,omitempty"`
}

// Holder is how a refusal speaks of who holds a kit. Both are required: an
// empty one is a caller's defect and is refused by name.
type Holder struct {
	// Named is the holder by name, already quoted: `item "Lieutenant's cloak"`.
	Named string
	// Noun is the holder mid-sentence: "the item".
	Noun string
}

func (h Holder) check() error {
	switch {
	case h.Named == "":
		return fmt.Errorf("a kit's holder needs a Named, the holder by name, already quoted")
	case h.Noun == "":
		return fmt.Errorf("a kit's holder %s needs a Noun, the holder mid-sentence", h.Named)
	}
	return nil
}

// Clone returns a copy that shares no map, slice or byte with the original.
// A map that is nil stays nil.
func (k Kit) Clone() Kit {
	out := k
	if k.Pools != nil {
		out.Pools = make(map[string]Pool, len(k.Pools))
		for name, pool := range k.Pools {
			pool.RestoreOn = slices.Clone(pool.RestoreOn)
			out.Pools[name] = pool
		}
	}
	if k.Grants != nil {
		out.Grants = make(map[string]Grant, len(k.Grants))
		for name, grant := range k.Grants {
			grant.Parameters = slices.Clone(grant.Parameters)
			out.Grants[name] = grant
		}
	}
	return out
}

// Validate refuses a kit the rulebook cannot run, naming the thing. holder says
// who holds the kit: its Named opens a refusal, its Noun is how the sentence
// refers to it after. It does not require a grant to exist: whether a holder
// must let something be used is the holder's own rule.
func (k Kit) Validate(holder Holder) error {
	if err := holder.check(); err != nil {
		return err
	}
	for _, name := range slices.Sorted(maps.Keys(k.Pools)) {
		if !validName(name) {
			return fmt.Errorf("%s pool name %q must be letters, digits, '-' and '_'", holder.Named, name)
		}
		if err := validatePool(holder.Named, name, k.Pools[name]); err != nil {
			return err
		}
	}
	for _, name := range slices.Sorted(maps.Keys(k.Grants)) {
		if !validName(name) {
			return fmt.Errorf("%s grant name %q must be letters, digits, '-' and '_'", holder.Named, name)
		}
		if err := k.validateGrant(holder, name, k.Grants[name]); err != nil {
			return err
		}
	}
	return nil
}

func validatePool(holder string, name string, pool Pool) error {
	if pool.Maximum < 1 {
		return fmt.Errorf("%s pool %q maximum %d must be at least 1", holder, name, pool.Maximum)
	}
	seen := make(map[coreResources.ResetType]struct{}, len(pool.RestoreOn))
	for _, rest := range pool.RestoreOn {
		if rest != coreResources.ResetShortRest && rest != coreResources.ResetLongRest {
			return fmt.Errorf("%s pool %q restore_on %q must be short_rest or long_rest", holder, name, rest)
		}
		if _, dup := seen[rest]; dup {
			return fmt.Errorf("%s pool %q restore_on names %q more than once", holder, name, rest)
		}
		seen[rest] = struct{}{}
	}
	return nil
}

func (k Kit) validateGrant(holder Holder, name string, grant Grant) error {
	if err := features.ValidateParameters(&features.ValidateParametersInput{
		Ref:        grant.Ability,
		Parameters: grant.Parameters,
	}); err != nil {
		return fmt.Errorf("%s grant %q: %w", holder.Named, name, err)
	}
	if grant.Pool == "" {
		if grant.Uses != 0 {
			return fmt.Errorf("%s grant %q has uses %d but names no pool to pay from", holder.Named, name, grant.Uses)
		}
		if features.MustBeCounted(grant.Ability) {
			return fmt.Errorf("%s grant %q is %s, which must be paid from a pool", holder.Named, name, abilityWord(grant.Ability))
		}
		return nil
	}
	if _, ok := k.Pools[grant.Pool]; !ok {
		return fmt.Errorf("%s grant %q names pool %q, which %s does not declare", holder.Named, name, grant.Pool, holder.Noun)
	}
	if grant.Uses < 1 {
		return fmt.Errorf("%s grant %q pays from pool %q but its uses %d is below 1", holder.Named, name, grant.Pool, grant.Uses)
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

// abilityWord is the word an author writes for a named ability, or the ref
// itself when no entry has one.
func abilityWord(ref core.Ref) string {
	for _, word := range features.NamedAbilities() {
		if named, ok := features.NamedAbility(word); ok && named == ref {
			return word
		}
	}
	return ref.String()
}
