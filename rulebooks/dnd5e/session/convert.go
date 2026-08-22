// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// This file is the converter layer, and its isolation is deliberate.
//
// Encapsulation is not free: keeping inner types off the exported surface (S2)
// means every value the composition hands back must be rewritten into a shape
// this package owns. That is the trade — hosts never recompile when an inner
// module changes, and we maintain a translation layer forever. Keeping it in
// one file makes the cost visible rather than smeared through the verbs, and
// gives it a single place to be tested.
//
// The failure mode of a hand-written converter is not a crash. It is silently
// dropping a field when an inner type grows, and shipping a projection that
// looks complete. convert_test.go closes that: every inner field must be
// carried across or explicitly justified as omitted.

// projectGrid maps a room's grid family onto the wire enum.
//
// An unrecognised shape yields the empty string rather than a guess. It is
// unreachable — the composition rejects unknown grid families at both Setup
// and Load — but inventing a plausible value for something we do not
// understand would turn an impossible state into a wrong answer, which is
// strictly worse than an obviously absent one.
func projectGrid(shape spatial.GridShape) GridKind {
	switch shape {
	case spatial.GridShapeSquare:
		return GridSquare
	case spatial.GridShapeHex:
		return GridHex
	default:
		return ""
	}
}

// projectLayout maps the composition's authoring frame onto the wire's render
// word (rpg-toolkit#1140). Nil — a square field, which declares no orientation
// by law — yields the empty string, which is what keeps Atlas.Layout's
// "present exactly when the grid is hex" true.
//
// The default arm is unreachable, the way projectGrid's is: encounter.Orientation
// is SEALED by an unexported method, so the only kinds that can exist are the
// two named here, and a hex field must declare one of them. It still refuses
// to guess rather than invent a layout, for the reason projectGrid gives — a
// guess would turn an impossible state into a wrong picture.
func projectLayout(o encounter.Orientation) HexLayout {
	if o == nil {
		return ""
	}
	switch o.Kind() {
	case encounter.OrientationPointyTop:
		return HexLayoutPointyTop
	case encounter.OrientationFlatTop:
		return HexLayoutFlatTop
	default:
		return ""
	}
}

// projectAtlas flattens the composition's room-by-room map into one map.
//
// The composition answers in its own terms — a footprint per room, a doorway
// naming the two it joins — because rooms are how it holds a field together.
// This is where that stops being anybody else's problem: the cells are
// concatenated and sorted, and a doorway becomes the pair of adjacent cells
// it always was in absolute space.
//
// Sorting matters more than it looks. Concatenating room by room would leave
// the grouping perfectly visible in the order, so a client could reconstruct
// the decomposition the reshape exists to hide — and would eventually depend
// on it. One order, derived from the coordinates themselves.
func projectAtlas(in encounter.Atlas) Atlas {
	out := Atlas{Layout: projectLayout(in.Orientation), Doorways: make([]AtlasDoorway, 0, len(in.Doorways))}

	for _, region := range in.Regions {
		// W1: every region in a field shares one grid family, so the last
		// writer wins and every writer agrees.
		out.Grid = projectGrid(region.Grid)
		out.Cells = append(out.Cells, regionCells(region, in.Orientation)...)

		for _, prop := range region.Props {
			out.Props = append(out.Props, AtlasProp{
				Ref:               prop.Ref,
				At:                prop.At,
				BlocksMovement:    prop.BlocksMovement,
				BlocksLineOfSight: prop.BlocksLineOfSight,
			})
		}

		for _, b := range region.Boundaries {
			out.Boundaries = append(out.Boundaries, AtlasBoundary{
				From:              b.From,
				To:                b.To,
				BlocksMovement:    b.BlocksMovement,
				BlocksLineOfSight: b.BlocksLineOfSight,
			})
		}
	}

	sortCells(out.Cells)
	sort.Slice(out.Props, func(i, j int) bool {
		if out.Props[i].At != out.Props[j].At {
			return before(out.Props[i].At, out.Props[j].At)
		}
		return out.Props[i].Ref < out.Props[j].Ref
	})
	sort.Slice(out.Boundaries, func(i, j int) bool {
		if out.Boundaries[i].From != out.Boundaries[j].From {
			return before(out.Boundaries[i].From, out.Boundaries[j].From)
		}
		return before(out.Boundaries[i].To, out.Boundaries[j].To)
	})

	for _, d := range in.Doorways {
		out.Doorways = append(out.Doorways, AtlasDoorway{
			Connection: d.Connection,
			From:       d.FromCell,
			To:         d.ToCell,
		})
	}

	return out
}

// regionCells enumerates the cells a region owns, in dungeon-absolute space.
//
// A region describes itself as "a Width x Height rectangle anchored HERE"
// rather than by listing cells (rpg-toolkit#1127), which is the whole reason
// [encounter.Atlas] reports an Orientation: on hex, the same rectangle covers
// a different set of cells under each layout, so the frame is not optional.
//
// # Why the anchor is added in OFFSET space
//
// An authored rectangle SHEARS when it becomes axial. Adding the anchor in
// axial instead would put that shear back one level up — two chambers anchored
// six columns apart would land at axial distances that vary by row. Three
// callers inside the composition got this wrong before #1131 found them, so
// this is not a hypothetical: it is the arithmetic with a track record.
//
// This is a SECOND implementation of a projection the composition also owns,
// which is a thing worth being nervous about. It is kept honest rather than
// trusted: TestEveryProjectedCellIsOwnedByItsRegion asks the composition's own
// RegionAt about every cell this produces, so the two answers cannot drift
// apart without a test failing. If the composition ever exports the
// enumeration itself, this should be deleted in favour of it.
func regionCells(r encounter.AtlasRegion, o encounter.Orientation) []spatial.Position {
	out := make([]spatial.Position, 0, r.Width*r.Height)

	for row := 0; row < r.Height; row++ {
		for col := 0; col < r.Width; col++ {
			if r.Grid == spatial.GridShapeHex {
				out = append(out, encounter.HexCellAt(o, col+int(r.Origin.X), row+int(r.Origin.Y)))
				continue
			}

			out = append(out, spatial.Position{
				X: r.Origin.X + float64(col),
				Y: r.Origin.Y + float64(row),
			})
		}
	}

	return out
}

// sortCells puts positions in one deterministic order: by X, then by Y.
func sortCells(cells []spatial.Position) {
	sort.Slice(cells, func(i, j int) bool { return before(cells[i], cells[j]) })
}

// before is the map's coordinate order, shared by every sorted list on the
// Atlas so that two of them can be read side by side.
func before(a, b spatial.Position) bool {
	if a.X != b.X {
		return a.X < b.X
	}
	return a.Y < b.Y
}

func projectStatus(in *encounter.Status) *Status {
	if in == nil {
		return nil
	}
	out := &Status{Open: in.Open}
	if in.Outcome == nil {
		return out
	}

	outcome := &Outcome{
		Ending:  in.Outcome.Ending,
		At:      in.Outcome.At,
		Members: make([]MemberOutcome, 0, len(in.Outcome.Members)),
	}
	for _, m := range in.Outcome.Members {
		outcome.Members = append(outcome.Members, projectMemberOutcome(m))
	}
	out.Outcome = outcome
	return out
}

func projectSightings(in []intel.Holding) []Sighting {
	out := make([]Sighting, 0, len(in))
	for _, h := range in {
		via := make([]string, 0, len(h.CurrentVia))
		for _, c := range h.CurrentVia {
			via = append(via, string(c))
		}
		out = append(out, Sighting{
			Subject:    string(h.Subject),
			Seen:       projectSeen(h.Channel, h.Payload),
			Payload:    append([]byte(nil), h.Payload...),
			Channel:    string(h.Channel),
			At:         h.At,
			CurrentVia: via,
			Status:     string(h.Status),
		})
	}
	return out
}

// projectSeen copies the sight channel's typed knowledge into Seen (ADR-0041).
// It is nil for every channel but sight, and nil if a sight-channel payload
// somehow fails to decode — an impossible state today (the composition is the
// only writer of sight payloads) that a wrong Position would be worse than
// admitting to.
//
// The decode itself happens in encounter.DecodeSightPayload, not here: this
// package never calls encoding/json on a payload. h.Channel is intel's own
// provenance field — a holding's last accepted testimony — so a held memory
// (CurrentVia empty) still carries the channel and payload that produced it,
// and still gets a Seen: "a memory keeps its last Seen" (the ADR's words).
func projectSeen(channel intel.Channel, payload []byte) *Seen {
	if channel != intel.Sight {
		return nil
	}
	pos, ok := encounter.DecodeSightPayload(payload)
	if !ok {
		return nil
	}
	return &Seen{Position: pos}
}

// projectStory drops each entry's audience roster deliberately.
//
// A story is queried by one viewer, and the roster names every OTHER viewer a
// beat was addressed to. Handing that back would tell Alice which members
// exist and were present for something — including members she has never
// perceived, and members in rooms she has never entered. The audience is a
// delivery rule, not story content, and it is the composition's business
// rather than the host's.
func projectStory(in []record.Entry) []StoryEntry {
	out := make([]StoryEntry, 0, len(in))
	for _, e := range in {
		var tags map[string]string
		if len(e.Tags) > 0 {
			tags = make(map[string]string, len(e.Tags))
			for k, v := range e.Tags {
				tags[k] = v
			}
		}
		out = append(out, StoryEntry{
			Seq:         e.Seq,
			At:          e.At,
			Correlation: e.Correlation,
			Tags:        tags,
			Payload:     append([]byte(nil), e.Payload...),
		})
	}
	return out
}

func projectMember(in encounter.Member) Member {
	return Member{
		ID:       string(in.ID),
		Kind:     MemberKind(in.Kind),
		Name:     in.Name,
		Position: in.Position,
	}
}

// onMap is gone (rpg-toolkit#1059), and its absence is the point.
//
// It projected a composition room-local cell onto the one map this seam
// speaks, and what was left for it to do had been shrinking for three slices:
// placement reads came back absolute at encounter v0.10.0, outcomes at v0.13.0
// (rpg-toolkit#1068), and the last room-local cells anywhere near this package
// were the ones the walk handed down to the composition's Move and read back
// out of it. The step verb takes and reports absolute cells, so there is
// nothing left to project.
//
// Its error path is why deleting it beats keeping it: an unprojectable cell
// returned the cell UNPROJECTED, on the grounds that a wrong answer a test can
// see beats a panic in a host. That fallback measurably masked a real bug —
// after #1068, projectMemberOutcome would have anchored every outcome twice,
// and every fixture hid it because their rooms sit far enough out that an
// absolute cell is never also a legal local one, so the second projection was
// refused and the refusal was swallowed (#1053, PR #1072's evidence). A helper
// with no callers cannot swallow anything.

// projectMemberOutcome carries the outcome's cell across unchanged.
//
// It used to project one: an outcome was the last shape the composition still
// reported room-local, so the seam re-asked for an absolute cell through
// Absolute. Encounter v0.13.0 reports the outcome on the dungeon map itself
// (rpg-toolkit#1068), and the same anchoring applied twice would put every
// member off by their room's origin — in the one report a host reads after
// there is nothing left to check it against.
func projectMemberOutcome(in encounter.MemberOutcome) MemberOutcome {
	return MemberOutcome{
		ID:       string(in.ID),
		Position: in.Position,
	}
}

// projectOutcome converts a bare outcome. projectStatus has its own inline
// copy of this walk because it must also handle the nil-Status case; this one
// serves the verbs that return an outcome directly.
func projectOutcome(in *encounter.Outcome) *Outcome {
	if in == nil {
		return nil
	}
	out := &Outcome{
		Ending:  in.Ending,
		At:      in.At,
		Members: make([]MemberOutcome, 0, len(in.Members)),
	}
	for _, m := range in.Members {
		out.Members = append(out.Members, projectMemberOutcome(m))
	}
	return out
}

// projectDiscoveries converts the per-observer perception deltas a verb
// produced.
//
// Observers with a nil delta are skipped rather than given an empty entry: a
// present key means "something changed for this observer", and manufacturing
// empty entries for everyone who happened to be in the encounter would make
// the map's size meaningless to a caller deciding whom to notify.
func projectDiscoveries(in map[encounter.MemberID]*intel.SurveilOutput) map[string]Discovery {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]Discovery, len(in))
	for id, delta := range in {
		if delta == nil {
			continue
		}
		out[string(id)] = projectDiscovery(delta)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func projectDiscovery(in *intel.SurveilOutput) Discovery {
	out := Discovery{}
	for _, r := range in.FirstContact {
		out.FirstContact = append(out.FirstContact, Report{
			Subject: string(r.Subject),
			Seen:    projectReportSeen(r.Payload),
			Payload: append([]byte(nil), r.Payload...),
		})
	}
	for _, s := range in.Refreshed {
		out.Refreshed = append(out.Refreshed, string(s))
	}
	for _, s := range in.Faded {
		out.Faded = append(out.Faded, string(s))
	}
	return out
}

// projectReportSeen decodes first-contact's Seen the same way projectSeen
// does, but cannot gate on channel the way projectSeen does: intel.Report
// carries no Channel of its own — a SurveilOutput is scoped to the one
// Channel its Surveil call used, but that channel is not threaded back onto
// each Report inside it. So this is decode-and-see rather than a channel
// check.
//
// That is equivalent to projectSeen's guard ONLY as long as sight is the only
// channel any composition surveils with — true today, since rebuildPercepts
// (encounter.go, refreshSight) is the sole Surveil call site in this
// codebase and always passes intel.Sight. The day a second channel starts
// calling Surveil, an undecodable payload here stops meaning "not sight" and
// starts meaning "channel this SDK has not typed yet OR truly bad bytes" —
// indistinguishable from here.
// TestProjectReportSeenCannotDistinguishSightFromALookalikePayload
// (seen_internal_test.go) documents the risk rather than closing it: closing
// it needs SurveilOutput (or the percept it is built from) to carry its own
// channel, which is a play/intel change outside this PR's scope.
func projectReportSeen(payload []byte) *Seen {
	pos, ok := encounter.DecodeSightPayload(payload)
	if !ok {
		return nil
	}
	return &Seen{Position: pos}
}
