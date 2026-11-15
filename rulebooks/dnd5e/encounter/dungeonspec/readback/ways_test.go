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

// WaysCastleSuite holds the ways through the castle's joins (rpg-project#981)
// to the file: read off the site, each countable by its connections alone.
type WaysCastleSuite struct {
	suite.Suite
	rb *readback.ReadBack
}

func TestWaysCastleSuite(t *testing.T) { suite.Run(t, new(WaysCastleSuite)) }

func (s *WaysCastleSuite) SetupSuite() {
	raw, err := os.ReadFile(castlePath)
	s.Require().NoError(err)
	s.rb, err = readback.Of(raw)
	s.Require().NoError(err)
}

// waysThrough is every way through the join with id.
func waysThrough(rb *readback.ReadBack, join string) []readback.Statement {
	var out []readback.Statement
	for _, st := range rb.Statements() {
		if st.About.Kind == "way" && connected(st, "join", join, "through") {
			out = append(out, st)
		}
	}
	return out
}

// methods is a way's method ids.
func methods(st readback.Statement) []string {
	var out []string
	for _, c := range st.Connects {
		if c.Kind == "method" {
			out = append(out, c.ID)
		}
	}
	return out
}

// lineWay is the line way through a join carried by the line on the creature.
func (s *WaysCastleSuite) lineWay(join, creature, line string) readback.Statement {
	for _, w := range waysThrough(s.rb, join) {
		if w.Kind == readback.KindWayLine && connected(w, "line", line, "by") && connected(w, "creature", creature, "by") {
			return w
		}
	}
	s.Require().Failf("no way", "no line way %s of %s through %s", line, creature, join)
	return readback.Statement{}
}

const gateJoin = "door:region-courtyard:region-gatehouse:gate-wall-door"

// TestWays_Castle_TheTroupeClaim: Performance 13 or Deception 15, needing
// `feast-tonight`; a miss steps the gate's conversation.
func (s *WaysCastleSuite) TestWays_Castle_TheTroupeClaim() {
	for _, who := range []string{"gate-1", "gate-2", "captain"} {
		w := s.lineWay(gateJoin, who, "troupe")
		s.Equal([]string{"skill:performance", "skill:deception"}, methods(w))
		s.True(connected(w, "fact", "feast-tonight", "needs"), w.Sentence)
		s.True(connected(w, "topic", "admitted", "grants"), w.Sentence)
		s.True(connected(w, "conversation", "front-gate", "on-miss"), w.Sentence)
		s.Contains(w.Sentence, "Performance against DC 13, or Deception against DC 15")
		s.Contains(w.Sentence, "On a miss: the line steps `front-gate` with `troupe-failed`")
	}
}

// TestWays_Castle_TheGuestClaimIsTheCaptains: Persuasion 11, the captain's alone.
func (s *WaysCastleSuite) TestWays_Castle_TheGuestClaimIsTheCaptains() {
	w := s.lineWay(gateJoin, "captain", "guest")
	s.Equal([]string{"skill:persuasion"}, methods(w))
	for _, other := range waysThrough(s.rb, gateJoin) {
		if connected(other, "line", "guest", "by") {
			s.True(connected(other, "creature", "captain", "by"), "only the captain carries `guest`")
		}
	}
}

// TestWays_Castle_VouchNeedsSomeoneAdmitted: no roll; the need is the guard
// recognising the SPEAKER as `admitted` (a topic) once the gate's
// conversation is in `admitted`.
func (s *WaysCastleSuite) TestWays_Castle_VouchNeedsSomeoneAdmitted() {
	w := s.lineWay(gateJoin, "gate-1", "vouch")
	s.Equal([]string{"free"}, methods(w))
	s.True(connected(w, "topic", "admitted", "needs"), w.Sentence)
	s.True(connected(w, "conversation", "front-gate", "needs"), w.Sentence)
	s.Contains(w.Sentence, "the guard recognises the speaker as `admitted`")
	s.NotContains(w.Sentence, "{self}")
}

// TestWays_Castle_TheStraggler: their own case, once; a miss turns them away.
func (s *WaysCastleSuite) TestWays_Castle_TheStraggler() {
	w := s.lineWay(gateJoin, "gate-2", "straggler")
	s.Equal([]string{"skill:persuasion", "skill:deception"}, methods(w))
	s.True(connected(w, "topic", "turned-away", "on-miss"), w.Sentence)
	s.False(connected(w, "topic", "turned-away", "needs"), "a NOT requirement is no need")
}

// TestWays_Castle_StandAsideDrawsEveryHolderOff: the captain's stand-aside
// teaches `gate-cowed`, and each of the three holders' routines leaves its
// post on that fact, so it is a way through, said with its reach and weight.
func (s *WaysCastleSuite) TestWays_Castle_StandAsideDrawsEveryHolderOff() {
	w := s.lineWay(gateJoin, "captain", "stand-aside")
	s.Equal([]string{"skill:intimidation"}, methods(w))
	for _, h := range []string{"gate-1", "gate-2", "captain"} {
		s.True(connected(w, "creature", h, "draws-off"), "%s leaves its post on `gate-cowed`", h)
	}
	s.Contains(w.Sentence, "on the row of weight 3 of 4, teaches `gate-cowed` to whoever its words reach")
	s.Contains(w.Sentence, "a holder the words do not reach keeps holding")
}

// TestWays_Castle_TheGateFight: one fight way, by the three holders.
func (s *WaysCastleSuite) TestWays_Castle_TheGateFight() {
	var fights []readback.Statement
	for _, w := range waysThrough(s.rb, gateJoin) {
		if w.Kind == readback.KindWayFight {
			fights = append(fights, w)
		}
	}
	s.Require().Len(fights, 1)
	s.Equal([]string{"fight"}, methods(fights[0]))
	for _, h := range []string{"gate-1", "gate-2", "captain"} {
		s.True(connected(fights[0], "creature", h, "by"))
	}
}

// TestWays_Castle_TheArrestSaysItsTrigger: the arrest's passage is carried
// by the conversation, entered from a warned state on a second claim's miss.
func (s *WaysCastleSuite) TestWays_Castle_TheArrestSaysItsTrigger() {
	ways := waysThrough(s.rb, "passage:region-gatehouse+room-1-region:region-cells:conversation-front-gate")
	s.Require().Len(ways, 1)
	w := ways[0]
	s.Equal(readback.KindWayPassage, w.Kind)
	s.True(connected(w, "conversation", "front-gate", "by"))
	s.Contains(w.Sentence, "from `warned-guest` on `troupe-failed`")
	s.Contains(w.Sentence, "from `warned-troupe` on `guest-failed`")
	s.True(connected(w, "line", "guest", "steps"))
	s.True(connected(w, "line", "troupe", "steps"))
}

// TestWays_Castle_TheLockedDoors: the kitchen door's two approaches and the
// cells door's one, each its own way.
func (s *WaysCastleSuite) TestWays_Castle_TheLockedDoors() {
	got := map[string][]string{}
	for _, join := range []string{"door:region-great-hall:region-kitchen:kitchen-wall-door", "door:region-barracks:region-cells:cells-wall-door"} {
		for _, w := range waysThrough(s.rb, join) {
			s.Equal(readback.KindWayLock, w.Kind, w.Sentence)
			got[join] = append(got[join], strings.Join(methods(w), "+"))
		}
	}
	s.Equal([]string{"ability:str", "ability:dex+tool:dnd5e:item:thieves-tools"}, got["door:region-great-hall:region-kitchen:kitchen-wall-door"])
	s.Equal([]string{"ability:dex+tool:dnd5e:item:thieves-tools"}, got["door:region-barracks:region-cells:cells-wall-door"])
}

// TestWays_Castle_TheGratesWaysAreHiddenUntilKnown: each of the grate's two
// passages is found by a search, revealed by the record `kitchen-way`, and
// used by its free line once the secret is known.
func (s *WaysCastleSuite) TestWays_Castle_TheGratesWaysAreHiddenUntilKnown() {
	for _, to := range []string{"region-chapel", "region-cells"} {
		kinds := map[readback.Kind]readback.Statement{}
		for _, w := range waysThrough(s.rb, "passage:region-kitchen:"+to+":prop-grate") {
			kinds[w.Kind] = w
		}
		s.Require().Len(kinds, 3, to)
		s.Equal([]string{"skill:investigation"}, methods(kinds[readback.KindWaySearch]))
		s.True(connected(kinds[readback.KindWaySearch], "concealment", "kitchen-passage", "by"))
		s.Equal([]string{"knowledge"}, methods(kinds[readback.KindWayTold]))
		s.True(connected(kinds[readback.KindWayTold], "record", "kitchen-way", "by"))
		p := kinds[readback.KindWayPassage]
		s.Equal([]string{"free"}, methods(p))
		s.True(connected(p, "concealment", "kitchen-passage", "needs"))
		s.True(connected(p, "prop", "grate", "by"))
	}
}

// TestWays_Castle_EveryJoinHasAWay: every join has at least one way, the
// free doors and the open floors a free one, and no way is none.
func (s *WaysCastleSuite) TestWays_Castle_EveryJoinHasAWay() {
	for _, j := range joinsOf(s.rb) {
		ways := waysThrough(s.rb, j.About.ID)
		s.NotEmpty(ways, "%s has a way", j.About.ID)
		for _, w := range ways {
			s.NotEqual(readback.KindWayNone, w.Kind)
			s.NotEmpty(methods(w), "%s: its method is countable", w.About.ID)
		}
	}
	for _, j := range []string{"open:region-armoury:region-courtyard", "door:region-chapel:region-great-hall:chapel-wall-door"} {
		ways := waysThrough(s.rb, j)
		s.Require().Len(ways, 1)
		s.Equal(readback.KindWayFree, ways[0].Kind)
	}
}

// TestWays_Castle_StableAndUniqueIDs: a way's id is its join's and its
// carrier's, the same on a second read, never shared.
func (s *WaysCastleSuite) TestWays_Castle_StableAndUniqueIDs() {
	raw, err := os.ReadFile(castlePath)
	s.Require().NoError(err)
	again, err := readback.Of(raw)
	s.Require().NoError(err)
	ids := func(rb *readback.ReadBack) []string {
		var out []string
		for _, st := range rb.Statements() {
			if st.About.Kind == "way" {
				out = append(out, st.About.ID)
			}
		}
		return out
	}
	first := ids(s.rb)
	s.Equal(first, ids(again))
	seen := map[string]bool{}
	for _, id := range first {
		s.False(seen[id], "way id %s said twice", id)
		seen[id] = true
	}
	s.Contains(first, "way:"+gateJoin+":line:captain:troupe")
}

// TestWays_Castle_HubsDoNotDoubleUp: a way names the carrier's needs, which
// the carrier's own statements already put in the hubs; a way is never a hub
// entry.
func (s *WaysCastleSuite) TestWays_Castle_HubsDoNotDoubleUp() {
	for _, sec := range s.rb.Sections {
		if sec.Title != "Topics" && sec.Title != "Facts" {
			continue
		}
		for _, g := range sec.Groups {
			for _, st := range g.Statements {
				s.NotEqual("way", st.About.Kind, "%s lists a way", g.Title)
			}
		}
	}
}

// TestWays_FoldedUnderTheirJoin: in both folds each way is an entry under
// its join, and every statement is still carried.
func (s *WaysCastleSuite) TestWays_FoldedUnderTheirJoin() {
	for _, secs := range [][]readback.FoldedSection{s.rb.Folded(), s.rb.FoldedEach()} {
		found := false
		walkItems(secs, func(it readback.Item) {
			if strings.Contains(it.Text, "join at the door `gate-wall-door`") {
				found = true
				s.GreaterOrEqual(len(it.Items), 10, "the gate's ways are under its join")
			}
		})
		s.True(found)
	}
}

// TestWays_Site_OpenFloorIsFree: the small site's two rooms on one floor
// have one free way between them.
func (s *JoinsSiteSuite) TestWays_Site_OpenFloorIsFree() {
	rb, err := readback.Of([]byte(s.raw))
	s.Require().NoError(err)
	ways := waysThrough(rb, "open:region-east:region-west")
	s.Require().Len(ways, 1)
	s.Equal(readback.KindWayFree, ways[0].Kind)
	s.Equal([]string{"free"}, methods(ways[0]))
}

// TestWays_Site_AClosedDoorIsFree: a door neither locked, held nor hidden
// between the walled rooms is a free way through its join.
func (s *JoinsSiteSuite) TestWays_Site_AClosedDoorIsFree() {
	rb, err := readback.Of(s.withWalls(wallAt("seam-wall", seamX, true, true, "seam-door"), "seam-door"))
	s.Require().NoError(err)
	ways := waysThrough(rb, "door:region-east:region-west:seam-door")
	s.Require().Len(ways, 1)
	s.Equal(readback.KindWayFree, ways[0].Kind)
	s.True(connected(ways[0], "door", "seam-door", "by"))
}
