// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
)

// Every shipping stat block and what it is worth. The worth is the 2014 table's
// value for the challenge rating THAT BLOCK CLAIMS in its own doc comment, not
// for the rating the SRD gives a monster of that name — two blocks here differ
// and the comments beside them say so.
//
// What this proves is each named value. That a NEW constructor was given a
// worth at all is TestNoRegisteredMonsterIsWorthNothing's job, and it proves it
// for everything the engine can actually reach.
func TestEveryConstructorAuthorsItsWorth(t *testing.T) {
	cases := []struct {
		name       string
		build      func(string) *monster.Monster
		challenge  string
		experience int
	}{
		{name: "animated armor", build: monsters.NewAnimatedArmor, challenge: "1", experience: 200},
		{name: "bandit", build: monsters.NewBanditMelee, challenge: "1/8", experience: 25},
		{name: "bandit archer", build: monsters.NewBanditRanged, challenge: "1/8", experience: 25},
		{name: "brown bear", build: monsters.NewBrownBear, challenge: "1", experience: 200},
		{name: "ghoul", build: monsters.NewGhoul, challenge: "1", experience: 200},
		{name: "giant rat", build: monsters.NewGiantRat, challenge: "1/8", experience: 25},
		{name: "goblin", build: monsters.NewGoblin, challenge: "1/4", experience: 50},
		{name: "goblin boss", build: monsters.NewGoblinBoss, challenge: "1", experience: 200},
		{name: "skeleton", build: monsters.NewSkeleton, challenge: "1/4", experience: 50},
		{name: "skeleton captain", build: monsters.NewSkeletonCaptain, challenge: "2", experience: 450},
		// CR 1 is this file's claim; the SRD thug is CR 1/2 and worth 100.
		{name: "thug", build: monsters.NewThug, challenge: "1", experience: 200},
		{name: "wolf", build: monsters.NewWolf, challenge: "1/4", experience: 50},
		{name: "zombie", build: monsters.NewZombie, challenge: "1/4", experience: 50},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			built := tc.build("xp-" + tc.name)

			assert.Equal(t, tc.experience, built.Experience(),
				"a CR %s block is worth %d on the 2014 table", tc.challenge, tc.experience)
			assert.Equal(t, tc.experience, built.ToData().Experience,
				"and the authored worth reaches the persisted sheet")
		})
	}
}

// The registry is how the rest of the engine reaches these blocks, so a
// constructor that the registry can build but that nobody gave a worth would
// still hand the party nothing. Checking through ByRef rather than the
// functions above closes that door from the other side.
func TestNoRegisteredMonsterIsWorthNothing(t *testing.T) {
	registered := monsters.Refs()
	require.NotEmpty(t, registered)

	for _, ref := range registered {
		build, ok := monsters.ByRef(ref)
		require.True(t, ok, "ref %q must resolve", ref)

		assert.Positive(t, build("xp-registry").Experience(),
			"%q is reachable from the registry and must be worth something", ref)
	}
}
