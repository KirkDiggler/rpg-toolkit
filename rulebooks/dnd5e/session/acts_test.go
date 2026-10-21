// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// acts_test.go is THE ACT STREAM at the seam (rpg-project#629): real verbs,
// folded through Manager.Acts and the live ActStream, and the one fold both
// share.

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/currency"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// actCapture is an ActStream that keeps what it is handed.
type actCapture struct {
	mu   sync.Mutex
	acts []session.Act
}

func (c *actCapture) PublishActs(_ context.Context, acts []session.Act) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.acts = append(c.acts, acts...)
	return nil
}

// of is everything published to one recipient, in publish order.
func (c *actCapture) of(member string) []session.Act {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []session.Act
	for _, a := range c.acts {
		if a.Recipient == member {
			out = append(out, a)
		}
	}
	return out
}

// actsOf is one member's acts after an id, read back.
func actsOf(t *testing.T, mgr *session.Manager, member, after string) *session.ActsOutput {
	t.Helper()
	out, err := mgr.Acts(context.Background(), &session.ActsInput{Session: "sess", Member: member, AfterID: after})
	require.NoError(t, err)
	return out
}

// TestCatchUpEqualsLive is #541 R2 for the act stream: a fighter's round —
// her end of turn, a skeleton's walk out of her reach, the window it opens,
// and the opportunity attack she takes — reaches her live as exactly the
// acts a catch-up folds afterwards, and resuming after ANY segment's id gives
// exactly the live suffix after it.
//
// THE INVARIANT IT GUARDS is the one the fold depends on: a segment never
// spans a commit. Every act that outlives a call is paused by a window whose
// own beat closes the segment, so a live fold of one commit and a catch-up
// fold of the whole story cut the same runs. A verb that continued an act
// across calls without a beat would fail here.
func (s *ReactWindowSuite) TestCatchUpEqualsLive() {
	live := &actCapture{}
	mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: live, Seats: newFakeSeats(), Explorations: newFakeExplorations(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{},
		TurnDriver: &pathWalker{paths: map[string][]spatial.Position{"skel-1": {hexCell(5, 0), hexCell(6, 0)}}, walked: map[string]bool{}},
		Sessions:   s.sessions, Encounters: s.encounters, Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.start(mgr, []sceneSeat{seatAt("fighter", 3, 0)}, reactSkeleton("skel-1", 4, 0))
	s.endTurn(mgr, "fighter")
	s.react(mgr, "fighter", session.Take(""))

	for _, member := range []string{"fighter", "skel-1"} {
		published := live.of(member)
		s.Require().NotEmpty(published, member)

		caught := actsOf(s.T(), mgr, member, "")
		s.Require().False(caught.Restarted, "the round fits the retention window, or this proves nothing")
		s.Equal(published, caught.Acts, "%s: live and catch-up fold the same acts", member)

		for i, act := range published {
			last := i
			for j := i; j < len(published); j++ {
				if published[j].ID == act.ID {
					last = j
				}
			}
			want := published[last+1:]
			got := actsOf(s.T(), mgr, member, act.ID).Acts
			if len(want) == 0 {
				s.Empty(got, "%s after %s", member, act.ID)
				continue
			}
			s.Equal(want, got, "%s after %s", member, act.ID)
		}
	}

	// And the round folded as a reader would tell it: the window the walk
	// opened, the fighter's swing as a reaction, then the walk itself — all
	// under the walk's one id, in that order.
	var walk string
	for _, act := range live.of("fighter") {
		if body, ok := act.Body.(session.WalkAct); ok && body.Mover.ID == "skel-1" {
			walk = act.Act
		}
	}
	s.Require().True(strings.HasPrefix(walk, "move:"), "the skeleton's walk is a move act, got %q", walk)
	var told []string
	for _, act := range live.of("fighter") {
		if act.Act != walk {
			continue
		}
		switch body := act.Body.(type) {
		case session.ClockEntry:
			told = append(told, string(body.Mark))
		case session.AttackAct:
			s.Require().NotNil(body.Reaction)
			told = append(told, string(act.Family))
		case session.WalkAct:
			told = append(told, "walk")
		}
	}
	s.Equal([]string{string(session.ClockWindowOpened), string(session.FamilyReaction), "walk"}, told)
}

// TestFoldLethalAttack is Law 6 and the state rule: a lethal swing is one
// hit carrying its own roll and damage, and the fall it caused is a state
// entry under the swing's act id.
func (s *AttackTestSuite) TestFoldLethalAttack() {
	mgr := s.duel(&sequenceDice{rolls: []int{15, 5}})
	s.characters.byID["bob"].HitPoints = 1

	_, err := s.swing(mgr)
	s.Require().NoError(err)

	acts := actsOf(s.T(), mgr, "alice", "").Acts
	var swing, fall *session.Act
	for i := range acts {
		if _, ok := acts[i].Body.(session.AttackAct); ok {
			swing = &acts[i]
			s.Require().Greater(len(acts), i+1, "something follows the swing")
			fall = &acts[i+1]
			break
		}
	}
	s.Require().NotNil(swing, "the swing folded")
	s.True(strings.HasPrefix(swing.Act, "attack:"), "got %q", swing.Act)
	s.Equal(session.FamilyAct, swing.Family)
	hit := swing.Body.(session.AttackAct)
	s.Equal(session.AttackHit, hit.Outcome)
	s.Equal(session.Party{Kind: session.PartyMember, ID: "alice"}, hit.By)
	s.Equal(session.Party{Kind: session.PartyMember, ID: "bob"}, hit.To)
	s.Require().NotNil(hit.Roll)
	s.Require().NotNil(hit.Roll.Beaten)
	s.True(*hit.Roll.Beaten)
	s.NotEmpty(hit.Roll.Expression)
	s.Positive(hit.Damage)

	s.Equal(swing.Act, fall.Act, "the fall is under the swing's act")
	s.Equal(session.FamilyState, fall.Family)
	downed := fall.Body.(session.StateChange)
	s.Equal(session.StateDowned, downed.Change)
	s.Equal(session.Party{Kind: session.PartyMember, ID: "bob"}, downed.Party)
	s.Require().NotNil(downed.Downed)
	s.Equal(string(encounter.KindPlayer), downed.Downed.Kind)
}

// speakingGoblin is the goblin carrying an appeal that lands and says
// "Fine.", and a time table that holds.
func (s *SpeakWorldSuite) speakingGoblin() *session.Manager {
	s.driver = session.Driver()
	s.authored = func(in *dungeonspec.MonsterPlacement) {
		line := appealLine()
		line.Success = []encounter.Answer{{Weight: 1, Say: "Fine."}}
		in.Options = []encounter.Option{line}
		in.Table = encounter.Table{encounter.AnswerTime: {{Weight: 1, Hold: true}}}
	}
	mgr := s.front([]int{10, 1, 1})
	out, err := s.appeal(mgr)
	s.Require().NoError(err)
	s.Require().True(out.Beaten)
	return mgr
}

// TestFoldSpeakFoldsAnswer is Law 6 for a line: the line and its answer are
// one speak act, the check inside it.
func (s *SpeakWorldSuite) TestFoldSpeakFoldsAnswer() {
	mgr := s.speakingGoblin()

	var lines []session.Act
	for _, act := range actsOf(s.T(), mgr, "alice", "").Acts {
		if _, ok := act.Body.(session.SpeakAct); ok {
			lines = append(lines, act)
		}
	}
	s.Require().Len(lines, 1)
	s.True(strings.HasPrefix(lines[0].Act, "speak:"), "got %q", lines[0].Act)
	line := lines[0].Body.(session.SpeakAct)
	s.Equal(session.Party{Kind: session.PartyMember, ID: "alice"}, line.By)
	s.Equal(session.Party{Kind: session.PartyMember, ID: "goblin"}, line.To)
	s.Equal("appeal", line.Option)
	s.Require().NotNil(line.Roll)
	s.Require().NotNil(line.Roll.Against)
	s.Require().NotNil(line.Roll.Beaten)
	s.True(*line.Roll.Beaten)
	s.GreaterOrEqual(line.Roll.Total, *line.Roll.Against)
	s.Require().NotNil(line.Answer, "the answer folds into the line")
	s.Equal("Fine.", line.Answer.Say)
	s.Equal(session.Party{Kind: session.PartyMember, ID: "goblin"}, line.Answer.Who)
}

// TestFoldTimePick: the round the line cost the world asks the goblin its
// time table, and that pick is a table answer under a time act — never part
// of the line.
func (s *SpeakWorldSuite) TestFoldTimePick() {
	mgr := s.speakingGoblin()

	var picks []session.Act
	for _, act := range actsOf(s.T(), mgr, "alice", "").Acts {
		if answer, ok := act.Body.(session.TableAnswer); ok && answer.Key == string(encounter.AnswerTime) {
			picks = append(picks, act)
		}
	}
	s.Require().NotEmpty(picks, "the goblin was asked")
	for _, pick := range picks {
		s.Equal(session.FamilyTableAnswer, pick.Family)
		s.True(strings.HasPrefix(pick.Act, "time:"), "a pick is a time act, got %q", pick.Act)
		answer := pick.Body.(session.TableAnswer)
		s.Equal(session.Party{Kind: session.PartyMember, ID: "goblin"}, answer.Who)
		s.Require().NotNil(answer.Roll, "a table with an eligible row rolls")
		s.Positive(answer.Roll.Of, "the table die's size")
		s.Require().Len(answer.Roll.Dice, 1)
		s.Equal(answer.Roll.Dice[0], answer.Roll.Total)
		s.LessOrEqual(answer.Roll.Total, answer.Roll.Of)
		s.Equal("hold", answer.Word)
	}
}

// TestTradingBeatsProduceNoAct is R9 and Law 8: a purchase is in the record
// and in no act.
func (s *TradeTestSuite) TestTradingBeatsProduceNoAct() {
	s.placeVendor(spatial.Position{X: 1, Y: 0})
	price := s.buyLongsword()

	record, err := s.mgr.Record(context.Background(), &session.RecordReadInput{Session: "sess"})
	s.Require().NoError(err)
	var bought string
	for _, e := range record.Events {
		if e.Beat == string(encounter.OutcomeBought) {
			bought = e.ID
		}
	}
	s.Require().NotEmpty(bought, "the record keeps the purchase (%v)", price)

	// The trade's act holds only what the trade cost the clock: the
	// purchase itself is no entry and no part of one.
	for _, act := range actsOf(s.T(), s.mgr, "alice", "").Acts {
		s.NotEqual(bought, act.ID, "no act is the purchase")
		if strings.HasPrefix(act.Act, "trade:") {
			s.Equal(session.FamilyClock, act.Family, "%+v", act)
		}
	}
}

// TestNilActStreamRefused: a manager without an act stream is refused at
// construction, naming the field.
func (s *ManagerTestSuite) TestNilActStreamRefused() {
	_, err := session.NewManager(&session.Config{
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: testCharacters(),
		Seats: newFakeSeats(), Explorations: newFakeExplorations(), Events: session.DiscardEvents{},
		Record: session.DiscardRecord{}, Dice: testDice{}, PresentationIDs: testPresentationIDs{}, TurnDriver: session.Pass{},
	})
	s.ErrorIs(err, session.ErrIncompleteConfig)
	s.Contains(err.Error(), "Acts")
}

// TestActCarriesNoSeq is R30: an act names no record seq — its ids are the
// record's wire id and the act tag. The fields are exactly these; a new one
// must be argued for here.
func TestActCarriesNoSeq(t *testing.T) {
	typ := reflect.TypeOf(session.Act{})
	var names []string
	for i := 0; i < typ.NumField(); i++ {
		names = append(names, typ.Field(i).Name)
	}
	require.Equal(t, []string{"Session", "ID", "Act", "Verb", "Recipient", "At", "Segment", "Family", "Body", "Undecoded"}, names)
}

// TestEveryBeatHasAFamily is R16 failing closed: every beat name the
// encounter exports is folded into a family, or is one of R9's record-only
// beats. A new beat in neither fails here, before it can reach a recipient
// unfolded.
func TestEveryBeatHasAFamily(t *testing.T) {
	kinds := beatKindsOfTheEncounter(t)
	var unmapped []string
	for kind, name := range kinds {
		_, mapped, recordOnly := session.FamilyOfBeatForTest(kind)
		if mapped == recordOnly {
			unmapped = append(unmapped, name+" ("+kind+")")
		}
	}
	sort.Strings(unmapped)
	require.Empty(t, unmapped, "every beat has exactly one family, or is record-only")

	for _, kind := range []string{
		encounter.BeatTransferred, encounter.BeatInteracted,
		string(encounter.OutcomeBought), string(encounter.OutcomeSold),
	} {
		_, _, recordOnly := session.FamilyOfBeatForTest(kind)
		require.True(t, recordOnly, "%s is record-only (R9)", kind)
	}
}

// buyLongsword funds alice and buys the vendor's longsword, as
// TestTradeBuysTheListedItem does.
func (s *TradeTestSuite) buyLongsword() currency.Money {
	price := currency.FromGold(15)
	s.fund(price)
	_, err := s.mgr.Trade(context.Background(), &session.TradeInput{
		Session: "sess", Actor: "alice", Target: "vendor",
		Give: session.TradeOffer{Currency: price},
		Receive: session.TradeOffer{Items: []session.TradeItem{
			{Type: shared.EquipmentTypeWeapon, ID: string(weapons.Longsword), Quantity: 1},
		}},
	})
	s.Require().NoError(err)
	return price
}
