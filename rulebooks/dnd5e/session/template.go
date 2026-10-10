// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// DeriveTemplateInput is one authored template to derive: the ref it is
// placed by and the author's strings.
type DeriveTemplateInput struct {
	// Ref is the template's own ref, `dnd5e:monsters:<id>`. It goes onto the
	// block, never the base's (rpg-project#555 R2), and its id is the
	// block's id.
	Ref string
	// Spec is the template as the dialect carried it.
	Spec dungeonspec.TemplateSpec
}

// DeriveTemplateOutput is what an authored template derives to.
type DeriveTemplateOutput struct {
	Block DerivedBlock
}

// DerivedBlock is a template's derived stat block: the numbers the rulebook
// works out from what the author wrote, for a host to show and never to
// compute (rpg-project#555 R7).
//
// It is this package's own view, not the rulebook's monster. The runtime
// monster never crosses this seam (S2), and the stored sheet's promise is that
// a host stores it without reading it. So the block is read off the assembled
// monster field by field, and nothing in it is recomputed here.
type DerivedBlock struct {
	ID   string
	Ref  string
	Name string

	HitPoints         int
	ArmorClass        int
	PassivePerception int
	ProficiencyBonus  int

	// Abilities are all six scores after the template is merged over its
	// base, keyed by short name ("str" through "cha").
	Abilities map[string]int

	// Attacks are the creature's weapon attacks, in action order.
	Attacks []DerivedAttack

	// Experience is what the creature is worth on its fall. It never
	// inherits from the base; 0 is worth nothing.
	Experience int
}

// DerivedAttack is one weapon attack on a derived block.
type DerivedAttack struct {
	// WeaponRef is the weapon's ref, `dnd5e:weapons:<id>`.
	WeaponRef string
	// AttackBonus is the whole bonus to hit: the ability modifier plus
	// proficiency.
	AttackBonus int
	// Damage is the roll as dice notation with every flat part folded in,
	// the attack ability's modifier included where the weapon adds it
	// ("1d6+1"). Several damage pools join with " + ".
	Damage string
}

// DeriveTemplate derives the stat block an authored template describes. It
// is the host's authoring-time echo: rpg-api calls it when a dungeon is
// compiled, so the studio shows the rulebook's numbers before anything is
// launched.
//
// ONE ASSEMBLY. Launch builds a template's monster through the same
// [assembleTemplate] this calls, so the block the studio was shown is the
// creature the run gets (R6), and the two cannot disagree about what a
// template may be. A host must never mirror this conversion.
//
// It lives in this package because this is the one package that imports both
// the dialect that carries a template and the rulebook that assembles it: the
// compiler may not know what a ref resolves to (C1), and the rulebook does not
// read the dialect's strings.
//
// There is no member yet at authoring time, so the block's ID is the
// template's id, the part of its ref after the second colon. A host never
// invents a member id to ask.
//
// Returns ErrNilInput, ErrBadRef (a malformed ref, base, armour or weapon),
// ErrNoLoader (a ref off the `dnd5e:monsters` route), ErrShadowedRef (a
// template named for a rulebook monster or base), ErrUnknownContent (an
// unknown base, named; an unknown armour, weapon, skill or ability), or
// ErrInvalidWorld (a template the rulebook refuses to assemble, with its
// reason as text).
func DeriveTemplate(in *DeriveTemplateInput) (*DeriveTemplateOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("derive template: %w", ErrNilInput)
	}
	ref, err := templateRefOf(in.Ref)
	if err != nil {
		return nil, err
	}
	built, err := assembleTemplate(ref.ID, ref, in.Spec)
	if err != nil {
		return nil, err
	}
	return &DeriveTemplateOutput{Block: blockOf(built.ToData())}, nil
}

// templateRefOf parses the ref a template is placed by. A ref that does not
// parse is [ErrBadRef]; one off the `dnd5e:monsters` route is [ErrNoLoader],
// as [instantiate] says of any ref.
func templateRefOf(raw string) (*core.Ref, error) {
	ref, err := core.ParseString(raw)
	if err != nil {
		return nil, fmt.Errorf("%q: %w: %v", raw, ErrBadRef, err)
	}
	if ref.Module != refs.Module || ref.Type != refs.TypeMonsters {
		return nil, fmt.Errorf("%q: %w", raw, ErrNoLoader)
	}
	return ref, nil
}

// assembleTemplate assembles the monster an authored template describes,
// as member memberID: the base it names, looked up in the rulebook; the
// template, read out of the author's strings; and [monster.FromTemplate],
// which derives every number and checks that the template and the base it is
// given are a pair (R6).
//
// SHADOWING IS REFUSED HERE, ONCE (R2). A template whose ref the rulebook
// already answers, through a constructor (`goblin`) or a base (`human`), is
// [ErrShadowedRef]: either answer would silently discard something somebody
// wrote. Because launch and the authoring echo both come through this
// function, they cannot disagree about it.
//
// An unknown base is refused BY NAME with [ErrUnknownContent]: the author
// wrote `dnd5e:monsters:elf` and is told `elf`, not handed a creature built
// from nothing. A rulebook monster that is not a base (a goblin) is refused
// the same way, because templates derive from bases.
//
// A refusal from the assembly itself is [ErrInvalidWorld] with the rulebook's
// reason carried as text. Its error is the rulebook's, and a host matching on
// it would be coupled to a module this seam exists to keep replaceable (S2).
func assembleTemplate(memberID string, ref *core.Ref, spec dungeonspec.TemplateSpec) (*monster.Monster, error) {
	_, constructed := monsters.ByRef(ref.String())
	_, isBase := monsters.BaseByRef(ref.String())
	if constructed || isBase {
		return nil, fmt.Errorf("template %q shadows rulebook monster %q; rename the template: %w",
			ref.ID, ref.String(), ErrShadowedRef)
	}

	// The conversion parses the base first, so a malformed base is ErrBadRef
	// and only a well-formed one can be unknown. The base is then looked up
	// through the parsed ref the template carries, so the pair FromTemplate
	// checks holds by construction.
	tmpl, err := templateOf(ref.ID, spec)
	if err != nil {
		return nil, fmt.Errorf("template %q: %w", ref.ID, err)
	}

	base, ok := monsters.BaseByRef(tmpl.Base.String())
	if !ok {
		return nil, fmt.Errorf("template %q: base %q is not a rulebook base: %w", ref.ID, tmpl.Base.String(), ErrUnknownContent)
	}

	built, err := monster.FromTemplate(&monster.FromTemplateInput{ID: memberID, Ref: ref, Template: tmpl, Base: base})
	if err != nil {
		return nil, fmt.Errorf("template %q: %w: %v", ref.ID, ErrInvalidWorld, err)
	}
	return built, nil
}

// blockOf reads the derived block off an assembled monster's sheet. Every
// number is the sheet's; the only arithmetic is folding a damage pool's flat
// parts into its notation, which is how a roll is written down.
func blockOf(sheet *monster.Data) DerivedBlock {
	block := DerivedBlock{
		ID:                sheet.ID,
		Name:              sheet.Name,
		HitPoints:         sheet.MaxHitPoints,
		ArmorClass:        sheet.ArmorClass,
		PassivePerception: sheet.Senses.PassivePerception,
		ProficiencyBonus:  sheet.ProficiencyBonus,
		Abilities:         make(map[string]int, len(sheet.AbilityScores)),
		Experience:        sheet.Experience,
	}
	if sheet.Ref != nil {
		block.Ref = sheet.Ref.String()
	}
	for ability, score := range sheet.AbilityScores {
		block.Abilities[string(ability)] = score
	}
	for _, action := range sheet.Actions {
		if action.Attack == nil {
			continue
		}
		modifier := 0
		if action.Attack.Ability != nil {
			modifier = action.Attack.Ability.Modifier
		}
		pools := make([]string, 0, len(action.Attack.Damage))
		for _, pool := range action.Attack.Damage {
			flat := pool.FlatBonus
			if pool.HasProperty(damage.AddsAttackAbilityModifier) {
				flat += modifier
			}
			pools = append(pools, notationOf(pool.Dice, flat))
		}
		block.Attacks = append(block.Attacks, DerivedAttack{
			WeaponRef:   action.Ref.String(),
			AttackBonus: action.Attack.AttackBonus,
			Damage:      strings.Join(pools, " + "),
		})
	}
	return block
}

// notationOf writes dice and a flat part as one roll: "1d6", "1d6+1", "1d4-1".
func notationOf(dice string, flat int) string {
	switch {
	case flat > 0:
		return fmt.Sprintf("%s+%d", dice, flat)
	case flat < 0:
		return fmt.Sprintf("%s%d", dice, flat)
	default:
		return dice
	}
}

// templateOf reads an authored template's strings into the rulebook's
// [monster.Template] — the one conversion between the dialect's carried
// strings and the rulebook's types.
//
// Every id is checked against the catalogue it names, refused as [arm]
// refuses a weapon: a malformed ref is [ErrBadRef], an armour, weapon, skill
// or ability nothing here knows is [ErrUnknownContent]. Absence is carried as
// absence (R8): no armour is nil (the base's), no skills or actions are nil
// (the base's), so [monster.Template.Merge] can tell "unstated" from "none".
// Experience is carried as written and never inherits; 0 is worth nothing.
//
// NAME IS IDENTITY, like the ref, so it is the template's and never the
// base's. An unnamed template is called by its id, the part of its ref after
// the second colon: an unnamed guard is "guard", not "Human". The base keeps
// its own name only when it is assembled as itself.
func templateOf(id string, spec dungeonspec.TemplateSpec) (monster.Template, error) {
	// The template names its base, and FromTemplate refuses one paired with
	// a different base: the pairing is checked inside the assembly.
	base, err := core.ParseString(spec.Base)
	if err != nil {
		return monster.Template{}, fmt.Errorf("base %q: %w: %v", spec.Base, ErrBadRef, err)
	}

	name := spec.Name
	if name == "" {
		name = id
	}

	out := monster.Template{
		Base:        base,
		Name:        name,
		HitDice:     spec.HitDice,
		Proficiency: spec.Proficiency,
		Experience:  spec.Experience,
	}

	if len(spec.Abilities) > 0 {
		out.Abilities = make(map[abilities.Ability]int, len(spec.Abilities))
		for key, score := range spec.Abilities {
			ability, ok := abilityOf(key)
			if !ok {
				return monster.Template{}, fmt.Errorf("ability %q: %w", key, ErrUnknownContent)
			}
			out.Abilities[ability] = score
		}
	}

	if spec.Armor != "" {
		parsed, err := core.ParseString(spec.Armor)
		if err != nil {
			return monster.Template{}, fmt.Errorf("%q: %w: %v", spec.Armor, ErrBadRef, err)
		}
		if parsed.Module != refs.Module || parsed.Type != refs.TypeArmor {
			return monster.Template{}, fmt.Errorf("%q: %w", spec.Armor, ErrUnknownContent)
		}
		worn := armor.ArmorID(parsed.ID)
		if _, err := armor.GetByID(worn); err != nil {
			return monster.Template{}, fmt.Errorf("%q: %w", spec.Armor, ErrUnknownContent)
		}
		out.Armor = &worn
	}

	if spec.Skills != nil {
		out.Skills = make([]skills.Skill, 0, len(spec.Skills))
		for _, key := range spec.Skills {
			skill, err := skills.GetByID(key)
			if err != nil {
				return monster.Template{}, fmt.Errorf("skill %q: %w", key, ErrUnknownContent)
			}
			out.Skills = append(out.Skills, skill)
		}
	}

	if spec.Actions != nil {
		out.Actions, err = weaponIDsOf(spec.Actions)
		if err != nil {
			return monster.Template{}, err
		}
	}

	return out, nil
}

// abilityOf is the ability an authored short name (str dex con int wis cha)
// names, matched exactly: the dialect writes lowercase, and a key that only
// matches after folding is not one the author wrote.
func abilityOf(key string) (abilities.Ability, bool) {
	for _, ability := range abilities.List() {
		if string(ability) == key {
			return ability, true
		}
	}
	return "", false
}
