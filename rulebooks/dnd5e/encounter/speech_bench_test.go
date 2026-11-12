// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// BenchmarkKeepUtterance is one creature's line on the 10k garrison keep
// (rpg-project#933): who sees the speaker, whose rooms the words reach, who
// notices the voice across the doors, every member's version written, and
// the voice landed on the sounds channel. Its cost is the roster's sight of
// the speaker plus the rooms within the loudness's bound — never the field.
func BenchmarkKeepUtterance(b *testing.B) {
	for _, loudness := range []string{"speak", "shout", "scream"} {
		b.Run(loudness, func(b *testing.B) {
			enc := openGarrisonKeep(b, false)
			speaker := firstMonster(b, enc)
			caught := 0
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				n, err := enc.ExportedUtter(speaker, "Who goes there?", loudness)
				if err != nil {
					b.Fatal(err)
				}
				caught = n
			}
			b.ReportMetric(float64(caught), "catchers")
		})
	}
}

// firstMonster is the roster's first monster, in member order.
func firstMonster(tb testing.TB, enc *encounter.Encounter) encounter.MemberID {
	tb.Helper()
	members, err := enc.Members()
	if err != nil {
		tb.Fatal(err)
	}
	for _, m := range members {
		if m.Kind == encounter.KindMonster {
			return m.ID
		}
	}
	tb.Fatal("the keep has no monster")
	return ""
}
