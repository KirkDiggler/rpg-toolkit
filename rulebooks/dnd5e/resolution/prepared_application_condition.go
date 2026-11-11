// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"encoding/json"
	"fmt"

	actions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
)

// A not-yet-delivered candidate freezes its canonical local construction. Live
// membership is never kept here; it remains on the persisted coordinator.
type frozenPreparedCondition struct {
	ApplicationID string          `json:"application_id"`
	MemberKey     string          `json:"member_key"`
	Data          json.RawMessage `json:"data"`
}

func freezePreparedCondition(prepared preparedCondition) (*frozenPreparedCondition, error) {
	if prepared.applicationID == "" || prepared.memberKey == "" || prepared.behavior == nil {
		return nil, fmt.Errorf("%w: missing prepared application delivery", ErrBadFrozen)
	}
	data, err := prepared.behavior.ToJSON()
	if err != nil {
		return nil, err
	}
	return &frozenPreparedCondition{ApplicationID: prepared.applicationID, MemberKey: prepared.memberKey, Data: data}, nil
}

func restorePreparedCondition(frozen *frozenPreparedCondition, declaration actions.ConditionApplication) (preparedCondition, error) {
	if frozen == nil || frozen.ApplicationID == "" || frozen.MemberKey == "" || len(frozen.Data) == 0 {
		return preparedCondition{}, fmt.Errorf("%w: incomplete application delivery", ErrBadFrozen)
	}
	behavior, err := conditions.LoadJSON(frozen.Data)
	if err != nil {
		return preparedCondition{}, fmt.Errorf("%w: %w", ErrBadFrozen, err)
	}
	if !behavior.Ref().Equals(&declaration.Ref) {
		return preparedCondition{}, fmt.Errorf("%w: delivery ref disagrees with frozen condition declaration", ErrBadFrozen)
	}
	name, err := conditionDescription(declaration.Ref)
	if err != nil {
		return preparedCondition{}, err
	}
	return preparedCondition{declaration: declaration, behavior: behavior, name: name, applicationID: frozen.ApplicationID, memberKey: frozen.MemberKey}, nil
}
