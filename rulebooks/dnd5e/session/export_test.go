// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// LoadWorldForTest composes a session's world exactly as a read does, so an
// external test can ask the encounter directly and compare its answer with
// what the session projected.
func (m *Manager) LoadWorldForTest(ctx context.Context, sessionID string) (*encounter.Encounter, error) {
	data, err := m.loadSessionData(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	return m.loadWorld(ctx, data)
}

// FamilyOfBeatForTest exposes the act family table (act_family.go) to
// TestEveryBeatHasAFamily: a beat's family, whether it has one, and whether
// it is one of R9's record-only beats.
func FamilyOfBeatForTest(beat string) (family Family, mapped, recordOnlyBeat bool) {
	family, mapped = familyOf(beat)
	return family, mapped, recordOnly[beat]
}

// SpeakMenuForTest reads the speak menu for a member exactly as Afford does
// ([readSpeakMenu]), for the benchmark that prices the menu apart from the
// load every Afford pays first. It answers the number of lines on the menu.
func SpeakMenuForTest(enc *encounter.Encounter, member string) (int, error) {
	menu, err := readSpeakMenu(enc, member)
	return len(menu.lines), err
}

// CountEncounterLoadsForTest counts every encounter this package loads from
// stored data until t ends, and returns the running count. The test that
// calls it must not run in parallel: the count is package-wide.
func CountEncounterLoadsForTest(t testing.TB) func() int {
	t.Helper()
	var n atomic.Int64
	previous := loadEncounter
	loadEncounter = func(in *encounter.LoadEncounterInput) (*encounter.Encounter, error) {
		n.Add(1)
		return previous(in)
	}
	t.Cleanup(func() { loadEncounter = previous })
	return func() int { return int(n.Load()) }
}

// ActFoldWatch is what [WatchActFoldForTest] saw the live fold do.
type ActFoldWatch struct {
	mu sync.Mutex
	// unbounded is, per recipient fold in commit order, the acts the fold
	// made before rpg-project#891: the recipient's WHOLE story, cut at the
	// last entry held before the baseline.
	unbounded []Act
	// folds is one row per recipient fold.
	folds []ActFoldRead
}

// ActFoldRead is one recipient fold at one commit: what it read, and what
// the reads it replaced would have.
type ActFoldRead struct {
	Member string
	// Read is how many entries the live fold's story read returned.
	Read int
	// Whole is how many entries the member's whole story holds — what the
	// unbounded fold copied.
	Whole int
	// Delta is how many of the member's entries are at or after the
	// baseline: what this verb told them.
	Delta int
	// Needed is how many entries the fold cannot do without: from the
	// member's first entry at the baseline, or the first entry of the
	// earliest act the delta continues, to the end.
	Needed int
}

// Unbounded is the unbounded fold's acts, every recipient fold in order.
func (w *ActFoldWatch) Unbounded() []Act {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]Act(nil), w.unbounded...)
}

// Folds is one row per recipient fold, in order.
func (w *ActFoldWatch) Folds() []ActFoldRead {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]ActFoldRead(nil), w.folds...)
}

// WatchActFoldForTest watches every story read the live act fold makes until
// t ends: beside each it folds the member's whole story the way the commit
// did before rpg-project#891, and counts what each read returned. The test
// that calls it must not run in parallel: the watch is package-wide.
func WatchActFoldForTest(t testing.TB, sessionID string) *ActFoldWatch {
	t.Helper()
	w := &ActFoldWatch{}
	previous := actStory
	actStory = func(enc *encounter.Encounter, baseline uint64, member encounter.MemberID) ([]record.Entry, error) {
		out, err := previous(enc, baseline, member)
		if err != nil {
			return out, err
		}
		whole, wholeErr := enc.Story(&encounter.StoryInput{Audience: member})
		if wholeErr != nil {
			t.Errorf("whole story of %s: %v", member, wholeErr)
			return out, err
		}
		held := ""
		first := len(whole)
		for i, e := range whole {
			if e.Seq >= baseline {
				first = i
				break
			}
			held = e.ID
		}
		acts := map[string]bool{}
		for _, e := range whole[first:] {
			if tag := e.Tags[encounter.ActTag]; tag != "" {
				acts[tag] = true
			}
		}
		start := first
		for i, e := range whole[:first] {
			if acts[e.Tags[encounter.ActTag]] {
				start = i
				break
			}
		}
		w.mu.Lock()
		defer w.mu.Unlock()
		w.unbounded = append(w.unbounded, actsAfter(sessionID, string(member), whole, held)...)
		w.folds = append(w.folds, ActFoldRead{Member: string(member), Read: len(out), Whole: len(whole), Delta: len(whole) - first, Needed: len(whole) - start})
		return out, err
	}
	t.Cleanup(func() { actStory = previous })
	return w
}

// CountParticipationAttachesForTest counts what the standing seam hands
// resolution's participation entry until t ends: the assessments, and the
// monster sheets attached for them. Every assessment asks about its players first, in
// one call that carries no monster (possibly an empty one), so a call with no
// monster in it is one assessment. The test that calls it must not run in parallel: the
// count is package-wide.
func CountParticipationAttachesForTest(t testing.TB) func() (assessments, monsterSheets int) {
	t.Helper()
	var nAssessments, nSheets atomic.Int64
	previous := participationOf
	participationOf = func(ctx context.Context, in *resolution.ParticipationInput) (*resolution.ParticipationOutput, error) {
		monsters := 0
		if in != nil {
			for _, p := range in.Participants {
				if p.Monster != nil {
					monsters++
				}
			}
		}
		nSheets.Add(int64(monsters))
		if monsters == 0 {
			nAssessments.Add(1)
		}
		return previous(ctx, in)
	}
	t.Cleanup(func() { participationOf = previous })
	return func() (int, int) { return int(nAssessments.Load()), int(nSheets.Load()) }
}
