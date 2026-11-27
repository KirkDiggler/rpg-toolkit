// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/readback"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monstertraits"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// template_lets_test.go plays "a creature is built with what its stat block lets
// it use" (rpg-project#1131, slice rpg-project#1229) on
// testdata/lieutenant-holds-second-wind-walk.yaml: a Lieutenant whose stat block
// says `lets: second wind` with one use of his own. The slice makes the ability
// and its count part of the creature's stored sheet and makes the content check
// judge the stat block's words. NOTHING USES THE ABILITY: no behaviour chooses it.
//
// The dice are secondWindDice with strike off, as a_run_and_a_retreat_test.go has
// them: every die is min(size, 10). The fighter has 60 of 60 hit points and only
// ends her turn.

const lieutenantSitePath = "testdata/lieutenant-holds-second-wind-walk.yaml"

func lieutenantSite(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(lieutenantSitePath)
	require.NoError(t, err)
	return raw
}

// lieutenantVariant is the site with one line of the stat block's `lets` replaced.
func lieutenantVariant(t *testing.T, from, to string) []byte {
	t.Helper()
	raw := lieutenantSite(t)
	require.Equal(t, 1, bytes.Count(raw, []byte(from)), "precondition: %q occurs once in the site", from)
	return bytes.Replace(raw, []byte(from), []byte(to), 1)
}

func compileSite(t *testing.T, raw []byte) dungeonspec.Compiled {
	t.Helper()
	compiled, err := dungeonspec.Load(raw)
	require.NoError(t, err, "the site compiles: the words inside `lets` are the rules' to judge")
	return compiled
}

// lieutenantScene is a site launched with a level 1 fighter who only ends her turn.
type lieutenantScene struct {
	t        *testing.T
	mgr      *session.Manager
	sessions *fakeSessions
	enc      *fakeEncounters
}

func newLieutenantScene(t *testing.T, compiled dungeonspec.Compiled) (*lieutenantScene, error) {
	t.Helper()
	fighter := armedFighter("alice")
	setLevel(fighter, 1)
	fighter.HitPoints, fighter.MaxHitPoints = 60, 60
	s := &lieutenantScene{t: t, sessions: newFakeSessions(), enc: newFakeEncounters()}
	var err error
	s.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: &actCapture{}, Seats: newFakeSeats(),
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{}, Dice: &secondWindDice{},
		TurnDriver: session.Driver(), Sessions: s.sessions, Encounters: s.enc, Characters: newFakeCharacters(fighter),
	})
	require.NoError(t, err)
	_, err = s.mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice"}})
	return s, err
}

func launchedLieutenant(t *testing.T) *lieutenantScene {
	t.Helper()
	s, err := newLieutenantScene(t, compileSite(t, lieutenantSite(t)))
	require.NoError(t, err)
	return s
}

// sheet is the stored sheet of the NPC with that id now.
func (s *lieutenantScene) sheet(id string) *monster.Data {
	s.t.Helper()
	for i := range s.sessions.byID["sess"].NPCs {
		if s.sessions.byID["sess"].NPCs[i].ID == id {
			return &s.sessions.byID["sess"].NPCs[i]
		}
	}
	require.FailNow(s.t, "no stored sheet", id)
	return nil
}

// Test 1. Launched with it: the stored sheet holds the pool, the grant and a full count.
func TestALieutenantIsLaunchedHoldingSecondWindAndItsCount(t *testing.T) {
	sheet := launchedLieutenant(t).sheet("lieutenant")

	require.NotNil(t, sheet.Kit, "the stat block's `lets` is a kit on the stored sheet")
	require.Len(t, sheet.Kit.Pools, 1)
	pool, ok := sheet.Kit.Pools["second-wind"]
	require.True(t, ok, "the pool is filed under the ability's name")
	require.Equal(t, 1, pool.Maximum)
	require.Len(t, sheet.Kit.Grants, 1)
	require.Contains(t, sheet.Kit.Grants, "second-wind")
	require.Equal(t, map[string]int{"second-wind": 1}, sheet.Left, "the count starts full")
}

// Test 2. The rules can describe the ability from the stored sheet alone.
func TestTheRulesDescribeTheLieutenantsAbilityFromHisStoredSheet(t *testing.T) {
	sheet := launchedLieutenant(t).sheet("lieutenant")

	loaded, err := monstertraits.LoadMonster(context.Background(), sheet)
	require.NoError(t, err)
	require.Equal(t, []string{"second-wind"}, loaded.Grants())
	out, err := loaded.GrantedAbility(&monster.GrantedAbilityInput{Grant: "second-wind"})
	require.NoError(t, err)
	require.NotNil(t, out.Definition.Cost)
	require.Equal(t, 1, out.Definition.Cost.Slots["bonus_action"], "it costs one bonus action")
	require.Equal(t, 1, out.Definition.Cost.Pools[monster.PoolKey("second-wind")], "and one of his own uses")
	require.Len(t, out.Definition.Cost.Pools, 1)
}

// Test 3. A fight leaves it alone: he acts as his table says and his count stays 1.
func TestAFightLeavesTheLieutenantsSecondWindAlone(t *testing.T) {
	s := launchedLieutenant(t)
	acted := 0
	for i := 0; i < 2; i++ {
		mark := lastSeq(storyBeats(t, s.mgr, "sess", "alice"))
		_, err := s.mgr.EndTurn(context.Background(), &session.EndTurnInput{
			Session: "sess", Member: "alice", DeclarationID: currentEndTurnID(t, s.mgr, "sess", "alice"),
		})
		require.NoError(t, err, "round %d: EndTurn returns no error", i+1)
		for _, b := range storyBeatsAfter(t, s.mgr, "sess", "alice", mark) {
			require.NotEqual(t, "activated", b.Beat, "round %d: nothing used an ability", i+1)
			if b.Fields["actor"] == "lieutenant" || b.Fields["member"] == "lieutenant" {
				acted++
			}
		}
		require.Equal(t, map[string]int{"second-wind": 1}, s.sheet("lieutenant").Left, "round %d: the count is untouched", i+1)
	}
	require.Positive(t, acted, "the Lieutenant took his turns: he walked and swung as his table says")
}

// Test 4. The content check accepts the site, and the read-back says the stat block's lets.
func TestTheContentCheckAcceptsTheLieutenantSite(t *testing.T) {
	compiled := compileSite(t, lieutenantSite(t))
	checked, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
	require.NoError(t, err)
	require.Empty(t, checked.Refusals)
	require.Empty(t, checked.Items)
	require.Empty(t, checked.Templates)

	rb, err := readback.Of(lieutenantSite(t))
	require.NoError(t, err)
	require.Empty(t, rb.NotYetSaid, "every word the site writes is one the read-back can say")
	found := false
	for _, st := range rb.Statements() {
		if st.Kind == readback.KindTemplateLets {
			found = true
		}
	}
	require.True(t, found, "the read-back has the place.template.lets statement")
}

// Test 5. The content check refuses a stat block the rules cannot run, at its place.
func TestTheContentCheckRefusesAStatBlockTheRulesCannotRun(t *testing.T) {
	cases := []struct {
		name     string
		from, to string
		contains []string
	}{
		{"an ability the rules do not know", "use: second wind", "use: fireball", []string{"fireball", "second wind"}},
		{"a count that cannot be used", "max: 1", "max: 0", []string{"uses.max"}},
		{"a cost a stat block does not declare", "uses: { max: 1, refills: { on: [short rest, long rest] } }", "cost: { charges: 1 }", []string{"charges"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			compiled := compileSite(t, lieutenantVariant(t, c.from, c.to))
			checked, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
			require.NoError(t, err)
			require.Empty(t, checked.Refusals)
			require.Empty(t, checked.Items)
			require.Len(t, checked.Templates, 1)
			r := checked.Templates[0]
			require.Equal(t, "gate-lieutenant", r.TemplateID)
			require.Equal(t, "templates.gate-lieutenant.lets", r.Path)
			require.Contains(t, r.Message, `stat block "gate-lieutenant"`)
			require.NotContains(t, r.Message, "item")
			for _, want := range c.contains {
				require.Contains(t, r.Message, want)
			}
			t.Logf("%s: %s", c.name, r.Message)
		})
	}
}

// Test 6. A launch of each bad site is refused by the stat block's own assembly
// (ErrInvalidWorld, the rules' sentence as text), and launches nothing.
func TestALaunchRefusesAStatBlockTheRulesCannotRunAndLaunchesNothing(t *testing.T) {
	cases := []struct {
		name     string
		from, to string
		contains string
	}{
		{"an ability the rules do not know", "use: second wind", "use: fireball", "fireball"},
		{"a count that cannot be used", "max: 1", "max: 0", "uses.max"},
		{"a cost a stat block does not declare", "uses: { max: 1, refills: { on: [short rest, long rest] } }", "cost: { charges: 1 }", "charges"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := newLieutenantScene(t, compileSite(t, lieutenantVariant(t, c.from, c.to)))
			require.ErrorIs(t, err, session.ErrInvalidWorld)
			require.ErrorContains(t, err, `stat block "gate-lieutenant"`)
			require.ErrorContains(t, err, c.contains)
			require.Empty(t, s.sessions.byID, "no session was stored")
			require.Zero(t, s.enc.saves, "no encounter was stored")
			t.Logf("launch: %v", err)
		})
	}
}

// Test 7. A site with no `lets` is as it was.
func TestASiteWithNoLetsIsAsItWas(t *testing.T) {
	raw, err := os.ReadFile(runWalkSitePath)
	require.NoError(t, err)
	compiled := compileSite(t, raw)

	checked, err := session.CheckContent(&session.CheckContentInput{Dungeon: &compiled})
	require.NoError(t, err)
	require.Empty(t, checked.Refusals)
	require.Empty(t, checked.Items)
	require.Empty(t, checked.Templates)

	s, err := newLieutenantScene(t, compiled)
	require.NoError(t, err)
	runner := s.sheet("runner")
	require.Nil(t, runner.Kit, "no `lets` is no kit")
	require.Empty(t, runner.Left, "and no counts")
}
