// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
)

// ChannelAdjustmentMode distinguishes explicit suppression from scaling.
// Default is an absent channel, never an empty or invented mode.
type ChannelAdjustmentMode string

const (
	// ChannelOff suppresses the authored contribution without a multiplier.
	ChannelOff ChannelAdjustmentMode = "off"
	// ChannelScale multiplies an authored contribution by a positive finite value.
	ChannelScale ChannelAdjustmentMode = "scale"
)

// ChannelAdjustment is opaque appearance intent, not a lighting or rules fact.
// Off requires Multiplier zero internally; Scale requires a finite positive
// dimensionless multiplier. Resource eligibility belongs to the asset provider.
type ChannelAdjustment struct {
	Mode       ChannelAdjustmentMode
	Multiplier float64
}

// MaterialChannelAdjustment belongs to one captured appearance owner. TargetID
// is opaque and unique within that owner. Nil Normal/Emissive means default;
// at least one channel must be present. Every ownership boundary copies values.
type MaterialChannelAdjustment struct {
	TargetID string
	Normal   *ChannelAdjustment
	Emissive *ChannelAdjustment
}

// ChannelAdjustmentData preserves stored multiplier presence: nil for Off and
// nonnil for Scale, including an explicitly authored one. Explicit null input
// is refused rather than collapsed to absence.
type ChannelAdjustmentData struct {
	Mode       ChannelAdjustmentMode `json:"mode"`
	Multiplier *float64              `json:"multiplier,omitempty"`
}

// MaterialChannelAdjustmentData persists only typed captured visual intent.
// It carries no source paths, parent identities or source-document lookup keys.
type MaterialChannelAdjustmentData struct {
	TargetID string                 `json:"target_id"`
	Normal   *ChannelAdjustmentData `json:"normal,omitempty"`
	Emissive *ChannelAdjustmentData `json:"emissive,omitempty"`
}

func validateChannelAdjustment(value ChannelAdjustment) error {
	switch value.Mode {
	case ChannelOff:
		if value.Multiplier == 0 {
			return nil
		}
	case ChannelScale:
		if value.Multiplier > 0 && !math.IsNaN(value.Multiplier) && !math.IsInf(value.Multiplier, 0) {
			return nil
		}
	}
	return fmt.Errorf("invalid material channel mode or multiplier: %w", ErrNoField)
}

func validateMaterialChannels(values []MaterialChannelAdjustment) error {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if strings.TrimSpace(value.TargetID) == "" || seen[value.TargetID] {
			return fmt.Errorf("material channel target must be nonblank and unique: %w", ErrNoField)
		}
		seen[value.TargetID] = true
		if value.Normal == nil && value.Emissive == nil {
			return fmt.Errorf("material channel target needs an adjustment: %w", ErrNoField)
		}
		for _, channel := range []*ChannelAdjustment{value.Normal, value.Emissive} {
			if channel != nil {
				if err := validateChannelAdjustment(*channel); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func cloneChannelAdjustment(value *ChannelAdjustment) *ChannelAdjustment {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneMaterialChannels(values []MaterialChannelAdjustment) []MaterialChannelAdjustment {
	if len(values) == 0 {
		return nil
	}
	result := make([]MaterialChannelAdjustment, len(values))
	for i, value := range values {
		result[i] = MaterialChannelAdjustment{TargetID: value.TargetID,
			Normal: cloneChannelAdjustment(value.Normal), Emissive: cloneChannelAdjustment(value.Emissive)}
	}
	return result
}

func sameChannelAdjustment(a, b *ChannelAdjustment) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func sameMaterialChannels(a, b []MaterialChannelAdjustment) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].TargetID != b[i].TargetID || !sameChannelAdjustment(a[i].Normal, b[i].Normal) ||
			!sameChannelAdjustment(a[i].Emissive, b[i].Emissive) {
			return false
		}
	}
	return true
}

func channelAdjustmentData(value *ChannelAdjustment) *ChannelAdjustmentData {
	if value == nil {
		return nil
	}
	result := &ChannelAdjustmentData{Mode: value.Mode}
	if value.Mode == ChannelScale {
		multiplier := value.Multiplier
		result.Multiplier = &multiplier
	}
	return result
}

func materialChannelsData(values []MaterialChannelAdjustment) []MaterialChannelAdjustmentData {
	if len(values) == 0 {
		return nil
	}
	result := make([]MaterialChannelAdjustmentData, len(values))
	for i, value := range values {
		result[i] = MaterialChannelAdjustmentData{TargetID: value.TargetID,
			Normal: channelAdjustmentData(value.Normal), Emissive: channelAdjustmentData(value.Emissive)}
	}
	return result
}

func channelAdjustmentFromData(value ChannelAdjustmentData) (ChannelAdjustment, error) {
	if (value.Mode == ChannelOff && value.Multiplier != nil) ||
		(value.Mode == ChannelScale && value.Multiplier == nil) {
		return ChannelAdjustment{}, fmt.Errorf("material channel multiplier presence differs from mode: %w", ErrNoField)
	}
	result := ChannelAdjustment{Mode: value.Mode}
	if value.Multiplier != nil {
		result.Multiplier = *value.Multiplier
	}
	if err := validateChannelAdjustment(result); err != nil {
		return ChannelAdjustment{}, err
	}
	return result, nil
}

func materialChannelsFromData(values []MaterialChannelAdjustmentData) ([]MaterialChannelAdjustment, error) {
	result := make([]MaterialChannelAdjustment, len(values))
	for i, value := range values {
		result[i].TargetID = value.TargetID
		for _, item := range []struct {
			source *ChannelAdjustmentData
			out    **ChannelAdjustment
		}{{value.Normal, &result[i].Normal}, {value.Emissive, &result[i].Emissive}} {
			if item.source == nil {
				continue
			}
			channel, err := channelAdjustmentFromData(*item.source)
			if err != nil {
				return nil, err
			}
			*item.out = &channel
		}
	}
	if err := validateMaterialChannels(result); err != nil {
		return nil, err
	}
	return result, nil
}

func decodeMaterialChannelObject(payload []byte, into any, allowed ...string) error {
	// Like the containing sight payload, a channel has one answer per key.
	// encoding/json alone would silently choose the last duplicate value.
	fields := json.NewDecoder(bytes.NewReader(payload))
	token, err := fields.Token()
	if err != nil || token != json.Delim('{') {
		return fmt.Errorf("material channel must be an object: %w", ErrNoField)
	}
	seen := map[string]bool{}
	for fields.More() {
		token, err = fields.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok || !slices.Contains(allowed, key) || seen[key] {
			return fmt.Errorf("unknown or duplicate material channel field: %w", ErrNoField)
		}
		seen[key] = true
		var raw json.RawMessage
		if err = fields.Decode(&raw); err != nil {
			return err
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		return fmt.Errorf("material channel has trailing JSON: %w", ErrNoField)
	}
	return nil
}

// UnmarshalJSON refuses an explicit null/missing/invalid multiplier instead of
// allowing the JSON pointer zero value to silently turn it into default or Off.
func (value *ChannelAdjustmentData) UnmarshalJSON(payload []byte) error {
	var wire struct {
		Mode       ChannelAdjustmentMode `json:"mode"`
		Multiplier json.RawMessage       `json:"multiplier"`
	}
	if err := decodeMaterialChannelObject(payload, &wire, "mode", "multiplier"); err != nil {
		return err
	}
	result := ChannelAdjustmentData{Mode: wire.Mode}
	if len(wire.Multiplier) > 0 {
		if wire.Mode == ChannelOff || bytes.Equal(bytes.TrimSpace(wire.Multiplier), []byte("null")) {
			return fmt.Errorf("invalid material channel multiplier presence: %w", ErrNoField)
		}
		var multiplier float64
		if err := json.Unmarshal(wire.Multiplier, &multiplier); err != nil {
			return err
		}
		result.Multiplier = &multiplier
	}
	if _, err := channelAdjustmentFromData(result); err != nil {
		return err
	}
	*value = result
	return nil
}

// UnmarshalJSON preserves channel absence and refuses explicit null channel
// objects. Additional source metadata is not part of this persisted value.
func (value *MaterialChannelAdjustmentData) UnmarshalJSON(payload []byte) error {
	var wire struct {
		TargetID string          `json:"target_id"`
		Normal   json.RawMessage `json:"normal"`
		Emissive json.RawMessage `json:"emissive"`
	}
	if err := decodeMaterialChannelObject(payload, &wire, "target_id", "normal", "emissive"); err != nil {
		return err
	}
	result := MaterialChannelAdjustmentData{TargetID: wire.TargetID}
	for _, channel := range []struct {
		raw json.RawMessage
		out **ChannelAdjustmentData
	}{{wire.Normal, &result.Normal}, {wire.Emissive, &result.Emissive}} {
		if len(channel.raw) == 0 {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(channel.raw), []byte("null")) {
			return fmt.Errorf("material channel cannot be null: %w", ErrNoField)
		}
		var decoded ChannelAdjustmentData
		if err := json.Unmarshal(channel.raw, &decoded); err != nil {
			return err
		}
		*channel.out = &decoded
	}
	if _, err := materialChannelsFromData([]MaterialChannelAdjustmentData{result}); err != nil {
		return err
	}
	*value = result
	return nil
}
