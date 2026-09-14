package spatial

import (
	"errors"
	"math"
)

// FootprintTraceInput describes a closed planar segment and one placed footprint.
type FootprintTraceInput struct {
	Placement FootprintPlacement
	From      Point
	To        Point
}

// FootprintTraceOutput distinguishes closed contact from positive-length interior.
// Enter and Leave parameterize the contact interval; a miss is the zero value.
type FootprintTraceOutput struct {
	Contact  bool
	Interior bool
	Enter    float64
	Leave    float64
}

// ErrBadFootprintTrace reports non-finite endpoints or unrepresentable arithmetic.
var ErrBadFootprintTrace = errors.New("spatial: invalid footprint trace")

// TraceFootprint reports contact with the full rectangle, independent of any grid.
// It does not decide whether contact blocks movement, sight, or an attack.
func TraceFootprint(in FootprintTraceInput) (FootprintTraceOutput, error) {
	f, err := footprintBox(in.Placement)
	if err != nil {
		return FootprintTraceOutput{}, err
	}
	for _, v := range []float64{in.From.X, in.From.Y, in.To.X, in.To.Y} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return FootprintTraceOutput{}, ErrBadFootprintTrace
		}
	}

	var ends [2]Point
	for i, p := range [2]Point{in.From, in.To} {
		dx, dy := p.X-f.centre.X, p.Y-f.centre.Y
		ends[i] = Point{
			X: dx*f.along.X + dy*f.along.Y,
			Y: dx*f.across.X + dy*f.across.Y,
		}
	}
	delta := Point{X: ends[1].X - ends[0].X, Y: ends[1].Y - ends[0].Y}
	for _, v := range []float64{ends[0].X, ends[0].Y, ends[1].X, ends[1].Y, delta.X, delta.Y} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return FootprintTraceOutput{}, ErrBadFootprintTrace
		}
	}
	if in.From == in.To {
		contact := math.Abs(ends[0].X) <= f.halfDepth && math.Abs(ends[0].Y) <= f.halfWidth
		return FootprintTraceOutput{Contact: contact}, nil
	}

	enter, leave := 0.0, 1.0
	axes := [2][3]float64{
		{ends[0].X, delta.X, f.halfDepth},
		{ends[0].Y, delta.Y, f.halfWidth},
	}
	for _, axis := range axes {
		start, change, half := axis[0], axis[1], axis[2]
		if change == 0 {
			if start < -half || start > half {
				return FootprintTraceOutput{}, nil
			}
			continue
		}
		a, b := (-half-start)/change, (half-start)/change
		if math.IsNaN(a) || math.IsNaN(b) || math.IsInf(a, 0) || math.IsInf(b, 0) {
			return FootprintTraceOutput{}, ErrBadFootprintTrace
		}
		if a > b {
			a, b = b, a
		}
		enter, leave = math.Max(enter, a), math.Min(leave, b)
		if enter > leave {
			return FootprintTraceOutput{}, nil
		}
	}

	mid := (enter + leave) / 2
	x, y := ends[0].X+mid*delta.X, ends[0].Y+mid*delta.Y
	interior := enter < leave && math.Abs(x) < f.halfDepth && math.Abs(y) < f.halfWidth

	return FootprintTraceOutput{Contact: true, Interior: interior, Enter: enter, Leave: leave}, nil
}
