// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAnUnrecognisedBeatStaysUnknown pins the mapper's default arm.
//
// Pinned HERE, against kindOf directly, rather than through a verb — because no
// verb can produce it. Every beat the composition emits today has a case, which
// is the point of rpg-toolkit#1038; the arm exists for the beat a LATER
// composition adds, and there is no honest way to drive that from this side of
// the seam without faking the composition itself.
//
// It is load-bearing rather than defensive. Delivering an uninterpretable beat
// keeps a client's sequence gapless, so it can tell "I do not understand this"
// from "I missed something" — dropping it would manufacture a hole and send the
// client into a resync it never needed. That is also what lets a newer
// composition ship a beat this version has never heard of without older clients
// losing their place.
func TestAnUnrecognisedBeatStaysUnknown(t *testing.T) {
	require.Equal(t, EventUnknown, kindOf([]byte(`{"beat":"transmogrified"}`)),
		"a beat this version does not know is delivered, not dropped")
	require.Equal(t, EventUnknown, kindOf([]byte(`{"actor":"alice"}`)),
		"and so is a beat with no name at all")
	require.Equal(t, EventUnknown, kindOf([]byte(`not json`)),
		"including a payload this package cannot even parse")
}

// TestTheAttackBeatsAreTheCompositionsOwnStrings guards the coupling from the
// other side.
//
// The seam-level pins in attackevents_test.go drive a real swing and are the
// evidence that matters; this states the two strings plainly, so that a diff
// renaming one of them has to say so out loud rather than degrading a table to
// unknown silently.
func TestTheAttackBeatsAreTheCompositionsOwnStrings(t *testing.T) {
	require.Equal(t, EventStruck, kindOf([]byte(`{"beat":"struck"}`)))
	require.Equal(t, EventMissed, kindOf([]byte(`{"beat":"missed"}`)))
}
