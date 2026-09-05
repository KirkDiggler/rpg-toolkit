// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// reserve.go is RESERVE AND ARRIVAL — something enters the run
// (rpg-project#375, the hold-out design §3.7, §3.8, R6, R9): a placement
// authored with a predicate waits nowhere until the predicate holds, and then
// it is placed, once, with a fact and a beat, and the run treats it as it
// treats anyone who walked into view.
//
// # The never-authored yardstick
//
// A reserved placement is ABSENT, not hidden. A reserved member is not on the
// roster (e.members), so it is on no clock, in no pair, in no fight, in no
// beat's audience, asked about by no capability, and in no projection —
// [Encounter.Members], [Encounter.AtlasFor], [Encounter.Story],
// [Encounter.ClockOf] answer as though it were never written. Its facts wait
// in e.reserve, which nothing but this file reads. A reserved prop is field
// STRUCTURE with a predicate — it stays in f.props, because the author placed
// it — and is kept off the canvas (compileCanvas), out of every atlas
// (Atlas's fold) and out of Hold's reach (the probe law: refused as an id that
// names nothing) until the journal says it arrived. Same field with and
// without the reserved placements, same projection for every member, until
// arrival: that is the claim, and A6 pins it byte for byte.
//
// # The four sites
//
// Predicates are Triggers and are evaluated where endings already are, at the
// one place each event is noticed (design §3.8): a fall at [Encounter.noticeDown],
// a round at [Encounter.noticeRounds], a fact at [Encounter.learnFact], a
// stance in the fold after a flip at [Encounter.settleStances]. Each site asks
// [Encounter.arrivals] with the one question it can answer — is this the fall,
// the round, the fact, the stance you were waiting for — and arrivals happen
// BEFORE that site's endings, so `ended` stays the story's last word.
// `{ round }` is the fight's own clock (R9): outside any fight there is no
// RoundStarted milestone and nothing to notice.
//
// # What an arrival is
//
// A member arrives the way a joiner joins — placed, on the world clock, its
// records seeded, its decider attached, the graph redeclared — and a prop
// arrives onto the canvas. Both write one fact, `arrived:<id>@<cell>`, on the
// truth grain (no audience, like `held:` and `dropped:`), and one `arrived`
// beat to everyone. THE FACT IS THE WHOLE RECORD: nothing stores "arrived"
// beside it. A prop's presence on every later map is folded from it
// (holdings.go propPlacements); a member is simply a member from then on, and
// its predicate is spent. Reload mid-reserve and the reserve is exactly as it
// was, because nothing about it changed.
//
// After every arrival that brought a MEMBER in, the same verb refreshes sight
// for the whole roster, so the newcomer is seen and sees, and a fight forms or
// is joined exactly as for anyone walking into view (trigger.go). The refresh
// runs inside whatever verb noticed the event — inside a participation pass,
// inside a turn ending — which is the shape a driven monster's own Move
// already has; classify's transition rule keeps a second refresh in one verb
// honest (a pair already watching is Refreshed, not FirstContact again).
//
// # Where it lands
//
// At the authored cell when nothing stands there; otherwise at the nearest
// free cell of that cell's REGION, by grid distance, ties broken in the map's
// coordinate order (C8). "Occupied" is the canvas's own word — anything
// standing there, member or prop — so nothing ever arrives under somebody's
// feet, whether or not they would have blocked a step. A region with no free
// cell is refused loudly (ErrBadPlacement) rather than arriving nowhere: the
// verb fails and the caller drops the encounter unsaved, which is doc.go's
// contract for every mutation this module cannot finish.

// arrivedPrefix is the parse side of [arrivedKind].
const arrivedPrefix = "arrived:"

// arrivedKind mints the fact kind recording that a placement entered the run
// at a cell — `arrived:<id>@<x>,<y>`, the design's own spelling (§3.7). The
// cell is IN THE KIND for [droppedKind]'s reason: a fold reads it from the one
// field every fact carries. Which KIND of thing arrived is the fact's Subject
// (`prop:<id>` or `member:<id>`), because a prop and a member may legally
// share a bare id at this seam.
func arrivedKind(id string, at spatial.Position) journal.Kind {
	return journal.Kind(fmt.Sprintf("%s%s@%g,%g", arrivedPrefix, id, at.X, at.Y))
}

// parseArrived reads an arrived fact's kind back into the id and the cell it
// names. Reports false for a kind this build did not mint — total, so a fold
// can never panic on one that slipped through the load boundary.
func parseArrived(kind string) (string, spatial.Position, bool) {
	rest, ok := strings.CutPrefix(kind, arrivedPrefix)
	if !ok {
		return "", spatial.Position{}, false
	}
	i := strings.LastIndex(rest, "@")
	if i < 0 {
		return "", spatial.Position{}, false
	}
	id, coords := rest[:i], rest[i+1:]
	x, y, ok := strings.Cut(coords, ",")
	if !ok {
		return "", spatial.Position{}, false
	}
	var at spatial.Position
	if _, err := fmt.Sscanf(x, "%g", &at.X); err != nil {
		return "", spatial.Position{}, false
	}
	if _, err := fmt.Sscanf(y, "%g", &at.Y); err != nil {
		return "", spatial.Position{}, false
	}
	return id, at, true
}

// The two kinds of thing that arrive, as the `arrived` beat names them — a
// CLOSED set, mirrored on the wire as PlacementKind (design §6): a client
// branches on it, because a prop is not a member.
const (
	// ArrivedMonster is the beat's kind for a member arriving from reserve.
	ArrivedMonster = "monster"

	// ArrivedProp is the beat's kind for a prop arriving from reserve.
	ArrivedProp = "prop"
)

// propSubject and memberSubject are the two Subject spellings an arrived
// fact carries — the holdings journal's own `prop:` prefix, and the member
// one beside it.
func propSubject(id PropID) journal.EntityID     { return journal.EntityID("prop:" + id) }
func memberSubject(id MemberID) journal.EntityID { return journal.EntityID("member:" + string(id)) }

// reservedMember is a member the run holds back: every fact the caller
// handed in, kept for the day it arrives, and nothing else — no cell, no
// clock, no percept, no entity in the graph.
type reservedMember struct {
	// record is the member as it will be registered, faction as NAMED (the
	// same stored-not-resolved rule memberRecord keeps).
	record memberRecord

	// at is where it arrives, DUNGEON-ABSOLUTE — an authored seat
	// converted once when it was reserved, or a joiner's own cell.
	at spatial.Position

	// holds is the author's placed records, seeded when it arrives and not
	// before: a holding is a fact about a member, and there is no member yet.
	holds []IntelID

	// decider is the behaviour to attach on arrival, or nil. Never
	// persisted, like every decider.
	decider Decider

	// arrives is the predicate. Spent on arrival.
	arrives Trigger
}

// validateArrival refuses a member predicate that cannot be kept
// (ErrNoMember): a kind that has no reserve to wait in, a predicate that can
// never hold, or a member waiting for its own fall. Asked before any mutation
// at both ways in (Setup, Join) and at Load.
func (f *field) validateArrival(id MemberID, kind MemberKind, t Trigger) error {
	if kind != KindMonster {
		return fmt.Errorf("member %q is a %s and arrives on a predicate, and only a monster waits in reserve: %w",
			id, kind, ErrNoMember)
	}
	what := fmt.Sprintf("member %q's arrival", id)
	if err := f.validatePredicate(what, t, ErrNoMember); err != nil {
		return err
	}
	if md, ok := t.(TriggerMemberDown); ok && md.Member == id {
		return fmt.Errorf("%s waits for its own fall, and it is not here to fall until it arrives: %w", what, ErrNoMember)
	}
	return nil
}

// validatePropArrivals refuses a prop predicate that cannot be kept
// (ErrNoField), by the same liveness rule an ending meets. Run at the end of
// compileField, once the factions a `{ stance }` names are compiled.
func (f *field) validatePropArrivals() error {
	for _, p := range f.props {
		if p.Arrives == nil {
			continue
		}
		if err := f.validatePredicate(fmt.Sprintf("prop %q's arrival", p.ID), p.Arrives, ErrNoField); err != nil {
			return err
		}
	}
	return nil
}

// reserveMember puts a validated member into reserve. The one writer of
// e.reserve besides Load's rebuild.
func (e *Encounter) reserveMember(rm *reservedMember) {
	if e.reserve == nil {
		e.reserve = make(map[MemberID]*reservedMember)
	}
	e.reserve[rm.record.ID] = rm
}

// inReserve reports whether an id is held in reserve — the second half of
// "already in the encounter" for Join.
func (e *Encounter) inReserve(id MemberID) bool {
	_, ok := e.reserve[id]
	return ok
}

// reservedIDs is every reserved member, in stable ID order (C8).
func (e *Encounter) reservedIDs() []MemberID {
	ids := make([]MemberID, 0, len(e.reserve))
	for id := range e.reserve {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

// arrivals is THE ONE EVALUATOR of the reserve against one noticed event.
// holds answers, for one predicate, whether the event that was just noticed
// is what it waited for — and how to say so in the journal. Members arrive
// first, in ID order, then props in authored order; the story's order is
// therefore a function of persisted data and nothing else (C8). A closed run
// arrives nobody.
//
// When a MEMBER arrived, sight is refreshed for the whole roster at the end,
// once — see the file's own doc for why that is this verb's refresh and not a
// later one's.
func (e *Encounter) arrivals(holds func(Trigger) (string, bool), at uint64) error {
	if e.outcome != nil {
		return nil
	}

	memberArrived := false
	for _, id := range e.reservedIDs() {
		rm := e.reserve[id]
		cause, ok := holds(rm.arrives)
		if !ok {
			continue
		}
		if err := e.arriveMember(rm, cause, at); err != nil {
			return err
		}
		delete(e.reserve, id)
		memberArrived = true
	}

	placements := e.holdings.propPlacements()
	for i, p := range e.field.props {
		if p.Arrives == nil || placements[p.ID].arrived {
			continue
		}
		cause, ok := holds(p.Arrives)
		if !ok {
			continue
		}
		if err := e.arriveProp(i, cause, at); err != nil {
			return err
		}
	}

	if !memberArrived {
		return nil
	}
	if _, _, err := e.refreshSight(e.rosterIDs()); err != nil {
		return fmt.Errorf("arrival refresh sight: %w", err)
	}
	return nil
}

// arriveMember brings one reserved member onto the map, the way Join does:
// the same placement, the same clock, the same seed, the same redeclaration,
// in the same order — then the fact, then the beat.
func (e *Encounter) arriveMember(rm *reservedMember, cause string, at uint64) error {
	id := rm.record.ID
	entity := &memberEntity{id: string(id), kind: rm.record.Kind, blocksMovement: rm.record.BlocksMovement}
	cell, err := e.arrivalCell(rm.at, e.field.isStandable, fmt.Sprintf("member %q", id))
	if err != nil {
		return err
	}
	if err := e.canvas.PlaceEntity(entity, cell); err != nil {
		return fmt.Errorf("arrival of %q: %w: %w", id, ErrBadPlacement, err)
	}

	record := rm.record
	e.members[id] = &record
	e.everMembers[id] = true
	if _, cerr := e.clock.Join(&clock.JoinInput{ID: core.EntityID(id)}); cerr != nil {
		return fmt.Errorf("arrival of %q world clock: %w", id, cerr)
	}
	if rm.decider != nil {
		e.deciders[id] = rm.decider
	}
	if err := e.holdings.seedIntel(id, rm.holds); err != nil {
		return fmt.Errorf("arrival of %q: %w", id, err)
	}
	if err := e.buildWorld(); err != nil {
		return fmt.Errorf("arrival of %q: %w", id, err)
	}

	if _, err := e.holdings.log.Append(journal.Fact{
		Kind:    arrivedKind(string(id), cell),
		Actor:   journal.EntityID(id),
		Subject: memberSubject(id),
		Outcome: journal.Outcome{Detail: cause},
	}); err != nil {
		return fmt.Errorf("arrival of %q: %w", id, err)
	}
	return e.appendArrivedBeat(string(id), ArrivedMonster, cell, at)
}

// arriveProp brings one reserved prop onto the canvas — the entity it would
// have had from the first frame, under the index-derived id compileCanvas
// would have given it — then the fact, then the beat. Every map from here on
// folds its presence from the fact (Atlas).
func (e *Encounter) arriveProp(index int, cause string, at uint64) error {
	p := e.field.props[index]
	cell, err := e.arrivalCell(e.field.cellAt(p.At), e.field.isFloor, fmt.Sprintf("prop %q", p.ID))
	if err != nil {
		return err
	}
	if err := e.canvas.PlaceEntity(propEntityOf(index, p), cell); err != nil {
		return fmt.Errorf("arrival of prop %q: %w: %w", p.ID, ErrBadPlacement, err)
	}
	if _, err := e.holdings.log.Append(journal.Fact{
		Kind:    arrivedKind(p.ID, cell),
		Actor:   journal.EntityID(p.ID),
		Subject: propSubject(p.ID),
		Outcome: journal.Outcome{Detail: cause},
	}); err != nil {
		return fmt.Errorf("arrival of prop %q: %w", p.ID, err)
	}
	return e.appendArrivedBeat(p.ID, ArrivedProp, cell, at)
}

// arrivalCell is where an arrival lands: the authored cell when it is floor
// of the right kind and nothing stands on it, else the nearest free cell of
// its region — grid distance first, then the map's coordinate order, so two
// runs of one save land in one place (C8). floor is the rule for the kind of
// thing arriving: standable for a member, any floor for a prop.
//
// Refused (ErrBadPlacement) when the cell is in no region to search — a prop
// authored on scenery that something has been dropped on — or when the region
// has no free cell left. Loud rather than nowhere: see the file's own doc.
func (e *Encounter) arrivalCell(at spatial.Position, floor func(spatial.Position) bool, what string) (spatial.Position, error) {
	free := func(c spatial.Position) bool { return floor(c) && !e.canvas.IsPositionOccupied(c) }
	if free(at) {
		return at, nil
	}
	region, owned := e.field.regionOf(at)
	if !owned {
		return spatial.Position{}, fmt.Errorf("%s cannot arrive at %v: something stands there and the cell is in no region to look for room in: %w",
			what, at, ErrBadPlacement)
	}
	found := false
	var best spatial.Position
	var bestDistance float64
	for _, c := range e.field.regionCells[region] {
		if !free(c) {
			continue
		}
		d := e.Distance(at, c)
		if !found || d < bestDistance || (d == bestDistance && cellBefore(c, best)) {
			found, best, bestDistance = true, c, d
		}
	}
	if !found {
		return spatial.Position{}, fmt.Errorf("%s cannot arrive at %v: something stands there and region %q has no free cell: %w",
			what, at, region, ErrBadPlacement)
	}
	return best, nil
}

// appendArrivedBeat records that a placement entered the run, to everyone: an
// arrival is physical state on the truth grain, like HELD and DROPPED (design
// §6), so the same beat reaches every member of the run — the arrival
// itself included, when it is a member.
func (e *Encounter) appendArrivedBeat(id, kind string, cell spatial.Position, at uint64) error {
	payload, err := json.Marshal(map[string]interface{}{
		"beat": "arrived",
		"id":   id,
		"kind": kind,
		"cell": cell,
	})
	if err != nil {
		return fmt.Errorf("arrived beat payload: %w", err)
	}
	if _, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: e.audienceFor(tableBeat),
		Tags:     map[string]string{"tag": "world"},
		Payload:  payload,
	}); err != nil {
		return fmt.Errorf("arrived append beat: %w", err)
	}
	return nil
}

// propEntityOf is the canvas entity for one authored prop — index-derived id,
// the author's two blocking answers — built here so compileCanvas and an
// arrival place the identical thing.
func propEntityOf(index int, p PropInput) *propEntity {
	return &propEntity{
		id:                fmt.Sprintf("prop-%d", index),
		ref:               p.Ref,
		blocksMovement:    *p.BlocksMovement,
		blocksLineOfSight: *p.BlocksLineOfSight,
	}
}

// onFall is the `{ down }` question: the member this predicate waits on is
// among those noticed down in this pass.
func onFall(down map[MemberID]bool) func(Trigger) (string, bool) {
	return func(t Trigger) (string, bool) {
		md, ok := t.(TriggerMemberDown)
		if !ok || !down[md.Member] {
			return "", false
		}
		return "the fall of " + string(md.Member), true
	}
}

// onRound is the `{ round }` question: this milestone started the round the
// predicate waits for.
func onRound(round int) func(Trigger) (string, bool) {
	return func(t Trigger) (string, bool) {
		tr, ok := t.(TriggerRound)
		if !ok || tr.Round != round {
			return "", false
		}
		return fmt.Sprintf("round %d started", round), true
	}
}

// onFact is the `{ fact }` question on the TRUTH grain (R5): the fact just
// appended to the run's journal is the one the predicate waits for, whoever
// learned it.
func onFact(id FactID) func(Trigger) (string, bool) {
	return func(t Trigger) (string, bool) {
		tf, ok := t.(TriggerFact)
		if !ok || tf.Fact != id {
			return "", false
		}
		return "the fact " + id + " was learned", true
	}
}

// onStance is the `{ stance }` question, asked in the fold after a flip: the
// pair the predicate names turned, and to the stance it waits for.
func onStance(before, after map[factionPair]Stance) func(Trigger) (string, bool) {
	return func(t Trigger) (string, bool) {
		ts, ok := t.(TriggerStance)
		if !ok {
			return "", false
		}
		pair := pairOf(ts.Between[0], ts.Between[1])
		if before[pair] == after[pair] || after[pair] != ts.Stance {
			return "", false
		}
		return fmt.Sprintf("%s turned %s", pair, ts.Stance), true
	}
}
