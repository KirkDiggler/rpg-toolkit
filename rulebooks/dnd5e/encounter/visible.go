// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/binary"
	"hash/fnv"
	"math"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// visibleCacheEntries bounds [Encounter.visibleCache]: at the bound the whole
// cache is dropped and refilled by use, which costs one sweep per visited key
// again and nothing else. A var so a test can shrink it.
var visibleCacheEntries = 4096

// visibleBits is one sweep's answer as a bitset over the sweep disk in
// [field.sweepCells] order: bit i set means disk[i] is visible from the key's
// cell. A disk of the whole floor is the same order every time, so the bits
// stay aligned across passes.
type visibleBits []uint64

func (b visibleBits) set(i int)      { b[i/64] |= 1 << (uint(i) % 64) }
func (b visibleBits) has(i int) bool { return b[i/64]&(1<<(uint(i)%64)) != 0 }

// visibleFrom is the floor seen from key's cell: the cached bits when the
// key was swept before under this blockers epoch, else one sweep of disk
// through sees, cached for the next observer to stand there with the same
// sight, opacity and concealment knowledge.
//
// THE RAYS ARE CAST ONCE PER CELL, NOT ONCE PER VISIT (rpg-project#879 F10):
// a mover re-tested every unknown cell of its disk from every cell it stepped
// to, and in a keep most of the disk is floor behind walls that is never
// learned and so never skipped. What a cell sees does not change when a
// member moves (members never block sight), so the second visit reads the
// first visit's answer.
func (e *Encounter) visibleFrom(key sightSweepKey, disk []spatial.Position, sees func(spatial.Position) bool) visibleBits {
	if bits, hit := e.visibleCache[key]; hit && len(bits) == (len(disk)+63)/64 {
		e.visibleHits++
		return bits
	}
	bits := make(visibleBits, (len(disk)+63)/64)
	for i, cell := range disk {
		if sees(cell) {
			bits.set(i)
		}
	}
	if len(e.visibleCache) >= visibleCacheEntries {
		e.visibleCache = nil
	}
	if e.visibleCache == nil {
		e.visibleCache = make(map[sightSweepKey]visibleBits)
	}
	e.visibleCache[key] = bits
	e.visibleMisses++

	return bits
}

// blockersEpoch names everything besides the sweep key that can change what
// a cell sees: every door's state and every opaque entity's cell. Members
// are never opaque ([memberEntity.BlocksLineOfSight]), so a member's walk
// leaves the epoch alone. Read once per sight pass by
// [Encounter.rebuildPercepts]; a changed value drops [Encounter.visibleCache].
func (e *Encounter) blockersEpoch() uint64 {
	h := fnv.New64a()
	var buf [8]byte
	for _, d := range e.doors {
		_, _ = h.Write([]byte(d.id))
		_, _ = h.Write([]byte(d.state.Kind()))
		if d.blocks() {
			_, _ = h.Write([]byte{1})
		}
		_, _ = h.Write([]byte{0})
	}
	// The canvas lists entities in no fixed order; opaque ones are few, so
	// their contributions are combined order-free.
	var opaque uint64
	for id, ent := range e.canvas.GetAllEntities() {
		if !entitiesBlockSight([]core.Entity{ent}) {
			continue
		}
		at, placed := e.canvas.GetEntityPosition(id)
		if !placed {
			continue
		}
		one := fnv.New64a()
		_, _ = one.Write([]byte(id))
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(at.X))
		_, _ = one.Write(buf[:])
		binary.LittleEndian.PutUint64(buf[:], math.Float64bits(at.Y))
		_, _ = one.Write(buf[:])
		opaque += one.Sum64()
	}
	binary.LittleEndian.PutUint64(buf[:], opaque)
	_, _ = h.Write(buf[:])

	return h.Sum64()
}
