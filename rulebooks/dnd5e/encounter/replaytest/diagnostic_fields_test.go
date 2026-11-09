// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package replaytest_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// A field named "on" in another record kind is not a player's sneak toggle.
// Diagnostics remain on the record for the full replay comparison, but must
// never be decoded as player commands or prevent a later fall being found.
func TestReplayDecoderKeepsDiagnosticFieldsOpaque(t *testing.T) {
	entries := []record.EntryData{
		{Seq: 1, Payload: []byte(`{"beat":"behavior_decision","member":"guard","on":"heard"}`)},
		{Seq: 2, Payload: []byte(`{"beat":"down","member":"guard"}`)},
		{Seq: 3, Payload: []byte(`{"beat":"sneaking","member":"alice","on":true}`)},
	}
	players := map[encounter.MemberID]bool{"alice": true}
	verbs := verbsOf(t, entries, players)
	require.Len(t, verbs, 1)
	require.Equal(t, []encounter.MemberID{"guard"}, fallsAfter(t, entries, players))
}
