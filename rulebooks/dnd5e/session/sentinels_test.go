// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// sentinels_test.go is the sentinel-shaped sibling of boundary_test.go
// (rpg-toolkit#1058).
//
// The AST boundary test reads exported SIGNATURES, and a sentinel error is not
// a type in a signature — so an inner package can become part of the host's
// contract through a channel that test never looks at. A host that matched on
// encounter.ErrBadPlacement would be coupled to the module this seam exists to
// keep replaceable, exactly as surely as if a struct had leaked, and nothing in
// CI would have said a word.
//
// So every refusal this seam can be DRIVEN INTO is checked here against an
// explicit list of the composition's own sentinels: our vocabulary must be
// present, and none of theirs may be reachable through errors.Is.
//
// Reachability is the discipline that keeps this honest. A case belongs here
// only if a caller or a stored blob can really produce it, which is why the
// list of scenarios below reads like a list of mistakes rather than a list of
// error values. The paths that exist but cannot be reached from a verb are
// pinned one layer down, on translate itself, in translate_internal_test.go.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// compositionSentinels is every error value the composition exports, written
// out rather than derived.
//
// Written out for the same reason boundary_test.go writes out its permitted
// types: a rule like "anything named Err*" would be less typing and would
// quietly cover values nobody weighed, while a list makes every future entry a
// visible line in a diff. When the composition gains a sentinel, it is added
// here — and if that addition turns this test red, a leak has been found rather
// than a chore created.
var compositionSentinels = map[string]error{
	"encounter.ErrNilInput":      encounter.ErrNilInput,
	"encounter.ErrNoMember":      encounter.ErrNoMember,
	"encounter.ErrNotMember":     encounter.ErrNotMember,
	"encounter.ErrNoEnding":      encounter.ErrNoEnding,
	"encounter.ErrClosed":        encounter.ErrClosed,
	"encounter.ErrNoField":       encounter.ErrNoField,
	"encounter.ErrBadPlacement":  encounter.ErrBadPlacement,
	"encounter.ErrBadConnection": encounter.ErrBadConnection,
	"encounter.ErrNoConnection":  encounter.ErrNoConnection,
	"encounter.ErrInBubble":      encounter.ErrInBubble,
	"encounter.ErrNoBubble":      encounter.ErrNoBubble,
	"encounter.ErrBadClock":      encounter.ErrBadClock,
	"encounter.ErrInvalidData":   encounter.ErrInvalidData,
	"encounter.ErrTrimmed":       encounter.ErrTrimmed,
	"encounter.ErrNoInitiative":  encounter.ErrNoInitiative,
}

// SentinelSuite drives each reachable refusal and checks what a host can match
// on afterwards.
type SentinelSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	mgr        *session.Manager
}

func TestSentinelSuite(t *testing.T) { suite.Run(t, new(SentinelSuite)) }

func (s *SentinelSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	// offsetWorld is the fixture with the most map in it: two rooms anchored
	// away from the origin, a doorway between them, and endings on both sides.
	// Anchoring matters here — a cell outside every room is a real coordinate
	// rather than a negative number, which is the mistake a client actually
	// makes.
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: offsetWorld(s.T()),
	})
	s.Require().NoError(err)
}

// refusedInOurVocabulary is the whole assertion of this file, in both
// directions: the sentinel a host is documented to match on is present, and not
// one of the composition's is.
//
// Asserting only the first half is what let this class of leak in. A refusal
// can carry our sentinel and theirs at the same time — errors.Is walks the
// whole chain — so the presence of ours proves nothing at all about what else
// the host can reach.
func (s *SentinelSuite) refusedInOurVocabulary(err error, want error) {
	s.T().Helper()
	s.Require().Error(err)
	s.Require().ErrorIs(err, want, "the host is answered in this package's vocabulary")
	for name, inner := range compositionSentinels {
		s.NotErrorIs(err, inner,
			"a host can reach "+name+" through this refusal, and one that matched on it "+
				"would break the day the composition is replaced (S2)")
	}
}

// TestAWalkOffTheMap is the headline case, and the reason this file exists.
//
// Pathing to a cell no room owns is arithmetic any client can get wrong — not
// corrupt state, not a defect in this package, just a route computed one cell
// too far. It is therefore the leak a real host hits first.
func (s *SentinelSuite) TestAWalkOffTheMap() {
	// alice stands at absolute (41,21); the hall runs (40,20)-(45,25). She
	// steps to its corner, then off the map entirely.
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 40, Y: 20}, {X: 39, Y: 19}},
	})
	s.refusedInOurVocabulary(err, session.ErrBadPosition)
	s.Contains(err.Error(), "39",
		"and the refusal still names the cell that was refused")
}

// TestAnEntryOffTheMap covers both doors into a session with the same mistake.
//
// Join and Spawn share one placement path, so a leak in it is a leak in both;
// asserting through both is what proves the sharing is real rather than
// remembered.
func (s *SentinelSuite) TestAnEntryOffTheMap() {
	nowhere := spatial.Position{X: 900, Y: 900}

	_, joinErr := s.mgr.Join(context.Background(), &session.JoinInput{
		Session: "sess", Member: "bob", Position: nowhere,
	})
	s.refusedInOurVocabulary(joinErr, session.ErrBadPosition)

	_, spawnErr := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(), Position: nowhere,
	})
	s.refusedInOurVocabulary(spawnErr, session.ErrBadPosition)
}

// TestACorruptStoredWorld is the case every verb shares, because every verb
// begins by loading.
//
// A blob whose member stands in a room the field does not declare is exactly
// what a hand-edited or half-migrated store looks like, and the composition
// refuses it in its own vocabulary — several sentinels deep. The host is
// entitled to hear "the stored world is invalid" and nothing else: the leaves
// underneath the composition (clock, intel, record) are replaceable too, and
// they report through the same channel.
func (s *SentinelSuite) TestACorruptStoredWorld() {
	s.encounters.byID["world"].Members[0].Room = "nowhere"
	ctx := context.Background()

	_, err := s.mgr.Status(ctx, &session.StatusInput{Session: "sess"})
	s.refusedInOurVocabulary(err, session.ErrInvalidWorld)

	_, err = s.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "alice"})
	s.refusedInOurVocabulary(err, session.ErrInvalidWorld)

	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 42, Y: 22}},
	})
	s.refusedInOurVocabulary(err, session.ErrInvalidWorld)

	s.Contains(err.Error(), "nowhere",
		"and the refusal still names the room the blob invented")
}

// TestAWorldThatWillNotLoad is the corrupt blob's twin at the OTHER door: the
// same refusal, reached before anything is stored rather than after.
//
// A host authoring a world gets it wrong long before a store corrupts one, so
// this is the first refusal most integrations meet — and the composition
// answers it with several sentinels stacked, which is several ways to couple a
// host to a module we intend to replace.
func (s *SentinelSuite) TestAWorldThatWillNotLoad() {
	broken := offsetWorld(s.T())
	broken.Members[0].Room = "nowhere"

	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "another", Encounter: "another-world", World: broken,
	})
	s.refusedInOurVocabulary(err, session.ErrInvalidWorld)
	s.Contains(err.Error(), "nowhere",
		"and the refusal still names the room the world invented")
}

// TestATrimmedStory is the case that already held, kept here so the list of
// refusals is the list of refusals rather than a list of bugs once fixed.
func (s *SentinelSuite) TestATrimmedStory() {
	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "trimmed", Encounter: "trimmed-world", World: trimmedWorld(s.T()),
	})
	s.Require().NoError(err)

	_, err = s.mgr.Story(context.Background(),
		&session.StoryInput{Session: "trimmed", Member: "alice", FromSeq: 1})
	s.refusedInOurVocabulary(err, session.ErrStoryTrimmed)
}

// TestAMemberTheEncounterDoesNotHave covers the reads that ask the composition
// about somebody by name.
func (s *SentinelSuite) TestAMemberTheEncounterDoesNotHave() {
	ctx := context.Background()

	_, err := s.mgr.View(ctx, &session.ViewInput{Session: "sess", Member: "nobody"})
	s.refusedInOurVocabulary(err, session.ErrNoMember)

	_, err = s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "nobody"})
	s.refusedInOurVocabulary(err, session.ErrNoMember)

	_, err = s.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "nobody"})
	s.refusedInOurVocabulary(err, session.ErrNoMember)
}

// TestAnEndingThatWasNeverDeclared and TestAClosedEncounter cover the write
// verbs' own refusals, which already route through translate — regression
// guards rather than new ground.
func (s *SentinelSuite) TestAnEndingThatWasNeverDeclared() {
	_, err := s.mgr.End(context.Background(), &session.EndInput{
		Session: "sess", Ending: "never-declared",
	})
	s.refusedInOurVocabulary(err, session.ErrNoEnding)
}

func (s *SentinelSuite) TestAClosedEncounter() {
	ctx := context.Background()

	// Walked onto rather than declared: offsetWorld's endings both fire on a
	// position, so the encounter is closed the way the game closes it.
	out, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{{X: 42, Y: 21}, {X: 43, Y: 21}, {X: 44, Y: 21}},
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Outcome, "the walk ended the encounter")

	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{{X: 44, Y: 22}},
	})
	s.refusedInOurVocabulary(err, session.ErrClosed)
}

// TestEndingATurnWhileFreeRoaming is the one refusal whose translation changes
// the WORD as well as the package — the composition's "no bubble" is this
// seam's "not in a fight" — so it also pins that translate is a mapping rather
// than a rewrap.
func (s *SentinelSuite) TestEndingATurnWhileFreeRoaming() {
	_, err := s.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: "sess", Member: "alice",
	})
	s.refusedInOurVocabulary(err, session.ErrNotInFight)
}
