// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

type CombatEndTestSuite struct {
	suite.Suite
}

func TestCombatEndSuite(t *testing.T) { suite.Run(t, new(CombatEndTestSuite)) }

// journalKiller is journalStriker's twin for the ending that nobody asks for:
// it records the swing on the SAME list the announcer writes to, and the swing
// it wraps drops its target.
//
// journalStriker cannot be reused because its inner is a *scriptedStriker and
// the whole point here is the one that kills.
type journalKiller struct {
	j     *journal
	inner *killerStriker
}

func (k journalKiller) Strike(
	ctx context.Context, enc *encounter.Encounter, attacker, target encounter.MemberID, action core.Ref,
) error {
	k.j.add("strike:" + string(attacker))
	return k.inner.Strike(ctx, enc, attacker, target, action)
}

// failAfterForming works until a test switches it off.
//
// It has to exist because forming a fight ALREADY announces — a fight's first
// turn start, which rpg-project#294 added — so an announcer that fails from the
// start cannot build the fight whose ending is under test. Switching it on
// afterwards isolates the failure to the ending, and the construction
// succeeding first is its own small proof that the form-time announcement is
// real.
type failAfterForming struct {
	j    *journal
	fail *error
}

func (a failAfterForming) Announce(
	_ context.Context, _ *encounter.Encounter, crossed []encounter.Boundary,
) error {
	for _, b := range crossed {
		a.j.add(fmt.Sprintf("%s:%s:r%d", b.Kind, b.Subject, b.Round))
	}
	if a.fail != nil {
		return *a.fail
	}
	return nil
}

// fightWithStanding is boundary_test's fightWithMonsters with the Standing
// capability opened up as well.
//
// A fight that ends ITSELF is half of what this file is about, and
// everyoneStanding can never produce one — nobody is ever down, so no fight is
// ever decided.
func (s *CombatEndTestSuite) fightWithStanding(
	driver encounter.TurnDriver, striker encounter.Striker, announcer encounter.Announcer,
	standing encounter.Standing, monsters ...core.EntityID,
) (*encounter.Encounter, error) {
	members := []encounter.MemberInput{
		{ID: alice, Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 2}},
	}
	for i, id := range monsters {
		members = append(members, encounter.MemberInput{
			ID: id, Kind: encounter.KindMonster,
			Position:  spatial.Position{X: float64(3 + i), Y: 2},
			SpeedFeet: 30, Targeting: "closest",
			Actions: []encounter.ActionView{
				{Ref: testMeleeAction, Name: "Shortsword", RangeFeet: 5, Kind: "melee"},
			},
		})
	}
	return encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: standing, Initiative: orderAsGiven{},
		TurnDriver: driver, Striker: striker, Mover: quietMover{}, Announcer: announcer,
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: encounter.HexesArePointyTop()},
			Regions: []encounter.RegionInput{rectRegion(room1, 0, 0, 10, 10)},
		},
		Members: members,
		Endings: []encounter.EndingInput{{Key: "called", Trigger: encounter.TriggerExternal{}}},
	})
}

// combatEndsIn returns the subjects of every combat-end boundary the journal
// saw, sorted — so a test asserts on WHO the fight ended for without also
// asserting on an order the composition never promised.
func combatEndsIn(j *journal) []string {
	var subjects []string
	for _, e := range j.entries {
		rest, ok := strings.CutPrefix(e, "combat_ended:")
		if !ok {
			continue
		}
		subjects = append(subjects, strings.Split(rest, ":")[0])
	}
	sort.Strings(subjects)
	return subjects
}

// TestADecidedEndingReachesEveryMemberOfTheFight is the fan-out, and it is the
// whole shape of combat end rather than a detail of it.
//
// DissolveInput names ONE member — the fight is reached through anybody in it
// (R6) — but the fight ends for everybody it held. The only subscriber that
// exists asks whether an ending is its own
// (dnd5e/conditions.RagingCondition.onCombatEnd), so a single ending announced
// once would expire nothing at all.
//
// The members the caller did NOT name are the assertion — and they are
// monsters, which is deliberate. Nothing on a monster subscribes to combat end
// today, and the ending is announced for them anyway: which members of a fight
// an ending is "about" is the effect's own predicate to answer, not a filter
// for this composition to hold (R3).
func (s *CombatEndTestSuite) TestADecidedEndingReachesEveryMemberOfTheFight() {
	j := &journal{}
	enc, err := s.fightWithStanding(
		passDriver{}, passStriker{}, journalAnnouncer{j: j}, everyoneStanding{}, goblin, bob)
	s.Require().NoError(err)

	out, err := enc.Dissolve(&encounter.DissolveInput{Member: alice})
	s.Require().NoError(err)

	held := make([]string, 0, len(out.Members))
	for _, id := range out.Members {
		held = append(held, string(id))
	}
	sort.Strings(held)

	s.Require().Len(held, 3, "precondition: the fight held all three: %v", held)
	s.Equal(held, combatEndsIn(j),
		"the fight ended for exactly the members it held, no more and no fewer: %v", j.entries)
	s.Contains(combatEndsIn(j), string(bob),
		"a member the caller never named still had their fight end: %v", j.entries)
}

// TestAnEndingNobodyAskedForIsAnnouncedToo is the other caller of the one
// place a fight ends, and the one no host controls.
//
// A driven monster's own killing blow decides the fight from inside Record,
// which Strike calls before returning — several frames down inside the
// caller's EndTurn. Nothing out there gets a chance to announce this, which is
// why the announcement lives at the ending itself rather than at either call
// site.
func (s *CombatEndTestSuite) TestAnEndingNobodyAskedForIsAnnouncedToo() {
	j := &journal{}
	standing := &downList{}
	striker := journalKiller{j: j, inner: &killerStriker{
		scriptedStriker: &scriptedStriker{kind: encounter.OutcomeStruck},
		standing:        standing,
	}}
	enc, err := s.fightWithStanding(
		&scriptedDriver{intents: []encounter.TurnIntent{
			encounter.Attack{Target: alice, Action: testMeleeAction},
		}},
		striker, journalAnnouncer{j: j}, standing, goblin)
	s.Require().NoError(err)

	// alice ends her turn; the goblin is driven, swings, and drops the only
	// player in the fight — so the fight decides itself mid-drive.
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)

	s.Equal([]string{"alice", "goblin"}, combatEndsIn(j),
		"a fight that ended itself still ended for everyone in it: %v", j.entries)

	swung := j.indexOf("strike:goblin")
	s.Require().NotEqual(-1, swung, "the goblin never swung: %v", j.entries)
	for i, e := range j.entries {
		if strings.HasPrefix(e, "combat_ended:") {
			s.Greater(i, swung,
				"the fight ends because of the swing, so it is announced after it: %v", j.entries)
		}
	}
}

// TestACombatEndCarriesTheRoundTheFightEndedOn pins the one number in the
// answer to the clock that produced it rather than to a default.
//
// It matters more than it looks. Round numbers are PER-FIGHT — clock.Turn's
// SetOrder starts every bubble at 1 and Dissolve sets it back to 0 — so a zero
// here would be indistinguishable from "the clock had no idea", which is the
// shape of the bug that made this whole slice necessary.
func (s *CombatEndTestSuite) TestACombatEndCarriesTheRoundTheFightEndedOn() {
	j := &journal{}
	enc, err := s.fightWithStanding(
		passDriver{}, passStriker{}, journalAnnouncer{j: j}, everyoneStanding{}, goblin)
	s.Require().NoError(err)

	// alice ends, the goblin is driven and passes, the order wraps to round 2.
	_, err = enc.EndTurn(&encounter.EndTurnInput{Member: alice})
	s.Require().NoError(err)
	s.Require().Contains(j.entries, "turn_started:alice:r2",
		"precondition: the fight is in round 2: %v", j.entries)

	_, err = enc.Dissolve(&encounter.DissolveInput{Member: alice})
	s.Require().NoError(err)

	s.Contains(j.entries, "combat_ended:alice:r2",
		"the ending carries the round it happened in, not round one and not zero: %v", j.entries)
}

// TestAnAnnouncerMalfunctionAbortsTheDissolve — the same contract Striker and
// TurnDriver have, and for the same reason. Nothing is persisted until the
// caller's own commit, so a failure costs the retry and nothing else; a
// dissolve that reported success while the boundary it owed nobody published
// would cost a barbarian's rage, silently, forever.
func (s *CombatEndTestSuite) TestAnAnnouncerMalfunctionAbortsTheDissolve() {
	var live error
	j := &journal{}
	enc, err := s.fightWithStanding(
		passDriver{}, passStriker{}, failAfterForming{j: j, fail: &live}, everyoneStanding{}, goblin)
	s.Require().NoError(err)

	boom := errors.New("announcer is down")
	live = boom

	_, err = enc.Dissolve(&encounter.DissolveInput{Member: alice})
	s.Require().ErrorIs(err, boom, "an announcer malfunction fails the verb that caused it")
}

// TestEveryPathThatEndsAFightAnnouncesIt is the structural half, in the shape
// TestNoCodePathProducesACastlessInteraction established.
//
// The two behavioural tests above cover the two callers of dissolveBubble that
// exist TODAY. That is only a complete claim while there are two, and a
// behavioural suite structurally cannot notice a third being added — which is
// precisely how CombatEndTopic came to have seven subscribers' worth of
// attention and no publisher at all.
//
// So the count is asserted directly, off the AST. A third way to end a fight
// fails this test and lands the author here, at the two tests that would need
// a sibling.
func (s *CombatEndTestSuite) TestEveryPathThatEndsAFightAnnouncesIt() {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	s.Require().NoError(err)
	pkg, ok := pkgs["encounter"]
	s.Require().True(ok, "the encounter package itself must be parseable")

	callers := map[string]bool{}
	var announces bool
	for _, file := range pkg.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			fn, isFunc := n.(*ast.FuncDecl)
			if !isFunc {
				return true
			}
			ast.Inspect(fn.Body, func(inner ast.Node) bool {
				call, isCall := inner.(*ast.CallExpr)
				if !isCall {
					return true
				}
				sel, isSel := call.Fun.(*ast.SelectorExpr)
				if !isSel {
					return true
				}
				switch sel.Sel.Name {
				case "dissolveBubble":
					callers[fn.Name.Name] = true
				case "announceBoundaries":
					if fn.Name.Name == "dissolveBubble" {
						announces = true
					}
				}
				return true
			})
			return true
		})
	}

	s.True(announces,
		"dissolveBubble itself must announce — that is what makes both callers safe")

	names := make([]string, 0, len(callers))
	for name := range callers {
		names = append(names, name)
	}
	sort.Strings(names)
	s.Equal([]string{"Dissolve", "noticeDown"}, names,
		"a third way to end a fight needs a third test above; these two are covered by "+
			"TestADecidedEndingReachesEveryMemberOfTheFight and TestAnEndingNobodyAskedForIsAnnouncedToo")

	// A guard on the guard: if the walk above found nothing at all it would
	// pass an empty comparison against an empty expectation and prove nothing.
	s.Require().NotEmpty(callers, "the AST walk found no callers, so it tested nothing")
}
