// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// targetBearingSuite reads the registry's bearing on an attack against the
// holder.
type targetBearingSuite struct{ suite.Suite }

func TestTargetBearingSuite(t *testing.T) { suite.Run(t, new(targetBearingSuite)) }

func (s *targetBearingSuite) TestTargetBearingLoadersHaveDescriptions() {
	for _, ref := range TargetBearingRefs() {
		parsed, err := core.ParseString(ref)
		s.Require().NoError(err, ref)
		display, found := DisplayFor(*parsed)
		s.True(found, "%s bears on attacks against its holder but is not registered", ref)
		s.NotEmpty(display.Detail, "%s bears on attacks against its holder but has no description", ref)
	}
}

// TestTargetBearingDescriptionsAreReaderNeutral: a held row's description is
// shown on an attacker's target candidate, so it never speaks to the holder as
// "you" — the attacker reading it is not the one prone, hidden or lit.
func (s *targetBearingSuite) TestTargetBearingDescriptionsAreReaderNeutral() {
	secondPerson := regexp.MustCompile(`(?i)\byou(r|rs|rself)?\b`)
	for _, ref := range TargetBearingRefs() {
		parsed, err := core.ParseString(ref)
		s.Require().NoError(err, ref)
		display, found := DisplayFor(*parsed)
		s.Require().True(found, "%s bears on attacks against its holder but is not registered", ref)
		s.Require().NotEmpty(display.Detail, "%s bears on attacks against its holder but has no description", ref)
		s.False(secondPerson.MatchString(display.Detail), "%s speaks to its holder: %q", ref, display.Detail)
	}
}
