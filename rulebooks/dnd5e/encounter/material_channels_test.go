// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

func channelIntent() []encounter.MaterialChannelAdjustment {
	return []encounter.MaterialChannelAdjustment{{TargetID: "masonry",
		Normal:   &encounter.ChannelAdjustment{Mode: encounter.ChannelScale, Multiplier: .5},
		Emissive: &encounter.ChannelAdjustment{Mode: encounter.ChannelOff}}}
}

func (s *StructuralWallSuite) TestMaterialChannelsLegacySiteHash() {
	// Measured before static channel fields were introduced. Omitted optional
	// intent must not change existing persisted site identity.
	s.Equal("7c7fa22b3396bb0046e60861744839adc708d99b7cdf2e8bd8a76d6134fc5660", string(s.play(structuralField()).Site().Hash()))
	field := structuralField()
	field.StructuralWalls[0].MaterialChannels = []encounter.MaterialChannelAdjustment{}
	s.Equal(s.play(structuralField()).Site().Hash(), s.play(field).Site().Hash())
}

func (s *StructuralWallSuite) TestMaterialChannelsStaticOwnersDetachAndReload() {
	field := structuralField()
	field.StructuralWalls[0].MaterialChannels = channelIntent()
	field.StructuralWalls[0].Openings[0].Door.FramePresentation = framePresentation()
	field.StructuralWalls[0].Openings[0].Door.FramePresentation.MaterialChannels = channelIntent()
	field.PropPresentations = []encounter.PropPresentation{{ID: "decor", Ref: "test:prop", HeightScale: 1, MaterialChannels: channelIntent()}}
	enc := s.play(field)
	field.StructuralWalls[0].MaterialChannels[0].Normal.Multiplier = 9
	field.StructuralWalls[0].Openings[0].Door.FramePresentation.MaterialChannels[0].Normal.Multiplier = 9
	field.PropPresentations[0].MaterialChannels[0].Normal.Multiplier = 9
	full, err := enc.Atlas()
	s.Require().NoError(err)
	s.Equal(channelIntent(), full.StructuralWalls[0].MaterialChannels)
	view, err := enc.AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(channelIntent(), view.StructuralWalls[0].MaterialChannels)
	s.Equal(channelIntent(), view.StructuralDoors[0].FramePresentation.MaterialChannels)
	s.Equal(channelIntent(), view.PropPresentations[0].MaterialChannels)
	view.StructuralWalls[0].MaterialChannels[0].Normal.Multiplier = 7
	view.StructuralDoors[0].FramePresentation.MaterialChannels[0].Normal.Multiplier = 7
	view.PropPresentations[0].MaterialChannels[0].Normal.Multiplier = 7
	stored := enc.Site().Data()
	*stored.Field.StructuralWalls[0].MaterialChannels[0].Normal.Multiplier = 8
	*stored.Field.StructuralWalls[0].Openings[0].Door.FramePresentation.MaterialChannels[0].Normal.Multiplier = 8
	*stored.Field.PropPresentations[0].MaterialChannels[0].Normal.Multiplier = 8
	loaded, err := s.reload(enc).AtlasFor("walker")
	s.Require().NoError(err)
	s.Equal(channelIntent(), loaded.StructuralWalls[0].MaterialChannels)
	s.Equal(channelIntent(), loaded.StructuralDoors[0].FramePresentation.MaterialChannels)
	s.Equal(channelIntent(), loaded.PropPresentations[0].MaterialChannels)
	s.Equal(full.Placed, loaded.Placed)
}

func (s *StructuralWallSuite) TestMaterialChannelsFrameDisclosureNeedsNoParent() {
	field := concealedStructuralField([]encounter.PropID{"wall-presence"}, nil, nil)
	field.StructuralWalls[0].MaterialChannels = channelIntent()
	field.StructuralWalls[0].Openings[0].Door.FramePresentation = framePresentation()
	field.StructuralWalls[0].Openings[0].Door.FramePresentation.MaterialChannels = channelIntent()
	view, err := s.play(field).AtlasFor("walker")
	s.Require().NoError(err)
	s.Empty(view.StructuralWalls)
	s.Require().Len(view.StructuralDoors, 1)
	s.Equal(channelIntent(), view.StructuralDoors[0].FramePresentation.MaterialChannels)
	payload, err := json.Marshal(view.StructuralDoors[0].FramePresentation)
	s.Require().NoError(err)
	s.NotContains(string(payload), "wall-presence")
}

func (s *StructuralWallSuite) TestMaterialChannelsMalformedStoredIntentRefusesBeforeHashing() {
	for _, owner := range []string{"wall", "prop", "frame"} {
		s.Run(owner, func() {
			field := structuralField()
			field.StructuralWalls[0].MaterialChannels = channelIntent()
			field.StructuralWalls[0].Openings[0].Door.FramePresentation = framePresentation()
			field.StructuralWalls[0].Openings[0].Door.FramePresentation.MaterialChannels = channelIntent()
			field.PropPresentations = []encounter.PropPresentation{{ID: "decor", Ref: "test:prop", HeightScale: 1, MaterialChannels: channelIntent()}}
			enc := s.play(field)
			site := enc.Site().Data()
			row := site.Field.StructuralWalls[0].MaterialChannels[0].Normal
			if owner == "prop" {
				row = site.Field.PropPresentations[0].MaterialChannels[0].Normal
			}
			if owner == "frame" {
				row = site.Field.StructuralWalls[0].Openings[0].Door.FramePresentation.MaterialChannels[0].Normal
			}
			row.Mode = encounter.ChannelOff // The old multiplier is still explicitly present.
			_, err := loadOverSite(enc.ToData(), site, encounter.LoadEncounterInput{Capabilities: encounter.RefusingCapabilities()})
			s.Require().Error(err)
			s.Contains(err.Error(), "material channel")
		})
	}
}

func (s *StructuralWallSuite) TestMaterialChannelsMalformedStaticIntentRefuses() {
	for _, owner := range []string{"wall", "prop", "frame"} {
		s.Run(owner, func() {
			field := structuralField()
			bad := []encounter.MaterialChannelAdjustment{{TargetID: "empty"}}
			switch owner {
			case "wall":
				field.StructuralWalls[0].MaterialChannels = bad
			case "prop":
				field.PropPresentations = []encounter.PropPresentation{{ID: "decor", Ref: "test:prop", HeightScale: 1, MaterialChannels: bad}}
			case "frame":
				field.StructuralWalls[0].Openings[0].Door.FramePresentation = framePresentation()
				field.StructuralWalls[0].Openings[0].Door.FramePresentation.MaterialChannels = bad
			}
			_, err := s.build(field)
			s.ErrorIs(err, encounter.ErrNoField)
		})
	}
}
