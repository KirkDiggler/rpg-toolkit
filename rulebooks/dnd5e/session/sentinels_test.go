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
// explicit list of the sentinels the modules underneath export: our vocabulary
// must be present, and none of theirs may be reachable through errors.Is.
//
// THE COMPOSITION IS NOT THE ONLY MODULE UNDERNEATH. This file was scoped to
// encounter's sentinels when it was written, and said so — which left the same
// leak open one module over for a swing, and one more for a ref
// (rpg-toolkit#1066). The lists below are now every module a verb can be
// refused BY, because a list that covers one of them reads like a guarantee and
// is not one.
//
// Reachability is the discipline that keeps this honest. A case belongs here
// only if a caller or a stored blob can really produce it, which is why the
// list of scenarios below reads like a list of mistakes rather than a list of
// error values. The paths that exist but cannot be reached from a verb are
// pinned one layer down, on the translations themselves, in
// translate_internal_test.go.

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
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
	"encounter.ErrNilInput":     encounter.ErrNilInput,
	"encounter.ErrNoMember":     encounter.ErrNoMember,
	"encounter.ErrNotMember":    encounter.ErrNotMember,
	"encounter.ErrNoEnding":     encounter.ErrNoEnding,
	"encounter.ErrClosed":       encounter.ErrClosed,
	"encounter.ErrNoField":      encounter.ErrNoField,
	"encounter.ErrBadPlacement": encounter.ErrBadPlacement,
	// The field's construction refusals (rpg-project#256): a world that
	// trips one fails to LOAD, and a load failure reaches a host as
	// ErrInvalidWorld with the reason as text, never as one of these.
	"encounter.ErrRegionEmpty":            encounter.ErrRegionEmpty,
	"encounter.ErrRegionOverlap":          encounter.ErrRegionOverlap,
	"encounter.ErrRegionArchetypeMissing": encounter.ErrRegionArchetypeMissing,
	"encounter.ErrRegionLightingMissing":  encounter.ErrRegionLightingMissing,
	"encounter.ErrEdgeNotAdjacent":        encounter.ErrEdgeNotAdjacent,
	"encounter.ErrEdgeOffFloor":           encounter.ErrEdgeOffFloor,
	"encounter.ErrDoorEdgeOffFloor":       encounter.ErrDoorEdgeOffFloor,
	"encounter.ErrNoDoor":                 encounter.ErrNoDoor,
	"encounter.ErrBadDoor":                encounter.ErrBadDoor,
	"encounter.ErrLocked":                 encounter.ErrLocked,
	"encounter.ErrDoorShut":               encounter.ErrDoorShut,
	"encounter.ErrNoRegion":               encounter.ErrNoRegion,
	"encounter.ErrInBubble":               encounter.ErrInBubble,
	"encounter.ErrNoBubble":               encounter.ErrNoBubble,
	"encounter.ErrBadClock":               encounter.ErrBadClock,
	"encounter.ErrInvalidData":            encounter.ErrInvalidData,
	"encounter.ErrTrimmed":                encounter.ErrTrimmed,
	"encounter.ErrNoInitiative":           encounter.ErrNoInitiative,
	"encounter.ErrNoStanding":             encounter.ErrNoStanding,
}

// resolutionSentinels is every error value the resolution module exports.
//
// Resolution is where a swing actually happens, and it is under S2 for the same
// reason the composition is: the day the strike machine is replaced, a host
// that branched on resolution.ErrBadParticipant breaks. translateResolution
// exists to keep that from being possible, and this list is what makes the
// claim mechanical rather than remembered.
//
// Written out whole rather than reduced to the four translateResolution names,
// because the arms are the answer to a question this list asks: an unlisted
// resolution error reaching a host is a leak whether or not anybody wrote an
// arm for it.
var resolutionSentinels = map[string]error{
	"resolution.ErrNilInput":              resolution.ErrNilInput,
	"resolution.ErrNoInitiative":          resolution.ErrNoInitiative,
	"resolution.ErrNoStanding":            resolution.ErrNoStanding,
	"resolution.ErrNoRoller":              resolution.ErrNoRoller,
	"resolution.ErrNoMachine":             resolution.ErrNoMachine,
	"resolution.ErrBadParticipant":        resolution.ErrBadParticipant,
	"resolution.ErrBadStep":               resolution.ErrBadStep,
	"resolution.ErrNoSaver":               resolution.ErrNoSaver,
	"resolution.ErrBadGate":               resolution.ErrBadGate,
	"resolution.ErrBadWorld":              resolution.ErrBadWorld,
	"resolution.ErrBadAttack":             resolution.ErrBadAttack,
	"resolution.ErrNoCombatant":           resolution.ErrNoCombatant,
	"resolution.ErrRecurrenceUnsupported": resolution.ErrRecurrenceUnsupported,
	// The economy's three, added with the slice that made them reachable
	// (rpg-toolkit#1097). ErrCannotPay is the one a PLAYER produces — a second
	// swing in a turn that bought one — and it is driven for real below. The
	// other two are wiring being wrong rather than an actor running out, and
	// they are here for the reason the whole list is written out: an unlisted
	// sentinel reaching a host is a leak whether or not anybody wrote an arm.
	"resolution.ErrCannotPay": resolution.ErrCannotPay,
	"resolution.ErrBadCost":   resolution.ErrBadCost,
	"resolution.ErrNoPayer":   resolution.ErrNoPayer,
	// The activation pair, added with the verb that made them reachable
	// (rpg-project#300), and split the same way for the same reason:
	// ErrActivationRefused is what a PLAYER produces (no charges, already
	// raging) and ErrBadActivation is wiring being wrong. Both are listed
	// because an unlisted sentinel reaching a host is a leak whether or not
	// anybody wrote an arm — Manager.Activate translates both to this
	// package's own vocabulary with %v, so neither is in the chain a host
	// sees.
	"resolution.ErrActivationRefused": resolution.ErrActivationRefused,
	"resolution.ErrBadActivation":     resolution.ErrBadActivation,
}

// refSentinels is core's identifier vocabulary — what a malformed ref is
// refused with underneath ErrBadRef.
//
// This is the third list, and it is the one whose OWNER is worth stating.
// ErrBadCharacter and ErrBadRef both sit over the rulebook modules the entity
// loaders call into, and of those only the ref parser answers with sentinel
// VALUES: character and monster report through rpgerr's codes, which are not
// reachable by errors.Is at all. So the matchable set underneath the loaders is
// core's, and listing it is what this file can actually assert.
//
// A ref crosses this seam as a STRING (S2 keeps core.Ref off the surface), so
// which parser reads it is an implementation detail — and a host that matched
// on core.ErrTooFewSegments would be coupled to that detail through the one
// channel the boundary test cannot see. When character or monster grows a
// sentinel of its own, it is added here and this file will say whether it
// escapes.
var refSentinels = map[string]error{
	"core.ErrEmptyString":       core.ErrEmptyString,
	"core.ErrInvalidFormat":     core.ErrInvalidFormat,
	"core.ErrEmptyComponent":    core.ErrEmptyComponent,
	"core.ErrInvalidCharacters": core.ErrInvalidCharacters,
	"core.ErrTooManySegments":   core.ErrTooManySegments,
	"core.ErrTooFewSegments":    core.ErrTooFewSegments,
}

// innerSentinels is the three lists as one: everything a refusal is checked
// against, whichever module produced it.
//
// One collection rather than three call sites, so a new module underneath this
// seam is covered by every scenario in this file the moment its list is added
// here — which is the opposite of how the resolution and ref leaks survived
// rpg-toolkit#1058.
var innerSentinels = []map[string]error{compositionSentinels, resolutionSentinels, refSentinels}

// sessionSentinels is the reviewed host-facing sentinel vocabulary. The AST
// completeness test below compares this allow-list with every exported Err*
// declaration in errors.go, so additions cannot silently escape review.
var sessionSentinels = map[string]error{
	"ErrNilInput":         session.ErrNilInput,
	"ErrNilConfig":        session.ErrNilConfig,
	"ErrIncompleteConfig": session.ErrIncompleteConfig,
	"ErrNotFound":         session.ErrNotFound,
	"ErrBadRepository":    session.ErrBadRepository,
	"ErrNoSession":        session.ErrNoSession,
	"ErrNoEncounter":      session.ErrNoEncounter,
	"ErrNoCharacter":      session.ErrNoCharacter,
	"ErrBadCharacter":     session.ErrBadCharacter,
	"ErrNoRef":            session.ErrNoRef,
	"ErrBadRef":           session.ErrBadRef,
	"ErrNoLoader":         session.ErrNoLoader,
	"ErrUnknownContent":   session.ErrUnknownContent,
	"ErrNoMemberID":       session.ErrNoMemberID,
	"ErrNoDeclarationID":  session.ErrNoDeclarationID,
	"ErrStaleDeclaration": session.ErrStaleDeclaration,
	"ErrNoMember":         session.ErrNoMember,
	"ErrStoryTrimmed":     session.ErrStoryTrimmed,
	"ErrClosed":           session.ErrClosed,
	"ErrNoEnding":         session.ErrNoEnding,
	"ErrEmptyPath":        session.ErrEmptyPath,
	"ErrBrokenPath":       session.ErrBrokenPath,
	"ErrLocked":           session.ErrLocked,
	"ErrDoorShut":         session.ErrDoorShut,
	"ErrNoConnection":     session.ErrNoConnection,
	"ErrBadPosition":      session.ErrBadPosition,
	"ErrNoSessionID":      session.ErrNoSessionID,
	"ErrNoEncounterID":    session.ErrNoEncounterID,
	"ErrSessionExists":    session.ErrSessionExists,
	"ErrInvalidWorld":     session.ErrInvalidWorld,
	"ErrInBubble":         session.ErrInBubble,
	"ErrNotInFight":       session.ErrNotInFight,
	"ErrNotYourTurn":      session.ErrNotYourTurn,
	"ErrNoCause":          session.ErrNoCause,
	"ErrNotACharacter":    session.ErrNotACharacter,
	"ErrNoSheet":          session.ErrNoSheet,
	"ErrDowned":           session.ErrDowned,
	"ErrBadAttack":        session.ErrBadAttack,
	"ErrCannotActivate":   session.ErrCannotActivate,
	"ErrBadActivation":    session.ErrBadActivation,
	"ErrOutOfReach":       session.ErrOutOfReach,
	"ErrCannotAfford":     session.ErrCannotAfford,
	"ErrBadCost":          session.ErrBadCost,
	"ErrInvalidSession":   session.ErrInvalidSession,
	"ErrSaveFailed":       session.ErrSaveFailed,
	"ErrBadTurnOutcome":   session.ErrBadTurnOutcome,
}

// TestExportedSentinelAllowListIsComplete makes sessionSentinels an actual
// allow-list rather than a list that can become incomplete unnoticed. It scans
// every Go file in the package, not just errors.go, so a stray exported Err*
// sentinel beside its one caller cannot bypass review.
func TestExportedSentinelAllowListIsComplete(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob package Go files: %v", err)
	}
	found := map[string]bool{}
	for _, path := range paths {
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
		for _, declaration := range file.Decls {
			gen, ok := declaration.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				values, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range values.Names {
					if name.IsExported() && strings.HasPrefix(name.Name, "Err") {
						found[name.Name] = true
					}
				}
			}
		}
	}
	if len(found) != len(sessionSentinels) {
		t.Fatalf("exported sentinel count changed: declarations=%d allow-list=%d", len(found), len(sessionSentinels))
	}
	for name := range found {
		if _, ok := sessionSentinels[name]; !ok {
			t.Errorf("exported sentinel %s is not reviewed in sessionSentinels", name)
		}
	}
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

// TestDeclarationSentinelVocabulary pins the selector-boundary additions as
// distinct host remedies. Completeness belongs to the exported allow-list/count
// test above rather than a tautological local len-two assertion.
func TestDeclarationSentinelVocabulary(t *testing.T) {
	selectorSentinels := []error{
		sessionSentinels["ErrNoDeclarationID"],
		sessionSentinels["ErrStaleDeclaration"],
	}
	for i, left := range selectorSentinels {
		for j, right := range selectorSentinels {
			if i != j && errors.Is(left, right) {
				t.Fatalf("selector sentinels %d and %d are not distinct", i, j)
			}
		}
	}
}

func (s *SentinelSuite) SetupTest() {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	// offsetWorld is the fixture with the most map in it: two regions
	// painted away from the origin, a door between them, and endings on both
	// sides. Painting it out there matters — a cell outside every region is
	// a real coordinate rather than a negative number, which is the mistake
	// a client actually makes.
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: offsetWorld(s.T()),
	})
	s.Require().NoError(err)
}

// refusedInOurVocabulary is the whole assertion of this file, in both
// directions: the sentinel a host is documented to match on is present, and not
// one of the inner modules' is.
//
// Asserting only the first half is what let this class of leak in. A refusal
// can carry our sentinel and theirs at the same time — errors.Is walks the
// whole chain — so the presence of ours proves nothing at all about what else
// the host can reach.
func (s *SentinelSuite) refusedInOurVocabulary(err error, want error) {
	s.T().Helper()
	s.Require().Error(err)
	s.Require().ErrorIs(err, want, "the host is answered in this package's vocabulary")
	for _, module := range innerSentinels {
		for name, inner := range module {
			s.NotErrorIs(err, inner,
				"a host can reach "+name+" through this refusal, and one that matched on it "+
					"would break the day that module is replaced (S2)")
		}
	}
}

// armedDuel rewires this suite onto a world where a swing means something.
//
// The map fixture in SetupTest is the wrong shape for one: its cast carries no
// weapon, and its rooms are anchored to make PATHING mistakes real rather than
// to put two people within reach of each other. duelWorld is already the
// smallest world a strike resolves in, so the swings below are the same swings
// attack_test.go pins rather than a second arrangement that could drift from
// them.
//
// The character repository is the parameter because every case here turns on
// what the host stored — an empty hand, one sheet under two names — which is
// the only part of a duel these refusals differ in.
func (s *SentinelSuite) armedDuel(chars *fakeCharacters) *session.Manager {
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: chars, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: duelWorld(s.T()),
	})
	s.Require().NoError(err)
	return mgr
}

func (s *SentinelSuite) swing(mgr *session.Manager) error {
	s.T().Helper()
	_, err := mgr.Attack(context.Background(), &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "bob",
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "alice"),
	})
	return err
}

// TestAWalkOffTheMap is the headline case, and the reason this file exists.
//
// Pathing to a cell no room owns is arithmetic any client can get wrong — not
// corrupt state, not a defect in this package, just a route computed one cell
// too far. It is therefore the leak a real host hits first.
func (s *SentinelSuite) TestAWalkOffTheMap() {
	// alice stands on authored [41,21]; the hall is painted over columns
	// 40..45 of rows 20..25. She steps to its west edge, then off the map.
	off := hexCell(39, 21)
	_, err := s.mgr.Move(context.Background(), &session.MoveInput{
		Session: "sess", Member: "alice",
		Path: []spatial.Position{hexCell(40, 21), off},
	})
	s.refusedInOurVocabulary(err, session.ErrBadPosition)
	s.Contains(err.Error(), fmt.Sprintf("(%v, %v)", off.X, off.Y),
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
	// A cell no region owns — the stored world naming somewhere that is
	// not on the map. Was `Room = "nowhere"` before members stopped
	// carrying a room at all (rpg-toolkit#1059).
	s.encounters.byID["world"].Members[0].Cell = &encounter.PositionData{X: 9999, Y: 9999}
	ctx := context.Background()

	_, err := s.mgr.Status(ctx, &session.StatusInput{Session: "sess"})
	s.refusedInOurVocabulary(err, session.ErrInvalidWorld)

	_, err = s.mgr.Where(ctx, &session.WhereInput{Session: "sess", Member: "alice"})
	s.refusedInOurVocabulary(err, session.ErrInvalidWorld)

	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{hexCell(42, 22)},
	})
	s.refusedInOurVocabulary(err, session.ErrInvalidWorld)

	s.Contains(err.Error(), "owned by no region",
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
	broken.Members[0].Cell = &encounter.PositionData{X: 9999, Y: 9999}

	_, err := s.mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "another", Encounter: "another-world", World: broken,
	})
	s.refusedInOurVocabulary(err, session.ErrInvalidWorld)
	s.Contains(err.Error(), "owned by no region",
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
		Path: []spatial.Position{hexCell(42, 21), hexCell(43, 21), hexCell(44, 21)},
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Outcome, "the walk ended the encounter")

	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "alice", Path: []spatial.Position{hexCell(44, 22)},
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

// TestASwingWithAnEmptyHandIsNotRefused pins the other side of rpg-toolkit#1168
// at this seam: a character with nothing in the main hand swings anyway — an
// unarmed strike, the rule rather than a gap — so this is no longer among the
// refusals a host meets from the rules. See attack_test.go's
// TestAnEmptyHandThrowsAnUnarmedStrike for the numbers.
func (s *SentinelSuite) TestASwingWithAnEmptyHandIsNotRefused() {
	mgr := s.armedDuel(newFakeCharacters(unarmedFighter("alice"), armedFighter("bob")))

	err := s.swing(mgr)
	s.NoError(err)
}

// TestOneSheetStoredUnderTwoNames pins dependency preflight ahead of
// resolution. The roster holds two members and the repository answers both
// with Alice's sheet; Afford marks the Attack Unreadable, and echoing that
// unavailable selector is stale before resolution or mutation.
func (s *SentinelSuite) TestOneSheetStoredUnderTwoNames() {
	chars := newFakeCharacters(armedFighter("alice"))
	chars.byID["bob"] = armedFighter("alice")

	err := s.swing(s.armedDuel(chars))
	s.refusedInOurVocabulary(err, session.ErrStaleDeclaration)
}

// TestASwingWithAnUnreadableSheet is the loader's own refusal on the swing
// path, kept here as the case that guards the OTHER wrap in the compiler.
//
// The rulebook answers this one through rpgerr rather than a sentinel value, so
// there is nothing for a host to match on today and this case was green from
// the start. It is here because that is a property of the character module this
// package does not control: the day a sentinel appears underneath the strict
// load, this scenario is what notices.
func (s *SentinelSuite) TestASwingWithAnUnreadableSheet() {
	mgr := s.armedDuel(newFakeCharacters(unreadableFighter("alice"), armedFighter("bob")))

	err := s.swing(mgr)
	s.refusedInOurVocabulary(err, session.ErrNoDeclarationID)
}

// TestADownedActorIsRefusedInOurWords is the death lane's own refusal, and the
// reason it belongs in this file is the two sentinels standing right behind it.
//
// The capability that answers "who is down" lives in this package, but the
// question is the composition's, and the composition has two ways to complain
// about the answer: encounter.ErrNoStanding if none was supplied, and
// encounter.ErrNotMember if one names somebody who is not in the roster. Both
// are reachable through a mis-wired seam rather than through a caller mistake —
// which is exactly the kind of leak that arrives wearing a working feature's
// clothes. A host that came to match on either would be coupled to the module
// this seam exists to keep replaceable.
//
// So the refusal is driven for real: bob puts alice at zero hit points, and
// alice is then refused the verbs a downed member cannot drive.
func (s *SentinelSuite) TestADownedActorIsRefusedInOurWords() {
	alice := armedFighter("alice")
	alice.Level = 5
	mgr := s.armedDuel(newFakeCharacters(alice, armedFighter("bob")))
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		_, err := mgr.Attack(ctx, &session.AttackInput{
			Session: "sess", Attacker: "alice", Target: "bob",
			DeclarationID: currentAttackID(s.T(), mgr, "sess", "alice"),
		})
		s.Require().NoError(err)
	}

	_, moveErr := mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "bob", Path: []spatial.Position{{X: 2, Y: 2}},
	})
	s.refusedInOurVocabulary(moveErr, session.ErrDowned)

	_, swingErr := mgr.Attack(ctx, &session.AttackInput{
		Session: "sess", Attacker: "bob", Target: "alice",
	})
	s.refusedInOurVocabulary(swingErr, session.ErrDowned)
	s.Contains(swingErr.Error(), "bob", "and the refusal still names who could not act")
}

// TestASecondSwingInOneTurn is the economy's refusal, driven for real.
//
// This is the most ORDINARY mistake on the list. Every other case here is a
// caller getting something wrong — a route off the map, a malformed ref, one
// sheet stored under two names — and this one is a player doing exactly what
// the game invites them to do and being told they have already acted. It will be
// the refusal a host sees most often by a wide margin, which makes it the one a
// leaked sentinel would hurt through most.
//
// The scene is a real fight rather than the duel the rest of this file uses,
// because free roam charges nothing and the refusal has no path there. See
// aFight in economy_test.go.
func (s *SentinelSuite) TestASecondSwingInOneTurn() {
	mgr, _, _, _ := aFight(s.T(), armedFighter("alice"), []int{1, 1, 1, 1})
	ctx := context.Background()

	_, err := mgr.Attack(ctx, &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton",
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "alice"),
	})
	s.Require().NoError(err, "the first swing is bought by the Attack action")

	_, err = mgr.Attack(ctx, &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "skeleton",
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "alice"),
	})
	s.refusedInOurVocabulary(err, session.ErrStaleDeclaration)
}

// TestASpawnNamingAMalformedRef is the third module and the second door.
//
// A ref crosses this seam as a string, so getting one wrong is the most
// ordinary mistake a host can make — a bare "skeleton" where the catalog wanted
// "dnd5e:monsters:skeleton". The parser underneath answers in core's
// vocabulary, and a host that matched on core.ErrTooFewSegments would be
// coupled to the fact that this package parses refs with core at all.
func (s *SentinelSuite) TestASpawnNamingAMalformedRef() {
	_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: "skeleton",
		Position: hexCell(42, 22),
	})
	s.refusedInOurVocabulary(err, session.ErrBadRef)
	s.Contains(err.Error(), "skeleton",
		"and the refusal still names the ref it could not read")
	s.Contains(err.Error(), "segments",
		"and still says what was wrong with it")
}
