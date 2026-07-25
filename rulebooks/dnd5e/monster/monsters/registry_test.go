// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestByRef_EveryCanonicalMonsterResolves(t *testing.T) {
	// The 11 refs verified to have a real constructor today (see the
	// inventory comment on the byRef map in registry.go) — bandit/
	// bandit-archer resolve via NewBanditMelee/NewBanditRanged despite the
	// name mismatch; the other 4 canonical refs (skeleton-archer,
	// giant-spider, giant-wolf-spider, bandit-captain) have no constructor
	// and are deliberately absent.
	want := []string{
		refs.Monsters.Skeleton().String(),
		refs.Monsters.Zombie().String(),
		refs.Monsters.SkeletonCaptain().String(),
		refs.Monsters.Ghoul().String(),
		refs.Monsters.GiantRat().String(),
		refs.Monsters.Wolf().String(),
		refs.Monsters.BrownBear().String(),
		refs.Monsters.Bandit().String(),
		refs.Monsters.BanditArcher().String(),
		refs.Monsters.Thug().String(),
		refs.Monsters.Goblin().String(),
	}

	// Closes the "12th entry never gets mapping-checked" drift window: a
	// future addition to byRef that isn't also added to this list would
	// otherwise go untested by the loop below.
	assert.ElementsMatch(t, want, monsters.Refs())

	for _, ref := range want {
		ctor, ok := monsters.ByRef(ref)
		require.True(t, ok, "ref %q must resolve", ref)
		require.NotNil(t, ctor, "ref %q must have a constructor", ref)

		// Mapping-correctness: the constructor registered under this ref
		// must actually build a monster carrying that same ref. Without
		// this, swapping e.g. NewBanditMelee/NewBanditRanged between the
		// bandit/bandit-archer map entries would still pass every other
		// assertion in this test.
		require.Equal(t, ref, ctor("test-id").Ref().String(),
			"constructor registered under %q must build a monster with that ref", ref)
	}
}

func TestByRef_UnconstructedCanonicalRefReturnsFalse(t *testing.T) {
	// bandit-captain IS a canonical ref (refs.Monsters.BanditCaptain()) but
	// has no constructor today — distinct from an entirely fictional ref,
	// and worth its own case so a future constructor addition here doesn't
	// silently get missed by only testing a made-up name.
	ctor, ok := monsters.ByRef(refs.Monsters.BanditCaptain().String())
	assert.False(t, ok)
	assert.Nil(t, ctor)
}

func TestByRef_UnknownRefReturnsFalse(t *testing.T) {
	ctor, ok := monsters.ByRef("dnd5e:monsters:beholder")
	assert.False(t, ok)
	assert.Nil(t, ctor)
}
