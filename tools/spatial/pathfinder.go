package spatial

// PathFinder finds paths between hex positions avoiding obstacles.
// Implementations can use different algorithms (A*, Dijkstra, weighted, etc.)
type PathFinder interface {
	// FindPath returns a path from start to goal avoiding blocked hexes.
	// Returns the path excluding start, including goal.
	// Returns empty slice if no path exists or start == goal.
	FindPath(start, goal CubeCoordinate, blocked map[CubeCoordinate]bool) []CubeCoordinate
}

// TraversalPredicate decides whether A* may traverse one directed adjacent
// pair. It must return true to permit the step from to. A predicate can model
// blocked crossings such as closed boundaries without converting them into
// blocked cells.
type TraversalPredicate func(from, to CubeCoordinate) bool

// TraversalSearchLimit bounds predicate-aware searches on the otherwise
// unbounded hex plane. MaxSteps is the inclusive maximum number of crossings
// in a returned path. A non-positive limit permits no path unless start equals
// goal, and a limit shorter than the direct distance cannot reach the goal.
//
// The limit is also a deterministic safety contract: A* never adds a position
// whose path cost exceeds MaxSteps, so a predicate that seals an unblocked goal
// terminates after examining a finite search area. Callers should set MaxSteps
// above the direct distance by enough steps for the detours their map allows.
type TraversalSearchLimit struct {
	// MaxSteps is the inclusive path-length and search-cost bound.
	MaxSteps int
}

// TraversalPathFinder is an optional extension for path finders that can
// consult a traversal predicate in addition to legacy blocked cells.
type TraversalPathFinder interface {
	// FindPathWithTraversal returns a path while honoring blocked cells and a
	// traversal predicate. A nil predicate permits every adjacent crossing.
	// Search is bounded by limit; it returns an empty slice when no route fits
	// within the bound or when no route exists.
	FindPathWithTraversal(
		start, goal CubeCoordinate,
		blocked map[CubeCoordinate]bool,
		canTraverse TraversalPredicate,
		limit TraversalSearchLimit,
	) []CubeCoordinate
}

// SimplePathFinder uses A* algorithm with uniform movement cost.
// It finds the shortest path around obstacles using hex distance as heuristic.
type SimplePathFinder struct{}

var _ PathFinder = (*SimplePathFinder)(nil)
var _ TraversalPathFinder = (*SimplePathFinder)(nil)

// NewSimplePathFinder creates a new A* pathfinder
func NewSimplePathFinder() *SimplePathFinder {
	return &SimplePathFinder{}
}

// FindPath implements PathFinder using A* algorithm.
// Uses hex distance as heuristic (admissible - never overestimates).
func (p *SimplePathFinder) FindPath(start, goal CubeCoordinate, blocked map[CubeCoordinate]bool) []CubeCoordinate {
	return p.findPath(start, goal, blocked, nil, -1)
}

// FindPathWithTraversal uses A* while also requiring each adjacent step to be
// permitted by canTraverse. Traversal-aware searches require an explicit limit
// because a predicate can seal an otherwise-unblocked goal on the unbounded hex
// plane. It preserves legacy FindPath behavior by leaving that API unbounded.
func (p *SimplePathFinder) FindPathWithTraversal(
	start, goal CubeCoordinate,
	blocked map[CubeCoordinate]bool,
	canTraverse TraversalPredicate,
	limit TraversalSearchLimit,
) []CubeCoordinate {
	if limit.MaxSteps < 0 {
		return []CubeCoordinate{}
	}
	return p.findPath(start, goal, blocked, canTraverse, limit.MaxSteps)
}

func (p *SimplePathFinder) findPath(
	start, goal CubeCoordinate,
	blocked map[CubeCoordinate]bool,
	canTraverse TraversalPredicate,
	maxSteps int,
) []CubeCoordinate {
	if start == goal {
		return []CubeCoordinate{}
	}

	// If goal is blocked, no path exists.
	if blocked[goal] {
		return []CubeCoordinate{}
	}
	if maxSteps >= 0 && start.Distance(goal) > maxSteps {
		return []CubeCoordinate{}
	}

	// Priority queue entry
	type node struct {
		pos    CubeCoordinate
		fScore int // g + h
	}

	// Open set as a slice (simple priority queue)
	openSet := []node{{pos: start, fScore: start.Distance(goal)}}

	// Track where we came from for path reconstruction
	cameFrom := make(map[CubeCoordinate]CubeCoordinate)

	// g-score: cost from start to this node
	gScore := make(map[CubeCoordinate]int)
	gScore[start] = 0

	// Track what's in open set for O(1) lookup
	inOpenSet := make(map[CubeCoordinate]bool)
	inOpenSet[start] = true

	for len(openSet) > 0 {
		// Find node with lowest f-score (simple linear search)
		bestIdx := 0
		for i, n := range openSet {
			if n.fScore < openSet[bestIdx].fScore {
				bestIdx = i
			}
		}
		current := openSet[bestIdx]

		// Remove from open set
		openSet = append(openSet[:bestIdx], openSet[bestIdx+1:]...)
		delete(inOpenSet, current.pos)

		// Found goal - reconstruct path
		if current.pos == goal {
			return p.reconstructPath(cameFrom, current.pos)
		}

		// A bounded traversal search may reach a position at the limit, but
		// cannot expand it. This keeps the otherwise-unbounded search finite.
		if maxSteps >= 0 && gScore[current.pos] >= maxSteps {
			continue
		}

		// Check all neighbors
		for _, neighbor := range current.pos.GetNeighbors() {
			// Skip blocked hexes
			if blocked[neighbor] {
				continue
			}
			if canTraverse != nil && !canTraverse(current.pos, neighbor) {
				continue
			}

			// Calculate tentative g-score (uniform cost = 1 per hex)
			tentativeG := gScore[current.pos] + 1

			// Is this a better path to neighbor?
			existingG, seen := gScore[neighbor]
			if !seen || tentativeG < existingG {
				cameFrom[neighbor] = current.pos
				gScore[neighbor] = tentativeG
				fScore := tentativeG + neighbor.Distance(goal)

				if !inOpenSet[neighbor] {
					// New node: add to open set
					openSet = append(openSet, node{pos: neighbor, fScore: fScore})
					inOpenSet[neighbor] = true
				} else {
					// Existing node: update fScore for optimal exploration order
					for i, n := range openSet {
						if n.pos == neighbor {
							openSet[i].fScore = fScore
							break
						}
					}
				}
			}
		}
	}

	// No path found
	return []CubeCoordinate{}
}

// reconstructPath builds the path from start to goal using cameFrom map.
// Uses O(n) algorithm: build reversed path, then reverse once.
func (p *SimplePathFinder) reconstructPath(
	cameFrom map[CubeCoordinate]CubeCoordinate,
	current CubeCoordinate,
) []CubeCoordinate {
	// Build path in reverse (from goal back to start) in O(n)
	reversed := []CubeCoordinate{current}
	for {
		prev, ok := cameFrom[current]
		if !ok {
			break
		}
		reversed = append(reversed, prev)
		current = prev
	}

	// reversed now contains [goal, ..., start]
	if len(reversed) == 0 {
		return reversed
	}

	// Remove start from path (path should exclude start)
	reversed = reversed[:len(reversed)-1]

	// Reverse to get path from first step after start to goal
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}

	return reversed
}
