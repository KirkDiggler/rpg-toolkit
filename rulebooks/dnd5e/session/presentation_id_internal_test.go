// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

type testPresentationIDs struct {
	value string
	calls *int
}

func (g testPresentationIDs) Generate() string {
	if g.calls != nil {
		*g.calls++
	}
	if g.value != "" {
		return g.value
	}
	return "presentation-test-id"
}
