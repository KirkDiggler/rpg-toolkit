// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package machine

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// Blob is what one machine persists: its current state, the bag keys that
// differ from their declared zero, its wake and its anchor. It names no
// recipe: the host keeps which program a blob runs. An empty State is the
// program's initial state, so a creature loaded before it had a machine
// runs from the start.
//
//   - WakeAt is the round the machine asked to be woken (a WakeIn act), or
//     nil; DueAt answers it before anything else.
//   - Anchor is the round an Anchor act stamped, or nil; a rounds leaf
//     counts from it.
type Blob struct {
	State  string  `json:"state"`
	Bag    Bag     `json:"bag,omitzero"`
	WakeAt *uint64 `json:"wake_at,omitzero"`
	Anchor *uint64 `json:"anchor,omitzero"`
}

// NewBlob returns a blob in the program's initial state, with an empty bag
// (every key at its declared zero), no wake and no anchor.
func (p *Program) NewBlob() Blob {
	return Blob{State: p.initial}
}

// UnmarshalJSONFrom decodes a blob, landing whole JSON numbers in the bag as
// int so they match the recipe's int keys. It returns the decoder's error
// for malformed JSON.
func (b *Blob) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	type plain Blob
	return json.UnmarshalDecode(dec, (*plain)(b), wholeNumbers)
}
