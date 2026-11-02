// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// wakebeats.go is TRIGGERS WITH SCOPE (rpg-project#766 S4, design §3 of
// ideas/the-world-runs): a beat the record already writes — a sound heard, a
// deed done, an enemy sighted, a fact learned, a clock's round — wakes the
// machines that listen for it, in the raise it landed in.
//
// # Landed, then matched
//
// The sites that write those beats land one [wakeBeat] each on
// [Encounter.landed] — [Encounter.makeSound] (one per hearer),
// [Encounter.landDeedAt], [Encounter.learnFact], the sight refresh (one per
// observer that gained an opposed subject) and the raise itself (`clock`).
// The next raise ([Encounter.wake]) matches them against the listener index
// in landing order and, for each, its listeners in seat order, then clears
// them. A beat lands only when somebody listens for it, so content with no
// authored recipe lands nothing and its record is what it was.
//
// LANDED BEATS PERSIST UNTIL THE RAISE THAT MATCHES THEM. A clang in a fight
// turn, or a step short of a pace, raises nothing; the beat waits for the
// round that does, and a save between the two keeps it
// ([EncounterData.Landed]). Time has not passed, so nobody acts on it yet;
// when it does, the listeners hear it before they are asked the time.
//
// # Scope
//
// A `within` is hexes from the listener to where the beat happened, measured
// on the canvas; a `regions` is partitions crossed, the same hops a sound
// pays ([Encounter.hopsFrom]). Answered per listener at match time, never by
// walking the roster: a broadcast beat (a deed, a clock) costs its listeners,
// and a targeted one (heard, seen, fact) costs the one member it names.
//
// # A reaction does not spend the clock
//
// A step fired by any beat but `time` that picks a key runs the same unit
// `time` does ([Encounter.runPick]) and spends nothing: it is a reaction, and
// the creature's next `time` is unchanged. A transition re-lists the member
// and re-arms it from its new state.

// The beats this composition supplies the machine (design §3).
const (
	// beatHeard is a sound that landed on the listener.
	beatHeard machine.Beat = "heard"
	// beatDeed is a deed done anywhere; `deed:<word>` names one word.
	beatDeed machine.Beat = "deed"
	// beatSeen is an opposed subject coming into the listener's sight.
	beatSeen machine.Beat = "seen"
	// beatFact is a fact the listener learned.
	beatFact machine.Beat = "fact"
	// beatClock prefixes a clock's round: `clock:world`, `clock:fight`,
	// `clock:bell:<prop>`, or an area clock's name.
	beatClock = "clock:"
	// deedPrefix prefixes one deed word: `deed:attacked`.
	deedPrefix = "deed:"
	// clockFight is the name of the round of a fight the listener's faction
	// is engaged in.
	clockFight = "fight"
	// bellPrefix names a prop's deed as a clock: `bell:<prop>`.
	bellPrefix = "bell:"
	// factPrefix prefixes a fact a guard reads: `fact:<id>`.
	factPrefix = "fact:"
	// factEnemy is the fact a guard reads the enemy band from.
	factEnemy machine.FactName = "enemy"
)

// wakeBeatBound is how many landed beats one pass matches: the anti-spin
// bound a fight turn has, for a pass whose reactions land beats of their own.
// A pass that reaches it drops the rest and says so ([BeatWakeCut], design
// §2). A variable only so a test can reach it.
var wakeBeatBound = 4096

// clockBeat is the beat a clock's round lands under.
func clockBeat(name string) machine.Beat { return machine.Beat(beatClock + name) }

// wakeBeat is one beat landed for the listeners: which beat, where it
// happened, whom it is about, the faction it addresses (`clock:fight`), the
// round it happened in, and — for a targeted beat — the one member it landed
// on, with the regions it crossed to reach them.
type wakeBeat struct {
	Beat    machine.Beat
	Cell    *spatial.Position
	Subject MemberID
	Member  MemberID
	Faction FactionID
	Hops    int
	At      uint64
}

// wakeEv is a beat as one listener is asked it: the round, and how far it
// was from the listener in hexes and regions (-1 when the beat names no
// place, which no scope admits).
type wakeEv struct {
	At       uint64
	Distance int
	Hops     int
}

// inScope is the scoped beats' InScope: within hexes, within regions, both
// when both are set.
func inScope(ev any, s machine.ScopeLeaf) bool {
	w, ok := ev.(wakeEv)
	if !ok || w.Distance < 0 {
		return false
	}
	if s.Within > 0 && w.Distance > s.Within {
		return false
	}
	if s.Regions > 0 && (w.Hops < 0 || w.Hops > s.Regions) {
		return false
	}

	return true
}

// wakeRound is every beat's Round: the round it happened in.
func wakeRound(ev any) (uint64, bool) {
	switch w := ev.(type) {
	case wakeEv:
		return w.At, true
	case tickEv:
		return w.Round, true
	}

	return 0, false
}

// listened reports whether anybody listens for one of these beats, so a
// site lands nothing for content nobody wrote a recipe for.
func (e *Encounter) listened(beats ...machine.Beat) bool {
	for _, b := range beats {
		if len(e.listeners.For(b)) > 0 {
			return true
		}
	}

	return false
}

// listensFor reports whether one member listens for a beat, and with what
// scope.
func (e *Encounter) listensFor(id MemberID, b machine.Beat) (*machine.ScopeLeaf, bool) {
	ls := e.listeners.For(b)
	i := sort.Search(len(ls), func(i int) bool { return ls[i].Key >= id })
	if i < len(ls) && ls[i].Key == id {
		return ls[i].Scope, true
	}

	return nil, false
}

// land appends one beat for the next raise to match.
func (e *Encounter) land(b wakeBeat) {
	e.landed = append(e.landed, b)
}

// landHeard lands a sound on each hearer that listens for `heard`.
func (e *Encounter) landHeard(heard []hearing, where spatial.Position, maker MemberID, at uint64) {
	if !e.listened(beatHeard) {
		return
	}
	for _, h := range heard {
		if _, ok := e.listensFor(h.member, beatHeard); !ok {
			continue
		}
		cell := where
		e.land(wakeBeat{Beat: beatHeard, Cell: &cell, Subject: maker, Member: h.member, Hops: h.hops, At: at})
	}
}

// deedWord is the author's word for a deed verb (`attack` is `attacked`), or
// empty for a verb no `when` word names.
func deedWord(verb string) string {
	for _, w := range WhenDeeds {
		if behavior.DeedVerbFor(w) == verb {
			return w
		}
	}

	return ""
}

// landDeedBeat lands one deed for whoever listens for deeds, or for that
// deed's word.
func (e *Encounter) landDeedBeat(verb string, actor MemberID, where spatial.Position, at uint64) {
	named := machine.Beat(deedPrefix + deedWord(verb))
	for _, b := range []machine.Beat{named, beatDeed} {
		if b == deedPrefix || !e.listened(b) {
			continue
		}
		cell := where
		e.land(wakeBeat{Beat: b, Cell: &cell, Subject: actor, At: at})
	}
}

// landFactBeat lands a fact on the member who learned it, when it listens.
func (e *Encounter) landFactBeat(member MemberID, at uint64) {
	if _, ok := e.listensFor(member, beatFact); !ok {
		return
	}
	e.land(wakeBeat{Beat: beatFact, Member: member, At: at})
}

// landSightings lands one `seen` per observer that listens for it and gained
// an opposed subject in this refresh, at the subject's cell.
func (e *Encounter) landSightings(deltas map[MemberID]*IntelDelta, at uint64) {
	if !e.listened(beatSeen) {
		return
	}
	for _, l := range e.listeners.For(beatSeen) {
		delta := deltas[l.Key]
		if delta == nil {
			continue
		}
		gained := make([]MemberID, 0, len(delta.FirstContact)+len(delta.Reacquired))
		for _, p := range delta.FirstContact {
			gained = append(gained, MemberID(p.ID))
		}
		for _, id := range delta.Reacquired {
			gained = append(gained, MemberID(id))
		}
		sort.Slice(gained, func(i, j int) bool { return gained[i] < gained[j] })
		for _, subject := range gained {
			m, ok := e.members[subject]
			if !ok || !e.opposed(l.Key, subject) {
				continue
			}
			cell, err := e.cellOf(m)
			if err != nil {
				continue
			}
			e.land(wakeBeat{Beat: beatSeen, Cell: &cell, Subject: subject, Member: l.Key, At: at})
			break // one sighting wakes it; the rest are the same news
		}
	}
}

// landClock lands one clock's round for its listeners: `world` for every
// member listening, `fight` for the members of one faction.
func (e *Encounter) landClock(name string, faction FactionID, at uint64) {
	b := clockBeat(name)
	if !e.listened(b) {
		return
	}
	e.land(wakeBeat{Beat: b, Faction: faction, At: at})
}

// matchLanded steps every listener of every landed beat, in landing order and
// seat order, then clears them. A reaction can land more beats; they are
// matched in this pass, up to [wakeBeatBound].
//
// Errors: a machine refused its blob, or a reaction's pick failed.
func (e *Encounter) matchLanded() error {
	for len(e.landed) > 0 {
		batch := e.landed
		e.landed = nil
		for i, b := range batch {
			if e.outcome != nil {
				e.landed = nil
				return nil
			}
			// THE BOUND IS THE PASS'S, counted across every call this raise
			// makes ([Encounter.wakeMatched], reset by [Encounter.wakePass]).
			if e.wakeMatched >= wakeBeatBound {
				dropped := len(batch) - i + len(e.landed)
				e.landed = nil
				return e.appendWakeCutBeat(dropped)
			}
			e.wakeMatched++
			if err := e.matchOne(b); err != nil {
				return err
			}
		}
	}

	return nil
}

// appendWakeCutBeat records a pass cut at [wakeBeatBound]: how many landed
// beats it matched and how many it dropped unmatched, audience everyone, as
// the tick that framed the pass.
func (e *Encounter) appendWakeCutBeat(dropped int) error {
	payload, err := json.Marshal(map[string]interface{}{
		"beat":    BeatWakeCut,
		"matched": e.wakeMatched,
		"dropped": dropped,
	})
	if err != nil {
		return fmt.Errorf("wake cut beat: %w", err)
	}
	if _, err := e.appendBeat(&record.AppendInput{
		At:       uint64(e.clock.ToData().HighWater),
		Audience: e.audienceFor(tableBeat),
		Tags:     map[string]string{"tag": BeatWakeCut},
		Payload:  payload,
	}); err != nil {
		return fmt.Errorf("wake cut beat: %w", err)
	}

	return nil
}

// matchOne steps the listeners of one landed beat: the one member a targeted
// beat names, or every listener of a broadcast one, in seat order.
func (e *Encounter) matchOne(b wakeBeat) error {
	if b.Member != "" {
		scope, ok := e.listensFor(b.Member, b.Beat)
		if !ok {
			return nil
		}
		return e.react(b.Member, b, scope)
	}
	// The slice For returns is replaced, never written, by a Set while it is
	// walked (machine.Listeners), so a reaction that re-lists is safe here.
	for _, l := range e.listeners.For(b.Beat) {
		if l.Key == b.Subject {
			continue // the doer is not woken by its own deed
		}
		if b.Beat == clockBeat(clockFight) {
			m, ok := e.members[l.Key]
			if !ok || factionOf(m) != b.Faction {
				continue
			}
		}
		if err := e.react(l.Key, b, l.Scope); err != nil {
			return err
		}
		if e.outcome != nil {
			return nil
		}
	}

	return nil
}

// evFor is a landed beat as one listener is asked it. An error only when the
// field's partitions cannot be derived ([field.partitionsNow]).
func (e *Encounter) evFor(id MemberID, m *memberRecord, b wakeBeat) (wakeEv, error) {
	ev := wakeEv{At: b.At, Distance: -1, Hops: -1}
	if b.Cell == nil {
		return ev, nil
	}
	own, err := e.cellOf(m)
	if err != nil {
		return ev, nil // an unplaced listener is at no distance, as before
	}
	ev.Distance = int(e.Distance(own, *b.Cell))
	if b.Member == id && b.Beat == beatHeard {
		ev.Hops = b.Hops
		return ev, nil
	}
	parts, err := e.field.partitionsNow()
	if err != nil {
		return wakeEv{}, err
	}
	if origin, ok := parts.of[*b.Cell]; ok {
		if at, ok := parts.of[own]; ok {
			hops, err := e.hopsFrom(origin)
			if err != nil {
				return wakeEv{}, err
			}
			if h, reached := hops[at]; reached {
				ev.Hops = h
			}
		}
	}

	return ev, nil
}

// react steps one listener's machine on a landed beat, when it is on the
// world clock, standing, and the beat is within the scope its state
// requires: the transition's beat and re-list, its picks run as reactions,
// and its re-arm.
//
// Errors: a machine refused its blob, or a pick failed.
func (e *Encounter) react(id MemberID, b wakeBeat, scope *machine.ScopeLeaf) error {
	m, ok := e.members[id]
	if !ok || !onDefaultClock(m) {
		return nil
	}
	p := e.programFor(m)
	if p == nil {
		return nil
	}
	on, err := e.onWorldClock(id)
	if err != nil || !on {
		return err
	}
	down, err := e.downNow()
	if err != nil {
		return fmt.Errorf("world wakes standing: %w", err)
	}
	if down[id] {
		return nil
	}
	ev, err := e.evFor(id, m, b)
	if err != nil {
		return fmt.Errorf("world wakes %q on %s: %w", id, b.Beat, err)
	}
	if scope != nil && !inScope(ev, *scope) {
		return nil // the cheap answer first: out of the state's own scope
	}
	now := uint64(e.clock.ToData().HighWater)
	stepped, err := p.Step(&m.Machine, b.Beat, ev, e.envFor(id), now)
	if err != nil {
		return fmt.Errorf("world wakes %q on %s: %w", id, b.Beat, err)
	}
	if !stepped.Fired {
		return nil
	}

	return e.afterStep(id, m, p, stepped, now, false)
}

// afterStep is what both a time step and a reaction do once the machine has
// fired: the transition's beat and re-list (and the end of a sleep the
// transition did not renew), the picks, and the re-arm.
//
// A TRANSITION IS TRUE THE MOMENT IT FIRES, whatever its pick then does: the
// beat and the relist land before the pick, so a pick that takes the member
// off the world clock cannot leave its state changed silently.
//
// spend is true for a `time` step, which pays one round of its own and the
// rounds it slept before it; a reaction spends nothing ([Encounter.forgiveOwed]
// when it re-arms).
func (e *Encounter) afterStep(id MemberID, m *memberRecord, p *machine.Program, stepped machine.Stepped, round uint64, spend bool) error {
	if stepped.To != "" {
		if err := e.appendStateBeat(id, stepped); err != nil {
			return err
		}
		e.listeners.Set(id, p.Listens(&m.Machine))
		// A NEW STATE ENDS THE OLD ONE'S SLEEP unless the transition slept
		// again: a guard woken by a clang is awake, not asleep until the
		// round its old wake named.
		if !hasWake(stepped.Acts) {
			m.Machine.WakeAt = nil
		}
	}
	// THE ROUNDS THIS STEP PAYS, read before the pick can raise the world
	// again: one for its own, and every round it slept before it (an
	// authored wake_in), so what stays owed is exactly the rounds after the
	// one it stepped in — the catch-up the heap still holds (#774).
	pay := 0
	if spend {
		budget, err := e.clock.Budget(&clock.BudgetInput{ID: core.EntityID(id)})
		if err != nil {
			return fmt.Errorf("world wakes budget %q: %w", id, err)
		}
		pay = budget - int(uint64(e.clock.ToData().HighWater)-round) //nolint:gosec // rounds are far below the int range
		pay = max(1, min(pay, budget))
	}
	for _, act := range stepped.Acts {
		if act.Pick == "" {
			continue
		}
		if !spend {
			e.reactionPicked = true
		}
		if err := e.runPick(id, m, AnswerKey(act.Pick)); err != nil {
			return err
		}
		if e.outcome != nil {
			return nil
		}
	}

	on, err := e.onWorldClock(id)
	if err != nil || !on {
		return err
	}
	if spend {
		if pay > 0 {
			if _, err := e.clock.Spend(&clock.SpendInput{ID: core.EntityID(id), Amount: pay}); err != nil {
				return fmt.Errorf("world wakes spend %q: %w", id, err)
			}
		}
		e.armAt(id, round)
		return nil
	}
	if stepped.To != "" || hasWake(stepped.Acts) {
		now := uint64(e.clock.ToData().HighWater)
		// A CREATURE ALREADY DUE KEEPS ITS ROUND. A posted guard who hears
		// the clang that the raise's own verb made is due in this raise; the
		// alert it enters wants time too, so it is asked in this raise, as
		// it would have been — a reaction never costs a turn (R4).
		if cur, armed := e.dueAt[id]; armed && cur <= now+1 && !hasWake(stepped.Acts) {
			if _, due := p.DueAt(&m.Machine, now); due {
				return nil
			}
		}
		e.armAt(id, now)
		// Armed from a reaction at the next round at the earliest, or taken
		// off the heap: nothing is owed, and the rounds it slept through are
		// not owed either.
		return e.forgiveOwed(id)
	}

	return nil
}

// forgiveOwed spends a member's whole world budget: the rounds it was granted
// while it was not due are not rounds it owes (#774). Without it, a creature
// woken from a state with no `time` would read its slept rounds as owed at
// the next load ([Encounter.armOwed]) and replay them.
//
// Errors: a clock read or spend failed.
func (e *Encounter) forgiveOwed(id MemberID) error {
	budget, err := e.clock.Budget(&clock.BudgetInput{ID: core.EntityID(id)})
	if err != nil {
		return fmt.Errorf("world wakes budget %q: %w", id, err)
	}
	if budget <= 0 {
		return nil
	}
	if _, err := e.clock.Spend(&clock.SpendInput{ID: core.EntityID(id), Amount: budget}); err != nil {
		return fmt.Errorf("world wakes spend %q: %w", id, err)
	}

	return nil
}

// hasWake reports whether a transition's acts arm a wake.
func hasWake(acts []machine.Act) bool {
	for _, a := range acts {
		if a.WakeIn > 0 {
			return true
		}
	}

	return false
}

// landFightRound lands `clock: fight` once for every faction among a fight
// round's fighters, in faction order: the round of the fight that faction is
// engaged in.
func (e *Encounter) landFightRound(fighters []MemberID, at uint64) {
	if !e.listened(clockBeat(clockFight)) {
		return
	}
	seen := map[FactionID]bool{}
	var factions []FactionID
	for _, id := range fighters {
		m, ok := e.members[id]
		if !ok {
			continue
		}
		f := factionOf(m)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		factions = append(factions, f)
	}
	sort.Slice(factions, func(i, j int) bool { return factions[i] < factions[j] })
	for _, f := range factions {
		e.landClock(clockFight, f, at)
	}
}

// WakeBeatData is the persistent representation of one landed beat
// ([EncounterData.Landed], rpg-project#766 S4): the beat, where it happened
// (omitted for one with no place), whom it is about, the one member a
// targeted beat landed on, the faction a `clock: fight` addresses, the
// regions a sound crossed, and the round it happened in.
type WakeBeatData struct {
	Beat    string        `json:"beat"`
	Cell    *PositionData `json:"cell,omitempty"`
	Subject MemberID      `json:"subject,omitempty"`
	Member  MemberID      `json:"member,omitempty"`
	Faction FactionID     `json:"faction,omitempty"`
	Hops    int           `json:"hops,omitempty"`
	At      uint64        `json:"at"`
}

// wakeBeatsData renders the landed beats, nil when none waits.
func wakeBeatsData(in []wakeBeat) []WakeBeatData {
	if len(in) == 0 {
		return nil
	}
	out := make([]WakeBeatData, len(in))
	for i, b := range in {
		d := WakeBeatData{
			Beat: string(b.Beat), Subject: b.Subject, Member: b.Member,
			Faction: b.Faction, Hops: b.Hops, At: b.At,
		}
		if b.Cell != nil {
			d.Cell = &PositionData{X: b.Cell.X, Y: b.Cell.Y}
		}
		out[i] = d
	}

	return out
}

// wakeBeatsFrom restores the landed beats. A beat no listener knows matches
// nobody, which is what a beat nobody listens for does at a raise; nothing
// here needs refusing.
func wakeBeatsFrom(in []WakeBeatData) []wakeBeat {
	if len(in) == 0 {
		return nil
	}
	out := make([]wakeBeat, len(in))
	for i, d := range in {
		b := wakeBeat{
			Beat: machine.Beat(d.Beat), Subject: d.Subject, Member: d.Member,
			Faction: d.Faction, Hops: d.Hops, At: d.At,
		}
		if d.Cell != nil {
			cell := spatial.Position{X: d.Cell.X, Y: d.Cell.Y}
			b.Cell = &cell
		}
		out[i] = b
	}

	return out
}
