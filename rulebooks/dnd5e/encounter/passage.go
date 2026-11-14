// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// passage.go is A LINE THAT REVEALS, AND A LINE THAT TAKES YOU SOMEWHERE
// (rpg-project#958, Passages S1 of #848): two grants a line's row may carry,
// enough to author a secret passage with nothing passage-only in the engine.
//
//   - REVEAL: whoever caught the answer — the members its words reached, or
//     a prop's witnesses, the speech rule every fact on a row already uses —
//     comes to know a concealment, through the one reveal writer
//     ([Encounter.revealConcealmentTo]). The servant who tells you the way.
//   - RELOCATE: the SPEAKER is set down on the first free cell of a region
//     ([Encounter.FreeCellsIn]) through the exported [Encounter.Relocate] —
//     so a held member is refused, a member in a fight is refused — with the
//     row's words and rounds and a cause ref that names no place
//     ([relocateCause]). The flagstone that takes you to the chapel.
//
// A PASSAGE IS CONTENT: a floor prop that belongs to a concealment, unseen and
// unsayable until known ([Encounter.speakableProp]), carrying one line per
// destination whose row relocates. No prop at the far end means no line back:
// one-way. Each player says the line themselves, standing within reach of the
// tile; not saying it is staying. Two travellers who go at different moments
// land on the next free cells, in the region's own order.
//
// A LINE THAT WOULD RELOCATE IS ASKED BEFORE IT WRITES ANYTHING
// ([Encounter.refuseRelocateAhead]): in a fight, held, unplaced, or with no
// free cell at the far end, the line is refused whole.

// relocateCause is the cause every row's relocation names: told to every
// observer of either end, so it names the grant and never the place.
var relocateCause = core.Ref{Module: "dnd5e", Type: "consequences", ID: "relocate"}

// RelocateGrant is a line's row setting its speaker down in a region
// (rpg-project#958): the destination, what the speaker is told, and how many
// world rounds the move takes.
type RelocateGrant struct {
	// To is the region the speaker is set down in. REQUIRED, a region of the
	// field.
	To RegionID `json:"to"`

	// Words is what the speaker is told. REQUIRED.
	Words string `json:"words"`

	// Rounds is how many world rounds pass with the move; zero or more.
	Rounds int `json:"rounds,omitempty"`
}

// validatePassageGrant refuses a reveal or a relocate nobody could author on
// purpose: two kinds in one grant, a relocate with no region or no words, or
// with negative rounds. Whether the region or the concealment exists is the
// field's question ([Encounter.checkGrantTargets]).
func validatePassageGrant(what string, c Consequence) error {
	kinds := 0
	for _, set := range []bool{c.Recognize != nil, c.Arrest != nil, c.Jail != nil, c.Reveal != "", c.Relocate != nil} {
		if set {
			kinds++
		}
	}
	if kinds > 1 {
		return fmt.Errorf("%s is of two kinds: one grant is one kind", what)
	}
	if r := c.Relocate; r != nil {
		switch {
		case r.To == "":
			return fmt.Errorf("%s relocates to no region", what)
		case r.Words == "":
			return fmt.Errorf("%s relocates and tells the speaker nothing: name its words", what)
		case r.Rounds < 0:
			return fmt.Errorf("%s relocates over %d rounds", what, r.Rounds)
		}
	}
	return nil
}

// errNoGrantTarget is the refusal of a row grant naming a region or a
// concealment the field does not have.
var errNoGrantTarget = errors.New("a line's grant names something this site does not have")

// checkGrantTargets refuses, at construction, a row grant on any line of the
// options given that names a region or a concealment the field lacks.
func (f *field) checkGrantTargets(owner string, options []Option) error {
	for _, o := range options {
		for i, g := range o.Grants {
			c := g.Consequence
			if r := c.Relocate; r != nil {
				if _, ok := f.regionCells[r.To]; !ok {
					return fmt.Errorf("%s option %q grant %d relocates to %q, which is not a region of this site: %w",
						owner, o.ID, i, r.To, errNoGrantTarget)
				}
			}
			if c.Reveal != "" && f.concealment(c.Reveal) == nil {
				return fmt.Errorf("%s option %q grant %d reveals %q, which is not a concealment of this site: %w",
					owner, o.ID, i, c.Reveal, errNoGrantTarget)
			}
		}
	}
	return nil
}

// concealment is the field's concealment with an id, or nil.
func (f *field) concealment(id ConcealmentID) *concealment {
	for i := range f.concealments {
		if f.concealments[i].id == id {
			return &f.concealments[i]
		}
	}
	return nil
}

// grantReveal teaches a concealment to everyone who caught the answer that
// granted it, each through the one reveal writer; one who knows it already
// learns nothing again.
func (e *Encounter) grantReveal(id ConcealmentID, line recognitionLine) error {
	c := e.field.concealment(id)
	if c == nil {
		return fmt.Errorf("reveal %q: %w", id, errNoGrantTarget)
	}
	for _, m := range line.learners {
		if _, ok := e.members[m]; !ok || e.world.knowsConcealment(m, id) {
			continue
		}
		if err := e.revealConcealmentTo(m, c, "told the way", line.at); err != nil {
			return err
		}
	}
	return nil
}

// grantRelocate sets the line's speaker down on the first free cell of the
// grant's region, through the exported verb, and returns the fight arriving
// formed, if one did.
func (e *Encounter) grantRelocate(g RelocateGrant, line recognitionLine) (*FormedBubble, error) {
	cells, err := e.FreeCellsIn(g.To, 1)
	if err != nil {
		return nil, fmt.Errorf("relocate: %w", err)
	}
	out, err := e.Relocate(&RelocateInput{
		Moves: []Relocation{{Member: line.speaker, To: cells[0]}},
		Cause: RelocationCause{Ref: relocateCause, Words: g.Words, Rounds: g.Rounds},
		Reach: line.reach,
	})
	if err != nil {
		return nil, err
	}
	return out.Formed, nil
}

// refuseRelocateAhead asks, before a line writes anything, whether a row of
// the verdict it would land relocates its speaker, and if so whether that
// relocation would refuse: the line is then refused whole. Every relocating
// row of the verdict is asked — the row is rolled after the line is written,
// so a line one of whose rows could not move the speaker is refused.
func (e *Encounter) refuseRelocateAhead(option Option, beaten bool, speaker MemberID) error {
	for _, g := range option.Grants {
		if g.Failure == beaten || g.Consequence.Relocate == nil {
			continue
		}
		cells, err := e.FreeCellsIn(g.Consequence.Relocate.To, 1)
		if err != nil {
			return fmt.Errorf("relocate: %w", err)
		}
		if _, _, err := e.validateRelocation(&RelocateInput{
			Moves: []Relocation{{Member: speaker, To: cells[0]}},
			Cause: RelocationCause{Ref: relocateCause, Words: g.Consequence.Relocate.Words, Rounds: g.Consequence.Relocate.Rounds},
		}, false); err != nil {
			return err
		}
	}
	return nil
}
