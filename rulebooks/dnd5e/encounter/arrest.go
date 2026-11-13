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
// The speaker's party — the PLAYER characters of the speaker's faction, the
// speaker included ([Encounter.arrestParty]) — is split three ways the moment
// the interaction enters the state (or, for a kept roll settled after its
// speaker walked away, as its words were said):
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
// # The taken are HELD (custody)
//
// From the arrest the taken are in custody ([ArrestRecord.Custody] is
// [CustodyHeld]): they cannot walk away. [Encounter.Step] — a dash
// included, which is a Step on more movement — [Encounter.SetSneaking]
// turning the mode on, and a cast whose landing moves its own caster refuse
// a member in custody with [ErrInCustody]; a taken member's sneak ends at
// the arrest. Speaking, looking, looting in reach, attacking and casting
// stay open: a fight is the one other way out (rpg-project#925, ruled
// 2026-11-13: "either held and walked off or they can fight"). [Encounter.Direct]
// is not gated: the march will move them by it. [Encounter.Exit] still
// lets a taken member leave the run, and the record keeps them taken.
//
// While custody holds, nothing a host can ask for moves a taken member: any
// [Encounter.Direct] on one — a friend's shove, a flee — and any
// [Encounter.Relocate] of one is refused with a [CustodyMoveError]. Only the
// engine's own custody consequence, which no request can name, relocates
// the held ([Encounter.relocate] with custody set). A cast lands on
// a held member as on anyone — the save, the damage, a condition — but its
// push does not move them, and its moved result says so ([HeldInPlace]);
// only a push that would carry the held caster themself refuses the cast.
//
// Custody ends — [CustodyBroken], for every taken member at once — when a
// fight forms or is joined with an arresting guard in it
// ([Encounter.breakCustody]). A fight without them (a taken member and a
// stranger) frees nobody, and the taken stay held inside it. The record's
// taken and wanted, and the topics the guards hold, stay.
//
// # What it does NOT do yet
//
// Nobody is moved. The taken are recorded, held, and the guards remember;
// walking them to the cell is the next slice (rpg-project#925's forced
// march), which [CustodyMarching] and [CustodyJailed] are named for. The
// gate is not released for anyone, taken or wanted.

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

// ErrInCustody is returned when a member the guards hold tries to walk away
// ([Encounter.Step], [Encounter.SetSneaking], a cast that moves its caster).
// Its text is the sentence the player is shown.
var ErrInCustody = errors.New("The guards hold you; you cannot walk away.") //nolint:staticcheck // the player's sentence

// CustodyMoveError is the refusal of a move imposed on a member the guards
// hold — a shove, a flee, any [Encounter.Direct] or [Encounter.Relocate]. It
// is [ErrInCustody] to errors.Is,
// and its text is the sentence the actor is shown.
type CustodyMoveError struct {
	// Mover is the held member the move was imposed on.
	Mover MemberID
}

// Error is the actor's sentence.
func (e *CustodyMoveError) Error() string {
	return fmt.Sprintf("The guards hold %s; they cannot be moved.", e.Mover)
}

// Is makes a CustodyMoveError an [ErrInCustody].
func (e *CustodyMoveError) Is(target error) bool { return target == ErrInCustody }

// CustodyPhase is where an arrest's custody of the taken stands.
type CustodyPhase string

// The custody phases. Only held and broken are entered today; marching and
// jailed are named for the forced march so the stored shape does not change.
// A record saved before custody existed has no phase, and reads as held: the
// taken were never released.
const (
	// CustodyHeld: the taken are held where they were taken.
	CustodyHeld CustodyPhase = "held"
	// CustodyMarching: the taken are being walked to the cell (not yet entered).
	CustodyMarching CustodyPhase = "marching"
	// CustodyJailed: the taken are in the cell (not yet entered).
	CustodyJailed CustodyPhase = "jailed"
	// CustodyBroken: a fight ended custody; the taken walk freely.
	CustodyBroken CustodyPhase = "broken"
)

// known reports whether a phase is one of the four.
func (c CustodyPhase) known() bool {
	return c == CustodyHeld || c == CustodyMarching || c == CustodyJailed || c == CustodyBroken
}

// holds reports whether the phase holds the taken: held, marching or jailed.
func (c CustodyPhase) holds() bool {
	return c == CustodyHeld || c == CustodyMarching || c == CustodyJailed
}

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

	// Custody is where custody of the taken stands: [CustodyHeld] from the
	// arrest, [CustodyBroken] once a fight ends it.
	Custody CustodyPhase `json:"custody"`
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

// arrestParty is whom an arrest can take or want: the PLAYER characters of
// the speaker's faction, the speaker among them when they are one, in id
// order. A creature authored into the party's faction is never taken and
// never wanted: the party is players only, to start, as the sharing dial
// rules it (rpg-project#935); the faction stays the pathway, so widening it
// is this one function.
func (e *Encounter) arrestParty(speaker MemberID) []MemberID {
	var out []MemberID
	for _, id := range e.partyOf(speaker) {
		if e.members[id].Kind == KindPlayer {
			out = append(out, id)
		}
	}
	return out
}

// arrestPartyCells is where each player of the speaker's party stands now
// ([Encounter.arrestParty]), nil when none is placed: what a kept line's
// scene keeps ([ReservedScene.PartyAt]) so an arrest settled later is
// decided where they stood when the words were said.
func (e *Encounter) arrestPartyCells(speaker MemberID) map[MemberID]spatial.Position {
	var out map[MemberID]spatial.Position
	for _, id := range e.arrestParty(speaker) {
		if at, placed := e.canvas.GetEntityPosition(string(id)); placed {
			if out == nil {
				out = map[MemberID]spatial.Position{}
			}
			out[id] = at
		}
	}
	return out
}

// applyArrest makes one interaction's arrest: decides the taken and the
// wanted, stores the record, ends the interaction's address, writes the
// guards' records and tells the arrest. A record already made is never made
// again.
//
// THE MOMENT IS THE LINE'S (rpg-project#938's rule): a line judged as spoken
// — an abandoned settle of a kept roll — is decided in the scene of its
// words: where each player stood then ([ReservedScene.PartyAt]), whom the
// creature spoken to saw ([ReservedScene.Seen]) and whom each other guard
// who witnessed it saw ([ReservedScene.Perceived]). Walking away before the
// settle dodges nothing, and walking into view before it takes nobody. Any
// other line is decided on the live reach as it is said. Who is wanted is
// read from the guards' memory as it stands when the record is made.
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
	seenBy := func(g, id MemberID) bool {
		if !line.spoken {
			return reach.Reaches(perception.Sight, g, id)
		}
		if g == line.spokenTo {
			return slices.Contains(line.seen, id)
		}
		return slices.Contains(line.perceived[g], id)
	}
	made := ArrestRecord{State: line.state, Cell: a.Cell, At: line.at, Custody: CustodyHeld}
	for _, id := range e.arrestParty(line.speaker) {
		if slices.Contains(guards, id) {
			continue
		}
		taken := ArrestedMember{ID: id}
		at, placed := e.canvas.GetEntityPosition(string(id))
		if line.spoken {
			at, placed = line.partyAt[id]
		}
		if placed {
			taken.InArea = e.inArrestArea(a, at)
		}
		var remembered []MemberID
		for _, g := range guards {
			if seenBy(g, id) {
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
	// TAKEN, A SNEAK IS OVER: the guards have their hands on them.
	for _, t := range made.Taken {
		e.endSneaking(t.ID)
	}

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
	// WANTED IS THE GUARDS' OWN MEMORY: no player is told of it, through the
	// guards' record any more than through the arrest's beat — not the one
	// wanted, and not a friend who watched them on the road, since watching
	// somebody tells you nothing of what a guard remembers. Each holder is
	// told his own holding, and nobody else.
	wantedLine := line
	wantedLine.holdersOnly = true
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
// told only the taken they perceive — on the live reach, or as they
// perceived them when the words were said for a line judged as spoken — and
// a taken member is always aware of being taken, so a beat never names a
// member its reader cannot see. The wanted are named to the arresting guards
// alone: who is wanted is the guards' memory, which no player perceives, so a
// player's beat — the wanted member's, or a friend's who watched them — never
// carries it. A reader told nobody is told nothing. Readers told the same
// share one beat, in the order first met ([Encounter.tellRecognition]'s
// shape).
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
		var taken, wanted []string
		for _, id := range takenIDs(made) {
			if guard || reader == id || line.perceives(reach, reader, id) {
				taken = append(taken, string(id))
			}
		}
		if guard {
			for _, id := range wantedIDs(made) {
				wanted = append(wanted, string(id))
			}
		}
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

// inCustody reports whether a member is held by any arrest: taken by one
// whose custody still holds.
func (e *Encounter) inCustody(id MemberID) bool {
	for _, r := range e.interactions {
		if r.arrest == nil || !r.arrest.Custody.holds() {
			continue
		}
		if slices.ContainsFunc(r.arrest.Taken, func(t ArrestedMember) bool { return t.ID == id }) {
			return true
		}
	}
	return false
}

// refuseInCustody is the custody door every walking-away verb asks first.
func (e *Encounter) refuseInCustody(verb string, id MemberID) error {
	if e.inCustody(id) {
		return fmt.Errorf("%s %q: %w", verb, id, ErrInCustody)
	}
	return nil
}

// breakCustody ends custody, for every taken member at once, of each arrest
// with one of its ARRESTING GUARDS among fighters: a fight with the guards
// who hold them is the way out the ruling leaves (rpg-project#925). A fight
// with nobody else — a taken member and a stranger — frees nobody: the
// guards who hold them are not in it, so the hold stands, and a held member
// still cannot walk on their turn ([Encounter.Step] asks custody before the
// turn). The record's taken and wanted, and what the guards hold, stay.
func (e *Encounter) breakCustody(fighters []MemberID) {
	for id, r := range e.interactions {
		if r.arrest == nil || !r.arrest.Custody.holds() {
			continue
		}
		for _, f := range fighters {
			if m, ok := e.members[f]; ok && carries(m.Options, id) {
				r.arrest.Custody = CustodyBroken
				break
			}
		}
	}
}
