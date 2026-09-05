// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

// flip.go is KNOWLEDGE ARRIVING, AND WHAT IT CHANGES (rpg-project#375, the
// hold-out design §3.3–§3.6, §3.8): a member comes to know a fact, the graph
// folds the pair's stance from it, and the composition notices what that
// folded to — a beat, a fight that has no sides left, an ending.
//
// # Nothing here decides a stance
//
// [Encounter.learnFact] appends ONE fact and then asks the graph twice, before
// and after. Which pairs turned is the difference between two folds; the
// composition keeps neither, and could not decide the answer if it wanted to
// — the Settle in the graph's declaration is the only thing that knows a
// chief's knowledge is the camp's (R11, "the graph should tell the truth").
//
// # The three predicate sites
//
// Predicates are evaluated where each event is noticed, before that verb's
// sight refresh (design §3.8): a fact at its append (here), a round at the
// RoundStarted milestone ([Encounter.noticeRounds]), a stance in the fold
// after a flip ([Encounter.settleStances]). A member down keeps its home in
// noticeDown.

// learnFact writes the fact that a member came to know a fact — actor and
// subject the learner, audience the learner alone (the subject is what the
// graph's Raise flags; the audience is whose fold carries it) — and then
// notices what the world folds to now. Idempotent: knowledge already held is
// never re-written.
func (e *Encounter) learnFact(member MemberID, id FactID, cause string, at uint64) error {
	if e.world.knowsFact(member, id) {
		return nil
	}
	before := e.stanceTable()
	if _, err := e.world.log.Append(journal.Fact{
		Kind:     factKnownKind(id),
		Actor:    journal.EntityID(member),
		Subject:  journal.EntityID(member),
		Audience: journal.Audience{journal.EntityID(member)},
		Outcome:  journal.Outcome{Detail: cause},
	}); err != nil {
		return fmt.Errorf("learn fact %q: %w", id, err)
	}

	if err := e.settleStances(before, at); err != nil {
		return err
	}
	// The fact site (design §3.8), on the TRUTH grain (R5): whatever waited
	// for this fact to exist in the run's journal arrives now — after the
	// flip it may have caused has settled, before the endings it may fire.
	if err := e.arrivals(onFact(id), at); err != nil {
		return fmt.Errorf("fact %q arrivals: %w", id, err)
	}
	return e.firedFact(id, at)
}

// settleStances notices every pair whose stance the last append turned: the
// `stance` beat to everyone (truth grain, like a door's state), the fights
// that lost their sides, and the stance endings that now hold. before is the
// fold from before the append; the fold after is asked here.
func (e *Encounter) settleStances(before map[factionPair]Stance, at uint64) error {
	after := e.stanceTable()
	turned := false
	for _, pair := range e.turnablePairs() {
		if before[pair] != StanceHostile || after[pair] == StanceHostile {
			continue
		}
		turned = true
		if err := e.appendStanceBeat(pair, after[pair], at); err != nil {
			return err
		}
	}
	if !turned {
		return nil
	}

	// A FIGHT WITH NO SIDES LEFT ENDS (R1, ByStance). A fight that still has
	// a hostile pair in it goes on — members of a third faction still
	// hostile keep their fight — and whoever in it is opposed to nobody any
	// more steps back to the world clock: they appear in no pair, so they are
	// in no fight.
	for _, bubble := range append([]*clock.Turn(nil), e.bubbles...) {
		order, err := bubble.Order()
		if err != nil {
			return fmt.Errorf("settle stances: %w", err)
		}
		members := make([]MemberID, 0, len(order))
		for _, id := range order {
			members = append(members, MemberID(id))
		}
		anyOpposed := false
		var idle []MemberID
		for _, a := range members {
			hasFoe := false
			for _, b := range members {
				if a != b && e.opposed(a, b) {
					hasFoe = true
				}
			}
			anyOpposed = anyOpposed || hasFoe
			if !hasFoe {
				idle = append(idle, a)
			}
		}
		if !anyOpposed {
			if _, err := e.dissolveBubble(bubble, ByStance()); err != nil {
				return fmt.Errorf("settle stances: %w", err)
			}
			continue
		}
		for _, id := range idle {
			if _, err := e.Transfer(&TransferInput{Member: id, To: ClockWorld}); err != nil {
				return fmt.Errorf("settle stances: %q leaves the fight: %w", id, err)
			}
		}
	}

	// The stance site (design §3.8; reserve.go): whatever waited for one of
	// the turned pairs to fold to this stance arrives now, before the endings
	// below.
	if err := e.arrivals(onStance(before, after), at); err != nil {
		return fmt.Errorf("stance arrivals: %w", err)
	}

	// The stance endings, in the fold after the flip — declaration order,
	// first match wins, the same walk every ending scan makes.
	if e.outcome != nil {
		return nil
	}
	for _, de := range e.endings {
		ts, ok := de.trigger.(TriggerStance)
		if !ok {
			continue
		}
		if after[pairOf(ts.Between[0], ts.Between[1])] != ts.Stance {
			continue
		}
		if _, err := e.closeWith(de.key, at); err != nil {
			return fmt.Errorf("stance ending %q: %w", de.key, err)
		}
		return nil
	}
	return nil
}

// appendStanceBeat records that a pair's stance turned, to everyone: a stance
// is truth grain, like a door's state — the same for every member (design §6:
// STANCE_CHANGED goes to everyone in the run). The pair is UNORDERED and is
// written in its one normalized order, so two flips of one pair read the
// same whichever way the file spelled it.
func (e *Encounter) appendStanceBeat(pair factionPair, to Stance, at uint64) error {
	payload, err := json.Marshal(map[string]interface{}{
		"beat":    "stance",
		"between": []FactionID{pair.a, pair.b},
		"stance":  string(to),
	})
	if err != nil {
		return fmt.Errorf("stance beat payload: %w", err)
	}
	if _, err := e.appendBeat(&record.AppendInput{
		At:       at,
		Audience: e.audienceFor(tableBeat),
		Tags:     map[string]string{"tag": "world"},
		Payload:  payload,
	}); err != nil {
		return fmt.Errorf("stance append beat: %w", err)
	}
	return nil
}

// firedFact evaluates every declared fact ending against the fact just
// learned — the truth grain: it exists in the run's journal, learned by
// anyone (design R5) — closing through the one close path if one fires.
// Declaration order decides when two could fire at once.
func (e *Encounter) firedFact(id FactID, at uint64) error {
	if e.outcome != nil {
		return nil
	}
	for _, de := range e.endings {
		tf, ok := de.trigger.(TriggerFact)
		if !ok || tf.Fact != id {
			continue
		}
		if _, err := e.closeWith(de.key, at); err != nil {
			return fmt.Errorf("fact ending %q: %w", de.key, err)
		}
		return nil
	}
	return nil
}

// noticeRounds evaluates every declared round ending at the one place a
// round is noticed — the RoundStarted milestone a turn ending or a formation
// crosses (design §3.8, R9: the fight's own clock, never the world's). An
// advance that started no round notices nothing.
func (e *Encounter) noticeRounds(ms []clock.Milestone) error {
	for _, m := range ms {
		if m.Kind != clock.RoundStarted || e.outcome != nil {
			continue
		}
		// The round site (design §3.8, R9; reserve.go): whatever waited for
		// this round of a fight arrives now, before the endings below. A
		// member arriving refreshes sight inside this call and joins the
		// fight whose round this is, as a straggler walking into view would.
		if err := e.arrivals(onRound(m.Round), uint64(e.clock.ToData().HighWater)); err != nil {
			return fmt.Errorf("round %d arrivals: %w", m.Round, err)
		}
		if e.outcome != nil {
			continue
		}
		for _, de := range e.endings {
			tr, ok := de.trigger.(TriggerRound)
			if !ok || tr.Round != m.Round {
				continue
			}
			if _, err := e.closeWith(de.key, uint64(e.clock.ToData().HighWater)); err != nil {
				return fmt.Errorf("round ending %q: %w", de.key, err)
			}
			break
		}
	}
	return nil
}

// sweepPresence is PRESENCE TRANSFER (design §3.6, R3 — "we throw it at the
// chief, or be in the same room as the chief holding the letter"): every
// member holding a record that reveals a fact teaches every faction mind
// standing in the same region. The record COPIES — the mind holds it too,
// and what it reveals is applied through the one routine a record is applied
// by — and the holder keeps it. Idempotent: a mind already holding the record
// is not taught it again.
//
// A mind that is Down can no longer learn (design §3.9 — a consequence, not a
// loss), so standing is asked, once, only when there is a candidate to ask
// about: a sweep with nobody carrying anything consults nothing.
//
// PRESENCE IS SYMMETRIC BY CONSTRUCTION (R3: the same region, nothing about
// who arrived). The mind walking INTO the carrier's region teaches him
// exactly as the carrier walking in does — a chief who charges into the
// yard after the letter's holder learns what the letter says and stands
// down. Kirk saw this on the raider camp (2026-09-05); it is the rule's
// own consequence, flagged as behaviour, not changed here.
func (e *Encounter) sweepPresence(at uint64) error {
	var down map[MemberID]bool
	for _, holder := range e.rosterIDs() {
		records := e.factRecordsHeldBy(holder)
		if len(records) == 0 {
			continue
		}
		cell, placed := e.canvas.GetEntityPosition(string(holder))
		if !placed {
			continue
		}
		region, owned := e.field.regionOf(cell)
		if !owned {
			continue
		}
		for _, faction := range e.field.factionIDs() {
			mind := e.world.minds[faction]
			if mind == "" || mind == holder {
				continue
			}
			mindCell, placed := e.canvas.GetEntityPosition(string(mind))
			if !placed {
				continue
			}
			if r, ok := e.field.regionOf(mindCell); !ok || r != region {
				continue
			}
			if down == nil {
				var err error
				if down, err = e.standingNow(); err != nil {
					return fmt.Errorf("presence transfer: %w", err)
				}
			}
			if down[mind] {
				continue
			}
			for _, rec := range records {
				if e.holdings.holdsRecord(mind, rec) {
					continue
				}
				if err := e.holdings.holdIntel(mind, rec, "carried into its presence by "+string(holder)); err != nil {
					return fmt.Errorf("presence transfer %q to %q: %w", rec, mind, err)
				}
				if err := e.applyReveals(mind, rec, at); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// factRecordsHeldBy is every record a member carries that reveals a fact —
// held directly, or on a prop they hold — sorted, deduplicated. The only
// question presence transfer asks of holdings.
func (e *Encounter) factRecordsHeldBy(member MemberID) []IntelID {
	seen := make(map[IntelID]bool)
	consider := func(id IntelID) {
		if rec, ok := e.field.intelByID[id]; ok && rec.Reveals.Fact != "" {
			seen[id] = true
		}
	}
	for _, item := range e.holdings.holdingsOf(member) {
		switch {
		case item.record != "":
			consider(item.record)
		case item.prop != "":
			if i := e.field.propIndexOf(item.prop); i >= 0 {
				for _, id := range e.field.props[i].Holds {
					consider(id)
				}
			}
		}
	}
	out := make([]IntelID, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
