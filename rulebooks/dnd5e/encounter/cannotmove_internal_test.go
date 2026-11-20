// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// cannotmove_internal_test.go is the two holds side by side (rpg-project#990):
// the gate's custody and a held effect stay separate, and a member both in
// custody and held always hears custody's refusal.

// holdingSheets answers inner's sheets with held named as holding each listed
// member.
type holdingSheets struct {
	inner Sheets
	held  map[MemberID][]string
}

func (h holdingSheets) Sheets(members []MemberID) (map[MemberID]SheetFacts, error) {
	out, err := h.inner.Sheets(members)
	if err != nil {
		return nil, err
	}
	for id, names := range h.held {
		if facts, ok := out[id]; ok {
			facts.CannotMove = names
			out[id] = facts
		}
	}
	return out, nil
}

// Catches the two refusals asked in the other order, or merged: in custody
// and held is custody's sentence; held alone is the hold's.
func TestCustodyIsAskedBeforeAHeldEffect(t *testing.T) {
	at := spatial.Position{X: 3, Y: 1}
	for _, tc := range []struct {
		custody CustodyPhase
		want    error
		not     error
	}{
		{CustodyHeld, ErrInCustody, ErrCannotMove},
		{CustodyBroken, ErrCannotMove, ErrInCustody},
	} {
		t.Run(string(tc.custody), func(t *testing.T) {
			enc := escapeSite(t, tc.custody, at)
			enc.sheets = holdingSheets{inner: enc.sheets, held: map[MemberID][]string{escPris: {"Held"}}}
			before := enc.members[escPris]
			from, ok := enc.canvas.GetEntityPosition(string(before.ID))
			require.True(t, ok)

			_, err := enc.Step(&StepInput{Member: escPris, To: spatial.Position{X: from.X, Y: from.Y + 1}, EndWalk: true})
			require.ErrorIs(t, err, tc.want)
			require.NotErrorIs(t, err, tc.not)
			now, _ := enc.canvas.GetEntityPosition(string(before.ID))
			require.Equal(t, from, now, "nobody moved")
		})
	}
}

// Catches a hold that silences its holder: a held member still speaks.
func TestAHeldMemberStillSpeaks(t *testing.T) {
	enc := escapeSite(t, CustodyBroken, spatial.Position{X: 3, Y: 1})
	enc.sheets = holdingSheets{inner: enc.sheets, held: map[MemberID][]string{escPris: {"Held"}}}
	enc.members[escGuard].Options = []Option{{ID: "hello", Say: "Hello.", Success: []Answer{{Weight: 1}}}}

	_, err := enc.Speak(context.Background(), &SpeakInput{
		Actor: escPris, Target: escGuard, OptionID: "hello", Range: 3, Roller: lowestDie{},
	})
	require.NoError(t, err)
}
