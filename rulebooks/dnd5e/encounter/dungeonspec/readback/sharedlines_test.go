// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// sharedlines_test.go holds the read-back of A CONVERSATION'S LINES WRITTEN
// ONCE (rpg-project#1025): each conversation said once with its carriers,
// each carrier pointing at the conversation it uses.

const conversationsPath = "testdata/conversations-site.yaml"

type SharedLinesSuite struct {
	suite.Suite
	rb *readback.ReadBack
}

func TestSharedLinesSuite(t *testing.T) { suite.Run(t, new(SharedLinesSuite)) }

func (s *SharedLinesSuite) SetupSuite() {
	raw, err := os.ReadFile(conversationsPath)
	s.Require().NoError(err)
	s.rb, err = readback.Of(raw)
	s.Require().NoError(err)
}

func (s *SharedLinesSuite) golden(name, got string) {
	path := "testdata/" + name
	if *update {
		s.Require().NoError(os.WriteFile(path, []byte(got), 0o600))
	}
	want, err := os.ReadFile(path)
	s.Require().NoError(err, "run go test -update to write %s", path)
	s.Equal(string(want), got)
}

func (s *SharedLinesSuite) TestText() { s.golden("conversations-site.readback.txt", s.rb.Text()) }

func (s *SharedLinesSuite) TestFolded() {
	s.golden("conversations-site.folded.txt", s.rb.FoldedText())
}

func (s *SharedLinesSuite) TestJSON() {
	out, err := json.MarshalIndent(s.rb, "", "  ")
	s.Require().NoError(err)
	s.golden("conversations-site.readback.json", string(out)+"\n")
}

// statement is the one statement of a kind at a path.
func (s *SharedLinesSuite) statement(kind readback.Kind, path string) readback.Statement {
	var found []readback.Statement
	for _, st := range s.rb.Statements() {
		if st.Kind == kind && st.Path == path {
			found = append(found, st)
		}
	}
	s.Require().Len(found, 1, "one %s at %s", kind, path)
	return found[0]
}

// TestTheConversationNamesItsCarriers: the conversation is said once, at
// its own path, connected to each creature that carries it.
func (s *SharedLinesSuite) TestTheConversationNamesItsCarriers() {
	head := s.statement(readback.KindSharedLines, "conversations.gate-claims")
	s.Equal(readback.Subject{Kind: readback.ConnSharedLines, ID: "gate-claims"}, head.About)
	for _, id := range []string{"gate-1", "gate-2", "captain"} {
		s.Contains(head.Connects, readback.Connection{Kind: "creature", ID: id, Role: "carries"})
	}
	s.Contains(head.Sentence, "It is carried by 3 carriers")
	s.Contains(head.Sentence, "`troupe`, `come-quietly` and `never-alive`")
	line := s.statement(readback.KindLine, "conversations.gate-claims.lines[0]")
	s.Equal(readback.Subject{Kind: readback.ConnSharedLines, ID: "gate-claims"}, line.About)
}

// TestACarrierSaysWhichConversationItsLinesCameFrom: each carrier's use is
// one statement at its `use:` entry; its own lines stay at their own paths,
// and a conversation's line is never said again under a carrier.
func (s *SharedLinesSuite) TestACarrierSaysWhichConversationItsLinesCameFrom() {
	for _, at := range []string{
		"room.room.monsterBindings.gate-1.options[0]",
		"room.room.monsterBindings.gate-2.options[0]",
		"room.room.monsterBindings.captain.options[1]",
	} {
		use := s.statement(readback.KindUsesLines, at)
		s.Contains(use.Connects, readback.Connection{Kind: readback.ConnSharedLines, ID: "gate-claims", Role: "speaks-lines-of"})
		s.Contains(use.Sentence, "carries the conversation `gate-claims`")
	}
	guest := s.statement(readback.KindLine, "room.room.monsterBindings.captain.options[0]")
	s.Equal(readback.Subject{Kind: "creature", ID: "captain"}, guest.About)
	for _, st := range s.rb.Statements() {
		if st.Kind == readback.KindLine && st.About.Kind == "creature" {
			s.Equal("room.room.monsterBindings.captain.options[0]", st.Path,
				"the captain's guest is the only line said under a creature")
		}
	}
}

// TestTheGatesAskedLinesPointAtTheConversation: the interaction's
// statements about a line read through a conversation name the line's own
// path, once per carrier.
func (s *SharedLinesSuite) TestTheGatesAskedLinesPointAtTheConversation() {
	var who []string
	for _, st := range s.rb.Statements() {
		if st.Kind == readback.KindConvAsked && st.Path == "conversations.gate-claims.lines[0].requires[1]" {
			for _, c := range st.Connects {
				who = append(who, c.ID)
			}
		}
	}
	sort.Strings(who)
	s.Equal([]string{"captain", "gate-1", "gate-2"}, who)
}

// passages is every passage and passage-way sentence of a read-back.
func passages(rb *readback.ReadBack) []string {
	var out []string
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindJoinPassage || st.Kind == readback.KindWayPassage {
			out = append(out, st.Sentence)
		}
	}
	sort.Strings(out)
	return out
}

// TestAPassageThroughAConversationIsSaidAsItsCopy: the castle's grate with
// its two ways written as a conversation says the same passages as the
// grate with them written on it (the shared castle read-back).
func TestAPassageThroughAConversationIsSaidAsItsCopy(t *testing.T) {
	raw, copied := theCastle(t)
	src := string(raw)
	start := strings.Index(src, "      grate:\n        options:\n")
	end := strings.Index(src, "      # Hand edit (rpg-project#1023): the cell's weak place.")
	require.True(t, start > 0 && end > start, "the castle still gives the grate its two ways")
	block := src[start+len("      grate:\n        options:\n") : end]
	shared := src[:start] + "      grate:\n        options:\n          - use: tunnels\n" + src[end:]
	lines := strings.ReplaceAll(block, "\n          ", "\n      ")
	lines = strings.Replace(lines, "          - id", "      - id", 1)
	shared = strings.Replace(shared, "\nroom:\n  version: 3", "\nconversations:\n  tunnels:\n    lines:\n"+lines+"room:\n  version: 3", 1)
	rb, err := readback.Of([]byte(shared))
	require.NoError(t, err)
	want := passages(copied)
	require.NotEmpty(t, want)
	require.Equal(t, want, passages(rb))
}
