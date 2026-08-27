// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// move_turnclock_test.go pins rpg-toolkit#1169: Move on the turn clock. The
// active member of a fight may still walk — priced at 5 feet per cell,
// refused whole before any step, and gated to whoever the clock is waiting
// on — where before this SDK refused every fight member's Move outright.
//
// The fixture is aFight's own shape (economy_test.go), widened to a second
// player: alice AND bob start in the hall, a skeleton spawns adjacent and
// pulls all three into one bubble, initiative order [alice, bob, skel-1].
// TWO REAL PLAYERS is what makes "not alice's turn" an observable state at
// all — a lone player fighting a Pass-driven monster cycles straight back
// through EndTurn in one call (see afford_test.go's own nextTurn), so there
// is no window in which a caller could ask a sole player to move out of
// turn. Bob's own turn, between alice's and the Pass-driven skeleton's,
// is that window.
type MoveTurnClockSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	stream     *fakeStream
	mgr        *session.Manager
}

func TestMoveTurnClockSuite(t *testing.T) { suite.Run(t, new(MoveTurnClockSuite)) }

func (s *MoveTurnClockSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(armedFighter("alice"), armedFighter("bob"))
	s.stream = &fakeStream{}

	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	enc, err := encounter.NewEncounter(&encounter.SetupInput{Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)}},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 5}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: &data})
	s.Require().NoError(err)

	spawned, err := mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 2, Y: 1},
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "arriving in plain sight must start a fight")

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockTurn, turn.Clock)
	s.Require().Equal("alice", turn.Active, "alice is first registered — first in initiative")
}

func (s *MoveTurnClockSuite) afford(member string) *session.AffordOutput {
	s.T().Helper()
	out, err := s.mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return out
}

func (s *MoveTurnClockSuite) endTurn(ctx context.Context, member string) (*session.EndTurnOutput, error) {
	s.T().Helper()
	return s.mgr.EndTurn(ctx, &session.EndTurnInput{
		Session: "sess", Member: member,
		DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", member),
	})
}

// moveDecl finds the VerbMove declaration, failing loudly if Afford ever
// stops carrying one — a silent absence would read as "nothing to report"
// rather than as the regression it is.
func (s *MoveTurnClockSuite) moveDecl(out *session.AffordOutput) session.Declaration {
	s.T().Helper()
	for _, d := range out.Declarations {
		if d.Verb == session.VerbMove {
			return d
		}
	}
	s.Require().Fail("no VerbMove declaration", "declarations: %+v", out.Declarations)
	return session.Declaration{}
}

func (s *MoveTurnClockSuite) storedMovement(member string) int {
	s.T().Helper()
	stored, ok := s.characters.byID[member]
	s.Require().True(ok, "%s must be in the repository", member)
	s.Require().NotNil(stored.ActionEconomy, "%s's sheet was never readied", member)
	return stored.ActionEconomy.MovementRemaining
}

func (s *MoveTurnClockSuite) where(member string) spatial.Position {
	s.T().Helper()
	out, err := s.mgr.Where(context.Background(), &session.WhereInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return out.Position
}

// TestActiveMemberSpendsMovementAndAffordSeesIt is the brief's own opening
// case: a 3-cell walk, on the turn clock, by the active member.
func (s *MoveTurnClockSuite) TestActiveMemberSpendsMovementAndAffordSeesIt() {
	out, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "alice"),
		Path: []spatial.Position{{X: 1, Y: 2}, {X: 1, Y: 3}, {X: 1, Y: 4}},
	})
	s.Require().NoError(err, "alice is active, and 3 cells cost 15 of her 30 feet")
	s.Require().Len(out.Steps, 3)
	s.Equal(spatial.Position{X: 1, Y: 4}, out.Steps[2].Position)

	// The sheet's own spend is durable, not merely reported.
	s.Equal(15, s.storedMovement("alice"), "30 speed - 15 spent = 15 left, persisted")

	// MOVED reached the stream — the walk is a real one, not a silent charge.
	var moved int
	for _, ev := range s.stream.published {
		if ev.Kind == session.EventMoved && ev.Recipient == "alice" {
			moved++
		}
	}
	s.Equal(3, moved, "one MOVED per cell entered, delivered to alice's own stream")

	// Afford shows the same number the sheet now holds — never a second copy
	// of the arithmetic.
	decl := s.moveDecl(s.afford("alice"))
	s.Equal(session.VerbMove, decl.Verb)
	s.True(decl.Available, "15 feet is still enough for at least one more cell")
	s.Require().NotNil(decl.Remaining, "Move's declaration always carries Remaining")
	s.Equal(15, *decl.Remaining)
	s.Equal(session.SlotNone, decl.Slot, "movement is capacity, never a per-turn slot")
}

// TestOverBudgetWalkIsRefusedWholeAndNothingIsSaved is the brief's second
// case: a walk longer than what is left is refused BEFORE any step, naming
// the currency, and the sheet it would have spent from is untouched.
func (s *MoveTurnClockSuite) TestOverBudgetWalkIsRefusedWholeAndNothingIsSaved() {
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "alice"),
		Path: []spatial.Position{{X: 1, Y: 2}, {X: 1, Y: 3}, {X: 1, Y: 4}},
	})
	s.Require().NoError(err, "spends 15, leaves 15")

	_, err = s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice", DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "alice"),
		Path: []spatial.Position{{X: 1, Y: 5}, {X: 1, Y: 6}, {X: 1, Y: 7}, {X: 1, Y: 8}},
	})
	s.Require().Error(err, "20 feet needed, 15 left")
	s.ErrorIs(err, session.ErrCannotAfford)
	s.Contains(err.Error(), "movement: 20 ft needed, 15 ft left",
		"the exact currency text the brief asks for, not a paraphrase of it")

	// Nothing moved: not the position, not the budget.
	s.Equal(spatial.Position{X: 1, Y: 4}, s.where("alice"), "the refused walk left her exactly where she was")
	s.Equal(15, s.storedMovement("alice"), "the refused spend never reached the sheet")
}

// TestEndTurnRefreshesMovementForTheNextRound is the brief's third case.
// alice ends her turn, bob — a real player, not Pass-driven — takes his and
// ends it too, and the round comes back around to alice with her movement
// seeded fresh from speed rather than left at whatever her first turn spent.
func (s *MoveTurnClockSuite) TestEndTurnRefreshesMovementForTheNextRound() {
	ctx := context.Background()

	_, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "alice"),
		Path: []spatial.Position{{X: 1, Y: 2}, {X: 1, Y: 3}, {X: 1, Y: 4}},
	})
	s.Require().NoError(err, "spends 15 of alice's 30")

	ended, err := s.endTurn(ctx, "alice")
	s.Require().NoError(err)
	s.Equal("bob", ended.Next, "a real player does not auto-resolve — the order simply advances")

	_, err = s.endTurn(ctx, "bob")
	s.Require().NoError(err, "bob's own turn, spending nothing, still ends cleanly")

	turn, err := s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal("alice", turn.Active, "the order wrapped back to her")
	s.Equal(2, turn.Round)

	decl := s.moveDecl(s.afford("alice"))
	s.True(decl.Available)
	s.Require().NotNil(decl.Remaining)
	s.Equal(30, *decl.Remaining, "a new turn re-seeds MovementRemaining from speed, not from where it was left")
}

// TestNotActiveMoveIsRefusedWithTheNamedSentinel is the brief's fourth case:
// during bob's turn, alice — still in the very same fight — may not walk.
// The refusal is the named sentinel, not the leaf play/clock one it is
// translated from (rpg-toolkit#1169's whole S2 point).
func (s *MoveTurnClockSuite) TestNotActiveMoveIsRefusedWithTheNamedSentinel() {
	ctx := context.Background()

	ended, err := s.endTurn(ctx, "alice")
	s.Require().NoError(err)
	s.Require().Equal("bob", ended.Next, "control: it is now genuinely bob's turn")

	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 1, Y: 2}},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotYourTurn)

	// Nothing moved: a refusal this early must not have moved her at all.
	s.Equal(spatial.Position{X: 1, Y: 1}, s.where("alice"))
}

// TestNotActiveWinsOverAffordability is the precedence Copilot caught on
// #1171: a member who is both out of turn AND asking for an unaffordable
// path must be told the TRUE reason, not the currency one Pay would also
// have refused with. Move asks the clock before it ever loads a sheet or
// prices anything, so the sheet is never touched at all — asserted directly
// against the fake repository's own call count, not inferred from the error
// alone.
func (s *MoveTurnClockSuite) TestNotActiveWinsOverAffordability() {
	ctx := context.Background()

	ended, err := s.endTurn(ctx, "alice")
	s.Require().NoError(err)
	s.Require().Equal("bob", ended.Next, "control: it is now genuinely bob's turn")

	before := s.characters.asked["alice"]

	// A path far longer than alice's whole 30-foot speed — if the turn gate
	// were checked AFTER pricing, or not at all, this would be refused with
	// ErrCannotAfford naming a currency shortfall instead.
	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{
			{X: 1, Y: 2}, {X: 1, Y: 3}, {X: 1, Y: 4}, {X: 1, Y: 5},
			{X: 1, Y: 6}, {X: 1, Y: 7}, {X: 1, Y: 8},
		},
	})
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrNotYourTurn)
	s.NotErrorIs(err, session.ErrCannotAfford, "the wrong reason must not also be true of this refusal")

	s.Equal(before, s.characters.asked["alice"],
		"the clock is asked before the sheet is — a refusal this early must never have loaded it")
}

// TestTurnClockMoveRequiresCurrentSelector pins both halves of the Move
// contract: a turn-clock walk cannot omit its current offer, while a selector
// that belonged to the fight stays stale after the fight dissolves and must
// never become permission for a free world-clock move.
func (s *MoveTurnClockSuite) TestTurnClockMoveRequiresCurrentSelector() {
	ctx := context.Background()
	path := []spatial.Position{{X: 1, Y: 2}}

	out, err := s.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: path})
	s.ErrorIs(err, session.ErrNoDeclarationID)
	s.Nil(out)
	s.Equal(spatial.Position{X: 1, Y: 1}, s.where("alice"))

	selector := s.moveDecl(s.afford("alice")).ID
	_, err = s.mgr.Dissolve(ctx, &session.DissolveInput{
		Session: "sess", Member: "alice", Cause: session.ByDecision(),
	})
	s.Require().NoError(err)

	out, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: path, DeclarationID: selector,
	})
	s.ErrorIs(err, session.ErrStaleDeclaration)
	s.Nil(out)
	s.Equal(spatial.Position{X: 1, Y: 1}, s.where("alice"), "the stale turn selector moved no cell")
}

// TestWorldClockMoveNeverTouchesTheEconomy is the brief's fifth case: free
// roam is untouched. Reuses corridorWorld/MoveTestSuite's own fixture (alice
// alone, never in a fight) rather than a fixture of this suite's own, so the
// claim is checked against the SAME scene the walking-a-path tests already
// exercise.
func (s *MoveTurnClockSuite) TestWorldClockMoveNeverTouchesTheEconomy() {
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	characters := testCharacters()
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	ctx := context.Background()
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: corridorWorld(s.T()),
	})
	s.Require().NoError(err)

	turn, err := mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Equal(session.ClockWorld, turn.Clock, "control: free roam, not a fight")

	out, err := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 2, Y: 1}, {X: 3, Y: 1}},
	})
	s.Require().NoError(err)
	s.Require().Len(out.Steps, 2)

	stored, ok := characters.byID["alice"]
	s.Require().True(ok)
	s.Nil(stored.ActionEconomy,
		"a free-roam walk must never ready, let alone spend from, a sheet — world Move skips offer compilation")

	direct, err := mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Equal(session.ClockWorld, direct.Clock)
	s.Empty(direct.Declarations, "empty, not zero — the economy does not apply on the world clock at all")
}

// TestMovedTurnEndedAndFightStartedCarryTypedBodies pins rpg-toolkit#941 for
// the three beat families this fixture's own setup and Move already produce:
// the spawn that pulled everyone into one bubble (FightStartedBody, in
// initiative order), a step of the walk (MovedBody), and ending alice's turn
// (TurnEndedBody). Bob — a real player, next in initiative — is NOT
// Pass-driven through automatically the way skel-1 would be, so this is
// exactly one turn-ended BEAT, delivered once per recipient (three events,
// one beat).
func (s *MoveTurnClockSuite) TestMovedTurnEndedAndFightStartedCarryTypedBodies() {
	ctx := context.Background()

	started := s.eventsOfKind(session.EventFightStarted)
	s.Require().NotEmpty(started, "the spawn in SetupTest already formed the bubble")
	fsBody, ok := started[0].Body.(session.FightStartedBody)
	s.Require().True(ok, "got %T", started[0].Body)
	s.Equal([]string{"alice", "bob", "skel-1"}, fsBody.Members)

	_, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 1, Y: 2}},
		DeclarationID: currentMoveID(s.T(), s.mgr, "sess", "alice"),
	})
	s.Require().NoError(err)

	moved := s.eventsOfKind(session.EventMoved)
	s.Require().NotEmpty(moved)
	movedBody, ok := moved[0].Body.(session.MovedBody)
	s.Require().True(ok, "got %T", moved[0].Body)
	s.Equal("alice", movedBody.Member)
	s.Equal(spatial.Position{X: 1, Y: 2}, movedBody.To)

	ended, err := s.endTurn(ctx, "alice")
	s.Require().NoError(err)
	s.Equal("bob", ended.Next)

	turnEnded := s.eventsOfKind(session.EventTurnEnded)
	s.Require().Len(turnEnded, 3, "one beat, delivered once per member of the encounter")
	first, ok := turnEnded[0].Body.(session.TurnEndedBody)
	s.Require().True(ok, "got %T", turnEnded[0].Body)
	s.Equal("alice", first.Member)
	s.Equal("bob", first.Next)
}

// eventsOfKind collects everything of one kind published across this
// suite's stream — MoveTurnClockSuite's own copy of DeathTestSuite's helper,
// since the two suites do not share a base.
func (s *MoveTurnClockSuite) eventsOfKind(kind session.EventKind) []session.Event {
	var out []session.Event
	for _, event := range s.stream.published {
		if event.Kind == kind {
			out = append(out, event)
		}
	}
	return out
}
