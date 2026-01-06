# Monster Pathfinding Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace greedy monster movement with A* pathfinding that navigates around walls and obstacles.

**Architecture:** Add a `PathFinder` interface in the monster package with a `SimplePathFinder` implementation using A*. The monster's `moveTowardEnemy` function will use this interface, allowing future algorithm swaps. Blocked hexes come from `PerceptionData.BlockedHexes`.

**Tech Stack:** Go, spatial package (CubeCoordinate, GetNeighbors, Distance)

---

## Task 1: Create PathFinder Interface

**Files:**
- Create: `rulebooks/dnd5e/monster/pathfinder.go`
- Test: `rulebooks/dnd5e/monster/pathfinder_test.go`

**Step 1: Create the interface file**

```go
// Package monster provides monster/enemy entity types for D&D 5e combat
package monster

import "github.com/KirkDiggler/rpg-toolkit/tools/spatial"

// PathFinder finds paths between hex positions avoiding obstacles.
// Implementations can use different algorithms (A*, Dijkstra, weighted, etc.)
type PathFinder interface {
	// FindPath returns a path from start to goal avoiding blocked hexes.
	// Returns the path excluding start, including goal.
	// Returns empty slice if no path exists or start == goal.
	FindPath(start, goal spatial.CubeCoordinate, blocked map[spatial.CubeCoordinate]bool) []spatial.CubeCoordinate
}
```

**Step 2: Commit**

```bash
git add rulebooks/dnd5e/monster/pathfinder.go
git commit -m "feat(monster): add PathFinder interface for movement algorithms"
```

---

## Task 2: Implement SimplePathFinder with A*

**Files:**
- Modify: `rulebooks/dnd5e/monster/pathfinder.go`
- Test: `rulebooks/dnd5e/monster/pathfinder_test.go`

**Step 1: Write the failing test for direct path (no obstacles)**

```go
package monster

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

type PathFinderTestSuite struct {
	suite.Suite
	pathFinder *SimplePathFinder
}

func TestPathFinderSuite(t *testing.T) {
	suite.Run(t, new(PathFinderTestSuite))
}

func (s *PathFinderTestSuite) SetupTest() {
	s.pathFinder = NewSimplePathFinder()
}

func (s *PathFinderTestSuite) TestDirectPath_NoObstacles() {
	start := spatial.CubeCoordinate{X: 0, Y: 0, Z: 0}
	goal := spatial.CubeCoordinate{X: 3, Y: 0, Z: -3}
	blocked := make(map[spatial.CubeCoordinate]bool)

	path := s.pathFinder.FindPath(start, goal, blocked)

	s.Require().Len(path, 3, "path should have 3 steps")
	s.Equal(goal, path[len(path)-1], "path should end at goal")

	// Verify each step is a valid neighbor of the previous
	current := start
	for _, next := range path {
		s.Equal(1, current.Distance(next), "each step should be 1 hex away")
		current = next
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./rulebooks/dnd5e/monster/... -run TestPathFinderSuite/TestDirectPath_NoObstacles -v
```

Expected: FAIL - `NewSimplePathFinder` not defined

**Step 3: Implement SimplePathFinder with A***

Add to `pathfinder.go`:

```go
// SimplePathFinder uses A* algorithm with uniform movement cost.
// It finds the shortest path around obstacles using hex distance as heuristic.
type SimplePathFinder struct{}

// NewSimplePathFinder creates a new A* pathfinder
func NewSimplePathFinder() *SimplePathFinder {
	return &SimplePathFinder{}
}

// FindPath implements PathFinder using A* algorithm.
// Uses hex distance as heuristic (admissible - never overestimates).
func (p *SimplePathFinder) FindPath(start, goal spatial.CubeCoordinate, blocked map[spatial.CubeCoordinate]bool) []spatial.CubeCoordinate {
	if start == goal {
		return []spatial.CubeCoordinate{}
	}

	// Priority queue entry
	type node struct {
		pos    spatial.CubeCoordinate
		fScore int // g + h
	}

	// Open set as a slice (simple priority queue)
	openSet := []node{{pos: start, fScore: start.Distance(goal)}}

	// Track where we came from for path reconstruction
	cameFrom := make(map[spatial.CubeCoordinate]spatial.CubeCoordinate)

	// g-score: cost from start to this node
	gScore := make(map[spatial.CubeCoordinate]int)
	gScore[start] = 0

	// Track what's in open set for O(1) lookup
	inOpenSet := make(map[spatial.CubeCoordinate]bool)
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
					openSet = append(openSet, node{pos: neighbor, fScore: fScore})
					inOpenSet[neighbor] = true
				}
			}
		}
	}

	// No path found
	return []spatial.CubeCoordinate{}
}

// reconstructPath builds the path from start to goal using cameFrom map
func (p *SimplePathFinder) reconstructPath(cameFrom map[spatial.CubeCoordinate]spatial.CubeCoordinate, current spatial.CubeCoordinate) []spatial.CubeCoordinate {
	path := []spatial.CubeCoordinate{current}
	for {
		prev, ok := cameFrom[current]
		if !ok {
			break
		}
		path = append([]spatial.CubeCoordinate{prev}, path...)
		current = prev
	}
	// Remove start from path (path should exclude start)
	if len(path) > 0 {
		path = path[1:]
	}
	return path
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./rulebooks/dnd5e/monster/... -run TestPathFinderSuite/TestDirectPath_NoObstacles -v
```

Expected: PASS

**Step 5: Commit**

```bash
git add rulebooks/dnd5e/monster/pathfinder.go rulebooks/dnd5e/monster/pathfinder_test.go
git commit -m "feat(monster): implement SimplePathFinder with A* algorithm"
```

---

## Task 3: Add Test for Path Around Obstacle

**Files:**
- Test: `rulebooks/dnd5e/monster/pathfinder_test.go`

**Step 1: Write test for L-shaped obstacle**

```go
func (s *PathFinderTestSuite) TestPathAroundLShapedWall() {
	// Monster at (0,0,0), target at (3,0,-3)
	// L-shaped wall blocking direct path:
	//   (1,-1,0), (1,0,-1), (2,0,-2)
	start := spatial.CubeCoordinate{X: 0, Y: 0, Z: 0}
	goal := spatial.CubeCoordinate{X: 3, Y: 0, Z: -3}
	blocked := map[spatial.CubeCoordinate]bool{
		{X: 1, Y: -1, Z: 0}:  true,
		{X: 1, Y: 0, Z: -1}:  true,
		{X: 2, Y: 0, Z: -2}:  true,
	}

	path := s.pathFinder.FindPath(start, goal, blocked)

	s.Require().NotEmpty(path, "should find path around wall")
	s.Equal(goal, path[len(path)-1], "path should end at goal")

	// Verify path doesn't go through blocked hexes
	for _, pos := range path {
		s.False(blocked[pos], "path should not include blocked hex %v", pos)
	}

	// Verify path is connected
	current := start
	for _, next := range path {
		s.Equal(1, current.Distance(next), "each step should be 1 hex away")
		current = next
	}
}
```

**Step 2: Run test**

```bash
go test ./rulebooks/dnd5e/monster/... -run TestPathFinderSuite/TestPathAroundLShapedWall -v
```

Expected: PASS (A* already handles this)

**Step 3: Commit**

```bash
git add rulebooks/dnd5e/monster/pathfinder_test.go
git commit -m "test(monster): add L-shaped wall pathfinding test"
```

---

## Task 4: Add Test for No Path (Trapped)

**Files:**
- Test: `rulebooks/dnd5e/monster/pathfinder_test.go`

**Step 1: Write test for completely surrounded monster**

```go
func (s *PathFinderTestSuite) TestNoPath_Surrounded() {
	start := spatial.CubeCoordinate{X: 0, Y: 0, Z: 0}
	goal := spatial.CubeCoordinate{X: 5, Y: 0, Z: -5}

	// Block all 6 neighbors of start
	blocked := map[spatial.CubeCoordinate]bool{
		{X: 1, Y: -1, Z: 0}:  true,
		{X: 1, Y: 0, Z: -1}:  true,
		{X: 0, Y: 1, Z: -1}:  true,
		{X: -1, Y: 1, Z: 0}:  true,
		{X: -1, Y: 0, Z: 1}:  true,
		{X: 0, Y: -1, Z: 1}:  true,
	}

	path := s.pathFinder.FindPath(start, goal, blocked)

	s.Empty(path, "should return empty path when completely surrounded")
}
```

**Step 2: Run test**

```bash
go test ./rulebooks/dnd5e/monster/... -run TestPathFinderSuite/TestNoPath_Surrounded -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add rulebooks/dnd5e/monster/pathfinder_test.go
git commit -m "test(monster): add test for trapped monster (no path)"
```

---

## Task 5: Add Test for Same Position

**Files:**
- Test: `rulebooks/dnd5e/monster/pathfinder_test.go`

**Step 1: Write test for start equals goal**

```go
func (s *PathFinderTestSuite) TestSamePosition() {
	pos := spatial.CubeCoordinate{X: 2, Y: -1, Z: -1}
	blocked := make(map[spatial.CubeCoordinate]bool)

	path := s.pathFinder.FindPath(pos, pos, blocked)

	s.Empty(path, "should return empty path when already at goal")
}
```

**Step 2: Run test**

```bash
go test ./rulebooks/dnd5e/monster/... -run TestPathFinderSuite/TestSamePosition -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add rulebooks/dnd5e/monster/pathfinder_test.go
git commit -m "test(monster): add test for same position edge case"
```

---

## Task 6: Integrate PathFinder into moveTowardEnemy

**Files:**
- Modify: `rulebooks/dnd5e/monster/monster.go` (lines 596-677)

**Step 1: Write integration test**

Add to existing monster test file or create new:

```go
func (s *MonsterTestSuite) TestMoveTowardEnemy_AroundObstacle() {
	monster := NewGoblin("goblin-1")

	// Monster at (0,0,0), enemy at (3,0,-3), wall in between
	perception := &PerceptionData{
		MyPosition: spatial.CubeCoordinate{X: 0, Y: 0, Z: 0},
		Enemies: []PerceivedEntity{
			{
				Entity:   &mockEntity{id: "enemy-1"},
				Position: spatial.CubeCoordinate{X: 3, Y: 0, Z: -3},
				Distance: 3,
				Adjacent: false,
			},
		},
		BlockedHexes: []spatial.CubeCoordinate{
			{X: 1, Y: -1, Z: 0},
			{X: 1, Y: 0, Z: -1},
		},
	}

	input := &TurnInput{
		Perception: perception,
		Speed:      6, // 30ft = 6 hexes
	}
	result := &TurnResult{
		Movement: make([]spatial.CubeCoordinate, 0),
	}

	monster.moveTowardEnemy(input, result)

	s.NotEmpty(result.Movement, "monster should move")
	// Verify path doesn't include blocked hexes
	blocked := make(map[spatial.CubeCoordinate]bool)
	for _, hex := range perception.BlockedHexes {
		blocked[hex] = true
	}
	for _, pos := range result.Movement {
		s.False(blocked[pos], "movement should not include blocked hex")
	}
}
```

**Step 2: Modify moveTowardEnemy to use PathFinder**

Replace lines 631-656 in monster.go:

```go
// moveTowardEnemy moves the monster toward the closest enemy if not already adjacent.
// Uses A* pathfinding to navigate around obstacles.
// Updates perception data to reflect new position after movement.
func (m *Monster) moveTowardEnemy(input *TurnInput, result *TurnResult) {
	if input.Perception == nil || len(input.Perception.Enemies) == 0 {
		return
	}

	closest := input.Perception.ClosestEnemy()
	if closest == nil || closest.Adjacent {
		// Already adjacent or no enemy - no movement needed
		return
	}

	// Calculate how far we can move (use input speed, fall back to monster's speed)
	speed := input.Speed
	if speed == 0 {
		speed = m.speed.Walk / 5 // Convert feet to hexes
	}
	if speed == 0 {
		return // Can't move
	}

	// Build blocked hex map for pathfinding
	blocked := make(map[spatial.CubeCoordinate]bool)
	for _, hex := range input.Perception.BlockedHexes {
		blocked[hex] = true
	}

	// Find path using A*
	pathFinder := NewSimplePathFinder()
	path := pathFinder.FindPath(input.Perception.MyPosition, closest.Position, blocked)

	if len(path) == 0 {
		return // No valid path - stay put
	}

	// Calculate how many hexes to move (stop 1 hex short to stay adjacent)
	hexesToMove := len(path) - 1 // Stop adjacent to target
	if hexesToMove <= 0 {
		return // Already close enough
	}
	if hexesToMove > speed {
		hexesToMove = speed
	}

	// Build movement path (include start position, then each hex moved to)
	current := input.Perception.MyPosition
	movementPath := []spatial.CubeCoordinate{current}
	for i := 0; i < hexesToMove; i++ {
		current = path[i]
		movementPath = append(movementPath, current)
	}

	// Record full path (every hex crossed)
	result.Movement = movementPath

	// Update perception with new position
	input.Perception.MyPosition = current

	// Recalculate distances and adjacency for enemies
	for i := range input.Perception.Enemies {
		enemy := &input.Perception.Enemies[i]
		enemy.Distance = current.Distance(enemy.Position)
		enemy.Adjacent = enemy.Distance == 1
	}

	// Recalculate distances and adjacency for allies
	for i := range input.Perception.Allies {
		ally := &input.Perception.Allies[i]
		ally.Distance = current.Distance(ally.Position)
		ally.Adjacent = ally.Distance == 1
	}
}
```

**Step 3: Run all tests**

```bash
go test ./rulebooks/dnd5e/monster/... -v
```

Expected: PASS

**Step 4: Commit**

```bash
git add rulebooks/dnd5e/monster/monster.go rulebooks/dnd5e/monster/monster_test.go
git commit -m "feat(monster): integrate A* pathfinding into moveTowardEnemy"
```

---

## Task 7: Run Full Test Suite and Lint

**Step 1: Run all monster tests**

```bash
go test ./rulebooks/dnd5e/monster/... -v
```

**Step 2: Run linter**

```bash
golangci-lint run ./rulebooks/dnd5e/monster/...
```

**Step 3: Fix any issues found**

**Step 4: Run pre-commit**

```bash
make pre-commit
```

**Step 5: Final commit if needed**

```bash
git add -A
git commit -m "chore(monster): fix lint issues"
```

---

## Task 8: Create Pull Request

**Step 1: Push branch**

```bash
git push -u origin feat/monster-pathfinding-522
```

**Step 2: Create PR**

```bash
gh pr create --title "feat(monster): implement A* pathfinding for monster movement" --body "$(cat <<'EOF'
## Summary

Implements A* pathfinding for monster movement to fix #522.

- Adds `PathFinder` interface for algorithm isolation
- Implements `SimplePathFinder` using A* with hex distance heuristic
- Updates `moveTowardEnemy` to use pathfinding instead of greedy algorithm
- Monsters now navigate around walls and obstacles

## Test Plan

- [x] Unit tests for A* pathfinder (direct path, L-shaped wall, surrounded, same position)
- [x] Integration test for monster movement around obstacles
- [x] All existing monster tests pass

Closes #522, #523

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

---

## Summary of Files Changed

| File | Action | Purpose |
|------|--------|---------|
| `rulebooks/dnd5e/monster/pathfinder.go` | Create | PathFinder interface + SimplePathFinder A* implementation |
| `rulebooks/dnd5e/monster/pathfinder_test.go` | Create | Unit tests for pathfinding |
| `rulebooks/dnd5e/monster/monster.go` | Modify | Update moveTowardEnemy to use PathFinder |
| `rulebooks/dnd5e/monster/monster_test.go` | Modify | Add integration test for obstacle avoidance |
