// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
)

type PresentationObservationDecodeSuite struct{ suite.Suite }

func TestPresentationObservationDecodeSuite(t *testing.T) {
	suite.Run(t, new(PresentationObservationDecodeSuite))
}
func (s *PresentationObservationDecodeSuite) TestConflictingCapturedPresentationIsRefused() {
	for _, mode := range []string{"empty", "identity", "scale", "light"} {
		s.Run(mode, func() {
			p := propObservation{Prop: &AtlasProp{ID: "p"}, Presentation: &PropPresentationData{ID: "p", Ref: "test:props:book", HeightScale: 1}}
			switch mode {
			case "empty":
				p.ObservedEmpty = true
			case "identity":
				p.Presentation.ID = "other"
			case "scale":
				p.Presentation.HeightScale = 0
			case "light":
				p.Presentation.PointLight = &PropPointLightData{Color: "bad", Range: 1}
			}
			raw, err := json.Marshal(p)
			s.Require().NoError(err)
			_, err = decodePropObservation(raw)
			s.ErrorIs(err, ErrInvalidData)
		})
	}
	valid, err := decodePropObservation([]byte(`{"prop":{"ID":"p"},"observed_empty":true}`))
	s.Require().NoError(err)
	s.Nil(valid.Presentation)
}
