// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/binary"
)

// fixedIDs is a deterministic record.IDSource (the composition holds no
// randomness, C8): a clock that never moves and entropy that counts. Two
// encounters built the same way with their own fixedIDs mint the same ids,
// which is what a test comparing two runs byte for byte, or pinning a golden,
// needs.
type fixedIDs struct {
	n uint64
}

func newFixedIDs() *fixedIDs { return &fixedIDs{} }

func (f *fixedIDs) NowMillis() uint64 { return 1_700_000_000_000 }

func (f *fixedIDs) Entropy(p []byte) error {
	for i := range p {
		if i%8 == 0 {
			f.n++
		}
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], f.n*0x9E3779B97F4A7C15)
		p[i] = b[i%8]
	}
	return nil
}
