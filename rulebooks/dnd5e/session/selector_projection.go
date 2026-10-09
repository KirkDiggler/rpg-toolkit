// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/healing"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
)

// selectorDefinition is the declaration selector's ALLOW-LIST projection of a
// [combatActions.Definition]: the mechanical fields an action's identity is
// made of, and nothing else. A field reaches the selector only by being
// written here. Prose — [combatActions.Definition.Description] and
// [combatActions.CastOption.Description] — is never written here, so editing
// what a card says can never change which declaration a client is holding.
//
// The JSON tags are the definition's own, field for field, so the canonical
// document this projection marshals to is byte-identical to the one the raw
// definition produced before it carried prose (TestAttackDeclarationIDGolden,
// TestCastDeclarationIDGolden).
//
// Name stays (provider-design O1). It is a display word, but it was already
// selector material; dropping it is a selector-version bump that invalidates
// every held ID, and that waits for a use case.
//
// TestSelectorProjectionClassifiesEveryDefinitionField classifies every field
// of the definition's type tree as mechanical or prose and fails on an
// unclassified one, so a new field cannot silently escape identity or
// silently enter it.
type selectorDefinition struct {
	// Ref is a POINTER for the same reason definitionVariant takes one:
	// [core.Ref] marshals to its string form only through its pointer
	// receiver.
	Ref      *core.Ref                      `json:"ref"`
	Name     string                         `json:"name"`
	Cost     *combat.SpendProfile           `json:"cost,omitempty"`
	Attack   *combatActions.AttackProfile   `json:"attack,omitempty"`
	Cast     *selectorCastProfile           `json:"cast,omitempty"`
	Sequence *combatActions.SequenceProfile `json:"sequence,omitempty"`
}

// selectorCastProfile mirrors every [combatActions.CastProfile] field with an
// identical tag. Its only departure is Options, whose elements drop the
// option's description.
type selectorCastProfile struct {
	Attack             *combatActions.AttackProfile     `json:"attack,omitempty"`
	RecipientBlockedBy []core.Ref                       `json:"recipient_blocked_by,omitempty"`
	Casting            *combat.SpellCasting             `json:"casting,omitempty"`
	Healing            *healing.Declaration             `json:"healing,omitempty"`
	HealingExcludes    []string                         `json:"healing_excludes,omitempty"`
	Stabilize          bool                             `json:"stabilize,omitempty"`
	RangeFeet          int                              `json:"range_feet"`
	Target             combatActions.CastTargetRule     `json:"target"`
	MinTargets         int                              `json:"min_targets"`
	MaxTargets         int                              `json:"max_targets"`
	Save               *saves.SaveGate                  `json:"save,omitempty"`
	Damage             []damage.Damage                  `json:"damage,omitempty"`
	DamageIfInjured    []damage.Damage                  `json:"damage_if_injured,omitempty"`
	Effects            []combatActions.CastEffect       `json:"effects,omitempty"`
	Area               *combatActions.CastArea          `json:"area,omitempty"`
	Move               *combatActions.CastMove          `json:"move,omitempty"`
	Concentration      *combatActions.CastConcentration `json:"concentration,omitempty"`
	Options            []selectorCastOption             `json:"options,omitempty"`
}

// selectorCastOption is a cast option's identity: the id execution reads and
// the label that was already selector material (O1). Its description is
// prose and is not here.
type selectorCastOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// selectorDefinitionOf builds the allow-list projection of definition. It
// shares the definition's nested mechanical values rather than copying them:
// the projection is marshaled at once and never retained or mutated.
func selectorDefinitionOf(definition *combatActions.Definition) *selectorDefinition {
	if definition == nil {
		return nil
	}
	ref := definition.Ref
	return &selectorDefinition{
		Ref:      &ref,
		Name:     definition.Name,
		Cost:     definition.Cost,
		Attack:   definition.Attack,
		Cast:     selectorCastProfileOf(definition.Cast),
		Sequence: definition.Sequence,
	}
}

func selectorCastProfileOf(profile *combatActions.CastProfile) *selectorCastProfile {
	if profile == nil {
		return nil
	}
	var options []selectorCastOption
	if profile.Options != nil {
		options = make([]selectorCastOption, len(profile.Options))
		for i, option := range profile.Options {
			options[i] = selectorCastOption{ID: option.ID, Label: option.Label}
		}
	}
	return &selectorCastProfile{
		Attack:             profile.Attack,
		RecipientBlockedBy: profile.RecipientBlockedBy,
		Casting:            profile.Casting,
		Healing:            profile.Healing,
		HealingExcludes:    profile.HealingExcludes,
		Stabilize:          profile.Stabilize,
		RangeFeet:          profile.RangeFeet,
		Target:             profile.Target,
		MinTargets:         profile.MinTargets,
		MaxTargets:         profile.MaxTargets,
		Save:               profile.Save,
		Damage:             profile.Damage,
		DamageIfInjured:    profile.DamageIfInjured,
		Effects:            profile.Effects,
		Area:               profile.Area,
		Move:               profile.Move,
		Concentration:      profile.Concentration,
		Options:            options,
	}
}
