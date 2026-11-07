// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
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
}

type domain string

const (
	domainNone     domain = ""
	domainDamage   domain = "outgoing_damage"
	domainIncoming domain = "incoming_damage"
	domainSave     domain = "saving_throw"
	domainCheck    domain = "ability_check"
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
	if count > 1 {
		return fmt.Errorf("effect query must name at most one context")
	}
	if q.Action != nil {
		return q.Action.Validate()
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
		if _, err := skills.GetByID(string(q.Check.Skill)); err != nil {
			return err
		}
	}
	return nil
}

func (q Query) domain() domain {
	switch {
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
	default:
		return domainNone
	}
}

func (q Query) subject() string {
	switch {
	case q.Action != nil:
		return q.Action.Actor
	case q.Incoming != nil:
		return q.Incoming.TargetID()
	case q.Save != nil:
		return q.Save.SaverID
	case q.Check != nil:
		return q.Check.CheckerID
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
		return skills.Ability(q.Check.Skill), true
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
