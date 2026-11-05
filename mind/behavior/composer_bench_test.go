// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package behavior_test

import (
	"context"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
)

type compositionBenchDie struct{}

func (compositionBenchDie) Roll(context.Context, int) (int, error) { return 1, nil }

// BenchmarkComposerDecide measures two requesting roles with one selected flat
// row, including defensive copies. It excludes real RNG, geometry and execution;
// it is not a keep or multiplayer performance measurement.
func BenchmarkComposerDecide(b *testing.B) {
	config := compositionConfig(compositionSpec("higher", "higher", 1), compositionSpec("lower", "lower", 1))
	composer, err := behavior.NewComposer(&config)
	if err != nil {
		b.Fatal(err)
	}
	input := &behavior.DecideInput{On: composeTick, Die: compositionBenchDie{}}
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := composer.Decide(ctx, input); err != nil {
			b.Fatal(err)
		}
	}
}
