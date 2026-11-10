// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// visibleCacheBound bounds [Encounter.visibleCache]: at the bound the whole
// cache is dropped and refilled by use, which costs one sweep per visited key
// again and nothing else. Every constructor copies it into
// [Encounter.visibleCacheEntries]; a test shrinks its own encounter's bound
// through [newEncounter], never a value another encounter reads
// (rpg-toolkit#2317).
const visibleCacheBound = 4096

// visibleBits is one sweep's answer as a bitset over the sweep disk in
// [field.sweepCells] order: bit i set means disk[i] is visible from the key's
// cell. A disk of the whole floor is the same order every time, so the bits
// stay aligned across passes.
type visibleBits []uint64

func (b visibleBits) set(i int)      { b[i/64] |= 1 << (uint(i) % 64) }
func (b visibleBits) has(i int) bool { return b[i/64]&(1<<(uint(i)%64)) != 0 }

// visibleFrom is the floor seen from key's cell: the cached bits when the
// key was swept before, else one sweep of disk through sees, cached for the
// next observer to stand there with the same sight, opacity and concealment
// knowledge. The key carries every input the sweep reads ([sweepKeyOf],
// [Encounter.sightOpacity]), so a hit is the same answer by construction;
// the encounter frees entries whose opacity has passed and at the bound.
//
// THE RAYS ARE CAST ONCE PER CELL, NOT ONCE PER VISIT (rpg-project#879 F10):
// a mover re-tested every unknown cell of its disk from every cell it stepped
// to, and in a keep most of the disk is floor behind walls that is never
// learned and so never skipped. What a cell sees does not change when a
// member moves (members never block sight), so the second visit reads the
// first visit's answer.
//
// TWO LAYERS, EACH WHERE ITS INPUTS LIVE (rpg-project#879 S5, R4): a key
// whose observer has no face hidden from it is RAW — its answer depends on
// the site and the key alone — and is read from and written to the site's
// memo ([siteSight]), shared by every game on that content and kept across
// loads; a key carrying a concealment view is the instance's, here. Both
// are sound by the key; the split decides only who may reuse an entry.
func (e *Encounter) visibleFrom(key sightSweepKey, disk []spatial.Position, raw bool, sees func(spatial.Position) bool) visibleBits {
	words := (len(disk) + 63) / 64
	if raw && e.site != nil {
		if bits, hit := e.site.sight.get(key, words); hit {
			e.visibleHits++
			return bits
		}
		bits := sweepDisk(disk, sees)
		e.site.sight.put(key, bits)
		e.visibleMisses++
		return bits
	}
	if bits, hit := e.visibleCache[key]; hit && len(bits) == words {
		e.visibleHits++
		return bits
	}
	bits := sweepDisk(disk, sees)
	if len(e.visibleCache) >= e.visibleCacheEntries {
		e.visibleCache = nil
	}
	if e.visibleCache == nil {
		e.visibleCache = make(map[sightSweepKey]visibleBits)
	}
	e.visibleCache[key] = bits
	e.visibleMisses++

	return bits
}

// sweepDisk is one sweep: bit i set when sees answers true for disk[i].
func sweepDisk(disk []spatial.Position, sees func(spatial.Position) bool) visibleBits {
	bits := make(visibleBits, (len(disk)+63)/64)
	for i, cell := range disk {
		if sees(cell) {
			bits.set(i)
		}
	}
	return bits
}
