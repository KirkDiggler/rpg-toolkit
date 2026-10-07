// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"maps"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/contributions"
)

// targetCensusSuite holds the second classification: every loader, once, for
// what a target can hold.
type targetCensusSuite struct{ suite.Suite }

func TestTargetCensusSuite(t *testing.T) { suite.Run(t, new(targetCensusSuite)) }

func (s *targetCensusSuite) TestEveryConditionLoaderIsClassifiedForTargetHeld() {
	loaders := slices.Sorted(maps.Keys(conditionLoaders))
	census := slices.Sorted(maps.Keys(targetCensus))
	s.Equal(loaders, census, "the target census and the loaders name exactly the same refs")

	for ref, entry := range targetCensus {
		switch entry.class {
		case censusAnswers, censusNotYetAnswering:
			s.Equal(contributions.ContributesNow, entry.participation, ref)
		case censusNotBearing:
			s.Empty(entry.participation, ref)
		default:
			s.Failf("unknown census class", "%s: %q", ref, entry.class)
		}
	}
}

func (s *targetCensusSuite) TestTargetAnsweringRefsHaveHeldRules() {
	var answering []string
	for ref, entry := range targetCensus {
		if entry.class == censusAnswers {
			answering = append(answering, ref)
		}
	}
	s.ElementsMatch(answering, slices.Collect(maps.Keys(targetHeldRules)),
		"one held rule per answering ref, and no other")
}

func (s *targetCensusSuite) TestTargetBearingLoadersHaveDescriptions() {
	for _, ref := range TargetBearingRefs() {
		parsed, err := core.ParseString(ref)
		s.Require().NoError(err, ref)
		display, found := DisplayFor(*parsed)
		s.True(found, "%s bears on attacks against its holder but has no catalog entry", ref)
		s.NotEmpty(display.Detail, "%s bears on attacks against its holder but has no description", ref)
	}
}

// TestTargetBearingDescriptionsAreReaderNeutral: a held row's description is
// shown on an attacker's target candidate, so it never speaks to the holder as
// "you" — the attacker reading it is not the one prone, hidden or lit.
func (s *targetCensusSuite) TestTargetBearingDescriptionsAreReaderNeutral() {
	secondPerson := regexp.MustCompile(`(?i)\byou(r|rs|rself)?\b`)
	for _, ref := range TargetBearingRefs() {
		parsed, err := core.ParseString(ref)
		s.Require().NoError(err, ref)
		display, found := DisplayFor(*parsed)
		s.Require().True(found, "%s bears on attacks against its holder but has no catalog entry", ref)
		s.Require().NotEmpty(display.Detail, "%s bears on attacks against its holder but has no description", ref)
		s.False(secondPerson.MatchString(display.Detail), "%s speaks to its holder: %q", ref, display.Detail)
	}
}
