// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package conditions

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

type rulesLibrarySuite struct{ suite.Suite }

func TestRulesLibrarySuite(t *testing.T) { suite.Run(t, new(rulesLibrarySuite)) }

// Catches a rules library that does not compile, or that lacks the Dexterity
// rule. Other rules may join it; this pins only that Dexterity is there.
func (s *rulesLibrarySuite) TestTheLibraryHoldsTheDexterityRule() {
	rules, err := rulesLibrary()
	s.Require().NoError(err)
	sources := make([]string, 0, len(rules))
	for _, r := range rules {
		sources = append(sources, r.source)
	}
	s.Contains(sources, "Dexterity")
}

// Catches a rule written with something the vocabulary would refuse being
// compiled anyway: an unknown ability, an unknown member, a blank source, a
// repeated key.
func (s *rulesLibrarySuite) TestABadRuleIsRefusedAtCompile() {
	for name, raw := range map[string]string{
		"unknown ability": strings.Replace(rulesData, `"ability":"dex"`, `"ability":"luck"`, 1),
		"unknown member":  strings.Replace(rulesData, `"ability":"dex"`, `"ability":"dex","times":2`, 1),
		"blank source":    strings.Replace(rulesData, `"source":"Dexterity"`, `"source":" "`, 1),
		"repeated key":    strings.Replace(rulesData, "\n]", ",\n"+strings.TrimSpace(strings.Trim(strings.TrimSpace(rulesData), "[]"))+"\n]", 1),
	} {
		s.Run(name, func() {
			s.Require().NotEqual(rulesData, raw)
			_, err := compileRules(json.RawMessage(raw))
			s.Error(err)
		})
	}
}
