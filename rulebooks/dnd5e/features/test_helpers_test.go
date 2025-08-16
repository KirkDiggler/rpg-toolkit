// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package features_test

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
)

// mockEntity is a test helper for creating entities
type mockEntity struct {
	id         string
	entityType core.EntityType
}

func (m *mockEntity) GetID() string            { return m.id }
func (m *mockEntity) GetType() core.EntityType { return m.entityType }