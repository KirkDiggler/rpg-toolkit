// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

const castle = "../../dungeonspec/testdata/world-builder-v4-castle.yaml"

type ContentCheckSuite struct{ suite.Suite }

func TestContentCheckSuite(t *testing.T) { suite.Run(t, new(ContentCheckSuite)) }

func (s *ContentCheckSuite) TestTheCastleCompiles() {
	var out, errOut bytes.Buffer
	s.Equal(0, run([]string{castle}, &out, &errOut))
	s.Equal("OK: the file compiles\n", out.String())
	s.Empty(errOut.String())
}

// TestAMisspeltTopicIsRefusedInTheCompilersWords: the gate guard's pass names
// `admited`, a topic nothing grants; the checker prints the compiler's own
// sentence at the path it refused, and exits 1.
func (s *ContentCheckSuite) TestAMisspeltTopicIsRefusedInTheCompilersWords() {
	raw, err := os.ReadFile(castle)
	s.Require().NoError(err)
	bad := strings.Replace(string(raw), "pass: { recognized: admitted }", "pass: { recognized: admited }", 1)
	s.Require().NotEqual(string(raw), bad)
	path := filepath.Join(s.T().TempDir(), "bad.yaml")
	s.Require().NoError(os.WriteFile(path, []byte(bad), 0o600))

	var out, errOut bytes.Buffer
	s.Equal(1, run([]string{path}, &out, &errOut))
	s.True(strings.HasPrefix(out.String(), "REFUSED: "), out.String())
	s.Contains(out.String(), "room.room.monsterBindings.gate-1.posts.alert.pass.recognized: "+
		"\"admited\" is a topic no interaction's `outcomes` grants, so no holder could ever recognize anyone on it")
}

func (s *ContentCheckSuite) TestUsageAndUnreadable() {
	var out, errOut bytes.Buffer
	s.Equal(2, run(nil, &out, &errOut))
	s.Contains(errOut.String(), "usage: contentcheck <file>")
	errOut.Reset()
	s.Equal(2, run([]string{filepath.Join(s.T().TempDir(), "missing.yaml")}, &out, &errOut))
	s.Contains(errOut.String(), "contentcheck:")
	s.Empty(out.String())
}
