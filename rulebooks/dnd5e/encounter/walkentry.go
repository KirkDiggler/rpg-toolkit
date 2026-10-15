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

// AdmitWalk joins a free-roaming member to the running fight when they are
// in CONTACT with it: an opposed member of that fight currently sees them.
// It never changes placement. Call before announcing or charging a step.
// Existing combat members are unchanged.
//
// # Contact is a hostile pair, never a teammate (the mad king D0 walk)
//
// This used to admit a walker who could SEE AN ALLY in the fight — the bard
// who stepped into view of the fighter was pulled into the fighter's bubble
// with no enemy anywhere in the bard's sight or the enemy's (rpg-project#569,
// found on the castle walk). The operator's ruling: a seat joins a fight only
// when IT is in contact — it sees a hostile, or a hostile sees it. Sight of
// an ally or a neutral forms and joins nothing.
//
// ONLY THE BEING-SEEN SIDE ADMITS HERE. A walker who sees an opposed member
// of the fight unseen has the drop (rpg-toolkit#1020) and keeps it: the
// choice to approach, sneak or back away is theirs, so their own sight is
// not admission. A walker an enemy in the fight is watching is in that fight
// — the same answer classify gives a spotted contact, read from current
// sight because the contact that made it may have been classified before
// the fight ran (a straggler the fight's own refresh missed).
//
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
	participation, err := e.participationNow()
	if err != nil {
		return nil, fmt.Errorf("admit walk: %w", err)
	}
	if !participation.contact[member.ID] {
		return out, nil
	}
	order, err := e.bubbles[0].Order()
	if err != nil {
		return nil, fmt.Errorf("admit walk: %w", err)
	}
	for _, id := range order {
		other, exists := e.members[MemberID(id)]
		if !exists || !participation.contact[other.ID] || !e.opposed(member.ID, other.ID) {
			continue
		}
		watching, err := e.currentlySees(other.ID, member.ID)
		if err != nil {
			return nil, fmt.Errorf("admit walk sight: %w", err)
		}
		if !watching {
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

// currentlySees reports whether observer's view holds subject on sight now —
// watched, not remembered.
func (e *Encounter) currentlySees(observer, subject MemberID) (bool, error) {
	holdings, err := e.memberIntel(observer)
	if err != nil {
		return false, err
	}
	for _, holding := range holdings {
		if holding.Subject == subject && holding.CurrentOn(perception.Sight) {
			return true, nil
		}
	}
	return false, nil
}
