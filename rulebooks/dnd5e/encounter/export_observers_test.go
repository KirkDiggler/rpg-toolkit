// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// export_observers_test.go is the test-only door into the bounded refresh
// (rpg-project#762) for the external package's keep and castle fixtures. In a
// _test.go file, so nothing outside this package reaches it at build time.

// ExportedObserversOf is [Encounter.observersOf].
func (e *Encounter) ExportedObserversOf(cells []spatial.Position, declared []MemberID) ([]MemberID, error) {
	return e.observersOf(cells, declared)
}

// ExportedProbeRebuilds hands fn every observer list
// [Encounter.rebuildPercepts] is asked to rebuild until the returned func
// puts the probe back.
func ExportedProbeRebuilds(fn func(observers []MemberID)) func() {
	prev := rebuildProbe
	rebuildProbe = fn
	return func() { rebuildProbe = prev }
}

// ExportedAgreesWithFullRefresh holds the percepts the bounded refreshes left
// to the ones a full refresh writes over the same world, now: it snapshots
// every member's holdings and known cells, rebuilds every member's percept
// ([Encounter.rebuildPercepts] over the roster, the look every verb ran
// before rpg-project#762), snapshots again, and names the first member whose
// two snapshots differ.
//
// Every holding is compared on everything but Confirmed. Confirmed is "when
// last landed, whether or not the content changed": the restamp of a look
// that changed nothing, and not taking that look is the point of the bound.
// Observed — when this content was first seen — is compared.
func (e *Encounter) ExportedAgreesWithFullRefresh() error {
	before, err := e.perceptSnapshot()
	if err != nil {
		return err
	}
	if _, err := e.rebuildPercepts(e.rosterIDs()); err != nil {
		return err
	}
	after, err := e.perceptSnapshot()
	if err != nil {
		return err
	}
	for _, id := range e.rosterIDs() {
		if before[id] != after[id] {
			return fmt.Errorf("member %q: bounded refresh left\n%s\nfull refresh writes\n%s", id, before[id], after[id])
		}
	}

	return nil
}

// perceptSnapshot is every member's holdings (all channels, Confirmed left
// out) and known cells, one sorted string each.
func (e *Encounter) perceptSnapshot() (map[MemberID]string, error) {
	out := make(map[MemberID]string, len(e.members))
	for _, id := range e.rosterIDs() {
		held, err := e.intelLog.Held(sightMember(id))
		if err != nil {
			return nil, err
		}
		lines := make([]string, 0, len(held))
		for _, h := range held {
			lines = append(lines, fmt.Sprintf("%s ch=%s obs=%d via=%v payload=%s",
				h.Subject, h.Channel, h.Observed, h.CurrentVia, hex.EncodeToString(h.Payload)))
		}
		sort.Strings(lines)
		known := e.world.knownCells(id)
		cells := make([]string, 0, len(known))
		for cell := range known {
			cells = append(cells, fmt.Sprintf("%v", cell))
		}
		sort.Strings(cells)
		out[id] = strings.Join(lines, "\n") + "\nknown " + strings.Join(cells, " ")
	}

	return out, nil
}
