package spatial

import "math"

// Boundary represents an undirected crossing between two adjacent grid
// positions. BasicRoom normalizes From and To when the boundary is registered,
// so the same boundary is found regardless of the caller's direction.
//
// Boundaries are runtime spatial state only. Higher-level domains own any
// authored or persisted representation and register their current crossings
// when reconstructing a room.
type Boundary struct {
	// From is one endpoint of the crossing. Registered boundaries expose the
	// lexicographically first endpoint here.
	From Position

	// To is the other endpoint of the crossing. Registered boundaries expose
	// the lexicographically second endpoint here.
	To Position

	// BlocksMovement reports whether an entity may cross this boundary.
	BlocksMovement bool

	// BlocksLineOfSight reports whether line of sight may cross this boundary.
	BlocksLineOfSight bool
}

type boundaryKey struct {
	first  Position
	second Position
}

func normalizedBoundary(boundary Boundary) Boundary {
	if positionLess(boundary.To, boundary.From) {
		boundary.From, boundary.To = boundary.To, boundary.From
	}
	return boundary
}

func newBoundaryKey(from, to Position) boundaryKey {
	boundary := normalizedBoundary(Boundary{From: from, To: to})
	return boundaryKey{first: boundary.From, second: boundary.To}
}

func isDiscretePosition(position Position) bool {
	return !math.IsNaN(position.X) && !math.IsNaN(position.Y) &&
		!math.IsInf(position.X, 0) && !math.IsInf(position.Y, 0) &&
		math.Trunc(position.X) == position.X && math.Trunc(position.Y) == position.Y
}

func positionLess(left, right Position) bool {
	if left.X != right.X {
		return left.X < right.X
	}
	return left.Y < right.Y
}

// CanonicalBoundaryRay returns the one deterministic grid ray for an
// unordered endpoint pair, oriented from from toward to. It derives the ray
// once from lexicographically ordered endpoints, then reverses it when the
// caller requested the opposite direction. Boundary traversal therefore
// crosses identical physical edges in both directions while callers retain
// their requested path direction.
func CanonicalBoundaryRay(grid Grid, from, to Position) []Position {
	if !positionLess(to, from) {
		return grid.GetLineOfSight(from, to)
	}

	path := grid.GetLineOfSight(to, from)
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}
	return path
}
