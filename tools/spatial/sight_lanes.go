package spatial

import (
	"errors"
	"math"
)

var (
	// ErrNoSightGrid indicates that SightLanes was called without a grid.
	ErrNoSightGrid = errors.New("spatial: sight lanes require a grid")
	// ErrNoSightObstructions indicates that SightLanes was called without obstruction reads.
	ErrNoSightObstructions = errors.New("spatial: sight lanes require obstruction reads")
	// ErrBadSightPosition indicates that a sight endpoint contains a non-finite coordinate.
	ErrBadSightPosition = errors.New("spatial: sight endpoints must be finite")
)

// SightLaneInput describes one lane whose obstruction facts should be read.
// Ray is the canonical grid ray from From toward To. Implementations must treat
// it as read-only and must not retain it after Along returns.
type SightLaneInput struct {
	From Position
	To   Position
	Ray  []Position
}

// SightLaneOutput reports the obstruction facts for one lane. A hard block
// cannot be bypassed by alternate lanes; a soft block can be bypassed, though
// either kind blocks the lane on which it is reported.
type SightLaneOutput struct {
	HardBlocked bool
	SoftBlocked bool
}

// SightCellInput identifies an alternate origin whose opacity should be read.
type SightCellInput struct {
	At Position
}

// SightCellOutput reports whether a cell is opaque as an alternate origin.
type SightCellOutput struct {
	Blocked bool
}

// SightObstructions supplies obstruction facts for a SightLanes query.
// Along reads a canonical lane and At reads whether an alternate origin is
// opaque. Implementations should expose a stable view for the whole query.
type SightObstructions interface {
	Along(SightLaneInput) (SightLaneOutput, error)
	At(SightCellInput) (SightCellOutput, error)
}

// SightLanesInput contains the collaborators and endpoints for a sight query.
type SightLanesInput struct {
	Grid         Grid
	From         Position
	To           Position
	Obstructions SightObstructions
}

// SightLanesOutput reports whether every eligible lane is blocked.
type SightLanesOutput struct {
	Blocked bool
}

// SightLanes evaluates the direct grid lane and, for a soft direct obstruction,
// progress-making alternate lanes from both endpoints. A hard direct
// obstruction is absolute, and gridless queries evaluate only the direct lane.
// It returns the zero output with any validation or obstruction callback error.
func SightLanes(in SightLanesInput) (SightLanesOutput, error) {
	if in.Grid == nil {
		return SightLanesOutput{}, ErrNoSightGrid
	}
	if in.Obstructions == nil {
		return SightLanesOutput{}, ErrNoSightObstructions
	}
	for _, coordinate := range []float64{in.From.X, in.From.Y, in.To.X, in.To.Y} {
		if math.IsNaN(coordinate) || math.IsInf(coordinate, 0) {
			return SightLanesOutput{}, ErrBadSightPosition
		}
	}

	direct, err := in.readLane(in.From, in.To)
	if err != nil {
		return SightLanesOutput{}, err
	}
	if direct.HardBlocked {
		return SightLanesOutput{Blocked: true}, nil
	}
	if !direct.SoftBlocked {
		return SightLanesOutput{}, nil
	}
	if in.Grid.GetShape() == GridShapeGridless {
		return SightLanesOutput{Blocked: true}, nil
	}

	distance := in.Grid.Distance(in.From, in.To)
	for _, alternate := range in.Grid.GetNeighbors(in.From) {
		occupied, err := in.Obstructions.At(SightCellInput{At: alternate})
		if err != nil {
			return SightLanesOutput{}, err
		}
		if occupied.Blocked || in.Grid.Distance(alternate, in.To) >= distance {
			continue
		}
		lane, err := in.readLane(alternate, in.To)
		if err != nil {
			return SightLanesOutput{}, err
		}
		if !lane.HardBlocked && !lane.SoftBlocked {
			return SightLanesOutput{}, nil
		}
	}
	for _, alternate := range in.Grid.GetNeighbors(in.To) {
		occupied, err := in.Obstructions.At(SightCellInput{At: alternate})
		if err != nil {
			return SightLanesOutput{}, err
		}
		if occupied.Blocked || in.Grid.Distance(in.From, alternate) >= distance {
			continue
		}
		lane, err := in.readLane(in.From, alternate)
		if err != nil {
			return SightLanesOutput{}, err
		}
		if !lane.HardBlocked && !lane.SoftBlocked {
			return SightLanesOutput{}, nil
		}
	}

	return SightLanesOutput{Blocked: true}, nil
}

func (in SightLanesInput) readLane(from, to Position) (SightLaneOutput, error) {
	return in.Obstructions.Along(SightLaneInput{
		From: from,
		To:   to,
		Ray:  CanonicalBoundaryRay(in.Grid, from, to),
	})
}
