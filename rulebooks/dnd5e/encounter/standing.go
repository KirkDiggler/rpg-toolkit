// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// Standing reports which of the given members are down — out of the fight,
// however the rulebook decides that. The composition asks; the rulebook
// answers.
//
// Injected rather than held, exactly as [Decider] and [InitiativeRoller] are,
// and for the same reason [InitiativeRoller]'s doc gives with "randomness"
// swapped for "hit points": this module's go.mod cannot import the rulebook
// (law C1), so defeat is a fact it can only be TOLD. Member IDs in, member IDs
// out — nothing here learns what a hit point is, and nothing here has to.
//
// # It is a pull, and that is the whole design
//
// The composition stores no down flag, in memory or in its blob. It asks at the
// choke points where the answer could matter and acts on the answer it gets.
// The alternative — being pushed a "this one is down" event and remembering it
// — recreates the dual state the seam reshape spent rpg-toolkit#1040 removing:
// heal a character and the sheet says four hit points while the composition
// still says down. There is one source of truth for defeat and it is not this
// package.
//
// Being a pull is also what makes it COMPLETE. Every route to zero — a strike,
// a hazard, a rule nobody has written yet — is noticed at the next consult,
// without that route knowing this interface exists.
//
// # What an implementation owes
//
// Answer about the members you were asked about. The composition hands over its
// current roster, and a name in the answer that was not in the question is a
// defect it refuses (ErrNotMember) rather than ignores — a mis-wired capability
// must look like a mis-wired capability, not like a rule that silently never
// fires. Order does not matter and duplicates are harmless.
//
// The answer is asked for again rather than carried between consults, including
// twice within one [Encounter.Pump]. Carrying it would be a cache, and a cache
// is the smallest possible version of the dual state above.
//
// Errors abort whatever verb was running, atomically (R5), the same as
// [InitiativeRoller]'s: a world that cannot find out who is standing does not
// half-act on a guess.
//
// # What being down governs today, and what it does not
//
// It governs the three things the death census (rpg-toolkit#959) named: a down
// member is on no side of a contact, so they neither start a fight nor join
// one; they take no turn, because they are spliced out of the order; and they
// take no [Encounter.Pump] action, because their decider is not consulted.
//
// It does NOT gate the caller-driven verbs. A down member can still be walked
// by [Encounter.Move] and still be the actor on an [Encounter.Record]. Both are
// the same gap seen twice: the swing stops because the TURN ORDER stops, and
// free roam has no turn order. Deliberate for this slice — the alternative is a
// refusal shape nobody has ruled on, and an unbuilt refusal beats a built-and-
// wrong one. The session supplies the real capability in the death lane's D4
// slice, which is where that call belongs.
//
// It also does not end a fight. A bubble whose whole living side is down keeps
// running here; self-dissolution is D2's ruling (ByDefeat), and this slice
// deliberately stops short of it.
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
	if len(e.members) == 0 {
		return nil, nil
	}

	roster := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		roster = append(roster, id)
	}
	sort.Slice(roster, func(i, j int) bool { return roster[i] < roster[j] })

	reported, err := e.standing.Standing(roster)
	if err != nil {
		return nil, fmt.Errorf("standing: %w", err)
	}

	down := make(map[MemberID]bool, len(reported))
	for _, id := range reported {
		if _, ok := e.members[id]; !ok {
			return nil, fmt.Errorf("standing: reported %q down, who is not a member: %w", id, ErrNotMember)
		}
		down[id] = true
	}

	return down, nil
}

// noticeDown is the world noticing, and it is the one place that happens.
//
// It pulls the standing answer, tells the story about anybody the story has not
// been told about yet, takes them out of whatever fight they were in, and hands
// the down set to [Encounter.applyTrigger], which has to classify contact
// knowing who is a body and who is an enemy.
//
// Placement: [Encounter.applyTrigger] is reached from [Encounter.refreshSight]
// — the choke point sight already flows through, so Move, Traverse, Pump, Join
// and Exit all pass here by writing the obvious call — and from Setup's first
// light, which calls applyTrigger directly so its scene-opened beat can land
// first. A scene can open with a body already on the floor.
//
// # The order inside one pass
//
// Per member, sorted: the beat, then the transfer that beat explains. Cause
// immediately before effect, which is the same law [Encounter.refreshSight]
// states for verbs, applied inside one.
//
// # Why the STORY is the ledger
//
// The consult runs at every sight refresh, so something has to decide whether a
// body is news. Nothing in this module can remember: doc.go's contract is that
// every caller loads, acts and saves, so an Encounter lives for one verb and an
// in-memory ledger would be empty on every call. That leaves persisted state,
// and a persisted down flag is exactly the dual state [Standing] exists to
// avoid.
//
// So the ledger is the STORY — the module's own persisted narration, and the
// same trick logFloor already plays (derived from the log rather than stored
// beside it, so it cannot drift out of agreement with the entries it
// describes). "Has the story already said this?" is a question with one answer,
// and no second thing to keep true.
//
// The cost, stated rather than hidden: the story has a retention window, so a
// down beat can age out of it, and the next consult will say it again. That is
// accepted. The obvious fix is to remember, and remembering is the disease.
func (e *Encounter) noticeDown() (map[MemberID]bool, error) {
	down, err := e.standingNow()
	if err != nil {
		return nil, err
	}
	if len(down) == 0 {
		return down, nil
	}

	told, err := e.storyToldDown()
	if err != nil {
		return nil, err
	}

	fallen := make([]MemberID, 0, len(down))
	for id := range down {
		fallen = append(fallen, id)
	}
	sort.Slice(fallen, func(i, j int) bool { return fallen[i] < fallen[j] })

	for _, id := range fallen {
		if !told[id] {
			if berr := e.appendDownBeat(id); berr != nil {
				return nil, berr
			}
		}

		// Out of the fight. A body keeps no turn, and the order closes over
		// the gap rather than holding it open — Transfer is the mid-round
		// removal a straggler leaving already used, and it prunes the bubble
		// if it empties.
		//
		// The world clock, not nowhere: R6 says every member is on exactly one
		// clock, and ruled fork (a) on rpg-toolkit#959 says a body is still a
		// member — on the map, in the roster, recordable, carried by Exit.
		bubble, berr := e.bubbleFor(id)
		if berr != nil {
			return nil, fmt.Errorf("standing bubble %q: %w", id, berr)
		}
		if bubble == nil {
			continue
		}
		if _, terr := e.Transfer(&TransferInput{Member: id, To: ClockWorld}); terr != nil {
			return nil, fmt.Errorf("standing transfer %q: %w", id, terr)
		}
	}

	return down, nil
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
			Beat   string `json:"beat"`
			Member string `json:"member"`
		}
		// A payload this pass cannot read is not this pass's business. Every
		// beat this module writes is a JSON object and decodes here (unknown
		// keys are ignored), so the skip is for a shape that could only come
		// from a hand-edited blob — and refusing every verb over one is worse
		// than not counting it as a death already told.
		if json.Unmarshal(entry.Payload, &beat) != nil {
			continue
		}
		if beat.Beat == string(OutcomeDown) && beat.Member != "" {
			told[MemberID(beat.Member)] = true
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

	memberIDs := make([]MemberID, 0, len(e.members))
	for mid := range e.members {
		memberIDs = append(memberIDs, mid)
	}
	sort.Slice(memberIDs, func(i, j int) bool { return memberIDs[i] < memberIDs[j] })

	if _, aerr := e.appendBeat(&record.AppendInput{
		At:       uint64(e.clock.ToData().HighWater),
		Audience: memberIDs,
		Tags:     map[string]string{"tag": "outcome"},
		Payload:  payload,
	}); aerr != nil {
		return fmt.Errorf("standing append beat: %w", aerr)
	}

	return nil
}
