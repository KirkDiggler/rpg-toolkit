// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
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

func projectAtlas(in encounter.Atlas) Atlas {
	out := Atlas{
		Rooms:    make([]AtlasRoom, 0, len(in.Rooms)),
		Doorways: make([]AtlasDoorway, 0, len(in.Doorways)),
	}

	for _, room := range in.Rooms {
		projected := AtlasRoom{
			ID:         room.ID,
			Grid:       projectGrid(room.Grid),
			Origin:     room.Origin,
			Width:      room.Width,
			Height:     room.Height,
			Cells:      append([]spatial.Position(nil), room.Cells...),
			Occluders:  append([]spatial.Position(nil), room.Occluders...),
			Boundaries: make([]AtlasBoundary, 0, len(room.Boundaries)),
		}
		for _, b := range room.Boundaries {
			projected.Boundaries = append(projected.Boundaries, AtlasBoundary{
				From:              b.From,
				To:                b.To,
				BlocksMovement:    b.BlocksMovement,
				BlocksLineOfSight: b.BlocksLineOfSight,
			})
		}
		out.Rooms = append(out.Rooms, projected)
	}

	for _, d := range in.Doorways {
		out.Doorways = append(out.Doorways, AtlasDoorway{
			Connection: d.Connection,
			From:       d.From,
			FromCell:   d.FromCell,
			To:         d.To,
			ToCell:     d.ToCell,
		})
	}

	return out
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
