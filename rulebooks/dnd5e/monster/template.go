// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"math"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/armor"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// Template is an authored stat block: what a designer writes down about a
// creature, never what the rulebook works out from it (rpg-project#555 R3).
//
// It stores scores, hit dice, armour, proficiency, trained skills, weapons and
// experience. Hit points, armour class, attack bonus, damage and passive
// Perception are NOT fields here. They are derived once, by [FromTemplate],
// because a stored total silently stops following the score it came from — a
// guard re-authored with CON 14 must gain hit points without anyone editing a
// second number.
//
// A template names a base and overrides it per field ([Template.Merge], R4).
// Every field's zero value means "the base's" except Experience, which means
// worth nothing (R8).
type Template struct {
	// Base is the rulebook base this template derives from
	// (dnd5e:monsters:human), which the caller resolves and passes to
	// [FromTemplate] as [FromTemplateInput.Base]. A rulebook base itself names
	// NO Base: that is what marks it as a base, so a derived template passed
	// where a base belongs is refused. It is NOT the derived creature's ref;
	// [FromTemplateInput.Ref] carries that explicitly.
	Base *core.Ref

	// Name is the display name. Empty = the base's.
	Name string

	// Abilities are partial scores. An absent key is the base's score.
	Abilities map[abilities.Ability]int

	// HitDice is the hit dice notation ("2d8"), one die size and no static
	// modifier — the CON modifier per die is derived. Empty = the base's.
	HitDice string

	// Armor is the body armour worn. nil = the base's; a pointer to the empty
	// ID states "nothing worn" explicitly, which is 10 + DEX.
	Armor *armor.ArmorID

	// Proficiency is the proficiency bonus. 0 = the base's.
	Proficiency int

	// Skills are the trained skills, each adding Proficiency. nil = the base's.
	Skills []skills.Skill

	// Actions are the catalogue weapons carried, in driver order. nil = the
	// base's; a stated list replaces the base's wholesale.
	Actions []weapons.WeaponID

	// Speed is the creature's movement. The zero value = the base's.
	Speed SpeedData

	// Experience is what the creature is worth on its fall. NOT inherited:
	// zero means worth nothing, because a captain's worth is authored and a
	// cook's is nothing (R8).
	Experience int
}

// Merge lays t over base, field by field (R4): a stated field wins, an
// unstated one is the base's. Abilities merge per key. Experience is t's
// alone. The result shares no map or slice with either input, so mutating a
// merged template can never reach a rulebook base.
func (t Template) Merge(base Template) Template {
	out := Template{
		Base:        base.Base,
		Name:        base.Name,
		HitDice:     base.HitDice,
		Armor:       base.Armor,
		Proficiency: base.Proficiency,
		Skills:      base.Skills,
		Actions:     base.Actions,
		Speed:       base.Speed,
		Experience:  t.Experience,
	}

	if t.Base != nil {
		out.Base = t.Base
	}
	if t.Name != "" {
		out.Name = t.Name
	}
	if t.HitDice != "" {
		out.HitDice = t.HitDice
	}
	if t.Armor != nil {
		out.Armor = t.Armor
	}
	if t.Proficiency != 0 {
		out.Proficiency = t.Proficiency
	}
	if t.Skills != nil {
		out.Skills = t.Skills
	}
	if t.Actions != nil {
		out.Actions = t.Actions
	}
	if t.Speed != (SpeedData{}) {
		out.Speed = t.Speed
	}

	out.Abilities = make(map[abilities.Ability]int, len(base.Abilities)+len(t.Abilities))
	for ability, score := range base.Abilities {
		out.Abilities[ability] = score
	}
	for ability, score := range t.Abilities {
		out.Abilities[ability] = score
	}

	if out.Armor != nil {
		worn := *out.Armor
		out.Armor = &worn
	}
	out.Skills = append([]skills.Skill(nil), out.Skills...)
	out.Actions = append([]weapons.WeaponID(nil), out.Actions...)

	return out
}

// FromTemplateInput is what [FromTemplate] assembles from. Named fields,
// because Template and Base share a type: as positional arguments a swapped
// call would compile and derive a silently wrong creature.
type FromTemplateInput struct {
	// ID is the monster entity's id ("guard-1"). Required.
	ID string

	// Ref is the derived creature's own ref — the template's
	// (dnd5e:monsters:guard), never the base's (rpg-project#555 R2). It is
	// stated here rather than on [Template] so it can never be inherited by
	// accident: a guard and a cook that both reported dnd5e:monsters:human
	// would lose the author's id off the sheet. Required.
	Ref *core.Ref

	// Template is the authored block: the overrides.
	Template Template

	// Base is the rulebook base the template names, looked up by the caller
	// (monsters.BaseByRef). Required, and it must be a base: it names no Base
	// of its own.
	Base Template
}

// FromTemplate assembles the monster a template describes, over the base the
// caller looked up (R6: the ONLY function that turns a template into a
// monster, and the one a rulebook base is itself assembled by).
//
// Derived, never authored:
//   - HP = floor(hit dice average) + CON modifier per die; current = max.
//   - AC = [armor.ArmorClass] of the worn armour (nil = 10 + DEX).
//   - Each weapon through the monster's own [Monster.SetWeapons] path.
//   - Each trained skill = the proficiency bonus; passive Perception =
//     10 + WIS modifier, + proficiency when Perception is trained.
//
// The sheet carries in.Ref. Its creature type is the base's catalogue fact (a
// guard derived from human is humanoid): the base the template names, or, for
// a base assembling itself, its own ref.
//
// It refuses by name rather than assembling a creature with a silent zero:
// a nil input, a missing id or ref, a missing base, a derived template passed
// as the base, a missing or malformed hit dice string, a score outside 1–30
// or missing, an unknown armour, skill or weapon, no weapons, and hit points
// that would come out below 1. A refusal returns no monster.
func FromTemplate(in *FromTemplateInput) (*Monster, error) {
	if in == nil {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "no template input")
	}
	if in.ID == "" {
		return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "template monster has no id")
	}
	if in.Ref == nil {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "template monster %q has no ref", in.ID)
	}
	if in.Base.Base != nil {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"template monster %q: a template cannot be a base (the base names base %q)",
			in.ID, in.Base.Base.String())
	}
	if in.Base.isEmpty() {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "template monster %q has no base", in.ID)
	}

	merged := in.Template.Merge(in.Base)
	typeRef := in.Template.Base
	if typeRef == nil {
		typeRef = in.Ref
	}

	if merged.Name == "" {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "template monster %q has no name", in.ID)
	}
	if merged.Proficiency < 0 {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "template proficiency %d is negative", merged.Proficiency)
	}
	if merged.Experience < 0 {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "template experience %d is negative", merged.Experience)
	}

	scores, err := scoresOf(merged.Abilities)
	if err != nil {
		return nil, err
	}

	hp, err := hitPointsOf(merged.HitDice, scores.Modifier(abilities.CON))
	if err != nil {
		return nil, err
	}

	ac, err := armorClassOf(merged.Armor, scores.Modifier(abilities.DEX))
	if err != nil {
		return nil, err
	}

	proficiency := proficiencyBonusOf(merged.Proficiency)
	proficiencies, perceptionTrained, err := proficienciesFor(merged.Skills, proficiency)
	if err != nil {
		return nil, err
	}

	passive := 10 + scores.Modifier(abilities.WIS)
	if perceptionTrained {
		passive += proficiency
	}

	m := New(Config{
		CreatureType:     creatureTypeFor("", typeRef),
		ID:               in.ID,
		Name:             merged.Name,
		Ref:              in.Ref,
		HP:               hp,
		AC:               ac,
		AbilityScores:    scores,
		ProficiencyBonus: proficiency,
		Experience:       merged.Experience,
		Proficiencies:    proficiencies,
		Senses:           SensesData{PassivePerception: passive},
	})
	m.SetSpeed(merged.Speed)

	if len(merged.Actions) == 0 {
		return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "template %q carries no actions", merged.Name)
	}
	if err := m.SetWeapons(merged.Actions); err != nil {
		return nil, rpgerr.Wrapf(err, "template %q actions", merged.Name)
	}

	return m, nil
}

// isEmpty reports whether t states nothing at all — the zero value a caller
// holds after ignoring a failed base lookup.
func (t Template) isEmpty() bool {
	return t.Base == nil && t.Name == "" && len(t.Abilities) == 0 && t.HitDice == "" &&
		t.Armor == nil && t.Proficiency == 0 && t.Skills == nil && t.Actions == nil &&
		t.Speed == (SpeedData{}) && t.Experience == 0
}

// scoresOf requires all six scores, each within 1–30, naming the one that
// is missing or out of range.
func scoresOf(authored map[abilities.Ability]int) (shared.AbilityScores, error) {
	scores := make(shared.AbilityScores, len(abilities.List()))
	for _, ability := range abilities.List() {
		score, ok := authored[ability]
		if !ok {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument, "template has no %s score", ability)
		}
		if score < 1 || score > 30 {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"template %s score %d is outside 1–30", ability, score)
		}
		scores[ability] = score
	}
	for ability := range authored {
		if _, err := abilities.GetByID(string(ability)); err != nil {
			return nil, rpgerr.Wrapf(err, "template ability %q", ability)
		}
	}
	return scores, nil
}

// hitPointsOf is floor(average) + conModifier per die. The notation is one
// die size with no static modifier: the per-die CON modifier is derived, so a
// stated "+2" would count it twice.
func hitPointsOf(notation string, conModifier int) (int, error) {
	if notation == "" {
		return 0, rpgerr.New(rpgerr.CodeInvalidArgument, "template has no hit dice")
	}
	pool, err := dice.ParseNotation(notation)
	if err != nil {
		return 0, rpgerr.Wrapf(err, "template hit dice %q", notation)
	}
	if strings.ContainsAny(pool.Notation(), "+-") {
		return 0, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"template hit dice %q must be one die size with no modifier (CON is added per die)", notation)
	}

	count := pool.Min() // no modifier, so the minimum is one per die
	hp := int(math.Floor(pool.Average())) + count*conModifier
	if hp < 1 {
		return 0, rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"template hit dice %q with CON modifier %d derives %d hit points", notation, conModifier, hp)
	}
	return hp, nil
}

// armorClassOf derives AC through the armour catalogue's own rule. nil or the
// empty ID is nothing worn. A shield is not body armour and is refused.
func armorClassOf(id *armor.ArmorID, dexModifier int) (int, error) {
	if id == nil || *id == "" {
		return armor.ArmorClass(nil, dexModifier), nil
	}
	worn, err := armor.GetByID(*id)
	if err != nil {
		return 0, rpgerr.Wrapf(err, "template armor %q", *id)
	}
	if worn.Category == armor.CategoryShield {
		return 0, rpgerr.Newf(rpgerr.CodeInvalidArgument, "template armor %q is a shield, not body armor", *id)
	}
	return armor.ArmorClass(&worn, dexModifier), nil
}

// proficienciesFor turns trained skills into the sheet's proficiency list,
// each worth the proficiency bonus, and reports whether Perception is among
// them.
func proficienciesFor(trained []skills.Skill, proficiency int) ([]ProficiencyData, bool, error) {
	out := make([]ProficiencyData, 0, len(trained))
	seen := make(map[skills.Skill]bool, len(trained))
	perception := false
	for _, skill := range trained {
		if _, err := skills.GetByID(string(skill)); err != nil {
			return nil, false, rpgerr.Wrapf(err, "template skill %q", skill)
		}
		if seen[skill] {
			continue
		}
		seen[skill] = true
		if skill == skills.Perception {
			perception = true
		}
		out = append(out, ProficiencyData{Skill: string(skill), Bonus: proficiency})
	}
	return out, perception, nil
}
