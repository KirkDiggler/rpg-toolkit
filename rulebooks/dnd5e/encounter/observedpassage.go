package encounter

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// ObservedPassages answers occupancy contributions from the viewer's current
// sight testimony and believed relations. Memories and unknown locations are
// absent. Missing observation facts return an error, never an invented policy.
// It does not consult live participation or unseen occupants.
func (e *Encounter) ObservedPassages(in *ViewInput) (map[MemberID]Passage, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	holdings, err := e.View(in)
	if err != nil {
		return nil, err
	}
	out := make(map[MemberID]Passage)
	for _, holding := range holdings {
		if !holding.CurrentOn(perception.Sight) || holding.Subject == in.Member {
			continue
		}
		seen, ok := DecodeSightTestimony(holding.Payload)
		if !ok {
			return nil, fmt.Errorf("passage for %q: invalid sight testimony", holding.Subject)
		}
		if seen.State == LocationUnknown {
			continue
		}
		member, exists := e.members[holding.Subject]
		if !exists || seen.Down == nil || seen.BlocksMovement == nil {
			return nil, fmt.Errorf("passage for %q: missing observed occupant facts", holding.Subject)
		}
		stance, err := e.BelievedStance(in.Member, holding.Subject)
		if err != nil {
			return nil, fmt.Errorf("passage for %q: %w", holding.Subject, err)
		}
		out[holding.Subject] = occupantPassage(occupantFacts{
			kind: member.Kind, down: *seen.Down, blocks: *seen.BlocksMovement, hostile: stance == StanceHostile,
		})
	}
	return out, nil
}

// A recorded interaction can change standing without moving anyone, and an
// attack turns its attacker toward its target without moving them either
// (rpg-project#582, design R13). Refresh through perception only when a
// current witness has an outdated observation of either; ghosts retain their
// last testimony. Reads never perform this update.
//
// A sighting that carries no facing at all was written before sightings did,
// and is not called outdated for it: the next refresh that runs for any other
// reason replaces it, and a train should not tell every watcher to look again
// at everybody because a blob is old.
func (e *Encounter) refreshChangedStanding(state *participationState) (map[MemberID]*IntelDelta, error) {
	if e.outcome != nil {
		return nil, nil
	}
	changed := map[MemberID]bool{}
	for _, observer := range e.rosterIDs() {
		held, err := e.memberIntel(observer)
		if err != nil {
			return nil, err
		}
		for _, holding := range held {
			if !holding.CurrentOn(perception.Sight) {
				continue
			}
			seen, ok := DecodeSightTestimony(holding.Payload)
			if !ok || seen.State != LocationKnown {
				continue
			}
			if seen.Down == nil || *seen.Down != state.down[holding.Subject] {
				changed[holding.Subject] = true
			}
			if subject, ok := e.members[holding.Subject]; ok && seen.Facing != nil && *seen.Facing != subject.Facing {
				changed[holding.Subject] = true
			}
		}
	}
	if len(changed) == 0 {
		return nil, nil
	}
	deltas, err := e.rebuildPercepts(e.rosterIDs())
	if err != nil {
		return nil, err
	}
	// ANNOUNCE WHAT THE REBUILD CHANGED, not what the pre-check flagged. The
	// check above only decides whether to rebuild; the rebuild rewrites EVERY
	// observer's testimony of every subject, so a target whose conditions
	// changed in the same train as its attacker turned is rewritten too, and
	// a beat naming only the attacker would leave the watcher's client
	// holding the old sighting of the target — a host's own freshness pass
	// skips a member a sight refresh already wrote. The rebuild's own Changed
	// lists are the answer (rpg-toolkit#2073 review, found by F2).
	subjects := changedSubjects(deltas)
	if err = e.appendSightedBeats(deltas, subjects, uint64(e.clock.ToData().HighWater)); err != nil {
		return nil, err
	}
	return deltas, nil
}

// changedSubjects is every member whose testimony some observer's rebuild
// actually changed, from the deltas' own Changed lists: the subjects a
// sighted beat owes "look again" to.
func changedSubjects(deltas map[MemberID]*IntelDelta) []MemberID {
	seen := map[MemberID]bool{}
	var out []MemberID
	for _, delta := range deltas {
		if delta == nil {
			continue
		}
		for _, subject := range delta.Changed {
			if id := MemberID(subject); !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })

	return out
}
