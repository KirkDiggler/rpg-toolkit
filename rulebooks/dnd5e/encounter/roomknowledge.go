// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

const roomKnownPrefix = "known:room:"

func roomKnownKind(id RegionID) journal.Kind { return journal.Kind(roomKnownPrefix + id) }

// knownRooms folds learned layout independently of mutable subject testimony.
// The journal persists only the fact of discovery; the field owns the geometry.
func (w *encounterWorld) knownRooms(member MemberID) map[RegionID]bool {
	out := make(map[RegionID]bool)
	for _, fact := range w.log.All() {
		kind := string(fact.Kind)
		if strings.HasPrefix(kind, roomKnownPrefix) && fact.Actor == journal.EntityID(member) {
			out[strings.TrimPrefix(kind, roomKnownPrefix)] = true
		}
	}
	return out
}

// discoverRooms teaches whole fixed regions from this observer's sight pass.
// Concealed floor still requires its existing discovery mechanism. The learned
// fact makes no claim about occupants, props, or a door's current state.
func (e *Encounter) discoverRooms(observer MemberID, reach sightReach) error {
	known := e.world.knownRooms(observer)
	hidden := e.hiddenFrom(observer)
	ids := make([]RegionID, 0, len(e.field.regions))
	for _, region := range e.field.regions {
		ids = append(ids, region.ID)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if known[id] {
			continue
		}
		for _, cell := range e.field.regionCells[id] {
			if hidden.cells[cell] || !reach.reachesCell(observer, cell) {
				continue
			}
			if err := e.learnRoom(observer, id); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

// learnRoom is the layout-only writer. Sight and opening a door share it;
// neither operation records mutable contents as a consequence of learning floor.
func (e *Encounter) learnRoom(member MemberID, id RegionID) error {
	if e.world.knownRooms(member)[id] {
		return nil
	}
	next, err := e.NextStorySeq()
	if err != nil {
		return err
	}
	var before Atlas
	// Initial discovery precedes scene-opened and is restored by the snapshot.
	// Later discovery is a recipient-scoped addition on the existing story.
	if next > 1 {
		before, err = e.AtlasFor(member)
		if err != nil {
			return err
		}
	}
	_, err = e.world.log.Append(journal.Fact{
		Kind: roomKnownKind(id), Actor: journal.EntityID(member),
		Subject: journal.EntityID(member), Audience: journal.Audience{journal.EntityID(member)},
	})
	if err != nil {
		return fmt.Errorf("discover room %q: %w", id, err)
	}
	if next > 1 {
		return e.appendRoomRevealedBeat(member, id, before)
	}
	return nil
}

// discoverOpenedRooms gives the opener the rooms joined by the opened edges.
// Other observers learn only through their own perception pass.
func (e *Encounter) discoverOpenedRooms(member MemberID, door *doorRecord) error {
	if member == "" {
		return nil
	}
	hidden := e.hiddenFrom(member)
	for _, edge := range door.edges {
		for _, cell := range []spatial.Position{edge.From, edge.To} {
			id, owned := e.field.regionOf(cell)
			if !owned || hidden.cells[cell] {
				continue
			}
			if err := e.learnRoom(member, id); err != nil {
				return err
			}
		}
	}
	return nil
}

// undiscoveredFrom adds ordinary unexplored floor to concealment's projection
// mask. This is a delivery mask only: unknown ordinary floor is not a secret
// wall and must not acquire concealment's movement/probe rules.
func (e *Encounter) undiscoveredFrom(member MemberID) hiddenView {
	hidden := e.hiddenFrom(member)
	known := e.world.knownRooms(member)
	// Existing explicit concealment discovery may teach a slice of a region
	// without seeing it. Preserve that knowledge; ordinary room discovery is
	// not an authority to forget separately learned geometry.
	revealed := make(map[spatial.Position]bool)
	for i := range e.field.concealments {
		c := &e.field.concealments[i]
		if e.world.knowsConcealment(member, c.id) {
			for _, cell := range e.hiddenCellsOf(c) {
				revealed[cell] = true
			}
		}
	}
	for _, cell := range e.field.cells {
		region, owned := e.field.regionOf(cell)
		if (!owned || !known[region]) && !revealed[cell] {
			hidden.cells[cell] = true
		}
	}
	// Unowned scenery directly bordering learned floor belongs to its visible
	// boundary. Wall footing is added by the existing atlas projection below.
	for _, cell := range e.field.cells {
		if _, owned := e.field.regionOf(cell); owned {
			continue
		}
		for _, neighbor := range adjacencyGrid.GetNeighbors(cell) {
			region, owned := e.field.regionOf(neighbor)
			if owned && known[region] && !hidden.cells[neighbor] {
				delete(hidden.cells, cell)
				break
			}
		}
	}
	// An authored boundary brings its unowned footing, but not the adjoining
	// room's interior. Read the pre-boundary set so long chains do not flood.
	footing := make(map[spatial.Position]bool)
	for _, wall := range e.field.walls {
		from, to := e.field.cellAt(wall.From), e.field.cellAt(wall.To)
		if hidden.cells[from] && hidden.cells[to] {
			continue
		}
		for _, cell := range []spatial.Position{from, to} {
			if _, owned := e.field.regionOf(cell); !owned {
				footing[cell] = true
			}
		}
	}
	for cell := range footing {
		delete(hidden.cells, cell)
	}
	for _, door := range e.doors {
		if hidden.doors[door.id] {
			continue
		}
		if id, concealed := e.hiddenDoorConcealment(door.id); concealed && e.world.knowsConcealment(member, id) {
			continue
		}
		visible := false
		for _, edge := range door.edges {
			if !hidden.cells[edge.From] || !hidden.cells[edge.To] {
				visible = true
				break
			}
		}
		// Footprint doors are projected with their fixed footprint rather than edges.
		if len(door.edges) > 0 && !visible {
			hidden.doors[door.id] = true
		}
	}
	return hidden
}
