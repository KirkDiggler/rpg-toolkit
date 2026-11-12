// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

const hail = "../../dungeonspec/testdata/hail-site.yaml"

type ReadBackSuite struct{ suite.Suite }

func TestReadBackSuite(t *testing.T) { suite.Run(t, new(ReadBackSuite)) }

func (s *ReadBackSuite) TestFoldedByDefault() {
	var out, errOut bytes.Buffer
	s.Equal(0, run([]string{hail}, &out, &errOut))
	s.Contains(out.String(), "READ-BACK: Hail Site (hail-site)")
	s.Contains(out.String(), "“We're expected.” (`guest`), said to the guard")
	s.NotContains(out.String(), "        at room.room")
	s.Contains(out.String(), "NOT YET SAID")
}

func (s *ReadBackSuite) TestFull() {
	var out, errOut bytes.Buffer
	s.Equal(0, run([]string{"--full", hail}, &out, &errOut))
	s.Contains(out.String(), "        at room.room.monsterBindings.porter.options[0].advance\n")
}

func (s *ReadBackSuite) TestJSON() {
	var out, errOut bytes.Buffer
	s.Equal(0, run([]string{"--json", hail}, &out, &errOut))
	var rb readback.ReadBack
	s.Require().NoError(json.Unmarshal(out.Bytes(), &rb))
	s.Equal("hail-site", rb.Key)
	s.NotEmpty(rb.Statements())
}

func (s *ReadBackSuite) TestRefusedAndUsage() {
	raw, err := os.ReadFile(hail)
	s.Require().NoError(err)
	path := filepath.Join(s.T().TempDir(), "bad.yaml")
	s.Require().NoError(os.WriteFile(path, []byte(strings.Replace(string(raw), "in: [fresh]", "in: [frsh]", 1)), 0o600))
	var out, errOut bytes.Buffer
	s.Equal(1, run([]string{path}, &out, &errOut))
	s.True(strings.HasPrefix(out.String(), "REFUSED: "), out.String())
	out.Reset()
	s.Equal(2, run(nil, &out, &errOut))
	s.Equal(2, run([]string{"--nope", hail}, &out, &errOut))
}
