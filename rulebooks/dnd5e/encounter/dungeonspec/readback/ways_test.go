// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

import (
	"fmt"
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
		if w.Kind == readback.KindWayLine && connected(w, "line", creature+"/"+line, "by") && connected(w, "creature", creature, "by") {
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

// methodDCs is a way's method connections as "id=DC".
func methodDCs(st readback.Statement) []string {
	var out []string
	for _, c := range st.Connects {
		if c.Kind == "method" {
			out = append(out, fmt.Sprintf("%s=%d", c.ID, c.Value))
		}
	}
	return out
}

// TestWays_Castle_EachMethodCarriesItsDC: a check's alternatives carry
// their own difficulty; a lock approach's ability and tool share theirs; a
// free way carries none.
func (s *WaysCastleSuite) TestWays_Castle_EachMethodCarriesItsDC() {
	s.Equal([]string{"skill:performance=13", "skill:deception=15"}, methodDCs(s.lineWay(gateJoin, "gate-1", "troupe")))
	var kitchen []string
	for _, w := range waysThrough(s.rb, "door:region-great-hall:region-kitchen:kitchen-wall-door") {
		kitchen = append(kitchen, methodDCs(w)...)
	}
	s.Equal([]string{"ability:str=15", "ability:dex=13", "tool:dnd5e:item:thieves-tools=13"}, kitchen)
	s.Equal([]string{"free=0"}, methodDCs(s.lineWay(gateJoin, "gate-1", "vouch")))
	s.Equal([]string{"fight=0"}, methodDCs(waysThrough(s.rb, gateJoin)[len(waysThrough(s.rb, gateJoin))-1]))
}

// TestWays_Castle_DCsTotalByMethodFromConnections: a consumer totals the
// difficulties of every way by method from the connections alone.
func (s *WaysCastleSuite) TestWays_Castle_DCsTotalByMethodFromConnections() {
	total := map[string]int{}
	for _, st := range s.rb.Statements() {
		if st.About.Kind != "way" {
			continue
		}
		for _, c := range st.Connects {
			if c.Kind == "method" {
				total[c.ID] += c.Value
			}
		}
	}
	// troupe on three carriers, straggler on two, guest, turn.
	s.Equal(3*15+2*15, total["skill:deception"])
	s.Equal(3*13, total["skill:performance"])
	s.Equal(2*13+11+15, total["skill:persuasion"])
	s.Equal(0, total["free"])
}

// TestWays_OnlyWaysCarryAValue: no statement but a way carries a number,
// so the line, lock and secret statements read as before.
func (s *WaysCastleSuite) TestWays_OnlyWaysCarryAValue() {
	for _, st := range s.rb.Statements() {
		if st.About.Kind == "way" {
			continue
		}
		for _, c := range st.Connects {
			s.Zero(c.Value, "%s %s", st.Kind, st.Path)
		}
	}
}

// TestWays_Castle_TheGuestClaimIsTheCaptains: Persuasion 11, the captain's alone.
func (s *WaysCastleSuite) TestWays_Castle_TheGuestClaimIsTheCaptains() {
	w := s.lineWay(gateJoin, "captain", "guest")
	s.Equal([]string{"skill:persuasion"}, methods(w))
	for _, other := range waysThrough(s.rb, gateJoin) {
		if connected(other, "line", "captain/guest", "by") || connected(other, "line", "gate-1/guest", "by") || connected(other, "line", "gate-2/guest", "by") {
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

// TestWays_Castle_TheArrestIsCarriedByGoingQuietly: entering `arrested`
// takes the party and moves nobody; the passage to the Cells is carried by
// `come-quietly`, a free line on each gate speaker that steps the
// conversation to `jailed`, whose jail sets the taken down. The misses that
// arrest are named per creature.
func (s *WaysCastleSuite) TestWays_Castle_TheArrestIsCarriedByGoingQuietly() {
	ways := waysThrough(s.rb, "passage:region-gatehouse+room-1-region:region-cells:conversation-front-gate")
	s.Require().Len(ways, 3)
	for i, who := range []string{"gate-1", "gate-2", "captain"} {
		w := ways[i]
		s.Equal(readback.KindWayPassage, w.Kind)
		s.True(connected(w, "line", who+"/come-quietly", "by"), w.Sentence)
		s.True(connected(w, "creature", who, "by"))
		s.True(connected(w, "conversation", "front-gate", "by"))
		s.True(connected(w, "topic", "arrested", "needs"))
		s.Equal([]string{"free"}, methods(w))
		s.Contains(w.Sentence, "steps `front-gate` to `jailed`, whose jail sets the taken down there")
		s.Contains(w.Sentence, "from `warned-guest` on `troupe-failed`")
		s.NotContains(w.Sentence, "Nobody chooses it")
		for _, line := range []string{"gate-1/troupe", "gate-2/troupe", "captain/troupe", "captain/guest"} {
			s.True(connected(w, "line", line, "arrests"), "%s arrests on a miss", line)
		}
		s.False(connected(w, "bar", "hold", "lifts"), "a passage's carrier lifts no bar")
	}
}

// TestWays_Castle_TheGatesWaysExactly: the gate's ways are exactly these,
// so a line that is not a way through (friends-inside, never-alive,
// come-quietly, the cook's and the pedlar's) is pinned absent.
func (s *WaysCastleSuite) TestWays_Castle_TheGatesWaysExactly() {
	var got []string
	for _, w := range waysThrough(s.rb, gateJoin) {
		by := ""
		for _, c := range w.Connects {
			if c.Kind == "line" && c.Role == "by" {
				by = c.ID
			}
		}
		bar := ""
		for _, c := range w.Connects {
			if c.Kind == "bar" {
				bar = c.ID
			}
		}
		got = append(got, string(w.Kind)+" "+bar+" "+by)
	}
	s.Equal([]string{
		"place.way.line hold gate-1/vouch", "place.way.line hold gate-1/straggler", "place.way.line hold gate-1/troupe",
		"place.way.line hold gate-2/vouch", "place.way.line hold gate-2/straggler", "place.way.line hold gate-2/troupe",
		"place.way.line hold captain/guest", "place.way.line hold captain/troupe", "place.way.line hold captain/stand-aside",
		"place.way.line hold captain/turn", "place.way.fight hold ",
	}, got)
}

// TestWays_Castle_TheLockedDoors: the kitchen door's two approaches and the
// cells door's one, each its own way.
func (s *WaysCastleSuite) TestWays_Castle_TheLockedDoors() {
	got := map[string][]string{}
	for _, join := range []string{"door:region-great-hall:region-kitchen:kitchen-wall-door", "door:region-barracks:region-cells:cells-wall-door"} {
		for _, w := range waysThrough(s.rb, join) {
			s.Equal(readback.KindWayLock, w.Kind, w.Sentence)
			s.True(connected(w, "bar", "lock", "lifts"))
			s.NotContains(w.Sentence, "also barred")
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
		s.True(connected(kinds[readback.KindWaySearch], "bar", "hidden", "lifts"))
		s.True(connected(kinds[readback.KindWayTold], "bar", "hidden", "lifts"))
		p := kinds[readback.KindWayPassage]
		s.Equal([]string{"free"}, methods(p))
		s.True(connected(p, "concealment", "kitchen-passage", "needs"))
		s.True(connected(p, "prop", "grate", "by"))
		s.True(connected(p, "line", "grate/"+map[string]string{"region-chapel": "to-chapel", "region-cells": "to-cells"}[to], "by"))
		s.Contains(p.Sentence, "not an alternative to it")
		for _, c := range p.Connects {
			s.NotEqual("bar", c.Kind, "the passage's carrier lifts no bar")
		}
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

// TestWays_FoldedUnderTheirJoin: in both folds the gate's join is one
// entry and its ways are exactly the entries beneath it.
func (s *WaysCastleSuite) TestWays_FoldedUnderTheirJoin() {
	var want []string
	for _, w := range waysThrough(s.rb, gateJoin) {
		want = append(want, w.Path)
	}
	for _, secs := range [][]readback.FoldedSection{s.rb.Folded(), s.rb.FoldedEach()} {
		var got []string
		walkItems(secs, func(it readback.Item) {
			if strings.Contains(it.Text, "join at the door `gate-wall-door`") {
				for _, c := range it.Items {
					got = append(got, c.Paths...)
				}
			}
		})
		s.Equal(want, got)
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

// WaysSiteSuite is the review's probes of rpg-toolkit#2443 on the small ways
// site: two holders on one door, judged per holder by the engine's pass.
type WaysSiteSuite struct {
	suite.Suite
	raw string
}

func TestWaysSiteSuite(t *testing.T) { suite.Run(t, new(WaysSiteSuite)) }

func (s *WaysSiteSuite) SetupTest() {
	raw, err := os.ReadFile("testdata/ways-site.yaml")
	s.Require().NoError(err)
	s.raw = string(raw)
}

const siteGate = "door:region-east:region-west:gate-door"

// read is the site with each old string replaced by its new one.
func (s *WaysSiteSuite) read(edits ...string) *readback.ReadBack {
	raw := s.raw
	for i := 0; i+1 < len(edits); i += 2 {
		s.Require().Contains(raw, edits[i])
		raw = strings.Replace(raw, edits[i], edits[i+1], 1)
	}
	rb, err := readback.Of([]byte(raw))
	s.Require().NoError(err)
	return rb
}

// byLine is the way through the site's gate carried by the line, or nil.
func byLine(rb *readback.ReadBack, line string) *readback.Statement {
	for _, w := range waysThrough(rb, siteGate) {
		if connected(w, "line", line, "by") {
			return &w
		}
	}
	return nil
}

// g2Pass is g2's post's pass, to replace.
const g2Pass = "            pass: { recognized: admitted }\n            at:\n              - { q: 1, r: 0 }"

// TestWays_Site_AGrantHeardByEveryHolderIsAWay: g1's `claim` makes g1 hold
// `admitted` and g2 hold it heard (g2 carries `gate`): every holder's pass
// is met.
func (s *WaysSiteSuite) TestWays_Site_AGrantHeardByEveryHolderIsAWay() {
	w := byLine(s.read(), "g1/claim")
	s.Require().NotNil(w)
	s.Equal(readback.KindWayLine, w.Kind)
	s.True(connected(*w, "bar", "hold", "lifts"))
	s.Contains(w.Sentence, "a holder who did not witness it keeps holding")
}

// TestWays_Site_ALineOnANonHolderIsNoWay (probe m1): `bribe` on a creature
// holding no post, with no conversation and no heard_by, reaches no holder.
func (s *WaysSiteSuite) TestWays_Site_ALineOnANonHolderIsNoWay() {
	s.Nil(byLine(s.read(), "idle/bribe"))
}

// TestWays_Site_APartialDrawOffIsPartial (mutant C): `scare` teaches `cowed`,
// which only g1's routine leaves its post on; g2 still holds.
func (s *WaysSiteSuite) TestWays_Site_APartialDrawOffIsPartial() {
	w := byLine(s.read(), "g1/scare")
	s.Require().NotNil(w)
	s.Equal(readback.KindWayPartial, w.Kind)
	s.True(connected(*w, "creature", "g1", "draws-off"))
	s.True(connected(*w, "creature", "g2", "still-holds"))
	s.Contains(w.Sentence, "It is not enough on its own: Guard (g2) still holds.")
}

// TestWays_Site_APasswordPassIsKnowledge (probe m4): g2 lets through only
// whoever knows `password`. `claim` no longer passes g2; knowing the
// password passes g2 but not g1. Both are partial; only the fight is a way.
func (s *WaysSiteSuite) TestWays_Site_APasswordPassIsKnowledge() {
	rb := s.read(g2Pass, "            pass: { fact: password }\n            at:\n              - { q: 1, r: 0 }")
	claim := byLine(rb, "g1/claim")
	s.Require().NotNil(claim)
	s.Equal(readback.KindWayPartial, claim.Kind)
	s.True(connected(*claim, "creature", "g2", "still-holds"))
	var known []readback.Statement
	for _, w := range waysThrough(rb, siteGate) {
		if connected(w, "fact", "password", "needs") && w.Kind != readback.KindWayLine {
			known = append(known, w)
		}
	}
	s.Require().Len(known, 1)
	s.Equal(readback.KindWayPartial, known[0].Kind)
	s.Equal([]string{"knowledge"}, methods(known[0]))
	s.True(connected(known[0], "creature", "g1", "still-holds"))
}

// TestWays_Site_EveryHolderOnAPasswordIsAKnownWay: when both holders pass
// on the password alone, knowing it is a way.
func (s *WaysSiteSuite) TestWays_Site_EveryHolderOnAPasswordIsAKnownWay() {
	rb := s.read(g2Pass, "            pass: { fact: password }\n            at:\n              - { q: 1, r: 0 }",
		"            pass: { recognized: admitted }\n            at:\n              - { q: 2, r: 0 }",
		"            pass: { fact: password }\n            at:\n              - { q: 2, r: 0 }")
	var kinds []readback.Kind
	for _, w := range waysThrough(rb, siteGate) {
		kinds = append(kinds, w.Kind)
	}
	s.Contains(kinds, readback.KindWayKnown)
	s.Nil(byLine(rb, "g1/claim"), "a grant meets no pass made only of a fact")
}

// TestWays_Site_AMixedPassNeedsItsFact (probe m3): g2's pass is `admitted`
// AND the password; `claim` is a way that needs the password too.
func (s *WaysSiteSuite) TestWays_Site_AMixedPassNeedsItsFact() {
	rb := s.read(g2Pass, "            pass: [{ recognized: admitted }, { fact: password }]\n            at:\n              - { q: 1, r: 0 }")
	w := byLine(rb, "g1/claim")
	s.Require().NotNil(w)
	s.Equal(readback.KindWayLine, w.Kind)
	s.True(connected(*w, "fact", "password", "needs"))
	s.Contains(w.Sentence, "the walker must also know the fact `password`")
}

// TestWays_Site_ALockedAndHeldDoorHasTwoBars (probe m5): the lock's ways
// lift the lock, the holders' ways the hold, and each says the other bar.
func (s *WaysSiteSuite) TestWays_Site_ALockedAndHeldDoorHasTwoBars() {
	rb := s.read("      gate-door: {closed: true}", "      gate-door:\n        closed: true\n        locked:\n          - ability: str\n            dc: 12")
	bars := map[string][]readback.Kind{}
	for _, w := range waysThrough(rb, siteGate) {
		for _, c := range w.Connects {
			if c.Kind == "bar" {
				bars[c.ID] = append(bars[c.ID], w.Kind)
			}
		}
		if w.Kind == readback.KindWayLock || w.Kind == readback.KindWayFight {
			s.Contains(w.Sentence, "lifting this bar alone does not get a walker through", w.Sentence)
		}
	}
	s.Equal([]readback.Kind{readback.KindWayLock}, bars["lock"])
	s.Contains(bars["hold"], readback.KindWayFight)
	s.Contains(bars["hold"], readback.KindWayLine)
}

// TestWays_Site_AnArrestWithNoJailHasNoWay: the hail site's arrest has no
// jail, so nobody passes and no way is said (the join says so).
func (s *WaysSiteSuite) TestWays_Site_AnArrestWithNoJailHasNoWay() {
	rb, err := readback.Of(arrestingHailSite(s.T(), "    approach: { within: 2 }\n"))
	s.Require().NoError(err)
	var passages int
	for _, j := range joinsOf(rb) {
		if j.Kind == readback.KindJoinPassage {
			passages++
			s.Empty(waysThrough(rb, j.About.ID))
		}
	}
	s.Equal(1, passages)
}
