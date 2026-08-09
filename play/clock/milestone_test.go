// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package clock_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/stretchr/testify/assert"
)

func TestMilestoneKindVocabularyIsClosed(t *testing.T) {
	want := map[clock.MilestoneKind]string{
		clock.TurnStarted: "turn_started", clock.TurnEnded: "turn_ended",
		clock.RoundStarted: "round_started", clock.Ticked: "ticked",
		clock.MemberJoined: "member_joined", clock.MemberLeft: "member_left",
		clock.Merged: "merged", clock.Dissolved: "dissolved",
	}
	assert.Len(t, want, 8)
	for kind, raw := range want {
		assert.Equal(t, raw, string(kind))
	}
}
