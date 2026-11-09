// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
)

// BenchmarkTraceStep measures the opt-in explanation cost on the same recipe.
func BenchmarkTraceStep(b *testing.B) {
	p := mustCompile(twoOnTime(fact("lit", machine.OpEq, true)))
	blob := p.NewBlob()
	facts := env{"lit": true}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := p.TraceStep(&machine.TraceStepInput{Blob: &blob, On: beatTime, Env: facts, Now: 1}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkOrdinaryStep uses an applying guard with no state transition, so
// repeated steps exercise the same real recipe without reset work in the loop.
func BenchmarkOrdinaryStep(b *testing.B) {
	p := mustCompile(twoOnTime(fact("lit", machine.OpEq, true)))
	blob := p.NewBlob()
	facts := env{"lit": true}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := p.Step(&blob, beatTime, nil, facts, 1); err != nil {
			b.Fatal(err)
		}
	}
}
