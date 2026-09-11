// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// sightedbeat.go is the per-recipient perception beat: one observer being
// told that WHO THEY CAN SEE changed.
//
// # Why this beat exists at all
//
// A client's picture of a peer is its observer's own testimony, snapshotted
// when they looked ([SightTestimony]). Nothing on the wire told that client
// when to re-read it. It worked anyway, by accident: the moved beat goes to
// the whole roster today, so every member re-reads the scene whenever anybody
// walks, and a subject stepping back into view happens to arrive on the same
// frame as their own movement.
//
// That accident is load-bearing and it is temporary. The moment audienceFor
// starts narrowing — which is the entire point of the shelf it already is —
// a subject walking back into your view does so on a beat you no longer
// receive, and your picture of them stays whatever it was when they left.
// This beat is what makes that narrowing safe: the observer is told about
// their OWN perception changing, which is news they are always entitled to,
// whatever they may not be told about the world.
//
// # A transition, never a state
//
// [intel.SurveilOutput.Refreshed] fires on every pass for every perceived
// subject. A beat on that would be a beat per observer per visible member
// per refresh — every step, every monster action — and a client would learn
// nothing from one arriving. So this beat is built from the two TRANSITIONS
// instead, and an observer whose perception did not change is sent nothing
// at all. Silence means "no change", which is the only reading that makes
// the beat worth receiving.
//
// # It names subjects and carries no testimony
//
// Deliberately (rpg-project ideas/perception-stream, "filtering is
// deliberately deferred"). The recipient re-reads their own view, which is
// already member-scoped and already the one computation of what they
// perceive. A beat that carried position and hands would be a SECOND
// computation of that answer, which is exactly how the reveal beats'
// own doc says a patch and a projection learn to disagree — and the client
// re-fetches on the reveal beats for that reason too (rpg-project#886).

// BeatSighted is the "beat" value of the story beat this composition appends
// when one observer's perception of the roster changes.
//
// EXPORTED BECAUSE A DECODER READS IT, for the same reason
// [BeatWindowOpened] is: it arrives with a session-side decoder being
// written against it in the same wave, so a rename fails to compile there
// instead of quietly producing a beat nobody renders.
const BeatSighted = "sighted"

// appendSightedBeats tells each observer whose perception changed this pass
// who entered and who left it. One beat per such observer, audience of
// exactly that observer; observers whose perception did not change are sent
// nothing.
//
// CALL THIS WITH THE DELTAS [Encounter.rebuildPercepts] JUST RETURNED, and
// before anything those deltas go on to cause. A fight forms BECAUSE somebody
// saw somebody, so the seeing has to be readable before the fight —
// [Encounter.refreshSight]'s law, applied to the beat that records the cause.
// The nested refreshes inside classification (noticeDown, Transfer) reach
// this through refreshSight themselves and append their own, which is why
// this is not also called on applyTrigger's merged output: that would report
// the same transitions twice.
//
// GAINED MERGES FIRST CONTACT WITH RE-ACQUISITION. To a recipient the two
// are one fact — somebody is in view who was not — and the client's answer
// to both is the same re-read. The distinction survives in [IntelDelta] for
// a consumer that ever needs it; nothing needs it today, and a use case
// brings the mechanism.
func (e *Encounter) appendSightedBeats(deltas map[MemberID]*IntelDelta, at uint64) error {
	if len(deltas) == 0 {
		return nil
	}

	// Sorted, because deltas is a map and a story is a transcript. Two runs
	// of one scene that differ only in beat order are two different stories
	// to anything comparing them.
	observers := make([]MemberID, 0, len(deltas))
	for observer := range deltas {
		observers = append(observers, observer)
	}
	sort.Slice(observers, func(i, j int) bool { return observers[i] < observers[j] })

	for _, observer := range observers {
		delta := deltas[observer]
		if delta == nil {
			continue
		}

		gained := make([]string, 0, len(delta.FirstContact)+len(delta.Reacquired))
		for _, report := range delta.FirstContact {
			gained = append(gained, string(report.Subject))
		}
		gained = appendSubjectStrings(gained, delta.Reacquired)
		lost := appendSubjectStrings(make([]string, 0, len(delta.Faded)), delta.Faded)

		// SORTED HERE TOO, and not only where the percept is built. This
		// beat's own guarantee is that two runs of one scene produce one
		// transcript, and a guarantee that holds only because a function
		// three calls away happens to sort is not one this file can make.
		// gained also concatenates two lists whose own orders mean nothing
		// to each other — first contacts then re-acquisitions — so even a
		// perfectly ordered percept would leave it grouped by a distinction
		// the beat deliberately does not draw.
		sort.Strings(gained)
		sort.Strings(lost)

		// NOTHING CHANGED, SO NOTHING IS SAID. This is the whole of the
		// noise control: a refresh in which everybody simply kept seeing
		// what they already saw appends no beats at all.
		if len(gained) == 0 && len(lost) == 0 {
			continue
		}

		payload := map[string]interface{}{
			"beat": BeatSighted,
		}
		// Omitted rather than empty, both of them. An absent key is "this
		// did not happen"; an empty list invites a reader to believe the
		// composition looked and found none, which is a different claim and
		// one this beat never makes — it is only ever appended because
		// something DID happen.
		if len(gained) > 0 {
			payload["gained"] = gained
		}
		if len(lost) > 0 {
			payload["lost"] = lost
		}

		beatBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal sighted beat: %w", err)
		}

		if _, err := e.appendBeat(&record.AppendInput{
			Audience: []MemberID{observer},
			Tags:     map[string]string{"tag": "sight"},
			Payload:  beatBytes,
			At:       at,
		}); err != nil {
			return fmt.Errorf("append sighted beat: %w", err)
		}
	}

	return nil
}

// appendSubjectStrings appends subjects to out as strings. The caller sorts
// what it builds — see the sort in appendSightedBeats for why the order these
// arrive in cannot be trusted.
func appendSubjectStrings(out []string, subjects []intel.Subject) []string {
	for _, subject := range subjects {
		out = append(out, string(subject))
	}
	return out
}
