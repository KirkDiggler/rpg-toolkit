// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// Action information: what a declaration IS, for the card a player reads
// before committing (provider-design R10–R14).
//
// # One attach point
//
// [attachInformation] runs in [Manager.Afford] alone, directly after effect
// rows, over the offers the turn path already compiled. Every gate and
// selector is settled before it runs and no execution caller of
// compileOffersFor calls it, so information can describe an action but never
// refuse, alter or select one. The rows outside that path — blockers, the
// world clock's social rows, reaction windows — carry only the prose their
// noun's owner already supplied.
//
// # Who writes what
//
// The root states facts as typed values ([combatActions.Describe]); this file
// renders each one into one label and value row, and the API and web copy it.
// Prose comes from the noun's owner: a spell's from the spell catalogue, an
// ability's from its combat ability or feature, an option's from whoever
// declared it, a reaction's from the condition or feature that offers it, a
// condition's from the display catalogue. Session authors prose only for the
// verbs it owns and the opportunity attack it names ([sessionVerbProse],
// [reactionDescription]).
//
// # Absent stays absent
//
// An empty description stays empty and no facts means no rows. A declaration
// with neither carries nil. Nothing is synthesised from a ref, an id or a
// name.

// sessionVerbProse is the prose for the verbs this package owns — they compile
// no rulebook definition, so no other owner exists to write it (R12).
var sessionVerbProse = map[Verb]string{
	VerbMove:       "Move along your chosen path, using the movement available to you.",
	VerbEndTurn:    "Finish your turn and let the next combatant act.",
	VerbDeathSave:  "Roll a death saving throw while dying. Successes help you stabilize; failures bring you closer to death.",
	VerbIntimidate: "Threaten a creature to influence its response. The outcome depends on the creature and the situation.",
	VerbPersuade:   "Try to change a creature's mind through conversation. The outcome depends on the creature and the situation.",
}

// reactionDescription sits beside [reactionName] and explains the reactions
// this package names. One entry, because the opportunity attack is the one
// reaction session names rather than receives from an offering owner.
var reactionDescription = map[string]string{
	refs.Conditions.OpportunityAttack().String(): "Use your reaction to make one melee attack against a creature that leaves your reach.",
}

// sessionVerbInformation is a session-owned verb's information, or nil for
// every verb session does not own. A fresh value per call, so no row shares
// one with another.
func sessionVerbInformation(verb Verb) *ActionInformation {
	text, owned := sessionVerbProse[verb]
	if !owned {
		return nil
	}
	return &ActionInformation{Description: text}
}

// proseInformation wraps owner-supplied prose with no facts, or nil when the
// owner supplied none.
func proseInformation(description string) *ActionInformation {
	if description == "" {
		return nil
	}
	return &ActionInformation{Description: description}
}

// attachInformationInput is the turn path's compiled offers, written in
// place.
type attachInformationInput struct {
	Offers []compiledOffer
}

// attachInformation sets Information on each compiled offer by verb: an
// attack or cast from its definition's described facts, an activation from
// its ability's prose, a session verb from [sessionVerbProse]. It writes
// Information and nothing else.
func attachInformation(in *attachInformationInput) error {
	for i := range in.Offers {
		offer := &in.Offers[i]
		var (
			info *ActionInformation
			err  error
		)
		switch offer.declaration.Verb {
		case VerbAttack:
			info, err = describedInformation(offer.attack)
		case VerbCast:
			info, err = describedInformation(offer.spell)
		case VerbActivate:
			info = proseInformation(offer.abilityDescription)
		default:
			info = sessionVerbInformation(offer.declaration.Verb)
		}
		if err != nil {
			return fmt.Errorf("information for %s: %w", offer.declaration.Verb, err)
		}
		offer.declaration.Information = info
	}
	return nil
}

// describedInformation is a compiled definition's information, or nil for a
// blocker that compiled none or a definition with neither prose nor facts.
func describedInformation(definition *combatActions.Definition) (*ActionInformation, error) {
	if definition == nil {
		return nil, nil
	}
	described, err := combatActions.Describe(&combatActions.DescribeInput{Definition: *definition})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadAttack, err)
	}
	details, err := renderFacts(described.Facts)
	if err != nil {
		return nil, err
	}
	if described.Description == "" && len(details) == 0 {
		return nil, nil
	}
	return &ActionInformation{Description: described.Description, Details: details}, nil
}

// renderFacts renders typed base facts into rows, in this fixed order: base
// damage, damage if injured, grip, save, each applied condition with its
// detail, healing, a weapon's reach or range (skipped for a cast, whose own
// range is what the player reads), the cast's range, targets, area,
// concentration. An enum value this renderer has no word for fails closed
// rather than rendering a guess.
func renderFacts(facts combatActions.BaseFacts) ([]ActionInformationDetail, error) {
	var rows []ActionInformationDetail
	add := func(label, value string) {
		rows = append(rows, ActionInformationDetail{Label: label, Value: value})
	}

	for _, pool := range facts.Damage {
		add("Base damage", renderDamage(pool))
	}
	if facts.Cast != nil {
		for _, pool := range facts.Cast.DamageIfInjured {
			add("Damage if injured", renderDamage(pool))
		}
	}
	if facts.Grip != combatActions.GripNone {
		grip, err := renderGrip(facts.Grip)
		if err != nil {
			return nil, err
		}
		add("Grip", grip)
	}

	cast := facts.Cast
	if cast != nil && cast.Save != nil {
		save, err := renderSave(cast.Save)
		if err != nil {
			return nil, err
		}
		add("Save", save)
	}
	if cast != nil {
		for _, effect := range cast.Effects {
			display, known := conditions.DisplayFor(effect.Ref)
			if !known {
				return nil, fmt.Errorf("%w: applied condition %s has no display catalogue entry",
					ErrBadAttack, effect.Ref.String())
			}
			label, err := effectLabel(effect)
			if err != nil {
				return nil, err
			}
			add(label, display.Name)
			if display.Detail != "" {
				add(display.Name, display.Detail)
			}
		}
		if cast.Healing != nil {
			add("Healing", renderHealing(cast.Healing))
		}
	}

	if cast == nil {
		if facts.Melee != nil {
			add("Reach", fmt.Sprintf("%d ft", facts.Melee.ReachFeet))
		}
		if facts.Ranged != nil {
			add("Range", renderRanged(facts.Ranged))
		}
		return rows, nil
	}

	add("Range", renderCastRange(cast))
	switch cast.Targets.Rule {
	case combatActions.CastTargetOneCreature, combatActions.CastTargetKnownCreature:
		add("Targets", renderTargets(cast.Targets))
	}
	if cast.Area != nil {
		area, err := renderArea(cast.Area)
		if err != nil {
			return nil, err
		}
		add("Area", area)
	}
	if cast.Concentration != nil {
		add("Concentration", "Required")
	}
	return rows, nil
}

func abilityAbbreviation(ability abilities.Ability) string {
	return strings.ToUpper(string(ability))
}

// renderDamage is "1d8 + STR modifier (+3) · Bludgeoning". The ability
// clause appears only when the modifier participates (damage.
// IncludesAbilityModifier's answer, carried on the fact); a stated but
// non-participating modifier renders nothing.
func renderDamage(pool combatActions.DamageFact) string {
	var b strings.Builder
	b.WriteString(pool.Dice)
	if pool.FlatBonus != 0 {
		fmt.Fprintf(&b, " %+d", pool.FlatBonus)
	}
	if pool.Ability != nil && pool.Ability.Participates {
		fmt.Fprintf(&b, " + %s modifier (%+d)", abilityAbbreviation(pool.Ability.Ability), pool.Ability.Modifier)
	}
	b.WriteString(" · ")
	b.WriteString(pool.Type.Display())
	return b.String()
}

func renderGrip(grip combatActions.Grip) (string, error) {
	switch grip {
	case combatActions.GripOneHanded:
		return "One-handed", nil
	case combatActions.GripTwoHanded:
		return "Two-handed", nil
	case combatActions.GripOffHand:
		return "Off-hand", nil
	default:
		return "", fmt.Errorf("%w: unrenderable grip %q", ErrBadAttack, grip)
	}
}

// renderSave is "CHA save · DC 13 · success: negated". The DC appears only
// when its source is static; an unknown DC is never guessed.
func renderSave(save *combatActions.SaveFact) (string, error) {
	names := make([]string, 0, len(save.Abilities))
	for _, ability := range save.Abilities {
		names = append(names, abilityAbbreviation(ability))
	}
	var b strings.Builder
	b.WriteString(strings.Join(names, " or "))
	b.WriteString(" save")
	if save.DCKnown {
		fmt.Fprintf(&b, " · DC %d", save.DC)
	}
	switch save.OnSuccess {
	case saves.Negated:
		b.WriteString(" · success: negated")
	case saves.Half:
		b.WriteString(" · success: half damage")
	default:
		return "", fmt.Errorf("%w: unrenderable save outcome %q", ErrBadAttack, save.OnSuccess)
	}
	switch save.Recurrence {
	case "", saves.RecurrenceNone:
	case saves.RecurrenceEndOfTurn:
		b.WriteString(" · repeats at end of turn")
	default:
		return "", fmt.Errorf("%w: unrenderable save recurrence %q", ErrBadAttack, save.Recurrence)
	}
	return b.String(), nil
}

func effectLabel(effect combatActions.EffectFact) (string, error) {
	if effect.OnFailedSave {
		return "On a failed save", nil
	}
	switch effect.Recipient {
	case combatActions.CastRecipientTarget:
		return "On the target", nil
	case combatActions.CastRecipientCaster:
		return "On you", nil
	default:
		return "", fmt.Errorf("%w: unrenderable effect recipient %q", ErrBadAttack, effect.Recipient)
	}
}

// renderHealing is "1d8 + 3 (Wisdom)", one clause per sourced modifier in
// order.
func renderHealing(healing *combatActions.HealingFact) string {
	var b strings.Builder
	b.WriteString(healing.Dice)
	for _, modifier := range healing.Modifiers {
		fmt.Fprintf(&b, " + %d (%s)", modifier.Amount, modifier.Name)
	}
	return b.String()
}

func renderRanged(ranged *combatActions.RangedDelivery) string {
	if ranged.LongFeet > 0 {
		return fmt.Sprintf("%d ft (long %d ft)", ranged.NormalFeet, ranged.LongFeet)
	}
	return fmt.Sprintf("%d ft", ranged.NormalFeet)
}

func renderCastRange(cast *combatActions.CastFacts) string {
	switch cast.Targets.Rule {
	case combatActions.CastTargetSelf:
		return "Self"
	case combatActions.CastTargetTouch:
		return "Touch"
	default:
		return fmt.Sprintf("%d ft", cast.RangeFeet)
	}
}

// renderTargets is "1 creature", "2 creatures" or "1 to 3 creatures".
func renderTargets(targets combatActions.TargetsFact) string {
	if targets.Min == targets.Max {
		if targets.Max == 1 {
			return "1 creature"
		}
		return fmt.Sprintf("%d creatures", targets.Max)
	}
	return fmt.Sprintf("%d to %d creatures", targets.Min, targets.Max)
}

// renderArea is "15 ft cube from your edge · affects others".
func renderArea(area *combatActions.CastArea) (string, error) {
	var shape string
	switch area.Footprint.Shape {
	case combatActions.AreaRadius:
		shape = "radius"
	case combatActions.AreaBox:
		shape = "cube"
	case combatActions.AreaTriangle:
		shape = "cone"
	default:
		return "", fmt.Errorf("%w: unrenderable area shape %q", ErrBadAttack, area.Footprint.Shape)
	}
	var origin string
	switch area.Footprint.Origin {
	case combatActions.AreaOriginCaster:
		origin = "from you"
	case combatActions.AreaOriginCasterEdge:
		origin = "from your edge"
	case combatActions.AreaOriginPoint:
		origin = "at a point"
	default:
		return "", fmt.Errorf("%w: unrenderable area origin %q", ErrBadAttack, area.Footprint.Origin)
	}
	var catches string
	switch area.Catches {
	case combatActions.AreaCatchesOthers:
		catches = "affects others"
	case combatActions.AreaCatchesEveryone:
		catches = "affects everyone"
	default:
		return "", fmt.Errorf("%w: unrenderable area catch %q", ErrBadAttack, area.Catches)
	}
	return fmt.Sprintf("%d ft %s %s · %s", area.Footprint.SizeFeet, shape, origin, catches), nil
}
