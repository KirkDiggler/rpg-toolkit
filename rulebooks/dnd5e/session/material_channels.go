// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"math"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// ChannelAdjustmentMode is the host-owned spelling of captured visual intent.
// Default is absence, not a third mode or a fabricated multiplier.
type ChannelAdjustmentMode string

const (
	// ChannelOff explicitly suppresses a supported authored contribution.
	ChannelOff ChannelAdjustmentMode = "off"
	// ChannelScale retains an explicit positive dimensionless multiplier.
	ChannelScale ChannelAdjustmentMode = "scale"
)

// ChannelAdjustment is a detached host value. Off has no Multiplier; Scale has
// a finite positive one. It supplies no resource or illumination interpretation.
type ChannelAdjustment struct {
	Mode       ChannelAdjustmentMode `json:"mode"`
	Multiplier *float64              `json:"multiplier,omitempty"`
}

// MaterialChannelAdjustment belongs to the same permitted/observed appearance
// as its containing wall, prop, frame or member. Nil channels mean default.
type MaterialChannelAdjustment struct {
	TargetID string             `json:"target_id"`
	Normal   *ChannelAdjustment `json:"normal,omitempty"`
	Emissive *ChannelAdjustment `json:"emissive,omitempty"`
}

func projectChannelAdjustment(value *encounter.ChannelAdjustment) *ChannelAdjustment {
	if value == nil {
		return nil
	}
	out := &ChannelAdjustment{Mode: ChannelAdjustmentMode(value.Mode)}
	if value.Mode == encounter.ChannelScale || value.Multiplier != 0 {
		multiplier := value.Multiplier
		out.Multiplier = &multiplier
	}
	return out
}

func projectMaterialChannels(values []encounter.MaterialChannelAdjustment) []MaterialChannelAdjustment {
	if len(values) == 0 {
		return nil
	}
	out := make([]MaterialChannelAdjustment, len(values))
	for i, value := range values {
		out[i] = MaterialChannelAdjustment{TargetID: value.TargetID,
			Normal: projectChannelAdjustment(value.Normal), Emissive: projectChannelAdjustment(value.Emissive)}
	}
	return out
}

func validMaterialChannels(values []MaterialChannelAdjustment) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.TargetID) == "" || seen[value.TargetID] || (value.Normal == nil && value.Emissive == nil) {
			return false
		}
		seen[value.TargetID] = true
		for _, channel := range []*ChannelAdjustment{value.Normal, value.Emissive} {
			if channel == nil {
				continue
			}
			switch channel.Mode {
			case ChannelOff:
				if channel.Multiplier != nil {
					return false
				}
			case ChannelScale:
				if channel.Multiplier == nil || *channel.Multiplier <= 0 || math.IsNaN(*channel.Multiplier) || math.IsInf(*channel.Multiplier, 0) {
					return false
				}
			default:
				return false
			}
		}
	}
	return true
}

// UnmarshalJSON delegates canonical presence/field validation to its owner, then
// copies into SDK-owned types. No encounter type crosses the exported boundary.
func (value *MaterialChannelAdjustment) UnmarshalJSON(payload []byte) error {
	var decoded encounter.MaterialChannelAdjustmentData
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return err
	}
	out := MaterialChannelAdjustment{TargetID: decoded.TargetID}
	for _, channel := range []struct {
		source *encounter.ChannelAdjustmentData
		target **ChannelAdjustment
	}{
		{decoded.Normal, &out.Normal}, {decoded.Emissive, &out.Emissive},
	} {
		if channel.source == nil {
			continue
		}
		copy := &ChannelAdjustment{Mode: ChannelAdjustmentMode(channel.source.Mode)}
		if channel.source.Multiplier != nil {
			multiplier := *channel.source.Multiplier
			copy.Multiplier = &multiplier
		}
		*channel.target = copy
	}
	*value = out
	return nil
}
