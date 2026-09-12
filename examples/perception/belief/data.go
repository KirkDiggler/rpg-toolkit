// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package belief

import (
	"errors"
	"fmt"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/examples/perception/testimony"
)

// ErrInvalidData reports persisted state this package could not have written.
var ErrInvalidData = errors.New("invalid belief data")

// Relation names, persisted as words rather than numbers. The vocabulary is
// sealed: a claim is same or distinct, and there is no third word — because the
// third state is the ABSENCE of a claim and absence is not stored.
const (
	sameWord     = "same"
	distinctWord = "distinct"
)

// Data is the persistent shape of everyone's claims.
type Data struct {
	Observers map[testimony.Observer]ObserverData `json:"observers,omitempty"`
}

// ObserverData is one observer's judgments.
type ObserverData struct {
	Claims []ClaimData `json:"claims,omitempty"`
	Names  []NameData  `json:"names,omitempty"`
}

// ClaimData is one recorded judgment about a pair.
type ClaimData struct {
	A        testimony.TrackID   `json:"a"`
	B        testimony.TrackID   `json:"b"`
	Relation string              `json:"relation"`
	At       testimony.StampData `json:"at"`
}

// NameData is one recorded identification.
type NameData struct {
	Track testimony.TrackID   `json:"track"`
	Name  Name                `json:"name"`
	At    testimony.StampData `json:"at"`
}

// ToData returns a persistent snapshot, ordered for a stable file.
func (b *Beliefs) ToData() Data {
	out := Data{}

	for _, o := range b.observers() {
		claims := b.Claims(o)
		names := b.Named(o)

		if len(claims) == 0 && len(names) == 0 {
			continue
		}

		od := ObserverData{}

		for _, c := range claims {
			od.Claims = append(od.Claims, ClaimData{
				A:        c.A,
				B:        c.B,
				Relation: word(c.Rel),
				At:       testimony.StampData{Seq: c.At.Seq, Tick: c.At.Tick},
			})
		}

		for _, n := range names {
			od.Names = append(od.Names, NameData{
				Track: n.Track,
				Name:  n.Name,
				At:    testimony.StampData{Seq: n.At.Seq, Tick: n.At.Tick},
			})
		}

		if out.Observers == nil {
			out.Observers = make(map[testimony.Observer]ObserverData)
		}

		out.Observers[o] = od
	}

	return out
}

// Load rebuilds beliefs, refusing anything this package could not have written.
func Load(d Data) (*Beliefs, error) {
	b := New()

	for o, od := range d.Observers {
		if o == "" {
			return nil, fmt.Errorf("load: %w: an observer with no name", ErrInvalidData)
		}

		if len(od.Claims) == 0 && len(od.Names) == 0 {
			return nil, fmt.Errorf("load %s: %w: an observer claiming nothing", o, ErrInvalidData)
		}

		if err := loadClaims(b, o, od.Claims); err != nil {
			return nil, err
		}

		if err := loadNames(b, o, od.Names); err != nil {
			return nil, err
		}
	}

	return b, nil
}

func loadClaims(b *Beliefs, o testimony.Observer, claims []ClaimData) error {
	for i, c := range claims {
		rel, ok := relationOf(c.Relation)
		if !ok {
			// "unrelated" lands here too, deliberately: no claim is not a claim,
			// and a stored one would be a judgment nobody made.
			return fmt.Errorf("load %s claim %d: %w: %q is not a relation anybody claims",
				o, i, ErrInvalidData, c.Relation)
		}

		if c.A == "" || c.B == "" {
			return fmt.Errorf("load %s claim %d: %w: a claim about no track", o, i, ErrInvalidData)
		}

		if c.A == c.B {
			return fmt.Errorf("load %s claim %d: %w: a track is not a pair", o, i, ErrInvalidData)
		}

		// Pairs are stored normalized, so one that is not could only have been
		// written by something that does not know how these are keyed — and
		// loading it would hide a duplicate behind a second spelling.
		if c.A > c.B {
			return fmt.Errorf("load %s claim %d: %w: a pair stored out of order", o, i, ErrInvalidData)
		}

		if rel, _ := b.Relation(o, c.A, c.B); rel != Unrelated {
			return fmt.Errorf("load %s claim %d: %w: the pair is claimed twice", o, i, ErrInvalidData)
		}

		at := testimony.Stamp{Seq: c.At.Seq, Tick: c.At.Tick}
		if err := b.Assert(o, c.A, c.B, rel, at); err != nil {
			return fmt.Errorf("load %s claim %d: %w", o, i, err)
		}
	}

	return nil
}

func loadNames(b *Beliefs, o testimony.Observer, names []NameData) error {
	for i, n := range names {
		if n.Track == "" {
			return fmt.Errorf("load %s name %d: %w: a name for no track", o, i, ErrInvalidData)
		}

		// An empty name is how an identification is RETRACTED, so a stored one
		// is a retraction that was saved instead of applied.
		if n.Name == "" {
			return fmt.Errorf("load %s name %d: %w: a track named nothing", o, i, ErrInvalidData)
		}

		if _, _, named := b.NameOf(o, n.Track); named {
			return fmt.Errorf("load %s name %d: %w: the track is named twice", o, i, ErrInvalidData)
		}

		at := testimony.Stamp{Seq: n.At.Seq, Tick: n.At.Tick}
		if err := b.Identify(o, n.Track, n.Name, at); err != nil {
			return fmt.Errorf("load %s name %d: %w", o, i, err)
		}
	}

	return nil
}

func (b *Beliefs) observers() []testimony.Observer {
	seen := make(map[testimony.Observer]struct{}, len(b.claims)+len(b.names))

	for o := range b.claims {
		seen[o] = struct{}{}
	}

	for o := range b.names {
		seen[o] = struct{}{}
	}

	out := make([]testimony.Observer, 0, len(seen))
	for o := range seen {
		out = append(out, o)
	}

	slices.Sort(out)

	return out
}

func word(r Relation) string {
	if r == Distinct {
		return distinctWord
	}

	return sameWord
}

func relationOf(s string) (Relation, bool) {
	switch s {
	case sameWord:
		return Same, true
	case distinctWord:
		return Distinct, true
	default:
		return Unrelated, false
	}
}
