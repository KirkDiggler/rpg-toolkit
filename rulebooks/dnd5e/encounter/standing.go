// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// Standing is the legacy binary source shape retained at constructor and
// resolution boundaries. NewEncounter and LoadEncounter require its concrete
// value to also implement [Participation], returning [ErrNoParticipation]
// otherwise. Play consults only that richer assessment; it never falls back to
// Standing or treats nil as everyone active.
//
// Keeping this interface lets existing Standing: fields remain source-compatible
// while making participation an explicit required capability. Implementations
// should answer both methods from the same rulebook truth.
type Standing interface {
	// Standing reports which of the given members are down. Returning none is
	// the ordinary answer.
	Standing(members []MemberID) (down []MemberID, err error)
}

// standingNow asks the capability who is down, right now, and returns the
// answer as a set.
//
// The roster goes over SORTED, for C8's reason: what a pass concludes must be a
// function of persisted data rather than of map iteration order, and a
// capability handed its question in a different order each time could answer
// differently each time. An empty roster is not a question worth asking, and
// the capability is not called for one.
func (e *Encounter) standingNow() (map[MemberID]bool, error) {
	participation, err := e.participationNow()
	if err != nil {
		return nil, err
	}
	return participation.down, nil
}

type participationPassInput struct {
	// newlyActive names clocks whose active slot was created before this pass,
	// such as EndTurn's successor. Mid-turn Record supplies none.
	newlyActive []*clock.Turn
	// deferReconcile keeps a one-sided bubble in place for the same call that
	// records a stabilized or recovered Death Save. Its explicit continuation
	// keeps control until the next turn-settlement boundary.
	deferReconcile bool
}

// noticeDown performs one complete participation pass. The historical name is
// retained for the call graph, but Down now governs narration only: Contact
// chooses sides, Turn chooses retain/auto-pass/remove, and PartyDefeated is the
// supplied run-ending policy answer.
//
// The order is causal and deterministic: append every new Down beat in sorted
// order; close a supplied party defeat; apply sorted Remove consequences;
// retain or reconcile one-sided order by supplied group policy; settle only
// newly active slots; then evaluate legacy declared MemberDown endings. Wait
// never moves a slot. The story remains the down-narration ledger, so a
// retained beat aging out may be told again rather than introducing duplicate
// persisted life state.
func (e *Encounter) noticeDown(
	inputs ...participationPassInput,
) (*participationState, map[MemberID]*IntelDelta, error) {
	in := participationPassInput{}
	if len(inputs) > 0 {
		in = inputs[0]
	}
	participation, err := e.participationNow()
	if err != nil {
		return nil, nil, err
	}
	down := participation.down

	// First narrate every newly down member. Down is a story fact only; it no
	// longer decides whether an initiative slot is retained or removed.
	if len(down) > 0 {
		told, terr := e.storyToldDown()
		if terr != nil {
			return nil, nil, terr
		}

		fallen := make([]MemberID, 0, len(down))
		for id := range down {
			fallen = append(fallen, id)
		}
		sort.Slice(fallen, func(i, j int) bool { return fallen[i] < fallen[j] })
		for _, id := range fallen {
			if told[id] {
				continue
			}
			if berr := e.appendDownBeat(id); berr != nil {
				return nil, nil, berr
			}
		}
	}

	// Party defeat is the rulebook's policy answer, not a threshold this
	// composition derives. It closes only after all causal down beats.
	if e.outcome == nil && participation.assessment.PartyDefeated {
		if _, cerr := e.closeWith(partyDefeatedEnding, uint64(e.clock.ToData().HighWater)); cerr != nil {
			return nil, nil, fmt.Errorf("participation party defeat: %w", cerr)
		}
		return participation, nil, nil
	}

	// Remove is the only answer that splices initiative. Wait (including a
	// dying member) and AutoPass (including a stabilized member) retain their
	// exact slots. All removals happen before scheduling, so an active member
	// becoming Remove hands the clock forward exactly once.
	toSettle := make(map[*clock.Turn]bool, len(in.newlyActive))
	for _, bubble := range in.newlyActive {
		toSettle[bubble] = true
	}
	removed := make([]MemberID, 0)
	for id, member := range participation.members {
		if member.Turn == TurnParticipationRemove {
			removed = append(removed, id)
		}
	}
	sort.Slice(removed, func(i, j int) bool { return removed[i] < removed[j] })

	var intelDeltas map[MemberID]*IntelDelta
	for _, id := range removed {
		bubble, berr := e.bubbleFor(id)
		if berr != nil {
			return nil, nil, fmt.Errorf("participation bubble %q: %w", id, berr)
		}
		if bubble == nil {
			continue
		}

		active, aerr := bubble.Active()
		if aerr != nil {
			return nil, nil, fmt.Errorf("participation active %q: %w", id, aerr)
		}
		removedWasActive := MemberID(active) == id

		// KeepTurnOrder is the supplied exception to ordinary fight defeat:
		// remove the member but preserve the one-sided clock for ordered
		// follow-up such as Death Saves.
		if !participation.assessment.KeepTurnOrder {
			decided, derr := e.fightIsDecided(bubble, participation)
			if derr != nil {
				return nil, nil, fmt.Errorf("participation fight %q: %w", id, derr)
			}
			if decided {
				if _, xerr := e.dissolveBubble(bubble, ByDefeat()); xerr != nil {
					return nil, nil, fmt.Errorf("participation dissolve %q: %w", id, xerr)
				}
				delete(toSettle, bubble)
				continue
			}
		}

		transferred, terr := e.transfer(&TransferInput{Member: id, To: ClockWorld}, false)
		if terr != nil {
			return nil, nil, fmt.Errorf("participation transfer %q: %w", id, terr)
		}
		intelDeltas = mergeIntelDeltas(intelDeltas, transferred.IntelDeltas)
		if removedWasActive {
			toSettle[bubble] = true
		}
	}

	// A later false KeepTurnOrder reconciles a one-sided bubble at its next
	// settlement boundary, or while it still holds a down member whose ordered
	// follow-up was the reason to retain it. This does not reinterpret an
	// unrelated one-member fight as defeat merely because somebody elsewhere
	// is down. Terminal Death Save detail defers same-call reconciliation so
	// its explicit Continuation remains authoritative.
	settlementOrder := append([]*clock.Turn(nil), e.bubbles...)
	if !participation.assessment.KeepTurnOrder && !in.deferReconcile {
		for _, bubble := range settlementOrder {
			if !toSettle[bubble] {
				order, oerr := bubble.Order()
				if oerr != nil {
					return nil, nil, fmt.Errorf("participation reconcile order: %w", oerr)
				}
				holdsDown := false
				for _, id := range order {
					if participation.down[id] {
						holdsDown = true
						break
					}
				}
				if !holdsDown {
					continue
				}
			}
			decided, derr := e.fightIsDecided(bubble, participation)
			if derr != nil {
				return nil, nil, fmt.Errorf("participation reconcile: %w", derr)
			}
			if !decided {
				continue
			}
			if _, xerr := e.dissolveBubble(bubble, ByDefeat()); xerr != nil {
				return nil, nil, fmt.Errorf("participation reconcile dissolve: %w", xerr)
			}
			delete(toSettle, bubble)
		}
	}

	// Drive only slots that became active in this pass. An already-active slot
	// whose mid-turn Record changes to AutoPass remains active until its ruled
	// continuation explicitly settles the turn.
	for _, bubble := range settlementOrder {
		if !toSettle[bubble] {
			continue
		}
		order, oerr := bubble.Order()
		if oerr != nil {
			return nil, nil, fmt.Errorf("participation order: %w", oerr)
		}
		if len(order) == 0 || !e.bubbleHasPlayer(order) {
			continue
		}
		drivenWrapped, drivenSeq, drivenDeltas, derr := e.driveTurnsWithParticipation(bubble, participation)
		if derr != nil {
			return nil, nil, fmt.Errorf("participation drive: %w", derr)
		}
		participation.scheduledWrapped = participation.scheduledWrapped || drivenWrapped
		if drivenSeq != 0 {
			participation.scheduledLastSeq = drivenSeq
		}
		intelDeltas = mergeIntelDeltas(intelDeltas, drivenDeltas)
	}

	// Preserve declared MemberDown endings. They still key off Down, but no
	// initiative consequence does.
	if e.outcome == nil && len(down) > 0 {
		if err := e.firedMemberDown(down); err != nil {
			return nil, nil, err
		}
	}

	return participation, intelDeltas, nil
}

// firedMemberDown evaluates every declared MemberDown ending against who is
// down, closing the encounter through the one close path if one fires.
// Declaration order decides when two could fire at once — the same order
// every ending scan walks.
func (e *Encounter) firedMemberDown(down map[MemberID]bool) error {
	for _, de := range e.endings {
		trigger, ok := de.trigger.(TriggerMemberDown)
		if !ok {
			continue
		}
		if !down[trigger.Member] {
			continue
		}
		at := uint64(e.clock.ToData().HighWater)
		if _, err := e.closeWith(de.key, at); err != nil {
			return fmt.Errorf("member-down ending %q: %w", de.key, err)
		}
		return nil
	}
	return nil
}

// fightIsDecided reports whether the complete supplied removal set leaves this
// bubble without a Contact side. It is reached before any one member transfers,
// so every TurnParticipationRemove member is excluded regardless of Contact:
// the census describes the post-removal bubble rather than its current order.
// Remove+Contact is rejected at capability ingress; the exclusion here remains
// defense in depth over this consequence boundary. Down is not consulted;
// retained dying and stabilized slots remain eligible exactly according to
// their independent Contact answers.
func (e *Encounter) fightIsDecided(bubble *clock.Turn, participation *participationState) (bool, error) {
	order, err := bubble.Order()
	if err != nil {
		return false, err
	}

	var players, monsters int
	for _, id := range order {
		member, ok := e.members[id]
		if !ok {
			// Unreachable against a coherent encounter — the load boundary
			// refuses a bubble holding a non-member, and every verb that can
			// remove one leaves the clock before it leaves the roster. Refused
			// rather than skipped anyway, because of the SHAPE of the wrong
			// answer: a ghost this pass cannot classify is a standing member it
			// does not count, so skipping makes the fight look MORE decided than
			// it is, and the ending would be written into the story and saved.
			// Loud, like ClockOf's on-no-clock check, for the same reason —
			// the caller's obligation on error is to drop the encounter unsaved.
			return false, fmt.Errorf("fight order holds %q, who is not a member: %w", id, ErrInvalidData)
		}
		memberParticipation, ok := participation.members[id]
		if !ok {
			return false, fmt.Errorf("fight order holds %q without a participation answer: %w", id, ErrInvalidData)
		}
		if memberParticipation.Turn == TurnParticipationRemove || !memberParticipation.Contact {
			continue
		}
		switch member.Kind {
		case KindPlayer:
			players++
		case KindMonster:
			monsters++
		}
	}

	return players == 0 || monsters == 0, nil
}

// storyToldDown reads back which members the RETAINED story already reports
// down. See noticeDown for why this is the ledger.
func (e *Encounter) storyToldDown() (map[MemberID]bool, error) {
	entries, err := e.story.All(&record.AllInput{Tags: map[string]string{"tag": "outcome"}})
	if err != nil {
		return nil, fmt.Errorf("standing story: %w", err)
	}

	told := make(map[MemberID]bool)
	for _, entry := range entries {
		var beat struct {
			Beat      string `json:"beat"`
			Member    string `json:"member"`
			Actor     string `json:"actor"`
			DeathSave *struct {
				Recovered bool `json:"recovered"`
			} `json:"death_save"`
		}
		// A payload this pass cannot read is not this pass's business. Every
		// beat this module writes is a JSON object and decodes here (unknown
		// keys are ignored), so the skip is for a shape that could only come
		// from a hand-edited blob — and refusing every verb over one is worse
		// than not counting it as a death already told.
		if json.Unmarshal(entry.Payload, &beat) != nil {
			continue
		}
		switch {
		case beat.Beat == string(OutcomeDown) && beat.Member != "":
			told[MemberID(beat.Member)] = true
		case beat.Beat == string(OutcomeDeathSave) && beat.Actor != "" &&
			beat.DeathSave != nil && beat.DeathSave.Recovered:
			// Recovery is already an authoritative closed Death Save fact. It
			// clears story-derived toldness so a later fall is new, without
			// inventing a second Up beat.
			delete(told, MemberID(beat.Actor))
		}
	}

	return told, nil
}

// appendDownBeat writes the minimal death beat: the kind, and who.
//
// Ruled fork (d) on rpg-toolkit#959 — everything a client needs to render a
// death, and nothing about hit points, which is a separate decision with its
// own case (census S3). Tag "outcome" and the current clock reading put it in
// the same family, with the same audience rule, as the struck and missed beats
// a client reads beside it.
func (e *Encounter) appendDownBeat(id MemberID) error {
	payload, err := json.Marshal(map[string]string{
		"beat":   string(OutcomeDown),
		"member": string(id),
	})
	if err != nil {
		return fmt.Errorf("standing beat payload: %w", err)
	}

	// subjectBeat, subject is the member going down — v1 still sends
	// everyone (audienceFor's doc).
	if _, aerr := e.appendBeat(&record.AppendInput{
		At:       uint64(e.clock.ToData().HighWater),
		Audience: e.audienceFor(subjectBeat, id),
		Tags:     map[string]string{"tag": "outcome"},
		Payload:  payload,
	}); aerr != nil {
		return fmt.Errorf("standing append beat: %w", aerr)
	}

	return nil
}
