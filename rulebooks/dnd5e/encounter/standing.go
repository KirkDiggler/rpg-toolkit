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
// It DOES end a fight, as of rpg-toolkit#1078: a bubble left with nobody
// standing on one side of it dissolves itself with cause [ByDefeat], in the pass
// that notices. Ruled fork (c) on rpg-toolkit#959, and only the bubble ends —
// the encounter stays open and the bodies stay in it.
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
// — the choke point sight already flows through, so Step, Pump, Join
// and Exit all pass here by writing the obvious call — and from Setup's first
// light, which calls applyTrigger directly so its scene-opened beat can land
// first. A scene can open with a body already on the floor.
//
// [Encounter.Record] reaches this function DIRECTLY rather than through
// applyTrigger, and the difference is the point (rpg-toolkit#1083). Record
// refreshes no sight, so it has no deltas to classify and no fight to start —
// what it has is a beat that may have just changed the answer this function
// pulls. Every other caller looks at a world something else changed; that one is
// the change. Calling applyTrigger from it would mean synthesising an empty
// delta map to satisfy a classification nobody asked for.
//
// # The order inside one pass
//
// Two passes over the fallen, sorted: ALL the news, then everything the news
// does. Cause before effect, which is the same law [Encounter.refreshSight]
// states for verbs, applied inside one.
//
// It is two passes rather than one interleaved loop because of what the second
// pass can do. A bubble that has run out of a side ends (see
// [Encounter.fightIsDecided]), and the beat saying so has to land after EVERY
// down beat that explains it — including the ones for members this loop has not
// reached yet. Interleaved, two monsters falling together would narrate the
// first body, then the fight ending, then the second body: an ending that
// arrives before half its own cause.
//
// # What the news does, per member
//
// A fight with somebody standing on both sides of it survives, and the body is
// spliced out of the order — Transfer is the mid-round removal a straggler
// leaving already used. A fight with nobody standing on one side ENDS instead,
// and ending it re-homes everyone the fight held, bodies included, so no splice
// is needed or wanted.
//
// The ending has to be decided BEFORE the splice rather than after it, which is
// the one place this deviates from how rpg-toolkit#1078 describes itself. If
// every member of a bubble is down, splicing first drains the bubble and
// dropBubbleIfIdle deletes it from inside Transfer — leaving a check that runs
// afterwards with no bubble to end and nobody to name in the beat. That husk
// prune is exactly the accidental ending D2 replaces, so it must not be reached.
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

	// The news, all of it, before anything acts on any of it.
	for _, id := range fallen {
		if told[id] {
			continue
		}
		if berr := e.appendDownBeat(id); berr != nil {
			return nil, berr
		}
	}

	// Then what the news does.
	for _, id := range fallen {
		bubble, berr := e.bubbleFor(id)
		if berr != nil {
			return nil, fmt.Errorf("standing bubble %q: %w", id, berr)
		}
		if bubble == nil {
			continue
		}

		// The fight this body was in may not be a fight any more. Ending it
		// re-homes everybody it held — the survivors AND the bodies — so the
		// splice below is not reached for anyone in it, and the members who
		// fell alongside this one find no bubble to be spliced out of.
		decided, derr := e.fightIsDecided(bubble, down)
		if derr != nil {
			return nil, fmt.Errorf("standing fight %q: %w", id, derr)
		}
		if decided {
			if _, xerr := e.dissolveBubble(bubble, ByDefeat()); xerr != nil {
				return nil, fmt.Errorf("standing dissolve %q: %w", id, xerr)
			}
			continue
		}

		// Out of the fight, which goes on without them. A body keeps no turn,
		// and the order closes over the gap rather than holding it open —
		// Transfer is the mid-round removal a straggler leaving already used.
		//
		// The world clock, not nowhere: R6 says every member is on exactly one
		// clock, and ruled fork (a) on rpg-toolkit#959 says a body is still a
		// member — on the map, in the roster, recordable, carried by Exit.
		if _, terr := e.Transfer(&TransferInput{Member: id, To: ClockWorld}); terr != nil {
			return nil, fmt.Errorf("standing transfer %q: %w", id, terr)
		}
	}

	// Then whether the world just ended. AFTER the bubble logic above, so a
	// boss death dissolves its fight first and the run closes on the world
	// clock — down → bubble-dissolved → ended is the beat order a client
	// reads (Kirk's ruling, rpg-project#269 §6.6).
	//
	// Evaluated against EVERYONE down, not only the newly narrated: the fire
	// is guarded by the outcome, not by the story ledger, so a down beat
	// aging out of the retention window and being re-said cannot re-close
	// anything, and an ending loaded over a member already down still fires
	// at the next consult.
	if e.outcome == nil {
		if err := e.firedMemberDown(down); err != nil {
			return nil, err
		}
	}

	return down, nil
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

// fightIsDecided reports whether this bubble has run out of a side: whether the
// members still standing in it are all players, or all monsters, or nobody at
// all.
//
// # Why it is asked about a bubble a body is IN, never about the bubble list
//
// A one-sided bubble is not the same thing as a decided fight. A caller can
// Transfer the last monster out of a fight with nobody down anywhere, and
// calling that a defeat would write an ending into the story that nothing in the
// fiction earned. So the question is reached through a member the world has just
// noticed is down — the fight went one-sided BECAUSE somebody fell, which is the
// only reading [ByDefeat] is entitled to.
//
// A fight nobody is left fighting for other reasons is a different ruling with a
// different cause, and it is not this one.
//
// # The all-down case
//
// Both counts zero is decided, and deliberately so. That is the scene D1 ended
// by accident: every member down, the order drained one splice at a time, and
// dropBubbleIfIdle removing what was left. The fight ended correctly and
// silently, because the pruner knows nothing about death. It now ends the way
// every other defeat does, with the beat that says so.
func (e *Encounter) fightIsDecided(bubble *clock.Turn, down map[MemberID]bool) (bool, error) {
	order, err := bubble.Order()
	if err != nil {
		return false, err
	}

	var players, monsters int
	for _, id := range order {
		if down[id] {
			continue
		}
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
