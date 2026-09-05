// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package graph

import (
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// Settle declares that a relation between two entities changes because one
// of them came to know something.
//
// Every other projection rewrites one entity's own outgoing edges. A stance
// between two groups is not one entity's edge: it is a pair, and it has to
// change in both directions at once or the graph contradicts itself — the
// camp no longer hostile to the party while the party is still hostile to
// the camp. Settle names the pair, and while Of carries OnFlag the pair's
// edges in Relations are replaced, in both directions, by To.
//
// To is what the pair's relation IS while the flag is up, not a conversion of
// what was there: unlike [Threshold], no prior edge is required. An empty To
// is the honest zero — the pair holds none of the named relations, which is
// what "neutral" means in a world whose only stances are hostile and allied.
//
// Of is who has to know. It is what lets a faction know what its mind knows:
// flag the chief, settle the camp's pair, and a scout who learns the same
// thing changes nothing.
//
// Flags only ever go up, so there is no reducer that puts the pair back. The
// way back is a different fact raising a different flag on a second Settle
// declared after this one, exactly as [AdoptStance] is overruled. First use:
// the hold-out — a goblin camp is hostile to the party until its chief comes
// to know the party saved the Wiseman.
type Settle struct {
	// OnFlag is the flag that settles the pair.
	OnFlag Flag

	// Of is the entity whose flag counts. A flag raised on anyone else leaves
	// the pair as declared.
	Of journal.EntityID

	// Between is the pair. Order does not matter: both directions change.
	// The two must differ — a pair of one is not a pair, and [New] refuses
	// it rather than folding a silent no-op over an authoring slip.
	Between [2]journal.EntityID

	// Relations are the edges replaced, in both directions. Membership must
	// not be one of them, and [New] refuses a Settle that names it.
	Relations []Relation

	// To is the relation the pair holds while the flag is up, in both
	// directions. Empty means the pair holds none of Relations at all.
	To Relation
}

func (p Settle) project(s *State) {
	if !s.Flagged(p.OnFlag, p.Of) {
		return
	}

	a, b := p.Between[0], p.Between[1]
	for _, dir := range [][2]journal.EntityID{{a, b}, {b, a}} {
		s.dropEdgesBetween(dir[0], dir[1], p.Relations)
		if p.To != "" {
			s.addEdge(Edge{From: dir[0], Rel: p.To, To: dir[1]})
		}
	}
}

// ErrNoRelations reports a [Settle] that names no relations to replace — a
// declaration that could never change anything.
var ErrNoRelations = errors.New("graph: this settle names no relations to replace — say which stance edges change")

// ErrPairOfOne reports a [Settle] whose pair names the same entity twice. A
// settle over such a pair would fold to nothing and say nothing, which is how
// a typo in a declaration goes unnoticed.
var ErrPairOfOne = errors.New("graph: this settle names one entity as both sides of its pair — a pair is two")

// ErrSettlesMembership reports a [Settle] that would rewrite the membership
// relation. Belonging is not a stance: a camp does not join the party because
// it stopped fighting them.
var ErrSettlesMembership = errors.New("graph: this settle names the membership relation — belonging is not a stance")

// adoptProjections validates the projections that carry references [New] can
// check. Only [Settle] names entities today; the others name flags, roles,
// and relations, none of which are declared anywhere to check against.
func (w *World) adoptProjections(projections []Projection) error {
	for _, p := range projections {
		if settle, ok := p.(Settle); ok {
			if err := w.checkSettle(settle); err != nil {
				return err
			}
		}
	}

	return nil
}

func (w *World) checkSettle(p Settle) error {
	if err := w.known(p.Of, p.Between[0], p.Between[1]); err != nil {
		return fmt.Errorf("settle on %q: %w", p.OnFlag, err)
	}
	if p.Between[0] == p.Between[1] {
		return fmt.Errorf("%w: settle on %q names %q twice", ErrPairOfOne, p.OnFlag, p.Between[0])
	}
	if len(p.Relations) == 0 {
		return fmt.Errorf("%w: settle on %q", ErrNoRelations, p.OnFlag)
	}
	if p.To == w.membership || slices.Contains(p.Relations, w.membership) {
		return fmt.Errorf("%w: settle on %q", ErrSettlesMembership, p.OnFlag)
	}

	return nil
}
