// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// MembersWithinInput asks who is standing inside a round footprint.
type MembersWithinInput struct {
	// Origin is the cell the footprint is centred on, in the same
	// dungeon-absolute coordinates [Member.Position] reports. A caller holding a
	// Member already holds a legal Origin; it never has to convert anything.
	Origin spatial.Position

	// RadiusCells is how far the footprint reaches, in CELLS — the unit
	// [Encounter.Distance] answers in, not the feet a rulebook authors.
	//
	// NAMED FOR ITS UNIT rather than called Radius, because this composition has
	// paid for unit confusion more than once (rpg-toolkit#1141, #1150) and a
	// bare Radius on a module whose callers think in feet is an invitation to
	// hand it 5 and mean one cell.
	//
	// Zero is a legal footprint and means the origin cell alone. It is not
	// refused here: a rulebook that converts a sub-cell distance and lands on
	// zero has authored something that cannot reach, and that is a content
	// error worth refusing where the content is declared, not a malformed
	// question to this module.
	RadiusCells float64
}

// MembersWithin reports who is standing inside a round footprint, in the same
// stable order (and with the same placement) [Encounter.Members] reports them.
//
// A ROSTER READ, FILTERED — the sibling of [Encounter.MembersIn], and built the
// same way for the same reason. MembersIn names its footprint by pointing at an
// authored region; this one names one in the moment, by a centre and a reach.
// Two ways of saying WHERE, one answer to WHO, both folded from the projection
// every other member read uses, because two reads disagreeing about where
// somebody stands is the dual-state defect this composition has paid for
// before.
//
// # It says who is there, never who it is for
//
// The caster of an area spell is standing at its centre and IS RETURNED. So is
// an ally, so is a shopkeeper, so is anything else with a placement. This module
// has no idea why it was asked.
//
// That is the seam, not an oversight. "Every creature other than you" is a rule
// a spell states, and rules live above this module — which cannot even read one,
// since [encounter's go.mod] does not require the rulebook (law C1). A caller
// takes the projection it needs from a complete answer. An [Member.Kind]-tagged
// roster is what makes that possible: a caller that must treat a KindWorld
// member differently can see it, rather than discovering later that something
// standing in the blast was silently never mentioned.
//
// # An empty answer is an ordinary answer
//
// Nobody standing in the blast is a fact worth reporting, and a spell that
// catches nobody still happened. Only a malformed question is an error: nil
// input is [ErrNilInput], and a negative reach is [ErrBadReach] — refused
// rather than answered empty, so content that converted to a backwards
// footprint cannot look like a spell that simply missed.
//
// Returns [ErrNoField] if a member's cell cannot be resolved, for
// [Encounter.Members]' reason.
func (e *Encounter) MembersWithin(in *MembersWithinInput) ([]Member, error) {
	if in == nil {
		return nil, fmt.Errorf("members within: %w", ErrNilInput)
	}
	if in.RadiusCells < 0 {
		return nil, fmt.Errorf("members within: reach %v cells: %w", in.RadiusCells, ErrBadReach)
	}

	all, err := e.Members()
	if err != nil {
		return nil, fmt.Errorf("members within: %w", err)
	}

	caught := make([]Member, 0, len(all))
	for _, m := range all {
		if e.Distance(in.Origin, m.Position) <= in.RadiusCells {
			caught = append(caught, m)
		}
	}
	return caught, nil
}

// CoverageThreshold is how much of a cell a footprint must lie on for the cell
// to count as covered: half.
//
// THE TABLETOP'S OWN RULE, and ours. A template on a table catches a square
// when it covers most of it, and half is where "most" has always been drawn.
// It lives HERE rather than in [tools/spatial] because it is a ruling, not
// arithmetic: spatial returns fractions and holds no opinion about what a
// fraction means, so a rulebook that counted a cell at a third would be a
// different number in a different file without spatial changing at all.
//
// A COMPARISON AT HALF NEEDS A TOLERANCE, because a box's edge that bisects a
// cell exactly produces a fraction a few ulp either side of 0.5 depending on
// which cosine it went through — and "the edge cut it exactly in half" must not
// mean "caught" on one bearing and "missed" on the one next to it.
const CoverageThreshold = 0.5

// coverageTolerance is the slack on the comparison against
// [CoverageThreshold]. Floating-point noise, nothing more: the nearest fraction
// a differently-placed cell produces is percentage points away, so there is no
// case this tolerance decides.
const coverageTolerance = 1e-9

// MembersCoveredInput asks who is standing under a footprint aimed at a cell.
type MembersCoveredInput struct {
	// Footprint is the shape, in FEET — the unit a rulebook authors a spell
	// in, and the unit this module's embedding is built in ([FeetPerCell]).
	// It is spatial's type because the shape is spatial's: what shapes exist
	// is the ruler's business, and a second vocabulary for them here would be
	// a second place to add a cone.
	Footprint spatial.Footprint

	// Anchor is the cell the footprint is placed at: the caster's own cell for
	// a shape that originates on them. Dungeon-absolute.
	Anchor spatial.Position

	// Toward is the cell the footprint is aimed at — a REFERENCE the caster
	// picked, not a calculation. Only its bearing from Anchor is read, so a
	// cell twice as far away in the same direction aims the same shape.
	//
	// Refused when it equals Anchor ([ErrBadReach]): there is no direction
	// from a cell to itself.
	Toward spatial.Position

	// AtEdge puts the footprint's near edge on the Anchor cell's boundary, so
	// the anchor cell is never under it — the 15-foot cube "originating from
	// you". False centres the footprint on the anchor cell instead.
	//
	// A BOOL RATHER THAN THE RULE ITSELF, because there are two and this
	// module is not the one that decides which a spell uses. The day a third
	// anchoring arrives it is spatial's [spatial.AnchorRule] that grows, and
	// this field becomes that type.
	AtEdge bool
}

// MembersCoveredOutput is who was caught, and what the shape lay on.
type MembersCoveredOutput struct {
	// Members are the covered members, in the same stable order (and with the
	// same placement) [Encounter.Members] reports them.
	Members []Member

	// Cells is what was covered and how much — only cells at or above
	// [CoverageThreshold], so a reader of this map is reading the same set the
	// members were drawn from and not a wider one.
	//
	// It is here because the beat and the atlas both want to draw the blast,
	// and re-deriving it from the member list is impossible: an empty cell
	// under the shape is part of the shape.
	Cells map[spatial.Position]float64
}

// MembersCovered reports who is standing under a footprint anchored at one cell
// and aimed at another.
//
// A ROSTER READ, FILTERED — the sibling of [Encounter.MembersWithin] and
// [Encounter.MembersIn], built the same way for the same reason. MembersIn
// names its footprint by pointing at an authored region, MembersWithin names a
// circle by a centre and a reach, and this one names an arbitrary shape by a
// transform. Three ways of saying WHERE, one answer to WHO, all folded from the
// projection every other member read uses.
//
// # The geometry is spatial's and the ruling is this module's
//
// [spatial.Coverage] rasterises the shape and answers in FRACTIONS, holding no
// opinion about what a fraction means. [CoverageThreshold] is the opinion: half
// a cell is covered. That split is why a cone or a polygon arrives without this
// function changing, and why a rulebook that wanted a third would change a
// constant rather than a raster.
//
// The plane comes from the field's own orientation at [FeetPerCell] across the
// flats, so a footprint authored in feet lands on the cells those feet reach.
// Nobody hands this module a scale and nobody can disagree with it about one.
//
// # It says who is there, never who it is for
//
// The caster is returned when they are standing under the shape — for an
// edge-anchored box they are not, by construction, but that is the SHAPE's
// doing and not a rule this module applied. "Every creature other than you" is
// a rule a spell states, and rules live above a module whose go.mod cannot
// import the rulebook (C1). [Encounter.MembersWithin] says this at more length
// and it is the same law.
//
// # An empty answer is an ordinary answer
//
// A blast that catches nobody still happened. Only a malformed question is an
// error: nil input is [ErrNilInput], an aim on the anchor is [ErrBadReach], and
// a footprint with no shape or no sides comes back as spatial's own
// [spatial.ErrNoFootprint] / [spatial.ErrBadFootprint], carried rather than
// reworded so one sentinel finds every place a shape was wrong.
func (e *Encounter) MembersCovered(in *MembersCoveredInput) (MembersCoveredOutput, error) {
	if in == nil {
		return MembersCoveredOutput{}, fmt.Errorf("members covered: %w", ErrNilInput)
	}

	emb := e.embedding()
	facing, ok := emb.Bearing(in.Anchor, in.Toward)
	if !ok {
		return MembersCoveredOutput{}, fmt.Errorf(
			"members covered: the shape is aimed at %v, which is where it is anchored: %w",
			in.Anchor, ErrBadReach)
	}

	anchor := spatial.AnchorAtCentre
	if in.AtEdge {
		anchor = spatial.AnchorAtEdge
	}
	cov, err := spatial.Coverage(emb, e.canvas.GetGrid(), spatial.CoverageInput{
		Footprint: in.Footprint,
		At:        in.Anchor,
		Facing:    facing,
		Anchor:    anchor,
	})
	if err != nil {
		return MembersCoveredOutput{}, fmt.Errorf("members covered: %w", err)
	}

	covered := make(map[spatial.Position]float64, len(cov.Cells))
	for cell, f := range cov.Cells {
		if f+coverageTolerance >= CoverageThreshold {
			covered[cell] = f
		}
	}

	all, err := e.Members()
	if err != nil {
		return MembersCoveredOutput{}, fmt.Errorf("members covered: %w", err)
	}

	out := MembersCoveredOutput{Members: make([]Member, 0, len(all)), Cells: covered}
	for _, m := range all {
		if _, hit := covered[m.Position]; hit {
			out.Members = append(out.Members, m)
		}
	}

	return out, nil
}

// embedding is this field's plane: its own declared orientation, at
// [FeetPerCell] across the flats.
//
// THE ONE PLACE THE SCALE MEETS THE GRID. units.go already says a cell is five
// feet for every module above; this is that same fact turned into a frame, so a
// footprint authored in feet rasterises onto the cells those feet reach. A
// caller supplying its own width would be a second answer to how big a cell is.
func (e *Encounter) embedding() spatial.HexEmbedding {
	return spatial.NewHexEmbedding(spatial.HexEmbeddingConfig{
		Orientation: e.field.orientation.spatial(),
		CellWidth:   FeetPerCell,
	})
}
