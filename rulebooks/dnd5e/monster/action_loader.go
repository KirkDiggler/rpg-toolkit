// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monster

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

const shortbowActionID = "shortbow"

// LoadAction creates a MonsterAction from ActionData.
// It dispatches to the appropriate action constructor based on the ref.
func LoadAction(data ActionData) (MonsterAction, error) {
	switch data.Ref.ID {
	case scimitarActionID:
		return loadScimitarAction(data)
	case shortbowActionID:
		return loadShortbowAction(data)
	default:
		return nil, rpgerr.New(rpgerr.CodeNotFound, "unknown action: "+data.Ref.ID)
	}
}

// loadScimitarAction creates a ScimitarAction from config
func loadScimitarAction(data ActionData) (*ScimitarAction, error) {
	var config ScimitarConfig
	if len(data.Config) > 0 {
		if err := json.Unmarshal(data.Config, &config); err != nil {
			return nil, rpgerr.Wrap(err, "failed to unmarshal scimitar config")
		}
	}
	if config.ID == "" {
		config.ID = "scimitar"
	}
	return NewScimitarAction(config), nil
}

// loadShortbowAction creates a ShortbowAction from config
// Placeholder for future implementation
func loadShortbowAction(_ ActionData) (MonsterAction, error) {
	// TODO: Implement shortbow action
	return nil, rpgerr.New(rpgerr.CodeNotFound, "shortbow action not yet implemented")
}
