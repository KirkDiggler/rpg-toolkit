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
		stance, _ := e.BelievedStance(in.Member, holding.Subject)
		out[holding.Subject] = occupantPassage(occupantFacts{
			kind: member.Kind, down: *seen.Down, blocks: *seen.BlocksMovement, hostile: stance == StanceHostile,
		})
	}
	return out, nil
}
