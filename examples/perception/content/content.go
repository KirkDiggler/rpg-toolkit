// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package content is the rulebook's payload vocabulary — the one thing in this
// spike that knows what a percept MEANS.
//
// It exists to make a point about layering. Package testimony stores payloads
// as opaque bytes and can never read them, which is what makes it incapable of
// telling a lie from the truth. But opacity is a property of the STORE, not of
// everyone: the projection encodes these, and a reconciler decodes them,
// because both live where the rulebook lives.
//
// If this package's types ever appear in a testimony signature, the boundary
// has been broken.
package content

import "encoding/json"

// Kind is what an observer would say they perceived.
type Kind string

const (
	// Creature is a thing that acts.
	Creature Kind = "creature"
	// Trace is evidence of a thing that is not here — footprints, a smell, a
	// scuff on the floor. Its Of names what the observer would say left it.
	Trace Kind = "trace"
	// Noise is something heard and not placed.
	Noise Kind = "noise"
)

// Percept is what one channel says about one track at one moment.
//
// Of is the observer's own reading of what they are looking at, not an entity
// reference. Two observers may hold a different Of for the same track, and
// both are testimony.
type Percept struct {
	Kind Kind   `json:"kind"`
	Of   string `json:"of,omitempty"`
	Note string `json:"note,omitempty"`
}

// Encode renders a percept for carriage as an opaque payload.
func Encode(p Percept) []byte {
	out, err := json.Marshal(p)
	if err != nil {
		// The struct is closed and every field marshals; a failure here would
		// be a defect in this package, not a runtime condition.
		panic("content: encode: " + err.Error())
	}

	return out
}

// Decode reads a payload back. A payload this package did not write is not an
// error the store could have caught — only the rulebook can tell.
func Decode(payload []byte) (Percept, error) {
	var p Percept
	if err := json.Unmarshal(payload, &p); err != nil {
		return Percept{}, err
	}

	return p, nil
}

// Key renders the change key an emitter stamps on a percept: content equality,
// deliberately not byte equality. Byte equality on a payload carrying a
// timestamp is never true of itself, which is why the store never compares
// payloads and the emitter declares the key instead.
func Key(p Percept) string {
	return string(p.Kind) + "|" + p.Of + "|" + p.Note
}
