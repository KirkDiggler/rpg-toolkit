// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/stretchr/testify/suite"
)

type PropPresentationDecodeSuite struct{ suite.Suite }

func TestPropPresentationDecodeSuite(t *testing.T) { suite.Run(t, new(PropPresentationDecodeSuite)) }

const validPresentationRow = `{"id":"books","ref":"dnd5e:props:books","origin":{"x":0,"y":3},"elevation":2,"facing_degrees":-30,"height_scale":1.5,"point_light":{"enabled":true,"offset":{"x":1,"y":2},"offset_elevation":3,"color":"#ffffff","intensity":2,"range":4}}`

func (s *PropPresentationDecodeSuite) TestBothRevealKindsCarryFullRecords() {
	for _, beat := range []string{"room_revealed", "concealment_revealed"} {
		payload := fmt.Sprintf(`{"beat":%q,"region":{"id":"room"},"concealment":"secret","prop_presentations":[%s]}`, beat, validPresentationRow)
		_, body := decodeBeat([]byte(payload))
		s.Require().NotNil(body)
		raw, err := json.Marshal(body)
		s.Require().NoError(err)
		rows, ok := propPresentationsFromPayload(raw)
		s.Require().True(ok)
		s.Require().Len(rows, 1)
		s.Equal("books", rows[0].ID)
		s.Equal(1.5, rows[0].HeightScale)
		s.Equal(3.0, rows[0].PointLight.OffsetElevation)
	}
}
func (s *PropPresentationDecodeSuite) TestMalformedRowsRefuseAtomicBody() {
	for _, row := range []string{
		`null`, strings.Replace(validPresentationRow, `"id":"books"`, `"id":""`, 1),
		strings.Replace(validPresentationRow, `"origin":{"x":0,"y":3}`, `"origin":null`, 1),
		strings.Replace(validPresentationRow, `"height_scale":1.5`, `"height_scale":0`, 1),
		strings.Replace(validPresentationRow, `"offset":{"x":1,"y":2}`, `"offset":null`, 1),
		strings.Replace(validPresentationRow, `"color":"#ffffff"`, `"color":"bad"`, 1),
		strings.Replace(validPresentationRow, `"range":4`, `"range":0`, 1),
		validPresentationRow + "," + validPresentationRow,
	} {
		for _, beat := range []string{"room_revealed", "concealment_revealed"} {
			_, body := decodeBeat([]byte(fmt.Sprintf(`{"beat":%q,"region":{"id":"room"},"concealment":"secret","structural_doors":[{"id":"valid"}],"prop_presentations":[%s]}`, beat, row)))
			s.Nil(body)
		}
	}
}
func (s *PropPresentationDecodeSuite) TestExclusiveDoorChannelsAndStandaloneIdentity() {
	for _, beat := range []string{"room_revealed", "concealment_revealed"} {
		standalone := strings.Replace(validPresentationRow, `"id":"books"`, `"id":"books","door_id":"world/door"`, 1)
		payload := fmt.Sprintf(`{"beat":%q,"region":{"id":"room"},"concealment":"secret","prop_presentations":[%s]}`, beat, standalone)
		_, body := decodeBeat([]byte(payload))
		s.Require().NotNil(body)
		raw, err := json.Marshal(body)
		s.Require().NoError(err)
		rows, ok := propPresentationsFromPayload(raw)
		s.Require().True(ok)
		s.Equal("world/door", rows[0].DoorID)
		for _, suffix := range []string{
			`"structural_doors":[{"id":"other"},{"id":"world/door"}],"prop_presentations":[` + standalone + `]`,
			`"prop_presentations":[` + standalone + `,` + strings.Replace(standalone, `"id":"books"`, `"id":"other"`, 1) + `]`,
		} {
			_, rejected := decodeBeat([]byte(fmt.Sprintf(`{"beat":%q,"region":{"id":"room"},"concealment":"secret",%s}`, beat, suffix)))
			s.Nil(rejected, "door collision refuses the whole body on either reveal path")
		}
	}
}

func (s *PropPresentationDecodeSuite) TestSnapshotProjectionDoesNotAliasLight() {
	in := encounter.Atlas{PropPresentations: []encounter.PropPresentation{{ID: "p", Ref: "r", HeightScale: 1, PointLight: &encounter.PropPointLight{Color: "#ffffff", Range: 4}}}}
	out := projectAtlas(in)
	s.Require().Len(out.PropPresentations, 1)
	out.PropPresentations[0].PointLight.Range = 99
	s.Equal(4.0, in.PropPresentations[0].PointLight.Range)
	rows, ok := propPresentationsFromPayload([]byte(`{}`))
	s.True(ok)
	s.Nil(rows)
}
