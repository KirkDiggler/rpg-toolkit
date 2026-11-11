// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

// pace_commit_internal_test.go drives a pace commit (rpg-project#879 S13)
// inside one write scope over a fenced host, where a fall and a later write
// can be put between two commits of one verb directly.

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// paceFenced is a whole-record fenced host over the strike fakes: a landed
// save writes the world, the session record and the sheets it carries, and
// advances the version; a save against any other version is refused stale.
type paceFenced struct {
	*strikeEncounters
	sessions *strikeSessions
	chars    *strikeCharacters
	version  uint64
	// carried is the sheet ids each landed save carried, in order.
	carried [][]string
}

func (p *paceFenced) GetFencedEncounter(ctx context.Context, in *GetFencedEncounterInput) (*GetFencedEncounterOutput, error) {
	data, err := p.GetEncounter(ctx, in.Encounter)
	if err != nil {
		return nil, err
	}
	return &GetFencedEncounterOutput{Data: data, Version: p.version}, nil
}

func (p *paceFenced) SaveFenced(_ context.Context, in *SaveFencedInput) (*SaveFencedOutput, error) {
	if in.Expected != p.version {
		return nil, fmt.Errorf("at %d, expected %d: %w", p.version, in.Expected, ErrStaleCheckpoint)
	}
	p.byID[in.Encounter] = in.Data
	if in.Session != nil {
		p.sessions.byID[in.Session.ID] = in.Session
	}
	var ids []string
	for _, sheet := range in.Characters {
		p.chars.byID[sheet.ID] = sheet
		ids = append(ids, sheet.ID)
	}
	p.carried = append(p.carried, ids)
	p.version++
	return &SaveFencedOutput{Version: p.version}, nil
}

// PaceCommitInternalSuite is one fighter and one goblin in a tomb, over a
// fenced host.
type PaceCommitInternalSuite struct {
	suite.Suite

	store *paceFenced
	mgr   *Manager
}

func TestPaceCommitInternalSuite(t *testing.T) { suite.Run(t, new(PaceCommitInternalSuite)) }

func (s *PaceCommitInternalSuite) SetupTest() {
	s.store = &paceFenced{
		strikeEncounters: &strikeEncounters{byID: map[string]*encounter.EncounterData{}},
		sessions:         &strikeSessions{byID: map[string]*SessionData{}},
		chars: &strikeCharacters{byID: map[string]*character.Data{
			"fighter": strikeFixtureFighter("fighter"),
		}},
	}
	mgr, err := NewManager(&Config{
		Record: DiscardRecord{}, Acts: DiscardActs{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: &scriptedDice{rolls: []int{10, 10, 10, 10, 10, 10}}, TurnDriver: Pass{},
		Sessions: s.store.sessions, Encounters: s.store, Characters: s.store.chars,
	})
	s.Require().NoError(err)
	s.Require().NotNil(mgr.fenced, "the fixture's premise: the host is fenced")
	s.mgr = mgr

	sc := scene{
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("tomb", 0, 0, 12, 6)},
		},
		Endings: []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
	}
	sc.Party = []sceneSeat{seatAt("fighter", 2, 0)}
	sc.Monsters = []dungeonspec.MonsterPlacement{monsterAt("goblin", refs.Monsters.Goblin().String(), 3, 0)}
	launchScene(s.T(), mgr, sc)
	s.store.carried = nil
}

// open is a fenced write scope with an act open, as a continued walk holds.
func (s *PaceCommitInternalSuite) open() *writeScope {
	scope, err := s.mgr.openForChange(context.Background(), string(VerbMove), "sess")
	s.Require().NoError(err)
	s.Require().True(scope.fence.held)
	return scope
}

// miss records a beat in the open act, the way a later step does.
func (s *PaceCommitInternalSuite) miss(scope *writeScope) {
	_, err := recordOne(scope.enc, &encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: "fighter", Targets: []encounter.MemberID{"goblin"},
	})
	s.Require().NoError(err)
}

// TestAFallIsSettledInThePaceCommitThatSavesIt: a fall before a pace
// boundary is paid in that boundary's save, so a crash after it cannot lose
// the grant; and the final commit, reading from the cursor, does not pay it
// again.
func (s *PaceCommitInternalSuite) TestAFallIsSettledInThePaceCommitThatSavesIt() {
	ctx := context.Background()
	scope := s.open()
	sheet, ok := npcSheet(scope.data, "goblin")
	s.Require().True(ok)
	worth := sheet.Experience
	s.Require().Positive(worth, "the fixture's premise: the goblin is worth something")
	before := s.store.chars.byID["fighter"].Experience
	dropped := *sheet
	dropped.HitPoints = 0
	scope.replaceMonsterSheet(&dropped)
	s.miss(scope) // the consult that notices the body and tells the fall

	_, _, err := s.mgr.commitPace(ctx, scope)
	s.Require().NoError(err)
	s.Require().Len(s.store.carried, 1)
	s.Contains(s.store.carried[0], "fighter", "the grant rides the pace commit's own save")
	s.Equal(before+worth, s.store.chars.byID["fighter"].Experience, "the fall is paid when its commit lands")
	s.NotEmpty(scope.act, "the act stays open across a pace commit")

	s.miss(scope)
	_, _, err = s.mgr.commit(ctx, scope)
	s.Require().NoError(err)
	s.Require().Len(s.store.carried, 2)
	s.Equal(before+worth, s.store.chars.byID["fighter"].Experience,
		"the final commit settles from the cursor: the fall is not paid twice")
	s.Empty(scope.act, "the final commit closes the act")
}

// TestASheetWrittenAfterAPaceCommitRidesTheNextSave: a pace commit leaves the
// fence's saved flag down, so a sheet the walk writes after it is held for
// the next save and lands there. Only the final commit arms the refusal: a
// sheet or seat written after it has no save left to ride.
func (s *PaceCommitInternalSuite) TestASheetWrittenAfterAPaceCommitRidesTheNextSave() {
	ctx := context.Background()
	scope := s.open()
	s.miss(scope)
	_, _, err := s.mgr.commitPace(ctx, scope)
	s.Require().NoError(err)
	s.False(scope.fence.saved, "a pace commit is not the verb's last save")

	fighter := s.store.chars.byID["fighter"]
	edited := *fighter
	edited.Experience = fighter.Experience + 7
	s.Require().NoError(s.mgr.sheetsFor(scope).save(ctx, &edited), "a write after a pace commit is accepted")

	s.miss(scope)
	_, _, err = s.mgr.commit(ctx, scope)
	s.Require().NoError(err)
	s.Require().Len(s.store.carried, 2)
	s.Contains(s.store.carried[1], "fighter", "the held sheet rides the final save")
	s.Equal(edited.Experience, s.store.chars.byID["fighter"].Experience)
	s.True(scope.fence.saved, "the final commit arms the refusal")

	err = s.mgr.sheetsFor(scope).save(ctx, &edited)
	s.ErrorIs(err, ErrBadRepository, "a sheet after the final save has nothing to ride")
	var saveErr *SaveError
	s.Require().ErrorAs(err, &saveErr)
	s.Equal([]string{"character:fighter"}, saveErr.Report.Failed)
	err = s.mgr.writeSeat(ctx, scope, "fighter", "")
	s.ErrorIs(err, ErrBadRepository, "nor a seat")
	sheets, seats := s.mgr.pending.take("sess")
	s.Empty(sheets, "the late sheet was not held")
	s.Empty(seats, "the late seat was not held")
}

// TestAStalePaceCommitLeavesTheCursor: a refused save moves nothing, so the
// commit's delta is still unsaved and undelivered, and the act stays open.
func (s *PaceCommitInternalSuite) TestAStalePaceCommitLeavesTheCursor() {
	ctx := context.Background()
	scope := s.open()
	s.miss(scope)
	from := scope.deltaFrom()
	s.store.version++ // another writer landed
	_, _, err := s.mgr.commitPace(ctx, scope)
	s.Require().ErrorIs(err, ErrStaleCheckpoint)
	s.Equal(from, scope.deltaFrom(), "the cursor moves only with a landed save")
	s.Empty(scope.savedThrough)
	s.False(scope.fence.saved)
}
