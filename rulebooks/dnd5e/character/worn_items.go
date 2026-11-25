// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"maps"
	"slices"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/magicitems"
)

// GrantedAbilityInput names one grant of one item the character owns.
type GrantedAbilityInput struct {
	ItemID string
	Grant  string
}

// GrantedAbilityOutput is what the gate needs: the description to run and the
// item's own budget to pay from.
type GrantedAbilityOutput struct {
	Definition *actions.Definition
	Budget     combat.PoolSource
}

// GrantedAbility answers for one grant of one WORN item: the description the
// gate runs, asked of the rulebook's named ability like a class feature's, and
// the item's own budget. Pay both in one payment:
//
//	combat.Pay(combat.WithPools(character, out.Budget), out.Definition.Cost)
//
// An item not owned, an item not worn and a grant the item does not have are
// refused, each naming the item. [Character.AbilityDefinition] is untouched:
// it answers for the character's own features only.
func (c *Character) GrantedAbility(in *GrantedAbilityInput) (*GrantedAbilityOutput, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "granted ability input is required")
	}
	item, ok := c.ownedItem(in.ItemID)
	if !ok {
		return nil, rpgerr.Newf(rpgerr.CodeNotFound, "item %q is not owned", in.ItemID)
	}
	if !c.isWorn(item.ID) {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidState, "item %q (%s) is not equipped", item.ID, item.Description.Name)
	}
	definition, err := itemDefinition(item, in.Grant)
	if err != nil {
		return nil, err
	}
	return &GrantedAbilityOutput{Definition: definition, Budget: &itemBudget{c: c, id: item.ID, pools: poolKeys(item)}}, nil
}

// isWorn reports whether an item's id is in an equipment slot.
func (c *Character) isWorn(id string) bool {
	for _, held := range c.equipmentSlots {
		if held == id {
			return true
		}
	}
	return false
}

// itemDefinition is the description of one grant, asked of the rulebook's
// named ability with the item as the giver and the grant's pool as its cost.
func itemDefinition(item magicitems.Data, grantName string) (*actions.Definition, error) {
	grant, ok := item.Description.Grants[grantName]
	if !ok {
		return nil, rpgerr.Newf(rpgerr.CodeNotFound, "item %q (%s) has no grant %q", item.ID, item.Description.Name, grantName)
	}
	ref := item.Description.Ref
	in := &features.DescribeFromRefInput{
		Ref:        grant.Ability,
		Parameters: slices.Clone(grant.Parameters),
		Giver: dnd5eEvents.RollSource{
			Ref:      &ref,
			Name:     item.Description.Name,
			Label:    item.Description.Name,
			SourceID: item.ID,
		},
	}
	if grant.Pool != "" {
		in.Pools = map[coreResources.ResourceKey]int{coreResources.ResourceKey(grant.Pool): grant.Uses}
	}
	definition, err := features.DescribeFromRef(in)
	if err != nil {
		return nil, rpgerr.Wrapf(err, "item %q (%s) grant %q", item.ID, item.Description.Name, grantName)
	}
	return definition, nil
}

func poolKeys(item magicitems.Data) map[coreResources.ResourceKey]string {
	keys := make(map[coreResources.ResourceKey]string, len(item.Description.Pools))
	for name := range item.Description.Pools {
		keys[coreResources.ResourceKey(name)] = name
	}
	return keys
}

// itemBudget is a worn item's own pools, read and debited on the character's
// live record of the item.
type itemBudget struct {
	c     *Character
	id    string
	pools map[coreResources.ResourceKey]string // key to the item's pool name
}

var _ combat.PoolSource = (*itemBudget)(nil)

func (b *itemBudget) HasPool(key coreResources.ResourceKey) bool {
	_, ok := b.pools[key]
	return ok
}

// index is the item's place in the live record, or -1 when it is no longer
// owned or no longer worn.
func (b *itemBudget) index() int {
	if !b.c.isWorn(b.id) {
		return -1
	}
	return slices.IndexFunc(b.c.items, func(i magicitems.Data) bool { return i.ID == b.id })
}

func (b *itemBudget) PoolLeft(key coreResources.ResourceKey) int {
	name, ok := b.pools[key]
	i := b.index()
	if !ok || i < 0 {
		return 0
	}
	return b.c.items[i].Left[name]
}

func (b *itemBudget) SpendPool(key coreResources.ResourceKey, n int) {
	name, ok := b.pools[key]
	i := b.index()
	if !ok || i < 0 || n <= 0 {
		return
	}
	b.c.items[i].Left[name] = max(0, b.c.items[i].Left[name]-n)
	b.c.poolChanged()
}

// wornItemAbilities lists one entry for each grant of each worn item, in item
// id then grant name order.
func (c *Character) wornItemAbilities() []AvailableAbility {
	var out []AvailableAbility
	for _, item := range slices.SortedFunc(slices.Values(c.items), func(a, b magicitems.Data) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	}) {
		if !c.isWorn(item.ID) {
			continue
		}
		for _, name := range slices.Sorted(maps.Keys(item.Description.Grants)) {
			out = append(out, c.grantedEntry(item, name))
		}
	}
	return out
}

func (c *Character) grantedEntry(item magicitems.Data, grantName string) AvailableAbility {
	grant := item.Description.Grants[grantName]
	ref := grant.Ability
	entry := AvailableAbility{
		Ref:        &ref,
		Name:       item.Description.Name,
		TargetKind: targetKindForRef(&ref),
		Giver:      &AbilityGiver{ItemID: item.ID, ItemName: item.Description.Name, Grant: grantName},
	}
	fail := func(err error) AvailableAbility {
		entry.Reason = err.Error()
		return entry
	}

	definition, err := itemDefinition(item, grantName)
	if err != nil {
		return fail(err)
	}
	entry.Name = definition.Name
	entry.Description = definition.Description

	var slot coreCombat.ActionType
	if definition.Cost == nil || len(definition.Cost.Slots) != 1 {
		return fail(rpgerr.Newf(rpgerr.CodeInvalidState,
			"item %q (%s) grant %q must cost exactly one slot", item.ID, item.Description.Name, grantName))
	}
	for s := range definition.Cost.Slots {
		slot = s
	}
	entry.ActionType = slot
	entry.EconomySlot = economySlotForActionType(slot)

	canUse := c.canUseAbilityByActionType(slot)
	reason := c.actionTypeExhaustedReason(slot)
	if grant.Pool != "" {
		entry.ResourceCurrent = item.Left[grant.Pool]
		entry.ResourceMax = item.Description.Pools[grant.Pool].Maximum
		if canUse && entry.ResourceCurrent < grant.Uses {
			canUse = false
			reason = "the " + item.Description.Name + " has no " + grant.Pool + " left"
		}
	}
	entry.CanUse = canUse
	entry.Reason = c.actionReason(canUse, reason)
	return entry
}

// restoreItemPools refills, for every item the character owns, worn or not,
// each pool whose restore_on lists a reset the rest satisfies.
func (c *Character) restoreItemPools(rest coreResources.ResetType) {
	for i := range c.items {
		for name, pool := range c.items[i].Description.Pools {
			if slices.ContainsFunc(pool.RestoreOn, func(reset coreResources.ResetType) bool {
				return restSatisfies(rest, reset)
			}) {
				c.items[i].Left[name] = pool.Maximum
			}
		}
	}
}
