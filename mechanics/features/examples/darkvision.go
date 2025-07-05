// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package examples provides example feature implementations.
package examples

import (
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/features"
)

// VisionModifier provides a vision range modifier.
type VisionModifier struct {
	visionType  string
	rangeInFeet int
}

// NewVisionModifier creates a new vision modifier.
func NewVisionModifier(visionType string, rangeInFeet int) *VisionModifier {
	return &VisionModifier{
		visionType:  visionType,
		rangeInFeet: rangeInFeet,
	}
}

// Source returns the source of the modifier.
func (v *VisionModifier) Source() string {
	return v.visionType
}

// Type returns the type of the modifier.
func (v *VisionModifier) Type() string {
	return "vision"
}

// Value returns the modifier value.
func (v *VisionModifier) Value() interface{} {
	return v.rangeInFeet
}

// ModifierValue returns the typed modifier value.
func (v *VisionModifier) ModifierValue() events.ModifierValue {
	return events.NewRawValue(v.rangeInFeet, v.visionType)
}

// Priority returns the priority of the modifier.
func (v *VisionModifier) Priority() int {
	return 100
}

// Condition checks if this modifier should apply.
func (v *VisionModifier) Condition(event events.Event) bool {
	// Vision modifiers apply to perception/vision checks
	return event.Type() == events.EventOnAbilityCheck || event.Type() == "vision_check"
}

// Duration returns how long this modifier lasts.
func (v *VisionModifier) Duration() events.Duration {
	return nil // Permanent
}

// SourceDetails returns rich source information.
func (v *VisionModifier) SourceDetails() *events.ModifierSource {
	return &events.ModifierSource{
		Type:        "racial",
		Name:        "Darkvision",
		Description: "You can see in dim light as if it were bright light, and in darkness as if it were dim light",
	}
}

// CreateDarkvisionFeature creates the Darkvision racial feature.
func CreateDarkvisionFeature(rangeInFeet int) features.Feature {
	return features.NewBasicFeature("darkvision", "Darkvision").
		WithDescription("You can see in dim light within 60 feet of you as if it were bright light, " +
			"and in darkness as if it were dim light.").
		WithType(features.FeatureRacial).
		WithLevel(0). // Available from character creation
		WithSource("Racial").
		WithTiming(features.TimingPassive).
		WithModifiers(NewVisionModifier("darkvision", rangeInFeet))
}

// CreateHalfOrcDarkvision creates the Half-Orc version of Darkvision.
func CreateHalfOrcDarkvision() features.Feature {
	feature := CreateDarkvisionFeature(60)
	// We need to cast to BasicFeature to access builder methods
	if basic, ok := feature.(*features.BasicFeature); ok {
		return basic.
			WithSource("Half-Orc").
			WithPrerequisites("race:half-orc")
	}
	return feature
}

// CreateDrowDarkvision creates the Drow version of Superior Darkvision.
func CreateDrowDarkvision() features.Feature {
	return features.NewBasicFeature("superior_darkvision", "Superior Darkvision").
		WithDescription("You can see in dim light within 120 feet of you as if it were bright light, " +
			"and in darkness as if it were dim light.").
		WithType(features.FeatureRacial).
		WithLevel(0).
		WithSource("Drow").
		WithTiming(features.TimingPassive).
		WithModifiers(NewVisionModifier("darkvision", 120)).
		WithPrerequisites("race:drow")
}
