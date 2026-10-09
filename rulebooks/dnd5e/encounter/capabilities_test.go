// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// TestCapabilitiesValidateRefusesEachMemberInOrder pins the one presence check:
// starting from nothing, each member supplied in the documented order moves the
// refusal to the next member's sentinel, and a fully supplied value passes.
// Roller, CheckResolver and Witness are not part of the check.
func TestCapabilitiesValidateRefusesEachMemberInOrder(t *testing.T) {
	full := encounter.RefusingCapabilities()

	var built encounter.Capabilities
	steps := []struct {
		supply func()
		next   error
	}{
		{func() {}, encounter.ErrNoInitiative},
		{func() { built.Initiative = full.Initiative }, encounter.ErrNoStanding},
		{func() { built.Standing = full.Standing }, encounter.ErrNoSight},
		{func() { built.Sight = full.Sight }, encounter.ErrNoEquipment},
		{func() { built.Equipment = full.Equipment }, encounter.ErrNoSheets},
		{func() { built.Sheets = full.Sheets }, encounter.ErrNoTurnDriver},
		{func() { built.Driver = full.Driver }, encounter.ErrNoStriker},
		{func() { built.Striker = full.Striker }, encounter.ErrNoMover},
		{func() { built.Mover = full.Mover }, encounter.ErrNoAnnouncer},
	}
	for _, step := range steps {
		step.supply()
		require.ErrorIs(t, built.Validate(), step.next)
	}
	built.Announcer = full.Announcer
	require.NoError(t, built.Validate(), "Roller, CheckResolver and Witness are not checked here")
}

// TestCapabilitiesValidateNamesTheMissingMember removes each required member
// from an otherwise complete value in turn and expects exactly that member's
// sentinel.
func TestCapabilitiesValidateNamesTheMissingMember(t *testing.T) {
	cases := []struct {
		name   string
		remove func(*encounter.Capabilities)
		want   error
	}{
		{"initiative", func(c *encounter.Capabilities) { c.Initiative = nil }, encounter.ErrNoInitiative},
		{"standing", func(c *encounter.Capabilities) { c.Standing = nil }, encounter.ErrNoStanding},
		{"sight", func(c *encounter.Capabilities) { c.Sight = nil }, encounter.ErrNoSight},
		{"equipment", func(c *encounter.Capabilities) { c.Equipment = nil }, encounter.ErrNoEquipment},
		{"sheets", func(c *encounter.Capabilities) { c.Sheets = nil }, encounter.ErrNoSheets},
		{"driver", func(c *encounter.Capabilities) { c.Driver = nil }, encounter.ErrNoTurnDriver},
		{"striker", func(c *encounter.Capabilities) { c.Striker = nil }, encounter.ErrNoStriker},
		{"mover", func(c *encounter.Capabilities) { c.Mover = nil }, encounter.ErrNoMover},
		{"announcer", func(c *encounter.Capabilities) { c.Announcer = nil }, encounter.ErrNoAnnouncer},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			caps := encounter.RefusingCapabilities()
			tc.remove(&caps)
			require.ErrorIs(t, caps.Validate(), tc.want)

			setup := emptyWorld()
			setup.Capabilities = caps
			_, err := encounter.NewEncounter(setup)
			require.ErrorIs(t, err, tc.want, "Setup refuses with the same sentinel")

			built, err := encounter.NewEncounter(emptyWorld())
			require.NoError(t, err)
			_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: built.ToData(), Capabilities: caps})
			require.ErrorIs(t, err, tc.want, "Load refuses with the same sentinel")
		})
	}
}

// TestAConcealedFieldRefusesAMissingCheckResolverOrWitness: the concealment
// pair is checked where the field is known, and only when it declares one.
func TestAConcealedFieldRefusesAMissingCheckResolverOrWitness(t *testing.T) {
	endings := []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}
	concealed := func(mutate func(*encounter.Capabilities)) *encounter.SetupInput {
		caps := encounter.RefusingCapabilities()
		mutate(&caps)
		return &encounter.SetupInput{Field: concealField(), Endings: endings, Capabilities: caps}
	}

	_, err := encounter.NewEncounter(concealed(func(c *encounter.Capabilities) { c.CheckResolver = nil }))
	require.ErrorIs(t, err, encounter.ErrNoCheckResolver)
	_, err = encounter.NewEncounter(concealed(func(c *encounter.Capabilities) { c.Witness = nil }))
	require.ErrorIs(t, err, encounter.ErrNoWitness)
	_, err = encounter.NewEncounter(concealed(func(c *encounter.Capabilities) { c.CheckResolver, c.Witness = nil, nil }))
	require.ErrorIs(t, err, encounter.ErrNoCheckResolver, "the resolver is refused before the witness")
	built, err := encounter.NewEncounter(concealed(func(*encounter.Capabilities) {}))
	require.NoError(t, err)

	load := func(mutate func(*encounter.Capabilities)) error {
		caps := encounter.RefusingCapabilities()
		mutate(&caps)
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: built.ToData(), Capabilities: caps})
		return err
	}
	require.ErrorIs(t, load(func(c *encounter.Capabilities) { c.CheckResolver = nil }), encounter.ErrNoCheckResolver)
	require.ErrorIs(t, load(func(c *encounter.Capabilities) { c.Witness = nil }), encounter.ErrNoWitness)
	require.ErrorIs(t, load(func(c *encounter.Capabilities) { c.CheckResolver, c.Witness = nil, nil }), encounter.ErrNoCheckResolver)

	plain := emptyWorld()
	plain.CheckResolver, plain.Witness = nil, nil
	plainBuilt, err := encounter.NewEncounter(plain)
	require.NoError(t, err, "a plain field needs neither")
	caps := encounter.RefusingCapabilities()
	caps.CheckResolver, caps.Witness = nil, nil
	_, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: plainBuilt.ToData(), Capabilities: caps})
	require.NoError(t, err, "a plain field loads without either")
}

// TestRefusingCapabilitiesRoundTripAndRefuseByName: a world built with the
// refusing value constructs, round-trips through ToData and LoadEncounter with
// the refusing value, and every member that answers an ask a world nobody is in
// never makes refuses by name.
func TestRefusingCapabilitiesRoundTripAndRefuseByName(t *testing.T) {
	built, err := encounter.NewEncounter(emptyWorld())
	require.NoError(t, err)
	loaded, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: built.ToData(), Capabilities: encounter.RefusingCapabilities()})
	require.NoError(t, err)
	require.Equal(t, built.ToData(), loaded.ToData())

	caps := encounter.RefusingCapabilities()
	require.Nil(t, caps.Roller)

	_, err = caps.Initiative.RollInitiative([]encounter.MemberID{alice})
	require.ErrorIs(t, err, encounter.ErrRefusingInitiative)
	_, err = caps.Driver.Act(encounter.MonsterView{})
	require.ErrorIs(t, err, encounter.ErrRefusingDriver)
	ctx := context.Background()
	require.ErrorIs(t, caps.Striker.Strike(ctx, built, alice, goblin, core.Ref{}), encounter.ErrRefusingStriker)
	require.ErrorIs(t, caps.Mover.Move(ctx, built, encounter.MoveStep{}), encounter.ErrRefusingMover)
	require.ErrorIs(t, caps.Announcer.Announce(ctx, built, nil), encounter.ErrRefusingAnnouncer)
	_, err = caps.CheckResolver.ResolveCheck(&encounter.ResolveCheckInput{})
	require.ErrorIs(t, err, encounter.ErrRefusingCheckResolver)
}
