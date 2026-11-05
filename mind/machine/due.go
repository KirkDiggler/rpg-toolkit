// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"cmp"
	"container/heap"
)

// Due answers who is due at a round: a heap of (round, key) ordered by round
// and then by key, so keys due at one round pop in key order (a host keying
// by seat pops in seat order). It holds one entry per key. It knows nothing
// of what a key names. The zero Due is empty and ready to use; it is not
// safe for concurrent use.
type Due[K cmp.Ordered] struct {
	h dueHeap[K]
}

// Arm puts k on the heap at round at, replacing any entry k already has,
// earlier or later.
func (d *Due[K]) Arm(k K, at uint64) {
	if d.h.index == nil {
		d.h.index = map[K]int{}
	}
	if i, ok := d.h.index[k]; ok {
		d.h.entries[i].at = at
		heap.Fix(&d.h, i)
		return
	}
	heap.Push(&d.h, dueEntry[K]{at: at, key: k})
}

// Disarm takes k off the heap; a key not on it is ignored.
func (d *Due[K]) Disarm(k K) {
	if i, ok := d.h.index[k]; ok {
		heap.Remove(&d.h, i)
	}
}

// Pop removes and returns every key due at or before upTo, in (round, key)
// order. It returns nil when none is.
func (d *Due[K]) Pop(upTo uint64) []K {
	var out []K
	for len(d.h.entries) > 0 && d.h.entries[0].at <= upTo {
		// The heap holds only dueEntry[K]; the comma-ok spells that
		// invariant for errcheck rather than asserting it blind.
		popped, _ := heap.Pop(&d.h).(dueEntry[K])
		out = append(out, popped.key)
	}
	return out
}

// Len returns how many keys are on the heap.
func (d *Due[K]) Len() int {
	return len(d.h.entries)
}

type dueEntry[K cmp.Ordered] struct {
	at  uint64
	key K
}

// dueHeap is the container/heap implementation behind Due, tracking each
// key's position so Arm and Disarm find it without a scan.
type dueHeap[K cmp.Ordered] struct {
	entries []dueEntry[K]
	index   map[K]int
}

func (h *dueHeap[K]) Len() int { return len(h.entries) }

func (h *dueHeap[K]) Less(i, j int) bool {
	a, b := h.entries[i], h.entries[j]
	if a.at != b.at {
		return a.at < b.at
	}
	return a.key < b.key
}

func (h *dueHeap[K]) Swap(i, j int) {
	h.entries[i], h.entries[j] = h.entries[j], h.entries[i]
	h.index[h.entries[i].key] = i
	h.index[h.entries[j].key] = j
}

func (h *dueHeap[K]) Push(x any) {
	e, _ := x.(dueEntry[K]) // the heap holds only dueEntry[K]
	h.index[e.key] = len(h.entries)
	h.entries = append(h.entries, e)
}

func (h *dueHeap[K]) Pop() any {
	n := len(h.entries) - 1
	e := h.entries[n]
	h.entries = h.entries[:n]
	delete(h.index, e.key)
	return e
}
