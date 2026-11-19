// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

// memory_test.go pins the read-back of a machine that remembers
// (rpg-project#1001): its memory site's text, fold and JSON, and that its
// bag is said, not left under not yet said.

import (
	"encoding/json"
	"os"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

const memoryPath = "testdata/memory-site.yaml"

// memory is the memory site's read-back.
func (s *GoldenSuite) memory() *readback.ReadBack {
	raw, err := os.ReadFile(memoryPath)
	s.Require().NoError(err)
	rb, err := readback.Of(raw)
	s.Require().NoError(err)
	return rb
}

func (s *GoldenSuite) TestMemorySiteText() { s.golden("memory-site.readback.txt", s.memory().Text()) }
func (s *GoldenSuite) TestMemorySiteFolded() {
	s.golden("memory-site.folded.txt", s.memory().FoldedText())
}

func (s *GoldenSuite) TestMemorySiteJSON() {
	out, err := json.MarshalIndent(s.memory(), "", "  ")
	s.Require().NoError(err)
	s.golden("memory-site.readback.json", string(out)+"\n")
}

// TestABagIsSaid: the bag is a statement per flag, and nothing of it is
// left under not yet said.
func (s *GoldenSuite) TestABagIsSaid() {
	rb := s.memory()
	s.Empty(rb.NotYetSaid)
	var flags []readback.Statement
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindRoutineFlag {
			flags = append(flags, st)
		}
	}
	s.Require().Len(flags, 1)
	s.Equal("behaviorDefinitions.watch.recipe.bag.warned", flags[0].Path)
	s.Equal("The routine `watch` remembers the flag `warned`, a bool that starts as false.", flags[0].Sentence)
	s.Equal([]readback.Connection{{Kind: readback.ConnFlag, ID: "warned", Role: "declares", Is: "false"}}, flags[0].Connects)
}
