// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster/monsters"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// deriveTemplateInput is one authored template to assemble: the member id it
// becomes, the ref it is placed by, and the author's strings.
type deriveTemplateInput struct {
	// ID is the member the monster becomes ("guard-1").
	ID string
	// Ref is the template's own ref, `dnd5e:monsters:<id>`. It goes onto the
	// sheet, never the base's (rpg-project#555 R2).
	Ref string
	// Spec is the template as the dialect carried it.
	Spec dungeonspec.TemplateSpec
}

// deriveTemplateOutput is the assembled monster.
type deriveTemplateOutput struct {
	Monster *monster.Monster
}

// deriveTemplate assembles the monster an authored template describes: the
// base it names, looked up in the rulebook; the template, read out of the
// author's strings; and [monster.FromTemplate], which derives every number
// and checks that the template and the base it is given are a pair (R6).
//
// It lives in this package because this is the one package that imports both
// the dialect that carries a template and the rulebook that assembles it: the
// compiler may not know what a ref resolves to (C1), and the rulebook does not
// read the dialect's strings.
//
// A ref that does not parse is [ErrBadRef]; one off the `dnd5e:monsters`
// route is [ErrNoLoader], as [instantiate] says of any ref. An unknown base is
// refused BY NAME with [ErrUnknownContent]: the author wrote
// `dnd5e:monsters:elf` and is told `elf`, not handed a creature built from
// nothing. A rulebook monster that is not a base (a goblin) is refused the
// same way, because templates derive from bases.
//
// A refusal from the assembly itself is [ErrInvalidWorld] with the rulebook's
// reason carried as text. Its error is the rulebook's, and a host matching on
// it would be coupled to a module this seam exists to keep replaceable (S2).
func deriveTemplate(in *deriveTemplateInput) (*deriveTemplateOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("derive template: %w", ErrNilInput)
	}
	ref, err := core.ParseString(in.Ref)
	if err != nil {
		return nil, fmt.Errorf("%q: %w: %v", in.Ref, ErrBadRef, err)
	}
	if ref.Module != refs.Module || ref.Type != refs.TypeMonsters {
		return nil, fmt.Errorf("%q: %w", in.Ref, ErrNoLoader)
	}

	base, ok := monsters.BaseByRef(in.Spec.Base)
	if !ok {
		return nil, fmt.Errorf("template %q: base %q is not a rulebook base: %w", ref.ID, in.Spec.Base, ErrUnknownContent)
	}

	tmpl, err := templateOf(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("template %q: %w", ref.ID, err)
	}

	built, err := monster.FromTemplate(&monster.FromTemplateInput{ID: in.ID, Ref: ref, Template: tmpl, Base: base})
	if err != nil {
		return nil, fmt.Errorf("template %q: %w: %v", ref.ID, ErrInvalidWorld, err)
	}
	return &deriveTemplateOutput{Monster: built}, nil
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
func templateOf(spec dungeonspec.TemplateSpec) (monster.Template, error) {
	// The template names its base, and FromTemplate refuses one paired with
	// a different base: the pairing is checked inside the assembly.
	base, err := core.ParseString(spec.Base)
	if err != nil {
		return monster.Template{}, fmt.Errorf("base %q: %w: %v", spec.Base, ErrBadRef, err)
	}

	out := monster.Template{
		Base:        base,
		Name:        spec.Name,
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
