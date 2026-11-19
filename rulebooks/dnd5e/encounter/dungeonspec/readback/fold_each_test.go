// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// FoldEachSuite holds the fold for one thing at a time (rpg-project#980) and
// the loudness said once, in both folds, to the castle.
type FoldEachSuite struct {
	suite.Suite
	rb *readback.ReadBack
}

func TestFoldEachSuite(t *testing.T) { suite.Run(t, new(FoldEachSuite)) }

func (s *FoldEachSuite) SetupSuite() {
	_, s.rb = theCastle(s.T())
}

// folds is both folds, by name.
func (s *FoldEachSuite) folds() map[string][]readback.FoldedSection {
	return map[string][]readback.FoldedSection{"Folded": s.rb.Folded(), "FoldedEach": s.rb.FoldedEach()}
}

// creatureItem is the folded entry of the creature whose heading starts with
// title, in the Creatures section.
func (s *FoldEachSuite) creatureItem(secs []readback.FoldedSection, title string) readback.Item {
	for _, sec := range secs {
		if sec.Title != "Creatures" {
			continue
		}
		for _, it := range sec.Items {
			if strings.HasPrefix(it.Text, title+":") {
				return it
			}
		}
	}
	s.Require().Failf("no creature", "no folded creature %s", title)
	return readback.Item{}
}

// walkItems calls fn on every item under secs, with its parent's text.
func walkItems(secs []readback.FoldedSection, fn func(it readback.Item)) {
	var walk func(it readback.Item)
	walk = func(it readback.Item) {
		fn(it)
		for _, c := range it.Items {
			walk(c)
		}
	}
	for _, sec := range secs {
		for _, it := range sec.Items {
			walk(it)
		}
	}
}

// TestFoldedEach_TheCaptainSaysHisOwnLines: under the one-thing fold the
// captain's entry says his `troupe` line with what it needs, and points at no
// other creature for lines he shares.
func (s *FoldEachSuite) TestFoldedEach_TheCaptainSaysHisOwnLines() {
	captain := s.creatureItem(s.rb.FoldedEach(), "Guard Captain (captain)")
	var troupe *readback.Item
	for i, it := range captain.Items {
		s.NotContains(it.Text, "has the same", "the captain points at nobody")
		if strings.Contains(it.Text, "(`troupe`)") {
			troupe = &captain.Items[i]
		}
	}
	s.Require().NotNil(troupe, "the captain's `troupe` line is said under him")
	s.Require().NotEmpty(troupe.Items)
	s.True(strings.HasPrefix(troupe.Items[0].Text, "Offered while: "), troupe.Items[0].Text)
	s.Contains(troupe.Items[0].Text, "`feast-tonight`")

	// The whole-file fold still shares.
	shared := s.creatureItem(s.rb.Folded(), "Guard Captain (captain)")
	found := false
	for _, it := range shared.Items {
		found = found || strings.Contains(it.Text, "has the same 3 lines as Guard (gate-1)")
	}
	s.True(found, "the whole-file fold keeps its sharing")
}

// reachTells are the words only a loudness's explanation says.
var reachTells = []string{
	"(the wall partition", "a closed door on the way counts as a room", "only the one it is said to, and only from beside",
	"wherever they stand",
}

// TestFolded_NoAnswerRowExplainsItsLoudness: in both folds, an answer names
// its loudness and does not explain it.
func (s *FoldEachSuite) TestFolded_NoAnswerRowExplainsItsLoudness() {
	rowLead := regexp.MustCompile(`^(When said|When done|On a win|On a miss): `)
	for name, secs := range s.folds() {
		named := 0
		walkItems(secs, func(it readback.Item) {
			if !rowLead.MatchString(it.Text) {
				return
			}
			for _, tell := range reachTells {
				s.NotContains(it.Text, tell, "%s: %s", name, it.Text)
			}
			// An answer that teaches says its loudness once: the fact clause
			// rides on the answer's (review of rpg-toolkit#2442).
			s.NotRegexp("at `\\w+` and every member", it.Text, "%s", name)
			s.NotContains(it.Text, "words reach at `", "%s: %s", name, it.Text)
			if strings.Contains(it.Text, ", at `speak`") {
				named++
			}
		})
		s.Positive(named, "%s: a row names its loudness", name)
	}
}

// TestFolded_ALoudnessIsExplainedOncePerCreature: in both folds, each
// creature whose answers speak at a loudness carries that loudness's
// explanation exactly once, and none it never speaks at.
func (s *FoldEachSuite) TestFolded_ALoudnessIsExplainedOncePerCreature() {
	at := regexp.MustCompile(", at `(\\w+)`: its words reach")
	for _, sec := range s.rb.Sections {
		if sec.Title != "Creatures" {
			continue
		}
		for _, g := range sec.Groups {
			want := map[string]bool{}
			for _, st := range g.Statements {
				if st.Kind == readback.KindLineRow {
					for _, m := range at.FindAllStringSubmatch(st.Sentence, -1) {
						want[m[1]] = true
					}
				}
			}
			for name, secs := range s.folds() {
				got := map[string]int{}
				for _, it := range s.creatureItem(secs, g.Title).Items {
					if m := regexp.MustCompile("^At `(\\w+)`, ").FindStringSubmatch(it.Text); m != nil {
						got[m[1]]++
						s.Contains(it.Text, "words reach")
					}
				}
				for l := range want {
					s.Equal(1, got[l], "%s: %s explains `%s` once", name, g.Title, l)
				}
				s.Len(got, len(want), "%s: %s explains only what it speaks at: %v", name, g.Title, got)
			}
		}
	}
	captain := s.creatureItem(s.rb.FoldedEach(), "Guard Captain (captain)")
	var speak readback.Item
	for _, it := range captain.Items {
		if strings.HasPrefix(it.Text, "At `speak`, ") {
			speak = it
		}
	}
	s.Contains(speak.Text, "the guard captain's room (the wall partition the guard captain stands in)")
	s.Contains(speak.Paths, "room.room.monsterBindings.captain.options[0].success[0]",
		"the explanation stands for each row it explains")
}

// reachLoudness names the loudness a full sentence's reach explanation is
// for, read from the explanation's own words (speechwords.go), or "".
func reachLoudness(sentence string) string {
	for _, r := range []struct{ tell, loud string }{
		{"(the wall partition", "speak"},
		{"and the next room over (a closed door on the way counts as a room)", "shout"},
		{"two rooms beyond", "scream"},
		{"only the one it is said to, and only from beside them", "whisper"},
	} {
		if strings.Contains(sentence, r.tell) {
			return r.loud
		}
	}
	return ""
}

// groupItem is the folded entry of a creature (heading "title: …") or a side
// (heading "title") in its section.
func groupItem(secs []readback.FoldedSection, section, title string) (readback.Item, bool) {
	for _, sec := range secs {
		if sec.Title != section {
			continue
		}
		for _, it := range sec.Items {
			if it.Text == title || strings.HasPrefix(it.Text, title+":") {
				return it, true
			}
		}
	}
	return readback.Item{}, false
}

// TestFolded_TheLoudnessEntryStandsForEveryAnswerItExplains: in both folds,
// every answer row and told record whose full sentence explains a loudness —
// a creature's or a side's — is carried by that group's one entry for that
// loudness, and the entry stands above the group's lines.
func (s *FoldEachSuite) TestFolded_TheLoudnessEntryStandsForEveryAnswerItExplains() {
	explained := 0
	for _, sec := range s.rb.Sections {
		if sec.Title != "Creatures" && sec.Title != "Sides" {
			continue
		}
		for _, g := range sec.Groups {
			for name, secs := range s.folds() {
				item, ok := groupItem(secs, sec.Title, g.Title)
				s.Require().True(ok, "%s: no folded %s", name, g.Title)
				entries := map[string]readback.Item{}
				at, firstLine := -1, -1
				for i, it := range item.Items {
					if m := regexp.MustCompile("^At `(\\w+)`, ").FindStringSubmatch(it.Text); m != nil {
						entries[m[1]] = it
						if at < 0 {
							at = i
						}
					}
					if len(it.Items) > 0 && firstLine < 0 {
						firstLine = i
					}
				}
				if len(entries) > 0 && firstLine >= 0 {
					s.Less(at, firstLine, "%s: %s says what its loudness reaches above its lines", name, g.Title)
				}
				for _, st := range g.Statements {
					if st.Kind != readback.KindLineRow && st.Kind != readback.KindLineGrant {
						continue
					}
					l := reachLoudness(st.Sentence)
					if l == "" {
						continue
					}
					explained++
					s.Contains(entries[l].Paths, st.Path, "%s: %s's `%s` entry stands for %s", name, g.Title, l, st.Path)
				}
			}
		}
	}
	s.Positive(explained)
}

// TestFoldedEach_DropsNothing: every statement is stood for by an entry of
// the one-thing fold that carries its path.
func (s *FoldEachSuite) TestFoldedEach_DropsNothing() {
	carried := map[string]bool{}
	walkItems(s.rb.FoldedEach(), func(it readback.Item) {
		for _, p := range it.Paths {
			carried[p] = true
		}
	})
	for _, st := range s.rb.Statements() {
		s.True(carried[st.Path], "statement at %s (%s) has no folded entry", st.Path, st.Kind)
	}
}

// TestFoldedEach_NoRoutinePointsAtAnother: under the one-thing fold every
// routine says its own states and rows.
func (s *FoldEachSuite) TestFoldedEach_NoRoutinePointsAtAnother() {
	walkItems(s.rb.FoldedEach(), func(it readback.Item) {
		s.NotContains(it.Text, "word for word those of")
	})
}
