// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type StructuralPatchDecodeSuite struct{ suite.Suite }

func TestStructuralPatchDecodeSuite(t *testing.T) { suite.Run(t, new(StructuralPatchDecodeSuite)) }

func (s *StructuralPatchDecodeSuite) TestBothRevealKindsPreserveReplacementsAndEmptyDefaults() {
	for _, beat := range []string{"room_revealed", "concealment_revealed"} {
		for name, row := range map[string]string{
			"populated":     `{"wall_id":"wall","openings":[{"id":"gap","position":3,"width":2}]}`,
			"empty":         `{"wall_id":"wall","openings":[]}`,
			"default empty": `{"wall_id":"wall"}`,
		} {
			s.Run(beat+"/"+name, func() {
				payload := fmt.Sprintf(`{"beat":%q,"region":{"id":"room"},"concealment":"secret","structural_wall_openings_replacements":[%s]}`, beat, row)
				_, body := decodeBeat([]byte(payload))
				s.Require().NotNil(body)
				raw, err := json.Marshal(body)
				s.Require().NoError(err)
				var fields map[string]json.RawMessage
				s.Require().NoError(json.Unmarshal(raw, &fields))
				s.Require().Contains(fields, "structural_wall_openings_replacements")
				var patches []struct {
					WallID   string                   `json:"wall_id"`
					Openings []AtlasStructuralOpening `json:"openings"`
				}
				s.Require().NoError(json.Unmarshal(fields["structural_wall_openings_replacements"], &patches))
				s.Require().Len(patches, 1)
				s.Equal("wall", patches[0].WallID)
				if name == "populated" {
					s.Equal([]AtlasStructuralOpening{{ID: "gap", Position: 3, Width: 2}}, patches[0].Openings)
				} else {
					s.Empty(patches[0].Openings, "present default is a clear, not a discarded record")
				}
			})
		}
	}
}

func (s *StructuralPatchDecodeSuite) TestMalformedReplacementsRefuseTheWholeBody() {
	for name, suffix := range map[string]string{
		"empty identity":         `"structural_wall_openings_replacements":[{"openings":[]}]`,
		"wrong type":             `"structural_wall_openings_replacements":true`,
		"duplicate wall":         `"structural_wall_openings_replacements":[{"wall_id":"w"},{"wall_id":"w"}]`,
		"empty opening identity": `"structural_wall_openings_replacements":[{"wall_id":"w","openings":[{"width":1}]}]`,
		"duplicate openings":     `"structural_wall_openings_replacements":[{"wall_id":"w","openings":[{"id":"o"},{"id":"o"}]}]`,
		"full row collision":     `"structural_walls":[{"id":"w"}],"structural_wall_openings_replacements":[{"wall_id":"w"}]`,
	} {
		for _, beat := range []string{"room_revealed", "concealment_revealed"} {
			s.Run(beat+"/"+name, func() {
				payload := fmt.Sprintf(`{"beat":%q,"region":{"id":"room"},"concealment":"secret","structural_doors":[{"id":"known/door"}],%s}`, beat, suffix)
				_, body := decodeBeat([]byte(payload))
				s.Nil(body, "a valid sibling door must not turn a malformed patch into a partial update")
			})
		}
	}
}
