// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAWorldNPCInteractedStillFoldsToNothing: a world NPC's `interacted`
// beat has no `view` (rpg-toolkit#1404) and stays the record's, as it was
// before a thing done to a prop gave the beat a family (rpg-project#959): an
// old record replays as before. A view this build does not know is
// unreadable, never guessed.
func TestAWorldNPCInteractedStillFoldsToNothing(t *testing.T) {
	require.Empty(t, piecesOf("alice", "interact", []byte(`{"beat":"interacted","actor":"alice","target":"vendor"}`)))

	pieces := piecesOf("alice", "interact", []byte(`{"beat":"interacted","view":"sideways","actor":"alice"}`))
	require.Len(t, pieces, 1)
	require.NotEmpty(t, pieces[0].undecoded)
}

// TestAThingRefusedSpeaksToItsDoer: each refusal a doer can meet is told in
// the second person, keeping its sentinel; one that already speaks, or a
// defect, is returned as it came.
func TestAThingRefusedSpeaksToItsDoer(t *testing.T) {
	for sentinel, sentence := range map[error]string{
		ErrStaleOption:      "That has changed since you looked; look again.",
		ErrLineReserved:     "That waits on somebody's roll.",
		ErrNoProp:           "You see no such thing here.",
		ErrNoOption:         "That is not something you can do here.",
		ErrStaleDeclaration: "You cannot reach that from here.",
		ErrOutOfRange:       "You cannot reach that from here.",
	} {
		got := thingRefused(fmt.Errorf("interact: %w", sentinel))
		var refusal *ThingRefusal
		require.ErrorAs(t, got, &refusal)
		require.ErrorIs(t, got, sentinel)
		require.Equal(t, sentence, refusal.Sentence)
		require.Equal(t, sentence, got.Error())
	}
	require.ErrorIs(t, thingRefused(ErrNoRoom), ErrNoRoom)
	var refusal *ThingRefusal
	require.False(t, errors.As(thingRefused(ErrNoRoom), &refusal), "a refusal that already speaks is not reworded")
}
