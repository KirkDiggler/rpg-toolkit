// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback_test

import (
	"os"
	"regexp"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
)

// connsOf is the ids of a statement's connections of a kind and role, sorted.
func connsOf(st readback.Statement, kind, role string) []string {
	out := []string{}
	for _, c := range st.Connects {
		if c.Kind == kind && c.Role == role {
			out = append(out, c.ID)
		}
	}
	sort.Strings(out)
	return out
}

// MachineSiteSuite holds a routine's shape (rpg-project#994) to the small
// machine site: every state, step, beat, pick, row, post and blocking state
// said as a connection, so the machine is drawn without a sentence.
type MachineSiteSuite struct {
	suite.Suite
	rb *readback.ReadBack
}

func TestMachineSiteSuite(t *testing.T) { suite.Run(t, new(MachineSiteSuite)) }

func (s *MachineSiteSuite) SetupSuite() {
	raw, err := os.ReadFile("testdata/machine-site.yaml")
	s.Require().NoError(err)
	s.rb, err = readback.Of(raw)
	s.Require().NoError(err)
}

// at is the statement of the kind at path.
func (s *MachineSiteSuite) at(kind readback.Kind, path string) readback.Statement {
	for _, st := range s.rb.Statements() {
		if st.Kind == kind && st.Path == path {
			return st
		}
	}
	s.Require().Failf("no statement", "no %s at %s", kind, path)
	return readback.Statement{}
}

const watch = "behaviorDefinitions.watch"

func (s *MachineSiteSuite) TestMachine_Site_TheRoutineSaysWhereItStarts() {
	st := s.at(readback.KindRoutine, watch)
	s.Equal([]string{"posted"}, connsOf(st, readback.ConnState, readback.RoleStarts))
	s.True(connected(st, "creature", "g", "runs"), "the existing runner connection stays")
}

func (s *MachineSiteSuite) TestMachine_Site_EachStateSaysItself() {
	for _, state := range []string{"posted", "alert", "gone"} {
		st := s.at(readback.KindRoutineState, watch+".recipe.states."+state)
		s.Equal([]string{state}, connsOf(st, readback.ConnState, readback.RoleIs), state)
	}
}

func (s *MachineSiteSuite) TestMachine_Site_OnlyTheTerminalStateIsFinal() {
	gone := s.at(readback.KindRoutineState, watch+".recipe.states.gone")
	s.Equal([]string{"gone"}, connsOf(gone, readback.ConnState, readback.RoleFinal))
	for _, state := range []string{"posted", "alert"} {
		st := s.at(readback.KindRoutineState, watch+".recipe.states."+state)
		s.Empty(connsOf(st, readback.ConnState, readback.RoleFinal), state)
	}
}

func (s *MachineSiteSuite) TestMachine_Site_AStepThatStaysHasFromAndOnButNoTo() {
	st := s.at(readback.KindRoutineWake, watch+".recipe.states.posted.transitions[0]")
	s.Equal([]string{"posted"}, connsOf(st, readback.ConnState, readback.RoleFrom))
	s.Equal([]string{"time"}, connsOf(st, readback.ConnBeat, readback.RoleOn))
	s.Empty(connsOf(st, readback.ConnState, readback.RoleTo))
}

func (s *MachineSiteSuite) TestMachine_Site_AWakeSaysItsPick() {
	st := s.at(readback.KindRoutineWake, watch+".recipe.states.posted.transitions[0]")
	s.Equal([]string{"post"}, connsOf(st, readback.ConnTable, readback.RolePicks))
}

func (s *MachineSiteSuite) TestMachine_Site_AStepThatMovesSaysWhere() {
	cowed := s.at(readback.KindRoutineWake, watch+".recipe.states.posted.transitions[1]")
	s.Equal([]string{"posted"}, connsOf(cowed, readback.ConnState, readback.RoleFrom))
	s.Equal([]string{"gone"}, connsOf(cowed, readback.ConnState, readback.RoleTo))
	s.Equal([]string{"fact"}, connsOf(cowed, readback.ConnBeat, readback.RoleOn))
	s.Empty(connsOf(cowed, readback.ConnTable, readback.RolePicks), "a step that acts on no rows picks none")
	s.True(connected(cowed, "fact", "cowed", "wakes-on"), "the guard's read stays")

	back := s.at(readback.KindRoutineWake, watch+".recipe.states.alert.transitions[0]")
	s.Equal([]string{"alert"}, connsOf(back, readback.ConnState, readback.RoleFrom))
	s.Equal([]string{"posted"}, connsOf(back, readback.ConnState, readback.RoleTo))
	s.Equal([]string{"post"}, connsOf(back, readback.ConnTable, readback.RolePicks))
}

func (s *MachineSiteSuite) TestMachine_Site_EachRowSaysItsTable() {
	for path, key := range map[string]string{
		watch + ".table.post[0]":   "post",
		watch + ".table.barred[0]": "barred",
		watch + ".table.barred[1]": "barred",
	} {
		st := s.at(readback.KindRoutineRow, path)
		s.Equal([]string{key}, connsOf(st, readback.ConnTable, readback.RoleRowOf), path)
	}
}

func (s *MachineSiteSuite) TestMachine_Site_APostIsHeldWhileItsState() {
	for _, state := range []string{"posted", "alert"} {
		st := s.at(readback.KindPost, "room.room.monsterBindings.g.posts."+state)
		s.Equal([]string{state}, connsOf(st, readback.ConnState, readback.RoleWhile), state)
		s.True(connected(st, "door", "gate-door", "holds"), "the door connection stays")
	}
}

func (s *MachineSiteSuite) TestMachine_Site_TheBodyBlocksWhileEachState() {
	st := s.at(readback.KindBlocking, "room.room.monsterBindings.g.blockingStates")
	s.Equal([]string{"alert", "posted"}, connsOf(st, readback.ConnState, readback.RoleWhile))
}

// MachineCastleSuite holds every castle routine's step connections to the
// file, read straight off the YAML with no help from the package under test.
type MachineCastleSuite struct {
	suite.Suite
	doc *yaml.Node
	rb  *readback.ReadBack
}

func TestMachineCastleSuite(t *testing.T) { suite.Run(t, new(MachineCastleSuite)) }

func (s *MachineCastleSuite) SetupSuite() {
	raw, err := os.ReadFile(castlePath)
	s.Require().NoError(err)
	var n yaml.Node
	s.Require().NoError(yaml.Unmarshal(raw, &n))
	s.doc = n.Content[0]
	s.rb, err = readback.Of(raw)
	s.Require().NoError(err)
}

var wakePath = regexp.MustCompile(`^behaviorDefinitions\.([^.]+)\.recipe\.states\.([^.]+)\.transitions\[(\d+)\]$`)

// TestMachine_Castle_EveryStepMatchesTheFile: each wake's from, to, beat and
// picks are the authored transition's, and the gate guard's starts `posted`.
func (s *MachineCastleSuite) TestMachine_Castle_EveryStepMatchesTheFile() {
	defs := child(s.doc, "behaviorDefinitions")
	seen := 0
	for _, st := range s.rb.Statements() {
		if st.Kind != readback.KindRoutineWake || st.About.Kind != "routine" {
			continue
		}
		m := wakePath.FindStringSubmatch(st.Path)
		s.Require().NotNil(m, st.Path)
		k, _ := strconv.Atoi(m[3])
		trs := child(child(child(child(child(defs, m[1]), "recipe"), "states"), m[2]), "transitions")
		s.Require().NotNil(trs, st.Path)
		tr := trs.Content[k]
		seen++

		s.Equal([]string{m[2]}, connsOf(st, readback.ConnState, readback.RoleFrom), st.Path)
		s.Equal([]string{child(tr, "on").Value}, connsOf(st, readback.ConnBeat, readback.RoleOn), st.Path)
		if to := child(tr, "to"); to != nil {
			s.Equal([]string{to.Value}, connsOf(st, readback.ConnState, readback.RoleTo), st.Path)
		} else {
			s.Empty(connsOf(st, readback.ConnState, readback.RoleTo), st.Path)
		}
		picks := []string{}
		entries(child(tr, "do"), "", func(act *yaml.Node, _ string) {
			if p := child(act, "pick"); p != nil {
				picks = append(picks, p.Value)
			}
		})
		sort.Strings(picks)
		s.Equal(picks, connsOf(st, readback.ConnTable, readback.RolePicks), st.Path)
	}
	s.Positive(seen, "the castle has routine steps")

	g1 := s.statementAt(readback.KindRoutine, "behaviorDefinitions.gate-1-policy")
	s.Equal([]string{"posted"}, connsOf(g1, readback.ConnState, readback.RoleStarts))
}

// TestMachine_Castle_FinalOnlyWhereTheFileSaysTerminal: no state with a way
// out is final.
func (s *MachineCastleSuite) TestMachine_Castle_FinalOnlyWhereTheFileSaysTerminal() {
	defs := child(s.doc, "behaviorDefinitions")
	for _, st := range s.rb.Statements() {
		if st.Kind != readback.KindRoutineState {
			continue
		}
		m := regexp.MustCompile(`^behaviorDefinitions\.([^.]+)\.recipe\.states\.([^.]+)$`).FindStringSubmatch(st.Path)
		s.Require().NotNil(m, st.Path)
		node := child(child(child(child(defs, m[1]), "recipe"), "states"), m[2])
		terminal := child(node, "terminal") != nil && child(node, "terminal").Value == "true"
		s.Equal([]string{m[2]}, connsOf(st, readback.ConnState, readback.RoleIs), st.Path)
		if terminal {
			s.Equal([]string{m[2]}, connsOf(st, readback.ConnState, readback.RoleFinal), st.Path)
		} else {
			s.Empty(connsOf(st, readback.ConnState, readback.RoleFinal), st.Path)
		}
	}
}

// TestMachine_Castle_TheGateGuardHoldsWhileItsStates: gate-1 blocks and
// holds its door in the states the file lists.
func (s *MachineCastleSuite) TestMachine_Castle_TheGateGuardHoldsWhileItsStates() {
	bind := child(child(child(child(s.doc, "room"), "room"), "monsterBindings"), "gate-1")
	var blocking []string
	entries(child(bind, "blockingStates"), "", func(n *yaml.Node, _ string) { blocking = append(blocking, n.Value) })
	sort.Strings(blocking)
	st := s.statementAt(readback.KindBlocking, "room.room.monsterBindings.gate-1.blockingStates")
	s.Equal(blocking, connsOf(st, readback.ConnState, readback.RoleWhile))

	posts := child(bind, "posts")
	s.Require().NotNil(posts)
	for i := 0; i+1 < len(posts.Content); i += 2 {
		state := posts.Content[i].Value
		p := s.statementAt(readback.KindPost, "room.room.monsterBindings.gate-1.posts."+state)
		s.Equal([]string{state}, connsOf(p, readback.ConnState, readback.RoleWhile), state)
	}
}

// TestMachine_Castle_AHubRepeatIsScopedByItsPath: a step repeated under a
// fact hub keeps its shape, and the routine its path names owns the state.
func (s *MachineCastleSuite) TestMachine_Castle_AHubRepeatIsScopedByItsPath() {
	repeats := 0
	for _, st := range s.rb.Statements() {
		if st.Kind != readback.KindFactAsk || len(connsOf(st, readback.ConnState, readback.RoleFrom)) == 0 {
			continue
		}
		repeats++
		s.Equal("fact", st.About.Kind, st.Path)
		m := wakePath.FindStringSubmatch(st.Path)
		s.Require().NotNil(m, "a repeat's path names the routine that owns the step: %s", st.Path)
		s.Equal([]string{m[2]}, connsOf(st, readback.ConnState, readback.RoleFrom), st.Path)
		own := s.statementAt(readback.KindRoutineWake, st.Path)
		s.Equal("routine", own.About.Kind)
		s.Equal(m[1], own.About.ID, "the owning routine is the one the path names")
		s.Equal(own.Connects, st.Connects, "the repeat keeps the step's connections")
	}
	s.Positive(repeats, "the castle repeats routine steps under fact hubs")
}

// statementAt is the first statement of the kind at path (the first is
// where it was said; a hub repeat comes later and has another kind).
func (s *MachineCastleSuite) statementAt(kind readback.Kind, path string) readback.Statement {
	for _, st := range s.rb.Statements() {
		if st.Kind == kind && st.Path == path {
			return st
		}
	}
	s.Require().Failf("no statement", "no %s at %s", kind, path)
	return readback.Statement{}
}
