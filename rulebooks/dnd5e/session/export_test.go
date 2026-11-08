// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"sync/atomic"
	"testing"

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
