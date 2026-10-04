// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"gopkg.in/yaml.v3"
)

// discoveryAttemptsShape preserves source presence and scalar kinds before
// yaml.v3's decoded zero values or integer truncation can hide a bad authoring value.
func discoveryAttemptsShape(c *yaml.Node, path string, add errSink) {
	node := optionalNode(c, "attempts", path, add)
	if node == nil {
		return
	}
	path += ".attempts"
	if node.Kind != yaml.MappingNode {
		add(path, errNotAMapping)
		return
	}
	for _, key := range []string{"max", "reset_hexes"} {
		if childNode(node, key) != nil {
			requireInteger(node, key, path, add)
		}
	}
	if childNode(node, "lifetime") != nil {
		requireString(node, "lifetime", path, add)
	}
}

// discoveryPolicyInput copies authored settings into the constructor's source
// shape. Policy interpretation/defaults belong to encounter, not this adapter.
func discoveryPolicyInput(in *DiscoveryAttemptsSpec) *encounter.DiscoveryPolicyInput {
	if in == nil {
		return nil
	}
	out := &encounter.DiscoveryPolicyInput{}
	if in.MaxAttempts != nil {
		value := *in.MaxAttempts
		out.MaxAttempts = &value
	}
	if in.ResetHexes != nil {
		value := *in.ResetHexes
		out.ResetHexes = &value
	}
	if in.Lifetime != nil {
		value := encounter.DiscoveryLifetime(*in.Lifetime)
		out.Lifetime = &value
	}
	return out
}
