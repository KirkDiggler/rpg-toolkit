package encounter

import (
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// ValidateWalkEnd refuses a known occupied destination before a voluntary walk
// starts. Unknown occupants are not queried; Step rechecks truth on arrival.
// Errors preserve missing observation contracts rather than treating them as fog.
func (e *Encounter) ValidateWalkEnd(in CellAtInput) error {
	passages, err := e.ObservedPassages(&ViewInput{Member: in.Mover})
	if err != nil {
		return err
	}
	holdings, err := e.View(&ViewInput{Member: in.Mover})
	if err != nil {
		return err
	}
	for _, holding := range holdings {
		passage, current := passages[holding.Subject]
		if !current || passage == PassageStandable {
			continue
		}
		seen, ok := DecodeSightTestimony(holding.Payload)
		if ok && seen.State == LocationKnown && seen.Position == in.Cell {
			return fmt.Errorf("walk destination is not standable: %w", ErrBadPlacement)
		}
	}
	return nil
}

// StepObstructedError is an ordinary obstacle encountered by a valid step.
// Callers may commit the completed prefix. Capability and malformed placement
// errors never use this marker. Unwrap preserves the original refusal.
type StepObstructedError struct{ cause error }

// PublicReason omits identities that the mover may not have observed.
func (e *StepObstructedError) PublicReason() string {
	if errors.Is(e.cause, ErrBadPlacement) {
		return "movement stopped at an obstruction"
	}
	return e.cause.Error()
}
func (e *StepObstructedError) Error() string { return e.cause.Error() }
func (e *StepObstructedError) Unwrap() error { return e.cause }

// standableEnd is the walk's own end-of-walk law, the twin of the EndWalk
// refusal a player's last [Encounter.Step] meets: a walk may cross a cell it
// cannot stop on, but it ends on one it can.
//
// WHY THE WALK AND NOT THE ROUTE. Every route here already ends on a
// standable cell ([cellQuery.nearestStop]: "may cross an ally, may not stop
// on one"), and then three callers cut the path to what the turn's movement
// pays for — the table driver's toward, the authored-cell Routed, and any
// driver's own [Move]. A cut can land on an ally mid-path, and [Encounter.stepTo]
// has never asked whether a cell may be STOPPED on, only whether it may be
// entered. So a monster finished its turn standing on its friend. The route
// cannot know the budget it will be cut to, and the drivers cannot read
// passage, so the one place that knows both the path and the fold is the
// walk that takes it.
//
// TWO CUTS, IN ORDER. First the path is walked forward through the same
// fold the step reads ([cellQuery.stepInto]) and cut at the first cell the
// step would refuse — a closed crossing or a Blocked cell — because a walk
// stops THERE whatever the path said next, and a last standable cell beyond
// that point is one the walker will never reach. A driver's own Move is
// checked only for length, so this is the cut that keeps an unreachable
// tail from standing in for a destination. Then the prefix that remains is
// cut back to its LAST standable cell, which keeps the walk through an ally
// to open floor beyond and drops only the tail nobody may finish on.
//
// Empty when no cell of it is standable: the creature stays where it is,
// exactly as a wall across the first step would leave it. Read live, against
// the fold, because that is the truth the step itself will meet; an unplaced
// walker has nowhere to step from and gets the empty path.
func (e *Encounter) standableEnd(mover MemberID, path []spatial.Position) ([]spatial.Position, error) {
	prev, placed := e.canvas.GetEntityPosition(string(mover))
	if !placed {
		return nil, nil
	}
	query, err := e.cellQuery()
	if err != nil {
		return nil, fmt.Errorf("walk end: %w", err)
	}
	end := -1
	for i, cell := range path {
		fact, crossed, err := query.stepInto(mover, prev, cell)
		if err != nil {
			return nil, fmt.Errorf("walk end at %v: %w", cell, err)
		}
		if !crossed || fact.Passage == PassageBlocked {
			break
		}
		if fact.Passage == PassageStandable {
			end = i
		}
		prev = cell
	}
	if end < 0 {
		return nil, nil
	}
	return path[:end+1], nil
}
