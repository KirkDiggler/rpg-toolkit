// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"maps"
	"slices"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
)

// GrantedAbilityInput names one grant of the creature's kit.
type GrantedAbilityInput struct {
	// Grant is the name the kit files the ability under: "second-wind".
	Grant string
}

// GrantedAbilityOutput is what the gate needs: the description to run. Its
// cost names the creature's pool by [PoolKey], which this sheet's own ledger
// answers.
type GrantedAbilityOutput struct {
	Definition *actions.Definition
}

// Grants lists the names of what the creature's kit lets it use, sorted. A
// creature with no kit has none.
func (m *Monster) Grants() []string {
	if m.kit == nil {
		return nil
	}

	return slices.Sorted(maps.Keys(m.kit.Grants))
}

// GrantedAbility describes one grant of the creature's kit: the description the
// gate runs, asked of the rulebook's named ability with the creature as its
// giver. It is the creature's side of what character.GrantedAbility is for a
// worn item. When the grant names a pool, the cost names it by [PoolKey], so one
// call pays the slot and the use:
//
//	combat.Pay(creature, out.Definition.Cost)
//
// A creature with no kit, and a grant the kit does not have, are refused by name.
func (m *Monster) GrantedAbility(in *GrantedAbilityInput) (*GrantedAbilityOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "granted ability input is required")
	}
	if m.kit == nil {
		return nil, rpgerr.Newf(rpgerr.CodeNotFound, "stat block %q has no kit, so it has no grant %q", m.name, in.Grant)
	}
	grant, ok := m.kit.Grants[in.Grant]
	if !ok {
		return nil, rpgerr.Newf(rpgerr.CodeNotFound, "stat block %q has no grant %q", m.name, in.Grant)
	}
	var ref *core.Ref
	if m.ref != nil {
		own := *m.ref
		ref = &own
	}
	describe := &features.DescribeFromRefInput{
		Ref:        grant.Ability,
		Parameters: slices.Clone(grant.Parameters),
		Giver: dnd5eEvents.RollSource{
			Ref:      ref,
			Name:     m.name,
			Label:    m.name,
			SourceID: m.id,
		},
	}
	if grant.Pool != "" {
		describe.Pools = map[coreResources.ResourceKey]int{PoolKey(grant.Pool): grant.Uses}
	}
	definition, err := features.DescribeFromRef(describe)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "stat block %q grant %q", m.name, in.Grant)
	}

	return &GrantedAbilityOutput{Definition: definition}, nil
}
