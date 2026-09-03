// Package customization defines provider-neutral character appearance intent.
package customization

import (
	"math"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
)

const (
	maxStyleRefBytes = 256
	maxColorSRGB     = 0xFFFFFF
)

// StyleSelectionKind identifies whether a customization slot selects a
// provider-owned style or explicitly requests no style.
type StyleSelectionKind string

const (
	// StyleSelectionStyle selects the provider-owned style named by StyleRef.
	StyleSelectionStyle StyleSelectionKind = "style"
	// StyleSelectionNone explicitly removes the style from the customization slot.
	StyleSelectionNone StyleSelectionKind = "none"
)

// StyleSelection distinguishes an exact opaque provider style reference from
// an explicit request for no style.
type StyleSelection struct {
	Kind     StyleSelectionKind `json:"kind"`
	StyleRef string             `json:"style_ref,omitempty"`
}

// HairCustomization stores optional provider-neutral hair rendering intent.
// An omitted slot or scalar leaves that aspect to the provider's default.
type HairCustomization struct {
	Scalp      *StyleSelection `json:"scalp,omitempty"`
	FacialHair *StyleSelection `json:"facial_hair,omitempty"`
	ColorSRGB  *uint32         `json:"color_srgb,omitempty"`
	Roughness  *float32        `json:"roughness,omitempty"`
}

// OutfitCustomization stores independently optional packed sRGB colors for
// the provider-authored primary and secondary outfit channels.
type OutfitCustomization struct {
	PrimaryColorSRGB   *uint32 `json:"primary_color_srgb,omitempty"`
	SecondaryColorSRGB *uint32 `json:"secondary_color_srgb,omitempty"`
}

// Appearance represents provider-neutral cosmetic character customization.
// Nil nested values preserve the provider's defaults or authored outfit colors.
type Appearance struct {
	Hair   *HairCustomization   `json:"hair,omitempty"`
	Outfit *OutfitCustomization `json:"outfit,omitempty"`
}

// ValidateAppearance validates the neutral shape and scalar bounds of an
// appearance. Nil and empty appearances are valid persistence values. It does
// not inspect provider membership, paths, or defaults, and never mutates the
// input.
func ValidateAppearance(appearance *Appearance) error {
	if appearance == nil {
		return nil
	}

	if appearance.Hair != nil {
		if err := validateStyleSelection("appearance.hair.scalp", appearance.Hair.Scalp); err != nil {
			return err
		}
		if err := validateStyleSelection("appearance.hair.facial_hair", appearance.Hair.FacialHair); err != nil {
			return err
		}
		if appearance.Hair.ColorSRGB != nil && *appearance.Hair.ColorSRGB > maxColorSRGB {
			return invalidArgument("appearance.hair.color_srgb must be between 0 and 0xFFFFFF")
		}
		if appearance.Hair.Roughness != nil && !validRoughness(*appearance.Hair.Roughness) {
			return invalidArgument("appearance.hair.roughness must be finite and between 0 and 1")
		}
	}

	if appearance.Outfit != nil {
		if appearance.Outfit.PrimaryColorSRGB != nil && *appearance.Outfit.PrimaryColorSRGB > maxColorSRGB {
			return invalidArgument("appearance.outfit.primary_color_srgb must be between 0 and 0xFFFFFF")
		}
		if appearance.Outfit.SecondaryColorSRGB != nil && *appearance.Outfit.SecondaryColorSRGB > maxColorSRGB {
			return invalidArgument("appearance.outfit.secondary_color_srgb must be between 0 and 0xFFFFFF")
		}
	}

	return nil
}

func validateStyleSelection(field string, selection *StyleSelection) error {
	if selection == nil {
		return nil
	}

	switch selection.Kind {
	case "":
		return invalidArgument(field + ".selection is required")
	case StyleSelectionStyle:
		if selection.StyleRef == "" {
			return invalidArgument(field + ".style_ref is required")
		}
		if len(selection.StyleRef) > maxStyleRefBytes {
			return invalidArgument(field + ".style_ref must be at most 256 bytes")
		}
	case StyleSelectionNone:
		if selection.StyleRef != "" {
			return invalidArgument(field + ".style_ref must be empty")
		}
	default:
		return invalidArgument(field + ".selection is invalid")
	}

	return nil
}

func validRoughness(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0) && value >= 0 && value <= 1
}

func invalidArgument(message string) error {
	return rpgerr.New(rpgerr.CodeInvalidArgument, message)
}

// CloneAppearance returns a deep copy of an appearance, preserving nil and
// present-zero values. A nil input returns nil.
func CloneAppearance(appearance *Appearance) *Appearance {
	if appearance == nil {
		return nil
	}

	clone := *appearance
	clone.Hair = cloneHairCustomization(appearance.Hair)
	clone.Outfit = cloneOutfitCustomization(appearance.Outfit)
	return &clone
}

func cloneHairCustomization(hair *HairCustomization) *HairCustomization {
	if hair == nil {
		return nil
	}

	clone := *hair
	clone.Scalp = cloneStyleSelection(hair.Scalp)
	clone.FacialHair = cloneStyleSelection(hair.FacialHair)
	clone.ColorSRGB = cloneUint32(hair.ColorSRGB)
	clone.Roughness = cloneFloat32(hair.Roughness)
	return &clone
}

func cloneOutfitCustomization(outfit *OutfitCustomization) *OutfitCustomization {
	if outfit == nil {
		return nil
	}

	clone := *outfit
	clone.PrimaryColorSRGB = cloneUint32(outfit.PrimaryColorSRGB)
	clone.SecondaryColorSRGB = cloneUint32(outfit.SecondaryColorSRGB)
	return &clone
}

func cloneStyleSelection(selection *StyleSelection) *StyleSelection {
	if selection == nil {
		return nil
	}

	clone := *selection
	return &clone
}

func cloneUint32(value *uint32) *uint32 {
	if value == nil {
		return nil
	}

	clone := *value
	return &clone
}

func cloneFloat32(value *float32) *float32 {
	if value == nil {
		return nil
	}

	clone := *value
	return &clone
}
