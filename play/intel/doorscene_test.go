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
	require.NoError(t, err, "hearing testimony: Report succeeds")
	require.Equal(t, []intel.Report{
		{Subject: behindDoor3, Payload: []byte(crashingSound)},
	}, reportOut.FirstContact, "hearing testimony: behind-door-3 lands FirstContact")
	require.Empty(t, reportOut.Updated, "hearing testimony: no Updated yet")

	// Verify holding: Held status (no CurrentVia), payload, channel, at
	h1, err := intelContainer.On(&intel.OnInput{Observer: alice, Subject: behindDoor3})
	require.NoError(t, err, "hearing testimony: On() succeeds")
	require.Equal(t, []byte(crashingSound), h1.Payload, "hearing testimony: payload is crashing")
	require.Equal(t, hearing, h1.Channel, "hearing testimony: Channel is hearing")
	require.Equal(t, uint64(1), h1.At, "hearing testimony: At timestamp is 1")
	require.Nil(t, h1.CurrentVia, "hearing testimony: no CurrentVia (Report holds)")
	require.Equal(t, intel.Held, h1.Status, "hearing testimony: Status is Held")

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
	require.NoError(t, err, "sight elsewhere: Surveil succeeds")
	require.Equal(t, []intel.Report{
		{Subject: goblin, Payload: []byte(goblinNear)},
	}, surveilOut.FirstContact, "sight elsewhere: goblin lands FirstContact")
	require.Empty(t, surveilOut.Refreshed, "sight elsewhere: no Refreshed yet")
	require.Empty(t, surveilOut.Faded, "sight elsewhere: no Faded yet")

	// Verify goblin holding: Current status, sight channel, CurrentVia = [sight]
	hGoblin, err := intelContainer.On(&intel.OnInput{Observer: alice, Subject: goblin})
	require.NoError(t, err, "sight elsewhere: On(goblin) succeeds")
	require.Equal(t, []byte(goblinNear), hGoblin.Payload, "sight elsewhere: goblin payload is goblin-near")
	require.Equal(t, sight, hGoblin.Channel, "sight elsewhere: goblin Channel is sight")
	require.Equal(t, uint64(2), hGoblin.At, "sight elsewhere: goblin At is 2")
	require.Equal(t, []intel.Channel{sight}, hGoblin.CurrentVia, "sight elsewhere: goblin CurrentVia is [sight]")
	require.Equal(t, intel.Current, hGoblin.Status, "sight elsewhere: goblin Status is Current")

	// Verify behind-door-3 still Held (unchanged)
	h2, err := intelContainer.On(&intel.OnInput{Observer: alice, Subject: behindDoor3})
	require.NoError(t, err, "sight elsewhere: On(behind-door-3) succeeds")
	require.Equal(t, []byte(crashingSound), h2.Payload, "sight elsewhere: behind-door-3 payload unchanged (crashing)")
	require.Equal(t, hearing, h2.Channel, "sight elsewhere: behind-door-3 Channel unchanged (hearing)")
	require.Equal(t, uint64(1), h2.At, "sight elsewhere: behind-door-3 At unchanged (1)")
	require.Nil(t, h2.CurrentVia, "sight elsewhere: behind-door-3 CurrentVia unchanged (nil)")
	require.Equal(t, intel.Held, h2.Status, "sight elsewhere: behind-door-3 Status unchanged (Held)")

	// ===== STEP 3: The goblin leaves =====
	// Surveil sight channel with empty percept (nothing visible).
	// Expected: Faded contains goblin, goblin becomes ghost (Held, last observation)
	surveilOut, err = intelContainer.Surveil(&intel.SurveilInput{
		Observer: alice,
		Channel:  sight,
		Percept:  []intel.Report{},
		At:       3,
	})
	require.NoError(t, err, "goblin fades: Surveil succeeds")
	require.Empty(t, surveilOut.FirstContact, "goblin fades: no FirstContact")
	require.Empty(t, surveilOut.Refreshed, "goblin fades: no Refreshed")
	require.Equal(t, []intel.Subject{goblin}, surveilOut.Faded, "goblin fades: goblin in Faded")

	// Verify ghost-goblin: Held status, payload still "goblin-near", Channel still sight
	hGoblinGhost, err := intelContainer.On(&intel.OnInput{Observer: alice, Subject: goblin})
	require.NoError(t, err, "goblin fades: On(goblin) succeeds")
	require.Equal(t, []byte(goblinNear), hGoblinGhost.Payload, "the ghost goblin: payload held (goblin-near)")
	require.Equal(t, sight, hGoblinGhost.Channel, "the ghost goblin: Channel held (sight)")
	require.Equal(t, uint64(2), hGoblinGhost.At, "the ghost goblin: At held at last observation (2)")
	require.Nil(t, hGoblinGhost.CurrentVia, "the ghost goblin: CurrentVia nil (no sustaining channels)")
	require.Equal(t, intel.Held, hGoblinGhost.Status, "the ghost goblin: held at last observation")

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
	require.NoError(t, err, "door opens: Surveil succeeds")
	require.Empty(t, surveilOut.FirstContact, "door opens: no FirstContact (already held)")
	require.Equal(t, []intel.Subject{behindDoor3}, surveilOut.Refreshed, "door opens: behind-door-3 in Refreshed")
	require.Empty(t, surveilOut.Faded, "door opens: no Faded")

	// Verify behind-door-3: Current status now, payload updated, Channel sight
	hDoorOpen, err := intelContainer.On(&intel.OnInput{Observer: alice, Subject: behindDoor3})
	require.NoError(t, err, "door opens: On(behind-door-3) succeeds")
	require.Equal(t, []byte(potsFloor), hDoorOpen.Payload, "door opens: payload refreshed (pots, floor)")
	require.Equal(t, sight, hDoorOpen.Channel, "door opens: hearing-holding overwritten by sight")
	require.Equal(t, uint64(4), hDoorOpen.At, "door opens: At updated to 4")
	require.Equal(t, []intel.Channel{sight}, hDoorOpen.CurrentVia, "door opens: CurrentVia now [sight]")
	require.Equal(t, intel.Current, hDoorOpen.Status, "door opens: Status now Current")

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
	require.NoError(t, err, "the charm: Report succeeds")
	require.Equal(t, []intel.Report{
		{Subject: theStranger, Payload: []byte(trustedFriend)},
	}, reportOut.FirstContact, "the charm: the-stranger lands FirstContact")
	require.Empty(t, reportOut.Updated, "the charm: no Updated")

	// Verify charm-holding: Held status (Report has empty CurrentVia),
	// payload is the false belief, Channel is charm
	hCharm, err := intelContainer.On(&intel.OnInput{Observer: alice, Subject: theStranger})
	require.NoError(t, err, "the charm: On(the-stranger) succeeds")
	require.Equal(t, []byte(trustedFriend), hCharm.Payload, "charm plants false belief: payload held faithfully")
	require.Equal(t, charm, hCharm.Channel, "charm plants false belief: Channel is charm")
	require.Equal(t, uint64(5), hCharm.At, "charm plants false belief: At is 5")
	require.Nil(t, hCharm.CurrentVia, "charm plants false belief: no CurrentVia (Report holds)")
	require.Equal(t, intel.Held, hCharm.Status, "charm plants false belief: Status is Held")

	// ===== STEP 6: Final ledger =====
	// HeldBy returns all holdings in sorted-by-Subject order.
	// Expected order: behind-door-3, goblin, the-stranger
	holdings, err := intelContainer.HeldBy(&intel.HeldByInput{Observer: alice})
	require.NoError(t, err, "final ledger: HeldBy succeeds")

	require.Len(t, holdings, 3, "final ledger: three holdings")

	// behind-door-3: Current, sight channel, "pots, floor"
	require.Equal(t, behindDoor3, holdings[0].Subject, "final ledger: behind-door-3 is first")
	require.Equal(t, []byte(potsFloor), holdings[0].Payload, "final ledger: behind-door-3 payload (pots, floor)")
	require.Equal(t, sight, holdings[0].Channel, "final ledger: behind-door-3 Channel (sight)")
	require.Equal(t, uint64(4), holdings[0].At, "final ledger: behind-door-3 At (4)")
	require.Equal(t, []intel.Channel{sight}, holdings[0].CurrentVia, "final ledger: behind-door-3 CurrentVia ([sight])")
	require.Equal(t, intel.Current, holdings[0].Status, "final ledger: behind-door-3 Status (Current)")

	// goblin: Held ghost, sight channel, "goblin-near"
	require.Equal(t, goblin, holdings[1].Subject, "final ledger: goblin is second")
	require.Equal(t, []byte(goblinNear), holdings[1].Payload, "final ledger: goblin payload (goblin-near)")
	require.Equal(t, sight, holdings[1].Channel, "final ledger: goblin Channel (sight)")
	require.Equal(t, uint64(2), holdings[1].At, "final ledger: goblin At (2)")
	require.Nil(t, holdings[1].CurrentVia, "final ledger: goblin ghost CurrentVia (nil)")
	require.Equal(t, intel.Held, holdings[1].Status, "final ledger: goblin Status (Held)")

	// the-stranger: Held, charm channel, "trusted-friend"
	require.Equal(t, theStranger, holdings[2].Subject, "final ledger: the-stranger is third")
	require.Equal(t, []byte(trustedFriend), holdings[2].Payload, "final ledger: the-stranger payload (trusted-friend)")
	require.Equal(t, charm, holdings[2].Channel, "final ledger: the-stranger Channel (charm)")
	require.Equal(t, uint64(5), holdings[2].At, "final ledger: the-stranger At (5)")
	require.Nil(t, holdings[2].CurrentVia, "final ledger: the-stranger CurrentVia (nil)")
	require.Equal(t, intel.Held, holdings[2].Status, "final ledger: the-stranger Status (Held)")
}
