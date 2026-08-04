package spatial

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

func positionLess(left, right Position) bool {
	if left.X != right.X {
		return left.X < right.X
	}
	return left.Y < right.Y
}
