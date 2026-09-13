// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior

import (
	"cmp"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior/deed"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// Retaliator is the bow skeleton's mind: it turns on whoever attacked it
// while that deed is fresher than its patience, and otherwise goes for the
// closest. It is the captain's retarget-on-a-witnessed-deed from
// mind/behavior's proofs with a shot in place of a heal.
//
// Four judgments and no state. Judge attaches an attack deed to the figure
// the deed names — a claim, because the deed already says who and the mind
// still has to believe it. Name is reflexive. Rank is the grudge, then the
// distance. Keep is nothing: it stands and fights.
type Retaliator struct {
	// Space is how it knows what closest means. Geometry is the caller's
	// (mind/behavior R11); a mind may ask.
	Space behavior.Space
	// Patience is how many ticks old a deed may be and still be held
	// against its actor.
	Patience uint64
}

// Judge attaches every attack deed to the member it names as actor, when
// the actor is held at all. The deed's subject is qualified by channel, so
// the store never merged them; only this claim does.
func (r *Retaliator) Judge(in *behavior.JudgeInput) (*behavior.JudgeOutput, error) {
	held := func(id core.EntityID) bool {
		return slices.ContainsFunc(in.Holdings, func(h behavior.Holding) bool { return h.Subject == id })
	}

	var same []behavior.Pair

	for _, h := range in.Holdings {
		if h.Channel != deed.Channel {
			continue
		}

		if d, err := deed.Decode(h.Payload); err == nil && d.Verb == encounter.DeedAttack && d.Actor != "" && held(d.Actor) {
			same = append(same, behavior.Pair{A: h.Subject, B: d.Actor})
		}
	}

	return &behavior.JudgeOutput{Same: same}, nil
}

// Name calls a contact by its bearer's id. A monster has no words for the
// people it fights; the id is the encounter's and never parsed.
func (r *Retaliator) Name(in *behavior.NameInput) (*behavior.NameOutput, error) {
	return &behavior.NameOutput{Name: behavior.Name(in.Contact.Holdings[0].Subject), Named: true}, nil
}

// Rank puts whoever attacked ME, fresher than Patience, first; then the
// closest. A deed against somebody else is not a grudge. Ghosts rank by
// the same distance, after everything live.
func (r *Retaliator) Rank(in *behavior.RankInput) (*behavior.RankOutput, error) {
	s := in.Situation
	ranked := slices.Clone(s.Contacts)

	distance := func(c behavior.Contact) int {
		out, err := r.Space.Distance(&behavior.DistanceInput{From: s.Self.Where, To: c.Where()})
		if err != nil || !out.Known {
			return int(^uint(0) >> 1)
		}

		return out.Steps
	}

	slices.SortStableFunc(ranked, func(a, b behavior.Contact) int {
		if ga, gb := r.grudge(a, s), r.grudge(b, s); ga != gb {
			if ga {
				return -1
			}

			return 1
		}

		if la, lb := a.Current(), b.Current(); la != lb {
			if la {
				return -1
			}

			return 1
		}

		if c := cmp.Compare(distance(a), distance(b)); c != 0 {
			return c
		}

		return cmp.Compare(a.Holdings[0].Subject, b.Holdings[0].Subject)
	})

	return &behavior.RankOutput{Ranked: ranked}, nil
}

// grudge reports whether the contact holds a fresh attack deed against the
// actor itself.
func (r *Retaliator) grudge(c behavior.Contact, s behavior.Situation) bool {
	for _, h := range c.Holdings {
		if h.Channel != deed.Channel {
			continue
		}

		d, err := deed.Decode(h.Payload)
		if err != nil || d.Verb != encounter.DeedAttack || d.Target != s.Actor {
			continue
		}

		if s.At-h.Confirmed <= r.Patience {
			return true
		}
	}

	return false
}

// Keep is nothing. A retaliator stands and fights.
func (r *Retaliator) Keep(*behavior.KeepInput) (*behavior.KeepOutput, error) {
	return &behavior.KeepOutput{Steps: 0}, nil
}
