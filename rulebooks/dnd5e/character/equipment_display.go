// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"context"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/equipment"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// EquippedItemView is the display projection of one inventory item: its
// identity (Name, Kind — "weapon" | "shield" | "armor" | "gear"), the
// slots it MAY occupy (SlotKeys, from CompatibleSlots), the slot it
// currently occupies (Slot — empty if merely carried), and the
// server-composed stat_line the web renders verbatim.
type EquippedItemView struct {
	ItemID   string
	Name     string
	Kind     string
	SlotKeys []string
	Slot     InventorySlot
	StatLine string
}

// SlotDefView is one entry in the character's equip-slot taxonomy: which
// slots exist, their display label, and which item Kinds each accepts.
// Static for now (main hand / off hand / armor, mirroring
// rpg-dnd5e-web#557 fixtures.ts's MARTIAL_SLOTS) — a future class- or
// module-driven taxonomy plugs in here without changing the shape rpg-api
// consumes.
type SlotDefView struct {
	Key          string
	DisplayLabel string
	Accepts      []string
}

// equipSlotTaxonomy is the static slot taxonomy EquipmentView.Slots is
// cloned from. See SlotDefView's doc for why it's static today.
var equipSlotTaxonomy = []SlotDefView{
	{Key: string(SlotMainHand), DisplayLabel: "Main hand", Accepts: []string{"weapon"}},
	{Key: string(SlotOffHand), DisplayLabel: "Off hand", Accepts: []string{"weapon", "shield"}},
	{Key: string(SlotArmor), DisplayLabel: "Armor", Accepts: []string{"armor"}},
}

// cloneSlotTaxonomy deep-copies equipSlotTaxonomy — including each
// SlotDefView's Accepts slice — so a caller mutating one EquipmentView's
// Slots can never affect the shared package-level taxonomy (or a
// concurrent/later EquipmentView call).
func cloneSlotTaxonomy() []SlotDefView {
	view := make([]SlotDefView, len(equipSlotTaxonomy))
	for i, def := range equipSlotTaxonomy {
		accepts := make([]string, len(def.Accepts))
		copy(accepts, def.Accepts)
		view[i] = SlotDefView{Key: def.Key, DisplayLabel: def.DisplayLabel, Accepts: accepts}
	}
	return view
}

// EquipmentView is the display projection rpg-api needs to serve
// equipped/inventory items and armor class (rpg-toolkit#811) — every item
// the character owns, the slot taxonomy, the composed AC total and note,
// and the resolved main-hand damage display (contract §5). This is THE
// single projection rpg-api maps to the wire CharacterData: every display
// field comes from here, composed with zero rules knowledge required
// upstream.
type EquipmentView struct {
	Items          []EquippedItemView
	Slots          []SlotDefView
	ACTotal        int
	ACNote         string
	MainHandDamage string
}

// EquipmentView composes the display projection for this character's
// inventory and effective AC: identity + SlotKeys + stat_line per owned
// item (equipped or carried), the slot taxonomy, AC total + note, and the
// resolved main-hand damage display. Consumable by rpg-api with zero
// rules knowledge.
func (c *Character) EquipmentView(ctx context.Context) *EquipmentView {
	items := make([]EquippedItemView, 0, len(c.inventory))
	for _, invItem := range c.inventory {
		id := invItem.Equipment.EquipmentID()
		items = append(items, EquippedItemView{
			ItemID:   id,
			Name:     invItem.Equipment.EquipmentName(),
			Kind:     itemKind(invItem.Equipment),
			SlotKeys: slotKeys(CompatibleSlots(invItem.Equipment)),
			Slot:     c.equipmentSlots.slotFor(id),
			StatLine: StatLine(equipment.ResolveEquipmentDetail(id)),
		})
	}

	breakdown := c.EffectiveAC(ctx)
	mainHand := c.GetEquippedSlot(SlotMainHand)
	return &EquipmentView{
		Items:          items,
		Slots:          cloneSlotTaxonomy(),
		ACTotal:        breakdown.Total,
		ACNote:         ACNote(breakdown),
		MainHandDamage: MainHandDamage(mainHand.AsWeapon(), c.GetEquippedSlot(SlotOffHand)),
	}
}

// slotKeys converts CompatibleSlots' typed InventorySlots into the plain
// strings EquippedItemView.SlotKeys carries. Returns nil for no
// compatible slots (slotless gear).
func slotKeys(slots []InventorySlot) []string {
	if len(slots) == 0 {
		return nil
	}
	keys := make([]string, len(slots))
	for i, s := range slots {
		keys[i] = string(s)
	}
	return keys
}

// MainHandDamage composes the resolved main-hand damage display for the
// given equipped state (rpg-toolkit#811, contract §5): occupancy changes
// the die. A versatile weapon grips two-handed — its die steps up one
// notch (e.g. "1d8" -> "1d10") — only when offHand is nil (the off hand
// is completely empty, not merely holding a shield). Whenever offHand
// holds a weapon, its damage folds in as a dual-wield fragment, e.g.
// "1d4 piercing · off-hand 1d4". Returns "" if mainWeapon is nil.
func MainHandDamage(mainWeapon *weapons.Weapon, offHand *EquippedItem) string {
	if mainWeapon == nil {
		return ""
	}

	damage := mainWeapon.Damage
	if mainWeapon.HasProperty(weapons.PropertyVersatile) && offHand == nil {
		damage = mainWeapon.VersatileDamage()
	}

	line := fmt.Sprintf("%s %s", damage, mainWeapon.DamageType)
	if offWeapon := offHand.AsWeapon(); offWeapon != nil {
		line += fmt.Sprintf(" · off-hand %s", offWeapon.Damage)
	}
	return line
}

// slotFor returns the slot itemID is equipped in, or "" if it's carried
// but not equipped.
func (e EquipmentSlots) slotFor(itemID string) InventorySlot {
	for slot, id := range e {
		if id == itemID {
			return slot
		}
	}
	return ""
}

// StatLine composes the display line rpg-api passes through and the web
// renders verbatim (rpg-toolkit#811), e.g. "1d8 slashing · versatile" or
// "AC 16 · heavy". detail is typically equipment.ResolveEquipmentDetail's
// output. Returns "" for nil detail or gear with no combat stats (tools,
// packs, ammunition, misc items) — those have no wire-relevant line today.
func StatLine(detail *equipment.EquipmentDetail) string {
	switch {
	case detail == nil:
		return ""
	case detail.Weapon != nil:
		return weaponStatLine(detail.Weapon)
	case detail.Armor != nil:
		return armorStatLine(detail.Armor)
	default:
		return ""
	}
}

// weaponStatLine composes "{damage} {damage type} · {properties}", e.g.
// "1d8 slashing · versatile". Thrown weapons carry their range inline
// ("thrown 20/60"); every other property already reads as its display
// word (weapons.WeaponProperty values are lowercase, hyphenated where
// needed — "two-handed", "finesse", etc).
func weaponStatLine(w *equipment.WeaponDetail) string {
	line := fmt.Sprintf("%s %s", w.Damage, w.DamageType)
	if len(w.Properties) == 0 {
		return line
	}

	fragments := make([]string, len(w.Properties))
	for i, prop := range w.Properties {
		if prop == weapons.PropertyThrown && w.Range != nil {
			fragments[i] = fmt.Sprintf("thrown %d/%d", w.Range.Normal, w.Range.Long)
			continue
		}
		fragments[i] = string(prop)
	}
	return line + " · " + strings.Join(fragments, ", ")
}

// armorStatLine composes "AC {value} · {category}" for worn armor, or
// "+{value} AC" for shields (a shield's AC is a bonus, not a base).
func armorStatLine(a *equipment.ArmorDetail) string {
	if a.Category == armor.CategoryShield {
		return fmt.Sprintf("+%d AC", a.BaseAC)
	}
	return fmt.Sprintf("AC %d · %s", a.BaseAC, a.Category)
}

// ACNote composes a human-readable summary of an AC breakdown, e.g.
// "16 chain mail + 2 shield" or "10 + 2 DEX + 3 (Unarmored Defense)". The
// breakdown stays the source of truth (rpg-toolkit#811 keeps this a
// projection over it, not a second source of AC math); this is display
// only. Returns "" for a nil or empty breakdown.
func ACNote(breakdown *combat.ACBreakdown) string {
	if breakdown == nil || len(breakdown.Components) == 0 {
		return ""
	}

	var b strings.Builder
	for i, comp := range breakdown.Components {
		label := acComponentLabel(comp)
		switch {
		case i == 0:
			fmt.Fprintf(&b, "%d", comp.Value)
		case comp.Value < 0:
			fmt.Fprintf(&b, " - %d", -comp.Value)
		default:
			fmt.Fprintf(&b, " + %d", comp.Value)
		}
		if label != "" {
			b.WriteString(" " + label)
		}
	}
	return b.String()
}

// acComponentLabel derives a display label for one AC component from its
// source ref, so the note reads naturally regardless of what feature,
// condition, or spell contributed it — no per-source hardcoding needed as
// new AC sources are added.
func acComponentLabel(comp combat.ACComponent) string {
	if comp.Source == nil {
		return ""
	}
	switch comp.Type {
	case combat.ACSourceArmor, combat.ACSourceShield:
		return spaceify(comp.Source.ID)
	case combat.ACSourceAbility:
		return strings.ToUpper(comp.Source.ID)
	default:
		// Feature/spell/item/condition sources: value + a readable label,
		// e.g. "unarmored_defense" -> "(Unarmored Defense)".
		return "(" + titleCase(spaceify(comp.Source.ID)) + ")"
	}
}

// spaceify turns a ref ID like "chain-mail" or "unarmored_defense" into
// space-separated words.
func spaceify(id string) string {
	id = strings.ReplaceAll(id, "-", " ")
	return strings.ReplaceAll(id, "_", " ")
}

// titleCase capitalizes the first letter of each word.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
