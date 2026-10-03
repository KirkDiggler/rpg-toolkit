// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

const (
	wantRagingDetail      = "Adds your rage damage bonus to melee weapon attacks that use Strength. Also grants advantage on Strength-based skill checks and Strength saving throws, and resistance to bludgeoning, piercing, and slashing damage."
	wantSneakAttackDetail = "Once per turn, adds extra damage to a qualifying attack when you have advantage or another enemy of the target is within 5 feet."
	wantBlessedDetail     = "Adds 1d4 to attack rolls and saving throws. Multiple Bless effects do not add extra dice to the same roll."
)

func TestDisplayForKnownConditions(t *testing.T) {
	cases := []struct {
		name string
		ref  core.Ref
		want Display
	}{
		{"fighting style defense", *refs.Conditions.FightingStyleDefense(), Display{Name: "Defense"}},
		{"raging", *refs.Conditions.Raging(), Display{Name: "Raging", Detail: wantRagingDetail}},
		{"martial arts", *refs.Conditions.MartialArts(), Display{Name: "Martial Arts"}},
		{"unarmored defense", *refs.Conditions.UnarmoredDefense(), Display{Name: "Unarmored Defense"}},
		{"unarmored movement", *refs.Conditions.UnarmoredMovement(), Display{Name: "Unarmored Movement"}},
		{"sneak attack (feature ref)", *refs.Features.SneakAttack(), Display{Name: "Sneak Attack", Detail: wantSneakAttackDetail}},
		{"brutal critical", *refs.Conditions.BrutalCritical(), Display{Name: "Brutal Critical"}},
		{"improved critical", *refs.Conditions.ImprovedCritical(), Display{Name: "Improved Critical"}},
		{"reckless attack", *refs.Conditions.RecklessAttack(), Display{Name: "Reckless Attack"}},
		{"dodging", *refs.Conditions.Dodging(), Display{Name: "Dodging"}},
		{"disengaging", *refs.Conditions.Disengaging(), Display{Name: "Disengaging"}},
		{"hidden", *refs.Conditions.Hidden(), Display{Name: "Hidden"}},
		{"helped", *refs.Conditions.Helped(), Display{Name: "Helped"}},
		{"prone", *refs.Conditions.Prone(), Display{Name: "Prone"}},
		{"opportunity attack", *refs.Conditions.OpportunityAttack(), Display{Name: "Opportunity Attack"}},
		{"unconscious", *refs.Conditions.Unconscious(), Display{Name: "Unconscious"}},
		{"guided", *refs.Conditions.Guided(), Display{Name: GuidedName}},
		{"resistance", *refs.Conditions.Resistance(), Display{Name: ResistanceName}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := DisplayFor(tc.ref)
			assert.True(t, ok, "known ref must be in the catalog")
			assert.Equal(t, tc.want, got)
			assert.NotEmpty(t, got.Name, "name is never empty for a known ref")
		})
	}
}

func TestDisplayForAllFightingStyles(t *testing.T) {
	for _, ref := range []*core.Ref{
		refs.Conditions.FightingStyleArchery(),
		refs.Conditions.FightingStyleDefense(),
		refs.Conditions.FightingStyleDueling(),
		refs.Conditions.FightingStyleGreatWeaponFighting(),
		refs.Conditions.FightingStyleProtection(),
		refs.Conditions.FightingStyleTwoWeaponFighting(),
	} {
		_, ok := DisplayFor(*ref)
		assert.True(t, ok, "fighting style %s must be in the catalog", ref.String())
	}
}

func TestDisplayForUnknownRefReturnsFalse(t *testing.T) {
	unknown := core.Ref{Module: refs.Module, Type: refs.TypeConditions, ID: "totally_unknown"}
	_, ok := DisplayFor(unknown)
	assert.False(t, ok, "unknown ref must not be in the catalog")
}

// TestDisplayCatalogExcludesShieldSpell confirms the no-magic boundary: the
// existing Shield spell condition is deliberately not promoted into the
// status-view catalog.
func TestDisplayCatalogExcludesShieldSpell(t *testing.T) {
	_, ok := DisplayFor(*refs.Spells.Shield())
	assert.False(t, ok, "Shield spell is not a status-view condition")
}

type effectTooltipContentSuite struct {
	suite.Suite
}

func TestEffectTooltipContentSuite(t *testing.T) {
	suite.Run(t, new(effectTooltipContentSuite))
}

func (s *effectTooltipContentSuite) TestCanonicalDescriptions() {
	for _, tc := range []struct {
		ref    *core.Ref
		name   string
		detail string
	}{
		{refs.Features.SneakAttack(), "Sneak Attack", wantSneakAttackDetail},
		{refs.Conditions.Raging(), "Raging", wantRagingDetail},
		{refs.Conditions.Blessed(), "Blessed", wantBlessedDetail},
	} {
		s.Run(tc.name, func() {
			display, found := DisplayFor(*tc.ref)
			s.Require().True(found)
			s.Equal(tc.name, display.Name)
			s.NotEmpty(display.Detail, "a name alone cannot explain the effect in a tooltip")
			s.Equal(tc.detail, display.Detail)
		})
	}
}
