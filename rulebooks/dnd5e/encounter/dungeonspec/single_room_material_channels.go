// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"math"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"gopkg.in/yaml.v3"
)

// RoomChannelAdjustment retains native mode and multiplier presence. A nil
// containing channel means default; Off has no multiplier and Scale has one.
type RoomChannelAdjustment struct {
	Mode       encounter.ChannelAdjustmentMode `yaml:"mode" json:"mode"`
	Multiplier *float64                        `yaml:"multiplier,omitempty" json:"multiplier,omitempty"`
}

// RoomMaterialChannelAdjustment is native appearance intent for an opaque
// provider-declared target. It supplies neither images nor mechanical meaning.
type RoomMaterialChannelAdjustment struct {
	TargetID string                 `yaml:"targetId" json:"targetId"`
	Normal   *RoomChannelAdjustment `yaml:"normal,omitempty" json:"normal,omitempty"`
	Emissive *RoomChannelAdjustment `yaml:"emissive,omitempty" json:"emissive,omitempty"`
}

func readMaterialChannels(owner *yaml.Node, path string, add errSink) []RoomMaterialChannelAdjustment {
	raw := childNode(owner, "materialChannels")
	if raw == nil {
		return nil
	}
	path += ".materialChannels"
	node := resolveNode(raw)
	if node == nil || isNull(node) {
		add(path, errNotNull)
		return nil
	}
	if node.Kind != yaml.SequenceNode {
		add(path, errNotAList)
		return nil
	}
	out := make([]RoomMaterialChannelAdjustment, 0, len(node.Content))
	for i, raw := range node.Content {
		at := fmt.Sprintf("%s[%d]", path, i)
		item := resolveNode(raw)
		if item == nil || isNull(item) {
			add(at, errNotNull)
			continue
		}
		if item.Kind != yaml.MappingNode {
			add(at, errNotAMapping)
			continue
		}
		id := requireString(item, "targetId", at, add)
		if id == nil {
			continue
		}
		value := RoomMaterialChannelAdjustment{TargetID: id.Value}
		for _, field := range []struct {
			name string
			out  **RoomChannelAdjustment
		}{
			{"normal", &value.Normal}, {"emissive", &value.Emissive},
		} {
			channelRaw := childNode(item, field.name)
			if channelRaw == nil {
				continue
			}
			where := at + "." + field.name
			channel := resolveNode(channelRaw)
			if channel == nil || isNull(channel) {
				add(where, errNotNull)
				continue
			}
			if channel.Kind != yaml.MappingNode {
				add(where, errNotAMapping)
				continue
			}
			mode := requireString(channel, "mode", where, add)
			if mode == nil {
				continue
			}
			decoded := &RoomChannelAdjustment{Mode: encounter.ChannelAdjustmentMode(mode.Value)}
			switch decoded.Mode {
			case encounter.ChannelOff:
				if childNode(channel, "multiplier") != nil {
					add(where+".multiplier", "Off cannot carry a multiplier")
				}
			case encounter.ChannelScale:
				if number := requireNumber(channel, "multiplier", where, add); number != nil {
					var multiplier float64
					if err := number.Decode(&multiplier); err != nil {
						add(where+".multiplier", errNotANumber)
					} else {
						decoded.Multiplier = &multiplier
					}
				}
			default:
				add(where+".mode", "must be off or scale; omit default channels")
			}
			*field.out = decoded
		}
		out = append(out, value)
	}
	materialChannelValues(out, path, add)
	return out
}

func materialChannelValues(values []RoomMaterialChannelAdjustment, path string, add errSink) {
	seen := make(map[string]bool, len(values))
	for i, value := range values {
		at := fmt.Sprintf("%s[%d]", path, i)
		if strings.TrimSpace(value.TargetID) == "" {
			add(at+".targetId", errRequired)
		}
		if seen[value.TargetID] {
			add(at+".targetId", "duplicate material channel target")
		}
		seen[value.TargetID] = true
		if value.Normal == nil && value.Emissive == nil {
			add(at, "must contain a channel adjustment")
		}
		for _, field := range []struct {
			name  string
			value *RoomChannelAdjustment
		}{
			{"normal", value.Normal}, {"emissive", value.Emissive},
		} {
			if field.value == nil {
				continue
			}
			where := at + "." + field.name
			switch field.value.Mode {
			case encounter.ChannelOff:
				if field.value.Multiplier != nil {
					add(where+".multiplier", "Off cannot carry a multiplier")
				}
			case encounter.ChannelScale:
				if field.value.Multiplier == nil || *field.value.Multiplier <= 0 || math.IsNaN(*field.value.Multiplier) || math.IsInf(*field.value.Multiplier, 0) {
					add(where+".multiplier", "must be finite and positive")
				}
			default:
				add(where+".mode", "must be off or scale")
			}
		}
	}
}

func materialChannelsOf(values []RoomMaterialChannelAdjustment) []encounter.MaterialChannelAdjustment {
	if len(values) == 0 {
		return nil
	}
	out := make([]encounter.MaterialChannelAdjustment, len(values))
	for i, value := range values {
		out[i].TargetID = value.TargetID
		for _, field := range []struct {
			source *RoomChannelAdjustment
			target **encounter.ChannelAdjustment
		}{
			{value.Normal, &out[i].Normal}, {value.Emissive, &out[i].Emissive},
		} {
			if field.source == nil {
				continue
			}
			channel := &encounter.ChannelAdjustment{Mode: field.source.Mode}
			if field.source.Multiplier != nil {
				channel.Multiplier = *field.source.Multiplier
			}
			*field.target = channel
		}
	}
	return out
}
