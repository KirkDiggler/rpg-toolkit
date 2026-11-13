// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// arrest.go is THE ARREST CONSEQUENCE (rpg-project#925, slice one; design
// #916 R7 and R11 as ruled 2026-11-13): an interaction entering a state may
// ARREST, and what it does is decided once, at that instant, and kept.
//
// # Who is taken, and who is wanted
//
// The speaker's party — the members of the speaker's faction, the speaker
// included ([Encounter.partyOf]) — is split three ways the moment the
// interaction enters the state:
//
//   - TAKEN: a member standing in the authored ARREST AREA ([ArrestArea]: an
//     explicit list of regions and cells, never an implicit distance), or
//     whom any of the ARRESTING GUARDS sees on sight at that instant. The
//     arresting guards are the interaction's carriers — the members whose
//     lines read or step it ([carries]) — who are up. Sight is the live
//     answer the recognition grant reads ([Encounter.placedSightReach]),
//     never the stored testimony of the last pass.
//   - WANTED: a member who is not taken and whom an arresting guard holds a
//     record of on sight: a sighting on the default topic of his intel
//     ([perception.Perception.On]) — seen, remembered or lost. A member who
//     was at the gate with the others and walked away before the arrest is
//     wanted; one no guard ever saw is not.
//   - NEITHER: everyone else.
//
// Each taken member is kept with its basis (in the area; seen by which
// guards) and each wanted member with the guards who hold its record
// ([ArrestRecord]). The record is stored with the interaction and reused on
// reload; a terminal state is entered once, so the split is never made
// again from changed positions or intel (R7).
//
// # What the guards come to hold
//
// The member the line was said to comes to hold [TopicArrested] about every
// taken member and [TopicWanted] about every wanted member, judged
// ([ChannelJudgement]); every other carrier who caught the line holds the
// same, heard ([ChannelHeard]) — exactly the write and the telling a
// `recognize` outcome makes ([Encounter.holdTopic]).
//
// # What it does NOT do yet
//
// Nobody is moved. The taken are recorded and the guards remember; walking
// them to the cell is the next slice (rpg-project#925's forced march).
// The gate is not released for anyone, taken or wanted.

// TopicArrested is the topic the arresting guards hold about each member
// they arrested (rpg-project#925): "guards who arrested the party remember
// that fact".
const TopicArrested TopicID = "arrested"

// TopicWanted is the topic the arresting guards hold about each member of
// the party they remember who was not there to be taken (rpg-project#925):
// to be arrested on sight.
const TopicWanted TopicID = "wanted"

// BeatArrested is the "beat" value of the story beat appended when an
// interaction arrests: the interaction, the cell they are to be held in, and
// the members taken and wanted that its reader perceives. EXPORTED BECAUSE A
// DECODER READS IT.
const BeatArrested = "arrested"

// AddressArrested: the interaction arrested the party, and its conversation
// with the member it addressed is over.
const AddressArrested AddressEndReason = "arrested"

// ErrBadArrest is returned when an interaction's arrest names a cell or an
// area the field does not have.
var ErrBadArrest = errors.New("encounter: bad arrest")

// Arrest is the consequence "the party is arrested, to be held in a cell"
// (rpg-project#925). It hangs only on an interaction's outcome, never on a
// line's row.
type Arrest struct {
	// Cell is the region the arrested are to be held in. REQUIRED, a region
	// of the field.
	Cell RegionID `json:"cell"`

	// Area is where a member of the party standing at the arrest is taken
	// whatever the guards see. REQUIRED non-empty.
	Area ArrestArea `json:"area"`
}

// ArrestArea is the arrest's explicit area (#916 R11: "Bind the area
// explicitly; no implementation may choose an implicit distance"): whole
// regions, and single cells besides.
type ArrestArea struct {
	// Regions are regions of the field, each wholly in the area.
	Regions []RegionID `json:"regions,omitempty"`

	// Cells are cells of the field's floor, in the authored frame a
	// [RegionInput]'s cells use.
	Cells []spatial.Position `json:"cells,omitempty"`
}

// ArrestRecord is what an arrest decided, the moment it was made: kept with
// the interaction, persisted, and never decided again (#916 R7).
type ArrestRecord struct {
	// State is the state whose entering arrested.
	State string `json:"state"`

	// Cell is [Arrest.Cell].
	Cell RegionID `json:"cell"`

	// At is the clock reading of the arrest.
	At uint64 `json:"at"`

	// Taken are the members arrested, in id order, each with its basis.
	Taken []ArrestedMember `json:"taken,omitempty"`

	// Wanted are the members remembered and not taken, in id order.
	Wanted []WantedMember `json:"wanted,omitempty"`
}

// ArrestedMember is one taken member and why.
type ArrestedMember struct {
	// ID is the member.
	ID MemberID `json:"id"`

	// InArea is true when the member stood in the arrest area.
	InArea bool `json:"in_area,omitempty"`

	// SeenBy are the arresting guards who saw the member at the arrest, in id
	// order.
	SeenBy []MemberID `json:"seen_by,omitempty"`
}

// WantedMember is one wanted member and which guards remember them.
type WantedMember struct {
	// ID is the member.
	ID MemberID `json:"id"`

	// RememberedBy are the arresting guards who hold a sighting of the
	// member, in id order.
	RememberedBy []MemberID `json:"remembered_by"`
}

// Arrests is every arrest the run's interactions have made, by interaction
// id: what the session reads to show who was taken and who is wanted.
func (e *Encounter) Arrests() map[InteractionID]ArrestRecord {
	out := map[InteractionID]ArrestRecord{}
	for id, r := range e.interactions {
		if r.arrest != nil {
			out[id] = cloneArrestRecord(*r.arrest)
		}
	}
	return out
}

// cloneArrest copies an arrest so no caller's value is aliased.
func cloneArrest(a Arrest) Arrest {
	a.Area.Regions = slices.Clone(a.Area.Regions)
	a.Area.Cells = slices.Clone(a.Area.Cells)
	return a
}

// cloneArrestRecord copies a record so no caller's value is aliased.
func cloneArrestRecord(r ArrestRecord) ArrestRecord {
	r.Taken = slices.Clone(r.Taken)
	for i := range r.Taken {
		r.Taken[i].SeenBy = slices.Clone(r.Taken[i].SeenBy)
	}
	r.Wanted = slices.Clone(r.Wanted)
	for i := range r.Wanted {
		r.Wanted[i].RememberedBy = slices.Clone(r.Wanted[i].RememberedBy)
	}
	return r
}

// validateArrestShape refuses an arrest nobody could author on purpose: no
// cell, and no area.
func validateArrestShape(what string, a Arrest) error {
	switch {
	case a.Cell == "":
		return fmt.Errorf("%s arrests to no cell", what)
	case len(a.Area.Regions) == 0 && len(a.Area.Cells) == 0:
		return fmt.Errorf("%s arrests from no area: name its regions or cells", what)
	}
	return nil
}

// validateArrests refuses an interaction's arrest whose cell or area regions
// the field does not have, or whose area cells are not floor — asked when
// the run is built or loaded, once the field is.
func validateArrests(interactions map[InteractionID]*interactionRecord, f *field) error {
	for _, id := range slices.Sorted(maps.Keys(interactions)) {
		outcomes := interactions[id].input.Outcomes
		for _, state := range slices.Sorted(maps.Keys(outcomes)) {
			for _, c := range outcomes[state] {
				if c.Arrest == nil {
					continue
				}
				what := fmt.Sprintf("interaction %q outcome %q", id, state)
				if _, ok := f.regionCells[c.Arrest.Cell]; !ok {
					return fmt.Errorf("%s arrests to %q, which is not a region: %w", what, c.Arrest.Cell, ErrBadArrest)
				}
				for _, region := range c.Arrest.Area.Regions {
					if _, ok := f.regionCells[region]; !ok {
						return fmt.Errorf("%s area names %q, which is not a region: %w", what, region, ErrBadArrest)
					}
				}
				for _, cell := range c.Arrest.Area.Cells {
					if !f.isFloor(f.cellAt(cell)) {
						return fmt.Errorf("%s area names %v, which is not floor: %w", what, cell, ErrBadArrest)
					}
				}
			}
		}
	}
	return nil
}

// inArrestArea reports whether an absolute cell is in the arrest's area.
func (e *Encounter) inArrestArea(a Arrest, cell spatial.Position) bool {
	if region, ok := e.field.regionOf(cell); ok && slices.Contains(a.Area.Regions, region) {
		return true
	}
	for _, authored := range a.Area.Cells {
		if e.field.cellAt(authored) == cell {
			return true
		}
	}
	return false
}

// arrestingGuards is the interaction's carriers who are up, in id order —
// every member whose lines read or step it.
func (e *Encounter) arrestingGuards(id InteractionID) ([]MemberID, error) {
	down, err := e.downNow()
	if err != nil {
		return nil, err
	}
	var out []MemberID
	for _, m := range slices.Sorted(maps.Keys(e.members)) {
		if !down[m] && carries(e.members[m].Options, id) {
			out = append(out, m)
		}
	}
	return out, nil
}

// applyArrest makes one interaction's arrest: decides the taken and the
// wanted on the live reach, stores the record, ends the interaction's
// address, writes the guards' records and tells the arrest. A record already
// made is never made again.
func (e *Encounter) applyArrest(r *interactionRecord, a Arrest, line recognitionLine) error {
	if r.arrest != nil {
		return nil
	}
	reach, err := e.placedSightReach()
	if err != nil {
		return err
	}
	guards, err := e.arrestingGuards(line.interaction)
	if err != nil {
		return err
	}
	made := ArrestRecord{State: line.state, Cell: a.Cell, At: line.at}
	for _, id := range e.partyOf(line.speaker) {
		if slices.Contains(guards, id) {
			continue
		}
		taken := ArrestedMember{ID: id}
		if at, placed := e.canvas.GetEntityPosition(string(id)); placed {
			taken.InArea = e.inArrestArea(a, at)
		}
		var remembered []MemberID
		for _, g := range guards {
			if reach.Reaches(perception.Sight, g, id) {
				taken.SeenBy = append(taken.SeenBy, g)
			}
			if _, held := e.intelLog.On(sightMember(g), sightMember(id)); held == nil {
				remembered = append(remembered, g)
			}
		}
		switch {
		case taken.InArea || len(taken.SeenBy) > 0:
			made.Taken = append(made.Taken, taken)
		case len(remembered) > 0:
			made.Wanted = append(made.Wanted, WantedMember{ID: id, RememberedBy: remembered})
		}
	}
	r.arrest = &made

	// THE CONVERSATION IS OVER (rpg-project#942's rule for a leaver): the
	// interaction addresses nobody once it has arrested.
	if r.address != nil {
		ended := *r.address
		r.address = nil
		if err := e.appendAddressBeat(BeatAddressEnded, line.interaction, ended, AddressArrested); err != nil {
			return err
		}
	}

	if err := e.tellArrest(reach, made, guards, line); err != nil {
		return err
	}
	if err := e.grantTopic(reach, TopicArrested, takenIDs(made), line); err != nil {
		return err
	}
	// THE WANTED ARE NOT TOLD THEY ARE WANTED, through the guards' record any
	// more than through the arrest's beat.
	wantedLine := line
	wantedLine.unaware = true
	if err := e.grantTopic(reach, TopicWanted, wantedIDs(made), wantedLine); err != nil {
		return err
	}
	return nil
}

// grantTopic is a recognition's write with its subjects given: the member
// the line was said to holds topic about subjects, judged, and every other
// carrier of the interaction who caught the line and is up holds the same,
// heard — [Encounter.grantRecognition]'s holders and channels.
func (e *Encounter) grantTopic(reach sightReach, topic TopicID, subjects []MemberID, line recognitionLine) error {
	holder := line.spokenTo
	if holder == "" || len(subjects) == 0 {
		return nil
	}
	if err := e.holdTopic(reach, holder, subjects, topic, ChannelJudgement, line); err != nil {
		return err
	}
	down, err := e.downNow()
	if err != nil {
		return err
	}
	carriers := line.witnesses
	if line.hearers != nil {
		carriers = line.hearers
	}
	for _, carrier := range carriers {
		if carrier == holder || carrier == line.speaker || down[carrier] {
			continue
		}
		m, ok := e.members[carrier]
		if !ok || !carries(m.Options, line.interaction) {
			continue
		}
		if err := e.holdTopic(reach, carrier, subjects, topic, ChannelHeard, line); err != nil {
			return err
		}
	}
	return nil
}

func takenIDs(r ArrestRecord) []MemberID {
	out := make([]MemberID, 0, len(r.Taken))
	for _, t := range r.Taken {
		out = append(out, t.ID)
	}
	return out
}

func wantedIDs(r ArrestRecord) []MemberID {
	out := make([]MemberID, 0, len(r.Wanted))
	for _, w := range r.Wanted {
		out = append(out, w.ID)
	}
	return out
}

// tellArrest appends the [BeatArrested] beats. Its readers are the
// arresting guards, every taken member and the line's witnesses; each is
// told only the members they perceive on the live reach — a taken member is
// always aware of being taken, a guard of whom he arrested — so a beat never
// names a member its reader cannot see. A wanted member is never told they
// are wanted: they perceive nothing of the guards deciding it, though a
// wanted witness is told the taken they see. A reader who perceives none is
// told nothing. Readers told the same share one beat, in the order first met
// ([Encounter.tellRecognition]'s shape).
func (e *Encounter) tellArrest(reach sightReach, made ArrestRecord, guards []MemberID, line recognitionLine) error {
	type telling struct {
		taken, wanted []string
		audience      []MemberID
	}
	var tellings []*telling
	byKey := map[string]*telling{}
	seen := map[MemberID]bool{}
	readers := append(append(slices.Clone(guards), takenIDs(made)...), line.witnesses...)
	for _, reader := range readers {
		if seen[reader] {
			continue
		}
		seen[reader] = true
		if _, ok := e.members[reader]; !ok {
			continue
		}
		guard := slices.Contains(guards, reader)
		// self says whether a reader is told about themselves: a taken member
		// knows they are taken; a wanted member, not taken, perceives nothing
		// of the guards deciding it, and is never told they are wanted.
		tell := func(ids []MemberID, self bool) []string {
			var out []string
			for _, id := range ids {
				if guard || (self && reader == id) || (reader != id && reach.Reaches(perception.Sight, reader, id)) {
					out = append(out, string(id))
				}
			}
			return out
		}
		taken, wanted := tell(takenIDs(made), true), tell(wantedIDs(made), false)
		if len(taken) == 0 && len(wanted) == 0 {
			continue
		}
		key := strings.Join(taken, "\x00") + "\x01" + strings.Join(wanted, "\x00")
		t, ok := byKey[key]
		if !ok {
			t = &telling{taken: taken, wanted: wanted}
			byKey[key] = t
			tellings = append(tellings, t)
		}
		t.audience = append(t.audience, reader)
	}
	for _, t := range tellings {
		payload := map[string]interface{}{
			"beat":        BeatArrested,
			"interaction": string(line.interaction),
			"cell":        string(made.Cell),
		}
		if len(t.taken) > 0 {
			payload["taken"] = t.taken
		}
		if len(t.wanted) > 0 {
			payload["wanted"] = t.wanted
		}
		beat, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("arrested beat: %w", err)
		}
		if _, err := e.appendBeat(&record.AppendInput{
			At: line.at, Audience: t.audience, Tags: map[string]string{"tag": BeatArrested}, Payload: beat,
		}); err != nil {
			return fmt.Errorf("arrested beat: %w", err)
		}
	}
	return nil
}
