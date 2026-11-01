// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"cmp"
	"slices"
)

// Listener is one key listening for a beat, with the scope its state
// requires, or nil when unscoped.
type Listener[K cmp.Ordered] struct {
	Key   K
	Scope *ScopeLeaf
}

// Listeners answers who listens for a beat: beat to listeners in key order.
// A host sets a key's listens from Program.Listens whenever its state
// changes and filters what For returns by scope itself. It knows nothing of
// what a key names. The zero Listeners is empty and ready to use; it is not
// safe for concurrent use.
//
// Set and Drop never change a slice For returned: they replace it. A host
// may walk the listeners for a beat, step each, and re-set the ones whose
// state changed as it goes.
type Listeners[K cmp.Ordered] struct {
	byBeat map[Beat][]Listener[K]
	byKey  map[K][]Listen
}

// Set replaces what k listens for with ls. A beat repeated in ls keeps its
// first listen; an empty ls forgets k.
func (l *Listeners[K]) Set(k K, ls []Listen) {
	ls = firstPerBeat(ls)
	if cur, held := l.byKey[k]; held && slices.EqualFunc(cur, ls, sameListen) {
		return // unchanged: the slices For returned stay as they are
	}
	l.Drop(k)
	if len(ls) == 0 {
		return
	}
	if l.byBeat == nil {
		l.byBeat = map[Beat][]Listener[K]{}
		l.byKey = map[K][]Listen{}
	}
	l.byKey[k] = ls
	for _, x := range ls {
		cur := l.byBeat[x.Beat]
		i, _ := slices.BinarySearchFunc(cur, k, byKey[K])
		next := make([]Listener[K], 0, len(cur)+1)
		next = append(next, cur[:i]...)
		next = append(next, Listener[K]{Key: k, Scope: x.Scope})
		next = append(next, cur[i:]...)
		l.byBeat[x.Beat] = next
	}
}

// Drop forgets k; a key not held is ignored.
func (l *Listeners[K]) Drop(k K) {
	held, ok := l.byKey[k]
	if !ok {
		return
	}
	delete(l.byKey, k)
	for _, x := range held {
		cur := l.byBeat[x.Beat]
		i, found := slices.BinarySearchFunc(cur, k, byKey[K])
		if !found {
			continue
		}
		if len(cur) == 1 {
			delete(l.byBeat, x.Beat)
			continue
		}
		next := make([]Listener[K], 0, len(cur)-1)
		next = append(next, cur[:i]...)
		next = append(next, cur[i+1:]...)
		l.byBeat[x.Beat] = next
	}
}

// For returns the listeners for beat b in key order, or nil. The slice is
// shared: read it, never write it.
func (l *Listeners[K]) For(b Beat) []Listener[K] {
	return l.byBeat[b]
}

func byKey[K cmp.Ordered](x Listener[K], k K) int {
	return cmp.Compare(x.Key, k)
}

// firstPerBeat copies ls keeping the first listen of each beat, with its
// scope copied so the caller's slice is not held.
func firstPerBeat(ls []Listen) []Listen {
	if len(ls) == 0 {
		return nil
	}
	out := make([]Listen, 0, len(ls))
	for _, x := range ls {
		if slices.ContainsFunc(out, func(y Listen) bool { return y.Beat == x.Beat }) {
			continue
		}
		if x.Scope != nil {
			s := *x.Scope
			x.Scope = &s
		}
		out = append(out, x)
	}
	return out
}

func sameListen(a, b Listen) bool {
	if a.Beat != b.Beat || (a.Scope == nil) != (b.Scope == nil) {
		return false
	}
	return a.Scope == nil || *a.Scope == *b.Scope
}
