// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// PropSighting is one observer's latest prop testimony, not live placement.
// Exactly one shape is present. ObservedEmpty disproves its spatial placement
// without revealing a destination or carrier. CurrentVia is empty for memory.
type PropSighting struct {
	Prop          *AtlasProp
	Placed        *AtlasPlacedProp
	ObservedEmpty bool
	// Presentation is captured with the observation, never refreshed from a
	// newer live pose while returning memory. Absent on observed-empty.
	Presentation *PropPresentation
	CurrentVia   []perception.Channel
	At           uint64
}

// DoorSighting is an observer's latest door state, including remembered locks.
// Its state is never refreshed by a read of the live door.
type DoorSighting struct {
	Door       Door
	CurrentVia []perception.Channel
	At         uint64
}

type propObservation struct {
	Prop          *AtlasProp            `json:"prop,omitempty"`
	Placed        *AtlasPlacedProp      `json:"placed,omitempty"`
	ObservedEmpty bool                  `json:"observed_empty,omitempty"`
	Presentation  *PropPresentationData `json:"presentation,omitempty"`
}

func (p propObservation) id() PropID {
	if p.Prop != nil {
		return p.Prop.ID
	}
	if p.Placed != nil {
		return p.Placed.ID
	}
	return ""
}

func (p propObservation) cells() []spatial.Position {
	if p.ObservedEmpty {
		return nil
	}
	if p.Prop != nil {
		return []spatial.Position{p.Prop.At}
	}
	if p.Placed != nil {
		return p.Placed.Cells
	}
	return nil
}

func decodePropObservation(payload []byte) (propObservation, error) {
	var out propObservation
	if err := json.Unmarshal(payload, &out); err != nil {
		return out, fmt.Errorf("prop testimony: %w", ErrInvalidData)
	}
	if (out.Prop == nil) == (out.Placed == nil) || out.id() == "" {
		return out, fmt.Errorf("prop testimony needs one identified shape: %w", ErrInvalidData)
	}
	if !out.ObservedEmpty && len(out.cells()) == 0 {
		return out, fmt.Errorf("prop testimony has no placement: %w", ErrInvalidData)
	}
	if out.Presentation != nil {
		if out.ObservedEmpty || out.Presentation.ID != out.id() {
			return out, fmt.Errorf("prop testimony has conflicting presentation: %w", ErrInvalidData)
		}
		if err := validatePropPresentation(propPresentationFromData(*out.Presentation)); err != nil {
			return out, fmt.Errorf("prop testimony has invalid presentation: %w", ErrInvalidData)
		}
	}
	return out, nil
}

// PropSightings returns only the caller's stored prop observations, sorted by ID.
// A room reveal does not create a prop sighting. ErrNotMember rejects strangers.
func (e *Encounter) PropSightings(in *ViewInput) ([]PropSighting, error) {
	held, err := e.observerHoldings(in)
	if err != nil {
		return nil, err
	}
	out := make([]PropSighting, 0)
	for _, h := range held {
		if _, ok := subjectID(h.Subject, propSubjectKind); !ok || h.Channel != perception.Sight {
			continue
		}
		p, err := decodePropObservation(h.Payload)
		if err != nil {
			return nil, err
		}
		var presentation *PropPresentation
		if p.Presentation != nil {
			value := propPresentationFromData(*p.Presentation)
			presentation = &value
		}
		out = append(out, PropSighting{Prop: p.Prop, Placed: p.Placed, ObservedEmpty: p.ObservedEmpty, Presentation: presentation, CurrentVia: h.CurrentVia, At: h.Observed})
	}
	return out, nil
}

// DoorSightings returns stored door observations with the same currency as
// creatures. Merely knowing a doorway does not disclose its state.
func (e *Encounter) DoorSightings(in *ViewInput) ([]DoorSighting, error) {
	held, err := e.observerHoldings(in)
	if err != nil {
		return nil, err
	}
	out := make([]DoorSighting, 0)
	for _, h := range held {
		if _, ok := subjectID(h.Subject, doorSubjectKind); !ok || h.Channel != perception.Sight {
			continue
		}
		var data DoorData
		if err := json.Unmarshal(h.Payload, &data); err != nil {
			return nil, fmt.Errorf("door testimony: %w", ErrInvalidData)
		}
		state, err := doorStateFromData(data.ID, data.State, data.Lock)
		if err != nil {
			return nil, err
		}
		door := Door{ID: data.ID, State: state}
		for _, edge := range data.Edges {
			door.Edges = append(door.Edges, DoorEdge{From: spatial.Position{X: edge.From.X, Y: edge.From.Y}, To: spatial.Position{X: edge.To.X, Y: edge.To.Y}})
		}
		if data.Placement != nil {
			p := placementFromData(*data.Placement)
			door.Placement = &p
		}
		out = append(out, DoorSighting{Door: door, CurrentVia: h.CurrentVia, At: h.Observed})
	}
	return out, nil
}

func (e *Encounter) observerHoldings(in *ViewInput) ([]perception.Holding, error) {
	if in == nil {
		return nil, fmt.Errorf("observations: %w", ErrNilInput)
	}
	if _, ok := e.members[in.Member]; !ok {
		return nil, fmt.Errorf("observations: %w", ErrNotMember)
	}
	return e.intelLog.Held(sightMember(in.Member))
}

type observationSupport struct {
	cells []spatial.Position
	prop  PropID
	door  DoorID
}

type observationReach struct {
	sightReach
	supports map[core.EntityID]observationSupport
	hidden   map[MemberID]hiddenView
}

func (r observationReach) Reaches(channel perception.Channel, observer, subject core.EntityID) bool {
	member, ok := subjectID(observer, memberSubjectKind)
	if !ok {
		return false
	}
	if other, ok := subjectID(subject, memberSubjectKind); ok {
		return r.sightReach.Reaches(channel, member, other)
	}
	support, ok := r.supports[subject]
	if !ok {
		return false
	}
	hidden := r.hidden[member]
	if hidden.props[support.prop] || hidden.doors[support.door] {
		return false
	}
	for _, cell := range support.cells {
		if !hidden.cells[cell] && r.reachesCell(member, cell) {
			return true
		}
	}
	return false
}

// appendMutablePresences extends the same complete sight pass used by creatures.
// Footprints retain their canonical shape and support; no anchor cell is invented.
func (e *Encounter) appendMutablePresences(presences []perception.Presence, geometry sightReach) ([]perception.Presence, observationReach, error) {
	reach := observationReach{sightReach: geometry, supports: make(map[core.EntityID]observationSupport), hidden: make(map[MemberID]hiddenView)}
	for observer := range geometry.positions {
		reach.hidden[observer] = e.hiddenFrom(observer)
	}
	atlas, err := e.Atlas()
	if err != nil {
		return nil, reach, err
	}
	presentations := make(map[PropID]*PropPresentationData, len(atlas.PropPresentations))
	for _, p := range atlas.PropPresentations {
		data := propPresentationData(p)
		presentations[p.ID] = &data
	}
	for _, prop := range atlas.Props {
		if !prop.Holdable {
			continue
		}
		payload, err := json.Marshal(propObservation{Prop: &prop, Presentation: presentations[prop.ID]})
		if err != nil {
			return nil, reach, err
		}
		id := sightProp(prop.ID)
		presences = append(presences, perception.Presence{ID: id, Payload: payload})
		reach.supports[id] = observationSupport{cells: []spatial.Position{prop.At}, prop: prop.ID}
	}
	for _, prop := range atlas.Placed {
		if !prop.Holdable {
			continue
		}
		payload, err := json.Marshal(propObservation{Placed: &prop, Presentation: presentations[prop.ID]})
		if err != nil {
			return nil, reach, err
		}
		id := sightProp(prop.ID)
		presences = append(presences, perception.Presence{ID: id, Payload: payload})
		reach.supports[id] = observationSupport{cells: prop.Cells, prop: prop.ID}
	}
	for _, door := range e.doors {
		payload, err := json.Marshal(doorDataFrom(door))
		if err != nil {
			return nil, reach, err
		}
		support := observationSupport{door: door.id}
		for _, edge := range door.edges {
			support.cells = append(support.cells, edge.From, edge.To)
		}
		if door.placement != nil {
			cells, supportErr := e.field.footprintObservationCells(*door.placement)
			if supportErr != nil {
				return nil, reach, fmt.Errorf("door %q observation support: %w", door.id, supportErr)
			}
			support.cells = cells
		}
		id := sightDoor(door.id)
		reach.supports[id] = support
		presences = append(presences, perception.Presence{ID: id, Payload: payload})
	}
	return presences, reach, nil
}

// mutableWitnesses captures the original audience of a factual change. Merely
// remembering an object is not permission to learn its unseen current state.
func (e *Encounter) mutableWitnesses(subject core.EntityID, actor MemberID) ([]MemberID, error) {
	ranges, err := e.sightNow()
	if err != nil {
		return nil, err
	}
	geometry := sightReach{positions: make(map[MemberID]spatial.Position), cells: ranges, canvas: e.canvas, areas: e.sightAreas}
	for _, member := range e.rosterIDs() {
		if cell, placed := e.canvas.GetEntityPosition(string(member)); placed {
			geometry.positions[member] = cell
		}
	}
	_, reach, err := e.appendMutablePresences(nil, geometry)
	if err != nil {
		return nil, err
	}
	out := make([]MemberID, 0)
	for _, member := range e.rosterIDs() {
		if member == actor {
			out = append(out, member)
			continue
		}
		if !reach.Reaches(perception.Sight, sightMember(member), subject) {
			continue
		}
		// A beat naming a carrier must not identify an unseen actor.
		if actor != "" && !geometry.Reaches(perception.Sight, member, actor) {
			continue
		}
		out = append(out, member)
	}
	return out, nil
}

// correctEmptyProps runs after the complete pass has retired current sight.
// Omission alone does nothing: every cell supporting the remembered placement
// must be positively visible before we can disprove that placement.
func (e *Encounter) correctEmptyProps(observer MemberID, reach observationReach, at uint64) (bool, error) {
	held, err := e.intelLog.Held(sightMember(observer))
	if err != nil {
		return false, err
	}
	changed := false
	for _, h := range held {
		if _, prop := subjectID(h.Subject, propSubjectKind); !prop || h.CurrentOn(perception.Sight) || h.Channel != perception.Sight {
			continue
		}
		p, err := decodePropObservation(h.Payload)
		if err != nil {
			return false, err
		}
		if p.ObservedEmpty {
			continue
		}
		complete := len(p.cells()) > 0 && !reach.hidden[observer].props[p.id()]
		for _, cell := range p.cells() {
			if reach.hidden[observer].cells[cell] || !reach.reachesCell(observer, cell) {
				complete = false
				break
			}
		}
		if !complete {
			continue
		}
		p.ObservedEmpty = true
		p.Presentation = nil
		if p.Prop != nil {
			p.Prop.At = spatial.Position{}
		}
		if p.Placed != nil {
			p.Placed.Placement = spatial.FootprintPlacement{}
			p.Placed.Cells = nil
		}
		payload, err := json.Marshal(p)
		if err != nil {
			return false, err
		}
		_, err = e.intelLog.Report(perception.ReportInput{Observer: sightMember(observer), Channel: perception.Sight, At: at, Reports: []perception.Presence{{ID: h.Subject, Payload: payload}}})
		if err != nil {
			return false, err
		}
		changed = true
	}
	return changed, nil
}
