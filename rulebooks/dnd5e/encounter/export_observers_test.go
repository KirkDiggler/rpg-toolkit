// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// export_observers_test.go is the test-only door into the bounded refresh
// (rpg-project#762) for the external package's keep and castle fixtures. In a
// _test.go file, so nothing outside this package reaches it at build time.

// ExportedObserversOf is [Encounter.observersOf].
func (e *Encounter) ExportedObserversOf(cells []spatial.Position, declared []MemberID) ([]MemberID, error) {
	return e.observersOf(cells, declared)
}

// ExportedProbeRebuilds hands fn every observer list e's
// [Encounter.rebuildPercepts] is asked to rebuild until the returned func
// puts the probe back.
func (e *Encounter) ExportedProbeRebuilds(fn func(observers []MemberID)) func() {
	prev := e.rebuildProbe
	e.rebuildProbe = fn
	return func() { e.rebuildProbe = prev }
}

// ExportedEveryoneLooks makes every look of e the whole roster's, as before
// rpg-project#762: the oracle a bounded twin is held to.
func (e *Encounter) ExportedEveryoneLooks() { e.everyoneLooks = true }

// ExportedHolding is what observer holds about subject, on any channel.
func (e *Encounter) ExportedHolding(observer, subject MemberID) (perception.Holding, bool, error) {
	held, err := e.memberIntel(observer)
	if err != nil {
		return perception.Holding{}, false, err
	}
	for _, h := range held {
		if h.Subject == core.EntityID(subject) {
			return h, true, nil
		}
	}
	return perception.Holding{}, false, nil
}

// ExportedPercepts is every member's percept as one sorted string each: every
// holding on every channel (subject, channel, Observed, CurrentVia, payload)
// and the member's known cells. A holding that is NOT current carries its
// Confirmed too — a memory's stamp, the one a mind ages it by. A current
// holding's does not: it is perceived now, and a look that would only have
// restamped it is the look the bound skips.
func (e *Encounter) ExportedPercepts() (map[MemberID]string, error) {
	out := make(map[MemberID]string, len(e.members))
	for _, id := range e.rosterIDs() {
		held, err := e.intelLog.Held(sightMember(id))
		if err != nil {
			return nil, err
		}
		lines := make([]string, 0, len(held))
		for _, h := range held {
			stamp := "now"
			if len(h.CurrentVia) == 0 {
				stamp = fmt.Sprintf("confirmed=%d", h.Confirmed)
			}
			lines = append(lines, fmt.Sprintf("%s ch=%s obs=%d %s via=%v payload=%s",
				h.Subject, h.Channel, h.Observed, stamp, h.CurrentVia, hex.EncodeToString(h.Payload)))
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
