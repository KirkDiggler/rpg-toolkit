// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// ThreadsSuite holds the record and secret hubs (rpg-project#973): a secret
// passage reads as one chain by connections alone.
type ThreadsSuite struct {
	suite.Suite
	rb *readback.ReadBack
}

func TestThreadsSuite(t *testing.T) { suite.Run(t, new(ThreadsSuite)) }

func (s *ThreadsSuite) SetupSuite() {
	_, s.rb = theCastle(s.T())
}

// hub is the group about {kind, id} in the section titled.
func hub(rb *readback.ReadBack, section, kind, id string) (readback.Group, bool) {
	for _, sec := range rb.Sections {
		if sec.Title != section {
			continue
		}
		for _, g := range sec.Groups {
			if g.About.Kind == kind && g.About.ID == id {
				return g, true
			}
		}
	}
	return readback.Group{}, false
}

// kindsAt is each statement of a group as "kind path".
func kindsAt(g readback.Group) []string {
	var out []string
	for _, st := range g.Statements {
		out = append(out, string(st.Kind)+" "+st.Path)
	}
	return out
}

// TestThreads_Castle_TheKitchenWayRecord: told by the scullion's line,
// revealing the kitchen passage, lifting the two passages' hidden bars.
func (s *ThreadsSuite) TestThreads_Castle_TheKitchenWayRecord() {
	g, ok := hub(s.rb, "Records", "record", "kitchen-way")
	s.Require().True(ok)
	s.Equal([]string{
		"record.reveal intel[4]",
		"record.tell room.room.monsterBindings.scullion.options[0].success[0].tells",
		"record.lift intel[4]",
		"record.lift intel[4]",
	}, kindsAt(g))
	s.True(connected(g.Statements[0], "concealment", "kitchen-passage", "reveals"))
	s.True(connected(g.Statements[1], "record", "kitchen-way", "tells"))
}

// TestThreads_Castle_TheKitchenPassageSecret: revealed by the record, found
// by its search, hiding the grate, barring the two passages, needed by the
// two ways that use the grate.
func (s *ThreadsSuite) TestThreads_Castle_TheKitchenPassageSecret() {
	g, ok := hub(s.rb, "Secrets", "concealment", "kitchen-passage")
	s.Require().True(ok)
	got := map[readback.Kind]int{}
	for _, st := range g.Statements {
		got[st.Kind]++
	}
	s.Equal(map[readback.Kind]int{
		readback.KindSecretHide: 1, readback.KindSecretReveal: 1, readback.KindSecretFind: 1,
		readback.KindSecretBar: 2, readback.KindSecretNeed: 2,
	}, got)
	for _, st := range g.Statements {
		switch st.Kind {
		case readback.KindSecretReveal:
			s.Equal("intel[4]", st.Path, "only the record reveals it")
		case readback.KindSecretFind:
			s.Equal("concealments.kitchen-passage.checks", st.Path)
			s.Contains(st.Sentence, "Investigation against DC 18")
			s.Equal([]readback.Connection{{Kind: "concealment", ID: "kitchen-passage", Role: "finds"}}, st.Connects)
		}
	}
	s.Equal(readback.KindSecretHide, g.Statements[0].Kind)
	s.True(connected(g.Statements[0], "prop", "grate", "hides"))
	var bars []string
	for _, st := range g.Statements {
		if st.Kind == readback.KindSecretBar {
			bars = append(bars, st.About.ID)
		}
	}
	s.Equal([]string{"kitchen-passage", "kitchen-passage"}, bars, "a hub's statements are about the hub")
}

// TestThreads_Castle_TheChainFollowsByConnections: from the scullion's line
// to the grate and the passages, each step a connection, no sentence read.
func (s *ThreadsSuite) TestThreads_Castle_TheChainFollowsByConnections() {
	var told, revealed, hidden, barred bool
	for _, st := range s.rb.Statements() {
		switch {
		case st.Kind == readback.KindLineGrant && connected(st, "record", "kitchen-way", "tells"):
			told = true
		case st.Kind == readback.KindIntel && st.About.ID == "kitchen-way" && connected(st, "concealment", "kitchen-passage", "reveals"):
			revealed = true
		case st.Kind == readback.KindConcealment && connected(st, "prop", "grate", "hides"):
			hidden = true
		case st.Kind == readback.KindJoinPassage && connected(st, "concealment", "kitchen-passage", "hidden-by"):
			barred = true
		}
	}
	s.True(told && revealed && hidden && barred, "told %v revealed %v hidden %v barred %v", told, revealed, hidden, barred)
}

// TestThreads_Castle_TheCaptainsOrdersLinkToTheirFact: the record's hub
// links to the fact it reveals, and the fact's hub lists the record.
func (s *ThreadsSuite) TestThreads_Castle_TheCaptainsOrdersLinkToTheirFact() {
	g, ok := hub(s.rb, "Records", "record", "captains-orders")
	s.Require().True(ok)
	s.True(connected(g.Statements[0], "fact", "party-recognized", "teaches"))
	held := false
	for _, st := range g.Statements {
		held = held || st.Kind == readback.KindRecordHold
	}
	s.True(held, "the orders are held by a prop")
	f, ok := hub(s.rb, "Facts", "fact", "party-recognized")
	s.Require().True(ok)
	back := false
	for _, st := range f.Statements {
		back = back || (st.Kind == readback.KindFactTeach && st.Path == g.Statements[0].Path)
	}
	s.True(back, "the fact's hub lists the record that teaches it")
}

// TestThreads_BothFoldsCarryTheHubs: the record and secret hubs fold in
// both forms, every path carried, with their headings.
func (s *ThreadsSuite) TestThreads_BothFoldsCarryTheHubs() {
	for _, secs := range [][]readback.FoldedSection{s.rb.Folded(), s.rb.FoldedEach()} {
		titles := map[string]bool{}
		for _, sec := range secs {
			titles[sec.Title] = true
		}
		s.True(titles["Records"] && titles["Secrets"])
		carried := map[string]bool{}
		walkItems(secs, func(it readback.Item) {
			for _, p := range it.Paths {
				carried[p] = true
			}
		})
		for _, st := range s.rb.Statements() {
			s.True(carried[st.Path], "%s at %s", st.Kind, st.Path)
		}
		var text []string
		walkItems(secs, func(it readback.Item) { text = append(text, it.Text) })
		all := strings.Join(text, "\n")
		s.Contains(all, "Told by: ")
		s.Contains(all, "Bars: ")
	}
}

// TestThreads_IDsStable: a second read names the same hubs.
func (s *ThreadsSuite) TestThreads_IDsStable() {
	raw, err := os.ReadFile(castlePath)
	s.Require().NoError(err)
	again, err := readback.Of(raw)
	s.Require().NoError(err)
	for _, title := range []string{"Records", "Secrets"} {
		var a, b []readback.Subject
		for _, sec := range s.rb.Sections {
			if sec.Title == title {
				for _, g := range sec.Groups {
					a = append(a, g.About)
				}
			}
		}
		for _, sec := range again.Sections {
			if sec.Title == title {
				for _, g := range sec.Groups {
					b = append(b, g.About)
				}
			}
		}
		s.NotEmpty(a)
		s.Equal(a, b)
	}
}

// TestThreads_Site_LonelyHubsStillHaveAHub: a record nothing tells or
// holds, and a secret nothing reveals, each still get a hub of their own.
func (s *ThreadsSuite) TestThreads_Site_LonelyHubsStillHaveAHub() {
	raw, err := os.ReadFile(joinsPath)
	s.Require().NoError(err)
	edited := strings.Replace(string(raw), "room:\n  version: 3\n",
		"intel:\n  - id: lonely\n    reveals:\n      fact: nobody-knows\nconcealments:\n  nook:\n    checks: [{ ability: investigation, dc: 12 }]\n"+
			"    cells: [{ r: 0, q: 5 }]\nroom:\n  version: 3\n", 1)
	s.Require().NotEqual(string(raw), edited)
	rb, err := readback.Of([]byte(edited))
	s.Require().NoError(err)
	r, ok := hub(rb, "Records", "record", "lonely")
	s.Require().True(ok)
	s.Equal([]string{"record.reveal intel[0]"}, kindsAt(r))
	c, ok := hub(rb, "Secrets", "concealment", "nook")
	s.Require().True(ok)
	s.Equal([]string{"secret.hide concealments.nook", "secret.find concealments.nook.checks"}, kindsAt(c))
	s.Contains(c.Statements[0].Sentence, "1 floor cell")
}

// TestThreads_Castle_AHubRepeatIsNeverAWay: a way repeated under a record or
// secret hub is a pointer to the way, carrying only {way, id, is}; so the
// statements that go through a join are exactly the ways, and no statement
// but a way carries a method.
func (s *ThreadsSuite) TestThreads_Castle_AHubRepeatIsNeverAWay() {
	through, ways := 0, 0
	for _, st := range s.rb.Statements() {
		isWay := strings.HasPrefix(string(st.Kind), "place.way.")
		if isWay {
			ways++
		}
		for _, c := range st.Connects {
			if c.Kind == "join" && c.Role == "through" {
				through++
			}
			if c.Kind == "method" {
				s.True(isWay, "%s at %s carries a method", st.Kind, st.Path)
			}
		}
	}
	s.Equal(ways, through)
	g, ok := hub(s.rb, "Records", "record", "kitchen-way")
	s.Require().True(ok)
	for _, st := range g.Statements {
		if st.Kind == readback.KindRecordLift {
			s.Require().Len(st.Connects, 1)
			s.Equal("way", st.Connects[0].Kind)
			s.Equal("is", st.Connects[0].Role)
			s.True(strings.HasPrefix(st.Connects[0].ID, "way:passage:region-kitchen:"), st.Connects[0].ID)
		}
	}
}
