// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// BenchmarkAfford_CastleGatehouse is one Afford for alice walked into the
// gatehouse in free roam, with the captain and both gate guards in sight
// and out of reach: the read whose Attack candidates each name an approach
// cell (rpg-project#872), over one unbounded walk flood of the castle.
func BenchmarkAfford_CastleGatehouse(b *testing.B) {
	g := openTheGatehouse(14)
	if err := g.walkIn(); err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	in := &session.AffordInput{Session: "sess", Member: "alice"}
	for b.Loop() {
		if _, err := g.mgr.Afford(ctx, in); err != nil {
			b.Fatal(err)
		}
	}
}
