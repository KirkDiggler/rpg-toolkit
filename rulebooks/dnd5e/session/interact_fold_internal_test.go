// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
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

// TestAWitnessNotShownTheThingIsToldThePlace keeps the act's Place part
// tested now that the castle's passage no longer stages one (rpg-project#959:
// whoever sees the hidden thing used comes to know it). The `place` version
// is the encounter's for a witness who sees the doer and not the thing's
// cell: it names where, and no thing. Merged at shared view with the doer's
// own version, it is the doer's, through them, with no place.
func TestAWitnessNotShownTheThingIsToldThePlace(t *testing.T) {
	place := []byte(`{"beat":"interacted","view":"place","actor":"alice","at":{"x":-11,"y":-14}}`)
	pieces := piecesOf("bob", "interact", place)
	require.Len(t, pieces, 1)
	at := spatial.Position{X: -11, Y: -14}
	require.Equal(t, InteractAct{Actor: member("alice"), Place: &at}, pieces[0].body)

	doer := []byte(`{"beat":"interacted","view":"actor","actor":"alice","prop":"grate","option":"to-chapel",` +
		`"do":"You lift the grate and take the way up.","beaten":true}`)
	merged, err := mergedInteraction([]utteranceVersion{{payload: place}, {payload: doer, via: []encounter.MemberID{"alice"}}})
	require.NoError(t, err)
	beaten := true
	require.Equal(t, InteractAct{
		Actor: member("alice"), Prop: prop("grate"), Option: "to-chapel",
		Words: "You lift the grate and take the way up.", Beaten: &beaten,
	}, merged.body, "the doer's version, and no place")
	require.Equal(t, []Party{member("alice")}, merged.perceived.Via)
}

// TestAHearerNotShownThePropHearsWordsToNoOne: since encounter v0.215.0 a
// spoken line's speech version names its prop only for a reader shown the
// prop (rpg-project#959); a hearer who never saw where it stands catches the
// words said to no one named, which this fold reads as it reads any line
// said to nobody it saw.
func TestAHearerNotShownThePropHearsWordsToNoOne(t *testing.T) {
	pieces := piecesOf("bob", "speak", []byte(`{"beat":"speech","utterance":"u9","speaker":"alice",`+
		`"words":"Lift it.","option":"open","place":{"x":1,"y":1},"loudness":"speak"}`))
	require.Len(t, pieces, 1)
	require.Empty(t, pieces[0].undecoded)
	act := pieces[0].body.(SpeechAct)
	require.Equal(t, Party{}, act.SpokenTo, "no prop named")
	require.NotNil(t, act.Words)
	require.Equal(t, "Lift it.", *act.Words)
}
