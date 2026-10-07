package encounter

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
)

// AdmitWalkInput identifies the member about to take a voluntary step.
type AdmitWalkInput struct {
	Member MemberID
}

// AdmitWalkOutput reports a combat entry without a spatial step. Joined ends
// the free-roam walk; its unentered remainder has consumed no movement.
type AdmitWalkOutput struct {
	Joined      bool
	Seq         uint64
	IntelDeltas map[MemberID]*IntelDelta
}

// AdmitWalk joins a free-roaming member to combat when they currently see a
// teammate in the running bubble. It never changes placement. Call before
// announcing or charging a step. Existing combat members are unchanged.
// Errors preserve input, perception, participation, and transfer failures.
func (e *Encounter) AdmitWalk(in *AdmitWalkInput) (*AdmitWalkOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if e.outcome != nil {
		return nil, ErrClosed
	}
	member, ok := e.members[in.Member]
	if !ok {
		return nil, ErrNotMember
	}
	bubble, err := e.bubbleFor(in.Member)
	if err != nil {
		return nil, err
	}
	out := &AdmitWalkOutput{}
	if bubble != nil || len(e.bubbles) == 0 {
		return out, nil
	}
	holdings, err := e.memberIntel(in.Member)
	if err != nil {
		return nil, fmt.Errorf("admit walk sight: %w", err)
	}
	for _, holding := range holdings {
		if !holding.CurrentOn(perception.Sight) {
			continue
		}
		other, exists := e.members[holding.Subject]
		if !exists || other.ID == member.ID {
			continue
		}
		// The owner answers: StanceBetween holds the no-faction rule, and a
		// faction is allied with itself, so one read covers teammates and
		// allied sides alike.
		stance, err := e.StanceBetween(member.ID, other.ID)
		if err != nil {
			return nil, fmt.Errorf("admit walk sight: %w", err)
		}
		if stance != StanceAllied {
			continue
		}
		teammateBubble, err := e.bubbleFor(other.ID)
		if err != nil {
			return nil, err
		}
		if teammateBubble == nil {
			continue
		}
		transferred, err := e.Transfer(&TransferInput{Member: in.Member, To: ClockTurn, Pos: e.bubbleSize()})
		if err != nil {
			return nil, fmt.Errorf("admit walk transfer: %w", err)
		}
		out.Joined, out.Seq, out.IntelDeltas = true, transferred.Seq, transferred.IntelDeltas
		return out, nil
	}
	return out, nil
}
