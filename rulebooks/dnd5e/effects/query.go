// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// Query is a closed rulebook assessment context. At most one field is present;
// zero fields requests descriptions without claiming any applied benefit. Events
// and frames are borrowed for this read only and are never changed by assessment.
type Query struct {
	// Action describes the actor's outgoing action/damage, including save-shaped
	// damage. Its Roll kind does not mean the actor is the saver; use Save for that.
	Action   *contributions.Frame
	Incoming *dndevents.IncomingDamageEvent
	Save     *dndevents.SavingThrowChainEvent
	Check    *dndevents.AbilityCheckChainEvent
	// Roll describes the recipient's own roll for pull-based contribution selection.
	Roll *RollQuery
	// Holder describes a holder outside any roll or action: who it is and
	// what its sheet says, for a number read off the sheet (initiative).
	Holder *HolderQuery
}

// HolderQuery is a holder and its ability scores, for an amount that reads
// them. Scores missing an ability means the sheet states no such score, which
// is not the same as a score of zero.
type HolderQuery struct {
	MemberID string
	Scores   shared.AbilityScores
}

// RollQuery identifies whose roll is described, independently of an outgoing
// action whose target may be the creature making a saving throw.
type RollQuery struct {
	MemberID string
	Kind     contributions.RollKind
}

type domain string

const (
	domainNone        domain = ""
	domainMaintenance domain = "maintenance"
	domainRoll        domain = "roll"
	domainDamage      domain = "outgoing_damage"
	domainIncoming    domain = "incoming_damage"
	domainSave        domain = "saving_throw"
	domainCheck       domain = "ability_check"
	domainHolder      domain = "holder"
)

// Validate refuses ambiguous contexts and malformed known identities/facts.
// Missing facts on a valid action frame remain unknown rather than becoming false.
func (q Query) Validate() error {
	count := 0
	if q.Action != nil {
		count++
	}
	if q.Incoming != nil {
		count++
	}
	if q.Save != nil {
		count++
	}
	if q.Check != nil {
		count++
	}
	if q.Roll != nil {
		count++
	}
	if q.Holder != nil {
		count++
		if q.Holder.MemberID == "" {
			return fmt.Errorf("holder query requires a member")
		}
	}
	if count > 1 {
		return fmt.Errorf("effect query must name at most one context")
	}
	if q.Action != nil {
		return q.Action.Validate()
	}
	if q.Roll != nil && (q.Roll.MemberID == "" || q.Roll.Kind == "") {
		return fmt.Errorf("roll query requires a recipient and roll kind")
	}
	if q.Incoming != nil {
		if q.Incoming.TargetID() == "" {
			return fmt.Errorf("incoming query requires target")
		}
		return q.Incoming.Frame().Validate()
	}
	if q.Save != nil {
		if q.Save.SaverID == "" {
			return fmt.Errorf("save query requires saver")
		}
		if _, err := abilities.GetByID(string(q.Save.Ability)); err != nil {
			return err
		}
	}
	if q.Check != nil {
		if q.Check.CheckerID == "" {
			return fmt.Errorf("check query requires checker")
		}
		// A bare-ability check names no skill; a stated skill or ability
		// must still be one the rulebook has.
		if q.Check.Skill != "" {
			if _, err := skills.GetByID(string(q.Check.Skill)); err != nil {
				return err
			}
		}
		if q.Check.Ability != "" {
			if _, err := abilities.GetByID(string(q.Check.Ability)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q Query) domain() domain {
	switch {
	case q.Roll != nil:
		return domainRoll
	case q.Action != nil:
		// The frame can describe a caster's save-shaped outgoing action. Its
		// roll kind does not make the actor the saver; Save names that context.
		return domainDamage
	case q.Incoming != nil:
		return domainIncoming
	case q.Save != nil:
		return domainSave
	case q.Check != nil:
		return domainCheck
	case q.Holder != nil:
		return domainHolder
	default:
		return domainNone
	}
}

func (q Query) subject() string {
	switch {
	case q.Roll != nil:
		return q.Roll.MemberID
	case q.Action != nil:
		return q.Action.Actor
	case q.Incoming != nil:
		return q.Incoming.TargetID()
	case q.Save != nil:
		return q.Save.SaverID
	case q.Check != nil:
		return q.Check.CheckerID
	case q.Holder != nil:
		return q.Holder.MemberID
	default:
		return ""
	}
}

func (q Query) ability() (abilities.Ability, bool) {
	switch {
	case q.Action != nil:
		return q.Action.Action.Ability.Get()
	case q.Save != nil:
		return q.Save.Ability, true
	case q.Check != nil:
		if q.Check.Ability != "" {
			return q.Check.Ability, true
		}
		if q.Check.Skill != "" {
			return skills.Ability(q.Check.Skill), true
		}
		// Neither stated: the ability is unknown, so a guard on it depends
		// rather than the whole check failing.
		return "", false
	default:
		return "", false
	}
}

// queryFrom is the single adaptation from the generic provider's opaque frame
// to the rulebook's closed set. No unrecognized frame is silently accepted.
func queryFrom(frame any) (Query, error) {
	var q Query
	switch value := frame.(type) {
	case nil:
	case Query:
		q = value
	case contributions.Frame:
		q.Action = &value
	case *dndevents.IncomingDamageEvent:
		q.Incoming = value
	case *dndevents.SavingThrowChainEvent:
		q.Save = value
	case *dndevents.AbilityCheckChainEvent:
		q.Check = value
	default:
		return q, fmt.Errorf("unsupported effect query %T", frame)
	}
	return q, q.Validate()
}
