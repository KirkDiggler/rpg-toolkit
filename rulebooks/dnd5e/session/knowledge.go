// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ViewOutput is one observer's coherent creature, prop, door and sight-area view.
// Mutable objects are observations, not additions to fixed room geometry.
type ViewOutput struct {
	Sightings []Sighting     `json:"sightings"`
	Props     []PropSighting `json:"props"`
	Doors     []DoorSighting `json:"doors"`
	Areas     []SightArea    `json:"areas"`

	// VisibleCells is SEEN NOW for this observer alone (rpg-project#586 R1):
	// the encounter's [encounter.Encounter.VisibleCellsFor] answer, passed
	// through unchanged — this package neither observes nor computes it, and
	// it is never a party union. Sorted, never nil, and a subset of the same
	// member's atlas cells. A placed member always has its own cell; an
	// unplaced member has none. Derived on read and never stored.
	VisibleCells []spatial.Position `json:"visible_cells"`

	// Sneaking is whether this observer is sneaking now (rpg-project#606,
	// design R6): the encounter's [encounter.Encounter.Sneaking] answer for
	// the viewer, passed through unchanged. The mode is server state; a
	// client draws its toggle from this and never keeps a flag of its own.
	Sneaking bool `json:"sneaking"`

	// WatchZones is the red floor for this observer (rpg-project#619, design
	// R15): the encounter's [encounter.Encounter.WatchZonesFor] answer for the
	// viewer, passed through unchanged — the same zones in the same order, each
	// with the same cells — one per watcher the viewer holds live, cut to the
	// floor the viewer knows. Never nil; empty unless the observer is
	// sneaking. This package adds no zone logic of its own.
	WatchZones []WatchZone `json:"watch_zones"`
}

// WatchZone is the floor one watcher would catch the viewer on, as far as
// the viewer knows the floor (sneaking R15). The session owns the view's
// shape; the encounter's zone type stays inside the boundary, and this is
// its values copied field for field, never recomputed.
type WatchZone struct {
	// Watcher names the watcher whose zone this is, as Sighting.Subject does.
	Watcher string `json:"watcher"`

	// Cells are the zone's cells exactly as the encounter answered them:
	// sorted in the map's coordinate order, never nil, possibly empty.
	Cells []spatial.Position `json:"cells"`
}

// PropSighting is the latest observation of one mutable prop. Exactly one shape
// is present. ObservedEmpty means no spatial placement is asserted by that shape.
type PropSighting struct {
	Prop          *AtlasProp       `json:"prop,omitempty"`
	Placed        *AtlasPlacedProp `json:"placed,omitempty"`
	ObservedEmpty bool             `json:"observed_empty"`
	// Presentation was captured by the provider with this observation.
	Presentation *PropPresentation `json:"presentation,omitempty"`
	CurrentVia   []string          `json:"current_via"`
	Status       string            `json:"status"`
	At           uint64            `json:"at"`
}

// DoorSighting carries an observed door state, current or remembered.
type DoorSighting struct {
	Door       Door     `json:"door"`
	CurrentVia []string `json:"current_via"`
	Status     string   `json:"status"`
	At         uint64   `json:"at"`
}

// KnowledgeInput selects a character's knowledge in one session. Player must be
// the authenticated host principal, not another client-controlled selector.
type KnowledgeInput struct {
	Session string
	Member  string
	Player  string
}

// KnowledgeOutput restores known geometry and mutable observations from one
// loaded encounter. Seq is its recipient-local state cutoff, not acknowledgement
// of historical narration.
type KnowledgeOutput struct {
	// DiscoverySharing is the viewer's current preference.
	DiscoverySharing *bool        `json:"discovery_sharing,omitempty"`
	Atlas            Atlas        `json:"atlas"`
	View             ViewOutput   `json:"view"`
	Where            WhereOutput  `json:"where"`
	Roster           RosterOutput `json:"roster"`
	Holding          []string     `json:"holding"`
	Seq              uint64       `json:"seq"`
}

// Knowledge returns one coherent snapshot for an authenticated owned seat.
// It composes provider projections; it neither observes nor computes visibility.
func (m *Manager) Knowledge(ctx context.Context, in *KnowledgeInput) (*KnowledgeOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("knowledge: %w", ErrNilInput)
	}
	release, lockErr := m.acquireSession(ctx, in.Session)
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()
	if in.Member == "" || in.Player == "" {
		return nil, fmt.Errorf("knowledge: %w", ErrNoMemberID)
	}
	enc, data, err := m.openWithRecord(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("knowledge: %w", err)
	}
	roster, err := m.rosterFrom(ctx, enc, data, &RosterInput{Session: in.Session, Player: in.Player, Member: in.Member})
	if err != nil {
		return nil, err
	}
	atlas, err := enc.AtlasFor(encounter.MemberID(in.Member))
	if err != nil {
		return nil, translate(err)
	}
	view, err := projectView(enc, &ViewInput{Session: in.Session, Member: in.Member})
	if err != nil {
		return nil, err
	}
	at, err := standsAt(enc, encounter.MemberID(in.Member))
	if err != nil {
		return nil, err
	}
	held, err := enc.HeldProps(&encounter.ViewInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, translate(err)
	}
	world := enc.WorldView()
	_, cursors, err := buildStreamNumbers(enc, &world, data.Streams)
	if err != nil {
		return nil, err
	}
	projected := projectAtlas(atlas)
	projected.DungeonKey = data.Dungeon
	holding := make([]string, 0, len(held))
	for _, id := range held {
		holding = append(holding, string(id))
	}
	preference, err := enc.DiscoverySharing(&encounter.DiscoverySharingInput{Member: encounter.MemberID(in.Member)})
	if err != nil {
		return nil, translate(err)
	}
	sharing := &preference.Sharing
	return &KnowledgeOutput{DiscoverySharing: sharing, Atlas: projected, View: *view, Where: WhereOutput{Position: at}, Roster: *roster, Holding: holding, Seq: cursors[in.Member].Count}, nil
}

func projectObjectSightings(enc *encounter.Encounter, member string) ([]PropSighting, []DoorSighting, error) {
	input := &encounter.ViewInput{Member: encounter.MemberID(member)}
	props, err := enc.PropSightings(input)
	if err != nil {
		return nil, nil, translate(err)
	}
	doors, err := enc.DoorSightings(input)
	if err != nil {
		return nil, nil, translate(err)
	}
	propOut := make([]PropSighting, 0, len(props))
	for _, p := range props {
		status, via := sightingStatus(perception.Holding{CurrentVia: p.CurrentVia})
		out := PropSighting{ObservedEmpty: p.ObservedEmpty, CurrentVia: via, Status: status, At: p.At}
		if p.Prop != nil {
			projected := projectAtlasProp(*p.Prop)
			out.Prop = &projected
		}
		if p.Placed != nil {
			projected := projectAtlasPlaced(*p.Placed)
			out.Placed = &projected
		}
		if p.Presentation != nil {
			projected := projectPropPresentation(*p.Presentation)
			out.Presentation = &projected
		}
		propOut = append(propOut, out)
	}
	doorOut := make([]DoorSighting, 0, len(doors))
	for _, d := range doors {
		status, via := sightingStatus(perception.Holding{CurrentVia: d.CurrentVia})
		doorOut = append(doorOut, DoorSighting{Door: projectDoor(d.Door), CurrentVia: via, Status: status, At: d.At})
	}
	return propOut, doorOut, nil
}
