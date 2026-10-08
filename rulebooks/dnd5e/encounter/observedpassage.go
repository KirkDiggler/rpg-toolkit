package encounter

import (
	"fmt"

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

// A recorded interaction can change standing without moving anyone. Refresh
// through perception only when a current witness has an outdated observation;
// ghosts retain their last testimony. Reads never perform this update.
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
		}
	}
	if len(changed) == 0 {
		return nil, nil
	}
	subjects := make([]MemberID, 0, len(changed))
	for _, id := range e.rosterIDs() {
		if changed[id] {
			subjects = append(subjects, id)
		}
	}
	deltas, err := e.rebuildPercepts(e.rosterIDs())
	if err != nil {
		return nil, err
	}
	if err = e.appendSightedBeats(deltas, subjects, uint64(e.clock.ToData().HighWater)); err != nil {
		return nil, err
	}
	return deltas, nil
}
