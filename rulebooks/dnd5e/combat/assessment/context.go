// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package assessment

import (
	"fmt"
	"math"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
)

// MemberFacts carries explicit facts about a member in the declared universe.
// Down is only observed standing, not proof of any other condition or capacity.
// Missing reaction readiness must not become a known spent or ready resource.
type MemberFacts struct {
	ID            string
	Down          contributions.Fact[bool]
	ReactionReady contributions.Fact[bool]
}

// PairFacts carries supplied facts in the From→To direction. Distance is in
// grid cells, as supplied by encounter, not a measurement performed by a rule.
// No reverse relationship/sight or unstated completeness is inferred here.
type PairFacts struct {
	From          string
	To            string
	DistanceCells contributions.Fact[float64]
	Hostile       contributions.Fact[bool]
	CanSee        contributions.Fact[bool]
}

// Context names the observer plus a bounded universe of other members. Complete
// is an explicit guarantee that the relevant universe is exhaustive. An observed
// collection alone cannot provide that guarantee, even when it is empty.
// Values carry no callback, world, target sheet or hidden provider inventory.
type Context struct {
	Observer string
	Members  []MemberFacts
	Pairs    []PairFacts
	Complete bool
}

// Validate refuses malformed identities, duplicate/out-of-universe pairs, and
// impossible known distances. Unknown facts are valid; invalid facts are errors.
func (c Context) Validate() error {
	if c.Observer == "" {
		return fmt.Errorf("assessment context requires an observer")
	}
	ids := map[string]bool{c.Observer: true}
	for _, member := range c.Members {
		if member.ID == "" || ids[member.ID] {
			return fmt.Errorf("invalid or duplicate context member %q", member.ID)
		}
		ids[member.ID] = true
	}
	pairs := make(map[[2]string]bool, len(c.Pairs))
	for _, pair := range c.Pairs {
		key := [2]string{pair.From, pair.To}
		if pair.From == pair.To || !ids[pair.From] || !ids[pair.To] || pairs[key] {
			return fmt.Errorf("invalid or duplicate context pair %q to %q", pair.From, pair.To)
		}
		pairs[key] = true
		if distance, known := pair.DistanceCells.Get(); known &&
			(distance < 0 || math.IsNaN(distance) || math.IsInf(distance, 0)) {
			return fmt.Errorf("invalid distance for context pair %q to %q", pair.From, pair.To)
		}
	}
	return nil
}

// Clone detaches the context's collections; each fact is already a scalar value.
func (c Context) Clone() Context {
	c.Members = slices.Clone(c.Members)
	c.Pairs = slices.Clone(c.Pairs)
	return c
}

// PairInput names two distinct subjects in the declared context universe.
type PairInput struct {
	From string
	To   string
}

// PairOutput returns detached facts, with unknown fields when the named pair was
// not supplied. Missing pair facts never become a fabricated negative answer.
type PairOutput struct {
	Facts PairFacts
}

// Pair looks up a directed pair, refusing subjects outside the declared universe
// and malformed input. It does not query live truth or synthesize reverse facts.
func (c Context) Pair(in *PairInput) (*PairOutput, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if in == nil || in.From == in.To || !c.contains(in.From) || !c.contains(in.To) {
		return nil, fmt.Errorf("pair requires distinct subjects in the declared universe")
	}
	for _, pair := range c.Pairs {
		if pair.From == in.From && pair.To == in.To {
			return &PairOutput{Facts: pair}, nil
		}
	}
	return &PairOutput{Facts: PairFacts{From: in.From, To: in.To}}, nil
}

func (c Context) contains(id string) bool {
	if id == "" {
		return false
	}
	if id == c.Observer {
		return true
	}
	return slices.ContainsFunc(c.Members, func(member MemberFacts) bool { return member.ID == id })
}
