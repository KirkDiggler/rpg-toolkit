package encounter

import "fmt"

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
