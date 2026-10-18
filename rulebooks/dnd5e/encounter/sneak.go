// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// sneak.go is SNEAKING (rpg-project#606, rpg-toolkit#1020; design
// rpg-project ideas/sneaking/design.md R1–R16): a mode a player declares in
// free roam, one Stealth roll per (sneaker, watcher) each time the sneaker
// crosses INTO that watcher's zone, and a won roll keeping the sneaker out of
// that one watcher's sight.
//
// FOUR EXISTING THINGS, NO SECOND VISIBILITY AUTHORITY. The zone is the one
// sight test ([sightReach.reachesCell]); the roll is the automatic check seam
// ([CheckResolver.ResolveDiscoveryCheck]), rolled by the rulebook; a won roll
// is the perception pass's own fade, because the suppressed pair is simply not
// reached; and the verdicts are discovery's armed shape, persisted.
//
// WHAT THIS MODULE NEVER DOES (C1, C8; design R8, R10): it never compares a
// Stealth total, never rolls, never halves a speed and never picks an arc or a
// re-arm distance. The resolver decides; the session halves pace; the
// re-arm distance arrives on [SheetFacts.WatchResetCells] and the arc on
// [SheetFacts.WatchArc].

// BeatSnuck is the "beat" value of the story beat a sneaking mover receives
// when one of its pairs is rolled (design R12): `{member, watcher, dc, total,
// beaten, calculation}`, audience the mover alone. `watcher` is written only
// when the mover holds that watcher live after the refresh that rolled, so a
// beat never names a creature the mover cannot see.
//
// EXPORTED BECAUSE A DECODER READS IT, [BeatHeard]'s reason.
const BeatSnuck = "snuck"

// stealthAbility is the one approach a sneak roll asks the resolver for. An
// opaque word here, matched by nobody in this module ([CheckApproach]); what
// Stealth adds is the rulebook's.
const stealthAbility = "stealth"

// SneakVerdictData is one (mover, watcher) pair's standing verdict —
// discovery's {Used, Armed} attempt shape, persisted on
// [EncounterData.SneakVerdicts] (design R10).
//
//   - Beaten is the resolver's verdict on the last roll: true is a won roll,
//     false a lost one — including the lost-without-a-roll a declaration in
//     plain view writes.
//   - Armed is whether the pair may roll again on its next crossing: false
//     from the roll until the mover stands the watcher's
//     [SheetFacts.WatchResetCells] outside its zone (and, for a lost pair,
//     out of its ordinary sight).
//   - InZone is the zone's own answer for the mover's cell at the last
//     judging pass, never a constant: a crossing is `!InZone && nowInZone`.
//
// A verdict STANDS while it is not Armed: a standing won verdict keeps the
// mover out of that watcher's sight; a standing lost one changes nothing.
//
// AN ARMED PAIR OUTSIDE THE ZONE IS UNSEEN (design R9, R14). A pair with no
// standing verdict — armed, or never rolled — sees the mover only inside the
// zone, so an armed pair whose InZone is false keeps the mover out of that
// watcher's sight exactly as a standing won verdict does. Under an arc that
// is a sneaker behind a watcher's back: ordinary sight reaches them, the
// zone does not. So the judgement writes an armed record for a never-rolled
// pair the moment ordinary sight reaches the mover outside the zone, and
// every reach built from the stored verdicts reads it ([sightReach.Reaches]).
// With no arc the zone is sight, and such a pair is unreached anyway.
type SneakVerdictData struct {
	Beaten bool `json:"beaten,omitempty"`
	Armed  bool `json:"armed,omitempty"`
	InZone bool `json:"in_zone,omitempty"`
}

// SetSneakingInput sets or clears a player's sneaking mode.
type SetSneakingInput struct {
	// Member is the player who declares.
	Member MemberID

	// Sneaking is the mode the player declares: true to sneak, false to stop.
	Sneaking bool
}

// SetSneakingOutput reports the mode now in effect and what the declaration's
// sight refresh caused.
type SetSneakingOutput struct {
	// Sneaking is the mode in effect after the call.
	Sneaking bool

	// IntelDeltas is what the refresh changed, per observer — nil for a
	// declaration of the mode already held, which refreshes nothing.
	IntelDeltas map[MemberID]*IntelDelta

	// Formed is a fight the refresh started, or nil. Declaring sneaking off
	// inside a hostile watcher's zone is seen at once (R6), and that sighting
	// forms a fight exactly as a step into view does.
	Formed *FormedBubble
}

// SetSneaking sets or clears a player's sneaking mode on the world clock
// (design R1, R6). It spends no time, then refreshes every member's sight, so
// a declaration is judged at once: stopping inside a watcher's zone is seen in
// the same call.
//
// DECLARING IN PLAIN VIEW IS A LOST VERDICT (R10, I1: nobody vanishes in plain
// view). Every watcher that holds the mover live, or whose zone already holds
// the mover's cell, gets a lost verdict with no roll and no beat, and keeps
// seeing the mover by ordinary sight until the pair re-arms.
//
// DECLARING THE MODE ALREADY HELD IS A NO-OP: no roll, no re-arm, no refresh,
// no beat; the current mode is returned. Declaring it off clears every
// verdict the mover holds.
//
// Errors: ErrNilInput; ErrNotMember for a stranger or a member who is not a
// player (a monster never sneaks, R16); ErrClosed; ErrInBubble for a member
// in a fight (free roam only, R1); ErrNoCheckResolver when the encounter was
// given no [CheckResolver], because the mode could never roll; anything the
// refresh returns (the encounter is then discarded, doc.go's R5).
func (e *Encounter) SetSneaking(in *SetSneakingInput) (*SetSneakingOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("set sneaking: %w", ErrNilInput)
	}
	member, ok := e.members[in.Member]
	if !ok {
		return nil, fmt.Errorf("set sneaking %q: %w", in.Member, ErrNotMember)
	}
	if member.Kind != KindPlayer {
		return nil, fmt.Errorf("set sneaking %q: only a player sneaks: %w", in.Member, ErrNotMember)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("set sneaking: %w", ErrClosed)
	}
	bubble, err := e.bubbleFor(in.Member)
	if err != nil {
		return nil, fmt.Errorf("set sneaking %q: %w", in.Member, err)
	}
	if bubble != nil {
		return nil, fmt.Errorf("set sneaking %q: sneaking is declared in free roam: %w", in.Member, ErrInBubble)
	}
	if e.checkResolver == nil {
		return nil, fmt.Errorf("set sneaking %q: %w", in.Member, ErrNoCheckResolver)
	}
	if member.Sneaking == in.Sneaking {
		return &SetSneakingOutput{Sneaking: member.Sneaking}, nil
	}

	if in.Sneaking {
		if err := e.declareSneaking(member); err != nil {
			return nil, fmt.Errorf("set sneaking %q: %w", in.Member, err)
		}
	} else {
		e.endSneaking(member.ID)
	}

	deltas, formed, err := e.refreshSight(e.rosterIDs())
	if err != nil {
		return nil, fmt.Errorf("set sneaking %q: %w", in.Member, err)
	}

	return &SetSneakingOutput{Sneaking: in.Sneaking, IntelDeltas: deltas, Formed: formed}, nil
}

// Sneaking reports whether a member is sneaking now. False for a stranger:
// nobody who is not here sneaks. The session reads it to halve a sneaker's
// pace (design R8), which this module never does.
func (e *Encounter) Sneaking(member MemberID) bool {
	m, ok := e.members[member]
	return ok && m.Sneaking
}

// declareSneaking turns the mode on, first writing a lost verdict against
// every watcher that already sees the mover — by its own live holding, or
// because its zone holds the mover's cell (R10).
//
// Errors: the mover is not on the canvas, or sight or a holding could not be
// read.
func (e *Encounter) declareSneaking(mover *memberRecord) error {
	geometry, sheets, err := e.watchReach()
	if err != nil {
		return err
	}
	cell, placed := geometry.positions[mover.ID]
	if !placed {
		return fmt.Errorf("sneaker is not on the canvas: %w", ErrBadPlacement)
	}
	for _, watcher := range e.watchersIn(geometry, e.rosterIDs()) {
		// An unsupplied passive judges no sneak ([sneakJudge.pair]).
		if watcher == mover.ID || sheets[watcher].PassivePerception == 0 {
			continue
		}
		live, err := e.holdsLive(watcher, mover.ID)
		if err != nil {
			return err
		}
		inZone := geometry.inWatchZone(watcher, cell)
		if live || inZone {
			e.setSneakVerdict(mover.ID, watcher, SneakVerdictData{InZone: inZone})
		}
	}
	mover.Sneaking = true

	return nil
}

// endSneaking clears a member's mode and every verdict it holds (R7). Called
// by every verb that shows the mover — SetSneaking off, Speak, RecordCast, an
// attack recorded, Exit — and by formation for every member a fight engages,
// before any turn budget reads the member's speed. A member who is not
// sneaking is untouched.
func (e *Encounter) endSneaking(member MemberID) {
	if m, ok := e.members[member]; ok {
		m.Sneaking = false
	}
	delete(e.sneakVerdicts, member)
	if len(e.sneakVerdicts) == 0 {
		e.sneakVerdicts = nil
	}
}

// forgetWatcher drops every verdict naming a watcher that is leaving the run,
// so no persisted pair names a non-member.
func (e *Encounter) forgetWatcher(watcher MemberID) {
	for mover, pairs := range e.sneakVerdicts {
		delete(pairs, watcher)
		if len(pairs) == 0 {
			delete(e.sneakVerdicts, mover)
		}
	}
	if len(e.sneakVerdicts) == 0 {
		e.sneakVerdicts = nil
	}
}

// dropSneakVerdict removes one pair's verdict, if it has one.
func (e *Encounter) dropSneakVerdict(mover, watcher MemberID) {
	pairs, ok := e.sneakVerdicts[mover]
	if !ok {
		return
	}
	delete(pairs, watcher)
	if len(pairs) == 0 {
		delete(e.sneakVerdicts, mover)
	}
	if len(e.sneakVerdicts) == 0 {
		e.sneakVerdicts = nil
	}
}

// standingSuppression is every pair the verdicts hide, subject first, then
// watcher; nil when none is hidden. Two kinds of pair are hidden: a standing
// won verdict — Beaten and not yet Armed (R11) — and an armed pair whose
// mover stood outside the zone at the last judging pass (R9, R14:
// [SneakVerdictData]). A standing LOST verdict hides nothing: that watcher
// sees the mover by ordinary sight (R10).
func (e *Encounter) standingSuppression() map[MemberID]map[MemberID]bool {
	var out map[MemberID]map[MemberID]bool
	for mover, pairs := range e.sneakVerdicts {
		for watcher, v := range pairs {
			wonStands := v.Beaten && !v.Armed
			unwatched := v.Armed && !v.InZone
			if !wonStands && !unwatched {
				continue
			}
			if out == nil {
				out = map[MemberID]map[MemberID]bool{}
			}
			if out[mover] == nil {
				out[mover] = map[MemberID]bool{}
			}
			out[mover][watcher] = true
		}
	}

	return out
}

// setSneakVerdict stores one pair's verdict.
func (e *Encounter) setSneakVerdict(mover, watcher MemberID, v SneakVerdictData) {
	if e.sneakVerdicts == nil {
		e.sneakVerdicts = map[MemberID]map[MemberID]SneakVerdictData{}
	}
	if e.sneakVerdicts[mover] == nil {
		e.sneakVerdicts[mover] = map[MemberID]SneakVerdictData{}
	}
	e.sneakVerdicts[mover][watcher] = v
}

// watchReach is [Encounter.placedSightReach] with every watcher's cone on it
// — the geometry a zone is asked of outside a pass ([Encounter.SetSneaking]'s
// declaration, [Encounter.WatchZonesFor]) — and the sheets the cones were
// read from, asked once.
//
// Errors: sight or the sheets could not be read.
func (e *Encounter) watchReach() (sightReach, map[MemberID]SheetFacts, error) {
	geometry, err := e.placedSightReach()
	if err != nil {
		return sightReach{}, nil, err
	}
	sheets, err := e.sheetsNow()
	if err != nil {
		return sightReach{}, nil, err
	}
	geometry.cones = e.watchCones(sheets)

	return geometry, sheets, nil
}

// watchCone is one watcher's supplied arc, in degrees, and the facing it is
// centred on, both read at use: the arc from [SheetFacts.WatchArc] this ask,
// the facing from the member as it stands this pass.
type watchCone struct {
	arc    int
	facing float64
}

// watchCones is every watcher's cone from one sheets answer: a member whose
// [SheetFacts.WatchArc] is non-zero, keyed by member. Nil when nobody has an
// arc, which is a zone of 360 for everybody.
//
// A WATCHER WITH NO PASSIVE PERCEPTION GETS NO CONE (R22). It judges no
// sneak and sees a sneaker by ordinary sight ([sneakJudge.pair]), so its
// zone stays its sight and the floor drawn red ([Encounter.WatchZonesFor])
// stays the floor it sees the sneaker on. A cone there would draw the floor
// behind it clear while it watches that floor all round.
//
// AN ABSENT FACING IS NOT A CASE. Every member carries one: the stated
// facing, or [UnstatedFacing] — 180, south, a rules bearing (R13) — from the
// moment it is seeded, so an unplaced or never-turned watcher's cone faces
// south. An unplaced watcher has no zone at all; its cone is never asked.
func (e *Encounter) watchCones(sheets map[MemberID]SheetFacts) map[MemberID]watchCone {
	var out map[MemberID]watchCone
	for id, facts := range sheets {
		m, ok := e.members[id]
		if !ok || facts.WatchArc == 0 || facts.PassivePerception == 0 {
			continue
		}
		if out == nil {
			out = map[MemberID]watchCone{}
		}
		out[id] = watchCone{arc: facts.WatchArc, facing: m.Facing}
	}

	return out
}

// placedSightReach is the sight geometry over every placed member, built the
// one way a pass builds it ([Encounter.newSightReach]).
func (e *Encounter) placedSightReach() (sightReach, error) {
	ranges, err := e.sightNow()
	if err != nil {
		return sightReach{}, err
	}
	positions := make(map[MemberID]spatial.Position, len(e.members))
	for _, id := range e.rosterIDs() {
		if cell, placed := e.canvas.GetEntityPosition(string(id)); placed {
			positions[id] = cell
		}
	}

	return e.newSightReach(positions, ranges), nil
}

// watchersIn is every placed non-player among ids, sorted: the members a
// sneaker can be judged against (design R9 — a watcher is any placed member
// that is not a player; another player is never one).
func (e *Encounter) watchersIn(reach sightReach, ids []MemberID) []MemberID {
	out := make([]MemberID, 0, len(ids))
	seen := make(map[MemberID]bool, len(ids))
	for _, id := range ids {
		m, ok := e.members[id]
		if !ok || m.Kind == KindPlayer || seen[id] {
			continue
		}
		if _, placed := reach.positions[id]; !placed {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })

	return out
}

// holdsLive reports whether observer holds subject live on sight now — a
// sighting whose CurrentVia is non-empty, not a ghost.
func (e *Encounter) holdsLive(observer, subject MemberID) (bool, error) {
	held, err := e.memberIntel(observer)
	if err != nil {
		return false, err
	}
	for _, h := range held {
		if h.Channel == perception.Sight && MemberID(h.Subject) == subject && len(h.CurrentVia) > 0 {
			return true, nil
		}
	}

	return false, nil
}

// inWatchZone is THE WATCH ZONE (design R9, R14), asked one cell at a time:
// does this watcher's zone hold this cell in the pass's geometry? A zone is
// every floor cell the watcher's sight reaches from where it stands, by the
// ONE sight test ([sightReach.reachesCell]) — its own Sight range, line of
// sight and sight areas — and never the cells it knows, because knowledge
// lags sight; narrowed by the watcher's supplied arc ([sightReach.inArc]).
//
// THE ONLY PLACE THE ARC APPLIES (R14). [sightReach.reachesCell],
// [sightReach.Reaches] and [Encounter.VisibleCellsFor] never read it, so a
// watcher's fog, its sight of anybody who is not sneaking, and its sight of a
// sneaker it already holds a lost verdict against stay all round (R2, R10).
// The judgement ([Encounter.judgeSneakers]) and the red floor
// ([Encounter.WatchZonesFor]) both ask this, so a cone narrows the floor that
// rolls and the floor drawn red at once.
func (s sightReach) inWatchZone(watcher MemberID, cell spatial.Position) bool {
	return s.canvas.field.isFloor(cell) && s.reachesCell(watcher, cell) && s.inArc(watcher, cell)
}

// inArc reports whether a cell lies inside a watcher's supplied arc: its
// bearing from the watcher's cell is within half the arc of the watcher's
// facing, EDGE INCLUSIVE, so an arc of 120 holds a cell at exactly 60 off the
// facing. A watcher with no cone ([watchCones]) watches all round.
//
// ONE FRAME. The bearing is [bearing] — the frame facing.go turns members in
// — so a watcher facing 0 watches the cells north of it on the same map its
// steps turned it on.
//
// THE WATCHER'S OWN CELL IS NEVER IN QUESTION: a cell has no bearing from
// itself, and a watcher's arc never hides its own feet.
func (s sightReach) inArc(watcher MemberID, cell spatial.Position) bool {
	cone, ok := s.cones[watcher]
	if !ok {
		return true
	}
	from, placed := s.positions[watcher]
	if !placed {
		return false
	}
	toward, ok := bearing(s.canvas.field.orientation, from, cell)
	if !ok {
		return true
	}
	off := math.Abs(math.Mod(toward-cone.facing+540, 360) - 180)

	return off <= float64(cone.arc)/2
}

// clearOfZone reports whether a cell stands at least reset cells from every
// cell of the watcher's zone — the re-arm distance (R10). Zero asks only
// whether the cell itself is outside. Only the cells nearer than reset are
// asked, so the cost is the ring around the mover and never the whole zone.
func (s sightReach) clearOfZone(watcher MemberID, at spatial.Position, reset int) bool {
	if reset <= 0 {
		return !s.inWatchZone(watcher, at)
	}
	seen := map[spatial.Position]bool{at: true}
	frontier := []spatial.Position{at}
	for d := 0; d < reset; d++ {
		var next []spatial.Position
		for _, cell := range frontier {
			if s.inWatchZone(watcher, cell) {
				return false
			}
			for _, n := range adjacencyGrid.GetNeighbors(cell) {
				if !seen[n] {
					seen[n] = true
					next = append(next, n)
				}
			}
		}
		frontier = next
	}

	return true
}

// snuckRoll is one pair rolled in a pass, waiting for its beat until the pass
// has said whether the mover now sees the watcher.
type snuckRoll struct {
	mover, watcher MemberID
	verdict        *ResolveCheckOutput
}

// sneakJudge is one pass's judgement in progress: the pass's geometry, the
// sheets read once when a pair first needs one, and what was rolled.
type sneakJudge struct {
	e      *Encounter
	reach  *sightReach
	sheets map[MemberID]SheetFacts
	rolls  []snuckRoll
}

// sheet is one watcher's facts, asking the capability once per pass.
func (j *sneakJudge) sheet(id MemberID) (SheetFacts, error) {
	if err := j.readSheets(); err != nil {
		return SheetFacts{}, err
	}

	return j.sheets[id], nil
}

// readSheets asks the capability once per pass, and puts every watcher's
// cone on the pass's reach from that one answer, so the zone the pass judges
// with is narrowed by the arcs read with the passives it rolls against.
func (j *sneakJudge) readSheets() error {
	if j.sheets != nil {
		return nil
	}
	all, err := j.e.sheetsNow()
	if err != nil {
		return err
	}
	if all == nil {
		all = map[MemberID]SheetFacts{}
	}
	j.sheets = all
	j.reach.cones = j.e.watchCones(all)

	return nil
}

// judgeSneakers is THE JUDGEMENT (design R10, R11), run inside
// [Encounter.rebuildPercepts] after the pass's geometry is built and BEFORE
// anything copies it: for each sneaking placed player, sorted, against each
// watcher among this pass's observers, sorted, it re-arms what has earned it,
// rolls each armed (or verdict-less) pair whose mover just crossed into the
// watcher's zone, records the zone's real answer on every pair, and sets on
// the reach every pair a standing won verdict suppresses.
//
// ONLY THIS PASS'S OBSERVERS. A pair whose watcher is not observing in this
// pass is left untouched, as the pass leaves that watcher's holdings
// untouched — the single-observer pass a concealment reveal runs judges
// nobody else.
//
// NO ROLL ON A CLOSED RUN: the ended beat is the story's last word.
//
// Errors: the sheets or the resolver failed, or the resolver answered no
// verdict; the verb is then discarded unsaved (R5).
func (e *Encounter) judgeSneakers(reach *sightReach, observers []MemberID) ([]snuckRoll, error) {
	movers := e.sneakersIn(*reach)
	if len(movers) == 0 {
		return nil, nil
	}
	judge := &sneakJudge{e: e, reach: reach}
	watchers := e.watchersIn(*reach, observers)
	if len(watchers) > 0 {
		// The cones before the first pair: every zone below is narrowed.
		if err := judge.readSheets(); err != nil {
			return nil, err
		}
	}
	for _, mover := range movers {
		for _, watcher := range watchers {
			if _, err := judge.pair(mover, watcher); err != nil {
				return nil, err
			}
		}
	}
	// The pass's pairs were just re-judged; a watcher outside the pass keeps
	// its standing answer. One map, rebuilt from the verdicts as they now
	// stand.
	reach.suppressed = e.standingSuppression()

	return judge.rolls, nil
}

// sneakersIn is every sneaking placed player, sorted.
func (e *Encounter) sneakersIn(reach sightReach) []MemberID {
	var out []MemberID
	for _, id := range e.rosterIDs() {
		m := e.members[id]
		if m.Kind != KindPlayer || !m.Sneaking {
			continue
		}
		if _, placed := reach.positions[id]; placed {
			out = append(out, id)
		}
	}

	return out
}

// pair judges one (mover, watcher) pair and reports whether a standing won
// verdict suppresses it now.
//
// AN UNSUPPLIED PASSIVE JUDGES NO SNEAK (lead ruling on rpg-toolkit#2078): a
// watcher whose [SheetFacts.PassivePerception] is zero — "not supplied",
// every world NPC today — rolls nothing and holds no verdict, and sees the
// sneaker by ordinary sight exactly as it sees a mover who is not sneaking
// (R2). Fail closed: no DC is invented for a number nobody gave.
func (j *sneakJudge) pair(mover, watcher MemberID) (bool, error) {
	e := j.e
	facts, err := j.sheet(watcher)
	if err != nil {
		return false, err
	}
	if facts.PassivePerception == 0 {
		e.dropSneakVerdict(mover, watcher)
		return false, nil
	}
	cell := j.reach.positions[mover]
	v, has := e.sneakVerdicts[mover][watcher]
	nowIn := j.reach.inWatchZone(watcher, cell)

	if has && !v.Armed && !nowIn {
		armed, err := j.rearms(watcher, cell, v)
		if err != nil {
			return false, err
		}
		v.Armed = armed
	}

	if !v.InZone && nowIn && (!has || v.Armed) && e.outcome == nil {
		verdict, err := j.roll(mover, watcher)
		if err != nil {
			return false, err
		}
		v = SneakVerdictData{Beaten: verdict.Beaten, InZone: true}
		has = true
	} else {
		v.InZone = nowIn
	}
	if !has {
		// OUTSIDE THE ZONE, UNSEEN (R9, R14). A never-rolled pair whose
		// mover ordinary sight reaches outside the zone — behind an arc —
		// is recorded armed, so every reach built from the verdicts keeps
		// the mover out of this watcher's sight ([SneakVerdictData]).
		if nowIn || !j.reach.reachesCell(watcher, cell) {
			return false, nil
		}
		v = SneakVerdictData{Armed: true}
	}
	e.setSneakVerdict(mover, watcher, v)

	return (v.Beaten && !v.Armed) || (v.Armed && !v.InZone), nil
}

// rearms reports whether a standing pair re-arms this pass: the mover stands
// the watcher's re-arm distance outside its zone, and — for a lost pair —
// the watcher's ordinary sight no longer reaches the mover (R10: hiding needs
// a broken line of sight first).
//
// THE SECOND TERM IS WHAT AN ARC MAKES BIND. With no arc a zone is the
// watcher's sight, so a mover outside it is already out of ordinary sight;
// under an arc a mover behind the watcher is outside the zone and still in
// its sight, and a lost pair holds until that sight breaks too.
func (j *sneakJudge) rearms(watcher MemberID, cell spatial.Position, v SneakVerdictData) (bool, error) {
	facts, err := j.sheet(watcher)
	if err != nil {
		return false, err
	}
	if !j.reach.clearOfZone(watcher, cell, facts.WatchResetCells) {
		return false, nil
	}

	return v.Beaten || !j.reach.reachesCell(watcher, cell), nil
}

// roll asks the resolver for the mover's Stealth against the watcher's
// passive Perception — supplied, so at least 1 ([sneakJudge.pair]) — and
// keeps the verdict for its beat.
func (j *sneakJudge) roll(mover, watcher MemberID) (*ResolveCheckOutput, error) {
	facts, err := j.sheet(watcher)
	if err != nil {
		return nil, err
	}
	if j.e.checkResolver == nil {
		return nil, fmt.Errorf("sneak roll for %q: %w", mover, ErrNoCheckResolver)
	}
	verdict, err := j.e.checkResolver.ResolveDiscoveryCheck(&ResolveCheckInput{
		Member:     mover,
		Approaches: []CheckApproach{{Ability: stealthAbility, DC: facts.PassivePerception}},
	})
	if err != nil {
		return nil, fmt.Errorf("sneak roll for %q against %q: %w", mover, watcher, err)
	}
	if verdict == nil {
		return nil, fmt.Errorf("sneak roll for %q returned no verdict: %w", mover, ErrInvalidData)
	}
	j.rolls = append(j.rolls, snuckRoll{mover: mover, watcher: watcher, verdict: verdict})

	return verdict, nil
}

// tellSnuck appends one [BeatSnuck] per pair a pass rolled, after the pass
// has run, so `watcher` is named exactly when the mover holds it live now.
func (e *Encounter) tellSnuck(rolls []snuckRoll, at uint64) error {
	for _, r := range rolls {
		body := map[string]any{
			"beat":        BeatSnuck,
			"member":      r.mover,
			"dc":          r.verdict.Applied.DC,
			"total":       r.verdict.Total,
			"beaten":      r.verdict.Beaten,
			"calculation": r.verdict.Calculation,
		}
		live, err := e.holdsLive(r.mover, r.watcher)
		if err != nil {
			return err
		}
		if live {
			body["watcher"] = r.watcher
		}
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("snuck beat: %w", err)
		}
		if _, err := e.appendBeat(&record.AppendInput{
			At:       at,
			Audience: e.audienceFor(tableBeat, r.mover),
			Tags:     map[string]string{"tag": BeatSnuck},
			Payload:  payload,
		}); err != nil {
			return fmt.Errorf("snuck beat: %w", err)
		}
	}

	return nil
}

// copySneakVerdicts deep-copies the verdicts for a snapshot or a load.
func copySneakVerdicts(in map[MemberID]map[MemberID]SneakVerdictData) map[MemberID]map[MemberID]SneakVerdictData {
	if len(in) == 0 {
		return nil
	}
	out := make(map[MemberID]map[MemberID]SneakVerdictData, len(in))
	for mover, pairs := range in {
		if len(pairs) == 0 {
			continue
		}
		copied := make(map[MemberID]SneakVerdictData, len(pairs))
		for watcher, v := range pairs {
			copied[watcher] = v
		}
		out[mover] = copied
	}
	if len(out) == 0 {
		return nil
	}

	return out
}

// validateSneakVerdicts refuses a loaded verdict that names a stranger, a
// mover who is not a sneaking player, or a watcher who is a player — none of
// which this build writes (ErrInvalidData).
func (e *Encounter) validateSneakVerdicts() error {
	for mover, pairs := range e.sneakVerdicts {
		m, ok := e.members[mover]
		if !ok || m.Kind != KindPlayer || !m.Sneaking {
			return fmt.Errorf("sneak verdict for %q, who is not a sneaking player: %w", mover, ErrInvalidData)
		}
		for watcher := range pairs {
			w, ok := e.members[watcher]
			if !ok || w.Kind == KindPlayer {
				return fmt.Errorf("sneak verdict for %q against %q, who is no watcher: %w", mover, watcher, ErrInvalidData)
			}
		}
	}

	return nil
}

// validateSneaking refuses a loaded mode on a non-player and a verdict this
// build would not write (ErrInvalidData).
func (e *Encounter) validateSneaking() error {
	for _, id := range e.rosterIDs() {
		if m := e.members[id]; m.Sneaking && m.Kind != KindPlayer {
			return fmt.Errorf("member %q sneaks, but only a player sneaks: %w", id, ErrInvalidData)
		}
	}

	return e.validateSneakVerdicts()
}
