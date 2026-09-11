package spatial

import (
	"container/heap"
	"errors"
	"sort"
)

// FieldInput asks for a distance field: every cell reachable from Sources
// under Passable, with its cost-weighted distance.
//
// Sources is plural on purpose — a field flooded from several threats at once
// is a different question than a field flooded from one, and both are reads of
// the same primitive.
//
// Passable is required: a field with no predicate would silently treat every
// cell as open, which is a zero value that lies. Cost nil means one per step,
// which makes the flood a breadth-first search. Limit 0 means unbounded.
type FieldInput struct {
	Sources  []Position
	Passable func(from, to Position) bool
	Cost     func(from, to Position) int
	Limit    int
}

// FieldOutput is the field. Dist holds every reached cell keyed by position,
// including the sources at distance 0. Prev holds the cell each reached cell
// was entered from, and is absent for sources.
type FieldOutput struct {
	Dist map[Position]int
	Prev map[Position]Position
}

// ErrNoSources is returned when a field is asked for with nothing to flood from.
var ErrNoSources = errors.New("spatial: field needs at least one source")

// ErrNoPassable is returned when Passable is nil; a field with no predicate
// would silently treat every cell as open, which is a zero value that lies.
var ErrNoPassable = errors.New("spatial: field needs a passable predicate")

// Field floods outward from Sources over g.GetNeighbors, Dijkstra by Cost,
// and returns the distance and predecessor of every cell it reached.
//
// It answers geometry and search over geometry only. What a cell means —
// blocked, costly, burning — is the game's, and reaches the field through
// Passable and Cost. Reach is a field with a Limit, a path is PathTo on the
// same field, and a blast that spreads around corners is a field under a
// walls-only predicate.
//
// Neighbors are relaxed in X-then-Y order so equal-cost ties break the same
// way on every run and on every grid.
func Field(g Grid, in FieldInput) (FieldOutput, error) {
	if len(in.Sources) == 0 {
		return FieldOutput{}, ErrNoSources
	}
	if in.Passable == nil {
		return FieldOutput{}, ErrNoPassable
	}
	cost := in.Cost
	if cost == nil {
		cost = unitCost
	}

	out := FieldOutput{Dist: make(map[Position]int), Prev: make(map[Position]Position)}
	pq := &fieldQueue{}
	for _, src := range in.Sources {
		if _, seen := out.Dist[src]; seen {
			continue
		}
		out.Dist[src] = 0
		heap.Push(pq, fieldItem{pos: src, dist: 0})
	}

	for pq.Len() > 0 {
		cur := pq.pop()
		if cur.dist > out.Dist[cur.pos] {
			continue // a stale entry, left behind when a cheaper route arrived
		}
		relaxNeighbors(g, &in, cost, out, pq, cur)
	}
	return out, nil
}

// PathTo reads the field backwards from goal to the source that reached it.
// The returned path excludes the source and ends at goal, matching the
// contract a route has always had. ok is false when goal was never reached.
func (f FieldOutput) PathTo(goal Position) ([]Position, bool) {
	if _, reached := f.Dist[goal]; !reached {
		return nil, false
	}
	var rev []Position
	cur := goal
	for {
		prev, hasPrev := f.Prev[cur]
		if !hasPrev {
			break // cur is a source
		}
		rev = append(rev, cur)
		cur = prev
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev, true
}

// unitCost is the default Cost: one per step, which makes the flood a
// breadth-first search.
func unitCost(_, _ Position) int { return 1 }

// relaxNeighbors offers cur's neighbors a cheaper route through cur, pushing
// each one it improved. It is split out of Field so the flood driver and the
// relaxation each stay readable.
func relaxNeighbors(g Grid, in *FieldInput, cost func(from, to Position) int,
	out FieldOutput, pq *fieldQueue, cur fieldItem) {
	ns := g.GetNeighbors(cur.pos)
	sortPositions(ns)
	for _, n := range ns {
		if !in.Passable(cur.pos, n) {
			continue
		}
		d := cur.dist + cost(cur.pos, n)
		if in.Limit > 0 && d > in.Limit {
			continue
		}
		if prev, seen := out.Dist[n]; seen && prev <= d {
			continue
		}
		out.Dist[n] = d
		out.Prev[n] = cur.pos
		heap.Push(pq, fieldItem{pos: n, dist: d})
	}
}

// sortPositions orders positions by X then Y, so the order neighbors are
// relaxed in does not depend on the grid implementation's direction list.
func sortPositions(ps []Position) {
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].X != ps[j].X {
			return ps[i].X < ps[j].X
		}
		return ps[i].Y < ps[j].Y
	})
}

// fieldItem is one cell waiting to be expanded, with the distance it was
// queued at.
type fieldItem struct {
	pos  Position
	dist int
}

// fieldQueue is a min-heap of fieldItem ordered by distance, then X, then Y.
type fieldQueue []fieldItem

func (q fieldQueue) Len() int { return len(q) }

func (q fieldQueue) Less(i, j int) bool {
	if q[i].dist != q[j].dist {
		return q[i].dist < q[j].dist
	}
	if q[i].pos.X != q[j].pos.X {
		return q[i].pos.X < q[j].pos.X
	}
	return q[i].pos.Y < q[j].pos.Y
}

func (q fieldQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }

// Push implements heap.Interface. Only Field pushes, and only fieldItem.
func (q *fieldQueue) Push(x any) {
	item, ok := x.(fieldItem)
	if !ok {
		return
	}
	*q = append(*q, item)
}

// Pop implements heap.Interface. Callers inside this package use pop, which
// keeps the type assertion in one place.
func (q *fieldQueue) Pop() any {
	old := *q
	n := len(old)
	item := old[n-1]
	*q = old[:n-1]
	return item
}

// pop returns the lowest-cost item. heap.Pop is untyped; this is the only
// place the value is read back as a fieldItem.
func (q *fieldQueue) pop() fieldItem {
	item, _ := heap.Pop(q).(fieldItem)
	return item
}
