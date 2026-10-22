// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

type PropPresentationDecodeSuite struct{ suite.Suite }

func TestPropPresentationDecodeSuite(t *testing.T) { suite.Run(t, new(PropPresentationDecodeSuite)) }

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
