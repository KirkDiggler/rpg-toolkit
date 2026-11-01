// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
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
