// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

import (
	"encoding/json"
	"flag"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

var update = flag.Bool("update", false, "rewrite the golden read-backs")

const hailPath = "../testdata/hail-site.yaml"

// GoldenSuite pins the read-back of one small site, text and JSON, so a
// change of wording is a visible diff in review.
type GoldenSuite struct{ suite.Suite }

func TestGoldenSuite(t *testing.T) { suite.Run(t, new(GoldenSuite)) }

func (s *GoldenSuite) golden(name, got string) {
	path := "testdata/" + name
	if *update {
		s.Require().NoError(os.WriteFile(path, []byte(got), 0o600))
	}
	want, err := os.ReadFile(path)
	s.Require().NoError(err, "run go test -update to write %s", path)
	s.Equal(string(want), got)
}

func (s *GoldenSuite) TestHailSiteText() {
	raw, err := os.ReadFile(hailPath)
	s.Require().NoError(err)
	rb, err := readback.Of(raw)
	s.Require().NoError(err)
	s.golden("hail-site.readback.txt", rb.Text())
}

func (s *GoldenSuite) TestHailSiteFolded() {
	raw, err := os.ReadFile(hailPath)
	s.Require().NoError(err)
	rb, err := readback.Of(raw)
	s.Require().NoError(err)
	s.golden("hail-site.folded.txt", rb.FoldedText())
}

func (s *GoldenSuite) TestHailSiteJSON() {
	raw, err := os.ReadFile(hailPath)
	s.Require().NoError(err)
	rb, err := readback.Of(raw)
	s.Require().NoError(err)
	out, err := json.MarshalIndent(rb, "", "  ")
	s.Require().NoError(err)
	s.golden("hail-site.readback.json", string(out)+"\n")
}

// TestAKeyNobodySaysIsListed: a binding key the read-back does not say
// (a creature's own arms) is listed under not yet said at its path, not
// dropped.
func (s *GoldenSuite) TestAKeyNobodySaysIsListed() {
	raw, err := os.ReadFile(hailPath)
	s.Require().NoError(err)
	edited := strings.Replace(string(raw), "        faction: watch\n",
		"        faction: watch\n        actions:\n          - dnd5e:weapons:club\n", 1)
	s.Require().NotEqual(string(raw), edited)
	rb, err := readback.Of([]byte(edited))
	s.Require().NoError(err)
	s.Contains(rb.NotYetSaid, readback.Unsaid{
		Path: "room.room.monsterBindings.porter.actions", Kind: "monsterBinding.actions",
		Why: "the read-back cannot say this yet",
	})
}

// TestARefusedFileIsTheCompilersSentence: a file the compiler refuses comes
// back as the compiler's own error, at its path.
func (s *GoldenSuite) TestARefusedFileIsTheCompilersSentence() {
	raw, err := os.ReadFile(hailPath)
	s.Require().NoError(err)
	edited := strings.Replace(string(raw), "in: [fresh]", "in: [frsh]", 1)
	_, err = readback.Of([]byte(edited))
	var refused *dungeonspec.ValidationError
	s.Require().ErrorAs(err, &refused)
	s.Contains(err.Error(), "room.room.monsterBindings.porter.options[0].requires")
}

// TestAnOldDialectIsNotASite: the read-back reads site documents only.
func (s *GoldenSuite) TestAnOldDialectIsNotASite() {
	_, err := readback.Of([]byte("version: 2\n"))
	s.ErrorIs(err, readback.ErrNotASite)
}
