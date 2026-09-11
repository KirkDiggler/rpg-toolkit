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
func (e *Encounter) appendSightedBeats(
	deltas map[MemberID]*IntelDelta, declared []MemberID, at uint64,
) error {
	if len(deltas) == 0 {
		return nil
	}

	declaredSet := make(map[intel.Subject]struct{}, len(declared))
	for _, id := range declared {
		declaredSet[intel.Subject(id)] = struct{}{}
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
		// THE DECLARED MEMBERS THIS OBSERVER CAN ACTUALLY SEE.
		//
		// A caller says WHICH members changed; who is owed telling is this
		// function's to work out, and the answer is per observer. Refreshed
		// is exactly "I hold this subject and just wrote to it", so
		// intersecting with it keeps the two people watching and drops the
		// three who cannot see a thing — including anybody holding the
		// subject only as a ghost, whose testimony is a memory of an older
		// moment and must not quietly acquire news they never witnessed.
		//
		// RE-ACQUISITIONS ARE EXCLUDED because gained already carries them.
		// A subject who came back into view AND changed while away is one
		// piece of news to a recipient — look again — and saying it twice in
		// one beat would have a client wonder which of the two it missed.
		changed := make([]string, 0, len(declared))
		if len(declaredSet) > 0 {
			reacquired := make(map[intel.Subject]struct{}, len(delta.Reacquired))
			for _, subject := range delta.Reacquired {
				reacquired[subject] = struct{}{}
			}
			for _, subject := range delta.Refreshed {
				if _, wanted := declaredSet[subject]; !wanted {
					continue
				}
				if _, already := reacquired[subject]; already {
					continue
				}
				changed = append(changed, string(subject))
			}
		}

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
		sort.Strings(changed)

		// NOTHING CHANGED, SO NOTHING IS SAID. This is the whole of the
		// noise control: a refresh in which everybody simply kept seeing
		// what they already saw appends no beats at all. A declared change
		// nobody could see is the same silence — the observers who cannot
		// see the subject are told nothing, which is the point of scoping
		// it per observer rather than broadcasting the fact.
		if len(gained) == 0 && len(lost) == 0 && len(changed) == 0 {
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
		if len(changed) > 0 {
			payload["changed"] = changed
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

// RecheckInput names the members whose observable facts changed outside this
// composition.
type RecheckInput struct {
	// Members is who changed. Not what about them changed — see Recheck.
	Members []MemberID
}

// RecheckOutput reports what the re-look produced.
type RecheckOutput struct {
	// IntelDeltas is each observer's intel movement from the refresh, the
	// same shape every other verb returns.
	IntelDeltas map[MemberID]*IntelDelta
	// Formed is a fight the refresh started, which a re-look is not expected
	// to produce — nobody moved — but which is reported rather than dropped
	// on the floor if one ever is.
	Formed *FormedBubble
}

// Recheck tells the composition that something an observer could SEE about
// these members has changed, and that everyone watching them should be told to
// look again.
//
// # Why a verb has to exist for this
//
// What a member is holding lives on a character sheet this module cannot read
// and does not own. It reaches the composition through the [Equipment]
// capability, and only ever at the moment sight refreshes — so a swap made
// while two people stand still watching each other changes nothing anybody can
// see until somebody happens to take a step. That is not a gap in the snapshot
// model; it is the snapshot model working, missing its trigger.
//
// EQUIPITEM STAYS THE SINGLE WRITER (Kirk's ruling, 2026-09-11: "I do not want
// two paths for 1 thing"). A session-side swap verb was considered — it is a
// real 5e object interaction and would get action economy right — and rejected
// because it would give equipment two ways in, one inside an encounter and one
// outside. The sheet is written once, in one place, and the composition is
// TOLD.
//
// # It names who, never what
//
// The caller says a member changed. It does not say a longsword was put away,
// and this verb would refuse to carry it if it did. Every watcher re-reads
// their OWN testimony, which is the only shape in which one of them can be
// wrong: a fact broadcast to the table is true for everybody by construction,
// and a game whose engine cannot lie can never have an illusion in it.
//
// # It is not a broadcast
//
// Who hears about it is worked out per observer from what they can actually
// see this pass (see appendSightedBeats). A member across the map, or one
// holding the subject only as a ghost, is told nothing — the ghost keeps the
// moment it recorded, which is the whole reason a ghost exists.
//
// Refuses a closed encounter, an empty list, and any member it does not have.
func (e *Encounter) Recheck(in *RecheckInput) (*RecheckOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("recheck: %w", ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("recheck: %w", ErrClosed)
	}
	if len(in.Members) == 0 {
		return nil, fmt.Errorf("recheck: %w", ErrNoMember)
	}
	// EVERY MEMBER CHECKED BEFORE ANY WORK, so a caller naming one stranger
	// beside four members does not get a half-done re-look and an error
	// (R5 atomicity, the same order every other verb validates in).
	for _, id := range in.Members {
		if _, ok := e.members[id]; !ok {
			return nil, fmt.Errorf("recheck %q: %w", id, ErrNotMember)
		}
	}

	// THE WHOLE ROSTER RE-LOOKS, not only the changed members' watchers.
	// Rebuilding one observer's percept and not another's would leave the two
	// of them reading the same world at two different moments, which is the
	// dual state the capabilities exist to avoid. The scoping that matters —
	// who is TOLD — happens per observer when the beat is built.
	deltas, formed, err := e.refreshSightDeclaring(e.rosterIDs(), in.Members)
	if err != nil {
		return nil, fmt.Errorf("recheck: %w", err)
	}

	return &RecheckOutput{IntelDeltas: deltas, Formed: formed}, nil
}
