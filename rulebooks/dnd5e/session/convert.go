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
	out := Atlas{Doorways: make([]AtlasDoorway, 0, len(in.Doorways))}

	for _, room := range in.Rooms {
		// W1: every room in a field shares one grid family, so the last
		// writer wins and every writer agrees.
		out.Grid = projectGrid(room.Grid)
		out.Cells = append(out.Cells, room.Cells...)
		out.Occluders = append(out.Occluders, room.Occluders...)
		for _, b := range room.Boundaries {
			out.Boundaries = append(out.Boundaries, AtlasBoundary{
				From:              b.From,
				To:                b.To,
				BlocksMovement:    b.BlocksMovement,
				BlocksLineOfSight: b.BlocksLineOfSight,
			})
		}
	}

	sortCells(out.Cells)
	sortCells(out.Occluders)
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
		outcome.Members = append(outcome.Members, MemberOutcome{
			ID:       string(m.ID),
			Room:     m.Room,
			Position: m.Position,
		})
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
			Payload:    append([]byte(nil), h.Payload...),
			Channel:    string(h.Channel),
			At:         h.At,
			CurrentVia: via,
			Status:     string(h.Status),
		})
	}
	return out
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
		ID:   string(in.ID),
		Kind: MemberKind(in.Kind),
		Room: in.Room,
	}
}

func projectMemberOutcome(in encounter.MemberOutcome) MemberOutcome {
	return MemberOutcome{
		ID:       string(in.ID),
		Room:     in.Room,
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
