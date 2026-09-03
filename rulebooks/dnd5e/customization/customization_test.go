package customization_test

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/customization"
)

func TestValidateAppearanceAcceptsValidAppearance(t *testing.T) {
	maxStyleRef := strings.Repeat("é", 128)
	zeroColor := uint32(0)
	maxColor := uint32(0xFFFFFF)
	zeroRoughness := float32(0)
	oneRoughness := float32(1)

	appearance := &customization.Appearance{
		Hair: &customization.HairCustomization{
			Scalp: &customization.StyleSelection{
				Kind:     customization.StyleSelectionStyle,
				StyleRef: maxStyleRef,
			},
			FacialHair: &customization.StyleSelection{Kind: customization.StyleSelectionNone},
			ColorSRGB:  &zeroColor,
			Roughness:  &zeroRoughness,
		},
		Outfit: &customization.OutfitCustomization{
			PrimaryColorSRGB:   &zeroColor,
			SecondaryColorSRGB: &maxColor,
		},
	}

	require.Len(t, []byte(maxStyleRef), 256)
	require.NoError(t, customization.ValidateAppearance(nil))
	require.NoError(t, customization.ValidateAppearance(&customization.Appearance{}))
	require.NoError(t, customization.ValidateAppearance(appearance))

	appearance.Hair.Roughness = &oneRoughness
	require.NoError(t, customization.ValidateAppearance(appearance))
}

func TestValidateAppearanceAcceptsUnknownWellShapedProviderReference(t *testing.T) {
	appearance := &customization.Appearance{
		Hair: &customization.HairCustomization{
			Scalp: &customization.StyleSelection{
				Kind:     customization.StyleSelectionStyle,
				StyleRef: "unknown-provider/ref-that-the-toolkit-does-not-interpret",
			},
		},
	}

	require.NoError(t, customization.ValidateAppearance(appearance))
}

func TestValidateAppearanceRefusesMalformedStyleAndColorFields(t *testing.T) {
	longStyleRef := strings.Repeat("é", 129)

	tests := []struct {
		name       string
		appearance *customization.Appearance
		message    string
	}{
		{
			name: "empty scalp kind",
			appearance: &customization.Appearance{Hair: &customization.HairCustomization{
				Scalp: &customization.StyleSelection{},
			}},
			message: "appearance.hair.scalp.selection is required",
		},
		{
			name: "unknown scalp kind",
			appearance: &customization.Appearance{Hair: &customization.HairCustomization{
				Scalp: &customization.StyleSelection{Kind: customization.StyleSelectionKind("catalog")},
			}},
			message: "appearance.hair.scalp.selection is invalid",
		},
		{
			name: "empty style ref",
			appearance: &customization.Appearance{Hair: &customization.HairCustomization{
				Scalp: &customization.StyleSelection{Kind: customization.StyleSelectionStyle},
			}},
			message: "appearance.hair.scalp.style_ref is required",
		},
		{
			name: "style ref over 256 UTF-8 bytes",
			appearance: &customization.Appearance{Hair: &customization.HairCustomization{
				Scalp: &customization.StyleSelection{
					Kind:     customization.StyleSelectionStyle,
					StyleRef: longStyleRef,
				},
			}},
			message: "appearance.hair.scalp.style_ref must be at most 256 bytes",
		},
		{
			name: "none with a style ref",
			appearance: &customization.Appearance{Hair: &customization.HairCustomization{
				FacialHair: &customization.StyleSelection{
					Kind:     customization.StyleSelectionNone,
					StyleRef: "must-not-be-present",
				},
			}},
			message: "appearance.hair.facial_hair.style_ref must be empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := customization.ValidateAppearance(test.appearance)
			require.Error(t, err)
			require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
			require.Equal(t, test.message, err.Error())
		})
	}
}

func TestValidateAppearanceRefusesRGB24Overflow(t *testing.T) {
	tooBright := uint32(0x1000000)

	tests := []struct {
		name       string
		appearance *customization.Appearance
		message    string
	}{
		{
			name: "hair color",
			appearance: &customization.Appearance{Hair: &customization.HairCustomization{
				ColorSRGB: &tooBright,
			}},
			message: "appearance.hair.color_srgb must be between 0 and 0xFFFFFF",
		},
		{
			name: "outfit primary color",
			appearance: &customization.Appearance{Outfit: &customization.OutfitCustomization{
				PrimaryColorSRGB: &tooBright,
			}},
			message: "appearance.outfit.primary_color_srgb must be between 0 and 0xFFFFFF",
		},
		{
			name: "outfit secondary color",
			appearance: &customization.Appearance{Outfit: &customization.OutfitCustomization{
				SecondaryColorSRGB: &tooBright,
			}},
			message: "appearance.outfit.secondary_color_srgb must be between 0 and 0xFFFFFF",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := customization.ValidateAppearance(test.appearance)
			require.Error(t, err)
			require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
			require.Equal(t, test.message, err.Error())
		})
	}
}

func TestValidateAppearanceRefusesInvalidRoughness(t *testing.T) {
	tests := []struct {
		name      string
		roughness float32
	}{
		{name: "below zero", roughness: -0.0001},
		{name: "above one", roughness: 1.0001},
		{name: "NaN", roughness: float32(math.NaN())},
		{name: "positive infinity", roughness: float32(math.Inf(1))},
		{name: "negative infinity", roughness: float32(math.Inf(-1))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appearance := &customization.Appearance{Hair: &customization.HairCustomization{
				Roughness: &test.roughness,
			}}

			err := customization.ValidateAppearance(appearance)
			require.Error(t, err)
			require.Equal(t, rpgerr.CodeInvalidArgument, rpgerr.GetCode(err))
			require.Equal(t,
				"appearance.hair.roughness must be finite and between 0 and 1",
				err.Error(),
			)
		})
	}
}

func TestValidateAppearanceDoesNotMutateInput(t *testing.T) {
	color := uint32(0)
	roughness := float32(0)
	appearance := &customization.Appearance{
		Hair: &customization.HairCustomization{
			Scalp: &customization.StyleSelection{
				Kind:     customization.StyleSelectionStyle,
				StyleRef: "unknown-provider:hair:38",
			},
			FacialHair: &customization.StyleSelection{Kind: customization.StyleSelectionNone},
			ColorSRGB:  &color,
			Roughness:  &roughness,
		},
		Outfit: &customization.OutfitCustomization{
			PrimaryColorSRGB:   &color,
			SecondaryColorSRGB: &color,
		},
	}
	before := customization.CloneAppearance(appearance)

	require.NoError(t, customization.ValidateAppearance(appearance))
	require.Equal(t, before, appearance)
}

func TestCloneAppearancePreservesValuesAndIsolatesEveryNestedPointer(t *testing.T) {
	zeroColor := uint32(0)
	maxColor := uint32(0xFFFFFF)
	zeroRoughness := float32(0)
	appearance := &customization.Appearance{
		Hair: &customization.HairCustomization{
			Scalp: &customization.StyleSelection{
				Kind:     customization.StyleSelectionStyle,
				StyleRef: "provider:hair:38",
			},
			FacialHair: &customization.StyleSelection{Kind: customization.StyleSelectionNone},
			ColorSRGB:  &zeroColor,
			Roughness:  &zeroRoughness,
		},
		Outfit: &customization.OutfitCustomization{
			PrimaryColorSRGB:   &zeroColor,
			SecondaryColorSRGB: &maxColor,
		},
	}

	clone := customization.CloneAppearance(appearance)
	require.Equal(t, appearance, clone)
	require.NotSame(t, appearance, clone)
	require.NotSame(t, appearance.Hair, clone.Hair)
	require.NotSame(t, appearance.Hair.Scalp, clone.Hair.Scalp)
	require.NotSame(t, appearance.Hair.FacialHair, clone.Hair.FacialHair)
	require.NotSame(t, appearance.Hair.ColorSRGB, clone.Hair.ColorSRGB)
	require.NotSame(t, appearance.Hair.Roughness, clone.Hair.Roughness)
	require.NotSame(t, appearance.Outfit, clone.Outfit)
	require.NotSame(t, appearance.Outfit.PrimaryColorSRGB, clone.Outfit.PrimaryColorSRGB)
	require.NotSame(t, appearance.Outfit.SecondaryColorSRGB, clone.Outfit.SecondaryColorSRGB)

	clone.Hair.Scalp.StyleRef = "changed:hair"
	clone.Hair.FacialHair.Kind = customization.StyleSelectionStyle
	clone.Hair.FacialHair.StyleRef = "changed:facial-hair"
	*clone.Hair.ColorSRGB = 0x123456
	*clone.Hair.Roughness = 1
	*clone.Outfit.PrimaryColorSRGB = 0x654321
	*clone.Outfit.SecondaryColorSRGB = 0

	require.Equal(t, "provider:hair:38", appearance.Hair.Scalp.StyleRef)
	require.Equal(t, customization.StyleSelectionNone, appearance.Hair.FacialHair.Kind)
	require.Empty(t, appearance.Hair.FacialHair.StyleRef)
	require.Zero(t, *appearance.Hair.ColorSRGB)
	require.Zero(t, *appearance.Hair.Roughness)
	require.Zero(t, *appearance.Outfit.PrimaryColorSRGB)
	require.Equal(t, uint32(0xFFFFFF), *appearance.Outfit.SecondaryColorSRGB)
}

func TestCloneAppearancePreservesNilAndEmptyValues(t *testing.T) {
	require.Nil(t, customization.CloneAppearance(nil))

	empty := &customization.Appearance{}
	clone := customization.CloneAppearance(empty)
	require.Equal(t, empty, clone)
	require.NotSame(t, empty, clone)
}
