// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// export_wholeacts_test.go is the test-only door into StoryInput.WholeActs'
// act lookup (rpg-project#891) for the external package's tests. In a
// _test.go file, so nothing outside this package reaches it.

// CountActLookupsForTest counts every whole-record act lookup a WholeActs
// read pays until t ends, and returns the running count. The test that
// calls it must not run in parallel: the count is package-wide.
func CountActLookupsForTest(t testing.TB) func() int {
	t.Helper()
	var n atomic.Int64
	previous := lookupActBeats
	lookupActBeats = func(e *Encounter, act string) ([]record.Entry, error) {
		n.Add(1)
		return previous(e, act)
	}
	t.Cleanup(func() { lookupActBeats = previous })
	return func() int { return int(n.Load()) }
}

// AppendBeatForTest appends one plain beat for exactly the audience given,
// under the open act, through the composition's one append seam.
func (e *Encounter) AppendBeatForTest(audience ...MemberID) error {
	ids := make([]core.EntityID, 0, len(audience))
	for _, id := range audience {
		ids = append(ids, core.EntityID(id))
	}
	payload, err := json.Marshal(map[string]string{"beat": "test-beat"})
	if err != nil {
		return err
	}
	_, err = e.appendBeat(&record.AppendInput{Audience: ids, Payload: payload})
	return err
}
