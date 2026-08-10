// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
)

// TestDoorScene scripts the AC1 design narrative: hearing testimony on a
// subject behind a door, a goblin appearing and fading as a ghost, the door
// opening to reveal the source, and a false belief planted by charm.
// Tests the full delta-transcript at each step and end-state holdings.
func TestDoorScene(t *testing.T) {
	const (
		alice         = core.EntityID("alice")
		behindDoor3   = intel.Subject("behind-door-3")
		goblin        = intel.Subject("goblin")
		theStranger   = intel.Subject("the-stranger")
		hearing       = intel.Channel("hearing")
		sight         = intel.Channel("sight")
		charm         = intel.Channel("charm")
		crashingSound = "crashing"
		goblinNear    = "goblin-near"
		potsFloor     = "pots, floor"
		trustedFriend = "trusted-friend"
	)

	// Create container
	intelContainer, err := intel.NewIntel()
	require.NoError(t, err)

	// ===== STEP 1: Hearing testimony =====
	// Report on subject "behind-door-3", channel "hearing", payload "crashing"
	// Expected: FirstContact contains behind-door-3, no Faded/Refreshed
	reportOut, err := intelContainer.Report(&intel.ReportInput{
		Observer: alice,
		Channel:  hearing,
		Reports: []intel.Report{
			{Subject: behindDoor3, Payload: []byte(crashingSound)},
		},
		At: 1,
	})
	require.NoError(t, err)
	require.Equal(t, []intel.Report{
		{Subject: behindDoor3, Payload: []byte(crashingSound)},
	}, reportOut.FirstContact)
	require.Empty(t, reportOut.Updated)

	// Verify holding: Held status (no CurrentVia), payload, channel, at
	h1, err := intelContainer.On(&intel.OnInput{Observer: alice, Subject: behindDoor3})
	require.NoError(t, err)
	require.Equal(t, []byte(crashingSound), h1.Payload)
	require.Equal(t, hearing, h1.Channel)
	require.Equal(t, uint64(1), h1.At)
	require.Nil(t, h1.CurrentVia)
	require.Equal(t, intel.Held, h1.Status)

	// ===== STEP 2: Sight elsewhere =====
	// Surveil sight channel with goblin percept (goblin-near).
	// Expected: FirstContact contains goblin, behind-door-3 untouched
	surveilOut, err := intelContainer.Surveil(&intel.SurveilInput{
		Observer: alice,
		Channel:  sight,
		Percept: []intel.Report{
			{Subject: goblin, Payload: []byte(goblinNear)},
		},
		At: 2,
	})
	require.NoError(t, err)
	require.Equal(t, []intel.Report{
		{Subject: goblin, Payload: []byte(goblinNear)},
	}, surveilOut.FirstContact)
	require.Empty(t, surveilOut.Refreshed)
	require.Empty(t, surveilOut.Faded)

	// Verify goblin holding: Current status, sight channel, CurrentVia = [sight]
	hGoblin, err := intelContainer.On(&intel.OnInput{Observer: alice, Subject: goblin})
	require.NoError(t, err)
	require.Equal(t, []byte(goblinNear), hGoblin.Payload)
	require.Equal(t, sight, hGoblin.Channel)
	require.Equal(t, uint64(2), hGoblin.At)
	require.Equal(t, []intel.Channel{sight}, hGoblin.CurrentVia)
	require.Equal(t, intel.Current, hGoblin.Status)

	// Verify behind-door-3 still Held (unchanged)
	h2, err := intelContainer.On(&intel.OnInput{Observer: alice, Subject: behindDoor3})
	require.NoError(t, err)
	require.Equal(t, []byte(crashingSound), h2.Payload)
	require.Equal(t, intel.Held, h2.Status)

	// ===== STEP 3: The goblin leaves =====
	// Surveil sight channel with empty percept (nothing visible).
	// Expected: Faded contains goblin, goblin becomes ghost (Held, last observation)
	surveilOut, err = intelContainer.Surveil(&intel.SurveilInput{
		Observer: alice,
		Channel:  sight,
		Percept:  []intel.Report{},
		At:       3,
	})
	require.NoError(t, err)
	require.Empty(t, surveilOut.FirstContact)
	require.Empty(t, surveilOut.Refreshed)
	require.Equal(t, []intel.Subject{goblin}, surveilOut.Faded)

	// Verify ghost-goblin: Held status, payload still "goblin-near", Channel still sight
	hGoblinGhost, err := intelContainer.On(&intel.OnInput{Observer: alice, Subject: goblin})
	require.NoError(t, err)
	require.Equal(t, []byte(goblinNear), hGoblinGhost.Payload)
	require.Equal(t, sight, hGoblinGhost.Channel)
	require.Nil(t, hGoblinGhost.CurrentVia) // Empty CurrentVia (no sustaining channels)
	require.Equal(t, intel.Held, hGoblinGhost.Status)

	// ===== STEP 4: The door opens =====
	// Surveil sight channel including behind-door-3 ("pots, floor").
	// The hearing-holding is overwritten because composition aimed the same subject.
	// Expected: Refreshed (not FirstContact) because behind-door-3 was already held;
	// Channel becomes sight, payload becomes "pots, floor", CurrentVia = [sight]
	surveilOut, err = intelContainer.Surveil(&intel.SurveilInput{
		Observer: alice,
		Channel:  sight,
		Percept: []intel.Report{
			{Subject: behindDoor3, Payload: []byte(potsFloor)},
		},
		At: 4,
	})
	require.NoError(t, err)
	require.Empty(t, surveilOut.FirstContact)
	require.Equal(t, []intel.Subject{behindDoor3}, surveilOut.Refreshed)
	require.Empty(t, surveilOut.Faded)

	// Verify behind-door-3: Current status now, payload updated, Channel sight
	hDoorOpen, err := intelContainer.On(&intel.OnInput{Observer: alice, Subject: behindDoor3})
	require.NoError(t, err)
	require.Equal(t, []byte(potsFloor), hDoorOpen.Payload)
	require.Equal(t, sight, hDoorOpen.Channel)
	require.Equal(t, uint64(4), hDoorOpen.At)
	require.Equal(t, []intel.Channel{sight}, hDoorOpen.CurrentVia)
	require.Equal(t, intel.Current, hDoorOpen.Status)

	// ===== STEP 5: The charm =====
	// Report on channel "charm", subject "the-stranger", payload "trusted-friend"
	// (a false belief — intel is truth-blind).
	// Expected: FirstContact, beliefs held faithfully
	reportOut, err = intelContainer.Report(&intel.ReportInput{
		Observer: alice,
		Channel:  charm,
		Reports: []intel.Report{
			{Subject: theStranger, Payload: []byte(trustedFriend)},
		},
		At: 5,
	})
	require.NoError(t, err)
	require.Equal(t, []intel.Report{
		{Subject: theStranger, Payload: []byte(trustedFriend)},
	}, reportOut.FirstContact)
	require.Empty(t, reportOut.Updated)

	// Verify charm-holding: Held status (Report has empty CurrentVia),
	// payload is the false belief, Channel is charm
	hCharm, err := intelContainer.On(&intel.OnInput{Observer: alice, Subject: theStranger})
	require.NoError(t, err)
	require.Equal(t, []byte(trustedFriend), hCharm.Payload)
	require.Equal(t, charm, hCharm.Channel)
	require.Equal(t, uint64(5), hCharm.At)
	require.Nil(t, hCharm.CurrentVia)
	require.Equal(t, intel.Held, hCharm.Status)

	// ===== STEP 6: Final ledger =====
	// HeldBy returns all holdings in sorted-by-Subject order.
	// Expected order: behind-door-3, goblin, the-stranger
	holdings, err := intelContainer.HeldBy(&intel.HeldByInput{Observer: alice})
	require.NoError(t, err)

	require.Len(t, holdings, 3)

	// behind-door-3: Current, sight channel, "pots, floor"
	require.Equal(t, behindDoor3, holdings[0].Subject)
	require.Equal(t, []byte(potsFloor), holdings[0].Payload)
	require.Equal(t, sight, holdings[0].Channel)
	require.Equal(t, []intel.Channel{sight}, holdings[0].CurrentVia)
	require.Equal(t, intel.Current, holdings[0].Status)

	// goblin: Held ghost, sight channel, "goblin-near"
	require.Equal(t, goblin, holdings[1].Subject)
	require.Equal(t, []byte(goblinNear), holdings[1].Payload)
	require.Equal(t, sight, holdings[1].Channel)
	require.Nil(t, holdings[1].CurrentVia)
	require.Equal(t, intel.Held, holdings[1].Status)

	// the-stranger: Held, charm channel, "trusted-friend"
	require.Equal(t, theStranger, holdings[2].Subject)
	require.Equal(t, []byte(trustedFriend), holdings[2].Payload)
	require.Equal(t, charm, holdings[2].Channel)
	require.Nil(t, holdings[2].CurrentVia)
	require.Equal(t, intel.Held, holdings[2].Status)
}
