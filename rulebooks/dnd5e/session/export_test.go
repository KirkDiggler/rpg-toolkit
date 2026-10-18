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
