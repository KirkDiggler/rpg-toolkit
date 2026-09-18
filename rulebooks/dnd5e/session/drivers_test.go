// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// perSessionDrivers is the host side of the seam, written as small as
// rpg-api's own: one driver per session id, built on first sight, never
// evicted (rule A6 — the cache is the host's because the session's lifetime
// is).
//
// errFor is how the fail-closed proof arms a refusal on a live Manager: a host
// that cannot say which driver serves a session has a wiring fault, and this
// is the shape of one.
//
// asks COUNTS, and built does not, which is the difference the counter exists
// for. built is keyed by session id, so a verb that resolved twice would land
// in the same entry and look identical to one that resolved once. The promise
// [TurnDriverSource] makes is that the source is asked ONCE per verb and that
// answer serves the whole verb — the thing that stops two capabilities in one
// call from holding two different brains — and only a count can hold that up.
// It matters most for the host the doc itself names: one that mints per ask,
// from per-session material fetched fresh, where a second ask is a second
// driver rather than a wasted map lookup.
type perSessionDrivers struct {
	built  map[string]*mindedPerSession
	asks   map[string]int
	errFor error
	nilFor bool
}

func newPerSessionDrivers() *perSessionDrivers {
	return &perSessionDrivers{built: map[string]*mindedPerSession{}, asks: map[string]int{}}
}

// DriverFor hands session sessionID its own driver, building one the first
// time that session is seen.
func (s *perSessionDrivers) DriverFor(_ context.Context, sessionID string) (session.TurnDriver, error) {
	// Counted BEFORE any refusal: an ask that failed is still an ask, and the
	// fail-closed proof reads this to show the verb asked once and stopped
	// there rather than retrying.
	s.asks[sessionID]++

	if s.errFor != nil {
		return nil, s.errFor
	}
	if s.nilFor {
		return nil, nil
	}
	if driver, ok := s.built[sessionID]; ok {
		return driver, nil
	}

	// The driver underneath is deliberately the boring one. What this seam
	// promises is about WHICH driver a verb gets and how often it asks, not
	// about what that driver decides — and the stateful mind this was written
	// against is deleted (rpg-project#465). The per-session memory the test
	// reads is [mindedPerSession.asked], which was always the observable.
	driver := &mindedPerSession{driver: session.Pass{}, asked: map[string]int{}}
	s.built[sessionID] = driver
	return driver, nil
}

// mindedPerSession is one session's own driver: a note of which members THIS
// driver has been asked about, over a driver that answers.
//
// The note IS the per-session memory under test. It was written to stand in
// for a stateful mind's own memory, which was never observable from out here;
// that mind is gone and the note is what remains, at the same granularity and
// holding the same fact: this driver has met this member before. That fact is
// exactly what leaked when one driver served every session, because member ids
// are authored per dungeon rather than minted per run.
type mindedPerSession struct {
	driver session.TurnDriver
	asked  map[string]int
}

func (d *mindedPerSession) Act(view session.MonsterView) (session.TurnIntent, error) {
	d.asked[view.Self]++
	return d.driver.Act(view)
}

// knows reports whether this driver has ever been asked about a member.
func (d *mindedPerSession) knows(member string) bool {
	_, met := d.asked[member]
	return met
}

// driven is every member this driver has ever taken a turn for.
func driven(d *mindedPerSession) []string {
	members := make([]string, 0, len(d.asked))
	for member := range d.asked {
		members = append(members, member)
	}
	return members
}

// driverScene builds ONE Manager over one host source. Two fighters exist so
// two sessions can each hold a player of their own; the skeletons deliberately
// do not, and that is the point of the test below.
func driverScene(t *testing.T, source session.TurnDriverSource) *session.Manager {
	t.Helper()

	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDrivers: source,
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: newFakeCharacters(armedFighter("fighter-a"), armedFighter("fighter-b")),
		Events:     session.DiscardEvents{},
	})
	require.NoError(t, err)
	return mgr
}

// startDungeon runs one party into one copy of the same authored tomb.
func startDungeon(t *testing.T, mgr *session.Manager, sessionID, fighter string, skeletons ...string) {
	t.Helper()

	ctx := context.Background()
	_, err := mgr.StartSession(ctx, &session.StartSessionInput{
		Session: sessionID, Encounter: sessionID + "-world", World: tombRoom(40, 6),
	})
	require.NoError(t, err)
	_, err = mgr.Join(ctx, &session.JoinInput{
		Session: sessionID, Member: fighter, Position: spatial.Position{X: 0, Y: 0},
	})
	require.NoError(t, err)

	for i, id := range skeletons {
		_, err = mgr.Spawn(ctx, &session.SpawnInput{
			Session: sessionID, ID: id, Ref: refs.Monsters.Skeleton().String(),
			Position: spatial.Position{X: float64(i + 1), Y: 0},
		})
		require.NoError(t, err)
	}
}

// endTurn hands the clock to whatever the session's driver has to answer for,
// and counts what that one verb cost the host's source.
//
// THE SELECTOR IS FETCHED FIRST, OUTSIDE THE COUNT, on purpose: Afford is a
// read verb and asks once itself, so folding it in would measure two verbs and
// prove neither. The window is the EndTurn alone.
func endTurn(t *testing.T, mgr *session.Manager, source *perSessionDrivers, sessionID, member string) {
	t.Helper()

	declarationID := currentEndTurnID(t, mgr, sessionID, member)

	before := source.asks[sessionID]
	_, err := mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: sessionID, Member: member, DeclarationID: declarationID,
	})
	require.NoError(t, err)
	require.Equal(t, before+1, source.asks[sessionID],
		"one write verb, one ask: the driver is resolved once and carried on the scope")
}

// Two sessions under one Manager get two drivers, and what one of them
// recorded is unknown to the other.
//
// THE CONTROL IS THE SHARED MEMBER ID. Both parties run the same authored
// tomb, so both hold a skeleton called skel-1 — member ids are authored per
// dungeon, not minted per run, which is precisely why one process-wide driver
// gave two tables one skeleton's assigned mind and names (rpg-api#980's
// caveat). skel-2 is the second session's stranger: a member the first
// session's driver has already driven a turn for, and the second's has never
// heard of.
//
// The driver each session gets is a real [session.Minded] — the stateful value
// this whole seam exists for — so what is isolated here is the thing that
// actually holds state, not a stand-in that behaves like it.
func TestEachSessionDrivesItsOwnTurnsWithItsOwnDriver(t *testing.T) {
	source := newPerSessionDrivers()
	mgr := driverScene(t, source)

	startDungeon(t, mgr, "sess-a", "fighter-a", "skel-1", "skel-2")
	startDungeon(t, mgr, "sess-b", "fighter-b", "skel-1")

	endTurn(t, mgr, source, "sess-a", "fighter-a")
	endTurn(t, mgr, source, "sess-b", "fighter-b")

	require.Len(t, source.built, 2, "the host was asked about two sessions and built two drivers")
	first, second := source.built["sess-a"], source.built["sess-b"]
	require.NotNil(t, first)
	require.NotNil(t, second)
	require.NotSame(t, first, second, "and they are two values, not one answer given twice")

	require.True(t, first.knows("skel-1"), "the first session's driver took its own skeleton's turn")
	require.True(t, second.knows("skel-1"), "so did the second's, for the member of the same name")

	// The SETS, not the counts: how many questions one turn costs a driver is
	// the composition's business and changes when the turn loop does. Who each
	// driver has ever been asked about is this seam's.
	require.ElementsMatch(t, []string{"skel-1", "skel-2"}, driven(first),
		"the first session's driver drove that session's members")
	require.ElementsMatch(t, []string{"skel-1"}, driven(second),
		"and the second's drove only its own: skel-2 is a name recorded in one session and unknown in the other")
}

// A source that cannot name a session's driver fails the verb, and nothing
// takes a turn.
//
// FAIL CLOSED, NEVER FALL BACK. The reference driver is right there and would
// answer this turn perfectly plausibly — which is the whole danger: a monster
// driven by somebody else's brain looks like a design choice, and the wiring
// fault that caused it is never reported anywhere.
//
// The verb is proved to have changed NOTHING, not merely to have returned an
// error: the fighter's own turn is still the current one afterwards.
func TestAResolverErrorFailsTheVerbAndDrivesNoTurn(t *testing.T) {
	boom := errors.New("the host cannot say which driver serves this session")

	source := newPerSessionDrivers()
	mgr := driverScene(t, source)
	startDungeon(t, mgr, "sess", "fighter-a", "skel-1")

	before := currentEndTurnID(t, mgr, "sess", "fighter-a")
	driver := source.built["sess"]
	require.NotNil(t, driver)
	require.Empty(t, driver.asked, "nothing has been driven yet")

	source.errFor = boom

	askedBeforeWrite := source.asks["sess"]
	_, err := mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "fighter-a", DeclarationID: before,
	})
	require.ErrorIs(t, err, boom, "the host's own error reaches the host, wrapped")
	require.Equal(t, askedBeforeWrite+1, source.asks["sess"],
		"it asked once and stopped there: a refused verb neither retries nor asks a second time")

	// A read is refused by the same door, for the same reason: a world this
	// package builds never holds a brain that belongs to somebody else.
	askedBeforeRefusedRead := source.asks["sess"]
	_, readErr := mgr.Roster(context.Background(), &session.RosterInput{Session: "sess", Player: "player-fighter-a"})
	require.ErrorIs(t, readErr, boom)
	require.Equal(t, askedBeforeRefusedRead+1, source.asks["sess"], "and a refused read asks once too")

	source.errFor = nil

	// ONE READ VERB, ONE ASK, on the path that succeeds. The two counts above
	// are taken where resolution FAILS, which is the cheap half: an error
	// returns before anything downstream could ask again. This is the half the
	// promise is actually about — the read path resolves beside the world and
	// carries that answer through the whole verb.
	askedBeforeRead := source.asks["sess"]
	_, err = mgr.Roster(context.Background(), &session.RosterInput{Session: "sess", Player: "player-fighter-a"})
	require.NoError(t, err)
	require.Equal(t, askedBeforeRead+1, source.asks["sess"],
		"one read verb, one ask: the driver is resolved next to the world and carried")

	require.Empty(t, driver.asked, "no turn was driven")
	require.Equal(t, before, currentEndTurnID(t, mgr, "sess", "fighter-a"),
		"and the fighter's turn is still the current one: the verb wrote nothing")
}

// A source that reports success and hands over no driver has broken its
// contract, and is refused where it happens.
//
// The alternative is a nil inside the seam that translates a driver's answer,
// which panics several frames down in the middle of somebody's turn — the
// failure S8's total construction prevents at NewManager, and which has to be
// prevented again now that a driver arrives per verb.
func TestASourceThatHandsOverNoDriverIsRefused(t *testing.T) {
	source := newPerSessionDrivers()
	mgr := driverScene(t, source)
	startDungeon(t, mgr, "sess", "fighter-a", "skel-1")

	source.nilFor = true
	_, err := mgr.Roster(context.Background(), &session.RosterInput{Session: "sess", Player: "player-fighter-a"})
	require.ErrorIs(t, err, session.ErrNoTurnDriver)
	require.Contains(t, err.Error(), "sess", "and it names the session nobody could name a driver for")
}

// Exactly one of the two driver fields is wired, and NewManager says so both
// ways.
//
// Neither is the S8 refusal every other required capability gets, and it names
// BOTH fields: a host that meant to wire the per-session source is not helped
// by an error naming only the other one. Both is its own refusal rather than a
// precedence rule, because picking either silently would leave a host watching
// the driver it did not mean to wire take every turn in the process.
func TestExactlyOneTurnDriverIsWired(t *testing.T) {
	base := func() *session.Config {
		return &session.Config{
			PresentationIDs: testPresentationIDs{}, Dice: testDice{},
			Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
			Characters: newFakeCharacters(armedFighter("fighter-a")), Events: session.DiscardEvents{},
		}
	}

	t.Run("neither", func(t *testing.T) {
		mgr, err := session.NewManager(base())
		require.ErrorIs(t, err, session.ErrIncompleteConfig)
		require.Contains(t, err.Error(), "TurnDriver or TurnDrivers",
			"the refusal names both doors, because either one satisfies it")
		require.Nil(t, mgr)
	})

	t.Run("both", func(t *testing.T) {
		cfg := base()
		cfg.TurnDriver = session.Pass{}
		cfg.TurnDrivers = newPerSessionDrivers()

		mgr, err := session.NewManager(cfg)
		require.ErrorIs(t, err, session.ErrAmbiguousConfig)
		require.NotErrorIs(t, err, session.ErrIncompleteConfig,
			"two answers is not a missing one, and the fix is to delete a line rather than add one")
		require.Contains(t, err.Error(), "TurnDriver and TurnDrivers")
		require.Nil(t, mgr)
	})

	t.Run("the stateless driver alone", func(t *testing.T) {
		cfg := base()
		cfg.TurnDriver = session.Pass{}

		mgr, err := session.NewManager(cfg)
		require.NoError(t, err)
		require.NotNil(t, mgr)
	})

	t.Run("the per-session source alone", func(t *testing.T) {
		cfg := base()
		cfg.TurnDrivers = newPerSessionDrivers()

		mgr, err := session.NewManager(cfg)
		require.NoError(t, err)
		require.NotNil(t, mgr)
	})
}
