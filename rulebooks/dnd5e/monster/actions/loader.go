// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
)

func unmarshalActionConfig(raw json.RawMessage, config any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(config); err != nil {
		return err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("unexpected trailing JSON value")
		}
		return err
	}

	return nil
}

const shortbowActionID = "shortbow"

// LoadAction creates a MonsterAction from ActionData.
// It dispatches to the appropriate action constructor based on the ref.
func LoadAction(data monster.ActionData) (monster.MonsterAction, error) {
	switch data.Ref.ID {
	case shortbowActionID:
		return loadShortbowAction(data)
	case "melee":
		return loadMeleeAction(data)
	case "ranged":
		return loadRangedAction(data)
	case "multiattack":
		return loadMultiattackAction(data)
	case "bite":
		return loadBiteAction(data)
	default:
		return nil, rpgerr.New(rpgerr.CodeNotFound, "unknown action: "+data.Ref.ID)
	}
}

// loadShortbowAction creates a ShortbowAction from config
// Placeholder for future implementation
func loadShortbowAction(_ monster.ActionData) (monster.MonsterAction, error) {
	// TODO: Implement shortbow action
	return nil, rpgerr.New(rpgerr.CodeNotFound, "shortbow action not yet implemented")
}

// loadMeleeAction creates a MeleeAction from config
func loadMeleeAction(data monster.ActionData) (monster.MonsterAction, error) {
	var config MeleeConfig
	if len(data.Config) > 0 {
		if err := unmarshalActionConfig(data.Config, &config); err != nil {
			return nil, rpgerr.Wrap(err, "failed to unmarshal melee config")
		}
	}
	return NewMeleeAction(config)
}

// loadRangedAction creates a RangedAction from config
func loadRangedAction(data monster.ActionData) (monster.MonsterAction, error) {
	var config RangedConfig
	if len(data.Config) > 0 {
		if err := unmarshalActionConfig(data.Config, &config); err != nil {
			return nil, rpgerr.Wrap(err, "failed to unmarshal ranged config")
		}
	}
	return NewRangedAction(config)
}

// loadMultiattackAction creates a MultiattackAction from config
func loadMultiattackAction(data monster.ActionData) (monster.MonsterAction, error) {
	var config MultiattackConfig
	if len(data.Config) > 0 {
		if err := json.Unmarshal(data.Config, &config); err != nil {
			return nil, rpgerr.Wrap(err, "failed to unmarshal multiattack config")
		}
	}
	return NewMultiattackAction(config), nil
}

// loadBiteAction creates a BiteAction from config
func loadBiteAction(data monster.ActionData) (monster.MonsterAction, error) {
	var config BiteConfig
	if len(data.Config) > 0 {
		if err := unmarshalActionConfig(data.Config, &config); err != nil {
			return nil, rpgerr.Wrap(err, "failed to unmarshal bite config")
		}
	}
	return NewBiteAction(config)
}

// LoadMonsterActions is a helper function that loads actions from ActionData
// and adds them to a monster. This is needed because the monster package
// cannot import the actions package directly (import cycle).
//
// Usage:
//
//	monster, err := monster.LoadFromData(ctx, data, bus)
//	if err := actions.LoadMonsterActions(monster, data.Actions); err != nil {
//	    // handle error
//	}
func LoadMonsterActions(m *monster.Monster, actionData []monster.ActionData) error {
	for _, data := range actionData {
		action, err := LoadAction(data)
		if err != nil {
			return rpgerr.Wrapf(err, "failed to load action %s", data.Ref.ID)
		}
		m.AddAction(action)
	}
	return nil
}
