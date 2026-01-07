package spatial

// PathFinder finds paths between hex positions avoiding obstacles.
// Implementations can use different algorithms (A*, Dijkstra, weighted, etc.)
type PathFinder interface {
	// FindPath returns a path from start to goal avoiding blocked hexes.
	// Returns the path excluding start, including goal.
	// Returns empty slice if no path exists or start == goal.
	FindPath(start, goal CubeCoordinate, blocked map[CubeCoordinate]bool) []CubeCoordinate
}

// SimplePathFinder uses A* algorithm with uniform movement cost.
// It finds the shortest path around obstacles using hex distance as heuristic.
type SimplePathFinder struct{}

// NewSimplePathFinder creates a new A* pathfinder
func NewSimplePathFinder() *SimplePathFinder {
	return &SimplePathFinder{}
}

// FindPath implements PathFinder using A* algorithm.
// Uses hex distance as heuristic (admissible - never overestimates).
func (p *SimplePathFinder) FindPath(start, goal CubeCoordinate, blocked map[CubeCoordinate]bool) []CubeCoordinate {
	if start == goal {
		return []CubeCoordinate{}
	}

	// If goal is blocked, no path exists
	if blocked[goal] {
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

		// Check all neighbors
		for _, neighbor := range current.pos.GetNeighbors() {
			// Skip blocked hexes
			if blocked[neighbor] {
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
