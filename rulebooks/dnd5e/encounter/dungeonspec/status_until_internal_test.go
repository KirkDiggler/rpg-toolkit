// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

// status_until_internal_test.go holds the grammar's judgment of an `until`
// built in Go (rpg-project#1080): a good number is accepted, whether or not
// any YAML wrote it.

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func TestAGoBuiltUntilIsJudgedByItsNumber(t *testing.T) {
	refused := func(rounds int) map[string]string {
		got := map[string]string{}
		g := &grammar{statusIDs: map[string]bool{"held": true}, add: func(at, msg string) { got[at] = msg }}
		g.applyStatus("row.apply_status", ApplyStatusSpec{
			Status: "held", To: "speaker", Until: &StatusUntilSpec{Rounds: rounds},
		}, false)
		return got
	}
	require.Empty(t, refused(3), "a good number built in code is not refused")
	require.Equal(t, map[string]string{"row.apply_status.until.rounds": encounter.StatusUntilRefusal(0)}, refused(0),
		"zero is refused with the rounds sentence, at the rounds path")
}
