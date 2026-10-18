// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// WatchZonesInput selects the sneaking member whose red floor is read.
type WatchZonesInput struct {
	Member MemberID
}

// WatchZonesOutput is one member's red floor: a zone per watcher they see.
type WatchZonesOutput struct {
	// Zones is one entry per watcher the member holds live, sorted by watcher
	// id. Never nil; empty when the member is not sneaking, is unplaced, or
	// sees no watcher.
	Zones []WatchZone
}

// WatchZone is the floor one watcher would catch the viewer on, as far as
// the viewer knows the floor.
type WatchZone struct {
	// Watcher is the placed non-player whose zone this is.
	Watcher MemberID

	// Cells is the watcher's zone intersected with the viewer's known cells,
	// less every cell a concealment still hides from the viewer, sorted in
	// the map's coordinate order. Never nil. A zone may be empty when none of
	// it is floor the viewer knows.
	Cells []spatial.Position
}

// WatchZonesFor is THE RED FLOOR (sneaking design R15, rpg-project#619): for a
// sneaking member, the cells where each watcher they can see would catch
// them, so a host can draw them red without computing a zone of its own.
//
// ONE ZONE, ONE TEST. A zone's cells are those the judgement crosses into
// ([sightReach.inWatchZone], R9), asked of the same geometry
// [Encounter.SetSneaking] judges a declaration with: the floor drawn red is
// the floor a roll is asked on. It is never built from what the watcher
// knows, because knowledge lags sight, and never from an arc or a distance
// picked here — the encounter holds no tuning of its own. A watcher's
// supplied arc ([SheetFacts.WatchArc]) narrows it here exactly as it narrows
// the judgement, because both ask the one predicate (R14).
//
// ONLY WATCHERS THE VIEWER SEES (R15, fog-of-war R5). A zone is sent for a
// watcher — a placed member that is not a player, the judgement's own
// definition ([Encounter.watchersIn]) — only while the viewer holds it live:
// a sight holding with a non-empty CurrentVia. A remembered watcher is a
// ghost whose zone would tell the viewer where it stands now; an unseen one
// would hand the client a creature its eyes never gave it.
// A watcher whose sheet supplies no passive Perception still has a zone
// (R22): it rolls nothing, but it keeps its ordinary sight, which is the
// zone — no arc narrows a watcher that judges no sneak ([watchCones]) — so
// the floor drawn red is still the floor it sees the sneaker on.
//
// ONLY FLOOR THE VIEWER KNOWS. Each zone is cut to the viewer's known cells
// and loses every cell a concealment hides from them, the refusals
// [Encounter.VisibleCellsFor] makes, so a zone never draws the shape of
// floor the viewer has not found.
//
// A member who is not sneaking is sent no zones (R15): the red floor is the
// sneaker's alone, and a walker's sight is unchanged (R2).
//
// ErrNilInput for a nil input, ErrNotMember for a stranger; an unplaced
// member answers an empty, non-nil list. Nothing here is stored: the zones
// are derived from current positions and the [Sight] capability's answer at
// the moment of the read.
func (e *Encounter) WatchZonesFor(in *WatchZonesInput) (*WatchZonesOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	viewer, ok := e.members[in.Member]
	if !ok {
		return nil, fmt.Errorf("watch zones for %q: %w", in.Member, ErrNotMember)
	}
	out := &WatchZonesOutput{Zones: make([]WatchZone, 0)}
	if !viewer.Sneaking {
		return out, nil
	}
	if _, placed := e.canvas.GetEntityPosition(string(in.Member)); !placed {
		return out, nil
	}
	geometry, _, err := e.watchReach()
	if err != nil {
		return nil, fmt.Errorf("watch zones for %q: %w", in.Member, err)
	}
	known := e.world.knownCells(in.Member)
	hidden := e.hiddenFrom(in.Member)
	// watchersIn sorts by id, so the zones come out in watcher order.
	for _, watcher := range e.watchersIn(geometry, e.rosterIDs()) {
		live, err := e.holdsLive(in.Member, watcher)
		if err != nil {
			return nil, fmt.Errorf("watch zones for %q: %w", in.Member, err)
		}
		if !live {
			continue
		}
		zone := WatchZone{Watcher: watcher, Cells: make([]spatial.Position, 0)}
		// e.field.cells is sorted by cellBefore, so each zone is too.
		for _, cell := range e.field.cells {
			if known[cell] && !hidden.cells[cell] && geometry.inWatchZone(watcher, cell) {
				zone.Cells = append(zone.Cells, cell)
			}
		}
		out.Zones = append(out.Zones, zone)
	}

	return out, nil
}
