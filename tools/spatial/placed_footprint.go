package spatial

import (
	"errors"
	"math"
)

// FootprintPlacement positions a footprint in the caller's continuous plane.
// LocalOffset is in the footprint's own along/across axes, before Facing.
type FootprintPlacement struct {
	Footprint   Footprint
	Origin      Point
	Facing      float64
	LocalOffset Point
}

// ErrBadFootprintPlacement reports non-finite or unrepresentable placed geometry.
var ErrBadFootprintPlacement = errors.New("spatial: invalid footprint placement")

type placedBox struct {
	centre    Point
	along     Point
	across    Point
	halfDepth float64
	halfWidth float64
	corners   [4]Point
}

func footprintBox(in FootprintPlacement) (placedBox, error) {
	if in.Footprint.Box == nil {
		return placedBox{}, ErrNoFootprint
	}
	b := *in.Footprint.Box
	for _, v := range []float64{b.W, b.D} {
		if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return placedBox{}, ErrBadFootprint
		}
	}
	for _, v := range []float64{in.Origin.X, in.Origin.Y, in.Facing, in.LocalOffset.X, in.LocalOffset.Y} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return placedBox{}, ErrBadFootprintPlacement
		}
	}

	a := math.Mod(in.Facing, 360) * math.Pi / 180
	f := placedBox{
		along:     Point{X: math.Cos(a), Y: math.Sin(a)},
		across:    Point{X: -math.Sin(a), Y: math.Cos(a)},
		halfDepth: b.D / 2,
		halfWidth: b.W / 2,
	}
	f.centre = Point{
		X: in.Origin.X + f.along.X*in.LocalOffset.X + f.across.X*in.LocalOffset.Y,
		Y: in.Origin.Y + f.along.Y*in.LocalOffset.X + f.across.Y*in.LocalOffset.Y,
	}
	local := [4]Point{
		{X: -f.halfDepth, Y: -f.halfWidth},
		{X: f.halfDepth, Y: -f.halfWidth},
		{X: f.halfDepth, Y: f.halfWidth},
		{X: -f.halfDepth, Y: f.halfWidth},
	}
	for i, p := range local {
		f.corners[i] = Point{
			X: f.centre.X + f.along.X*p.X + f.across.X*p.Y,
			Y: f.centre.Y + f.along.Y*p.X + f.across.Y*p.Y,
		}
		for _, v := range []float64{f.corners[i].X, f.corners[i].Y} {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return placedBox{}, ErrBadFootprintPlacement
			}
		}
	}
	area := polygonArea(f.corners[:])
	if area <= 0 || math.IsNaN(area) || math.IsInf(area, 0) {
		return placedBox{}, ErrBadFootprintPlacement
	}

	return f, nil
}
