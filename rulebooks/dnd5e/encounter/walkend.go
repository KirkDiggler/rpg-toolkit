package encounter

import (
	"errors"
	"fmt"
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
